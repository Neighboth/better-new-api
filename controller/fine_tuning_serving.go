package controller

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/notebookscript"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
)

// ============================================================================
// Direct Streaming Model Download (Short-Lived Signed Ticket + HTTP Range)
// ============================================================================

type fineTuningDownloadTicket struct {
	JobID     string
	UserID    int
	FilePath  string
	Filename  string
	ExpiresAt time.Time
}

var (
	ftDownloadTicketMu sync.Mutex
	ftDownloadTickets  = map[string]fineTuningDownloadTicket{}
)

func generateDownloadTicketToken() string {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return strings.ReplaceAll(common.GetUUID(), "-", "")
	}
	return hex.EncodeToString(buf)
}

func pruneExpiredDownloadTicketsLocked(now time.Time) {
	for k, v := range ftDownloadTickets {
		if now.After(v.ExpiresAt) {
			delete(ftDownloadTickets, k)
		}
	}
}

func streamFineTuningZipFile(c *gin.Context, filePath, downloadName string) {
	f, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "FT_MODEL_MISSING", "message": "Model archive file not found on disk"})
		return
	}
	defer f.Close()

	fi, statErr := f.Stat()
	if statErr != nil || fi.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "code": "FT_MODEL_MISSING", "message": "Model archive file not found on disk"})
		return
	}

	safeName := filepath.Base(downloadName)
	if safeName == "" || safeName == "." || safeName == ".." {
		safeName = "fine-tuned-model.zip"
	}

	// Disable proxy response buffering so multi-GB archives stream first bytes immediately.
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.ReplaceAll(safeName, `"`, "")))
	c.Header("Cache-Control", "no-store")
	c.Header("X-Accel-Buffering", "no")
	http.ServeContent(c.Writer, c.Request, safeName, fi.ModTime(), f)
}

// UserCreateFineTuningDownloadTicket issues a short-lived signed download URL so
// the browser streams the .zip archive directly without Axios RAM buffering or timeouts.
func UserCreateFineTuningDownloadTicket(c *gin.Context) {
	userId := c.GetInt("id")
	jobID := strings.TrimSpace(c.Param("job_id"))
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || (job.UserId != userId && c.GetInt("role") < common.RoleAdminUser) {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_JOB_NOT_FOUND", "message": "Fine-tuning job not found"})
		return
	}
	if job.Status != "succeeded" {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_MODEL_NOT_READY", "message": "Model is not ready for download yet; training must complete first"})
		return
	}
	if job.FilesPurgedAt > 0 || job.ModelFilePath == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_MODEL_EXPIRED", "message": "Model files have expired and been purged from storage"})
		return
	}
	fi, statErr := os.Stat(job.ModelFilePath)
	if statErr != nil || fi.Size() <= 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_MODEL_MISSING", "message": "Model archive file not found on disk"})
		return
	}

	token := generateDownloadTicketToken()
	filename := job.JobID + ".zip"
	now := time.Now()
	ftDownloadTicketMu.Lock()
	pruneExpiredDownloadTicketsLocked(now)
	ftDownloadTickets[token] = fineTuningDownloadTicket{
		JobID:     job.JobID,
		UserID:    userId,
		FilePath:  job.ModelFilePath,
		Filename:  filename,
		ExpiresAt: now.Add(15 * time.Minute),
	}
	ftDownloadTicketMu.Unlock()

	downloadURL := fmt.Sprintf("/api/fine_tuning/download/%s?ticket=%s", url.PathEscape(job.JobID), url.QueryEscape(token))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"ticket":       token,
			"filename":     filename,
			"size":         fi.Size(),
			"download_url": downloadURL,
		},
	})
}

// DownloadFineTuningModelWithTicket streams the trained .zip archive directly to
// the browser with Range/resume support using a short-lived signed ticket.
func DownloadFineTuningModelWithTicket(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("job_id"))
	ticketToken := strings.TrimSpace(c.Query("ticket"))
	if jobID == "" || ticketToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid or missing download ticket"})
		return
	}

	now := time.Now()
	ftDownloadTicketMu.Lock()
	pruneExpiredDownloadTicketsLocked(now)
	t, ok := ftDownloadTickets[ticketToken]
	ftDownloadTicketMu.Unlock()
	if !ok || t.JobID != jobID || now.After(t.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Download link has expired; please click Download again"})
		return
	}

	streamFineTuningZipFile(c, t.FilePath, t.Filename)
}

