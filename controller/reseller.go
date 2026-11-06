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
	query := model.QueryResellerRedemptions(userId, c.Query("keyword"))
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
	Type  int    `json:"type"` // only balance codes can be newly created
}

// ResellerCreateRedemption creates balance-only codes using the reseller's quota.
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

	if req.Type != 0 || req.Quota > common.MaxQuota/req.Count {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid balance type or amount"})
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = fmt.Sprintf("Reseller Code - %s", time.Now().Format("2006-01-02"))
	}
	createdKeys, err := model.CreateResellerBalanceCodes(userId, req.Quota, req.Count, name)
	if err != nil {
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

	if err := model.RefundResellerCode(userId, id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Code deleted and quota refunded"})
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
