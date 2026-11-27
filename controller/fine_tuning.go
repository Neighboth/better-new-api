package controller

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	service.CancelFineTuningJobRunner(jobID)
	_ = service.ApplyFineTuningWorkerCallback(job, "cancelled", "", "", job.TrainedTokens)
	updated, _ := model.GetFineTuningJobByJobID(jobID)
	if updated != nil {
		job = updated
	}
	c.JSON(http.StatusOK, service.FormatOpenAIFineTuningJob(job))
}

type fineTuningWorkerCallbackRequest struct {
	Token          string          `json:"token"`
	Status         string          `json:"status"`
	FineTunedModel string          `json:"fine_tuned_model"`
	TrainedTokens  int64           `json:"trained_tokens"`
	Progress       *int            `json:"progress,omitempty"`
	LogLine        string          `json:"log_line,omitempty"`
	EvalReport     json.RawMessage `json:"eval_report,omitempty"`
	Error          string          `json:"error"`
}

func checkFineTuningStorageCapHTTP(c *gin.Context, additionalBytes int64, statusOnFail int) bool {
	allowed, usedBytes, maxBytes := service.CheckFineTuningStorageQuota(additionalBytes)
	if allowed {
		return true
	}
	usedGB := float64(usedBytes) / (1024.0 * 1024.0 * 1024.0)
	maxGB := float64(maxBytes) / (1024.0 * 1024.0 * 1024.0)
	c.JSON(statusOnFail, gin.H{
		"success":          false,
		"code":             "FT_STORAGE_QUOTA_EXCEEDED",
		"message":          fmt.Sprintf("Global fine-tuning storage limit reached (%.2f GB / %.2f GB). Please delete old fine-tuning models or increase the limit in System Settings.", usedGB, maxGB),
		"storage_used_gb":  usedGB,
		"storage_limit_gb": maxGB,
	})
	return false
}

func verifyFineTuningWorkerToken(c *gin.Context, job *model.FineTuningJob, bodyToken string) bool {
	token := strings.TrimSpace(bodyToken)
	if token == "" {
		token = strings.TrimSpace(c.Query("token"))
	}
	if token == "" {
		token = strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	}
	return job.CallbackToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(job.CallbackToken)) == 1
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
	if !verifyFineTuningWorkerToken(c, job, req.Token) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid callback token"})
		return
	}
	if strings.TrimSpace(req.LogLine) != "" {
		service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "train", req.LogLine)
	}
	if trimmedEval := strings.TrimSpace(string(req.EvalReport)); trimmedEval != "" && trimmedEval != "null" {
		_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
			"eval_report_json": trimmedEval,
			"updated_at":       time.Now().Unix(),
		})
	}
	if req.Progress != nil {
		pct := *req.Progress
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
			"progress":   pct,
			"updated_at": time.Now().Unix(),
		})
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}
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

// FineTuningWorkerDownloadDataset streams the uploaded training dataset to the
// remote fine-tuning worker using per-job callback-token authentication.
func FineTuningWorkerDownloadDataset(c *gin.Context) {
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Job not found"})
		return
	}
	if !verifyFineTuningWorkerToken(c, job, "") {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid callback token"})
		return
	}
	if job.TrainingFilePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Dataset file not found"})
		return
	}
	if _, statErr := os.Stat(job.TrainingFilePath); statErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Dataset file missing on disk"})
		return
	}
	c.File(job.TrainingFilePath)
}

// FineTuningWorkerDownloadBaseModel streams locally uploaded base weights to the
// remote fine-tuning worker using per-job callback-token authentication.
func FineTuningWorkerDownloadBaseModel(c *gin.Context) {
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Job not found"})
		return
	}
	if !verifyFineTuningWorkerToken(c, job, "") {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid callback token"})
		return
	}
	if job.BaseModelFilePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Base model file not found"})
		return
	}
	if _, statErr := os.Stat(job.BaseModelFilePath); statErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Base model file missing on disk"})
		return
	}
	c.File(job.BaseModelFilePath)
}

// FineTuningWorkerUploadCheckpoint atomically persists the latest resumable
// training checkpoint (.zip) uploaded periodically during training so another
// compute provider can seamlessly resume if the current worker disconnects.
func FineTuningWorkerUploadCheckpoint(c *gin.Context) {
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Job not found"})
		return
	}
	if !verifyFineTuningWorkerToken(c, job, c.PostForm("token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid callback token"})
		return
	}
	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "file is required"})
		return
	}
	if !checkFineTuningStorageCapHTTP(c, fh.Size, http.StatusInsufficientStorage) {
		return
	}
	destPath := service.FineTuningCheckpointZipPath(job.UserId, job.JobID)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	tmpPath := destPath + ".tmp"
	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer src.Close()
	out, err := os.Create(tmpPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	written, copyErr := io.Copy(out, src)
	_ = out.Close()
	if copyErr != nil || written == 0 {
		_ = os.Remove(tmpPath)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to store checkpoint"})
		return
	}
	if rErr := os.Rename(tmpPath, destPath); rErr != nil {
		_ = os.Remove(tmpPath)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": rErr.Error()})
		return
	}

	updates := map[string]any{
		"checkpoint_path": destPath,
		"updated_at":      time.Now().Unix(),
	}
	if ep, pErr := strconv.Atoi(strings.TrimSpace(c.PostForm("epoch"))); pErr == nil && ep >= 0 {
		updates["checkpoint_epoch"] = ep
	}
	if st, pErr := strconv.Atoi(strings.TrimSpace(c.PostForm("step"))); pErr == nil && st >= 0 {
		updates["checkpoint_step"] = st
	}
	if tt, pErr := strconv.ParseInt(strings.TrimSpace(c.PostForm("trained_tokens")), 10, 64); pErr == nil && tt > 0 {
		updates["trained_tokens"] = tt
	}
	if pct, pErr := strconv.Atoi(strings.TrimSpace(c.PostForm("progress"))); pErr == nil && pct >= 0 && pct <= 100 {
		updates["progress"] = pct
	}
	_ = model.UpdateFineTuningJobStatus(job.JobID, updates)
	c.JSON(http.StatusOK, gin.H{"success": true, "bytes": written})
}

