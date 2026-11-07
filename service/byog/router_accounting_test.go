package byog

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUncertainDispatchRetainsPaidUserHold(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	old := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = old })
	require.NoError(t, db.AutoMigrate(&model.ByogNode{}, &model.User{}))
	node := model.ByogNode{NodeId: "dispatch-no-tunnel", UserId: 1, Status: "online", DeviceType: "desktop"}
	require.NoError(t, db.Create(&node).Error)
	require.NoError(t, db.Create(&model.User{Id: 2, Username: "paid-user", Quota: 1, AffCode: "paid-aff"}).Error)
	router := &Router{}
	for _, tc := range []struct {
		userID int
		retain bool
	}{{2, true}, {1, false}} {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		ctx.Set("id", tc.userID)
		err := router.DispatchInference(ctx, &NodeSession{NodeId: node.NodeId}, &model.ByogModel{}, InferencePayload{RequestId: "no-tunnel"})
		require.Error(t, err)
		require.Equal(t, tc.retain, ctx.GetBool(common.WalletTransferHoldRetainContextKey))
	}
}

func TestPaidByogRejectsEmptySQLWalletBeforeDispatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	old := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = old })
	require.NoError(t, db.AutoMigrate(&model.ByogNode{}, &model.User{}))
	node := model.ByogNode{NodeId: "paid-empty-wallet", UserId: 1, Status: "online", DeviceType: "desktop"}
	require.NoError(t, db.Create(&node).Error)
	require.NoError(t, db.Create(&model.User{Id: 2, Username: "empty-user", Quota: 0, AffCode: "empty-aff"}).Error)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("id", 2)
	ctx.Set(common.WalletTransferHoldContextKey, "hold")
	err = (&Router{}).DispatchInference(ctx, &NodeSession{NodeId: node.NodeId}, &model.ByogModel{}, InferencePayload{})
	require.Error(t, err)
	require.Equal(t, http.StatusPaymentRequired, rec.Code)
	require.False(t, ctx.GetBool(common.WalletTransferHoldRetainContextKey))
}
