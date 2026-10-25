package controller

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	byogservice "github.com/QuantumNous/new-api/service/byog"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestByogEndToEndLifecycle tests the entire BYOG lifecycle end-to-end:
// 1. Database setup & global enabling
// 2. User pairing code generation
// 3. Worker registration with hardware specs & pairing
// 4. WebSocket reverse tunnel connection & keepalive
// 5. Worker telemetry & model download progress reporting
// 6. Worker model ready state transition
// 7. Relay / Playground non-streaming chat completion via tunnel
// 8. Relay / Playground streaming SSE chat completion via tunnel
// 9. Offline / missing model fallback handling (503 Service Unavailable)
func TestByogEndToEndLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	common.RedisEnabled = false

	// 1. Setup isolated in-memory SQLite database
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "byog-e2e.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	originalDB := model.DB
	model.DB = db
	model.LOG_DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalDB
		_ = sqlDB.Close()
	})

	require.NoError(t, db.AutoMigrate(
		&model.ByogNode{},
		&model.ByogModel{},
		&model.ByogSetting{},
		&model.ByogNodeBan{},
		&model.ByogAssignmentRule{},
		&model.Option{},
		&model.User{},
		&model.Token{},
		&model.Log{},
	))

	// Create test user and token
	testUser := &model.User{
		Username: "byog_tester",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
		Quota:    1000000,
	}
	require.NoError(t, db.Create(testUser).Error)

	// Configure BYOG Settings (Enabled globally)
	require.NoError(t, db.Create(&model.ByogSetting{
		Enabled:            true,
		Tier1ProviderPrice: 0.15,
		Tier2ProviderPrice: 0.30,
		Tier3ProviderPrice: 0.60,
	}).Error)

	// Create a test BYOG model definition
	testModelId := "byog-qwen-coder-e2e"
	require.NoError(t, db.Create(&model.ByogModel{
		ModelId:     testModelId,
		Description: "Qwen 2.5 Coder 7B (BYOG)",
		DownloadUrl: "http://127.0.0.1:3000/models/qwen.gguf",
		IsEnabled:   true,
		PricingTier: 1,
		FileSize:    4 * 1024 * 1024 * 1024,
	}).Error)

	// 2. Setup Gin Router with BYOG worker and tunnel endpoints
	router := gin.New()
	router.POST("/api/v1/worker/register", RegisterByogWorker)
	router.GET("/api/v1/worker/tunnel", ByogWorkerTunnel)

	// Relay endpoint with TryHandleByogRelay
	router.POST("/v1/chat/completions", func(c *gin.Context) {
		c.Set("id", testUser.Id)
		c.Set("token_id", 1)
		c.Set("token_name", "test-token")
		c.Set(string(constant.ContextKeyUsingGroup), "default")

		// Extract model from body
		bodyBytes, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "read body failed"})
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

		var req struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(bodyBytes, &req)

		// Call BYOG relay handler
		handled := byogservice.TryHandleByogRelay(c, req.Model)
		if !handled {
			c.JSON(http.StatusNotFound, gin.H{"error": "model not found in BYOG"})
		}
	})

	// Start test HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	// 3. User generates pairing code
	userPairingSessionsMu.Lock()
	testPairingCode := "BYOG-TEST-PAIR-9999"
	cleanCode := strings.ReplaceAll(testPairingCode, "-", "")
	userPairingSessions[testPairingCode] = userPairingSession{
		UserId:    testUser.Id,
		CreatedAt: time.Now().Unix(),
	}
	userPairingSessions[cleanCode] = userPairingSession{
		UserId:    testUser.Id,
		CreatedAt: time.Now().Unix(),
	}
	userPairingSessionsMu.Unlock()

	// 4. Worker Registers with Server
	testHwid := "hwid-test-desktop-win-001"
	regPayload := map[string]interface{}{
		"hwid":         testHwid,
		"device_name":  "RTX-4090-Node-1",
		"device_type":  "desktop",
		"os":           "windows",
		"pairing_code": testPairingCode,
		"cpu_model":    "AMD Ryzen 9 7950X",
		"gpu_model":    "NVIDIA GeForce RTX 4090",
		"vram_gb":      24,
		"ram_gb":       64,
		"disk_free_gb": 1000,
	}
	regBytes, _ := json.Marshal(regPayload)

	resp, err := http.Post(server.URL+"/api/v1/worker/register", "application/json", bytes.NewReader(regBytes))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var regResult struct {
		Success bool `json:"success"`
		Data    struct {
			NodeId   string `json:"node_id"`
			IsPaired bool   `json:"is_paired"`
			UserId   int    `json:"user_id"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&regResult))
	require.True(t, regResult.Success)
	require.NotEmpty(t, regResult.Data.NodeId)
	require.True(t, regResult.Data.IsPaired)
	require.Equal(t, testUser.Id, regResult.Data.UserId)
	nodeId := regResult.Data.NodeId

	// Verify node in DB
	dbNode, err := model.GetByogNodeByNodeId(nodeId)
	require.NoError(t, err)
	require.NotNil(t, dbNode)
	require.Equal(t, testUser.Id, dbNode.UserId)
	require.Equal(t, "RTX-4090-Node-1", dbNode.DeviceName)

	// 5. Worker connects to WebSocket Tunnel
	wsUrl := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/worker/tunnel?node_id=" + nodeId
	wsConn, wsResp, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	require.NoError(t, err)
	defer wsConn.Close()
	require.Equal(t, http.StatusSwitchingProtocols, wsResp.StatusCode)

	// Verify session registered in GlobalHub
	session, ok := byogservice.GlobalHub.GetSession(nodeId)
	require.True(t, ok)
	require.NotNil(t, session)

	// Worker message handling loop in background
	workerDone := make(chan struct{})
	var workerMu sync.Mutex
	workerReceivedInferences := 0

	go func() {
		defer close(workerDone)
		for {
			var frame byogservice.TunnelFrame
			if err := wsConn.ReadJSON(&frame); err != nil {
				return
			}

			switch frame.Type {
			case "ping":
				_ = wsConn.WriteJSON(map[string]string{"type": "pong"})

			case byogservice.MsgTypeInferenceReq:
				workerMu.Lock()
				workerReceivedInferences++
				workerMu.Unlock()

				var infReq byogservice.InferencePayload
				_ = json.Unmarshal(frame.Payload, &infReq)

				if infReq.Stream {
					// Stream 3 chunks
					chunks := []string{"Merhaba ", "dunya, ", "BYOG calisiyor!"}
					for i, ch := range chunks {
						chunkPayload, _ := json.Marshal(map[string]interface{}{
							"request_id": infReq.RequestId,
							"content":    ch,
							"token_id":   100 + i,
							"is_final":   i == len(chunks)-1,
						})
						_ = wsConn.WriteJSON(map[string]interface{}{
							"type":    byogservice.MsgTypeInferenceChunk,
							"id":      infReq.RequestId,
							"payload": json.RawMessage(chunkPayload),
						})
						time.Sleep(10 * time.Millisecond)
					}
				} else {
					// Non-streaming chunk
					chunkPayload, _ := json.Marshal(map[string]interface{}{
						"request_id": infReq.RequestId,
						"content":    "Merhaba dunya, BYOG calisiyor!",
						"token_id":   100,
						"is_final":   true,
					})
					_ = wsConn.WriteJSON(map[string]interface{}{
						"type":    byogservice.MsgTypeInferenceChunk,
						"id":      infReq.RequestId,
						"payload": json.RawMessage(chunkPayload),
					})
				}

				// Send done frame
				donePayload, _ := json.Marshal(map[string]interface{}{
					"request_id":    infReq.RequestId,
					"total_tokens":  6,
					"prompt_tokens": 4,
					"ttft_ms":       35,
					"tps":           42.5,
				})
				_ = wsConn.WriteJSON(map[string]interface{}{
					"type":    byogservice.MsgTypeInferenceDone,
					"id":      infReq.RequestId,
					"payload": json.RawMessage(donePayload),
				})
			}
		}
	}()

	// 6. Test Heartbeat & Model Download Progress Reporting
	heartbeatDownload, _ := json.Marshal(map[string]interface{}{
		"model_status":      "downloading",
		"download_status":   "downloading",
		"downloaded_mb":     1200,
		"total_mb":          4000,
		"download_progress": 30,
		"current_tps":       0.0,
		"installed_models":  []string{},
	})
	require.NoError(t, wsConn.WriteJSON(byogservice.TunnelFrame{
		Type:    byogservice.MsgTypeHeartbeat,
		Payload: heartbeatDownload,
	}))

	time.Sleep(50 * time.Millisecond)
	dbNode, err = model.GetByogNodeByNodeId(nodeId)
	require.NoError(t, err)
	assert.Equal(t, "downloading", dbNode.ModelStatus)
	assert.Equal(t, 1200, dbNode.DownloadedMB)
	assert.Equal(t, 4000, dbNode.TotalMB)
	assert.Equal(t, 30, dbNode.DownloadProgress)

	// 7. Transition Model to Ready State
	heartbeatReady, _ := json.Marshal(map[string]interface{}{
		"model_status":      "ready",
		"download_status":   "completed",
		"downloaded_mb":     4000,
		"total_mb":          4000,
		"download_progress": 100,
		"current_tps":       0.0,
		"installed_models":  []string{testModelId},
	})
	require.NoError(t, wsConn.WriteJSON(byogservice.TunnelFrame{
		Type:    byogservice.MsgTypeHeartbeat,
		Payload: heartbeatReady,
	}))

	time.Sleep(50 * time.Millisecond)
	dbNode, err = model.GetByogNodeByNodeId(nodeId)
	require.NoError(t, err)
	assert.Equal(t, "ready", dbNode.ModelStatus)
	assert.Contains(t, dbNode.InstalledModels, testModelId)

	// Verify session has installed model marked
	assert.True(t, session.InstalledModels[testModelId])

	// 8. Test Non-Streaming Chat Completion (Playground / API format)
	chatReqNonStream := map[string]interface{}{
		"model": testModelId,
		"messages": []map[string]string{
			{"role": "user", "content": "Selam BYOG!"},
		},
		"stream":      false,
		"temperature": 0.7,
		"max_tokens":  256,
	}
	chatBytes, _ := json.Marshal(chatReqNonStream)

	chatResp, err := http.Post(server.URL+"/v1/chat/completions", "application/json", bytes.NewReader(chatBytes))
	require.NoError(t, err)
	defer chatResp.Body.Close()
	require.Equal(t, http.StatusOK, chatResp.StatusCode)

	var completionResult struct {
		Id      string `json:"id"`
		Object  string `json:"object"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	require.NoError(t, json.NewDecoder(chatResp.Body).Decode(&completionResult))
	assert.Equal(t, "chat.completion", completionResult.Object)
	assert.Equal(t, testModelId, completionResult.Model)
	require.NotEmpty(t, completionResult.Choices)
	assert.Equal(t, "assistant", completionResult.Choices[0].Message.Role)
	assert.Equal(t, "Merhaba dunya, BYOG calisiyor!", completionResult.Choices[0].Message.Content)
	assert.Equal(t, "stop", completionResult.Choices[0].FinishReason)
	assert.Equal(t, 4, completionResult.Usage.PromptTokens)
	assert.Equal(t, 1, completionResult.Usage.CompletionTokens)

	// 9. Test Streaming SSE Chat Completion (Playground / Web Stream format)
	chatReqStream := map[string]interface{}{
		"model": testModelId,
		"messages": []map[string]string{
			{"role": "user", "content": "Bize bir siir yaz"},
		},
		"stream": true,
	}
	chatStreamBytes, _ := json.Marshal(chatReqStream)

	streamResp, err := http.Post(server.URL+"/v1/chat/completions", "application/json", bytes.NewReader(chatStreamBytes))
	require.NoError(t, err)
	defer streamResp.Body.Close()
	require.Equal(t, http.StatusOK, streamResp.StatusCode)
	assert.Contains(t, streamResp.Header.Get("Content-Type"), "text/event-stream")

	scanner := bufio.NewScanner(streamResp.Body)
	var fullStreamedText strings.Builder
	receivedDone := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				receivedDone = true
				break
			}
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil && len(chunk.Choices) > 0 {
				fullStreamedText.WriteString(chunk.Choices[0].Delta.Content)
			}
		}
	}
	assert.True(t, receivedDone, "Stream must terminate with [DONE]")
	assert.Equal(t, "Merhaba dunya, BYOG calisiyor!", fullStreamedText.String())

	// 10. Test Offline Node Fallback (Requesting a model with no online worker)
	offlineReq := map[string]interface{}{
		"model": "non-existent-byog-model",
		"messages": []map[string]string{
			{"role": "user", "content": "Test offline"},
		},
	}
	offlineBytes, _ := json.Marshal(offlineReq)
	offlineResp, err := http.Post(server.URL+"/v1/chat/completions", "application/json", bytes.NewReader(offlineBytes))
	require.NoError(t, err)
	defer offlineResp.Body.Close()
	// Should return 404 (handled=false falls to router 404)
	assert.Equal(t, http.StatusNotFound, offlineResp.StatusCode)

	// Clean disconnect
	_ = wsConn.Close()
	select {
	case <-workerDone:
	case <-time.After(1 * time.Second):
	}

	byogservice.GlobalHub.Unregister(nodeId)
	_, online := byogservice.GlobalHub.GetSession(nodeId)
	assert.False(t, online)
}