// FineTuningWorkerDownloadCheckpoint streams the latest saved checkpoint archive
// so a newly allocated compute node can resume training from the last step.
func FineTuningWorkerDownloadCheckpoint(c *gin.Context) {
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Job not found"})
		return
	}
	if !verifyFineTuningWorkerToken(c, job, "") {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid callback token"})
		return
	}
	ckptPath := strings.TrimSpace(job.CheckpointPath)
	if ckptPath == "" {
		ckptPath = service.FineTuningCheckpointZipPath(job.UserId, job.JobID)
	}
	if info, statErr := os.Stat(ckptPath); statErr != nil || info.Size() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Checkpoint not found"})
		return
	}
	c.File(ckptPath)
}

// FineTuningWorkerUploadChunk receives a raw streaming chunk (e.g. 16 MB) of a
// checkpoint or final trained model .zip archive and writes it directly to disk
// at the requested byte offset. This avoids reverse-proxy 413 payload limits
// (Cloudflare 100 MB, Nginx, etc.) and supports multi-gigabyte / multi-terabyte
// (2 TB+) archives with constant low RAM usage.
func FineTuningWorkerUploadChunk(c *gin.Context) {
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Job not found"})
		return
	}
	if !verifyFineTuningWorkerToken(c, job, "") {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid callback token"})
		return
	}

	kind := strings.ToLower(strings.TrimSpace(c.Query("kind")))
	if kind != "artifact" && kind != "checkpoint" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "kind must be artifact or checkpoint"})
		return
	}
	uploadID := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, strings.TrimSpace(c.Query("upload_id")))
	if uploadID == "" {
		uploadID = "default"
	}

	offset, err := strconv.ParseInt(strings.TrimSpace(c.Query("offset")), 10, 64)
	if err != nil || offset < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid offset"})
		return
	}
	totalSize, _ := strconv.ParseInt(strings.TrimSpace(c.Query("total_size")), 10, 64)
	isFinal := c.Query("final") == "1" || strings.EqualFold(c.Query("final"), "true")

	if offset == 0 {
		projectedAdd := totalSize
		if projectedAdd <= 0 && c.Request.ContentLength > 0 {
			projectedAdd = c.Request.ContentLength
		}
		if !checkFineTuningStorageCapHTTP(c, projectedAdd, http.StatusInsufficientStorage) {
			return
		}
	} else if c.Request.ContentLength > 0 {
		if !checkFineTuningStorageCapHTTP(c, c.Request.ContentLength, http.StatusInsufficientStorage) {
			return
		}
	}

	storageDir := service.FineTuningStorageDirForUser(job.UserId, job.JobID)
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	var destPath string
	if kind == "artifact" {
		destPath = filepath.Join(storageDir, job.JobID+".zip")
	} else {
		destPath = service.FineTuningCheckpointZipPath(job.UserId, job.JobID)
	}
	partPath := destPath + "." + uploadID + ".part"

	flags := os.O_CREATE | os.O_WRONLY
	if offset == 0 {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(partPath, flags, 0o644)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	if offset > 0 {
		if _, sErr := f.Seek(offset, io.SeekStart); sErr != nil {
			_ = f.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": sErr.Error()})
			return
		}
	}
	written, copyErr := io.Copy(f, c.Request.Body)
	closeErr := f.Close()
	if copyErr != nil || closeErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed writing upload chunk"})
		return
	}

	if !isFinal {
		c.JSON(http.StatusOK, gin.H{"success": true, "written": written, "offset": offset, "final": false})
		return
	}

	fi, statErr := os.Stat(partPath)
	if statErr != nil || fi.Size() == 0 || (totalSize > 0 && fi.Size() < totalSize) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "incomplete chunked upload"})
		return
	}
	if rErr := os.Rename(partPath, destPath); rErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": rErr.Error()})
		return
	}

	updates := map[string]any{
		"updated_at": time.Now().Unix(),
	}
	if kind == "artifact" {
		updates["model_file_path"] = destPath
	} else {
		updates["checkpoint_path"] = destPath
		if ep, pErr := strconv.Atoi(strings.TrimSpace(c.Query("epoch"))); pErr == nil && ep >= 0 {
			updates["checkpoint_epoch"] = ep
		}
		if st, pErr := strconv.Atoi(strings.TrimSpace(c.Query("step"))); pErr == nil && st >= 0 {
			updates["checkpoint_step"] = st
		}
		if pct, pErr := strconv.Atoi(strings.TrimSpace(c.Query("progress"))); pErr == nil && pct >= 0 && pct <= 100 {
			updates["progress"] = pct
		}
	}
	if tt, pErr := strconv.ParseInt(strings.TrimSpace(c.Query("trained_tokens")), 10, 64); pErr == nil && tt > 0 {
		updates["trained_tokens"] = tt
	}
	_ = model.UpdateFineTuningJobStatus(job.JobID, updates)
	c.JSON(http.StatusOK, gin.H{"success": true, "bytes": fi.Size(), "final": true})
}

