package controller

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/finetunescript"
	"github.com/QuantumNous/new-api/pkg/notebookscript"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRound10TestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.FineTuningJob{}, &model.NotebookDeployment{}))
	prevDB := model.DB
	model.DB = db
	t.Cleanup(func() {
		model.DB = prevDB
	})
}

func TestValidateCustomFineTunedModelID(t *testing.T) {
	valid := []string{"benim-modelim", "acme_v2.1", "org:chat-135m", "ab"}
	for _, id := range valid {
		got, err := service.ValidateCustomFineTunedModelID(id)
		require.NoError(t, err, "expected valid id %q", id)
		assert.Equal(t, id, got)
	}

	invalid := []string{"", "a", "has space", "bad/slash", "-leading", "trailing-"}
	for _, id := range invalid {
		_, err := service.ValidateCustomFineTunedModelID(id)
		assert.Error(t, err, "expected error for invalid id %q", id)
	}
}

func TestQLoRAQuantizationScriptAndVRAM(t *testing.T) {
	profiles := []string{"NF4", "FP4", "Q2_K", "Q4_K_M", "Q4_L", "Q4_0", "INT8", "Q8_0"}
	for _, q := range profiles {
		script := finetunescript.Generate(finetunescript.Config{
			JobID:          "ftjob-test1",
			FineTunedModel: "benim-modelim",
			BaseModel:      "HuggingFaceTB/SmolLM2-135M-Instruct",
			TaskType:       "text",
			Method:         "qlora",
			Quantization:   q,
			Epochs:         2,
		})
		assert.Contains(t, script, `QUANTIZATION = "`+q+`"`)
		assert.Contains(t, script, "def _build_qlora_bnb_config")
		assert.Contains(t, script, "def run_post_training_eval")
		assert.Contains(t, script, `"eval_report": eval_report`)
	}

	// Verify VRAM scaling reflects quantization bit width (INT8 > NF4 > Q2_K)
	assert.Greater(t, service.QLoRAQuantizationFactor("INT8"), service.QLoRAQuantizationFactor("NF4"))
	assert.Greater(t, service.QLoRAQuantizationFactor("NF4"), service.QLoRAQuantizationFactor("Q2_K"))

	int8Compat := service.CheckFineTuningHardwareCompat(service.FineTuningEstimateInput{
		GPU:          "NVIDIA_TESLA_T4",
		Method:       "qlora",
		Quantization: "INT8",
		ModelName:    "Qwen/Qwen2.5-14B-Instruct",
	})
	q2Compat := service.CheckFineTuningHardwareCompat(service.FineTuningEstimateInput{
		GPU:          "NVIDIA_TESLA_T4",
		Method:       "qlora",
		Quantization: "Q2_K",
		ModelName:    "Qwen/Qwen2.5-14B-Instruct",
	})
	assert.Greater(t, int8Compat.RequiredVRAMGB, q2Compat.RequiredVRAMGB)
}

func TestNotebookScriptPreloadAndRealStreaming(t *testing.T) {
	script := notebookscript.Generate(notebookscript.Config{
		Provider:               "colab",
		DeploymentName:         "ft-serve-test",
		ModelRepo:              "HuggingFaceTB/SmolLM2-135M-Instruct",
		TaskType:               "text",
		Accelerator:            "NVIDIA_TESLA_T4",
		FineTunedModelID:       "benim-modelim",
		FineTunedArtifactURL:   "https://example.com/artifact.zip",
		FineTunedArtifactToken: "secret-tok",
		FineTuningMethod:       "lora",
	})
	assert.Contains(t, script, "TextIteratorStreamer")
	assert.Contains(t, script, "def _preload_on_startup()")
	assert.Contains(t, script, "def _load_causal_fast(")
	assert.Contains(t, script, "torch.inference_mode()")
	assert.Contains(t, script, `"ready": bool(globals().get("_model_ready", True))`)
	assert.Contains(t, script, `if hdata.get("ready", True):`)
	assert.Contains(t, script, `"X-Accel-Buffering": "no"`)
}

