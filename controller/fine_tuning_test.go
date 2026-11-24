package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupFineTuningTestDB(t *testing.T) {
	t.Helper()
	prevDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.FineTuningJob{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = prevDB
	})
}

func TestFineTuningLifecycle(t *testing.T) {
	setupFineTuningTestDB(t)
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("id", 42)
		c.Set("token_id", 7)
		c.Next()
	})
	r.POST("/v1/fine_tuning/jobs", CreateFineTuningJob)
	r.GET("/v1/fine_tuning/jobs", ListFineTuningJobs)
	r.GET("/v1/fine_tuning/jobs/:fine_tuning_job_id", RetrieveFineTuningJob)
	r.POST("/v1/fine_tuning/jobs/:fine_tuning_job_id/cancel", CancelFineTuningJob)
	r.POST("/api/deployments/fine_tuning/:job_id/callback", FineTuningWorkerCallback)

	// Disabled by default -> 403
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap["model_deployment.fine_tuning.enabled"] = "false"
	common.OptionMapRWMutex.Unlock()

	req := httptest.NewRequest(http.MethodPost, "/v1/fine_tuning/jobs", bytes.NewBufferString(`{"model":"Qwen/Qwen2.5-0.5B","training_file":"file-123"}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Enable fine-tuning
	common.OptionMapRWMutex.Lock()
	common.OptionMap["model_deployment.fine_tuning.enabled"] = "true"
	common.OptionMapRWMutex.Unlock()

	// Create job
	req = httptest.NewRequest(http.MethodPost, "/v1/fine_tuning/jobs", bytes.NewBufferString(`{"model":"Qwen/Qwen2.5-0.5B","training_file":"file-123"}`))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	jobID, ok := created["id"].(string)
	require.True(t, ok)
	assert.Contains(t, jobID, "ftjob-")
	assert.Equal(t, "queued", created["status"])

	// Worker callback updates status to succeeded
	stored, err := model.GetFineTuningJobByJobID(jobID)
	require.NoError(t, err)
	cbBody, _ := json.Marshal(map[string]any{
		"token":            stored.CallbackToken,
		"status":           "succeeded",
		"fine_tuned_model": "ft:Qwen/Qwen2.5-0.5B:custom",
		"trained_tokens":   12345,
	})
	req = httptest.NewRequest(http.MethodPost, "/api/deployments/fine_tuning/"+jobID+"/callback", bytes.NewReader(cbBody))
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Retrieve updated job
	req = httptest.NewRequest(http.MethodGet, "/v1/fine_tuning/jobs/"+jobID, nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var retrieved map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &retrieved))
	assert.Equal(t, "succeeded", retrieved["status"])
	assert.Equal(t, "ft:Qwen/Qwen2.5-0.5B:custom", retrieved["fine_tuned_model"])
	assert.Equal(t, float64(12345), retrieved["trained_tokens"])
}
