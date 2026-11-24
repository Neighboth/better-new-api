package service

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func IsFineTuningEnabled() bool {
	common.OptionMapRWMutex.RLock()
	enabled := common.OptionMap["model_deployment.fine_tuning.enabled"] == "true"
	common.OptionMapRWMutex.RUnlock()
	return enabled
}

func GetFineTuningPricing() (float64, map[string]float64) {
	common.OptionMapRWMutex.RLock()
	rawPrice := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.price_per_second"])
	rawMult := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.gpu_multipliers"])
	common.OptionMapRWMutex.RUnlock()

	pricePerSec := 0.002
	if rawPrice != "" {
		if parsed, err := strconv.ParseFloat(rawPrice, 64); err == nil && parsed >= 0 {
			pricePerSec = parsed
		}
	}
	multipliers := map[string]float64{
		"CPU":               0.5,
		"NVIDIA_TESLA_T4":   1.0,
		"NVIDIA_L4":         1.5,
		"NVIDIA_TESLA_A100": 3.0,
		"NVIDIA_H100_80GB":  5.0,
	}
	if rawMult != "" {
		var parsed map[string]float64
		if common.UnmarshalJsonStr(rawMult, &parsed) == nil && len(parsed) > 0 {
			multipliers = parsed
		}
	}
	return pricePerSec, multipliers
}

func EstimateFineTuningCostUSD(gpu string, durationSeconds int64) float64 {
	if durationSeconds <= 0 {
		return 0
	}
	baseRate, mults := GetFineTuningPricing()
	mult := 1.0
	if m, ok := mults[strings.TrimSpace(gpu)]; ok && m > 0 {
		mult = m
	}
	return baseRate * mult * float64(durationSeconds)
}

func FineTuningStorageDir(jobID string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, jobID)
	return filepath.Join("data", "fine_tuning", safe)
}

func CleanupFineTuningStorage(jobID string) {
	dir := FineTuningStorageDir(jobID)
	_ = os.RemoveAll(dir)
}

func GenerateFineTuningJobID() string {
	buf := make([]byte, 12)
	_, _ = rand.Read(buf)
	return "ftjob-" + hex.EncodeToString(buf)
}

func FormatOpenAIFineTuningJob(job *model.FineTuningJob) map[string]any {
	var hyper map[string]any
	if strings.TrimSpace(job.Hyperparameters) != "" {
		_ = common.UnmarshalJsonStr(job.Hyperparameters, &hyper)
	}
	if hyper == nil {
		hyper = map[string]any{}
	}
	var fineTunedModel any
	if strings.TrimSpace(job.FineTunedModel) != "" {
		fineTunedModel = job.FineTunedModel
	}
	var validationFile any
	if strings.TrimSpace(job.ValidationFile) != "" {
		validationFile = job.ValidationFile
	}
	var finishedAt any
	if job.FinishedAt > 0 {
		finishedAt = job.FinishedAt
	}
	var errObj any
	if strings.TrimSpace(job.Error) != "" {
		errObj = map[string]any{
			"code":    "fine_tuning_error",
			"message": job.Error,
		}
	}
	return map[string]any{
		"object":           "fine_tuning.job",
		"id":               job.JobID,
		"model":            job.Model,
		"created_at":       job.CreatedAt,
		"finished_at":      finishedAt,
		"fine_tuned_model": fineTunedModel,
		"organization_id":  "org-newapi",
		"result_files":     []string{},
		"status":           job.Status,
		"validation_file":  validationFile,
		"training_file":    job.TrainingFile,
		"hyperparameters":  hyper,
		"trained_tokens":   job.TrainedTokens,
		"error":            errObj,
	}
}

func ApplyFineTuningWorkerCallback(job *model.FineTuningJob, status, fineTunedModel, errMsg string, trainedTokens int64) error {
	now := time.Now().Unix()
	updates := map[string]any{
		"status":     status,
		"updated_at": now,
	}
	if strings.TrimSpace(fineTunedModel) != "" {
		updates["fine_tuned_model"] = strings.TrimSpace(fineTunedModel)
	}
	if strings.TrimSpace(errMsg) != "" {
		updates["error"] = strings.TrimSpace(errMsg)
	}
	if trainedTokens > 0 {
		updates["trained_tokens"] = trainedTokens
	}
	if status == "succeeded" || status == "failed" || status == "cancelled" {
		updates["finished_at"] = now
	}
	return model.UpdateFineTuningJobStatus(job.JobID, updates)
}