func TestFineTuningDownloadTicket24HTTLAndRangeSlide(t *testing.T) {
	setupRound10TestDB(t)
	gin.SetMode(gin.TestMode)

	tmpZip := filepath.Join(t.TempDir(), "ftjob-dl.zip")
	payload := bytes.Repeat([]byte("ZIPDATA"), 1024)
	require.NoError(t, os.WriteFile(tmpZip, payload, 0o644))

	job := &model.FineTuningJob{
		JobID:          "ftjob-dl",
		UserId:         42,
		Model:          "SmolLM2-135M",
		FineTunedModel: "benim-modelim",
		TrainingFile:   "train.jsonl",
		Status:         "succeeded",
		ModelFilePath:  tmpZip,
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}
	require.NoError(t, model.CreateFineTuningJob(job))

	// Issue signed download ticket
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	c1.Request = httptest.NewRequest(http.MethodPost, "/api/user/fine_tuning/jobs/ftjob-dl/download_ticket", nil)
	c1.Params = gin.Params{{Key: "job_id", Value: "ftjob-dl"}}
	c1.Set("id", 42)
	c1.Set("role", common.RoleCommonUser)
	UserCreateFineTuningDownloadTicket(c1)
	require.Equal(t, http.StatusOK, w1.Code)

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Ticket     string `json:"ticket"`
			TTLSeconds int64  `json:"ttl_seconds"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(w1.Body.Bytes(), &resp))
	require.True(t, resp.Success)
	assert.Equal(t, int64(86400), resp.Data.TTLSeconds)
	require.NotEmpty(t, resp.Data.Ticket)

	// Perform Range download request and verify HTTP 206 Partial Content
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	req2 := httptest.NewRequest(http.MethodGet, "/api/fine_tuning/download/ftjob-dl?ticket="+resp.Data.Ticket, nil)
	req2.Header.Set("Range", "bytes=0-13")
	c2.Request = req2
	c2.Params = gin.Params{{Key: "job_id", Value: "ftjob-dl"}}
	DownloadFineTuningModelWithTicket(c2)
	assert.Equal(t, http.StatusPartialContent, w2.Code)
	assert.Equal(t, "ZIPDATAZIPDATA", w2.Body.String())
}

func TestFineTuningSleepingAutoWakeListingIsolation(t *testing.T) {
	setupRound10TestDB(t)
	now := time.Now().Unix()

	sleepingAutoWake := &model.FineTuningJob{
		JobID:                     "ftjob-sleep-1",
		UserId:                    10,
		Model:                     "SmolLM2-135M",
		FineTunedModel:            "benim-modelim",
		TrainingFile:              "train.jsonl",
		Status:                    "succeeded",
		ServingStatus:             "sleeping",
		ServingAutoWake:           true,
		ServingIdleTimeoutSeconds: 300,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	stoppedManual := &model.FineTuningJob{
		JobID:           "ftjob-stop-1",
		UserId:          10,
		Model:           "SmolLM2-135M",
		FineTunedModel:  "stopped-model",
		TrainingFile:    "train.jsonl",
		Status:          "succeeded",
		ServingStatus:   "stopped",
		ServingAutoWake: false,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	require.NoError(t, model.CreateFineTuningJob(sleepingAutoWake))
	require.NoError(t, model.CreateFineTuningJob(stoppedManual))

	// Owning user sees sleeping auto-wake model, not manually stopped model
	list10, err := model.ListUserActiveServingFineTuningJobs(10)
	require.NoError(t, err)
	require.Len(t, list10, 1)
	assert.Equal(t, "benim-modelim", list10[0].FineTunedModel)

	found, err := model.GetUserActiveServingFineTuningJobByModel(10, "benim-modelim")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "sleeping", found.ServingStatus)

	// Other user MUST NOT see user 10's private model
	list99, err := model.ListUserActiveServingFineTuningJobs(99)
	require.NoError(t, err)
	assert.Empty(t, list99)
	other, err := model.GetUserActiveServingFineTuningJobByModel(99, "benim-modelim")
	assert.Error(t, err)
	assert.Nil(t, other)
}

func TestStudioDiscordGateConfigAndEnforcement(t *testing.T) {
	setupRound10TestDB(t)
	gin.SetMode(gin.TestMode)

	// Save & restore global options + Discord OAuth setting + mock Discord API URL
	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = map[string]string{}
	}
	prevBotEn := common.OptionMap["discord_bot.enabled"]
	prevBotTok := common.OptionMap["discord_bot.token"]
	prevModels := common.OptionMap["playground_setting.studio_discord_gate_models"]
	prevInvite := common.OptionMap["playground_setting.studio_discord_gate_invite_url"]
	prevGuild := common.OptionMap["playground_setting.studio_discord_gate_guild_id"]
	common.OptionMapRWMutex.Unlock()

	prevOAuth := *system_setting.GetDiscordSettings()
	prevBaseURL := discordAPIBaseURL
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap["discord_bot.enabled"] = prevBotEn
		common.OptionMap["discord_bot.token"] = prevBotTok
		common.OptionMap["playground_setting.studio_discord_gate_models"] = prevModels
		common.OptionMap["playground_setting.studio_discord_gate_invite_url"] = prevInvite
		common.OptionMap["playground_setting.studio_discord_gate_guild_id"] = prevGuild
		common.OptionMapRWMutex.Unlock()
		*system_setting.GetDiscordSettings() = prevOAuth
		discordAPIBaseURL = prevBaseURL
	})

	memberPresent := false
	mockDiscord := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bot test-bot-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/guilds/999888/members/discord-u1") {
			if memberPresent {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"user":{"id":"discord-u1"}}`))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer mockDiscord.Close()
	discordAPIBaseURL = mockDiscord.URL

	// Prerequisite check: bot enabled but OAuth disabled => gate MUST be inactive
	common.OptionMapRWMutex.Lock()
	common.OptionMap["discord_bot.enabled"] = "true"
	common.OptionMap["discord_bot.token"] = "test-bot-token"
	common.OptionMap["playground_setting.studio_discord_gate_models"] = "veo-3, sora-2"
	common.OptionMap["playground_setting.studio_discord_gate_invite_url"] = "https://discord.gg/test"
	common.OptionMap["playground_setting.studio_discord_gate_guild_id"] = "999888"
	common.OptionMapRWMutex.Unlock()

	oauthCfg := prevOAuth
	oauthCfg.Enabled = false
	oauthCfg.ClientId = "client-123"
	*system_setting.GetDiscordSettings() = oauthCfg
	assert.False(t, GetStudioDiscordGateConfig().Active)

	// Enable Discord OAuth too => gate becomes active
	oauthCfg.Enabled = true
	*system_setting.GetDiscordSettings() = oauthCfg
	cfg := GetStudioDiscordGateConfig()
	assert.True(t, cfg.Active)
	assert.True(t, cfg.IsModelGated("veo-3"))
	assert.True(t, cfg.IsModelGated("SORA-2"))
	assert.False(t, cfg.IsModelGated("dall-e-3"))

	// Create user WITHOUT linked Discord account
	user := &model.User{Id: 77, Username: "studio-user", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)

	// Unlinked user MUST be blocked on gated model with 403 STUDIO_DISCORD_GATE_REQUIRED
	wBlock1 := httptest.NewRecorder()
	cBlock1, _ := gin.CreateTestContext(wBlock1)
	cBlock1.Request = httptest.NewRequest(http.MethodPost, "/pg/videos/generations", nil)
	cBlock1.Set("id", 77)
	assert.False(t, enforceStudioDiscordGate(cBlock1, "veo-3"))
	assert.Equal(t, http.StatusForbidden, wBlock1.Code)
	assert.Contains(t, wBlock1.Body.String(), "STUDIO_DISCORD_GATE_REQUIRED")

	// Link Discord account, but user NOT yet in guild => still blocked
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 77).Update("discord_id", "discord-u1").Error)
	wBlock2 := httptest.NewRecorder()
	cBlock2, _ := gin.CreateTestContext(wBlock2)
	cBlock2.Request = httptest.NewRequest(http.MethodPost, "/pg/videos/generations", nil)
	cBlock2.Set("id", 77)
	assert.False(t, enforceStudioDiscordGate(cBlock2, "veo-3"))
	assert.Equal(t, http.StatusForbidden, wBlock2.Code)

	// User joins Discord server => immediately allowed!
	memberPresent = true
	wAllow := httptest.NewRecorder()
	cAllow, _ := gin.CreateTestContext(wAllow)
	cAllow.Request = httptest.NewRequest(http.MethodPost, "/pg/videos/generations", nil)
	cAllow.Set("id", 77)
	assert.True(t, enforceStudioDiscordGate(cAllow, "veo-3"))

	// User leaves Discord server => immediately blocked again!
	memberPresent = false
	wLeft := httptest.NewRecorder()
	cLeft, _ := gin.CreateTestContext(wLeft)
	cLeft.Request = httptest.NewRequest(http.MethodPost, "/pg/videos/generations", nil)
	cLeft.Set("id", 77)
	assert.False(t, enforceStudioDiscordGate(cLeft, "veo-3"))
	assert.Equal(t, http.StatusForbidden, wLeft.Code)

	// Admin removes condition => allowed unconditionally!
	common.OptionMapRWMutex.Lock()
	common.OptionMap["playground_setting.studio_discord_gate_models"] = ""
	common.OptionMapRWMutex.Unlock()
	wCleared := httptest.NewRecorder()
	cCleared, _ := gin.CreateTestContext(wCleared)
	cCleared.Request = httptest.NewRequest(http.MethodPost, "/pg/videos/generations", nil)
	cCleared.Set("id", 77)
	assert.True(t, enforceStudioDiscordGate(cCleared, "veo-3"))

	_ = context.Background()
}
