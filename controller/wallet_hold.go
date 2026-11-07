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
	offset := 0
	if raw := c.Query("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid offset"})
			return
		}
		offset = value
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
	if err := query.Order("created_at asc, id asc").Offset(offset).Limit(limit).Find(&holds).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	ids := make([]string, 0, len(holds))
	for _, hold := range holds {
		ids = append(ids, hold.ID)
	}
	eventsByHold := make(map[string][]model.WalletHoldEvent)
	if len(ids) > 0 {
		var events []model.WalletHoldEvent
		if err := model.DB.Where("hold_id IN ?", ids).Order("created_at asc, event_key asc").Find(&events).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		for _, event := range events {
			eventsByHold[event.HoldID] = append(eventsByHold[event.HoldID], event)
		}
	}
	items := make([]gin.H, 0, len(holds))
	for _, hold := range holds {
		items = append(items, gin.H{"id": hold.ID, "user_id": hold.UserID, "created_at": hold.CreatedAt, "source": hold.Source, "reference": hold.Reference, "events": eventsByHold[hold.ID]})
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

// FinalizeByogWalletHold is a root-only reconciliation path for a stopped
// BYOG operation. The operator must investigate provider evidence first; this
// endpoint cannot prove that a worker has stopped on another coordinator.
func FinalizeByogWalletHold(c *gin.Context) {
	id := c.Param("id")
	raw, err := hex.DecodeString(id)
	if err != nil || len(raw) != 16 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid hold ID"})
		return
	}
	var request struct {
		UserID      int    `json:"user_id"`
		ActualQuota *int   `json:"actual_quota"`
		Reason      string `json:"reason"`
		Confirmed   bool   `json:"confirmed"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.UserID <= 0 || request.ActualQuota == nil || *request.ActualQuota < 0 || !request.Confirmed || len(strings.TrimSpace(request.Reason)) < 10 || len(strings.TrimSpace(request.Reason)) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "User ID, actual quota, confirmation and a 10–500 character reason are required"})
		return
	}
	var event model.WalletHoldEvent
	if err := model.DB.Where("hold_id = ? AND event_key = ? AND user_id = ?", id, "byog-precharge", request.UserID).First(&event).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if event.Delta >= 0 || *request.ActualQuota > -event.Delta {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Actual quota exceeds the BYOG reservation"})
		return
	}
	if err := model.FinalizeByogWalletAndToken(request.UserID, id, event.TokenID, -event.Delta, *request.ActualQuota); err != nil {
		common.ApiError(c, err)
		return
	}
	// Separate commits are replay-safe: an interrupted call can be retried
	// with the same amount and reason without double-crediting either balance.
	if err := model.ManuallyResolveWalletTransferHold(request.UserID, id, 0, c.GetInt("id"), request.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// FinalizeMidjourneyWalletHold reconciles a stopped, ambiguous submission.
// A billable result requires the durable pending task so later failure refunds
// have authoritative accounting evidence. Root must verify the provider first.
func FinalizeMidjourneyWalletHold(c *gin.Context) {
	id := c.Param("id")
	raw, err := hex.DecodeString(id)
	if err != nil || len(raw) != 16 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid hold ID"})
		return
	}
	var request struct {
		UserID    int    `json:"user_id"`
		Billable  *bool  `json:"billable"`
		TaskID    int    `json:"task_id"`
		Reason    string `json:"reason"`
		Confirmed bool   `json:"confirmed"`
	}
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.UserID <= 0 || request.Billable == nil || (*request.Billable && request.TaskID <= 0) || !request.Confirmed || len(strings.TrimSpace(request.Reason)) < 10 || len(strings.TrimSpace(request.Reason)) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "User ID, billable decision, task ID for billable work, confirmation and a 10–500 character reason are required"})
		return
	}
	var event model.WalletHoldEvent
	if err := model.DB.Where("hold_id = ? AND event_key = ? AND user_id = ?", id, "midjourney-precharge", request.UserID).First(&event).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if event.Delta >= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid Midjourney reservation"})
		return
	}
	if *request.Billable {
		_, err = model.FinalizeMidjourneyReservedTask(request.UserID, id, request.TaskID, event.TokenID, -event.Delta)
	} else {
		err = model.ReleaseMidjourneyWalletAndToken(request.UserID, id, event.TokenID, -event.Delta)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.ManuallyResolveWalletTransferHold(request.UserID, id, 0, c.GetInt("id"), request.Reason); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
