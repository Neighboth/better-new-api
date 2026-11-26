package controller

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestFineTuningWorkerUploadChunkReassemblesAtomically(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prevDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.FineTuningJob{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = prevDB
	})

	jobID := "ftjob-chunked-upload-test"
	t.Cleanup(func() {
		service.CleanupFineTuningStorageForUser(777, jobID)
	})

	job := &model.FineTuningJob{
		JobID:         jobID,
		UserId:        777,
		Model:         "HuggingFaceTB/SmolLM2-135M",
		Status:        "running",
		CallbackToken: "tok-chunk-secret",
	}
	require.NoError(t, model.CreateFineTuningJob(job))

	r := gin.New()
	r.Use(middleware.DecompressRequestMiddleware())
	r.POST("/api/deployments/fine_tuning/:job_id/upload_chunk", FineTuningWorkerUploadChunk)

	part1 := bytes.Repeat([]byte("A"), 64*1024)
	part2 := bytes.Repeat([]byte("B"), 32*1024)
	total := len(part1) + len(part2)

	// Chunk 1
	req1 := httptest.NewRequest(http.MethodPost, "/api/deployments/fine_tuning/"+jobID+"/upload_chunk?token=tok-chunk-secret&kind=artifact&upload_id=u1&offset=0&total_size=98304&final=0", bytes.NewReader(part1))
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code, w1.Body.String())

	// Chunk 2 (final)
	req2 := httptest.NewRequest(http.MethodPost, "/api/deployments/fine_tuning/"+jobID+"/upload_chunk?token=tok-chunk-secret&kind=artifact&upload_id=u1&offset=65536&total_size=98304&final=1&trained_tokens=12345", bytes.NewReader(part2))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code, w2.Body.String())

	updated, err := model.GetFineTuningJobByJobID(jobID)
	require.NoError(t, err)
	require.NotEmpty(t, updated.ModelFilePath)
	assert.Equal(t, int64(12345), updated.TrainedTokens)

	raw, err := os.ReadFile(updated.ModelFilePath)
	require.NoError(t, err)
	assert.Len(t, raw, total)
	assert.Equal(t, append(part1, part2...), raw)
}

func TestMapAcceleratorForProviderTranslatesCrossProviderGPUs(t *testing.T) {
	assert.Equal(t, "T4", mapAcceleratorForProvider("NVIDIA_TESLA_T4", "modal"))
	assert.Equal(t, "NvidiaTeslaT4", mapAcceleratorForProvider("NVIDIA_TESLA_T4", "kaggle"))
	assert.Equal(t, "NVIDIA_TESLA_T4", mapAcceleratorForProvider("T4", "colab"))
	assert.Equal(t, "NVIDIA_L4", mapAcceleratorForProvider("L4", "colab"))
	assert.Equal(t, "A100-80GB", mapAcceleratorForProvider("NVIDIA_TESLA_A100_80GB", "modal"))
	assert.Equal(t, "A100-80GB", mapAcceleratorForProvider("NVIDIA_TESLA_A100_80GB", "runpod"))
	assert.Equal(t, "", mapAcceleratorForProvider("TPU_V5E_1", "modal"))
	assert.Equal(t, "TpuV5E8", mapAcceleratorForProvider("TPU_V5E_1", "kaggle"))
}

func TestFreeInternalStudioOptimizerSkipsBillingAndRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", nil)
	c.Set(string(constant.ContextKeyFreeInternalRequest), true)

	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-4o",
		UsingGroup:      "default",
	}
	priceData, err := helper.ModelPriceHelper(c, info, 500, &types.TokenCountMeta{})
	require.NoError(t, err)
	assert.True(t, priceData.FreeModel, "Studio prompt optimizer must be marked FreeModel")
	assert.Equal(t, 0, priceData.QuotaToPreConsume, "Studio prompt optimizer must pre-consume 0 quota")

	assert.Nil(t, service.PreConsumeBilling(c, 999999, info))
	assert.NoError(t, service.SettleBilling(c, info, 999999))
}

func TestFineTuningDownloadTicketAndRangeStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prevDB := model.DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.FineTuningJob{}))
	model.DB = db
	t.Cleanup(func() { model.DB = prevDB })

	tmpDir := t.TempDir()
	zipPath := filepath.Join(tmpDir, "ftjob-dl-1.zip")
	payload := bytes.Repeat([]byte("0123456789ABCDEF"), 4096) // 64 KB
	require.NoError(t, os.WriteFile(zipPath, payload, 0o644))

	job := &model.FineTuningJob{
		JobID:         "ftjob-dl-1",
		UserId:        42,
		Model:         "HuggingFaceTB/SmolLM2-135M",
		Status:        "succeeded",
		ModelFilePath: zipPath,
	}
	require.NoError(t, model.CreateFineTuningJob(job))

	r := gin.New()
	r.POST("/api/user/fine_tuning/jobs/:job_id/download_ticket", func(c *gin.Context) {
		c.Set("id", 42)
		c.Set("role", common.RoleCommonUser)
		UserCreateFineTuningDownloadTicket(c)
	})
	r.GET("/api/fine_tuning/download/:job_id", DownloadFineTuningModelWithTicket)

	// Step 1: Request download ticket
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodPost, "/api/user/fine_tuning/jobs/ftjob-dl-1/download_ticket", nil))
	require.Equal(t, http.StatusOK, w1.Code)
	var ticketResp struct {
		Success bool `json:"success"`
		Data    struct {
			Ticket      string `json:"ticket"`
			Filename    string `json:"filename"`
			Size        int64  `json:"size"`
			DownloadURL string `json:"download_url"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(w1.Body.Bytes(), &ticketResp))
	require.True(t, ticketResp.Success)
	require.NotEmpty(t, ticketResp.Data.Ticket)
	assert.Equal(t, int64(len(payload)), ticketResp.Data.Size)
	assert.Equal(t, "ftjob-dl-1.zip", ticketResp.Data.Filename)

	// Step 2: Stream partial byte range directly with signed ticket (simulates browser/resume)
	reqRange := httptest.NewRequest(http.MethodGet, ticketResp.Data.DownloadURL, nil)
	reqRange.Header.Set("Range", "bytes=16-31")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, reqRange)
	assert.Equal(t, http.StatusPartialContent, w2.Code)
	assert.Equal(t, "no", w2.Header().Get("X-Accel-Buffering"))
	assert.Contains(t, w2.Header().Get("Content-Disposition"), "ftjob-dl-1.zip")
	assert.Equal(t, payload[16:32], w2.Body.Bytes())

	// Step 3: Invalid ticket rejected
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/api/fine_tuning/download/ftjob-dl-1?ticket=invalid", nil))
	assert.Equal(t, http.StatusUnauthorized, w3.Code)
}

func TestFineTuningServingPrivateIsolationAndAutoStopOnZeroBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prevDB := model.DB
	prevRedis := common.RedisEnabled
	common.RedisEnabled = false
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.FineTuningJob{}, &model.NotebookDeployment{}))
	model.DB = db
	t.Cleanup(func() {
		model.DB = prevDB
		common.RedisEnabled = prevRedis
	})

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	prevFT := common.OptionMap["model_deployment.fine_tuning.enabled"]
	prevServe := common.OptionMap["model_deployment.fine_tuning.serving_enabled"]
	prevPrices := common.OptionMap["model_deployment.fine_tuning.serving_gpu_multipliers"]
	common.OptionMap["model_deployment.fine_tuning.enabled"] = "true"
	common.OptionMap["model_deployment.fine_tuning.serving_enabled"] = "true"
	common.OptionMap["model_deployment.fine_tuning.serving_gpu_multipliers"] = `[{"id":"CPU","label":"CPU","kind":"cpu","price_per_second":0.01,"enabled":true}]`
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap["model_deployment.fine_tuning.enabled"] = prevFT
		common.OptionMap["model_deployment.fine_tuning.serving_enabled"] = prevServe
		common.OptionMap["model_deployment.fine_tuning.serving_gpu_multipliers"] = prevPrices
		common.OptionMapRWMutex.Unlock()
	})

	// Create mock upstream OpenAI server for User A's fine-tuned model
	var seenDirectPath string
	mockUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/_jobs" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		seenDirectPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-ft","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"private-ft-ok"},"finish_reason":"stop"}]}`))
	}))
	defer mockUpstream.Close()

	userA := &model.User{Id: 101, Username: "alice", AffCode: "ALICE1", Quota: 50000, Status: common.UserStatusEnabled, Group: "default"}
	userB := &model.User{Id: 202, Username: "bob", AffCode: "BOB2", Quota: 50000, Status: common.UserStatusEnabled, Group: "default"}
	require.NoError(t, db.Create(userA).Error)
	require.NoError(t, db.Create(userB).Error)

	// Private NotebookDeployment must NOT appear in public ListNotebookDeployments("")
	privDep := &model.NotebookDeployment{
		OwnerUserID:     userA.Id,
		FineTuningJobID: "ftjob-priv-1",
		Provider:        "colab",
		Name:            "ft-serve-priv",
		ModelID:         "ft:Qwen/Qwen2.5-0.5B:alice",
		ModelRepo:       "Qwen/Qwen2.5-0.5B",
		TaskType:        "text",
		Accelerator:     "CPU",
		Status:          "running",
		EndpointURL:     mockUpstream.URL,
	}
	require.NoError(t, model.CreateNotebookDeployment(privDep))
	publicDeps, err := model.ListNotebookDeployments("")
	require.NoError(t, err)
	assert.Empty(t, publicDeps, "Private user fine-tuned deployments must never leak into public deployment list")

	now := time.Now().Unix()
	ftJob := &model.FineTuningJob{
		JobID:               "ftjob-priv-1",
		UserId:              userA.Id,
		Model:               "Qwen/Qwen2.5-0.5B",
		FineTunedModel:      "ft:Qwen/Qwen2.5-0.5B:alice",
		Status:              "succeeded",
		ServingStatus:       "running",
		ServingDeploymentID: privDep.Id,
		ServingProvider:     "colab",
		ServingGPU:          "CPU",
		ServingRamTier:      "normal",
		ServingEndpointURL:  mockUpstream.URL,
		ServingStartedAt:    now - 10,
		ServingLastBilledAt: now - 10,
	}
	require.NoError(t, model.CreateFineTuningJob(ftJob))

	// Isolation: User A sees it, User B does NOT
	aActive, err := model.ListUserActiveServingFineTuningJobs(userA.Id)
	require.NoError(t, err)
	require.Len(t, aActive, 1)
	bActive, err := model.ListUserActiveServingFineTuningJobs(userB.Id)
	require.NoError(t, err)
	assert.Empty(t, bActive)

	// User B cannot route to User A's fine-tuned model
	wB := httptest.NewRecorder()
	cB, _ := gin.CreateTestContext(wB)
	cB.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{"model":"ft:Qwen/Qwen2.5-0.5B:alice","messages":[{"role":"user","content":"hi"}]}`)))
	assert.False(t, service.TryHandleUserFineTunedRelay(cB, userB.Id, "ft:Qwen/Qwen2.5-0.5B:alice"))

	// User A routes cleanly from Playground /pg/chat/completions -> upstream /v1/chat/completions
	wA := httptest.NewRecorder()
	cA, _ := gin.CreateTestContext(wA)
	cA.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", bytes.NewReader([]byte(`{"model":"ft:Qwen/Qwen2.5-0.5B:alice","messages":[{"role":"user","content":"hi"}]}`)))
	cA.Request.Header.Set("Content-Type", "application/json")
	assert.True(t, service.TryHandleUserFineTunedRelay(cA, userA.Id, "ft:Qwen/Qwen2.5-0.5B:alice"))
	assert.Equal(t, "/v1/chat/completions", seenDirectPath, "Playground /pg/ path must normalize to /v1/")
	assert.Equal(t, http.StatusOK, wA.Code)
	assert.Contains(t, wA.Body.String(), "private-ft-ok")

	// Deplete User A balance -> watchdog bills remaining quota and immediately stops hosting
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", userA.Id).Update("quota", 1000).Error)
	require.NoError(t, model.UpdateFineTuningJobStatus(ftJob.JobID, map[string]any{
		"serving_last_billed_at": now - 60, // 60s * $0.01/s * 500000 = 300000 quota > 1000 remaining
	}))

	tickFineTuningServingWatchdog(context.Background())

	stoppedJob, err := model.GetFineTuningJobByJobID(ftJob.JobID)
	require.NoError(t, err)
	assert.Equal(t, "stopped", stoppedJob.ServingStatus)
	assert.Equal(t, "insufficient_balance", stoppedJob.ServingStopReason)
	assert.Empty(t, stoppedJob.ServingEndpointURL)

	// Immediately disappears from User A's active models list
	aAfter, err := model.ListUserActiveServingFineTuningJobs(userA.Id)
	require.NoError(t, err)
	assert.Empty(t, aAfter)

	// User A balance never goes negative
	var freshA model.User
	require.NoError(t, db.First(&freshA, userA.Id).Error)
	assert.GreaterOrEqual(t, freshA.Quota, 0)
}
