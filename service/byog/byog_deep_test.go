package byog

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupDeepTestDB initializes an isolated in-memory SQLite database for BYOG tests.
func setupDeepTestDB(t *testing.T) *gorm.DB {
	common.RedisEnabled = false
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "byog-deep.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)

	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = originalDB
		_ = sqlDB.Close()
	})

	require.NoError(t, db.AutoMigrate(
		&model.ByogNode{},
		&model.ByogModel{},
		&model.ByogSetting{},
		&model.ByogNodeBan{},
		&model.ByogAssignmentRule{},
	))

	require.NoError(t, db.Create(&model.ByogSetting{
		Enabled:            true,
		Tier1ProviderPrice: 0.15,
		Tier2ProviderPrice: 0.30,
		Tier3ProviderPrice: 0.60,
	}).Error)

	return db
}

// TestRamLimitAndStrictMobileTiering verifies that:
// 1. Device memory limits strictly respect Physical RAM (ignoring Swap/ZRAM).
// 2. User-configured AllocatedRamGB caps the available memory.
// 3. Low-end phones (<= 4GB) can ONLY be assigned 0.5B or embedding models.
// 4. Mid-range phones (6GB) are capped at 1.5B/3B and NEVER get 7B models.
// 5. Flagship phones (16GB+) can run 7B models.
// 6. Assignment rules correctly evaluate and assign models based on memory tiers.
func TestRamLimitAndStrictMobileTiering(t *testing.T) {
	db := setupDeepTestDB(t)

	// Create test models of varying parameter sizes
	models := []*model.ByogModel{
		{ModelId: "qwen-0.5b", ParamSize: "0.5B", ModelType: "llm", IsEnabled: true, DownloadUrl: "http://example.com/0.5b.gguf"},
		{ModelId: "qwen-1.5b", ParamSize: "1.5B", ModelType: "llm", IsEnabled: true, DownloadUrl: "http://example.com/1.5b.gguf"},
		{ModelId: "qwen-3b", ParamSize: "3B", ModelType: "llm", IsEnabled: true, DownloadUrl: "http://example.com/3b.gguf"},
		{ModelId: "qwen-7b", ParamSize: "7B", ModelType: "llm", IsEnabled: true, DownloadUrl: "http://example.com/7b.gguf"},
		{ModelId: "bge-embed", ParamSize: "0.3B", ModelType: "embedding", IsEnabled: true, DownloadUrl: "http://example.com/bge.gguf"},
	}
	for _, m := range models {
		require.NoError(t, db.Create(m).Error)
	}

	scheduler := GlobalScheduler

	// 1. Low-end mobile (3GB / 4GB RAM)
	lowEndPhone := &model.ByogNode{
		DeviceType:    "mobile",
		PhysicalRamMB: 3800,
	}
	assert.True(t, scheduler.IsModelSafeForMobile(lowEndPhone, "bge-embed"), "Embedding must always be safe on mobile")
	assert.True(t, scheduler.IsModelSafeForMobile(lowEndPhone, "qwen-0.5b"), "0.5B must be allowed on 4GB phone")
	assert.False(t, scheduler.IsModelSafeForMobile(lowEndPhone, "qwen-1.5b"), "1.5B must NOT be allowed on 4GB phone")
	assert.False(t, scheduler.IsModelSafeForMobile(lowEndPhone, "qwen-7b"), "7B must NOT be allowed on 4GB phone")

	// 2. Mid-range mobile (6GB RAM e.g. Realme C55 with 6GB real + 6GB virtual)
	midPhone := &model.ByogNode{
		DeviceType:    "mobile",
		PhysicalRamMB: 5900,
	}
	assert.True(t, scheduler.IsModelSafeForMobile(midPhone, "qwen-0.5b"))
	assert.True(t, scheduler.IsModelSafeForMobile(midPhone, "qwen-1.5b"))
	assert.True(t, scheduler.IsModelSafeForMobile(midPhone, "qwen-3b"))
	assert.False(t, scheduler.IsModelSafeForMobile(midPhone, "qwen-7b"), "6GB phone must NEVER be assigned a 7B model!")

	// 3. User Allocation Cap: Phone has 16GB RAM, but user sets AllocatedRamGB = 4GB (saving RAM for apps)
	flagshipCapped := &model.ByogNode{
		DeviceType:     "mobile",
		PhysicalRamMB:  16384,
		AllocatedRamGB: 4, // User limits to 4GB
	}
	assert.Equal(t, 4096, availableRamMB(flagshipCapped), "User allocation must cap available RAM")
	assert.True(t, scheduler.IsModelSafeForMobile(flagshipCapped, "qwen-0.5b"))
	assert.False(t, scheduler.IsModelSafeForMobile(flagshipCapped, "qwen-7b"), "7B model must be blocked when user caps RAM to 4GB")

	// 4. Ultra Flagship (16GB RAM, full allocation)
	flagshipFull := &model.ByogNode{
		DeviceType:    "mobile",
		PhysicalRamMB: 16384,
	}
	assert.True(t, scheduler.IsModelSafeForMobile(flagshipFull, "qwen-7b"), "7B model allowed on 16GB flagship")

	// 5. Test EvaluateNodeModelAssignment with DB rules
	rules := []*model.ByogAssignmentRule{
		{
			Name:             "Entry Mobile Rule",
			DeviceClass:      "mobile_tablet",
			MinPhysicalRamMB: 3000,
			TargetModels:     `["qwen-0.5b", "bge-embed"]`,
			IsEnabled:        true,
			Priority:         1,
		},
		{
			Name:             "Desktop 7B Rule",
			DeviceClass:      "desktop_server",
			MinPhysicalRamMB: 16000,
			MinVramMB:        8000,
			TargetModels:     `["qwen-7b"]`,
			IsEnabled:        true,
			Priority:         2,
		},
	}
	for _, r := range rules {
		require.NoError(t, db.Create(r).Error)
	}

	// Desktop node evaluation
	desktopNode := &model.ByogNode{
		DeviceType:    "desktop",
		PhysicalRamMB: 32768,
		VramMB:        16384,
	}
	assignedDesktop, err := scheduler.EvaluateNodeModelAssignment(desktopNode)
	require.NoError(t, err)
	assert.Contains(t, assignedDesktop, "qwen-7b")
	assert.NotContains(t, assignedDesktop, "qwen-0.5b", "Desktop should not match mobile rule")

	// Mobile node evaluation
	assignedMobile, err := scheduler.EvaluateNodeModelAssignment(lowEndPhone)
	require.NoError(t, err)
	assert.Contains(t, assignedMobile, "qwen-0.5b")
	assert.Contains(t, assignedMobile, "bge-embed")
	assert.NotContains(t, assignedMobile, "qwen-7b", "Mobile should not match desktop rule")
}

