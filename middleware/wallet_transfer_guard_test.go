package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWalletTransferGuardBlocksIssuanceAndReleasesOnReturn(t *testing.T) {
	setupDashboardAuthMiddlewareTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.WalletTransferHold{}, &model.Redemption{}, &model.UserBalancePackage{}, &model.Task{}))
	user := createMiddlewarePATUser(t, "wallet-guard", "wallet-guard-token")
	require.NoError(t, model.DB.Model(user).Update("quota", 100).Error)
	oldBatch := common.BatchUpdateEnabled
	common.BatchUpdateEnabled = false
	t.Cleanup(func() { common.BatchUpdateEnabled = oldBatch })
	router := gin.New()
	router.Use(func(c *gin.Context) { c.Set("id", user.Id); c.Next() }, WalletTransferGuard())
	router.GET("/ok", func(c *gin.Context) {
		require.NotEmpty(t, c.GetString(common.WalletTransferHoldContextKey))
		_, err := model.CreateResellerBalanceCodes(user.Id, 10, 1, "while-active")
		require.Error(t, err)
		c.Status(http.StatusNoContent)
	})
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))
	_, err := model.CreateResellerBalanceCodes(user.Id, 10, 1, "after-return")
	require.NoError(t, err)
}

func TestWalletTransferGuardRetainsFailedAndPanickedOperations(t *testing.T) {
	setupDashboardAuthMiddlewareTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.WalletTransferHold{}))
	user := createMiddlewarePATUser(t, "wallet-retain", "wallet-retain-token")
	router := gin.New()
	router.Use(gin.Recovery(), func(c *gin.Context) { c.Set("id", user.Id); c.Next() }, WalletTransferGuard())
	router.GET("/failed", func(c *gin.Context) {
		c.Set(common.WalletTransferHoldRetainContextKey, true)
		c.Status(http.StatusInternalServerError)
	})
	router.GET("/panic", func(c *gin.Context) { panic("failed operation") })
	for _, path := range []string{"/failed", "/panic"} {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	var count int64
	require.NoError(t, model.DB.Model(&model.WalletTransferHold{}).Where("user_id = ? AND state = ?", user.Id, "open").Count(&count).Error)
	require.Equal(t, int64(2), count)
}
