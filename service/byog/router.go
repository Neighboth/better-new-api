package byog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type Router struct {
	stickySessions sync.Map // map[sessionHash]string (nodeId)
	nodeQueues     sync.Map // map[nodeId]*NodeQueue
}

type NodeQueue struct {
	ActiveCount int32
	MaxCapacity int32
	Lock        sync.Mutex
}

var GlobalRouter = &Router{}

// Get or create queue for a node
func (r *Router) getNodeQueue(nodeId string, deviceType string) *NodeQueue {
	maxCap := int32(4)
	if deviceType == "mobile" || deviceType == "tablet" {
		maxCap = 1 // Mobile strictly concurrency 1
	}
	val, _ := r.nodeQueues.LoadOrStore(nodeId, &NodeQueue{
		ActiveCount: 0,
		MaxCapacity: maxCap,
	})
	return val.(*NodeQueue)
}

// SelectNodeForRequest applies Smart Geo-Routing, Single-Node Rule & Sticky Sessions (Section 2.5)
func (r *Router) SelectNodeForRequest(modelId string, conversationId string, promptPrefix string, clientIp string) (*NodeSession, error) {
	// 1. Check if model exists in BYOG catalog
	modelDef, err := model.GetByogModelByModelId(modelId)
	if err != nil || modelDef == nil {
		return nil, fmt.Errorf("model %s not found in BYOG catalog", modelId)
	}

	onlineSessions := GlobalHub.GetAllOnlineNodes()
	if len(onlineSessions) == 0 {
		return nil, fmt.Errorf("no online BYOG compute nodes available")
	}

	// 2. Filter candidate nodes capable of running this model
	var capableSessions []*NodeSession
	for _, s := range onlineSessions {
		n, err := model.GetByogNodeByNodeId(s.NodeId)
		if err != nil || n == nil {
			continue
		}
		if n.DeviceType == "mobile" || n.DeviceType == "tablet" {
			if GlobalScheduler.IsModelSafeForMobile(n, modelId) {
				capableSessions = append(capableSessions, s)
			}
		} else {
			capableSessions = append(capableSessions, s)
		}
	}

	if len(capableSessions) == 0 {
		return nil, fmt.Errorf("no node with sufficient capacity to run %s", modelId)
	}

	// Prioritize nodes that already have the model downloaded/installed on disk or in VRAM!
	var installedSessions []*NodeSession
	for _, s := range capableSessions {
		if s.InstalledModels != nil && s.InstalledModels[modelId] {
			installedSessions = append(installedSessions, s)
		}
	}
	if len(installedSessions) > 0 {
		capableSessions = installedSessions
	}

	// 3. Single Node Rule (Section 2.5):
	// If only 1 capable node exists, route directly to it regardless of ping/location!
	if len(capableSessions) == 1 {
		return capableSessions[0], nil
	}

	// 4. Sticky Session & Prefix Caching Check (Section 2.5)
	sessionKey := conversationId
	if sessionKey == "" && len(promptPrefix) >= 64 {
		h := sha256.Sum256([]byte(promptPrefix[:64]))
		sessionKey = hex.EncodeToString(h[:])
	}

	if sessionKey != "" {
		if val, ok := r.stickySessions.Load(sessionKey); ok {
			targetNodeId := val.(string)
			for _, s := range capableSessions {
				if s.NodeId == targetNodeId && !s.IsPaused && !s.IsGamerMode {
					return s, nil
				}
			}
		}
	}

	// 5. Smart Geo-Routing & Latency Match
	// Pick node with lowest ping / highest bandwidth
	var bestSession *NodeSession
	minPing := 999999

	for _, s := range capableSessions {
		n, err := model.GetByogNodeByNodeId(s.NodeId)
		if err != nil || n == nil {
			continue
		}

		effectivePing := n.PingMs
		if effectivePing <= 0 {
			effectivePing = 50
		}

		if effectivePing < minPing {
			minPing = effectivePing
			bestSession = s
		}
	}

	if bestSession != nil && sessionKey != "" {
		r.stickySessions.Store(sessionKey, bestSession.NodeId)
	}

	if bestSession == nil {
		bestSession = capableSessions[0]
	}

	return bestSession, nil
}

