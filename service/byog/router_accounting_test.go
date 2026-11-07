package byog

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
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

func TestPaidByogReservesBeforeDispatchAndSettlesBoundedUsage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldLogDB, oldRedis, oldBatch, oldConsume := model.DB, model.LOG_DB, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	model.DB, model.LOG_DB, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = db, db, false, false, false
	t.Cleanup(func() {
		model.DB, model.LOG_DB, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = oldDB, oldLogDB, oldRedis, oldBatch, oldConsume
	})
	require.NoError(t, db.AutoMigrate(&model.ByogNode{}, &model.ByogSetting{}, &model.User{}, &model.WalletTransferHold{}, &model.WalletHoldEvent{}, &model.Token{}))
	node := model.ByogNode{NodeId: "bounded-dispatch", UserId: 1, Status: "online", DeviceType: "desktop"}
	require.NoError(t, db.Create(&node).Error)
	require.NoError(t, db.Create(&model.User{Id: 2, Username: "bounded-user", Quota: 10000, AffCode: "bounded-aff"}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 2, UserId: 2, Key: "bounded-test-token", Status: common.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10000}).Error)
	hold, err := model.BeginWalletTransferHold(2)
	require.NoError(t, err)
	connections := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, upgradeErr := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if upgradeErr == nil {
			connections <- conn
		}
	}))
	t.Cleanup(server.Close)
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	serverConn := <-connections
	t.Cleanup(func() { serverConn.Close() })
	session := &NodeSession{NodeId: node.NodeId, Conn: serverConn}
	workerErr := make(chan error, 1)
	go func() {
		var request TunnelFrame
		if err := client.ReadJSON(&request); err != nil {
			workerErr <- err
			return
		}
		value, ok := session.PendingRequests.Load(request.Id)
		if !ok {
			workerErr <- fmt.Errorf("request not registered")
			return
		}
		ch := value.(chan *TunnelFrame)
		payload, _ := json.Marshal(ChunkPayload{Content: "Hi"})
		ch <- &TunnelFrame{Type: MsgTypeInferenceChunk, Payload: payload}
		payload, _ = json.Marshal(DonePayload{PromptTokens: 4})
		ch <- &TunnelFrame{Type: MsgTypeInferenceDone, Payload: payload}
		workerErr <- nil
	}()
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("id", 2)
	ctx.Set("token_id", 2)
	ctx.Set(common.WalletTransferHoldContextKey, hold)
	payload := InferencePayload{RequestId: "bounded-request", Prompt: "hi", Messages: json.RawMessage(`[]`), MaxTokens: 2}
	modelDef := &model.ByogModel{ModelId: "unknown-byog-model", PricingTier: 2}
	price, err := snapshotByogPrice(modelDef)
	require.NoError(t, err)
	expected, err := price.quota(4, 1, false)
	require.NoError(t, err)
	err = (&Router{}).DispatchInference(ctx, session, modelDef, payload)
	require.NoError(t, err)
	require.NoError(t, <-workerErr)
	var events []model.WalletHoldEvent
	require.NoError(t, db.Where("hold_id = ?", hold).Find(&events).Error)
	require.Len(t, events, 2)
	var pre, final int
	for _, event := range events {
		if event.EventKey == "byog-precharge" {
			pre = event.Delta
		}
		if event.EventKey == "byog-final" {
			final = event.Delta
		}
	}
	require.Less(t, pre, 0)
	require.GreaterOrEqual(t, final, 0)
	require.Equal(t, -expected, pre+final)
	quota, err := model.GetUserQuota(2, true)
	require.NoError(t, err)
	require.Equal(t, 10000+pre+final, quota)
	var token model.Token
	require.NoError(t, db.First(&token, 2).Error)
	require.Equal(t, 10000-expected, token.RemainQuota)
	require.Equal(t, expected, token.UsedQuota)
	require.False(t, ctx.GetBool(common.WalletTransferHoldRetainContextKey))
	require.NoError(t, model.ResolveWalletTransferHold(2, hold, 0))
}

func TestPaidByogRejectsInsufficientMaximumBeforeDispatch(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldRedis, oldBatch := model.DB, common.RedisEnabled, common.BatchUpdateEnabled
	model.DB, common.RedisEnabled, common.BatchUpdateEnabled = db, false, false
	t.Cleanup(func() { model.DB, common.RedisEnabled, common.BatchUpdateEnabled = oldDB, oldRedis, oldBatch })
	require.NoError(t, db.AutoMigrate(&model.ByogNode{}, &model.ByogSetting{}, &model.User{}, &model.WalletTransferHold{}, &model.WalletHoldEvent{}))
	node := model.ByogNode{NodeId: "insufficient-max", UserId: 1, DeviceType: "desktop"}
	require.NoError(t, db.Create(&node).Error)
	require.NoError(t, db.Create(&model.User{Id: 2, Username: "small-wallet", Quota: 1, AffCode: "small-aff"}).Error)
	hold, err := model.BeginWalletTransferHold(2)
	require.NoError(t, err)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("id", 2)
	ctx.Set(common.WalletTransferHoldContextKey, hold)
	err = (&Router{}).DispatchInference(ctx, &NodeSession{NodeId: node.NodeId}, &model.ByogModel{ModelId: "unknown-byog-model", PricingTier: 2}, InferencePayload{Prompt: "hi", MaxTokens: 100})
	require.Error(t, err)
	require.Equal(t, http.StatusPaymentRequired, rec.Code)
	require.False(t, ctx.GetBool(common.WalletTransferHoldRetainContextKey))
	quota, err := model.GetUserQuota(2, true)
	require.NoError(t, err)
	require.Equal(t, 1, quota)
	var events int64
	require.NoError(t, db.Model(&model.WalletHoldEvent{}).Where("hold_id = ?", hold).Count(&events).Error)
	require.Zero(t, events)
	require.NoError(t, model.ResolveWalletTransferHold(2, hold, 0))
}