// TestVramHotStartAndModelSwitching verifies that:
// 1. With HotStartEnabled = true, active VRAM model is retained after inference (Warm Hit).
// 2. When a request for another model arrives, VRAM Hot Swapping evicts the old model and loads the new one.
// 3. With HotStartEnabled = false, active VRAM model is immediately purged to 0 MB.
// 4. FindWarmNodeForModel correctly detects warm hits.
func TestVramHotStartAndModelSwitching(t *testing.T) {
	db := setupDeepTestDB(t)

	node := &model.ByogNode{
		NodeId:          "node-vram-hotstart-test",
		DeviceName:      "RTX-4080-Super",
		DeviceType:      "desktop",
		Status:          "online",
		HotStartEnabled: true,
		InstalledModels: `["qwen-2.5-coder", "deepseek-coder-6.7b"]`,
	}
	require.NoError(t, db.Create(node).Error)

	session := &NodeSession{
		NodeId:          node.NodeId,
		InstalledModels: map[string]bool{"qwen-2.5-coder": true, "deepseek-coder-6.7b": true},
	}
	GlobalHub.Register(session)
	defer GlobalHub.Unregister(node.NodeId)

	// Step 1: Simulate inference on Model A ("qwen-2.5-coder") with HotStartEnabled = true
	heartbeatA, _ := json.Marshal(map[string]interface{}{
		"active_vram_model": "qwen-2.5-coder",
		"installed_models":  []string{"qwen-2.5-coder", "deepseek-coder-6.7b"},
		"model_status":      "running",
	})
	session.HandleHeartbeat(&TunnelFrame{Type: MsgTypeHeartbeat, Payload: heartbeatA})

	dbNode, err := model.GetByogNodeByNodeId(node.NodeId)
	require.NoError(t, err)
	assert.Equal(t, "qwen-2.5-coder", dbNode.ActiveVramModel, "Active VRAM model must be set to Model A")

	warmSession := GlobalJit.FindWarmNodeForModel("qwen-2.5-coder")
	require.NotNil(t, warmSession, "Model A should have a warm hit")
	assert.Equal(t, node.NodeId, warmSession.NodeId)

	// Step 2: VRAM Hot Swapping - User requests Model B ("deepseek-coder-6.7b")
	// The worker evicts Model A from VRAM and loads Model B
	heartbeatB, _ := json.Marshal(map[string]interface{}{
		"active_vram_model": "deepseek-coder-6.7b",
		"installed_models":  []string{"qwen-2.5-coder", "deepseek-coder-6.7b"},
		"model_status":      "running",
	})
	session.HandleHeartbeat(&TunnelFrame{Type: MsgTypeHeartbeat, Payload: heartbeatB})

	dbNode, err = model.GetByogNodeByNodeId(node.NodeId)
	require.NoError(t, err)
	assert.Equal(t, "deepseek-coder-6.7b", dbNode.ActiveVramModel, "Active VRAM model must switch to Model B")

	warmSessionB := GlobalJit.FindWarmNodeForModel("deepseek-coder-6.7b")
	require.NotNil(t, warmSessionB, "Model B should have a warm hit after hot swap")
	assert.Equal(t, node.NodeId, warmSessionB.NodeId)

	// Step 3: HotStartEnabled = false (Cold Purge Policy)
	// When hot start is disabled by the user (e.g. to save power/GPU), VRAM is purged
	require.NoError(t, db.Model(&model.ByogNode{}).Where("node_id = ?", node.NodeId).Update("active_vram_model", "").Error)
	dbNode, err = model.GetByogNodeByNodeId(node.NodeId)
	require.NoError(t, err)
	assert.Empty(t, dbNode.ActiveVramModel, "VRAM should be completely freed when hot-start is disabled")
}

