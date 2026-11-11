package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service/captcha"
	"github.com/gin-gonic/gin"
)

type captchaVerifyResponse struct {
	Success bool `json:"success"`
}

func verifyCaptchaToken(verifyURL string, secret string, response string, remoteIP string) (bool, error) {
	rawRes, err := http.PostForm(verifyURL, url.Values{
		"secret":   {secret},
		"response": {response},
		"remoteip": {remoteIP},
	})
	if err != nil {
		return false, err
	}
	defer rawRes.Body.Close()
	var res captchaVerifyResponse
	if err := common.DecodeJson(rawRes.Body, &res); err != nil {
		return false, err
	}
	return res.Success, nil
}

type mtCaptchaVerifyResponse struct {
	Success bool   `json:"success"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// verifyMTCaptcha posts the token of the widget to the MTCaptcha validation
// service together with the private key of the installation.
func verifyMTCaptcha(privateKey, token, remoteIP string) (bool, error) {
	if strings.TrimSpace(privateKey) == "" || strings.TrimSpace(token) == "" {
		return false, nil
	}
	body, err := json.Marshal(map[string]string{
		"privatekey": privateKey,
		"token":      token,
		"remoteip":   remoteIP,
	})
	if err != nil {
		return false, err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"https://service.mtcaptcha.com/mtcv1/api/validate",
		bytes.NewReader(body),
	)
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var parsed mtCaptchaVerifyResponse
	if err := common.DecodeJson(resp.Body, &parsed); err != nil {
		return false, err
	}
	if parsed.Success || strings.EqualFold(parsed.Status, "OK") {
		return true, nil
	}
	return false, nil
}

func abortWithCaptchaError(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{
		"success": false,
		"message": message,
	})
	c.Abort()
}

// TurnstileCheck keeps its historical name but now enforces whichever captcha
// provider the administrator configured: Cloudflare Turnstile, Google
// reCAPTCHA, hCaptcha or the self-hosted image captcha. When captcha is off
// the request passes through untouched.
//
// The frontend sends the token in the "turnstile" query parameter and,
// optionally, the provider that issued it in "captcha_provider". When the
// admin enabled the captcha fallback chain, any configured provider is
// accepted so a fallback challenge verifies against its own keys.
func TurnstileCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		captchaType := common.GetEffectiveCaptchaType()
		if captchaType == common.CaptchaTypeOff {
			c.Next()
			return
		}

		token := c.Query("turnstile")
		if token == "" {
			abortWithCaptchaError(c, i18n.T(c, i18n.MsgCaptchaTokenRequired))
			return
		}

		provider := c.Query("captcha_provider")
		if provider == "" {
			provider = captchaType
		}
		if provider != captchaType {
			if !common.CaptchaFallbackEnabled() || !common.IsCaptchaProviderConfigured(provider) {
				abortWithCaptchaError(c, i18n.T(c, i18n.MsgCaptchaProviderMismatch))
				return
			}
		}

		if provider == common.CaptchaTypeImage {
			// The image captcha packs "captchaID:answer" into the same field so
			// every existing caller keeps a single token parameter.
			parts := strings.SplitN(token, ":", 2)
			if len(parts) != 2 || !captcha.Verify(parts[0], parts[1]) {
				abortWithCaptchaError(c, i18n.T(c, i18n.MsgCaptchaImageInvalid))
				return
			}
			c.Next()
			return
		}

		var verifyURL, secret string
		switch provider {
		case common.CaptchaTypeTurnstile:
			verifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
			secret = common.TurnstileSecretKey
		case common.CaptchaTypeRecaptcha:
			verifyURL = "https://www.google.com/recaptcha/api/siteverify"
			secret = common.RecaptchaSecretKey
		case common.CaptchaTypeHCaptcha:
			verifyURL = "https://hcaptcha.com/siteverify"
			secret = common.HCaptchaSecretKey
		case common.CaptchaTypeAltcha:
			// ALTCHA verifies locally: the payload carries the HMAC signature of
			// the challenge this installation issued.
			if !captcha.VerifyAltcha(token, common.AltchaSigningKey()) {
				abortWithCaptchaError(c, i18n.T(c, i18n.MsgCaptchaVerifyFailed))
				return
			}
			c.Next()
			return
		case common.CaptchaTypeMTCaptcha:
			ok, err := verifyMTCaptcha(common.MTCaptchaPrivateKey, token, c.ClientIP())
			if err != nil {
				common.SysLog(err.Error())
				abortWithCaptchaError(c, i18n.T(c, i18n.MsgCaptchaVerifyFailed))
				return
			}
			if !ok {
				abortWithCaptchaError(c, i18n.T(c, i18n.MsgCaptchaVerifyFailed))
				return
			}
			c.Next()
			return
		default:
			abortWithCaptchaError(c, i18n.T(c, i18n.MsgCaptchaProviderMismatch))
			return
		}
		if secret == "" {
			abortWithCaptchaError(c, i18n.T(c, i18n.MsgCaptchaProviderMismatch))
			return
		}

		ok, err := verifyCaptchaToken(verifyURL, secret, token, c.ClientIP())
		if err != nil {
			common.SysLog(err.Error())
			abortWithCaptchaError(c, i18n.T(c, i18n.MsgCaptchaVerifyFailed))
			return
		}
		if !ok {
			abortWithCaptchaError(c, i18n.T(c, i18n.MsgCaptchaVerifyFailed))
			return
		}
		c.Next()
	}
}