func TestPaidByogRetainsReserveWhenDispatchOutcomeIsUnknown(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldRedis, oldBatch := model.DB, common.RedisEnabled, common.BatchUpdateEnabled
	model.DB, common.RedisEnabled, common.BatchUpdateEnabled = db, false, false
	t.Cleanup(func() { model.DB, common.RedisEnabled, common.BatchUpdateEnabled = oldDB, oldRedis, oldBatch })
	require.NoError(t, db.AutoMigrate(&model.ByogNode{}, &model.ByogSetting{}, &model.User{}, &model.WalletTransferHold{}, &model.WalletHoldEvent{}))
	node := model.ByogNode{NodeId: "ambiguous-max", UserId: 1, DeviceType: "desktop"}
	require.NoError(t, db.Create(&node).Error)
	require.NoError(t, db.Create(&model.User{Id: 2, Username: "funded-wallet", Quota: 10000, AffCode: "funded-aff"}).Error)
	hold, err := model.BeginWalletTransferHold(2)
	require.NoError(t, err)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("id", 2)
	ctx.Set(common.WalletTransferHoldContextKey, hold)
	err = (&Router{}).DispatchInference(ctx, &NodeSession{NodeId: node.NodeId}, &model.ByogModel{ModelId: "unknown-byog-model", PricingTier: 2}, InferencePayload{Prompt: "hi", MaxTokens: 100})
	require.Error(t, err)
	require.True(t, ctx.GetBool(common.WalletTransferHoldRetainContextKey))
	var event model.WalletHoldEvent
	require.NoError(t, db.Where("hold_id = ? AND event_key = ?", hold, "byog-precharge").First(&event).Error)
	require.Less(t, event.Delta, 0)
	quota, err := model.GetUserQuota(2, true)
	require.NoError(t, err)
	require.Equal(t, 10000+event.Delta, quota)
	// This test is only cleanup; production operators must reconcile first.
	require.NoError(t, model.ApplyWalletHoldDelta(2, hold, "test-refund", -event.Delta))
	require.NoError(t, model.ResolveWalletTransferHold(2, hold, 0))
}

func TestPaidByogRejectsWorkerOutputPastAuthorizedLimit(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	oldDB, oldLogDB, oldRedis, oldBatch, oldConsume := model.DB, model.LOG_DB, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	model.DB, model.LOG_DB, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = db, db, false, false, false
	t.Cleanup(func() {
		model.DB, model.LOG_DB, common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = oldDB, oldLogDB, oldRedis, oldBatch, oldConsume
	})
	require.NoError(t, db.AutoMigrate(&model.ByogNode{}, &model.ByogSetting{}, &model.User{}, &model.WalletTransferHold{}, &model.WalletHoldEvent{}))
	node := model.ByogNode{NodeId: "excess-worker", UserId: 1, DeviceType: "desktop"}
	require.NoError(t, db.Create(&node).Error)
	require.NoError(t, db.Create(&model.User{Id: 2, Username: "bounded-client", Quota: 10000, AffCode: "bounded-client-aff"}).Error)
	hold, err := model.BeginWalletTransferHold(2)
	require.NoError(t, err)
	connections := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err == nil {
			connections <- conn
		}
	}))
	t.Cleanup(server.Close)
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { client.Close() })
	conn := <-connections
	t.Cleanup(func() { conn.Close() })
	session := &NodeSession{NodeId: node.NodeId, Conn: conn}
	workerErr := make(chan error, 1)
	go func() {
		var request TunnelFrame
		if err := client.ReadJSON(&request); err != nil {
			workerErr <- err
			return
		}
		value, ok := session.PendingRequests.Load(request.Id)
		if !ok {
			workerErr <- fmt.Errorf("request not registered")
			return
		}
		ch := value.(chan *TunnelFrame)
		for _, piece := range []string{"first-piece", "excess-piece"} {
			payload, _ := json.Marshal(ChunkPayload{Content: piece})
			ch <- &TunnelFrame{Type: MsgTypeInferenceChunk, Payload: payload}
		}
		workerErr <- nil
	}()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("id", 2)
	ctx.Set(common.WalletTransferHoldContextKey, hold)
	err = (&Router{}).DispatchInference(ctx, session, &model.ByogModel{ModelId: "unknown-byog-model", PricingTier: 2}, InferencePayload{RequestId: "excess", Prompt: "hi", MaxTokens: 1, Stream: true})
	require.Error(t, err)
	require.NoError(t, <-workerErr)
	require.Contains(t, rec.Body.String(), "first-piece")
	require.NotContains(t, rec.Body.String(), "excess-piece")
	require.True(t, ctx.GetBool(common.WalletTransferHoldRetainContextKey))
	var events []model.WalletHoldEvent
	require.NoError(t, db.Where("hold_id = ?", hold).Find(&events).Error)
	require.Len(t, events, 1)
	require.Equal(t, "byog-precharge", events[0].EventKey)
	require.NoError(t, model.ApplyWalletHoldDelta(2, hold, "test-refund", -events[0].Delta))
	require.NoError(t, model.ResolveWalletTransferHold(2, hold, 0))
}