// FineTuningWorkerDownloadArtifact streams the trained .zip archive to a remote
// model-serving worker authenticated by per-job CallbackToken.
func FineTuningWorkerDownloadArtifact(c *gin.Context) {
	jobID := strings.TrimSpace(c.Param("job_id"))
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Job not found"})
		return
	}
	if !verifyFineTuningWorkerToken(c, job, "") {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid callback token"})
		return
	}
	if job.Status != "succeeded" || job.FilesPurgedAt > 0 || job.ModelFilePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Model archive unavailable"})
		return
	}
	streamFineTuningZipFile(c, job.ModelFilePath, job.JobID+".zip")
}

// ============================================================================
// One-Click Fine-Tuned Model Hosting / Serving & Zero-Balance Auto-Stop
// ============================================================================

type startFineTuningServingRequest struct {
	GPU     string `json:"gpu"`
	RamTier string `json:"ram_tier"`
}

func resolvePublicServerBaseURL(c *gin.Context) string {
	base := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if base != "" {
		return base
	}
	if c != nil && c.Request != nil && c.Request.Host != "" {
		scheme := "http"
		if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		return scheme + "://" + c.Request.Host
	}
	return ""
}

func resolveServingProviderCandidate(gpuID string) (*fineTuningCandidate, error) {
	candidates := resolveFineTuningCandidates(gpuID, "")
	for i := range candidates {
		// Fine-tuned serving supports Modal, Colab, and Kaggle script runtimes.
		switch candidates[i].Provider {
		case "modal", "colab", "kaggle":
			chosen := candidates[i]
			return &chosen, nil
		}
	}
	return nil, fmt.Errorf("no enabled compute provider account available for accelerator %s", gpuID)
}

