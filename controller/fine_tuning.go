package controller

import (
	"crypto/subtle"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type createFineTuningJobRequest struct {
	Model           string         `json:"model"`
	TrainingFile    string         `json:"training_file"`
	ValidationFile  string         `json:"validation_file"`
	Hyperparameters map[string]any `json:"hyperparameters"`
	Suffix          string         `json:"suffix"`
	Provider        string         `json:"provider"`
}

func fineTuningDisabledResponse(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{
		"error": gin.H{
			"message": "Fine-tuning is disabled on this server",
			"type":    "invalid_request_error",
			"code":    "fine_tuning_disabled",
		},
	})
}

func CreateFineTuningJob(c *gin.Context) {
	if !service.IsFineTuningEnabled() {
		fineTuningDisabledResponse(c)
		return
	}
	var req createFineTuningJobRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid JSON body", "type": "invalid_request_error"}})
		return
	}
	req.Model = strings.TrimSpace(req.Model)
	req.TrainingFile = strings.TrimSpace(req.TrainingFile)
	if req.Model == "" || req.TrainingFile == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "model and training_file are required", "type": "invalid_request_error"}})
		return
	}

	userId := c.GetInt("id")
	tokenId := c.GetInt("token_id")
	hyperJSON := "{}"
	if req.Hyperparameters != nil {
		if b, err := common.Marshal(req.Hyperparameters); err == nil {
			hyperJSON = string(b)
		}
	}
	now := time.Now().Unix()
	job := &model.FineTuningJob{
		JobID:           service.GenerateFineTuningJobID(),
		UserId:          userId,
		TokenId:         tokenId,
		Model:           req.Model,
		TrainingFile:    req.TrainingFile,
		ValidationFile:  strings.TrimSpace(req.ValidationFile),
		Hyperparameters: hyperJSON,
		Status:          "queued",
		Provider:        strings.TrimSpace(req.Provider),
		CallbackToken:   common.GetUUID(),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := model.CreateFineTuningJob(job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error(), "type": "server_error"}})
		return
	}
	c.JSON(http.StatusOK, service.FormatOpenAIFineTuningJob(job))
}

func ListFineTuningJobs(c *gin.Context) {
	if !service.IsFineTuningEnabled() {
		fineTuningDisabledResponse(c)
		return
	}
	userId := c.GetInt("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	jobs, err := model.ListUserFineTuningJobs(userId, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error(), "type": "server_error"}})
		return
	}
	data := make([]map[string]any, 0, len(jobs))
	for i := range jobs {
		data = append(data, service.FormatOpenAIFineTuningJob(&jobs[i]))
	}
	c.JSON(http.StatusOK, gin.H{
		"object":   "list",
		"data":     data,
		"has_more": false,
	})
}

func RetrieveFineTuningJob(c *gin.Context) {
	if !service.IsFineTuningEnabled() {
		fineTuningDisabledResponse(c)
		return
	}
	jobID := c.Param("fine_tuning_job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "Fine-tuning job not found", "type": "invalid_request_error"}})
		return
	}
	userId := c.GetInt("id")
	if job.UserId != userId && c.GetInt("role") < common.RoleAdminUser {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "Fine-tuning job not found", "type": "invalid_request_error"}})
		return
	}
	c.JSON(http.StatusOK, service.FormatOpenAIFineTuningJob(job))
}

func CancelFineTuningJob(c *gin.Context) {
	if !service.IsFineTuningEnabled() {
		fineTuningDisabledResponse(c)
		return
	}
	jobID := c.Param("fine_tuning_job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "Fine-tuning job not found", "type": "invalid_request_error"}})
		return
	}
	userId := c.GetInt("id")
	if job.UserId != userId && c.GetInt("role") < common.RoleAdminUser {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "Fine-tuning job not found", "type": "invalid_request_error"}})
		return
	}
	if job.Status == "succeeded" || job.Status == "failed" || job.Status == "cancelled" {
		c.JSON(http.StatusOK, service.FormatOpenAIFineTuningJob(job))
		return
	}
	_ = service.ApplyFineTuningWorkerCallback(job, "cancelled", "", "", job.TrainedTokens)
	updated, _ := model.GetFineTuningJobByJobID(jobID)
	if updated != nil {
		job = updated
	}
	c.JSON(http.StatusOK, service.FormatOpenAIFineTuningJob(job))
}

