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
	service.CancelFineTuningJobRunner(jobID)
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
	Progress       *int   `json:"progress,omitempty"`
	LogLine        string `json:"log_line,omitempty"`
	Error          string `json:"error"`
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
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":                     service.IsFineTuningEnabled(),
			"price_per_second":            pricePerSec,
			"gpu_multipliers":             gpuMultipliers,
			"gpu_options":                 gpuOptions,
			"colab_enabled":               service.IsColabDeploymentEnabled(),
			"colab_normal_ram_multiplier": colabNormalMult,
			"colab_high_ram_multiplier":   colabHighMult,
			"retention_days":              service.GetFineTuningRetentionDays(),
			"expiry_warning_days":         service.GetFineTuningExpiryWarningDays(),
			"jobs":                        jobs,
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
	var baseModel, trainingFile, trainingData, gpu, provider, suffix, method, taskType, ramTier, modelSource, modelSourceURL, stagedUploadID string
	epochs := 3
	var datasetBytes int64

	ct := c.GetHeader("Content-Type")
	if strings.HasPrefix(ct, "multipart/form-data") {
		baseModel = strings.TrimSpace(c.PostForm("model"))
		trainingFile = strings.TrimSpace(c.PostForm("training_file"))
		trainingData = strings.TrimSpace(c.PostForm("training_data"))
		gpu = strings.TrimSpace(c.PostForm("gpu"))
		provider = strings.TrimSpace(c.PostForm("provider"))
		suffix = strings.TrimSpace(c.PostForm("suffix"))
		method = strings.ToLower(strings.TrimSpace(c.PostForm("method")))
		taskType = strings.ToLower(strings.TrimSpace(c.PostForm("task_type")))
		ramTier = strings.ToLower(strings.TrimSpace(c.PostForm("ram_tier")))
		modelSource = strings.ToLower(strings.TrimSpace(c.PostForm("model_source")))
		modelSourceURL = strings.TrimSpace(c.PostForm("model_url"))
		stagedUploadID = strings.TrimSpace(c.PostForm("staged_upload_id"))
		if ep, err := strconv.Atoi(c.PostForm("epochs")); err == nil && ep > 0 && ep <= 50 {
			epochs = ep
		}
	} else {
		var req struct {
			Model          string `json:"model"`
			TrainingFile   string `json:"training_file"`
			TrainingData   string `json:"training_data"`
			GPU            string `json:"gpu"`
			Provider       string `json:"provider"`
			Suffix         string `json:"suffix"`
			Method         string `json:"method"`
			TaskType       string `json:"task_type"`
			RamTier        string `json:"ram_tier"`
			ModelSource    string `json:"model_source"`
			ModelURL       string `json:"model_url"`
			StagedUploadID string `json:"staged_upload_id"`
			Epochs         int    `json:"epochs"`
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
		method = strings.ToLower(strings.TrimSpace(req.Method))
		taskType = strings.ToLower(strings.TrimSpace(req.TaskType))
		ramTier = strings.ToLower(strings.TrimSpace(req.RamTier))
		modelSource = strings.ToLower(strings.TrimSpace(req.ModelSource))
		modelSourceURL = strings.TrimSpace(req.ModelURL)
		stagedUploadID = strings.TrimSpace(req.StagedUploadID)
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
	switch modelSource {
	case "huggingface", "url", "upload":
	default:
		modelSource = "huggingface"
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
			if err := os.MkdirAll(storageDir, 0o755); err == nil {
				ext := strings.ToLower(filepath.Ext(fh.Filename))
				switch ext {
				case ".jsonl", ".json", ".parquet", ".csv", ".zip", ".tar", ".gz", ".txt":
				default:
					ext = ".jsonl"
				}
				dst := filepath.Join(storageDir, "train"+ext)
				if src, oErr := fh.Open(); oErr == nil {
					if out, cErr := os.Create(dst); cErr == nil {
						written, _ := io.Copy(out, io.LimitReader(src, 1024<<20))
						_ = out.Close()
						trainingFilePath = dst
						datasetBytes = written
						if trainingFile == "" {
							trainingFile = fh.Filename
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

	// Calculate realistic duration & estimated cost from model size, counted dataset samples, bytes, epochs, method, and accelerator speed
	var modelFileBytes int64
	if baseModelFilePath != "" {
		if fi, sErr := os.Stat(baseModelFilePath); sErr == nil {
			modelFileBytes = fi.Size()
		}
	}
	dsSamples, _, _, _ := service.InspectFineTuningDatasetFile(trainingFilePath, trainingFile)
	modelParamHint := strings.TrimSpace(c.PostForm("model_param_size"))
	estimatedSec := service.EstimateFineTuningDurationRealistic(service.FineTuningEstimateInput{
		GPU:            gpu,
		TaskType:       taskType,
		Method:         method,
		Epochs:         epochs,
		DatasetBytes:   datasetBytes,
		DatasetSamples: dsSamples,
		ModelName:      baseModel,
		ModelParamHint: modelParamHint,
		ModelFileBytes: modelFileBytes,
	})
	estCost := service.EstimateFineTuningCostUSDWithRAM(gpu, ramTier, estimatedSec)

	// Verify user balance covers estimated fine-tuning cost
	if estCost > 0 {
		requiredQuota := common.QuotaRound(estCost * common.QuotaPerUnit)
		if userQuota, qErr := model.GetUserQuota(userId, false); qErr == nil && requiredQuota > 0 && userQuota < requiredQuota {
			service.CleanupFineTuningStorageForUser(userId, jobID)
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "Insufficient balance for estimated fine-tuning cost"})
			return
		}
	}

	hyperBytes, _ := common.Marshal(map[string]any{
		"n_epochs":      epochs,
		"suffix":        suffix,
		"method":        method,
		"task_type":     taskType,
		"ram_tier":      ramTier,
		"model_source":  modelSource,
		"model_url":     modelSourceURL,
		"dataset_bytes": datasetBytes,
	})
	now := time.Now().Unix()
	ftName := "ft:" + baseModel
	if suffix != "" {
		ftName = ftName + ":" + suffix
	} else {
		ftName = ftName + ":" + jobID[len(jobID)-6:]
	}

	job := &model.FineTuningJob{
		JobID:             jobID,
		UserId:            userId,
		Model:             baseModel,
		FineTunedModel:    ftName,
		TrainingFile:      trainingFile,
		TrainingFilePath:  trainingFilePath,
		BaseModelFilePath: baseModelFilePath,
		Hyperparameters:   string(hyperBytes),
		Status:            "queued",
		Provider:          provider,
		GPU:               gpu,
		Method:            method,
		TaskType:          taskType,
		RamTier:           ramTier,
		ModelSource:       modelSource,
		ModelSourceURL:    modelSourceURL,
		DurationSeconds:   estimatedSec,
		DatasetBytes:      datasetBytes,
		EstimatedCostUSD:  estCost,
		CallbackToken:     common.GetUUID(),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := model.CreateFineTuningJob(job); err != nil {
		service.CleanupFineTuningStorageForUser(userId, jobID)
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	service.StartFineTuningJobRunner(job.JobID)
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
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"job_id":           job.JobID,
			"fine_tuned_model": job.FineTunedModel,
			"status":           job.Status,
			"progress":         job.Progress,
			"trained_tokens":   job.TrainedTokens,
			"error":            job.Error,
			"logs":             service.ReadFineTuningJobLogs(job.UserId, job.JobID),
		},
	})
}

func UserDownloadFineTuningModel(c *gin.Context) {
	userId := c.GetInt("id")
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || (job.UserId != userId && c.GetInt("role") < common.RoleAdminUser) {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Fine-tuning job not found"})
		return
	}
	if job.Status != "succeeded" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Model is not ready for download yet; training must complete first"})
		return
	}
	if job.FilesPurgedAt > 0 || job.ModelFilePath == "" {
		c.JSON(http.StatusGone, gin.H{"success": false, "message": "Model files have expired and been purged from storage"})
		return
	}
	if _, statErr := os.Stat(job.ModelFilePath); statErr != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Model archive file not found on disk"})
		return
	}
	c.FileAttachment(job.ModelFilePath, job.JobID+".zip")
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
		},
	})
}

// AdminDeleteFineTuningJob deletes any fine-tuning job and removes its files on disk.
func AdminDeleteFineTuningJob(c *gin.Context) {
	jobID := c.Param("job_id")
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Fine-tuning job not found"})
		return
	}
	if err := model.AdminDeleteFineTuningJob(jobID); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	service.CleanupFineTuningStorageForUser(job.UserId, jobID)
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AdminGetFineTuningStats aggregates site-wide Fine-Tuning metrics, 30-day
// trend series, accelerator breakdown, and task-type distribution for the
// Site Statistics dashboard.
func AdminGetFineTuningStats(c *gin.Context) {
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
		}
		totalTokens += j.TrainedTokens
		totalBilledQ += j.BilledQuota
		totalDatasetMB += float64(j.DatasetBytes) / (1024.0 * 1024.0)

		day := time.Unix(j.CreatedAt, 0).UTC().Format("2006-01-02")
		cur := dailyMap[day]
		cur.Jobs++
		if j.Status == "succeeded" {
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
		if j.Status == "succeeded" {
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

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
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
		},
	})
}
