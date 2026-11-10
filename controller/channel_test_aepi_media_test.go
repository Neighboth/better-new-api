package controller

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAepiChannelTestSubmitsRealJobsAndReturnsMediaURL(t *testing.T) {
	t.Run("image model polls job until done and includes output URL", func(t *testing.T) {
		var pollCount atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/v1/jobs" {
				var req map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				require.Equal(t, "image", req["operation"])
				require.Equal(t, "gpt-image-2.5-flare", req["model"])
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"id":"job_img_1","status":"queued"}`))
				return
			}
			if r.Method == http.MethodGet && r.URL.Path == "/v1/jobs/job_img_1" {
				n := pollCount.Add(1)
				w.WriteHeader(http.StatusOK)
				if n == 1 {
					_, _ = w.Write([]byte(`{"id":"job_img_1","status":"running","progress":40}`))
					return
				}
				_, _ = w.Write([]byte(`{"id":"job_img_1","status":"done","progress":100,"output":{"url":"/files/out-job_img_1.jpg"}}`))
				return
			}
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}))
		defer srv.Close()

		ch := &model.Channel{
			Type:    constant.ChannelTypeAepi,
			Name:    "Aepi Test",
			Key:     "sk_test_key",
			BaseURL: common.GetPointer(srv.URL),
		}
		res := testAepiChannel(context.Background(), ch, 1, "gpt-image-2.5-flare", "", time.Now())
		require.NoError(t, res.localErr)
		require.Nil(t, res.newAPIError)
		require.Contains(t, res.message, srv.URL+"/files/out-job_img_1.jpg")
		require.Contains(t, res.message, "görsel bağlantısı")
	})

	t.Run("video model submits create job and returns failure when upstream fails", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/v1/jobs" {
				var req map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				require.Equal(t, "create", req["operation"])
				require.Equal(t, "seedance-2.0-fast", req["model"])
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(`{"id":"job_vid_1","status":"queued"}`))
				return
			}
			if r.Method == http.MethodGet && r.URL.Path == "/v1/jobs/job_vid_1" {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"job_vid_1","status":"error","error":"provider moderation rejected"}`))
				return
			}
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}))
		defer srv.Close()

		ch := &model.Channel{
			Type:    constant.ChannelTypeAepi,
			Key:     "sk_test_key",
			BaseURL: common.GetPointer(srv.URL),
		}
		res := testAepiChannel(context.Background(), ch, 1, "seedance-2.0-fast", "", time.Now())
		require.Error(t, res.localErr)
		require.Contains(t, res.localErr.Error(), "provider moderation rejected")
	})

	t.Run("rescale uploads source PNG first then submits rescale job", func(t *testing.T) {
		var uploaded atomic.Bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/uploads") {
				body, _ := io.ReadAll(r.Body)
				require.Greater(t, len(body), 10)
				uploaded.Store(true)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"upload_abc123"}`))
				return
			}
			if r.Method == http.MethodPost && r.URL.Path == "/v1/jobs" {
				var req map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
				require.Equal(t, "rescale", req["operation"])
				require.Equal(t, "upload_abc123", req["source"])
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"id":"job_res_1","status":"done","output":{"url":"https://cdn.example.com/rescaled.png"}}`))
				return
			}
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
		}))
		defer srv.Close()

		ch := &model.Channel{
			Type:    constant.ChannelTypeAepi,
			Key:     "sk_test_key",
			BaseURL: common.GetPointer(srv.URL),
		}
		res := testAepiChannel(context.Background(), ch, 1, "rescale", "", time.Now())
		require.NoError(t, res.localErr)
		require.True(t, uploaded.Load())
		require.Contains(t, res.message, "https://cdn.example.com/rescaled.png")
	})
}