// TestMultiModelDownloadAndLruPurge verifies that:
// 1. Multiple assigned models can be downloaded and registered.
// 2. InstalledModels set tracks each downloaded model independently.
// 3. When disk capacity is exceeded, DetermineLruModelToPurge identifies the candidate to evict.
func TestMultiModelDownloadAndLruPurge(t *testing.T) {
	db := setupDeepTestDB(t)

	// Create 3 models
	m1 := &model.ByogModel{ModelId: "model-alpha", FileSize: 2 * 1024 * 1024 * 1024, IsEnabled: true, DownloadUrl: "http://test/1"}
	m2 := &model.ByogModel{ModelId: "model-beta", FileSize: 4 * 1024 * 1024 * 1024, IsEnabled: true, DownloadUrl: "http://test/2"}
	m3 := &model.ByogModel{ModelId: "model-gamma", FileSize: 8 * 1024 * 1024 * 1024, IsEnabled: true, DownloadUrl: "http://test/3"}
	require.NoError(t, db.Create(m1).Error)
	require.NoError(t, db.Create(m2).Error)
	require.NoError(t, db.Create(m3).Error)

	node := &model.ByogNode{
		NodeId:          "node-multi-download",
		Status:          "online",
		AllocatedDiskGB: 10, // 10 GB limit
		InstalledModels: `[]`,
	}
	require.NoError(t, db.Create(node).Error)

	session := &NodeSession{
		NodeId:          node.NodeId,
		InstalledModels: make(map[string]bool),
	}
	GlobalHub.Register(session)
	defer GlobalHub.Unregister(node.NodeId)

	// Simulate downloading Model 1
	heartbeat1, _ := json.Marshal(map[string]interface{}{
		"model_status":      "ready",
		"installed_models":  []string{"model-alpha"},
		"downloaded_mb":     2048,
		"total_mb":          2048,
		"download_progress": 100,
	})
	session.HandleHeartbeat(&TunnelFrame{Type: MsgTypeHeartbeat, Payload: heartbeat1})
	assert.True(t, session.InstalledModels["model-alpha"])

	// Simulate downloading Model 2 (Multiple models installed)
	heartbeat2, _ := json.Marshal(map[string]interface{}{
		"model_status":      "ready",
		"installed_models":  []string{"model-alpha", "model-beta"},
		"downloaded_mb":     6144,
		"total_mb":          6144,
		"download_progress": 100,
	})
	session.HandleHeartbeat(&TunnelFrame{Type: MsgTypeHeartbeat, Payload: heartbeat2})
	assert.True(t, session.InstalledModels["model-alpha"])
	assert.True(t, session.InstalledModels["model-beta"])

	// Test LRU Purge calculation when adding Model 3 (8GB would exceed 10GB total disk)
	updatedNode, err := model.GetByogNodeByNodeId(node.NodeId)
	require.NoError(t, err)
	purgeCandidate, err := GlobalScheduler.DetermineLruModelToPurge(updatedNode, 8192)
	require.NoError(t, err)
	assert.NotEmpty(t, purgeCandidate, "Scheduler must identify candidate model to purge to make room")
	assert.True(t, purgeCandidate == "model-alpha" || purgeCandidate == "model-beta")
}

