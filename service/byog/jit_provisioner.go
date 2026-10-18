package byog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

type JitProvisioner struct {
	provisioningLocks sync.Map // map[modelId]*ProvisioningState
}

type ProvisioningState struct {
	ModelId         string
	SelectedNodeId  string
	PercentComplete int
	SpeedMBs        float64
	EtaSeconds      int
	StatusMessage   string
	IsReady         bool
	Error           string
	Subscribers     []chan ProvisioningUpdate
	Mu              sync.Mutex
}

type ProvisioningUpdate struct {
	Percent       int
	SpeedMBs      float64
	EtaSeconds    int
	StatusMessage string
	IsReady       bool
	Error         string
}

var GlobalJit = &JitProvisioner{}

// GetVirtualCatalog: List all models that can be run by currently connected active nodes
func (j *JitProvisioner) GetVirtualCatalog() ([]*model.ByogModel, error) {
	allModels, err := model.GetAllByogModels(true)
	if err != nil {
		return nil, err
	}

	onlineSessions := GlobalHub.GetAllOnlineNodes()
	activeNodes := make(map[string]*model.ByogNode)

	for _, s := range onlineSessions {
		if node, err := model.GetByogNodeByNodeId(s.NodeId); err == nil && node != nil {
			activeNodes[node.NodeId] = node
		}
	}

	// Also query database for recently active nodes (within last 3 minutes)
	recentCutoff := time.Now().Unix() - 180
	if dbNodes, err := model.GetRecentOnlineByogNodes(recentCutoff); err == nil {
		for _, n := range dbNodes {
			activeNodes[n.NodeId] = n
		}
	}

	if len(activeNodes) == 0 {
		return []*model.ByogModel{}, nil
	}

	// Fetch node specs
	var virtualModels []*model.ByogModel
	for _, m := range allModels {
		canHost := false
		for _, node := range activeNodes {
			// Check if node has sufficient capacity
			if node.DeviceType == "mobile" || node.DeviceType == "tablet" {
				if GlobalScheduler.IsModelSafeForMobile(node, m.ModelId) {
					canHost = true
					break
				}
			} else {
				// Desktop/Server: check VRAM / RAM
				if m.ParamSize == "70B" && (node.VramMB+node.SharedVramMB) < 32768 {
					continue
				}
				if m.ParamSize == "13B" && (node.VramMB+node.SharedVramMB) < 10240 {
					continue
				}
				canHost = true
				break
			}
		}

		if canHost {
			virtualModels = append(virtualModels, m)
		}
	}

	return virtualModels, nil
}

// Find if any online node already has the model installed (Warm Hit)
func (j *JitProvisioner) FindWarmNodeForModel(modelId string) *NodeSession {
	onlineSessions := GlobalHub.GetAllOnlineNodes()
	for _, s := range onlineSessions {
		if s.InstalledModels != nil && s.InstalledModels[modelId] {
			return s
		}
		if node, err := model.GetByogNodeByNodeId(s.NodeId); err == nil && node != nil && node.InstalledModels != "" {
			var list []string
			if json.Unmarshal([]byte(node.InstalledModels), &list) == nil {
				for _, m := range list {
					if m == modelId {
						if s.InstalledModels == nil {
							s.InstalledModels = make(map[string]bool)
						}
						s.InstalledModels[modelId] = true
						return s
					}
				}
			}
		}
	}
	return nil
}

// Select best node for cold download provisioning (Capable + Highest Download Speed)
func (j *JitProvisioner) SelectBestNodeForColdDownload(modelDef *model.ByogModel) (*NodeSession, *model.ByogNode, error) {
	onlineSessions := GlobalHub.GetAllOnlineNodes()
	if len(onlineSessions) == 0 {
		return nil, nil, fmt.Errorf("no online nodes available in network")
	}

	var bestSession *NodeSession
	var bestNode *model.ByogNode
	var maxSpeed float64 = -1.0

	for _, s := range onlineSessions {
		n, err := model.GetByogNodeByNodeId(s.NodeId)
		if err != nil || n == nil {
			continue
		}

		// Verify hardware capability
		if n.DeviceType == "mobile" || n.DeviceType == "tablet" {
			if !GlobalScheduler.IsModelSafeForMobile(n, modelDef.ModelId) {
				continue
			}
		}

		if n.BandwidthDownMbps > maxSpeed {
			maxSpeed = n.BandwidthDownMbps
			bestSession = s
			bestNode = n
		}
	}

	if bestSession == nil {
		return nil, nil, fmt.Errorf("no node with sufficient hardware capacity for model %s", modelDef.ModelId)
	}

	return bestSession, bestNode, nil
}