// DispatchInference routes request through reverse tunnel, limits concurrency, and streams response
func (r *Router) DispatchInference(c *gin.Context, session *NodeSession, modelDef *model.ByogModel, payload InferencePayload) error {
	node, err := model.GetByogNodeByNodeId(session.NodeId)
	if err != nil || node == nil {
		return fmt.Errorf("node not found: %s", session.NodeId)
	}

	// Concurrency limiter check (Section 3.7) with queuing
	queue := r.getNodeQueue(session.NodeId, node.DeviceType)
	waitDeadline := time.Now().Add(120 * time.Second)
	acquired := false
	for time.Now().Before(waitDeadline) {
		queue.Lock.Lock()
		if queue.ActiveCount < queue.MaxCapacity {
			atomic.AddInt32(&queue.ActiveCount, 1)
			queue.Lock.Unlock()
			acquired = true
			break
		}
		queue.Lock.Unlock()
		select {
		case <-c.Request.Context().Done():
			return fmt.Errorf("client disconnected while queued")
		case <-time.After(200 * time.Millisecond):
		}
	}
	if !acquired {
		return fmt.Errorf("node concurrency capacity reached, worker busy")
	}

	defer func() {
		queue.Lock.Lock()
		atomic.AddInt32(&queue.ActiveCount, -1)
		queue.Lock.Unlock()
	}()

	// Setup response channels
	startTime := time.Now()
	reqId := payload.RequestId
	if reqId == "" {
		reqId = fmt.Sprintf("req_%d", time.Now().UnixNano())
		payload.RequestId = reqId
	}

	respChan := make(chan *TunnelFrame, 1024)
	session.PendingRequests.Store(reqId, respChan)
	defer session.PendingRequests.Delete(reqId)

	// Send inference request frame to worker
	rawPayload, _ := json.Marshal(payload)
	err = session.WriteFrame(TunnelFrame{
		Type:    MsgTypeInferenceReq,
		Id:      reqId,
		Payload: rawPayload,
	})
	if err != nil {
		return fmt.Errorf("failed to dispatch to worker tunnel: %v", err)
	}

	// Prepare HTTP response headers
	flusher, _ := c.Writer.(http.Flusher)
	if payload.Stream {
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")
	} else {
		c.Header("Content-Type", "application/json")
	}

	// The first token of a device covers the load of the model: a phone that
	// keeps the model on the disk needs minutes for the load, the wait for the
	// first token is therefore longer than the wait between two tokens of a
	// generation that already runs.
	firstTokenTimeout := 300 * time.Second
	interTokenTimeout := 90 * time.Second
	timeoutTicker := time.NewTicker(firstTokenTimeout)
	defer timeoutTicker.Stop()

	var totalTokensGenerated int
	var receivedFirstChunk bool
	var fullNonStreamContent strings.Builder

	// streamError writes an OpenAI style error into the answer of the client,
	// a stream receives it as an event, a plain answer as a JSON body.
	streamError := func(message string, errType string) {
		errBody := map[string]interface{}{
			"error": map[string]interface{}{
				"message": message,
				"type":    errType,
			},
		}
		if payload.Stream {
			errJson, _ := json.Marshal(errBody)
			_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", string(errJson))
			_, _ = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
			if flusher != nil {
				flusher.Flush()
			}
		} else if !c.Writer.Written() {
			c.JSON(http.StatusBadGateway, errBody)
		}
	}

	for {
		select {
		case frame, ok := <-respChan:
			if !ok {
				return fmt.Errorf("worker response channel closed before the request completed")
			}

			switch frame.Type {
			case MsgTypeInferenceChunk:
				receivedFirstChunk = true
				timeoutTicker.Reset(interTokenTimeout)
				var chunk ChunkPayload
				if err := json.Unmarshal(frame.Payload, &chunk); err == nil {
					// The final marker of the worker carries no content, it
					// closes the answer and it is not a generated token.
					if chunk.Content == "" {
						continue
					}
					totalTokensGenerated++
					if payload.Stream {
						// Stream OpenAI compatible chunk
						openAiChunk := map[string]interface{}{
							"id":      reqId,
							"object":  "chat.completion.chunk",
							"created": time.Now().Unix(),
							"model":   modelDef.ModelId,
							"choices": []map[string]interface{}{
								{
									"index": 0,
									"delta": map[string]string{
										"content": chunk.Content,
									},
									"finish_reason": nil,
								},
							},
						}
						data, _ := json.Marshal(openAiChunk)
						_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", string(data))
						if flusher != nil {
							flusher.Flush()
						}
					} else {
						fullNonStreamContent.WriteString(chunk.Content)
					}
				}

			case MsgTypeTunnelClosed:
				streamError("The connection of the BYOG worker closed while the request was running. Please retry in a moment.", "byog_worker_disconnected")
				return fmt.Errorf("worker tunnel closed while request %s was running", reqId)

			case MsgTypeInferenceDone:
				var done DonePayload
				_ = json.Unmarshal(frame.Payload, &done)

				// The done frame reports total_tokens as the sum of the prompt
				// and the generation of the worker, so it must not replace the
				// count of the generated chunks here: completion_tokens of the
				// usage block is the number of the generated tokens and the
				// quota math below adds done.PromptTokens on top of it.
				if done.Error != "" && totalTokensGenerated == 0 {
					streamError(done.Error, "byog_worker_error")
					return fmt.Errorf("worker reported error: %s", done.Error)
				}

				// Calculate multipliers & credit tokens
				multiplier, _ := model.CalculateNodeMultipliers(node)
				tokensToCredit := int64(float64(totalTokensGenerated) * multiplier)

				userId := c.GetInt("id")
				isOwner := userId > 0 && userId == node.UserId
				clientIp := c.ClientIP()
				isSameIp := clientIp != "" && (clientIp == node.IpAddress || clientIp == "127.0.0.1" || clientIp == "::1")

				// Item 10: Anti-Manipulation / Alt-Account Self-Farming Block:
				// If the caller is the owner of the node, or querying from the exact same IP/network,
				// they cannot earn mining tokens from self-queries!
				if isOwner || isSameIp {
					tokensToCredit = 0
				}

				// Credit tokens to compute provider node
				if tokensToCredit > 0 {
					_ = model.DB.Model(&model.ByogNode{}).
						Where("node_id = ?", session.NodeId).
						Updates(map[string]interface{}{
							"total_tokens_processed": gormExpr("total_tokens_processed + ?", totalTokensGenerated),
							"total_earned_credits":   gormExpr("total_earned_credits + ?", float64(tokensToCredit)*0.000001),
						})
				} else {
					_ = model.DB.Model(&model.ByogNode{}).
						Where("node_id = ?", session.NodeId).
						Update("total_tokens_processed", gormExpr("total_tokens_processed + ?", totalTokensGenerated))
				}

				// Charge customer:
				// Item 18: If the user owns this node, access to their own node's models is 100% FREE!
				if isOwner {
					// Free API usage for owner on their own device!
				} else if userId > 0 {
					setting, _ := model.GetByogSetting()
					providerPricePer1M := 0.20
					if setting != nil {
						switch modelDef.PricingTier {
						case 1:
							providerPricePer1M = setting.Tier1ProviderPrice
						case 3:
							providerPricePer1M = setting.Tier3ProviderPrice
						default:
							providerPricePer1M = setting.Tier2ProviderPrice
						}
					}

					totalTokens := done.PromptTokens + totalTokensGenerated
					if totalTokens <= 0 {
						totalTokens = totalTokensGenerated
					}

					var quotaToDeduct int64 = 0
					ratioOrPrice, usePrice, exist := ratio_setting.GetModelRatioOrPrice(modelDef.ModelId)
					if exist {
						if usePrice {
							quotaToDeduct = int64(ratioOrPrice * float64(common.QuotaPerUnit))
						} else {
							quotaToDeduct = int64(float64(totalTokens) * ratioOrPrice * 2.0 / 1000.0 * float64(common.QuotaPerUnit) / 500.0)
						}
					} else {
						// 3x provider cost price (providerPricePer1M is USD per 1M tokens)
						salePricePer1M := 3.0 * providerPricePer1M
						quotaPerToken := (salePricePer1M * float64(common.QuotaPerUnit)) / 1000000.0
						quotaToDeduct = int64(float64(totalTokens) * quotaPerToken)
					}

					if quotaToDeduct > 0 {
						if err := model.DecreaseUserQuota(userId, int(quotaToDeduct), true); err != nil {
							c.Set(common.WalletTransferHoldRetainContextKey, true)
							common.SysError("BYOG wallet settlement failed: " + err.Error())
						}
					}

					tokenId := c.GetInt("token_id")
					tokenName := c.GetString("token_name")
					usingGroup := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
					if usingGroup == "" {
						usingGroup = "default"
					}
					useTime := time.Since(startTime).Seconds()
					model.RecordConsumeLog(c, userId, model.RecordConsumeLogParams{
						ChannelId:        0,
						PromptTokens:     done.PromptTokens,
						CompletionTokens: totalTokensGenerated,
						ModelName:        modelDef.ModelId,
						TokenName:        tokenName,
						Quota:            int(quotaToDeduct),
						Content:          fmt.Sprintf("BYOG Yerel Model: %s (%s)", modelDef.ModelId, node.DeviceName),
						TokenId:          tokenId,
						UseTimeSeconds:   int(useTime),
						IsStream:         payload.Stream,
						Group:            usingGroup,
						Other: map[string]interface{}{
							"node_id":     node.NodeId,
							"device_name": node.DeviceName,
							"device_type": node.DeviceType,
							"country":     node.Country,
							"byog":        true,
						},
					})
				}

				if payload.Stream {
					_, _ = fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
					if flusher != nil {
						flusher.Flush()
					}
				} else {
					// Output full non-streaming OpenAI response
					openAiResp := map[string]interface{}{
						"id":      reqId,
						"object":  "chat.completion",
						"created": time.Now().Unix(),
						"model":   modelDef.ModelId,
						"choices": []map[string]interface{}{
							{
								"index": 0,
								"message": map[string]string{
									"role":    "assistant",
									"content": fullNonStreamContent.String(),
								},
								"finish_reason": "stop",
							},
						},
						"usage": map[string]interface{}{
							"prompt_tokens":     done.PromptTokens,
							"completion_tokens": totalTokensGenerated,
							"total_tokens":      done.PromptTokens + totalTokensGenerated,
						},
					}
					c.JSON(http.StatusOK, openAiResp)
				}
				return nil

			case MsgTypeInferenceError:
				// The payload of the error is a JSON object, the message of the
				// worker is the field "error" of that object.
				errMsg := ""
				var errPayload struct {
					Error string `json:"error"`
				}
				if json.Unmarshal(frame.Payload, &errPayload) == nil && errPayload.Error != "" {
					errMsg = errPayload.Error
				} else {
					errMsg = strings.TrimSpace(string(frame.Payload))
				}
				if errMsg == "" {
					errMsg = "unknown worker error"
				}
				streamError(fmt.Sprintf("Worker reported error: %s", errMsg), "byog_worker_error")
				return fmt.Errorf("worker reported error: %s", errMsg)
			}

		case <-timeoutTicker.C:
			if !receivedFirstChunk {
				streamError(fmt.Sprintf("The BYOG worker did not produce the first token within %d seconds. The device may still be loading the model or is overloaded.", int(firstTokenTimeout.Seconds())), "byog_timeout")
				return fmt.Errorf("worker inference timed out waiting for the first token")
			}
			streamError(fmt.Sprintf("The BYOG worker stopped producing tokens for %d seconds.", int(interTokenTimeout.Seconds())), "byog_timeout")
			return fmt.Errorf("worker inference timed out between tokens")

		case <-c.Request.Context().Done():
			return fmt.Errorf("client disconnected")
		}
	}
}
