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

type FineTuningGPUOption struct {
	ID             string  `json:"id"`
	Label          string  `json:"label"`
	PricePerSecond float64 `json:"price_per_second"`
	Enabled        bool    `json:"enabled"`
}

var defaultFineTuningGPUs = []FineTuningGPUOption{
	{ID: "CPU", Label: "CPU", PricePerSecond: 0.0005, Enabled: true},
	{ID: "NVIDIA_TESLA_T4", Label: "NVIDIA T4 (16GB)", PricePerSecond: 0.0015, Enabled: true},
	{ID: "NVIDIA_L4", Label: "NVIDIA L4 (24GB)", PricePerSecond: 0.0025, Enabled: true},
	{ID: "NVIDIA_A10G", Label: "NVIDIA A10G (24GB)", PricePerSecond: 0.0030, Enabled: true},
	{ID: "NVIDIA_TESLA_A100", Label: "NVIDIA A100 (80GB)", PricePerSecond: 0.0060, Enabled: true},
	{ID: "NVIDIA_H100_80GB", Label: "NVIDIA H100 (80GB)", PricePerSecond: 0.0100, Enabled: true},
}

func GetFineTuningGPUOptions() []FineTuningGPUOption {
	common.OptionMapRWMutex.RLock()
	rawPrice := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.price_per_second"])
	rawConfig := strings.TrimSpace(common.OptionMap["model_deployment.fine_tuning.gpu_multipliers"])
	common.OptionMapRWMutex.RUnlock()

	basePrice := 0.002
	if rawPrice != "" {
		if parsed, err := strconv.ParseFloat(rawPrice, 64); err == nil && parsed >= 0 {
			basePrice = parsed
		}
	}

	// New structured row format: [{"id":"...","label":"...","price_per_second":0.0015,"enabled":true}]
	if strings.HasPrefix(rawConfig, "[") {
		var rows []FineTuningGPUOption
		if common.UnmarshalJsonStr(rawConfig, &rows) == nil && len(rows) > 0 {
			for i := range rows {
				if rows[i].Label == "" {
					rows[i].Label = rows[i].ID
				}
				if rows[i].PricePerSecond < 0 {
					rows[i].PricePerSecond = 0
				}
			}
			return rows
		}
	}

	// Legacy multiplier map format fallback: {"CPU":0.5,"NVIDIA_TESLA_T4":1,...}
	if strings.HasPrefix(rawConfig, "{") {
		var legacy map[string]float64
		if common.UnmarshalJsonStr(rawConfig, &legacy) == nil && len(legacy) > 0 {
			result := make([]FineTuningGPUOption, 0, len(defaultFineTuningGPUs))
			seen := make(map[string]bool, len(defaultFineTuningGPUs))
			for _, def := range defaultFineTuningGPUs {
				seen[def.ID] = true
				if mult, ok := legacy[def.ID]; ok {
					result = append(result, FineTuningGPUOption{
						ID:             def.ID,
						Label:          def.Label,
						PricePerSecond: basePrice * mult,
						Enabled:        mult >= 0,
					})
				} else {
					result = append(result, def)
				}
			}
			for id, mult := range legacy {
				if !seen[id] {
					result = append(result, FineTuningGPUOption{
						ID:             id,
						Label:          id,
						PricePerSecond: basePrice * mult,
						Enabled:        mult >= 0,
					})
				}
			}
			return result
		}
	}

	out := make([]FineTuningGPUOption, len(defaultFineTuningGPUs))
	copy(out, defaultFineTuningGPUs)
	return out
}

func GetFineTuningPricing() (float64, map[string]float64) {
	opts := GetFineTuningGPUOptions()
	mults := make(map[string]float64, len(opts))
	for _, o := range opts {
		if o.Enabled {
			mults[o.ID] = o.PricePerSecond
		}
	}
	return 1.0, mults
}

func FindEnabledFineTuningGPU(gpuID string) (*FineTuningGPUOption, bool) {
	trimmed := strings.TrimSpace(gpuID)
	for _, opt := range GetFineTuningGPUOptions() {
		if opt.ID == trimmed && opt.Enabled {
			copied := opt
			return &copied, true
		}
	}
	return nil, false
}

func EstimateFineTuningCostUSD(gpu string, durationSeconds int64) float64 {
	if durationSeconds <= 0 {
		return 0
	}
	trimmed := strings.TrimSpace(gpu)
	for _, opt := range GetFineTuningGPUOptions() {
		if opt.ID == trimmed {
			return opt.PricePerSecond * float64(durationSeconds)
		}
	}
	return 0.002 * float64(durationSeconds)
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