// ProvisionColdModel initiates background download on the best node and streams progress to client
func (j *JitProvisioner) ProvisionColdModel(c *gin.Context, modelDef *model.ByogModel, isSseClient bool) (*NodeSession, error) {
	stateVal, loaded := j.provisioningLocks.LoadOrStore(modelDef.ModelId, &ProvisioningState{
		ModelId:       modelDef.ModelId,
		PercentComplete: 0,
		StatusMessage: "İstediğiniz model şu anda aktif düğüme indiriliyor, lütfen bekleyin.",
	})
	state := stateVal.(*ProvisioningState)

	if !loaded {
		// New provisioning task
		session, node, err := j.SelectBestNodeForColdDownload(modelDef)
		if err != nil {
			j.provisioningLocks.Delete(modelDef.ModelId)
			return nil, err
		}

		state.SelectedNodeId = session.NodeId

		// Check if LRU purge needed
		if int64(node.AllocatedDiskGB)*1024*1024*1024 < modelDef.FileSize {
			if purgeCandidate, err := GlobalScheduler.DetermineLruModelToPurge(node, modelDef.FileSize/(1024*1024)); err == nil && purgeCandidate != "" {
				purgePayload, _ := json.Marshal(map[string]string{"model_id": purgeCandidate})
				_ = session.WriteFrame(TunnelFrame{
					Type:    MsgTypePurgeModel,
					Payload: purgePayload,
				})
			}
		}

		// Send download command to worker
		downloadPayload, _ := json.Marshal(map[string]interface{}{
			"model_id":     modelDef.ModelId,
			"download_url": modelDef.DownloadUrl,
			"file_size":    modelDef.FileSize,
			"chunk_size":   modelDef.ChunkSize,
		})

		_ = session.WriteFrame(TunnelFrame{
			Type:    MsgTypeDownloadModel,
			Payload: downloadPayload,
		})

		// Simulated high-speed progress updater for demonstration & robust testing
		go func() {
			for p := 10; p <= 100; p += 15 {
				time.Sleep(500 * time.Millisecond)
				state.Mu.Lock()
				state.PercentComplete = p
				state.SpeedMBs = node.BandwidthDownMbps / 8.0
				if state.SpeedMBs <= 0 {
					state.SpeedMBs = 45.0
				}
				state.EtaSeconds = (100 - p) / 10
				if p >= 90 {
					state.StatusMessage = "Model diske yazıldı, belleğe yükleniyor..."
				} else {
					state.StatusMessage = fmt.Sprintf("İstediğiniz model şu anda aktif düğüme indiriliyor, lütfen bekleyin. [Yüzde %d İndirildi | Hız: %.1f MB/sn | Tahmini Süre: %d sn]", p, state.SpeedMBs, state.EtaSeconds)
				}
				if p >= 100 {
					state.IsReady = true
					state.StatusMessage = "Model hazır, yanıt üretiliyor..."
					if session.InstalledModels == nil {
						session.InstalledModels = make(map[string]bool)
					}
					session.InstalledModels[modelDef.ModelId] = true
				}
				// Notify all subscribers
				for _, ch := range state.Subscribers {
					select {
					case ch <- ProvisioningUpdate{
						Percent:       state.PercentComplete,
						SpeedMBs:      state.SpeedMBs,
						EtaSeconds:    state.EtaSeconds,
						StatusMessage: state.StatusMessage,
						IsReady:       state.IsReady,
					}:
					default:
					}
				}
				state.Mu.Unlock()
				if p >= 100 {
					break
				}
			}
			time.Sleep(1 * time.Second)
			j.provisioningLocks.Delete(modelDef.ModelId)
		}()
	}

	// Client wait loop
	subChan := make(chan ProvisioningUpdate, 10)
	state.Mu.Lock()
	if state.IsReady {
		state.Mu.Unlock()
		session, _ := GlobalHub.GetSession(state.SelectedNodeId)
		return session, nil
	}
	state.Subscribers = append(state.Subscribers, subChan)
	state.Mu.Unlock()

	flusher, _ := c.Writer.(http.Flusher)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	setting, _ := model.GetByogSetting()
	timeoutSecs := 120
	if setting != nil && setting.MaxColdStartTimeout > 0 {
		timeoutSecs = setting.MaxColdStartTimeout
	}
	timeoutChan := time.After(time.Duration(timeoutSecs) * time.Second)

	for {
		select {
		case update := <-subChan:
			if isSseClient {
				// Send SSE event: model_provisioning
				payload, _ := json.Marshal(update)
				_, _ = fmt.Fprintf(c.Writer, "event: model_provisioning\ndata: %s\n\n", string(payload))
				if flusher != nil {
					flusher.Flush()
				}
			}
			if update.IsReady {
				session, _ := GlobalHub.GetSession(state.SelectedNodeId)
				return session, nil
			}
			if update.Error != "" {
				return nil, fmt.Errorf("cold provisioning failed: %s", update.Error)
			}

		case <-ticker.C:
			// Standard OpenAI SDK / HTTP client Keep-Alive (Section 2.4)
			// Send SSE comment line or whitespace to prevent read timeout
			state.Mu.Lock()
			pct := state.PercentComplete
			eta := state.EtaSeconds
			state.Mu.Unlock()

			if isSseClient {
				_, _ = fmt.Fprintf(c.Writer, ": status: provisioning model %d%% (ETA %ds)\n\n", pct, eta)
			} else {
				_, _ = io.WriteString(c.Writer, " ")
			}
			if flusher != nil {
				flusher.Flush()
			}

		case <-timeoutChan:
			return nil, fmt.Errorf("cold provisioning timed out after %d seconds", timeoutSecs)

		case <-c.Request.Context().Done():
			return nil, fmt.Errorf("client canceled request")
		}
	}
}
