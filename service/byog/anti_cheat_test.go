package byog

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestCountersStayReadableAfterAnInference makes sure that the counters of a node
// stay numbers.
//
// The counters are increased with an expression of the database. A plain string
// would be written into the column and the node could not be read any more, the
// worker would be reported as unknown.
func TestCountersStayReadableAfterAnInference(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.AutoMigrate(&model.ByogNode{}, &model.ByogSetting{}))

	node := &model.ByogNode{
		NodeId: "node-counters",
		Status: "online",
	}
	require.NoError(t, db.Create(node).Error)

	require.NoError(t, db.Model(&model.ByogNode{}).
		Where("node_id = ?", node.NodeId).
		Updates(map[string]interface{}{
			"total_tokens_processed": gormExpr("total_tokens_processed + ?", 14),
			"total_earned_credits":   gormExpr("total_earned_credits + ?", 0.000014),
		}).Error)

	updated, err := model.GetByogNodeByNodeId(node.NodeId)
	require.NoError(t, err)
	assert.Equal(t, int64(14), updated.TotalTokensProcessed)
	assert.InDelta(t, 0.000014, updated.TotalEarnedCredits, 0.0000001)
}

// TestReputationStaysANumber makes sure that the reputation of a node stays a
// number when it is increased or decreased.
func TestReputationStaysANumber(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.AutoMigrate(&model.ByogNode{}, &model.ByogSetting{}))

	node := &model.ByogNode{
		NodeId:          "node-reputation",
		Status:          "online",
		ReputationScore: 50,
	}
	require.NoError(t, db.Create(node).Error)

	GlobalAntiCheat.IncreaseReputation(node.NodeId, 10)
	GlobalAntiCheat.IncreaseReputation(node.NodeId, 80)
	GlobalAntiCheat.DecreaseReputation(node.NodeId, 20)

	updated, err := model.GetByogNodeByNodeId(node.NodeId)
	require.NoError(t, err)
	assert.Equal(t, 80, updated.ReputationScore)
}