// UserCheckFineTuningModel inspects a selected Hugging Face repository or staged
// local upload and verifies it is compatible with fine-tuning using our
// OpenAI-format dataset, selected task type, and method.
func UserCheckFineTuningModel(c *gin.Context) {
	userId := c.GetInt("id")
	baseModel := strings.TrimSpace(c.Query("model"))
	modelSource := strings.ToLower(strings.TrimSpace(c.Query("model_source")))
	stagedUploadID := strings.TrimSpace(c.Query("staged_upload_id"))
	taskType := strings.ToLower(strings.TrimSpace(c.DefaultQuery("task_type", "text")))
	method := strings.ToLower(strings.TrimSpace(c.DefaultQuery("method", "lora")))
	datasetHasImages := strings.EqualFold(strings.TrimSpace(c.Query("dataset_has_images")), "true")

	localFilePath := ""
	if stagedUploadID != "" {
		stagedDir := service.FineTuningStagedDirForUser(userId, stagedUploadID)
		if entries, rErr := os.ReadDir(stagedDir); rErr == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					localFilePath = filepath.Join(stagedDir, entry.Name())
					if baseModel == "" {
						baseModel = entry.Name()
					}
					modelSource = "upload"
					break
				}
			}
		}
	}

	hfToken := ""
	if hfAcc, hfErr := ResolveProviderAccount("huggingface", ""); hfErr == nil && hfAcc != nil {
		hfToken = strings.TrimSpace(hfAcc.Token)
	}

	result := service.CheckFineTuningModelCompatibility(
		c.Request.Context(),
		baseModel,
		modelSource,
		localFilePath,
		taskType,
		method,
		datasetHasImages,
		hfToken,
	)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": result})
}

// FineTuningWorkerUploadArtifact receives the real trained .zip archive from the
// remote fine-tuning worker and persists it into uploads/fine_tuning/<user>/<job>/.
func FineTuningWorkerUploadArtifact(c *gin.Context) {
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Job not found"})
		return
	}
	if !verifyFineTuningWorkerToken(c, job, c.PostForm("token")) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid callback token"})
		return
	}
	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "file is required"})
		return
	}
	if !checkFineTuningStorageCapHTTP(c, fh.Size, http.StatusInsufficientStorage) {
		return
	}
	storageDir := service.FineTuningStorageDirForUser(job.UserId, job.JobID)
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	destZip := filepath.Join(storageDir, job.JobID+".zip")
	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer src.Close()
	out, err := os.Create(destZip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	written, copyErr := io.Copy(out, src)
	_ = out.Close()
	if copyErr != nil || written == 0 {
		_ = os.Remove(destZip)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to store artifact"})
		return
	}

	updates := map[string]any{
		"model_file_path": destZip,
		"updated_at":      time.Now().Unix(),
	}
	if tt, pErr := strconv.ParseInt(strings.TrimSpace(c.PostForm("trained_tokens")), 10, 64); pErr == nil && tt > 0 {
		updates["trained_tokens"] = tt
	}
	_ = model.UpdateFineTuningJobStatus(job.JobID, updates)
	c.JSON(http.StatusOK, gin.H{"success": true, "bytes": written})
}

func populateFineTuningFileFlags(jobs []model.FineTuningJob) {
	for i := range jobs {
		jobs[i].HasModelFile = false
		if jobs[i].Status == "succeeded" && jobs[i].FilesPurgedAt == 0 && jobs[i].ModelFilePath != "" {
			if _, statErr := os.Stat(jobs[i].ModelFilePath); statErr == nil {
				jobs[i].HasModelFile = true
			}
		}
	}
}

func UserListFineTuningJobs(c *gin.Context) {
	userId := c.GetInt("id")
	jobs, err := model.ListUserFineTuningJobs(userId, 100)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	populateFineTuningFileFlags(jobs)
	pricePerSec, gpuMultipliers := service.GetFineTuningPricing()
	gpuOptions := service.GetFineTuningGPUOptions()
	colabNormalMult, colabHighMult := service.GetColabRAMMultipliers()
	servingGPUOptions := service.GetFineTuningServingGPUOptions()
	servingNormalMult, servingHighMult := service.GetFineTuningServingColabRAMMultipliers()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":                             service.IsFineTuningEnabled(),
			"price_per_second":                    pricePerSec,
			"gpu_multipliers":                     gpuMultipliers,
			"gpu_options":                         gpuOptions,
			"colab_enabled":                       service.IsColabDeploymentEnabled(),
			"colab_normal_ram_multiplier":         colabNormalMult,
			"colab_high_ram_multiplier":           colabHighMult,
			"serving_enabled":                     service.IsFineTuningServingEnabled(),
			"serving_gpu_options":                 servingGPUOptions,
			"serving_colab_normal_ram_multiplier": servingNormalMult,
			"serving_colab_high_ram_multiplier":   servingHighMult,
			"retention_days":                      service.GetFineTuningRetentionDays(),
			"expiry_warning_days":                 service.GetFineTuningExpiryWarningDays(),
			"storage_used_bytes":                  service.CalculateFineTuningStorageUsedBytes(),
			"max_storage_gb":                      service.GetFineTuningMaxStorageGB(),
			"jobs":                                jobs,
		},
	})
}

// UserUploadStagedFineTuningModel receives a local base model file before job
// creation so the popup can show real upload progress and clean it up if the
// user cancels without starting training.
func UserUploadStagedFineTuningModel(c *gin.Context) {
	if !service.IsFineTuningEnabled() {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Fine-tuning is disabled in System Settings"})
		return
	}
	userId := c.GetInt("id")
	mf, err := c.FormFile("model_file")
	if err != nil || mf == nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "model_file is required"})
		return
	}
	if !checkFineTuningStorageCapHTTP(c, mf.Size, http.StatusOK) {
		return
	}
	safeName := filepath.Base(mf.Filename)
	if safeName == "" || safeName == "." || safeName == ".." {
		safeName = "model.safetensors"
	}
	if strings.HasSuffix(strings.ToLower(safeName), ".gguf") {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "GGUF files are quantized inference-only weights and cannot be fine-tuned; please upload .safetensors, .pt, or .bin"})
		return
	}

	uploadID := "up-" + strings.ReplaceAll(common.GetUUID(), "-", "")[:16]
	stagedDir := service.FineTuningStagedDirForUser(userId, uploadID)
	if err := os.MkdirAll(stagedDir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	dst := filepath.Join(stagedDir, safeName)
	src, err := mf.Open()
	if err != nil {
		_ = os.RemoveAll(stagedDir)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		_ = os.RemoveAll(stagedDir)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	written, copyErr := io.Copy(out, src)
	_ = out.Close()
	if copyErr != nil {
		_ = os.RemoveAll(stagedDir)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": copyErr.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"upload_id": uploadID,
			"filename":  safeName,
			"size":      written,
		},
	})
}

