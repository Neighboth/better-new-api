package controller

import (
	"context"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var channelOAuthService = service.NewChannelOAuthService()

type channelOAuthStartRequest struct {
	Provider string `json:"provider" binding:"required"`
}

type channelOAuthExchangeRequest struct {
	Provider   string `json:"provider" binding:"required"`
	SessionID string `json:"session_id" binding:"required"`
	Callback  string `json:"callback" binding:"required"`
}

func StartChannelOAuth(c *gin.Context) {
	var req channelOAuthStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider != service.ChannelOAuthCodex && provider != service.ChannelOAuthAntigravity {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "unsupported channel OAuth provider"})
		return
	}
	flow, err := channelOAuthService.Generate(provider)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": flow})
}

func ExchangeChannelOAuth(c *gin.Context) {
	var req channelOAuthExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider != service.ChannelOAuthCodex && provider != service.ChannelOAuthAntigravity {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "unsupported channel OAuth provider"})
		return
	}
	credential, err := channelOAuthService.Exchange(context.WithoutCancel(c.Request.Context()), provider, req.SessionID, req.Callback)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	channelType := constant.ChannelTypeCodex
	if provider == service.ChannelOAuthAntigravity {
		channelType = constant.ChannelTypeAntigravity
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"provider": provider, "channel_type": channelType, "key": credential}})
}
