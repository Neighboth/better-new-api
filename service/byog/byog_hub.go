package byog

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gorilla/websocket"
)

// Tunnel message types
const (
	MsgTypeHeartbeat       = "heartbeat"
	MsgTypeHeartbeatAck    = "heartbeat_ack"
	MsgTypeInferenceReq    = "inference_request"
	MsgTypeInferenceChunk  = "inference_chunk"
	MsgTypeInferenceDone   = "inference_done"
	MsgTypeInferenceError  = "inference_error"
	MsgTypeCanaryReq       = "canary_request"
	MsgTypeCanaryResp      = "canary_response"
	MsgTypeByteProbeReq    = "byte_probe_request"
	MsgTypeByteProbeResp   = "byte_probe_response"
	MsgTypeDownloadModel   = "download_model"
	MsgTypeDownloadProgress = "download_progress"
	MsgTypePurgeModel      = "purge_model"
	MsgTypeForceUpdate     = "force_update"
	MsgTypePauseWorker     = "pause_worker"
	MsgTypeResumeWorker    = "resume_worker"
)

// Tunnel Frame
type TunnelFrame struct {
	Type      string          `json:"type"`
	Id        string          `json:"id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// Telemetry Payload in Heartbeat
type TelemetryPayload struct {
	CpuTemp         float64  `json:"cpu_temp"`
	GpuTemp         float64  `json:"gpu_temp"`
	BatteryTemp     float64  `json:"battery_temp"`
	FanSpeed        int      `json:"fan_speed"`
	BatteryPercent  int      `json:"battery_percent"`
	IsCharging      bool     `json:"is_charging"`
	CurrentTps      float64  `json:"current_tps"`
	CurrentTtftMs   int      `json:"current_ttft_ms"`
	TotalTokens     int64    `json:"total_tokens"`
	InstalledModels []string `json:"installed_models"`
	DownloadStatus  string   `json:"download_status,omitempty"`
	IsGamerMode     bool     `json:"is_gamer_mode"`
	IsThrottled     bool     `json:"is_throttled"`
}

// Inference Request Payload
type InferencePayload struct {
	RequestId       string          `json:"request_id"`
	ModelId         string          `json:"model_id"`
	LoraId          string          `json:"lora_id,omitempty"`
	Messages        json.RawMessage `json:"messages"`
	Prompt          string          `json:"prompt,omitempty"`
	Stream          bool            `json:"stream"`
	Temperature     float64         `json:"temperature"`
	MaxTokens       int             `json:"max_tokens"`
	Seed            int64           `json:"seed,omitempty"`
	Stop            []string        `json:"stop,omitempty"`
	TopP            float64         `json:"top_p,omitempty"`
	PresencePenalty float64         `json:"presence_penalty,omitempty"`
}

// Inference Chunk Payload
type ChunkPayload struct {
	RequestId string `json:"request_id"`
	Content   string `json:"content"`
	TokenId   int    `json:"token_id,omitempty"`
	Logprob   float64 `json:"logprob,omitempty"`
	IsFinal   bool   `json:"is_final"`
}

// Inference Done Payload
type DonePayload struct {
	RequestId    string  `json:"request_id"`
	TotalTokens  int     `json:"total_tokens"`
	PromptTokens int     `json:"prompt_tokens"`
	TtftMs       int     `json:"ttft_ms"`
	Tps          float64 `json:"tps"`
	RawTokens    []int   `json:"raw_tokens,omitempty"`
	Logprobs     []float64 `json:"logprobs,omitempty"`
	Error        string  `json:"error,omitempty"`
}

// Worker Node Session
type NodeSession struct {
	NodeId          string
	UserId          int
	Conn            *websocket.Conn
	SendChan        chan TunnelFrame
	PendingRequests sync.Map // map[requestId]chan *TunnelFrame
	LastHeartbeat   time.Time
	UptimeStart     time.Time
	ActiveRequests  int32
	MaxConcurrency  int
	DeviceType      string
	IsPaused        bool
	IsGamerMode     bool
	InstalledModels map[string]bool
	Mu              sync.Mutex
}

// Hub Managing All Active Node Sessions
type Hub struct {
	sessions sync.Map // map[nodeId]*NodeSession
	mu       sync.RWMutex
}

var GlobalHub = &Hub{}

func (h *Hub) Register(session *NodeSession) {
	h.sessions.Store(session.NodeId, session)
	// Update Node Status in DB
	common.SysLog(fmt.Sprintf("[BYOG] Node connected: %s (User: %d)", session.NodeId, session.UserId))
	_ = model.DB.Model(&model.ByogNode{}).
		Where("node_id = ?", session.NodeId).
		Updates(map[string]interface{}{
			"status":            "online",
			"last_heartbeat_at": time.Now().Unix(),
		})
}

func (h *Hub) Unregister(nodeId string) {
	if val, ok := h.sessions.LoadAndDelete(nodeId); ok {
		session := val.(*NodeSession)
		session.Mu.Lock()
		if session.Conn != nil {
			_ = session.Conn.Close()
		}
		session.Mu.Unlock()
		common.SysLog(fmt.Sprintf("[BYOG] Node disconnected: %s", nodeId))
		_ = model.DB.Model(&model.ByogNode{}).
			Where("node_id = ?", nodeId).
			Updates(map[string]interface{}{
				"status": "offline",
			})
	}
}

func (h *Hub) GetSession(nodeId string) (*NodeSession, bool) {
	val, ok := h.sessions.Load(nodeId)
	if !ok {
		return nil, false
	}
	return val.(*NodeSession), true
}

func (h *Hub) GetAllOnlineNodes() []*NodeSession {
	var list []*NodeSession
	h.sessions.Range(func(key, value interface{}) bool {
		s := value.(*NodeSession)
		if !s.IsPaused && !s.IsGamerMode {
			list = append(list, s)
		}
		return true
	})
	return list
}

func (s *NodeSession) WriteFrame(frame TunnelFrame) error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Conn == nil {
		return errors.New("connection closed")
	}
	frame.Timestamp = time.Now().UnixMilli()
	return s.Conn.WriteJSON(frame)
}

// Handle Heartbeat Frame
func (s *NodeSession) HandleHeartbeat(frame *TunnelFrame) {
	var telemetry TelemetryPayload
	if err := json.Unmarshal(frame.Payload, &telemetry); err == nil {
		s.LastHeartbeat = time.Now()
		s.IsGamerMode = telemetry.IsGamerMode

		// Update node's local models
		s.InstalledModels = make(map[string]bool)
		for _, m := range telemetry.InstalledModels {
			s.InstalledModels[m] = true
		}

		// Calculate server-side uptime
		uptimeSecs := int64(time.Since(s.UptimeStart).Seconds())

		installedJson, _ := json.Marshal(telemetry.InstalledModels)

		status := "online"
		if telemetry.IsGamerMode {
			status = "gamer_mode"
		} else if telemetry.IsThrottled {
			status = "throttled"
		}

		// Fetch existing node to calculate multipliers
		node, err := model.GetByogNodeByNodeId(s.NodeId)
		if err == nil && node != nil {
			node.CpuTemp = telemetry.CpuTemp
			node.GpuTemp = telemetry.GpuTemp
			node.BatteryTemp = telemetry.BatteryTemp
			node.FanSpeed = telemetry.FanSpeed
			node.BatteryPercent = telemetry.BatteryPercent
			node.IsCharging = telemetry.IsCharging
			node.CurrentTps = telemetry.CurrentTps
			node.CurrentTtftMs = telemetry.CurrentTtftMs
			node.TotalTokensProcessed += telemetry.TotalTokens
			node.UptimeSeconds = uptimeSecs
			node.Status = status
			node.InstalledModels = string(installedJson)

			// Calculate dynamic boost multipliers
			multiplier, bonusKeys := model.CalculateNodeMultipliers(node)
			bonusJson, _ := json.Marshal(bonusKeys)
			node.ActiveMultipliers = string(bonusJson)

			// Check if battery cutoff applies on mobile
			if node.DeviceType == "mobile" || node.DeviceType == "tablet" {
				if node.BatteryCutoffEnabled && node.BatteryPercent < node.BatteryCutoffPercent {
					s.IsPaused = true
					node.Status = "paused"
				}
				if node.BatteryTempSupported && node.BatteryTemp >= node.MaxBatteryTemp {
					s.IsPaused = true
					node.Status = "throttled"
				}
			}

			_ = model.SaveByogNode(node)

			// Ack back with active multiplier
			ackPayload, _ := json.Marshal(map[string]interface{}{
				"multiplier": multiplier,
				"bonuses":    bonusKeys,
				"uptime":     uptimeSecs,
			})
			_ = s.WriteFrame(TunnelFrame{
				Type:    MsgTypeHeartbeatAck,
				Payload: ackPayload,
			})
		}
	}
}

// WebSocket upgrader
var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Auth handled by token/node_id
	},
	ReadBufferSize:  65536,
	WriteBufferSize: 65536,
}
