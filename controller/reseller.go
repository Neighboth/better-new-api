package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// ResellerGetRedemptions lists redemption codes created by the reseller.
func ResellerGetRedemptions(c *gin.Context) {
	userId := c.GetInt("id")
	pStr := c.DefaultQuery("p", c.DefaultQuery("page", "1"))
	page, _ := strconv.Atoi(pStr)
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	startIdx := (page - 1) * pageSize

	var redemptions []*model.Redemption
	var total int64
	query := model.DB.Model(&model.Redemption{}).Where("user_id = ?", userId)
	keyword := strings.TrimSpace(c.Query("keyword"))
	if keyword != "" {
		query = query.Where("name LIKE ? OR key LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if err := query.Order("id desc").Limit(pageSize).Offset(startIdx).Find(&redemptions).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	usedUserIds := make([]int, 0, len(redemptions))
	for _, r := range redemptions {
		r.IsReseller = true
		if r.UsedUserId > 0 {
			usedUserIds = append(usedUserIds, r.UsedUserId)
		}
	}
	if len(usedUserIds) > 0 {
		var users []model.User
		model.DB.Model(&model.User{}).Select("id, username").Where("id IN ?", usedUserIds).Find(&users)
		userMap := make(map[int]string)
		for _, u := range users {
			userMap[u.Id] = u.Username
		}
		for _, r := range redemptions {
			if name, ok := userMap[r.UsedUserId]; ok {
				r.UsedUsername = name
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": redemptions,
			"total": total,
		},
	})
}

type ResellerCreateRedemptionRequest struct {
	Name  string `json:"name"`
	Quota int    `json:"quota"`
	Count int    `json:"count"`
	Type  int    `json:"type"` // 0: Quota, 1: Requests, 2: Tokens
}

// ResellerCreateRedemption creates redemption codes using the reseller's available quota/requests/tokens.
func ResellerCreateRedemption(c *gin.Context) {
	userId := c.GetInt("id")
	var req ResellerCreateRedemptionRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request payload"})
		return
	}
	if req.Count <= 0 {
		req.Count = 1
	}
	if req.Count > 100 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Cannot generate more than 100 codes at once"})
		return
	}
	if req.Quota <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Amount must be greater than 0"})
		return
	}

	if req.Type < 0 || req.Type > 2 || req.Quota > common.MaxQuota/req.Count {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid balance type or amount"})
		return
	}

	totalQuotaNeeded := req.Quota * req.Count

	var user model.User
	tx := model.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Where("id = ?", userId).First(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "User not found"})
		return
	}

	switch req.Type {
	case 1: // Requests
		if user.RequestsBalance < totalQuotaNeeded {
			tx.Rollback()
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("Insufficient requests balance. Needed: %d, Available: %d", totalQuotaNeeded, user.RequestsBalance),
			})
			return
		}
		user.RequestsBalance -= totalQuotaNeeded
	case 2: // Tokens
		if user.TokensBalance < int64(totalQuotaNeeded) {
			tx.Rollback()
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("Insufficient tokens balance. Needed: %d, Available: %d", totalQuotaNeeded, user.TokensBalance),
			})
			return
		}
		user.TokensBalance -= int64(totalQuotaNeeded)
	default: // Quota
		if user.Quota < totalQuotaNeeded {
			tx.Rollback()
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": fmt.Sprintf("Insufficient quota. Needed: %d, Available: %d", totalQuotaNeeded, user.Quota),
			})
			return
		}
		user.Quota -= totalQuotaNeeded
	}

	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to update balance"})
		return
	}

	now := time.Now().Unix()
	var createdKeys []string
	for i := 0; i < req.Count; i++ {
		key := "cdk_" + common.GetRandomString(28)
		name := req.Name
		if name == "" {
			name = fmt.Sprintf("Reseller Code - %s", time.Now().Format("2006-01-02"))
		}
		redemption := model.Redemption{
			UserId:      userId,
			Name:        name,
			Key:         key,
			Quota:       req.Quota,
			Type:        req.Type,
			CreatedTime: now,
			Status:      common.RedemptionCodeStatusEnabled,
		}
		if err := tx.Create(&redemption).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to generate redemption code"})
			return
		}
		createdKeys = append(createdKeys, key)
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Redemption codes generated successfully",
		"data": gin.H{
			"count": len(createdKeys),
			"keys":  createdKeys,
		},
	})
}

