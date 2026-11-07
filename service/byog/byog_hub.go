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
	MsgTypeHeartbeat        = "heartbeat"
	MsgTypeHeartbeatAck     = "heartbeat_ack"
	MsgTypeInferenceReq     = "inference_request"
	MsgTypeInferenceChunk   = "inference_chunk"
	MsgTypeInferenceDone    = "inference_done"
	MsgTypeInferenceError   = "inference_error"
	MsgTypeCanaryReq        = "canary_request"
	MsgTypeCanaryResp       = "canary_response"
	MsgTypeByteProbeReq     = "byte_probe_request"
	MsgTypeByteProbeResp    = "byte_probe_response"
	MsgTypeDownloadModel    = "download_model"
	MsgTypeDownloadProgress = "download_progress"
	MsgTypePurgeModel       = "purge_model"
	MsgTypeForceUpdate      = "force_update"
	MsgTypePauseWorker      = "pause_worker"
	MsgTypeResumeWorker     = "resume_worker"
	MsgTypeDeviceRemoved    = "device_removed"
	MsgTypePairingRevoked   = "pairing_revoked"
	// MsgTypeTunnelClosed is a synthetic frame the coordinator produces when the
	// tunnel of a node dies while requests are waiting. The waiter of a request
	// receives an immediate error instead of waiting for its timeout.
	MsgTypeTunnelClosed = "tunnel_closed"
)

// NotifyDeviceRemoved tells a worker that it was removed from the network.
//
// The worker clears the pairing and shows the state in its interface, the device does
// not have to wait for the next registration to notice the removal.
func NotifyDeviceRemoved(nodeId string, message string) {
	if nodeId == "" {
		return
	}
	session, ok := GlobalHub.GetSession(nodeId)
	if !ok || session == nil {
		return
	}
	payload, err := json.Marshal(map[string]interface{}{
		"node_id": nodeId,
		"message": message,
		"reason":  "removed",
	})
	if err != nil {
		return
	}
	_ = session.WriteFrame(TunnelFrame{Type: MsgTypeDeviceRemoved, Payload: payload})
}