// UserStartFineTuningServing launches a private compute instance hosting the
// user's trained fine-tuned model (visible only in that user's Playground & /v1/models).
func UserStartFineTuningServing(c *gin.Context) {
	if !service.IsFineTuningServingEnabled() {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "FT_SERVING_DISABLED",
			"message": "Fine-tuned model hosting is disabled in System Settings",
		})
		return
	}

	userId := c.GetInt("id")
	jobID := strings.TrimSpace(c.Param("job_id"))
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || job.UserId != userId {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "FT_JOB_NOT_FOUND",
			"message": "Fine-tuning job not found",
		})
		return
	}
	if job.Status != "succeeded" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "FT_MODEL_NOT_READY",
			"message": "Model is not ready yet; training must complete first",
		})
		return
	}
	if job.FilesPurgedAt > 0 || job.ModelFilePath == "" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "FT_MODEL_EXPIRED",
			"message": "Model files have expired and been purged from storage",
		})
		return
	}
	if _, statErr := os.Stat(job.ModelFilePath); statErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "FT_MODEL_MISSING",
			"message": "Model archive file not found on disk",
		})
		return
	}
	if job.ServingStatus == "starting" || job.ServingStatus == "running" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "FT_SERVING_ALREADY_ACTIVE",
			"message": "This fine-tuned model is already starting or running",
		})
		return
	}

	var req startFineTuningServingRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "Invalid request payload"})
		return
	}
	gpuID := strings.TrimSpace(req.GPU)
	ramTier := strings.ToLower(strings.TrimSpace(req.RamTier))
	if ramTier != "high" {
		ramTier = "normal"
	}

	gpuOpt, ok := service.FindEnabledFineTuningServingGPU(gpuID)
	if !ok || gpuOpt == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "FT_SERVING_GPU_DISABLED",
			"message": "Selected accelerator is not enabled for model hosting",
		})
		return
	}

	// Ensure positive user balance covers at least 60 seconds of hosting before provisioning.
	pricePerSec := service.ResolveFineTuningServingPricePerSecond(gpuID, ramTier)
	minQuota := 1
	if pricePerSec > 0 {
		minQuota = common.QuotaRound(pricePerSec * 60.0 * common.QuotaPerUnit)
		if minQuota < 1 {
			minQuota = 1
		}
	}
	if userQuota, qErr := model.GetUserQuota(userId, true); qErr == nil && userQuota < minQuota {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "FT_SERVING_INSUFFICIENT_BALANCE",
			"message": "Insufficient balance to activate this model",
		})
		return
	}

	cand, candErr := resolveServingProviderCandidate(gpuID)
	if candErr != nil || cand == nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    "FT_SERVING_NO_PROVIDER",
			"message": "No enabled compute provider account supports the selected accelerator",
		})
		return
	}

	// Ensure callback token exists for worker artifact download auth.
	callbackToken := strings.TrimSpace(job.CallbackToken)
	if callbackToken == "" {
		callbackToken = common.GetUUID()
		_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{"callback_token": callbackToken})
	}

	serverBase := resolvePublicServerBaseURL(c)
	artifactURL := fmt.Sprintf("%s/api/deployments/fine_tuning/%s/artifact_download?token=%s",
		serverBase, url.PathEscape(job.JobID), url.QueryEscape(callbackToken))

	// Resolve provider credentials & HF token
	common.OptionMapRWMutex.RLock()
	modalVolPath := strings.TrimSpace(common.OptionMap["model_deployment.modal.shared_volume_path"])
	common.OptionMapRWMutex.RUnlock()

	var modalTokenID, modalTokenSecret, kaggleUser, kaggleKey, hfToken string
	switch cand.Provider {
	case "modal":
		modalTokenID = strings.TrimSpace(cand.Account.TokenID)
		modalTokenSecret = strings.TrimSpace(cand.Account.TokenSecret)
	case "kaggle":
		kaggleUser = strings.TrimSpace(cand.Account.Username)
		kaggleKey = strings.TrimSpace(cand.Account.APIKey)
	case "colab":
		if _, tokErr := getValidColabAccessTokenForAccount(c.Request.Context(), cand.Account.ID); tokErr != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"code":    "FT_SERVING_NO_PROVIDER",
				"message": tokErr.Error(),
			})
			return
		}
	}
	if hfAcc, hfErr := ResolveProviderAccount("huggingface", ""); hfErr == nil && hfAcc != nil {
		hfToken = strings.TrimSpace(hfAcc.Token)
	}

	taskType := notebookscript.NormalizeTaskType(job.TaskType)
	deployName := "ft-serve-" + sanitizeSlug(job.JobID)
	depToken := common.GetUUID()
	now := time.Now().Unix()

	dep := &model.NotebookDeployment{
		OwnerUserID:        userId,
		FineTuningJobID:    job.JobID,
		Provider:           cand.Provider,
		AccountID:          cand.Account.ID,
		AccountLabel:       cand.Account.Label,
		Name:               deployName,
		ModelID:            job.FineTunedModel,
		ModelRepo:          job.Model,
		TaskType:           taskType,
		Accelerator:        cand.Accelerator,
		HighRAM:            ramTier == "high",
		Group:              "private",
		Priority:           0,
		Weight:             1,
		Enabled:            1,
		IdleTimeoutSeconds: 0, // Controlled explicitly by user Stop or zero-balance watchdog
		Status:             "provisioning",
		StatusMessage:      "Starting private fine-tuned model server...",
		CallbackToken:      depToken,
		CreatedTime:        now,
		UpdatedTime:        now,
	}
	if err := model.CreateNotebookDeployment(dep); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	callbackURL := buildDeploymentCallbackURL(c, dep.Id, depToken)
	script := notebookscript.Generate(notebookscript.Config{
		Provider:               cand.Provider,
		DeploymentName:         deployName,
		ModelRepo:              job.Model,
		TaskType:               taskType,
		Accelerator:            cand.Accelerator,
		IdleTimeoutSeconds:     0,
		HFToken:                hfToken,
		CallbackURL:            callbackURL,
		ShutdownToken:          depToken,
		FineTunedModelID:       job.FineTunedModel,
		FineTunedArtifactURL:   artifactURL,
		FineTunedArtifactToken: callbackToken,
		FineTuningMethod:       job.Method,
	})
	dep.Script = script
	_ = model.UpdateNotebookDeploymentFields(dep.Id, map[string]any{"script": script})

	_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
		"serving_status":         "starting",
		"serving_deployment_id":  dep.Id,
		"serving_provider":       cand.Provider,
		"serving_gpu":            gpuID,
		"serving_ram_tier":       ramTier,
		"serving_endpoint_url":   "",
		"serving_started_at":     0,
		"serving_last_billed_at": 0,
		"serving_error":          "",
		"serving_stop_reason":    "",
		"updated_at":             now,
	})

	go provisionDeploymentAsync(provisionParams{
		ID:                     dep.Id,
		Provider:               cand.Provider,
		AccountID:              cand.Account.ID,
		Name:                   deployName,
		ModelRepo:              job.Model,
		TaskType:               taskType,
		PrimaryAccelerator:     cand.Accelerator,
		Candidates:             []string{cand.Accelerator},
		HighRAM:                ramTier == "high",
		IdleTimeoutSeconds:     0,
		HFToken:                hfToken,
		CallbackURL:            callbackURL,
		CallbackToken:          depToken,
		Script:                 script,
		ModalTokenID:           modalTokenID,
		ModalTokenSecret:       modalTokenSecret,
		ModalVolPath:           modalVolPath,
		KaggleUser:             kaggleUser,
		KaggleKey:              kaggleKey,
		FineTunedModelID:       job.FineTunedModel,
		FineTunedArtifactURL:   artifactURL,
		FineTunedArtifactToken: callbackToken,
		FineTuningMethod:       job.Method,
	})

	updated, _ := model.GetFineTuningJobByJobID(job.JobID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": updated})
}