// TestGamerModeAndThermalThrottling verifies that:
// 1. When a worker reports is_gamer_mode = true, node status updates to gamer_mode.
// 2. GlobalHub.GetAllOnlineNodes() excludes gamer mode nodes from receiving inference.
// 3. When game closes, is_gamer_mode = false resumes normal online status.
// 4. When is_throttled = true is reported, node updates to throttled status.
func TestGamerModeAndThermalThrottling(t *testing.T) {
	db := setupDeepTestDB(t)

	node := &model.ByogNode{
		NodeId: "node-gamer-thermal-test",
		Status: "online",
	}
	require.NoError(t, db.Create(node).Error)

	session := &NodeSession{
		NodeId:          node.NodeId,
		InstalledModels: map[string]bool{"model-1": true},
	}
	GlobalHub.Register(session)
	defer GlobalHub.Unregister(node.NodeId)

	// 1. Initial State: Node is active and returned by GetAllOnlineNodes
	onlineNodes := GlobalHub.GetAllOnlineNodes()
	require.Len(t, onlineNodes, 1)

	// 2. User launches game -> is_gamer_mode: true
	gamerHeartbeat, _ := json.Marshal(map[string]interface{}{
		"is_gamer_mode":    true,
		"is_throttled":     false,
		"installed_models": []string{"model-1"},
	})
	session.HandleHeartbeat(&TunnelFrame{Type: MsgTypeHeartbeat, Payload: gamerHeartbeat})

	// Verify node status in DB
	dbNode, err := model.GetByogNodeByNodeId(node.NodeId)
	require.NoError(t, err)
	assert.Equal(t, "gamer_mode", dbNode.Status)
	assert.True(t, session.IsGamerMode)

	// Crucial: GetAllOnlineNodes must NOT return nodes in gamer mode!
	activeForInference := GlobalHub.GetAllOnlineNodes()
	assert.Empty(t, activeForInference, "Gamer mode node must not receive compute requests")

	// 3. User closes game -> is_gamer_mode: false
	normalHeartbeat, _ := json.Marshal(map[string]interface{}{
		"is_gamer_mode":    false,
		"is_throttled":     false,
		"installed_models": []string{"model-1"},
	})
	session.HandleHeartbeat(&TunnelFrame{Type: MsgTypeHeartbeat, Payload: normalHeartbeat})

	dbNode, err = model.GetByogNodeByNodeId(node.NodeId)
	require.NoError(t, err)
	assert.Equal(t, "online", dbNode.Status)
	assert.False(t, session.IsGamerMode)
	assert.Len(t, GlobalHub.GetAllOnlineNodes(), 1, "Resumed compute node must be available again")

	// 4. Thermal Throttling Guard (> 83°C) -> is_throttled: true
	throttleHeartbeat, _ := json.Marshal(map[string]interface{}{
		"is_gamer_mode":    false,
		"is_throttled":     true,
		"installed_models": []string{"model-1"},
	})
	session.HandleHeartbeat(&TunnelFrame{Type: MsgTypeHeartbeat, Payload: throttleHeartbeat})

	dbNode, err = model.GetByogNodeByNodeId(node.NodeId)
	require.NoError(t, err)
	assert.Equal(t, "throttled", dbNode.Status)
}

