package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWalletHoldReconciliationRequiresConfirmationAndRecordsActor(t *testing.T) {
	db := setupManageUserTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.WalletTransferHold{}, &model.WalletHoldEvent{}))
	user := model.User{Username: "hold_user", Quota: 100, AffCode: "hold_user_aff"}
	require.NoError(t, db.Create(&user).Error)
	id, err := model.BeginWalletTransferHold(user.Id)
	require.NoError(t, err)
	call := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/wallet-holds/"+id+"/resolve", strings.NewReader(body))
		c.Params = gin.Params{{Key: "id", Value: id}}
		c.Set("id", 999)
		ResolveWalletHold(c)
		return rec
	}
	require.Equal(t, http.StatusBadRequest, call(fmt.Sprintf(`{"user_id":%d,"delta":-10,"reason":"checked provider logs"}`, user.Id)).Code)
	rec := call(fmt.Sprintf(`{"user_id":%d,"delta":-10,"reason":"checked provider logs","confirmed":true}`, user.Id))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"success":true`)
	var hold model.WalletTransferHold
	require.NoError(t, db.First(&hold, "id = ?", id).Error)
	require.Equal(t, "resolved", hold.State)
	require.Equal(t, 999, hold.ResolvedBy)
	require.Equal(t, "checked provider logs", hold.ResolutionNote)
	require.NoError(t, db.First(&user, user.Id).Error)
	require.Equal(t, 90, user.Quota)
}

func TestWalletHoldListIncludesJournalEvidence(t *testing.T) {
	db := setupManageUserTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.WalletTransferHold{}, &model.WalletHoldEvent{}))
	user := model.User{Username: "hold_list", Quota: 100, AffCode: "hold_list_aff"}
	require.NoError(t, db.Create(&user).Error)
	id, err := model.BeginWalletTransferHoldWithReference(user.Id, "relay", "request-example")
	require.NoError(t, err)
	ok, err := model.ReserveWalletHoldQuota(user.Id, id, "relay-precharge", 20)
	require.NoError(t, err)
	require.True(t, ok)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/wallet-holds?user_id=%d&limit=1&offset=0", user.Id), nil)
	GetOpenWalletHolds(c)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), id)
	require.Contains(t, rec.Body.String(), "request-example")
	require.Contains(t, rec.Body.String(), "relay-precharge")
	require.Contains(t, rec.Body.String(), `"delta":-20`)
}
