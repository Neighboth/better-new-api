package byog

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestHeartbeatPersistsDownloadProgress makes sure that the progress a worker
// reports while a model is downloaded reaches the node row of the panel.
func TestHeartbeatPersistsDownloadProgress(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.AutoMigrate(&model.ByogNode{}, &model.ByogSetting{}))

	node := &model.ByogNode{
		NodeId:      "node-download-progress",
		Status:      "online",
		ModelStatus: "downloading",
	}
	require.NoError(t, db.Create(node).Error)

	session := &NodeSession{
		NodeId:          node.NodeId,
		InstalledModels: map[string]bool{},
	}

	payload, err := json.Marshal(map[string]interface{}{
		"model_status":      "downloading",
		"download_status":   "downloading",
		"downloaded_mb":     120,
		"total_mb":          400,
		"download_progress": 30,
		"installed_models":  []string{"qwen-2.5-0.5b-instruct"},
		"current_tps":       0,
	})
	require.NoError(t, err)

	session.HandleHeartbeat(&TunnelFrame{Type: MsgTypeHeartbeat, Payload: payload})

	updated, err := model.GetByogNodeByNodeId(node.NodeId)
	require.NoError(t, err)
	assert.Equal(t, "downloading", updated.ModelStatus)
	assert.Equal(t, 120, updated.DownloadedMB)
	assert.Equal(t, 400, updated.TotalMB)
	assert.Equal(t, 30, updated.DownloadProgress)
	assert.Equal(t, "downloading", updated.DownloadStatus)
	assert.Contains(t, updated.InstalledModels, "qwen-2.5-0.5b-instruct")
}

// TestHeartbeatKeepsProgressWhenTotalIsUnknown protects the case that a worker
// reports the transferred amount before it knows the total size of the model.
func TestHeartbeatKeepsProgressWhenTotalIsUnknown(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.AutoMigrate(&model.ByogNode{}, &model.ByogSetting{}))

	node := &model.ByogNode{
		NodeId:      "node-download-unknown-total",
		Status:      "online",
		ModelStatus: "downloading",
	}
	require.NoError(t, db.Create(node).Error)

	session := &NodeSession{
		NodeId:          node.NodeId,
		InstalledModels: map[string]bool{},
	}

	payload, err := json.Marshal(map[string]interface{}{
		"model_status":      "downloading",
		"downloaded_mb":     64,
		"download_progress": 16,
	})
	require.NoError(t, err)

	session.HandleHeartbeat(&TunnelFrame{Type: MsgTypeHeartbeat, Payload: payload})

	updated, err := model.GetByogNodeByNodeId(node.NodeId)
	require.NoError(t, err)
	assert.Equal(t, 64, updated.DownloadedMB)
	assert.Equal(t, 16, updated.DownloadProgress)
}

func TestOldSessionCannotUnregisterReplacement(t *testing.T) {
	oldDB := model.DB
	model.DB = nil
	t.Cleanup(func() { model.DB = oldDB })
	hub := &Hub{}
	old := &NodeSession{NodeId: "same-node"}
	replacement := &NodeSession{NodeId: "same-node"}
	hub.Register(old)
	hub.Register(replacement)
	hub.UnregisterSession(old)
	current, ok := hub.GetSession("same-node")
	require.True(t, ok)
	assert.Same(t, replacement, current)
	hub.UnregisterSession(replacement)
	_, ok = hub.GetSession("same-node")
	assert.False(t, ok)
}

func TestAuthenticatedHubRejectsRevokedNode(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	oldDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = oldDB })
	require.NoError(t, db.AutoMigrate(&model.ByogNode{}, &model.ByogNodeBan{}))
	node := &model.ByogNode{NodeId: "revoked-test", UserId: 2, DeviceName: "test", HWID: "hw", WorkerSecretHash: "hash", Status: "online"}
	require.NoError(t, db.Create(node).Error)
	hub := &Hub{}
	session := &NodeSession{NodeId: node.NodeId, UserId: node.UserId}
	require.False(t, hub.RegisterAuthenticated(session, "wrong"))
	ban := model.ByogNodeBan{NodeId: "other-node", UserId: node.UserId, HWID: "other-hw", BanReason: "test", ViolationCode: "test", IsActive: true}
	require.NoError(t, db.Create(&ban).Error)
	require.False(t, hub.RegisterAuthenticated(session, "hash"))
	require.NoError(t, db.Model(&ban).Update("is_active", false).Error)
	require.True(t, hub.RegisterAuthenticated(session, "hash"))
	hub.UnregisterSession(session)
	require.NoError(t, db.Delete(node).Error)
	require.False(t, hub.RegisterAuthenticated(session, "hash"))
	_, ok := hub.GetSession(node.NodeId)
	require.False(t, ok)
}