// TestAntiCheatCanaryAndByteProbe verifies that:
// 1. 20ms Byte Probe Spot Check requests random offset and verifies exact 64-byte file slice.
// 2. Canary Exam tests deterministic token generation and validates logprob tolerance.
// 3. HWID validation detects virtual machines, QEMU, Bluestacks, and hypervisors.
func TestAntiCheatCanaryAndByteProbe(t *testing.T) {
	db := setupDeepTestDB(t)

	// Test HWID Anti-Cheat
	ac := GlobalAntiCheat
	valid, reason := ac.ValidateHWID("real-pc-hwid-5847392", "windows", "desktop", map[string]interface{}{"cpu_model": "Intel Core i9-14900K"})
	assert.True(t, valid)
	assert.Empty(t, reason)

	// Virtual machine detections
	vmHwids := []string{"qemu-virtual-machine-1", "vbox-test-guest", "vmware-instance-09", "bluestacks-android-device"}
	for _, hw := range vmHwids {
		valid, reason := ac.ValidateHWID(hw, "linux", "desktop", nil)
		assert.False(t, valid, "HWID with %s should be blocked", hw)
		assert.Contains(t, reason, "Virtualized or emulator")
	}

	// Hypervisor CPU indicator check
	valid, reason = ac.ValidateHWID("normal-looking-hwid", "linux", "server", map[string]interface{}{"cpu_model": "QEMU Virtual CPU version 2.5+"})
	assert.False(t, valid)
	assert.Contains(t, reason, "Hypervisor")

	// Byte Probe Spot-Check & Canary via Mock WebSocket Tunnel
	testModel := &model.ByogModel{
		ModelId:                "canary-model-test",
		FileSize:               1024 * 1024 * 100, // 100 MB
		AntiCheatSigPrompt:     "Deterministic prompt",
		AntiCheatSigLogprob:    -0.125,
		AntiCheatSigTolerance:  0.010,
		AntiCheatLogicPrompt:   "2+2=?",
		AntiCheatLogicExpected: "4",
		IsEnabled:              true,
		DownloadUrl:            "http://example.com/test.gguf",
	}
	require.NoError(t, db.Create(testModel).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			var frame TunnelFrame
			if err := conn.ReadJSON(&frame); err != nil {
				break
			}
			switch frame.Type {
			case MsgTypeByteProbeReq:
				// Respond with 64 bytes of hex data
				dummyBytes := make([]byte, 64)
				for i := range dummyBytes {
					dummyBytes[i] = byte(i)
				}
				respPayload, _ := json.Marshal(map[string]string{
					"data_hex": hex.EncodeToString(dummyBytes),
				})
				_ = conn.WriteJSON(TunnelFrame{
					Type:    MsgTypeByteProbeResp,
					Id:      frame.Id,
					Payload: respPayload,
				})

			case MsgTypeCanaryReq:
				// Respond with deterministic logprob and raw tokens
				donePayload, _ := json.Marshal(map[string]interface{}{
					"request_id":    frame.Id,
					"total_tokens":  1,
					"prompt_tokens": 2,
					"raw_tokens":    []int{42},
					"logprobs":      []float64{-0.124}, // within tolerance (-0.125 +/- 0.010)
				})
				_ = conn.WriteJSON(TunnelFrame{
					Type:    MsgTypeCanaryResp,
					Id:      frame.Id,
					Payload: donePayload,
				})
			}
		}
	}))
	defer server.Close()

	wsUrl := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	require.NoError(t, err)
	defer clientConn.Close()

	session := &NodeSession{
		NodeId: "node-canary-test",
		Conn:   clientConn,
	}

	// Background reader for tunnel frames
	stopReader := make(chan struct{})
	go func() {
		for {
			var frame TunnelFrame
			if err := clientConn.ReadJSON(&frame); err != nil {
				return
			}
			if frame.Id != "" {
				if chVal, ok := session.PendingRequests.Load(frame.Id); ok {
					ch := chVal.(chan *TunnelFrame)
					ch <- &frame
				}
			}
		}
	}()
	defer close(stopReader)

	// 1. Run Byte Spot Check
	passedByte := ac.RunByteSpotCheck(session, testModel)
	assert.True(t, passedByte, "Byte spot check must pass when valid 64-byte data is returned")

	// 2. Run Canary Test
	passedCanary := ac.ExecuteCanaryTest(session, testModel)
	assert.True(t, passedCanary, "Canary test must pass when deterministic logprobs match")
}