// UserStopFineTuningServing stops a user's active/starting hosted fine-tuned model,
// settles final elapsed compute seconds, tears down remote compute, and removes
// the model from /v1/models and Playground immediately.
func UserStopFineTuningServing(c *gin.Context) {
	userId := c.GetInt("id")
	jobID := strings.TrimSpace(c.Param("job_id"))
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || (job.UserId != userId && c.GetInt("role") < common.RoleAdminUser) {
		c.JSON(http.StatusOK, gin.H{"success": false, "code": "FT_JOB_NOT_FOUND", "message": "Fine-tuning job not found"})
		return
	}
	stopFineTuningServingJob(job.JobID, "user_stopped")
	updated, _ := model.GetFineTuningJobByJobID(job.JobID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": updated})
}

var (
	ftServingOpMu sync.Mutex
)

// billFineTuningServingDelta deducts elapsed serving seconds safely and reports
// whether the user's balance has been depleted.
func billFineTuningServingDelta(job *model.FineTuningJob, now int64) (depleted bool) {
	if job == nil || job.ServingStatus != "running" {
		return false
	}
	lastBilled := job.ServingLastBilledAt
	if lastBilled <= 0 {
		lastBilled = job.ServingStartedAt
	}
	if lastBilled <= 0 || now <= lastBilled {
		return false
	}
	deltaSec := now - lastBilled
	if deltaSec > 300 {
		// Guard against long server downtime double-charging while offline
		deltaSec = 300
	}

	pricePerSec := service.ResolveFineTuningServingPricePerSecond(job.ServingGPU, job.ServingRamTier)
	costUSD := float64(deltaSec) * pricePerSec
	deltaQuota, _ := common.QuotaRoundChecked(costUSD * common.QuotaPerUnit)

	userQuota, qErr := model.GetUserQuota(job.UserId, true)
	if qErr != nil {
		return false
	}
	if userQuota <= 0 {
		return true
	}

	charge := deltaQuota
	if charge > userQuota {
		charge = userQuota
		depleted = true
	} else if userQuota-charge <= 0 {
		depleted = true
	}

	if charge > 0 {
		if err := model.DecreaseUserQuota(job.UserId, charge, false); err == nil {
			model.UpdateUserUsedQuotaAndRequestCount(job.UserId, charge)
		}
	}

	_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
		"serving_last_billed_at": now,
		"serving_billed_seconds": job.ServingBilledSeconds + deltaSec,
		"serving_billed_quota":   job.ServingBilledQuota + int64(charge),
		"updated_at":             now,
	})
	job.ServingLastBilledAt = now
	job.ServingBilledSeconds += deltaSec
	job.ServingBilledQuota += int64(charge)
	return depleted
}

