package controller

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestInjectApkConfig(t *testing.T) {
	apkPath := filepath.Join("..", "worker", "binaries", "byog-worker.apk")
	rawApk, err := os.ReadFile(apkPath)
	if err != nil {
		t.Skipf("byog-worker.apk not found at %s, skipping", apkPath)
	}

	payload := map[string]string{
		"server_url":  "https://example.com",
		"system_name": "TestSystem",
		"app_name":    "TestSystem BYOG",
		"favicon":     "https://example.com/favicon.ico",
	}
	cfgJson, err := json.Marshal(payload)
	assert.NoError(t, err)

	// Valid injection
	customized, err := injectApkConfig(rawApk, cfgJson)
	assert.NoError(t, err)
	assert.NotNil(t, customized)
	assert.Greater(t, len(customized), len(rawApk))

	// Corrupt input safety tests - must return error without panicking
	_, err = injectApkConfig([]byte{}, cfgJson)
	assert.Error(t, err)

	_, err = injectApkConfig([]byte("not an apk file at all"), cfgJson)
	assert.Error(t, err)
}

func TestResolveNodeGeoIP(t *testing.T) {
	// Private / local IPs must fallback safely to Turkey / Istanbul rather than (0, 0)
	country, city, lat, lon := resolveNodeGeoIP("127.0.0.1")
	assert.True(t, country == "Turkey" || country == "Türkiye", "Expected Turkey/Türkiye but got %s", country)
	assert.NotEmpty(t, city)
	assert.Greater(t, lat, 35.0)
	assert.Less(t, lat, 43.0)
	assert.Greater(t, lon, 25.0)
	assert.Less(t, lon, 45.0)

	country, city, lat, lon = resolveNodeGeoIP("192.168.1.1")
	assert.True(t, country == "Turkey" || country == "Türkiye")
	assert.NotEmpty(t, city)
	assert.NotZero(t, lat)
	assert.NotZero(t, lon)

	country, city, lat, lon = resolveNodeGeoIP("10.0.0.15")
	assert.True(t, country == "Turkey" || country == "Türkiye")
	assert.NotZero(t, lat)
	assert.NotZero(t, lon)
}

func TestDownloadByogWorkerEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.SystemName = "TestPlatform"
	common.Favicon = "/favicon.ico"

	router := gin.New()
	router.GET("/api/byog/worker/download/:platform/:arch", DownloadByogWorker)

	// 1. Test Android APK
	req, _ := http.NewRequest(http.MethodGet, "/api/byog/worker/download/android/apk", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/vnd.android.package-archive", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "byog-worker.apk")
	assert.Greater(t, w.Body.Len(), 10000)

	// 2. Test Windows AMD64
	//
	// The dashboard hands out the program of the installation. The shortcut of
	// the client is asked for on its own.
	req, _ = http.NewRequest(http.MethodGet, "/api/byog/worker/download/windows/amd64", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "BYOG Worker Kurulumu.exe")

	// 3. Test the shortcut of the client of Windows
	req, _ = http.NewRequest(http.MethodGet, "/api/byog/worker/download/windows/amd64?shortcut=1", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "TestPlatform BYOG.lnk")
	assert.Greater(t, w.Body.Len(), 100)
	assert.Contains(t, string(w.Body.Bytes()), "powershell.exe")

	// 3. Test Linux AMD64
	req, _ = http.NewRequest(http.MethodGet, "/api/byog/worker/download/linux/amd64", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "byog-worker-linux-amd64")

	// 4. Test Darwin ARM64
	req, _ = http.NewRequest(http.MethodGet, "/api/byog/worker/download/darwin/arm64", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "byog-worker-darwin-arm64")

	// 5. Test Nonexistent Platform
	req, _ = http.NewRequest(http.MethodGet, "/api/byog/worker/download/solaris/sparc", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestDynamicApkBrandingAndFavicon(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/worker/download/:platform/:arch", DownloadByogWorker)

	// Test with custom company name
	common.SystemName = "Acme AI"
	common.Favicon = "/favicon.ico"

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/worker/download/android/apk", nil)
	req.Host = "acme-ai.example.com"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	apkBytes := w.Body.Bytes()
	assert.Greater(t, len(apkBytes), 100000)

	// Verify that the injected APK has the custom config
	eocdSig := []byte{0x50, 0x4b, 0x05, 0x06}
	eocdPos := bytes.LastIndex(apkBytes, eocdSig)
	assert.NotEqual(t, -1, eocdPos)

	cdOffset := binary.LittleEndian.Uint32(apkBytes[eocdPos+16 : eocdPos+20])
	magic := apkBytes[cdOffset-16 : cdOffset]
	assert.Equal(t, "APK Sig Block 42", string(magic))

	blockSize := binary.LittleEndian.Uint64(apkBytes[cdOffset-24 : cdOffset-16])
	blockStart := int64(cdOffset) - 8 - int64(blockSize)
	offset := blockStart + 8

	var foundPayload string
	for offset < int64(cdOffset)-24 {
		pairLen := binary.LittleEndian.Uint64(apkBytes[offset : offset+8])
		pairId := binary.LittleEndian.Uint32(apkBytes[offset+8 : offset+12])
		valBytes := apkBytes[offset+12 : offset+8+int64(pairLen)]
		if pairId == 0x71777777 {
			foundPayload = string(valBytes)
			break
		}
		offset += 8 + int64(pairLen)
	}

	assert.NotEmpty(t, foundPayload)
	var parsed map[string]string
	err := json.Unmarshal([]byte(foundPayload), &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "http://acme-ai.example.com", parsed["server_url"])
	assert.Equal(t, "Acme AI", parsed["system_name"])
	assert.Equal(t, "Acme AI BYOG", parsed["app_name"])
	assert.Equal(t, "http://acme-ai.example.com/favicon.ico", parsed["favicon"])
	assert.NotEmpty(t, parsed["favicon_base64"])
	assert.True(t, strings.HasPrefix(parsed["favicon_base64"], "data:image/"))

	// Test default system name when empty
	common.SystemName = ""
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req)
	assert.Equal(t, http.StatusOK, w2.Code)
}

func TestGetBrandingResolution(t *testing.T) {
	// Case 1: SystemName_tr in OptionMap takes precedence over empty or default
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap["SystemName_tr"] = "Türkçe Yapay Zeka"
	common.OptionMapRWMutex.Unlock()
	common.SystemName = "New API"

	sysName, appName, _, _ := getBranding("http://127.0.0.1:3000")
	assert.Equal(t, "Türkçe Yapay Zeka", sysName)
	assert.Equal(t, "Türkçe Yapay Zeka BYOG", appName)

	// Clean up
	common.OptionMapRWMutex.Lock()
	delete(common.OptionMap, "SystemName_tr")
	common.OptionMapRWMutex.Unlock()
}
