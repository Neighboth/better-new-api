package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service/captcha"

	"github.com/gin-gonic/gin"
)

// GetAltchaChallenge issues a signed proof-of-work challenge for the ALTCHA
// widget. The widget fetches this endpoint through its `challengeurl`
// attribute, so the body is the bare challenge document.
func GetAltchaChallenge(c *gin.Context) {
	key := common.AltchaSigningKey()

	challenge, err := captcha.GenerateAltcha(key, 0, 0)
	if err != nil {
		common.SysError("failed to generate altcha challenge: " + err.Error())
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "failed to generate challenge",
		})
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, challenge)
}

// GetImageCaptcha issues a new self-hosted image captcha. The codes it
// hands out are one-way pictures that any client must prove it can read;
// allowing anyone to fetch one is safe, otherwise fallback scenarios fail
// whenever the administrator disables the global captcha.
func GetImageCaptcha(c *gin.Context) {
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
