package service

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFineTuningGlobalStorageQuotaCap(t *testing.T) {
	tmpRoot := t.TempDir()
	prevOverride := fineTuningRootStorageDirOverride
	fineTuningRootStorageDirOverride = tmpRoot
	t.Cleanup(func() {
		fineTuningRootStorageDirOverride = prevOverride
	})

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	prevMaxGB := common.OptionMap["model_deployment.fine_tuning.max_storage_gb"]
	common.OptionMap["model_deployment.fine_tuning.max_storage_gb"] = "0.001" // ~1.04 MB
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap["model_deployment.fine_tuning.max_storage_gb"] = prevMaxGB
		common.OptionMapRWMutex.Unlock()
	})

	jobDir := FineTuningStorageDirForUser(1, "ftjob-cap")
	require.NoError(t, os.MkdirAll(jobDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "artifact.zip"), bytes.Repeat([]byte("A"), 800_000), 0o644))

	allowedSmall, used, maxB := CheckFineTuningStorageQuota(100_000)
	assert.True(t, allowedSmall)
	assert.Equal(t, int64(800_000), used)
	assert.Greater(t, maxB, int64(1_000_000))

	allowedBig, _, _ := CheckFineTuningStorageQuota(500_000)
	assert.False(t, allowedBig, "upload pushing total over global fine-tuning storage cap must be rejected")

	// Unlimited when set to 0
	common.OptionMapRWMutex.Lock()
	common.OptionMap["model_deployment.fine_tuning.max_storage_gb"] = "0"
	common.OptionMapRWMutex.Unlock()
	allowedUnlimited, _, maxZero := CheckFineTuningStorageQuota(999_999_999)
	assert.True(t, allowedUnlimited)
	assert.Equal(t, int64(0), maxZero)
}

func TestTryProxyDeploymentStreamsSSEWithFlush(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		time.Sleep(15 * time.Millisecond)
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := []byte(`{"model":"benim-modelim","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	ok := tryProxyDeployment(c, model.NotebookDeployment{
		Provider:    "colab",
		EndpointURL: upstream.URL,
	}, body)
	require.True(t, ok)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "no", w.Header().Get("X-Accel-Buffering"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Contains(t, w.Body.String(), `"content":"Hello"`)
	assert.Contains(t, w.Body.String(), "data: [DONE]")
}