func TestExtractMediaURLFromTestResponseAndServeCachedMedia(t *testing.T) {
	// Case 1: OpenAI hosted URL
	url := extractMediaURLFromTestResponse([]byte(`{"created":1,"data":[{"url":"https://img.example.com/cat.png"}]}`))
	require.Equal(t, "https://img.example.com/cat.png", url)

	// Case 2: Video URL
	vidUrl := extractMediaURLFromTestResponse([]byte(`{"data":[{"video_url":"https://vid.example.com/waves.mp4"}]}`))
	require.Equal(t, "https://vid.example.com/waves.mp4", vidUrl)

	// Case 3: Base64 PNG stored & served cleanly via GetChannelTestMedia
	pngBytes := buildAepiTestPNG()
	b64Payload, _ := json.Marshal(map[string]any{
		"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(pngBytes)}},
	})
	mediaPath := extractMediaURLFromTestResponse(b64Payload)
	require.True(t, strings.HasPrefix(mediaPath, "/api/test-media/"))

	token := strings.TrimPrefix(mediaPath, "/api/test-media/")
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: token}}
	GetChannelTestMedia(c)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "image/png", w.Header().Get("Content-Type"))
	require.Equal(t, pngBytes, w.Body.Bytes())

	// Case 4: Empty media response returns ""
	require.Equal(t, "", extractMediaURLFromTestResponse([]byte(`{"created":1,"data":[{}]}`)))
}

func TestGetTopUpInfoDirectLinkCryptoAndWaffoRequirePayMethods(t *testing.T) {
	confirmPaymentComplianceForTest(t)

	origPayMethods := operation_setting.PayMethods
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	origDirectUrl := common.OptionMap["DirectLinkUrl"]
	origCryptoWallets := common.OptionMap["CryptoWallets"]
	origEnableDirect := common.OptionMap["EnableDirectLinkTopup"]
	origEnableCrypto := common.OptionMap["EnableCryptoTopup"]
	common.OptionMapRWMutex.Unlock()
	origWaffoEnabled := setting.WaffoEnabled
	origWaffoSandbox := setting.WaffoSandbox
	origWaffoKey := setting.WaffoApiKey
	origWaffoPriv := setting.WaffoPrivateKey
	origWaffoCert := setting.WaffoPublicCert
	t.Cleanup(func() {
		operation_setting.PayMethods = origPayMethods
		common.OptionMapRWMutex.Lock()
		common.OptionMap["DirectLinkUrl"] = origDirectUrl
		common.OptionMap["CryptoWallets"] = origCryptoWallets
		common.OptionMap["EnableDirectLinkTopup"] = origEnableDirect
		common.OptionMap["EnableCryptoTopup"] = origEnableCrypto
		common.OptionMapRWMutex.Unlock()
		setting.WaffoEnabled = origWaffoEnabled
		setting.WaffoSandbox = origWaffoSandbox
		setting.WaffoApiKey = origWaffoKey
		setting.WaffoPrivateKey = origWaffoPriv
		setting.WaffoPublicCert = origWaffoCert
	})

	common.OptionMapRWMutex.Lock()
	common.OptionMap["DirectLinkUrl"] = "https://pay.example.com/checkout"
	common.OptionMap["CryptoWallets"] = `[{"id":"w1","network":"TRC20 USDT","address":"TAbc"}]`
	common.OptionMap["EnableDirectLinkTopup"] = "false"
	common.OptionMap["EnableCryptoTopup"] = "false"
	common.OptionMapRWMutex.Unlock()
	setting.WaffoEnabled = false
	setting.WaffoSandbox = false
	setting.WaffoApiKey = "k"
	setting.WaffoPrivateKey = "p"
	setting.WaffoPublicCert = "c"

	// 1. Without PayMethods entries, NONE of Direct Link / Crypto / Waffo should be enabled
	operation_setting.PayMethods = []map[string]string{{"name": "Alipay", "type": "alipay"}}
	require.False(t, isDirectLinkTopUpEnabled())
	require.False(t, isCryptoTopUpEnabled())
	require.False(t, isWaffoTopUpEnabled())

	// 2. Adding direct_link, crypto, and waffo to PayMethods enables them without toggles!
	operation_setting.PayMethods = []map[string]string{
		{"name": "Harici Öde", "type": "direct_link"},
		{"name": "Kripto", "type": "crypto"},
		{"name": "Waffo", "type": "waffo"},
	}
	require.True(t, isDirectLinkTopUpEnabled())
	require.True(t, isCryptoTopUpEnabled())
	require.True(t, isWaffoTopUpEnabled())

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	GetTopUpInfo(c)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	require.Equal(t, true, resp.Data["enable_direct_link_topup"])
	require.Equal(t, true, resp.Data["enable_crypto_topup"])
	require.Equal(t, true, resp.Data["enable_waffo_topup"])
	methods, ok := resp.Data["pay_methods"].([]any)
	require.True(t, ok)
	require.Len(t, methods, 3)
}
