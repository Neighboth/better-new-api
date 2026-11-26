package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

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
