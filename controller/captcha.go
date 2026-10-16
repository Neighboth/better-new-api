package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service/captcha"

	"github.com/gin-gonic/gin"
)

// GetImageCaptcha issues a new self-hosted image captcha. Available when the
// administrator selected the "image" captcha type, or when the captcha
// fallback chain is enabled (image is the last-resort fallback).
func GetImageCaptcha(c *gin.Context) {
	// Admins can always fetch one (the settings page has a captcha test
	// button); everyone else only when image is the active provider or the
	// fallback chain may reach it.
	isAdmin := c.GetInt("role") >= common.RoleAdminUser
	if common.GetEffectiveCaptchaType() != common.CaptchaTypeImage && !common.CaptchaFallbackEnabled() && !isAdmin {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "image captcha is not enabled",
		})
		return
	}
	id, img, err := captcha.Generate()
	if err != nil {
		common.SysError("failed to generate image captcha: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "failed to generate captcha",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"captcha_id": id,
			"image":      "data:image/png;base64," + img,
		},
	})
}
