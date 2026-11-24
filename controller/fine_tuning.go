package controller

import (
	"crypto/subtle"
	"net/http"
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