func stopFineTuningServingJob(jobID string, reason string) {
	ftServingOpMu.Lock()
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || job == nil {
		ftServingOpMu.Unlock()
		return
	}
	now := time.Now().Unix()
	if job.ServingStatus == "running" {
		_ = billFineTuningServingDelta(job, now)
	}
	depID := job.ServingDeploymentID
	_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
		"serving_status":        "stopped",
		"serving_deployment_id": 0,
		"serving_endpoint_url":  "",
		"serving_stop_reason":   reason,
		"updated_at":            now,
	})
	ftServingOpMu.Unlock()

	if depID > 0 {
		if dep, dErr := model.GetNotebookDeploymentByID(depID); dErr == nil && dep != nil {
			if stopErr := stopNotebookDeploymentRemote(dep); stopErr != nil {
				common.SysError(fmt.Sprintf("fine-tuning serving remote stop failed for job %s (dep #%d): %v", jobID, depID, stopErr))
			}
			_ = model.DeleteNotebookDeployment(depID)
		}
	}
}

func tickFineTuningServingWatchdog(_ context.Context) {
	jobs, err := model.ListAllActiveOrStartingServingFineTuningJobs()
	if err != nil || len(jobs) == 0 {
		return
	}
	now := time.Now().Unix()
	servingEnabled := service.IsFineTuningServingEnabled()

	for i := range jobs {
		job := &jobs[i]

		// If admin disabled fine-tuned model hosting, stop all active runtimes cleanly.
		if !servingEnabled {
			stopFineTuningServingJob(job.JobID, "serving_disabled")
			continue
		}

		// Sync starting state with linked private NotebookDeployment
		if job.ServingStatus == "starting" {
			if job.ServingDeploymentID <= 0 {
				_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
					"serving_status": "failed",
					"serving_error":  "Deployment record missing",
					"updated_at":     now,
				})
				continue
			}
			dep, dErr := model.GetNotebookDeploymentByID(job.ServingDeploymentID)
			if dErr != nil || dep == nil {
				_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
					"serving_status":        "failed",
					"serving_deployment_id": 0,
					"serving_error":         "Deployment record missing",
					"updated_at":            now,
				})
				continue
			}
			switch strings.ToLower(strings.TrimSpace(dep.Status)) {
			case "running":
				if strings.TrimSpace(dep.EndpointURL) != "" {
					_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
						"serving_status":         "running",
						"serving_endpoint_url":   dep.EndpointURL,
						"serving_started_at":     now,
						"serving_last_billed_at": now,
						"serving_error":          "",
						"updated_at":             now,
					})
				}
			case "failed", "stopped":
				errMsg := strings.TrimSpace(dep.ErrorMessage)
				if errMsg == "" {
					errMsg = strings.TrimSpace(dep.StatusMessage)
				}
				if errMsg == "" {
					errMsg = "Hosted model runtime failed to start"
				}
				_ = stopNotebookDeploymentRemote(dep)
				_ = model.DeleteNotebookDeployment(dep.Id)
				_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
					"serving_status":        "failed",
					"serving_deployment_id": 0,
					"serving_endpoint_url":  "",
					"serving_error":         service.SanitizeFineTuningLogLine(errMsg),
					"serving_stop_reason":   "provider_error",
					"updated_at":            now,
				})
			}
			continue
		}

		// Running job: bill elapsed seconds & immediately tear down if user balance hit 0
		if job.ServingStatus == "running" {
			ftServingOpMu.Lock()
			fresh, fErr := model.GetFineTuningJobByJobID(job.JobID)
			if fErr != nil || fresh == nil || fresh.ServingStatus != "running" {
				ftServingOpMu.Unlock()
				continue
			}
			depleted := billFineTuningServingDelta(fresh, now)
			ftServingOpMu.Unlock()

			if depleted {
				common.SysLog(fmt.Sprintf("fine-tuning serving auto-stopped for job %s (user #%d): insufficient balance", job.JobID, job.UserId))
				stopFineTuningServingJob(job.JobID, "insufficient_balance")
			}
		}
	}
}

var ftServingWorkerOnce sync.Once

// StartFineTuningServingBillingWorker runs the 15-second status sync, compute-time
// billing, and automatic zero-balance teardown loop for hosted fine-tuned models.
func StartFineTuningServingBillingWorker() {
	ftServingWorkerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				tickFineTuningServingWatchdog(context.Background())
			}
		}()
	})
}