type fineTuningWorkerCallbackRequest struct {
	Token          string `json:"token"`
	Status         string `json:"status"`
	FineTunedModel string `json:"fine_tuned_model"`
	TrainedTokens  int64  `json:"trained_tokens"`
	Error          string `json:"error"`
}

func FineTuningWorkerCallback(c *gin.Context) {
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Job not found"})
		return
	}
	var req fineTuningWorkerCallbackRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid payload"})
		return
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		token = strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	}
	if job.CallbackToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(job.CallbackToken)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid callback token"})
		return
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	switch status {
	case "queued", "running", "succeeded", "failed", "cancelled":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid status"})
		return
	}
	if err := service.ApplyFineTuningWorkerCallback(job, status, req.FineTunedModel, req.Error, req.TrainedTokens); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UserListFineTuningJobs(c *gin.Context) {
	userId := c.GetInt("id")
	jobs, err := model.ListUserFineTuningJobs(userId, 100)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	for i := range jobs {
		if jobs[i].ModelFilePath != "" {
			if _, statErr := os.Stat(jobs[i].ModelFilePath); statErr == nil {
				jobs[i].HasModelFile = true
			}
		}
		if !jobs[i].HasModelFile && jobs[i].Status == "succeeded" {
			jobs[i].HasModelFile = true
		}
	}
	pricePerSec, gpuMultipliers := service.GetFineTuningPricing()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":          service.IsFineTuningEnabled(),
			"price_per_second": pricePerSec,
			"gpu_multipliers":  gpuMultipliers,
			"jobs":             jobs,
		},
	})
}

func UserCreateFineTuningJob(c *gin.Context) {
	if !service.IsFineTuningEnabled() {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Fine-tuning is disabled in System Settings"})
		return
	}
	userId := c.GetInt("id")
	var baseModel, trainingFile, trainingData, gpu, provider, suffix string
	epochs := 3
	estimatedSec := int64(300)

	ct := c.GetHeader("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		baseModel = strings.TrimSpace(c.PostForm("model"))
		trainingFile = strings.TrimSpace(c.PostForm("training_file"))
		trainingData = strings.TrimSpace(c.PostForm("training_data"))
		gpu = strings.TrimSpace(c.PostForm("gpu"))
		provider = strings.TrimSpace(c.PostForm("provider"))
		suffix = strings.TrimSpace(c.PostForm("suffix"))
		if ep, err := strconv.Atoi(c.PostForm("epochs")); err == nil && ep > 0 && ep <= 50 {
			epochs = ep
		}
		if ds, err := strconv.ParseInt(c.PostForm("estimated_seconds"), 10, 64); err == nil && ds > 0 {
			estimatedSec = ds
		}
	} else {
		var req struct {
			Model            string `json:"model"`
			TrainingFile     string `json:"training_file"`
			TrainingData     string `json:"training_data"`
			GPU              string `json:"gpu"`
			Provider         string `json:"provider"`
			Suffix           string `json:"suffix"`
			Epochs           int    `json:"epochs"`
			EstimatedSeconds int64  `json:"estimated_seconds"`
		}
		if err := common.DecodeJson(c.Request.Body, &req); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid request payload"})
			return
		}
		baseModel = strings.TrimSpace(req.Model)
		trainingFile = strings.TrimSpace(req.TrainingFile)
		trainingData = strings.TrimSpace(req.TrainingData)
		gpu = strings.TrimSpace(req.GPU)
		provider = strings.TrimSpace(req.Provider)
		suffix = strings.TrimSpace(req.Suffix)
		if req.Epochs > 0 && req.Epochs <= 50 {
			epochs = req.Epochs
		}
		if req.EstimatedSeconds > 0 {
			estimatedSec = req.EstimatedSeconds
		}
	}

	if baseModel == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Base model is required"})
		return
	}
	if gpu == "" {
		gpu = "NVIDIA_TESLA_T4"
	}

	jobID := service.GenerateFineTuningJobID()
	storageDir := service.FineTuningStorageDir(jobID)
	var trainingFilePath string

	if strings.HasPrefix(ct, "multipart/form-data") {
		if fh, fErr := c.FormFile("file"); fErr == nil && fh != nil {
			if err := os.MkdirAll(storageDir, 0o755); err == nil {
				dst := filepath.Join(storageDir, "train.jsonl")
				if src, oErr := fh.Open(); oErr == nil {
					if out, cErr := os.Create(dst); cErr == nil {
						_, _ = io.Copy(out, io.LimitReader(src, 50<<20))
						_ = out.Close()
						trainingFilePath = dst
						if trainingFile == "" {
							trainingFile = fh.Filename
						}
					}
					_ = src.Close()
				}
			}
		}
	}
	if trainingFilePath == "" && trainingData != "" {
		if err := os.MkdirAll(storageDir, 0o755); err == nil {
			dst := filepath.Join(storageDir, "train.jsonl")
			if os.WriteFile(dst, []byte(trainingData), 0o644) == nil {
				trainingFilePath = dst
				if trainingFile == "" {
					trainingFile = "inline-dataset.jsonl"
				}
			}
		}
	}
	if trainingFile == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Training dataset file or content is required"})
		return
	}

	hyperBytes, _ := common.Marshal(map[string]any{
		"n_epochs": epochs,
		"suffix":   suffix,
	})
	now := time.Now().Unix()
	estCost := service.EstimateFineTuningCostUSD(gpu, estimatedSec)
	ftName := "ft:" + baseModel
	if suffix != "" {
		ftName = ftName + ":" + suffix
	} else {
		ftName = ftName + ":" + jobID[len(jobID)-6:]
	}

	job := &model.FineTuningJob{
		JobID:            jobID,
		UserId:           userId,
		Model:            baseModel,
		FineTunedModel:   ftName,
		TrainingFile:     trainingFile,
		TrainingFilePath: trainingFilePath,
		Hyperparameters:  string(hyperBytes),
		Status:           "queued",
		Provider:         provider,
		GPU:              gpu,
		DurationSeconds:  estimatedSec,
		EstimatedCostUSD: estCost,
		CallbackToken:    common.GetUUID(),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := model.CreateFineTuningJob(job); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": job})
}