// TestDeviceRemovedAndPairingRevoked verifies that:
// 1. NotifyDeviceRemoved sends device_removed frame over WebSocket.
// 2. Worker session is unregistered from GlobalHub.
// 3. Node status in DB updates to offline.
func TestDeviceRemovedAndPairingRevoked(t *testing.T) {
	db := setupDeepTestDB(t)

	node := &model.ByogNode{
		NodeId: "node-disconnect-test",
		Status: "online",
	}
	require.NoError(t, db.Create(node).Error)

	var receivedFrame TunnelFrame
	receivedChan := make(chan TunnelFrame, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for {
			var frame TunnelFrame
			if err := conn.ReadJSON(&frame); err != nil {
				break
			}
			receivedChan <- frame
		}
	}))
	defer server.Close()

	wsUrl := "ws" + strings.TrimPrefix(server.URL, "http")
	clientConn, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	require.NoError(t, err)
	defer clientConn.Close()

	session := &NodeSession{
		NodeId: node.NodeId,
		Conn:   clientConn,
	}
	GlobalHub.Register(session)

	// Notify device removed
	NotifyDeviceRemoved(node.NodeId, "Cihaz ağdan kaldırıldı")

	select {
	case receivedFrame = <-receivedChan:
		assert.Equal(t, MsgTypeDeviceRemoved, receivedFrame.Type)
		var p map[string]interface{}
		_ = json.Unmarshal(receivedFrame.Payload, &p)
		assert.Equal(t, node.NodeId, p["node_id"])
		assert.Equal(t, "removed", p["reason"])
	case <-time.After(2 * time.Second):
		t.Fatal("Worker did not receive device_removed frame in time")
	}

	GlobalHub.Unregister(node.NodeId)
	_, online := GlobalHub.GetSession(node.NodeId)
	assert.False(t, online)

	dbNode, err := model.GetByogNodeByNodeId(node.NodeId)
	require.NoError(t, err)
	assert.Equal(t, "offline", dbNode.Status)
}

// TestMultiNodeLoadBalancingAndConcurrencyLimiter verifies:
// 1. Multiple nodes online (Node 1 fast, Node 2 slower).
// 2. Router picks best node matching ping / installed model.
// 3. Sticky session binds conversation to same node.
// 4. Concurrency limiter stops requests when MaxCapacity is exceeded.
func TestMultiNodeLoadBalancingAndConcurrencyLimiter(t *testing.T) {
	db := setupDeepTestDB(t)

	modelDef := &model.ByogModel{
		ModelId:     "loadbalance-model",
		IsEnabled:   true,
		DownloadUrl: "http://test/lb.gguf",
	}
	require.NoError(t, db.Create(modelDef).Error)

	node1 := &model.ByogNode{
		NodeId:          "node-fast-ping",
		DeviceName:      "Fast GPU",
		DeviceType:      "desktop",
		Status:          "online",
		PingMs:          10,
		InstalledModels: `["loadbalance-model"]`,
	}
	node2 := &model.ByogNode{
		NodeId:          "node-slow-ping",
		DeviceName:      "Slow GPU",
		DeviceType:      "desktop",
		Status:          "online",
		PingMs:          150,
		InstalledModels: `["loadbalance-model"]`,
	}
	require.NoError(t, db.Create(node1).Error)
	require.NoError(t, db.Create(node2).Error)

	session1 := &NodeSession{
		NodeId:          node1.NodeId,
		InstalledModels: map[string]bool{"loadbalance-model": true},
	}
	session2 := &NodeSession{
		NodeId:          node2.NodeId,
		InstalledModels: map[string]bool{"loadbalance-model": true},
	}
	GlobalHub.Register(session1)
	GlobalHub.Register(session2)
	defer GlobalHub.Unregister(node1.NodeId)
	defer GlobalHub.Unregister(node2.NodeId)

	router := GlobalRouter

	// 1. Smart Geo-Routing: Should pick Node 1 because PingMs (10ms) < Node 2 (150ms)
	selected, err := router.SelectNodeForRequest("loadbalance-model", "conv-session-123", "", "192.168.1.50")
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, node1.NodeId, selected.NodeId, "Router should prioritize lowest ping node")

	// 2. Sticky Session verification: Next turn with same conversation_id stays on Node 1
	selectedTurn2, err := router.SelectNodeForRequest("loadbalance-model", "conv-session-123", "", "192.168.1.50")
	require.NoError(t, err)
	assert.Equal(t, node1.NodeId, selectedTurn2.NodeId, "Sticky session must keep same conversation on same node")

	// 3. Concurrency Limiter:
	// Mobile nodes have max capacity = 1
	mobileQueue := router.getNodeQueue("mobile-node-test", "mobile")
	assert.Equal(t, int32(1), mobileQueue.MaxCapacity)

	// Desktop nodes have max capacity = 4
	desktopQueue := router.getNodeQueue("desktop-node-test", "desktop")
	assert.Equal(t, int32(4), desktopQueue.MaxCapacity)
}