// ResellerDeleteRedemption deletes an unused code and refunds quota to the reseller.
func ResellerDeleteRedemption(c *gin.Context) {
	userId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid code ID"})
		return
	}

	tx := model.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var redemption model.Redemption
	if err := tx.Where("id = ? AND user_id = ?", id, userId).First(&redemption).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Code not found or access denied"})
		return
	}

	if redemption.Status == common.RedemptionCodeStatusUsed {
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Cannot delete already redeemed code"})
		return
	}

	// Refund balance according to type
	var user model.User
	if err := tx.Where("id = ?", userId).First(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "User not found"})
		return
	}

	switch redemption.Type {
	case 1:
		user.RequestsBalance += redemption.Quota
	case 2:
		user.TokensBalance += int64(redemption.Quota)
	default:
		user.Quota += redemption.Quota
	}

	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to refund balance"})
		return
	}

	if err := tx.Delete(&redemption).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to delete redemption code"})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Code deleted and quota refunded"})
}

// ResellerGetUserLogs allows a reseller to view usage logs of a user who redeemed their code.
func ResellerGetUserLogs(c *gin.Context) {
	resellerId := c.GetInt("id")
	targetUserId, err := strconv.Atoi(c.Param("userId"))
	if err != nil || targetUserId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid user ID"})
		return
	}

	// Verify the target user redeemed a code belonging to this reseller
	var count int64
	if err := model.DB.Model(&model.Redemption{}).Where("user_id = ? AND used_user_id = ?", resellerId, targetUserId).Count(&count).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if count == 0 {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "You do not have permission to view logs for this user"})
		return
	}

	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	tokenName := c.Query("token_name")
	modelName := c.Query("model_name")
	group := c.Query("group")
	requestId := c.Query("request_id")
	upstreamRequestId := c.Query("upstream_request_id")

	logs, total, err := model.GetUserLogs(targetUserId, logType, startTimestamp, endTimestamp, modelName, tokenName, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), group, requestId, upstreamRequestId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

// ResellerGetSummary returns aggregate metrics for the calling reseller.
func ResellerGetSummary(c *gin.Context) {
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Reseller user not found"})
		return
	}

	var totalCodes int64
	model.DB.Model(&model.Redemption{}).Where("user_id = ?", userId).Count(&totalCodes)

	var usedCodes int64
	model.DB.Model(&model.Redemption{}).Where("user_id = ? AND (status = ? OR used_user_id > 0)", userId, common.RedemptionCodeStatusUsed).Count(&usedCodes)
	unusedCodes := totalCodes - usedCodes

	var totalQuotaDist int64
	model.DB.Model(&model.Redemption{}).Where("user_id = ? AND type = 0", userId).Select("COALESCE(SUM(quota), 0)").Scan(&totalQuotaDist)

	var totalRequestsDist int64
	model.DB.Model(&model.Redemption{}).Where("user_id = ? AND type = 1", userId).Select("COALESCE(SUM(quota), 0)").Scan(&totalRequestsDist)

	var totalTokensDist int64
	model.DB.Model(&model.Redemption{}).Where("user_id = ? AND type = 2", userId).Select("COALESCE(SUM(quota), 0)").Scan(&totalTokensDist)

	var activeSubUsers int64
	model.DB.Model(&model.Redemption{}).Where("user_id = ? AND used_user_id > 0", userId).Distinct("used_user_id").Count(&activeSubUsers)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"total_codes":                totalCodes,
			"used_codes":                 usedCodes,
			"unused_codes":               unusedCodes,
			"total_quota_distributed":    totalQuotaDist,
			"total_requests_distributed": totalRequestsDist,
			"total_tokens_distributed":   totalTokensDist,
			"reseller_quota":             user.Quota,
			"reseller_requests":          user.RequestsBalance,
			"reseller_tokens":            user.TokensBalance,
			"active_sub_users":           activeSubUsers,
		},
	})
}