func UserCancelFineTuningJob(c *gin.Context) {
	userId := c.GetInt("id")
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || (job.UserId != userId && c.GetInt("role") < common.RoleAdminUser) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Fine-tuning job not found"})
		return
	}
	if job.Status != "succeeded" && job.Status != "failed" && job.Status != "cancelled" {
		_ = service.ApplyFineTuningWorkerCallback(job, "cancelled", "", "", job.TrainedTokens)
	}
	updated, _ := model.GetFineTuningJobByJobID(jobID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": updated})
}

func UserDownloadFineTuningModel(c *gin.Context) {
	userId := c.GetInt("id")
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || (job.UserId != userId && c.GetInt("role") < common.RoleAdminUser) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Fine-tuning job not found"})
		return
	}
	if job.ModelFilePath != "" {
		if _, statErr := os.Stat(job.ModelFilePath); statErr == nil {
			c.FileAttachment(job.ModelFilePath, filepath.Base(job.ModelFilePath))
			return
		}
	}
	manifest, _ := common.Marshal(map[string]any{
		"job_id":           job.JobID,
		"base_model":       job.Model,
		"fine_tuned_model": job.FineTunedModel,
		"status":           job.Status,
		"gpu":              job.GPU,
		"trained_tokens":   job.TrainedTokens,
		"created_at":       job.CreatedAt,
		"finished_at":      job.FinishedAt,
	})
	c.Header("Content-Disposition", `attachment; filename="`+job.JobID+`-manifest.json"`)
	c.Data(http.StatusOK, "application/json; charset=utf-8", manifest)
}

func UserDeleteFineTuningJob(c *gin.Context) {
	userId := c.GetInt("id")
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || (job.UserId != userId && c.GetInt("role") < common.RoleAdminUser) {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Fine-tuning job not found"})
		return
	}
	if err := model.DeleteFineTuningJob(jobID, job.UserId); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	service.CleanupFineTuningStorage(jobID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}
