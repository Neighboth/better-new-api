package model

import (
	"strings"
)

type FineTuningJob struct {
	Id              int    `json:"id" gorm:"primaryKey;autoIncrement"`
	JobID           string `json:"job_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	UserId          int    `json:"user_id" gorm:"index;not null"`
	TokenId         int    `json:"token_id" gorm:"index"`
	Model           string `json:"model" gorm:"type:varchar(256);not null"`
	FineTunedModel  string `json:"fine_tuned_model" gorm:"type:varchar(256)"`
	TrainingFile    string `json:"training_file" gorm:"type:varchar(256);not null"`
	ValidationFile  string `json:"validation_file" gorm:"type:varchar(256)"`
	Hyperparameters string `json:"hyperparameters" gorm:"type:text"`
	Status          string `json:"status" gorm:"type:varchar(32);index;not null"` // queued | running | succeeded | failed | cancelled
	TrainedTokens   int64  `json:"trained_tokens"`
	Error            string  `json:"error" gorm:"type:text"`
	Provider          string  `json:"provider" gorm:"type:varchar(32)"`
	GPU               string  `json:"gpu" gorm:"type:varchar(64)"`
	Method            string  `json:"method" gorm:"type:varchar(32)"`
	Quantization      string  `json:"quantization" gorm:"type:varchar(32)"`
	TaskType          string  `json:"task_type" gorm:"type:varchar(32)"`
	RamTier           string  `json:"ram_tier" gorm:"type:varchar(16)"`
	Progress          int     `json:"progress"`
	ModelSource       string  `json:"model_source" gorm:"type:varchar(32)"`
	ModelSourceURL    string  `json:"model_source_url" gorm:"type:text"`
	DeploymentID      int     `json:"deployment_id"`
	DurationSeconds           int64   `json:"duration_seconds"`
	DatasetBytes              int64   `json:"dataset_bytes"`
	EstimatedCostUSD          float64 `json:"estimated_cost_usd"`
	BilledQuota               int64   `json:"billed_quota"`
	BilledSeconds             int64   `json:"billed_seconds"`
	CompatWarningAcknowledged bool    `json:"compat_warning_acknowledged"`
	CompatWarningReason       string  `json:"compat_warning_reason" gorm:"type:text"`
	CheckpointEpoch           int     `json:"checkpoint_epoch"`
	CheckpointStep            int     `json:"checkpoint_step"`
	AttemptCount              int     `json:"attempt_count"`
	EvalReportJSON            string  `json:"eval_report_json" gorm:"type:text"`
	TrainingFilePath          string  `json:"-" gorm:"type:text"`
	BaseModelFilePath         string  `json:"-" gorm:"type:text"`
	CheckpointPath            string  `json:"-" gorm:"type:text"`
	ModelFilePath             string  `json:"-" gorm:"type:text"`
	HasModelFile              bool    `json:"has_model_file" gorm:"-"`
	ServingStatus             string  `json:"serving_status" gorm:"type:varchar(32);index"` // "" | stopped | starting | running | sleeping | failed
	ServingDeploymentID       int     `json:"serving_deployment_id"`
	ServingProvider           string  `json:"serving_provider" gorm:"type:varchar(32)"`
	ServingGPU                string  `json:"serving_gpu" gorm:"type:varchar(64)"`
	ServingRamTier            string  `json:"serving_ram_tier" gorm:"type:varchar(16)"`
	ServingEndpointURL        string  `json:"-" gorm:"type:text"`
	ServingStartedAt          int64   `json:"serving_started_at" gorm:"bigint"`
	ServingLastBilledAt       int64   `json:"serving_last_billed_at" gorm:"bigint"`
	ServingLastRequestAt      int64   `json:"serving_last_request_at" gorm:"bigint"`
	ServingIdleTimeoutSeconds int     `json:"serving_idle_timeout_seconds"`
	ServingAutoWake           bool    `json:"serving_auto_wake"`
	ServingBilledSeconds      int64   `json:"serving_billed_seconds"`
	ServingBilledQuota        int64   `json:"serving_billed_quota"`
	ServingError              string  `json:"serving_error" gorm:"type:text"`
	ServingStopReason         string  `json:"serving_stop_reason" gorm:"type:varchar(64)"`
	CallbackToken        string `json:"-" gorm:"type:varchar(64)"`
	CreatedAt            int64  `json:"created_at" gorm:"bigint;index"`
	FinishedAt           int64  `json:"finished_at" gorm:"bigint"`
	ExpiresAt            int64  `json:"expires_at" gorm:"bigint;index"`
	ExpiryWarningSentAt  int64  `json:"expiry_warning_sent_at" gorm:"bigint"`
	FilesPurgedAt        int64  `json:"files_purged_at" gorm:"bigint"`
	UpdatedAt            int64  `json:"updated_at" gorm:"bigint"`
}

func CreateFineTuningJob(job *FineTuningJob) error {
	return DB.Create(job).Error
}

func GetFineTuningJobByJobID(jobID string) (*FineTuningJob, error) {
	var job FineTuningJob
	if err := DB.Where("job_id = ?", strings.TrimSpace(jobID)).First(&job).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func ListUserFineTuningJobs(userId int, limit int) ([]FineTuningJob, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	var jobs []FineTuningJob
	err := DB.Where("user_id = ?", userId).Order("id desc").Limit(limit).Find(&jobs).Error
	return jobs, err
}

// ExistsUserFineTunedModel reports whether userId already has a fine-tuning job
// using modelID (case-insensitive), optionally excluding excludeJobID.
func ExistsUserFineTunedModel(userId int, modelID string, excludeJobID string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(modelID))
	if userId <= 0 || trimmed == "" {
		return false
	}
	q := DB.Model(&FineTuningJob{}).Where("user_id = ? AND LOWER(fine_tuned_model) = ?", userId, trimmed)
	if strings.TrimSpace(excludeJobID) != "" {
		q = q.Where("job_id != ?", strings.TrimSpace(excludeJobID))
	}
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// ListUserActiveServingFineTuningJobs returns fine-tuned models currently actively
// hosted or sleeping with auto-wake for userId only. Never returns models belonging to other users.
func ListUserActiveServingFineTuningJobs(userId int) ([]FineTuningJob, error) {
	if userId <= 0 {
		return nil, nil
	}
	var jobs []FineTuningJob
	err := DB.Where(
		"user_id = ? AND ((serving_status = ? AND serving_endpoint_url != ?) OR (serving_status IN ? AND serving_auto_wake = ?))",
		userId, "running", "", []string{"sleeping", "starting"}, true,
	).
		Order("id desc").
		Find(&jobs).Error
	return jobs, err
}

// GetUserActiveServingFineTuningJobByModel resolves a private running (or auto-wake
// sleeping/starting) fine-tuned model exclusively for its owning user.
func GetUserActiveServingFineTuningJobByModel(userId int, fineTunedModel string) (*FineTuningJob, error) {
	if userId <= 0 {
		return nil, nil
	}
	trimmed := strings.TrimSpace(fineTunedModel)
	if trimmed == "" {
		return nil, nil
	}
	var job FineTuningJob
	err := DB.Where(
		"user_id = ? AND LOWER(fine_tuned_model) = ? AND ((serving_status = ? AND serving_endpoint_url != ?) OR (serving_status IN ? AND serving_auto_wake = ?))",
		userId, strings.ToLower(trimmed), "running", "", []string{"sleeping", "starting"}, true,
	).
		Order("id desc").
		First(&job).Error
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// ListAllActiveOrStartingServingFineTuningJobs returns every fine-tuned model
// currently provisioning or running so the background watchdog can sync status
// and bill/stop when user balance is exhausted.
func ListAllActiveOrStartingServingFineTuningJobs() ([]FineTuningJob, error) {
	var jobs []FineTuningJob
	err := DB.Where("serving_status IN ?", []string{"starting", "running"}).
		Order("id asc").
		Find(&jobs).Error
	return jobs, err
}

func ListAllFineTuningJobs(limit int) ([]FineTuningJob, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var jobs []FineTuningJob
	err := DB.Order("id desc").Limit(limit).Find(&jobs).Error
	return jobs, err
}

func ListFineTuningJobsExpiringSoon(warnHorizon, now int64) ([]FineTuningJob, error) {
	var jobs []FineTuningJob
	err := DB.Where("status = ? AND files_purged_at = 0 AND expiry_warning_sent_at = 0 AND expires_at > ? AND expires_at <= ?", "succeeded", now, warnHorizon).
		Find(&jobs).Error
	return jobs, err
}

func ListFineTuningJobsExpired(now int64) ([]FineTuningJob, error) {
	var jobs []FineTuningJob
	err := DB.Where("status = ? AND files_purged_at = 0 AND expires_at > 0 AND expires_at <= ?", "succeeded", now).
		Find(&jobs).Error
	return jobs, err
}

func UpdateFineTuningJobStatus(jobID string, fields map[string]any) error {
	return DB.Model(&FineTuningJob{}).Where("job_id = ?", strings.TrimSpace(jobID)).Updates(fields).Error
}

func DeleteFineTuningJob(jobID string, userId int) error {
	return DB.Where("job_id = ? AND user_id = ?", strings.TrimSpace(jobID), userId).Delete(&FineTuningJob{}).Error
}

func AdminDeleteFineTuningJob(jobID string) error {
	return DB.Where("job_id = ?", strings.TrimSpace(jobID)).Delete(&FineTuningJob{}).Error
}
