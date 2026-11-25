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
	TaskType          string  `json:"task_type" gorm:"type:varchar(32)"`
	RamTier           string  `json:"ram_tier" gorm:"type:varchar(16)"`
	Progress          int     `json:"progress"`
	ModelSource       string  `json:"model_source" gorm:"type:varchar(32)"`
	ModelSourceURL    string  `json:"model_source_url" gorm:"type:text"`
	DeploymentID      int     `json:"deployment_id"`
	DurationSeconds     int64   `json:"duration_seconds"`
	DatasetBytes        int64   `json:"dataset_bytes"`
	EstimatedCostUSD    float64 `json:"estimated_cost_usd"`
	BilledQuota         int64   `json:"billed_quota"`
	TrainingFilePath    string  `json:"-" gorm:"type:text"`
	BaseModelFilePath   string  `json:"-" gorm:"type:text"`
	CheckpointPath      string  `json:"-" gorm:"type:text"`
	ModelFilePath       string  `json:"-" gorm:"type:text"`
	HasModelFile        bool    `json:"has_model_file" gorm:"-"`
	CallbackToken       string  `json:"-" gorm:"type:varchar(64)"`
	CreatedAt           int64   `json:"created_at" gorm:"bigint;index"`
	FinishedAt          int64   `json:"finished_at" gorm:"bigint"`
	ExpiresAt           int64   `json:"expires_at" gorm:"bigint;index"`
	ExpiryWarningSentAt int64   `json:"expiry_warning_sent_at" gorm:"bigint"`
	FilesPurgedAt       int64   `json:"files_purged_at" gorm:"bigint"`
	UpdatedAt           int64   `json:"updated_at" gorm:"bigint"`
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
