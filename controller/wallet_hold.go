package controller

import (
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetOpenWalletHolds and ResolveWalletHold are registered with RootAuth only.
// Holds must be investigated before release; time alone proves nothing.
func GetOpenWalletHolds(c *gin.Context) {
	limit := 50
	if value, err := strconv.Atoi(c.Query("limit")); err == nil && value > 0 && value <= 100 {
		limit = value
	}
	query := model.DB.Model(&model.WalletTransferHold{}).Where("state = ?", "open")
	if raw := strings.TrimSpace(c.Query("user_id")); raw != "" {
		userID, err := strconv.Atoi(raw)
		if err != nil || userID <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid user ID"})
			return
		}
		query = query.Where("user_id = ?", userID)
	}
	var holds []model.WalletTransferHold
	if err := query.Order("created_at asc").Limit(limit).Find(&holds).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(holds))
	for _, hold := range holds {
		items = append(items, gin.H{"id": hold.ID, "user_id": hold.UserID, "created_at": hold.CreatedAt})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": items})
}

type resolveWalletHoldRequest struct {
	UserID    int    `json:"user_id"`
	Delta     int    `json:"delta"`
	Reason    string `json:"reason"`
	Confirmed bool   `json:"confirmed"`
}

func ResolveWalletHold(c *gin.Context) {
	id := c.Param("id")
	raw, err := hex.DecodeString(id)
	if err != nil || len(raw) != 16 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid hold ID"})
		return
	}
	var request resolveWalletHoldRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.UserID <= 0 || !request.Confirmed || len(strings.TrimSpace(request.Reason)) < 10 || len(strings.TrimSpace(request.Reason)) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "User ID, confirmation and a 10–500 character reason are required"})
		return
	}
	if err := model.ManuallyResolveWalletTransferHold(request.UserID, id, request.Delta, c.GetInt("id"), request.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
