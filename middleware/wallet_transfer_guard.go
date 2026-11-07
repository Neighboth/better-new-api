package middleware

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// WalletTransferGuard records potential late charges before executing relay
// middleware or handlers. A crash, panic, failed settlement or explicit async
// handoff leaves the hold open for reconciliation instead of freeing value.
func WalletTransferGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt("id")
		if userID <= 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "wallet identity unavailable"})
			return
		}
		reference := c.GetString(common.RequestIdKey)
		if len(reference) > 191 {
			reference = reference[:191]
		}
		holdID, err := model.BeginWalletTransferHoldWithReference(userID, "relay", reference)
		if err != nil {
			common.SysError("wallet transfer hold creation failed: " + err.Error())
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "wallet accounting temporarily unavailable"})
			return
		}
		c.Set(common.WalletTransferHoldContextKey, holdID)
		// This is deliberately not a defer: a panic must leave the hold open.
		c.Next()
		if c.GetBool(common.WalletTransferHoldRetainContextKey) {
			return
		}
		if err := model.ResolveWalletTransferHold(userID, holdID, 0); err != nil {
			common.SysError("wallet transfer hold release failed: " + err.Error())
		}
	}
}