// Tunnel Frame
type TunnelFrame struct {
	Type      string          `json:"type"`
	Id        string          `json:"id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// Telemetry Payload in Heartbeat
//
// The hardware of the computer is measured by the client and reported with
// every heartbeat. The fields that the client cannot measure are pointers, a
// value that is missing is not the same as a value of zero: a computer without
// a dedicated graphics adapter reports zero megabytes of video memory and the
// panel of the network shows that it has none.
type TelemetryPayload struct {
	CpuTemp          float64  `json:"cpu_temp"`
	GpuTemp          float64  `json:"gpu_temp"`
	BatteryTemp      float64  `json:"battery_temp"`
	FanSpeed         int      `json:"fan_speed"`
	BatteryPercent   int      `json:"battery_percent"`
	IsCharging       bool     `json:"is_charging"`
	CurrentTps       float64  `json:"current_tps"`
	CurrentTtftMs    int      `json:"current_ttft_ms"`
	TotalTokens      int64    `json:"total_tokens"`
	InstalledModels  []string `json:"installed_models"`
	ModelStatus      string   `json:"model_status,omitempty"`
	DownloadedMB     int      `json:"downloaded_mb,omitempty"`
	TotalMB          int      `json:"total_mb,omitempty"`
	DownloadProgress int      `json:"download_progress,omitempty"`
	ActiveVramModel  string   `json:"active_vram_model,omitempty"`
	TotalDiskGB      int      `json:"total_disk_gb,omitempty"`
	FreeDiskGB       int      `json:"free_disk_gb,omitempty"`
	DownloadStatus   string   `json:"download_status,omitempty"`
	IsGamerMode      bool     `json:"is_gamer_mode"`
	IsThrottled      bool     `json:"is_throttled"`

	// The model of the progress that the client reports.
	DownloadModelId   string   `json:"download_model_id,omitempty"`
	DownloadSpeedBps  float64  `json:"download_speed_bps,omitempty"`
	EtaSeconds        int      `json:"eta_seconds,omitempty"`
	DownloadStartedAt int64    `json:"download_started_at,omitempty"`
	DownloadQueue     []string `json:"download_queue,omitempty"`

	// The hardware of the computer.
	CpuModel          string          `json:"cpu_model,omitempty"`
	GpuModel          string          `json:"gpu_model,omitempty"`
	CpuCores          *int            `json:"cpu_cores,omitempty"`
	PhysicalRamMB     *int            `json:"physical_ram_mb,omitempty"`
	SwapRamMB         *int            `json:"swap_ram_mb,omitempty"`
	VramMB            *int            `json:"vram_mb,omitempty"`
	SharedVramMB      *int            `json:"shared_vram_mb,omitempty"`
	Disks             json.RawMessage `json:"disks,omitempty"`
	BandwidthDownMbps *float64        `json:"bandwidth_down_mbps,omitempty"`
	BandwidthUpMbps   *float64        `json:"bandwidth_up_mbps,omitempty"`
	PingMs            *int            `json:"ping_ms,omitempty"`
	HotStartEnabled   *bool           `json:"hot_start_enabled,omitempty"`
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
	RequestId string  `json:"request_id"`
	Content   string  `json:"content"`
	TokenId   int     `json:"token_id,omitempty"`
	Logprob   float64 `json:"logprob,omitempty"`
	IsFinal   bool    `json:"is_final"`
}

// Inference Done Payload
type DonePayload struct {
	RequestId    string    `json:"request_id"`
	TotalTokens  int       `json:"total_tokens"`
	PromptTokens int       `json:"prompt_tokens"`
	TtftMs       int       `json:"ttft_ms"`
	Tps          float64   `json:"tps"`
	RawTokens    []int     `json:"raw_tokens,omitempty"`
	Logprobs     []float64 `json:"logprobs,omitempty"`
	Error        string    `json:"error,omitempty"`
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
	h.mu.Lock()
	defer h.mu.Unlock()
	h.registerLocked(session)
}

// RegisterAuthenticated rechecks revocation while synchronizing with session
// removal. Deletion commits before Unregister, so either order closes the race.
func (h *Hub) RegisterAuthenticated(session *NodeSession, secretHash string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if model.DB == nil || session == nil || session.UserId <= 0 || secretHash == "" {
		return false
	}
	var node model.ByogNode
	if err := model.DB.Where("node_id = ? AND user_id = ? AND worker_secret_hash = ? AND status <> ?", session.NodeId, session.UserId, secretHash, "banned").First(&node).Error; err != nil {
		return false
	}
	banned, err := model.CheckByogBan(node.UserId, node.HWID)
	if err != nil || banned {
		return false
	}
	h.registerLocked(session)
	return true
}

func (h *Hub) registerLocked(session *NodeSession) {
	if old, loaded := h.sessions.Swap(session.NodeId, session); loaded {
		previous := old.(*NodeSession)
		if previous != session {
			previous.Mu.Lock()
			if previous.Conn != nil {
				_ = previous.Conn.Close()
			}
			previous.Mu.Unlock()
			previous.FailPendingRequests("the worker reconnected")
		}
	}
	// Update Node Status in DB
	common.SysLog(fmt.Sprintf("[BYOG] Node connected: %s (User: %d)", session.NodeId, session.UserId))
	if model.DB != nil {
		_ = model.DB.Model(&model.ByogNode{}).
			Where("node_id = ? AND status <> ?", session.NodeId, "banned").
			Updates(map[string]interface{}{
				"status":            "online",
				"last_heartbeat_at": time.Now().Unix(),
			})
	}
}

// Unregister removes the current session (for example on administrative removal).
func (h *Hub) Unregister(nodeId string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if val, ok := h.sessions.LoadAndDelete(nodeId); ok {
		h.closeSession(val.(*NodeSession))
	}
}

// UnregisterSession must not remove a newer connection for the same node.
func (h *Hub) UnregisterSession(session *NodeSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions.CompareAndDelete(session.NodeId, session) {
		h.closeSession(session)
	}
}

func (h *Hub) closeSession(session *NodeSession) {
	session.Mu.Lock()
	if session.Conn != nil {
		_ = session.Conn.Close()
	}
	session.Mu.Unlock()
	session.FailPendingRequests("the connection of the worker closed while the request was running")
	common.SysLog(fmt.Sprintf("[BYOG] Node disconnected: %s", session.NodeId))
	if model.DB != nil {
		_ = model.DB.Model(&model.ByogNode{}).Where("node_id = ? AND status <> ?", session.NodeId, "banned").Updates(map[string]interface{}{"status": "offline"})
	}
}

// FailPendingRequests wakes every request that waits for an answer of the node
// with a synthetic tunnel-closed frame.
func (s *NodeSession) FailPendingRequests(reason string) {
	payload, err := json.Marshal(map[string]interface{}{
		"error":   reason,
		"node_id": s.NodeId,
	})
	if err != nil {
		return
	}
	s.PendingRequests.Range(func(key, value interface{}) bool {
		ch, ok := value.(chan *TunnelFrame)
		if !ok {
			return true
		}
		frame := &TunnelFrame{
			Type:      MsgTypeTunnelClosed,
			Id:        fmt.Sprintf("%v", key),
			Payload:   payload,
			Timestamp: time.Now().UnixMilli(),
		}
		select {
		case ch <- frame:
		default:
		}
		return true
	})
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
			if telemetry.CurrentTps > 0 {
				node.CurrentTps = telemetry.CurrentTps
			}
			if telemetry.CurrentTtftMs > 0 {
				node.CurrentTtftMs = telemetry.CurrentTtftMs
			}
			if telemetry.TotalTokens > node.TotalTokensProcessed {
				node.TotalTokensProcessed = telemetry.TotalTokens
			}
			node.UptimeSeconds = uptimeSecs
			node.Status = status
			node.InstalledModels = string(installedJson)

			// The download telemetry follows the very same merge rules as the
			// HTTP registration of the worker: one path must never write the
			// progress of an older moment over the progress of the other path,
			// the bar of the panel never jumps between the two reports.
			queueJSON := ""
			if len(telemetry.DownloadQueue) > 0 || telemetry.ModelStatus == "ready" {
				if encoded, err := json.Marshal(telemetry.DownloadQueue); err == nil {
					queueJSON = string(encoded)
				}
			}
			model.ApplyDownloadTelemetry(node, model.DownloadTelemetry{
				ModelId:        telemetry.DownloadModelId,
				StartedAt:      telemetry.DownloadStartedAt,
				HasProgress:    telemetry.DownloadModelId != "" || telemetry.DownloadedMB > 0 || telemetry.TotalMB > 0 || telemetry.DownloadProgress > 0,
				DownloadedMB:   telemetry.DownloadedMB,
				TotalMB:        telemetry.TotalMB,
				Progress:       telemetry.DownloadProgress,
				SpeedBps:       telemetry.DownloadSpeedBps,
				EtaSeconds:     telemetry.EtaSeconds,
				ModelStatus:    telemetry.ModelStatus,
				DownloadStatus: telemetry.DownloadStatus,
				QueueJSON:      queueJSON,
			})
			if telemetry.ActiveVramModel != "" {
				node.ActiveVramModel = telemetry.ActiveVramModel
			}
			// The hardware of the computer. The client reports the values that
			// it measured, the coordinator stores them as they are.
			if telemetry.CpuModel != "" {
				node.CpuModel = telemetry.CpuModel
			}
			if telemetry.GpuModel != "" {
				node.GpuModel = telemetry.GpuModel
			}
			if telemetry.CpuCores != nil && *telemetry.CpuCores > 0 {
				node.CpuCores = *telemetry.CpuCores
			}
			if telemetry.PhysicalRamMB != nil && *telemetry.PhysicalRamMB > 0 {
				node.PhysicalRamMB = *telemetry.PhysicalRamMB
			}
			if telemetry.SwapRamMB != nil && *telemetry.SwapRamMB >= 0 {
				node.SwapRamMB = *telemetry.SwapRamMB
			}
			if telemetry.VramMB != nil && *telemetry.VramMB >= 0 {
				node.VramMB = *telemetry.VramMB
			}
			if telemetry.SharedVramMB != nil && *telemetry.SharedVramMB >= 0 {
				node.SharedVramMB = *telemetry.SharedVramMB
			}
			if telemetry.BandwidthDownMbps != nil && *telemetry.BandwidthDownMbps > 0 {
				node.BandwidthDownMbps = *telemetry.BandwidthDownMbps
			}
			if telemetry.BandwidthUpMbps != nil && *telemetry.BandwidthUpMbps > 0 {
				node.BandwidthUpMbps = *telemetry.BandwidthUpMbps
			}
			if telemetry.PingMs != nil && *telemetry.PingMs > 0 {
				node.PingMs = *telemetry.PingMs
			}
			if telemetry.HotStartEnabled != nil {
				node.HotStartEnabled = *telemetry.HotStartEnabled
			}
			if len(telemetry.Disks) > 2 {
				var disks []map[string]interface{}
				if err := json.Unmarshal(telemetry.Disks, &disks); err == nil && len(disks) > 0 {
					node.Disks = string(telemetry.Disks)
				}
			}
			if telemetry.TotalDiskGB > 0 {
				node.TotalDiskGB = telemetry.TotalDiskGB
			}
			if telemetry.FreeDiskGB > 0 {
				node.FreeDiskGB = telemetry.FreeDiskGB
			}
			if telemetry.PhysicalRamMB != nil && node.AllocatedRamGB > float64(*telemetry.PhysicalRamMB)/1024.0 {
				node.AllocatedRamGB = float64(*telemetry.PhysicalRamMB) / 1024.0
			}
			if telemetry.VramMB != nil {
				if *telemetry.VramMB <= 512 {
					node.VramMB = 0
					node.AllocatedVramGB = 0
				} else {
					node.VramMB = *telemetry.VramMB
					if node.AllocatedVramGB > node.VramMB/1024 {
						node.AllocatedVramGB = node.VramMB / 1024
					}
				}
			}
			if node.AllocatedDiskGB > node.FreeDiskGB && node.FreeDiskGB > 0 {
				node.AllocatedDiskGB = node.FreeDiskGB
			}
			node.LastHeartbeatAt = time.Now().Unix()
			node.UpdatedAt = time.Now().Unix()

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

			_ = model.SaveByogLiveNode(node)

			// Ack back with active multiplier
			ackPayload, _ := json.Marshal(map[string]interface{}{
				"multiplier": multiplier,
				"bonuses":    bonusKeys,
				"uptime":     uptimeSecs,
				// The settings of the node belong to the installation: the
				// window of the client and the page of the client show the same
				// values because both read them from here.
				"hot_start_enabled": node.HotStartEnabled,
				"allocated_ram_gb":  node.AllocatedRamGB,
				"allocated_vram_gb": node.AllocatedVramGB,
				"allocated_cores":   node.AllocatedCores,
				"allocated_disk_gb": node.AllocatedDiskGB,
				"free_disk_gb":      node.FreeDiskGB,
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