// UserDeleteStagedFineTuningModel deletes a staged uploaded model file when
// the user cancels or closes the fine-tuning popup without starting training.
func UserDeleteStagedFineTuningModel(c *gin.Context) {
	userId := c.GetInt("id")
	uploadID := strings.TrimSpace(c.Param("upload_id"))
	if uploadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "upload_id is required"})
		return
	}
	stagedDir := service.FineTuningStagedDirForUser(userId, uploadID)
	_ = os.RemoveAll(stagedDir)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func UserCreateFineTuningJob(c *gin.Context) {
	if !service.IsFineTuningEnabled() {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Fine-tuning is disabled in System Settings"})
		return
	}
	userId := c.GetInt("id")
	var baseModel, customModelID, trainingFile, trainingData, gpu, provider, suffix, method, quantization, taskType, ramTier, modelSource, modelSourceURL, stagedUploadID, modelParamHint string
	epochs := 3
	var datasetBytes int64
	var clientModelFileBytes int64
	var ackCompatWarning bool

	ct := c.GetHeader("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		baseModel = strings.TrimSpace(c.PostForm("model"))
		customModelID = strings.TrimSpace(c.PostForm("model_id"))
		trainingFile = strings.TrimSpace(c.PostForm("training_file"))
		trainingData = strings.TrimSpace(c.PostForm("training_data"))
		gpu = strings.TrimSpace(c.PostForm("gpu"))
		provider = strings.TrimSpace(c.PostForm("provider"))
		suffix = strings.TrimSpace(c.PostForm("suffix"))
		method = strings.ToLower(strings.TrimSpace(c.PostForm("method")))
		quantization = strings.ToUpper(strings.TrimSpace(c.PostForm("quantization")))
		taskType = strings.ToLower(strings.TrimSpace(c.PostForm("task_type")))
		ramTier = strings.ToLower(strings.TrimSpace(c.PostForm("ram_tier")))
		modelSource = strings.ToLower(strings.TrimSpace(c.PostForm("model_source")))
		modelSourceURL = strings.TrimSpace(c.PostForm("model_url"))
		stagedUploadID = strings.TrimSpace(c.PostForm("staged_upload_id"))
		modelParamHint = strings.TrimSpace(c.PostForm("model_param_size"))
		if mb, err := strconv.ParseInt(strings.TrimSpace(c.PostForm("model_file_bytes")), 10, 64); err == nil && mb > 0 {
			clientModelFileBytes = mb
		}
		ackCompatWarning = strings.EqualFold(strings.TrimSpace(c.PostForm("acknowledge_compat_warning")), "true")
		if ep, err := strconv.Atoi(c.PostForm("epochs")); err == nil && ep > 0 && ep <= 50 {
			epochs = ep
		}
	} else {
		var req struct {
			Model                    string `json:"model"`
			ModelID                  string `json:"model_id"`
			TrainingFile             string `json:"training_file"`
			TrainingData             string `json:"training_data"`
			GPU                      string `json:"gpu"`
			Provider                 string `json:"provider"`
			Suffix                   string `json:"suffix"`
			Method                   string `json:"method"`
			Quantization             string `json:"quantization"`
			TaskType                 string `json:"task_type"`
			RamTier                  string `json:"ram_tier"`
			ModelSource              string `json:"model_source"`
			ModelURL                 string `json:"model_url"`
			StagedUploadID           string `json:"staged_upload_id"`
			ModelParamSize           string `json:"model_param_size"`
			ModelFileBytes           int64  `json:"model_file_bytes"`
			AcknowledgeCompatWarning bool   `json:"acknowledge_compat_warning"`
			Epochs                   int    `json:"epochs"`
		}
		if err := common.DecodeJson(c.Request.Body, &req); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid request payload"})
			return
		}
		baseModel = strings.TrimSpace(req.Model)
		customModelID = strings.TrimSpace(req.ModelID)
		trainingFile = strings.TrimSpace(req.TrainingFile)
		trainingData = strings.TrimSpace(req.TrainingData)
		gpu = strings.TrimSpace(req.GPU)
		provider = strings.TrimSpace(req.Provider)
		suffix = strings.TrimSpace(req.Suffix)
		method = strings.ToLower(strings.TrimSpace(req.Method))
		quantization = strings.ToUpper(strings.TrimSpace(req.Quantization))
		taskType = strings.ToLower(strings.TrimSpace(req.TaskType))
		ramTier = strings.ToLower(strings.TrimSpace(req.RamTier))
		modelSource = strings.ToLower(strings.TrimSpace(req.ModelSource))
		modelSourceURL = strings.TrimSpace(req.ModelURL)
		stagedUploadID = strings.TrimSpace(req.StagedUploadID)
		modelParamHint = strings.TrimSpace(req.ModelParamSize)
		clientModelFileBytes = req.ModelFileBytes
		ackCompatWarning = req.AcknowledgeCompatWarning
		if req.Epochs > 0 && req.Epochs <= 50 {
			epochs = req.Epochs
		}
	}

	switch taskType {
	case "text", "image", "audio", "vision":
	default:
		taskType = "text"
	}
	if ramTier != "high" {
		ramTier = "normal"
	}
	switch method {
	case "lora", "qlora", "sft", "dpo", "dreambooth", "textual_inversion", "full":
	default:
		method = "lora"
	}
	if method == "qlora" {
		if quantization == "" {
			quantization = "NF4"
		}
	} else {
		quantization = ""
	}
	switch modelSource {
	case "huggingface", "url", "upload":
	default:
		modelSource = "huggingface"
	}

	// Enforce global storage limit before saving any new dataset/model files
	if !checkFineTuningStorageCapHTTP(c, int64(len(trainingData)), http.StatusOK) {
		return
	}

	jobID := service.GenerateFineTuningJobID()
	storageDir := service.FineTuningStorageDirForUser(userId, jobID)
	var trainingFilePath string
	var baseModelFilePath string

	// Adopt pre-uploaded staged model file if provided
	if stagedUploadID != "" {
		stagedDir := service.FineTuningStagedDirForUser(userId, stagedUploadID)
		if entries, rErr := os.ReadDir(stagedDir); rErr == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				srcPath := filepath.Join(stagedDir, entry.Name())
				if os.MkdirAll(storageDir, 0o755) == nil {
					dstPath := filepath.Join(storageDir, "base_"+entry.Name())
					if os.Rename(srcPath, dstPath) == nil {
						baseModelFilePath = dstPath
						modelSource = "upload"
						if baseModel == "" {
							baseModel = entry.Name()
						}
					}
				}
				break
			}
		}
		_ = os.RemoveAll(stagedDir)
	}

	if strings.HasPrefix(ct, "multipart/form-data") {
		if baseModelFilePath == "" {
			if mf, mErr := c.FormFile("model_file"); mErr == nil && mf != nil {
				if !checkFineTuningStorageCapHTTP(c, mf.Size, http.StatusOK) {
					service.CleanupFineTuningStorageForUser(userId, jobID)
					return
				}
				if err := os.MkdirAll(storageDir, 0o755); err == nil {
					safeName := filepath.Base(mf.Filename)
					if safeName == "" || safeName == "." || safeName == ".." {
						safeName = "base-model.safetensors"
					}
					dst := filepath.Join(storageDir, "base_"+safeName)
					if src, oErr := mf.Open(); oErr == nil {
						if out, cErr := os.Create(dst); cErr == nil {
							_, _ = io.Copy(out, src)
							_ = out.Close()
							baseModelFilePath = dst
							modelSource = "upload"
							if baseModel == "" {
								baseModel = safeName
							}
						}
						_ = src.Close()
					}
				}
			}
		}
		if fh, fErr := c.FormFile("file"); fErr == nil && fh != nil {
			if !checkFineTuningStorageCapHTTP(c, fh.Size, http.StatusOK) {
				service.CleanupFineTuningStorageForUser(userId, jobID)
				return
			}
			origName := filepath.Base(fh.Filename)
			if err := service.ValidateFineTuningDatasetFile(taskType, origName, nil); err != nil {
				service.CleanupFineTuningStorageForUser(userId, jobID)
				c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
				return
			}
			if err := os.MkdirAll(storageDir, 0o755); err == nil {
				ext := strings.ToLower(filepath.Ext(origName))
				dst := filepath.Join(storageDir, "train"+ext)
				if src, oErr := fh.Open(); oErr == nil {
					if out, cErr := os.Create(dst); cErr == nil {
						written, _ := io.Copy(out, io.LimitReader(src, 1024<<20))
						_ = out.Close()
						trainingFilePath = dst
						datasetBytes = written
						if trainingFile == "" {
							trainingFile = origName
						}
					}
					_ = src.Close()
				}
			}
		}
	}

	if modelSource == "url" && baseModel == "" && modelSourceURL != "" {
		baseModel = modelSourceURL
	}
	if baseModel == "" {
		service.CleanupFineTuningStorageForUser(userId, jobID)
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Base model is required"})
		return
	}
	if strings.HasSuffix(strings.ToLower(baseModel), ".gguf") {
		service.CleanupFineTuningStorageForUser(userId, jobID)
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "GGUF files are inference-only and cannot be fine-tuned; select a .safetensors model or full repository"})
		return
	}
	if gpu == "" {
		for _, opt := range service.GetFineTuningGPUOptions() {
			if opt.Enabled {
				gpu = opt.ID
				break
			}
		}
	}
	if _, ok := service.FindEnabledFineTuningGPU(gpu); !ok {
		service.CleanupFineTuningStorageForUser(userId, jobID)
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Selected GPU option is disabled or invalid"})
		return
	}

	if trainingFilePath == "" && trainingData != "" {
		if err := os.MkdirAll(storageDir, 0o755); err == nil {
			dst := filepath.Join(storageDir, "train.jsonl")
			if os.WriteFile(dst, []byte(trainingData), 0o644) == nil {
				trainingFilePath = dst
				datasetBytes = int64(len(trainingData))
				if trainingFile == "" {
					trainingFile = "inline-dataset.jsonl"
				}
			}
		}
	}
	if trainingFile == "" {
		service.CleanupFineTuningStorageForUser(userId, jobID)
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Training dataset file is required"})
		return
	}

	// Strictly validate dataset content: for text/vision, MUST be valid OpenAI JSONL format!
	dsValidation, vErr := service.ValidateOpenAIDatasetFile(taskType, method, trainingFilePath, trainingFile)
	if vErr != nil {
		service.CleanupFineTuningStorageForUser(userId, jobID)
		c.JSON(http.StatusOK, gin.H{"success": false, "message": vErr.Error()})
		return
	}

	// Strictly verify the chosen base model is compatible with our OpenAI dataset & task/method
	hfToken := ""
	if hfAcc, hfErr := ResolveProviderAccount("huggingface", ""); hfErr == nil && hfAcc != nil {
		hfToken = strings.TrimSpace(hfAcc.Token)
	}
	modelCompat := service.CheckFineTuningModelCompatibility(
		c.Request.Context(),
		baseModel,
		modelSource,
		baseModelFilePath,
		taskType,
		method,
		dsValidation.HasImages,
		hfToken,
	)
	if modelCompat.Checked && !modelCompat.Compatible {
		service.CleanupFineTuningStorageForUser(userId, jobID)
		c.JSON(http.StatusOK, gin.H{"success": false, "message": modelCompat.Reason})
		return
	}

	// Resolve direct custom Model ID (e.g. "benim-modelim") with uniqueness check per user
	var ftName string
	if customModelID != "" {
		validatedID, idErr := service.ValidateCustomFineTunedModelID(customModelID)
		if idErr != nil {
			service.CleanupFineTuningStorageForUser(userId, jobID)
			c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_INVALID_MODEL_ID", "message": idErr.Error()})
			return
		}
		if model.ExistsUserFineTunedModel(userId, validatedID, "") {
			service.CleanupFineTuningStorageForUser(userId, jobID)
			c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_MODEL_ID_EXISTS", "message": "You already have a fine-tuned model with this Model ID"})
			return
		}
		ftName = validatedID
	} else if suffix != "" {
		ftName = "ft:" + baseModel + ":" + suffix
	} else {
		baseSlug := sanitizeSlug(filepath.Base(baseModel))
		if baseSlug == "" {
			baseSlug = "model"
		}
		ftName = baseSlug + "-" + jobID[len(jobID)-6:]
	}

	// Calculate realistic duration, hardware compatibility & estimated cost
	modelFileBytes := clientModelFileBytes
	if modelCompat.TotalWeightBytes > modelFileBytes {
		modelFileBytes = modelCompat.TotalWeightBytes
	}
	if baseModelFilePath != "" {
		if fi, sErr := os.Stat(baseModelFilePath); sErr == nil && fi.Size() > 0 {
			modelFileBytes = fi.Size()
		}
	}
	dsSamples := dsValidation.SampleCount
	if dsSamples <= 0 {
		dsSamples, _, _, _ = service.InspectFineTuningDatasetFile(trainingFilePath, trainingFile)
	}
	estInput := service.FineTuningEstimateInput{
		GPU:            gpu,
		RamTier:        ramTier,
		TaskType:       taskType,
		Method:         method,
		Quantization:   quantization,
		Epochs:         epochs,
		DatasetBytes:   datasetBytes,
		DatasetSamples: dsSamples,
		ModelName:      baseModel,
		ModelParamHint: modelParamHint,
		ModelFileBytes: modelFileBytes,
	}
	compat := service.CheckFineTuningHardwareCompat(estInput)
	estimatedSec := service.EstimateFineTuningDurationRealistic(estInput)
	estCost := service.EstimateFineTuningCostUSDWithRAM(gpu, ramTier, estimatedSec)

	// Verify user balance covers estimated fine-tuning cost
	if estCost > 0 {
		requiredQuota := common.QuotaRound(estCost * common.QuotaPerUnit)
		if userQuota, qErr := model.GetUserQuota(userId, false); qErr == nil && requiredQuota > 0 && userQuota < requiredQuota {
			service.CleanupFineTuningStorageForUser(userId, jobID)
			c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_INSUFFICIENT_BALANCE", "message": "Insufficient balance for estimated fine-tuning cost"})
			return
		}
	}

	hyperBytes, _ := common.Marshal(map[string]any{
		"n_epochs":         epochs,
		"model_id":         ftName,
		"suffix":           suffix,
		"method":           method,
		"quantization":     quantization,
		"task_type":        taskType,
		"ram_tier":         ramTier,
		"model_source":     modelSource,
		"model_url":        modelSourceURL,
		"model_file_bytes": modelFileBytes,
		"dataset_bytes":    datasetBytes,
	})
	now := time.Now().Unix()

	job := &model.FineTuningJob{
		JobID:                     jobID,
		UserId:                    userId,
		Model:                     baseModel,
		FineTunedModel:            ftName,
		TrainingFile:              trainingFile,
		TrainingFilePath:          trainingFilePath,
		BaseModelFilePath:         baseModelFilePath,
		Hyperparameters:           string(hyperBytes),
		Status:                    "queued",
		Provider:                  provider,
		GPU:                       gpu,
		Method:                    method,
		Quantization:              quantization,
		TaskType:                  taskType,
		RamTier:                   ramTier,
		ModelSource:               modelSource,
		ModelSourceURL:            modelSourceURL,
		DurationSeconds:           estimatedSec,
		DatasetBytes:              datasetBytes,
		EstimatedCostUSD:          estCost,
		CompatWarningAcknowledged: ackCompatWarning || !compat.Compatible,
		CompatWarningReason:       compat.Reason,
		CallbackToken:             common.GetUUID(),
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := model.CreateFineTuningJob(job); err != nil {
		service.CleanupFineTuningStorageForUser(userId, jobID)
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	service.StartFineTuningJobRunner(job.JobID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": job})
}

// UserUpdateFineTuningJob lets the owner rename the direct Model ID of a fine-tuned model
// (before or after training) and syncs any active serving deployment immediately.
func UserUpdateFineTuningJob(c *gin.Context) {
	userId := c.GetInt("id")
	jobID := strings.TrimSpace(c.Param("job_id"))
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || (job.UserId != userId && c.GetInt("role") < common.RoleAdminUser) {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_JOB_NOT_FOUND", "message": "Fine-tuning job not found"})
		return
	}
	var req struct {
		ModelID        string `json:"model_id"`
		FineTunedModel string `json:"fine_tuned_model"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid request payload"})
		return
	}
	rawID := strings.TrimSpace(req.ModelID)
	if rawID == "" {
		rawID = strings.TrimSpace(req.FineTunedModel)
	}
	validatedID, idErr := service.ValidateCustomFineTunedModelID(rawID)
	if idErr != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_INVALID_MODEL_ID", "message": idErr.Error()})
		return
	}
	if model.ExistsUserFineTunedModel(job.UserId, validatedID, job.JobID) {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_MODEL_ID_EXISTS", "message": "You already have a fine-tuned model with this Model ID"})
		return
	}

	now := time.Now().Unix()
	if err := model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
		"fine_tuned_model": validatedID,
		"updated_at":       now,
	}); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if job.ServingDeploymentID > 0 {
		_ = model.UpdateNotebookDeploymentFields(job.ServingDeploymentID, map[string]any{
			"model_id":     validatedID,
			"updated_time": now,
		})
	}
	if job.ServingStatus != "" && job.ServingStatus != "stopped" {
		service.AppendFineTuningServingLog(job.UserId, job.JobID, "rename",
			fmt.Sprintf("Model ID updated from %q to %q", job.FineTunedModel, validatedID))
	}

	updated, _ := model.GetFineTuningJobByJobID(job.JobID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": updated})
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
		service.CancelFineTuningJobRunner(jobID)
		_ = service.ApplyFineTuningWorkerCallback(job, "cancelled", "", "", job.TrainedTokens)
	}
	updated, _ := model.GetFineTuningJobByJobID(jobID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": updated})
}

func UserGetFineTuningJobLogs(c *gin.Context) {
	userId := c.GetInt("id")
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || (job.UserId != userId && c.GetInt("role") < common.RoleAdminUser) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Fine-tuning job not found"})
		return
	}
	trainingLogs := service.ReadFineTuningJobLogs(job.UserId, job.JobID)
	servingLogs := service.ReadFineTuningServingLogs(job.UserId, job.JobID)

	// Merge live sanitized runtime logs from active/starting private NotebookDeployment
	if job.ServingDeploymentID > 0 {
		if dep, dErr := model.GetNotebookDeploymentByID(job.ServingDeploymentID); dErr == nil && dep != nil {
			liveParts := make([]string, 0, 2)
			if strings.TrimSpace(dep.StatusMessage) != "" && dep.Status != "running" {
				liveParts = append(liveParts, "[status] "+service.SanitizeFineTuningLogLine(dep.StatusMessage))
			}
			if strings.TrimSpace(dep.Logs) != "" {
				var sanitizedLines []string
				for _, ln := range strings.Split(strings.TrimRight(dep.Logs, "\n"), "\n") {
					s := strings.TrimSpace(ln)
					if s != "" {
						sanitizedLines = append(sanitizedLines, service.SanitizeFineTuningLogLine(s))
					}
				}
				if len(sanitizedLines) > 0 {
					liveParts = append(liveParts, strings.Join(sanitizedLines, "\n"))
				}
			}
			if len(liveParts) > 0 {
				liveText := strings.Join(liveParts, "\n")
				if servingLogs != "" {
					servingLogs = servingLogs + "\n--- Live Runtime Logs ---\n" + liveText
				} else {
					servingLogs = liveText
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"job_id":                       job.JobID,
			"fine_tuned_model":             job.FineTunedModel,
			"status":                       job.Status,
			"progress":                     job.Progress,
			"trained_tokens":               job.TrainedTokens,
			"error":                        job.Error,
			"logs":                         trainingLogs,
			"training_logs":                trainingLogs,
			"serving_logs":                 servingLogs,
			"serving_status":               job.ServingStatus,
			"serving_gpu":                  job.ServingGPU,
			"serving_ram_tier":             job.ServingRamTier,
			"serving_idle_timeout_seconds": job.ServingIdleTimeoutSeconds,
			"serving_stop_reason":          job.ServingStopReason,
			"serving_error":                job.ServingError,
		},
	})
}

func UserDownloadFineTuningModel(c *gin.Context) {
	userId := c.GetInt("id")
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || (job.UserId != userId && c.GetInt("role") < common.RoleAdminUser) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "FT_JOB_NOT_FOUND", "message": "Fine-tuning job not found"})
		return
	}
	if job.Status != "succeeded" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "code": "FT_MODEL_NOT_READY", "message": "Model is not ready for download yet; training must complete first"})
		return
	}
	if job.FilesPurgedAt > 0 || job.ModelFilePath == "" {
		c.JSON(http.StatusGone, gin.H{"success": false, "code": "FT_MODEL_EXPIRED", "message": "Model files have expired and been purged from storage"})
		return
	}
	streamFineTuningZipFile(c, job.ModelFilePath, job.JobID+".zip")
}

func UserDeleteFineTuningJob(c *gin.Context) {
	userId := c.GetInt("id")
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || (job.UserId != userId && c.GetInt("role") < common.RoleAdminUser) {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_JOB_NOT_FOUND", "message": "Fine-tuning job not found"})
		return
	}
	if job.ServingStatus == "starting" || job.ServingStatus == "running" || job.ServingStatus == "sleeping" {
		stopFineTuningServingJob(job.JobID, "job_deleted")
	}
	if err := model.DeleteFineTuningJob(jobID, job.UserId); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	service.CleanupFineTuningStorageForUser(job.UserId, jobID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AdminListFineTuningJobs returns all fine-tuning jobs across all users with
// usernames & earnings summary for the Fine-Tuning settings admin management table.
func AdminListFineTuningJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	jobs, err := model.ListAllFineTuningJobs(limit)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	populateFineTuningFileFlags(jobs)

	userIDs := make([]int, 0, len(jobs))
	seen := make(map[int]bool, len(jobs))
	for _, j := range jobs {
		if !seen[j.UserId] {
			seen[j.UserId] = true
			userIDs = append(userIDs, j.UserId)
		}
	}
	usernames := make(map[int]string, len(userIDs))
	if len(userIDs) > 0 {
		var users []model.User
		if model.DB.Select("id, username").Where("id IN ?", userIDs).Find(&users).Error == nil {
			for _, u := range users {
				usernames[u.Id] = u.Username
			}
		}
	}

	type adminFineTuningItem struct {
		model.FineTuningJob
		Username string `json:"username"`
	}
	items := make([]adminFineTuningItem, 0, len(jobs))
	var totalRevenueUSD float64
	var totalTokens int64
	for _, j := range jobs {
		items = append(items, adminFineTuningItem{
			FineTuningJob: j,
			Username:      usernames[j.UserId],
		})
		if j.Status == "succeeded" {
			totalRevenueUSD += j.EstimatedCostUSD
		}
		totalTokens += j.TrainedTokens
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"jobs":                 items,
			"total_jobs":           len(items),
			"total_revenue_usd":    totalRevenueUSD,
			"total_trained_tokens": totalTokens,
			"storage_used_bytes":   service.CalculateFineTuningStorageUsedBytes(),
			"max_storage_gb":       service.GetFineTuningMaxStorageGB(),
		},
	})
}

// AdminDeleteFineTuningJob deletes any fine-tuning job and removes its files on disk.
func AdminDeleteFineTuningJob(c *gin.Context) {
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_JOB_NOT_FOUND", "message": "Fine-tuning job not found"})
		return
	}
	if job.ServingStatus == "starting" || job.ServingStatus == "running" || job.ServingStatus == "sleeping" {
		stopFineTuningServingJob(job.JobID, "job_deleted")
	}
	if err := model.AdminDeleteFineTuningJob(jobID); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	service.CleanupFineTuningStorageForUser(job.UserId, jobID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

var (
	ftStatsCacheMu  sync.Mutex
	ftStatsCacheVal gin.H
	ftStatsCacheExp time.Time
)

// AdminGetFineTuningStats aggregates site-wide Fine-Tuning metrics, 30-day
// trend series, accelerator breakdown, and task-type distribution for the
// Site Statistics dashboard. Cached for 30s to keep Site Statistics instant.
func AdminGetFineTuningStats(c *gin.Context) {
	forceRefresh := strings.EqualFold(c.Query("refresh"), "true") || c.Query("refresh") == "1"
	if !forceRefresh {
		ftStatsCacheMu.Lock()
		if ftStatsCacheVal != nil && time.Now().Before(ftStatsCacheExp) {
			cached := ftStatsCacheVal
			ftStatsCacheMu.Unlock()
			c.JSON(http.StatusOK, gin.H{"success": true, "data": cached})
			return
		}
		ftStatsCacheMu.Unlock()
	}

	jobs, err := model.ListAllFineTuningJobs(500)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	var (
		totalJobs      int
		succeededJobs  int
		runningJobs    int
		failedJobs     int
		totalTokens    int64
		totalCostUSD   float64
		totalBilledQ   int64
		totalDatasetMB float64
	)

	now := time.Now().UTC()
	dailyMap := make(map[string]struct {
		Jobs       int
		RevenueUSD float64
		Tokens     int64
	})
	dayOrder := make([]string, 0, 30)
	for i := 29; i >= 0; i-- {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		dayOrder = append(dayOrder, d)
	}

	type accelStat struct {
		GPU        string  `json:"gpu"`
		Jobs       int     `json:"jobs"`
		RevenueUSD float64 `json:"revenue_usd"`
		Tokens     int64   `json:"trained_tokens"`
	}
	accelMap := map[string]*accelStat{}
	taskMap := map[string]int{}

	for _, j := range jobs {
		totalJobs++
		switch j.Status {
		case "succeeded":
			succeededJobs++
			totalCostUSD += j.EstimatedCostUSD
		case "running", "queued":
			runningJobs++
		case "failed":
			failedJobs++
			if j.BilledQuota > 0 {
				totalCostUSD += j.EstimatedCostUSD
			}
		}
		totalTokens += j.TrainedTokens
		totalBilledQ += j.BilledQuota
		totalDatasetMB += float64(j.DatasetBytes) / (1024.0 * 1024.0)

		day := time.Unix(j.CreatedAt, 0).UTC().Format("2006-01-02")
		cur := dailyMap[day]
		cur.Jobs++
		if j.Status == "succeeded" || j.BilledQuota > 0 {
			cur.RevenueUSD += j.EstimatedCostUSD
		}
		cur.Tokens += j.TrainedTokens
		dailyMap[day] = cur

		gpuKey := j.GPU
		if gpuKey == "" {
			gpuKey = "CPU"
		}
		as, ok := accelMap[gpuKey]
		if !ok {
			as = &accelStat{GPU: gpuKey}
			accelMap[gpuKey] = as
		}
		as.Jobs++
		if j.Status == "succeeded" || j.BilledQuota > 0 {
			as.RevenueUSD += j.EstimatedCostUSD
		}
		as.Tokens += j.TrainedTokens

		tt := j.TaskType
		if tt == "" {
			tt = "text"
		}
		taskMap[tt]++
	}

	type dailyPoint struct {
		Date       string  `json:"date"`
		Jobs       int     `json:"jobs"`
		RevenueUSD float64 `json:"revenue_usd"`
		Tokens     int64   `json:"trained_tokens"`
	}
	dailyHistory := make([]dailyPoint, 0, len(dayOrder))
	for _, d := range dayOrder {
		v := dailyMap[d]
		dailyHistory = append(dailyHistory, dailyPoint{
			Date:       d,
			Jobs:       v.Jobs,
			RevenueUSD: v.RevenueUSD,
			Tokens:     v.Tokens,
		})
	}

	byAccel := make([]accelStat, 0, len(accelMap))
	for _, v := range accelMap {
		byAccel = append(byAccel, *v)
	}

	payload := gin.H{
		"total_jobs":           totalJobs,
		"succeeded_jobs":       succeededJobs,
		"running_jobs":         runningJobs,
		"failed_jobs":          failedJobs,
		"total_trained_tokens": totalTokens,
		"total_revenue_usd":    totalCostUSD,
		"total_billed_quota":   totalBilledQ,
		"total_dataset_mb":     totalDatasetMB,
		"daily_history":        dailyHistory,
		"by_accelerator":       byAccel,
		"by_task_type":         taskMap,
	}

	ftStatsCacheMu.Lock()
	ftStatsCacheVal = payload
	ftStatsCacheExp = time.Now().Add(30 * time.Second)
	ftStatsCacheMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    payload,
	})
}
