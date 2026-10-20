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
