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

// AdminGetResellerConfig returns the reseller config for a specific user.
func AdminGetResellerConfig(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("userId"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid user ID"})
		return
	}
	config, err := model.GetOrCreateResellerConfig(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

type AdminUpdateResellerRequest struct {
	ChildPanelEnabled *bool `json:"child_panel_enabled"`
}

// AdminUpdateResellerConfig updates reseller feature flags for a user.
func AdminUpdateResellerConfig(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("userId"))
	if err != nil || userId <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid user ID"})
		return
	}
	var req AdminUpdateResellerRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request payload"})
		return
	}
	config, err := model.GetOrCreateResellerConfig(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if req.ChildPanelEnabled != nil {
		config.ChildPanelEnabled = *req.ChildPanelEnabled
	}
	if err := model.UpdateResellerConfig(config); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Reseller settings updated", "data": config})
}

// ResellerGetSelfConfig returns the calling reseller's own config.
func ResellerGetSelfConfig(c *gin.Context) {
	userId := c.GetInt("id")
	config, err := model.GetOrCreateResellerConfig(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": config})
}

type ResellerUpdateSelfConfigRequest struct {
	CustomDomain        *string `json:"custom_domain"`
	SiteName            *string `json:"site_name"`
	Logo                *string `json:"logo"`
	LogoUrl             *string `json:"logo_url"`
	Favicon             *string `json:"favicon"`
	FaviconUrl          *string `json:"favicon_url"`
	SeoTitle            *string `json:"seo_title"`
	SeoDescription      *string `json:"seo_description"`
	SeoKeywords         *string `json:"seo_keywords"`
	HomepageContent     *string `json:"homepage_content"`
	HomePageContent     *string `json:"home_page_content"`
	EpayPartnerId       *string `json:"epay_partner_id"`
	EpayKey             *string `json:"epay_key"`
	EpayPartnerKey      *string `json:"epay_partner_key"`
	EpayUrl             *string `json:"epay_url"`
	EpayGatewayUrl      *string `json:"epay_gateway_url"`
	StripeApiSecret     *string `json:"stripe_api_secret"`
	StripeWebhookSecret *string `json:"stripe_webhook_secret"`
	StripePriceId       *string `json:"stripe_price_id"`
	CreemApiKey         *string `json:"creem_api_key"`
	CreemWebhookSecret  *string `json:"creem_webhook_secret"`
	CreemTestMode       *bool   `json:"creem_test_mode"`
	WaffoMerchantId     *string `json:"waffo_merchant_id"`
	WaffoApiKey         *string `json:"waffo_api_key"`
	WaffoPrivateKey     *string `json:"waffo_private_key"`
}

// ResellerUpdateSelfConfig allows a reseller to update their branding and payment settings.
func ResellerUpdateSelfConfig(c *gin.Context) {
	userId := c.GetInt("id")
	config, err := model.GetOrCreateResellerConfig(userId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	var req ResellerUpdateSelfConfigRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid request payload"})
		return
	}

	if !config.ChildPanelEnabled {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Child panel is not enabled for your account"})
		return
	}

	if req.CustomDomain != nil {
		cleanDomain := strings.TrimSpace(strings.ToLower(*req.CustomDomain))
		cleanDomain = strings.TrimPrefix(cleanDomain, "http://")
		cleanDomain = strings.TrimPrefix(cleanDomain, "https://")
		cleanDomain = strings.Split(cleanDomain, "/")[0]
		cleanDomain = strings.Split(cleanDomain, ":")[0]

		// Check if domain is already taken by another reseller
		if cleanDomain != "" {
			existing, err := model.GetResellerConfigByDomain(cleanDomain)
			if err == nil && existing != nil && existing.UserId != userId {
				c.JSON(http.StatusOK, gin.H{"success": false, "message": "Domain is already assigned to another panel"})
				return
			}
		}
		config.CustomDomain = cleanDomain
	}
	if req.SiteName != nil {
		config.SiteName = strings.TrimSpace(*req.SiteName)
	}
	if req.Logo != nil {
		config.Logo = strings.TrimSpace(*req.Logo)
	} else if req.LogoUrl != nil {
		config.Logo = strings.TrimSpace(*req.LogoUrl)
	}
	if req.Favicon != nil {
		config.Favicon = strings.TrimSpace(*req.Favicon)
	} else if req.FaviconUrl != nil {
		config.Favicon = strings.TrimSpace(*req.FaviconUrl)
	}
	if req.SeoTitle != nil {
		config.SeoTitle = strings.TrimSpace(*req.SeoTitle)
	}
	if req.SeoDescription != nil {
		config.SeoDescription = strings.TrimSpace(*req.SeoDescription)
	}
	if req.SeoKeywords != nil {
		config.SeoKeywords = strings.TrimSpace(*req.SeoKeywords)
	}
	if req.HomepageContent != nil {
		config.HomepageContent = *req.HomepageContent
	} else if req.HomePageContent != nil {
		config.HomepageContent = *req.HomePageContent
	}

	// Payment Gateways (Child panel only)
	if req.EpayPartnerId != nil {
		config.EpayPartnerId = strings.TrimSpace(*req.EpayPartnerId)
	}
	if req.EpayKey != nil {
		config.EpayKey = strings.TrimSpace(*req.EpayKey)
	} else if req.EpayPartnerKey != nil {
		config.EpayKey = strings.TrimSpace(*req.EpayPartnerKey)
	}
	if req.EpayUrl != nil {
		config.EpayUrl = strings.TrimSpace(*req.EpayUrl)
	} else if req.EpayGatewayUrl != nil {
		config.EpayUrl = strings.TrimSpace(*req.EpayGatewayUrl)
	}

	if req.StripeApiSecret != nil {
		config.StripeApiSecret = strings.TrimSpace(*req.StripeApiSecret)
	}
	if req.StripeWebhookSecret != nil {
		config.StripeWebhookSecret = strings.TrimSpace(*req.StripeWebhookSecret)
	}
	if req.StripePriceId != nil {
		config.StripePriceId = strings.TrimSpace(*req.StripePriceId)
	}

	if req.CreemApiKey != nil {
		config.CreemApiKey = strings.TrimSpace(*req.CreemApiKey)
	}
	if req.CreemWebhookSecret != nil {
		config.CreemWebhookSecret = strings.TrimSpace(*req.CreemWebhookSecret)
	}
	if req.CreemTestMode != nil {
		config.CreemTestMode = *req.CreemTestMode
	}

	if req.WaffoMerchantId != nil {
		config.WaffoMerchantId = strings.TrimSpace(*req.WaffoMerchantId)
	}
	if req.WaffoApiKey != nil {
		config.WaffoApiKey = strings.TrimSpace(*req.WaffoApiKey)
	}
	if req.WaffoPrivateKey != nil {
		config.WaffoPrivateKey = strings.TrimSpace(*req.WaffoPrivateKey)
	}

	if err := model.UpdateResellerConfig(config); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Settings saved successfully", "data": config})
}

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
}

// ResellerCreateRedemption creates redemption codes using the reseller's available quota.
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
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Quota must be greater than 0"})
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

	if user.Quota < totalQuotaNeeded {
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": fmt.Sprintf("Insufficient quota. Needed: %d, Available: %d", totalQuotaNeeded, user.Quota),
		})
		return
	}

	// Deduct quota
	user.Quota -= totalQuotaNeeded
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

	// Refund quota
	var user model.User
	if err := tx.Where("id = ?", userId).First(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "User not found"})
		return
	}
	user.Quota += redemption.Quota
	if err := tx.Save(&user).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Failed to refund quota"})
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

