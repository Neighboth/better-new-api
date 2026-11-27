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

	// Clear any server HTTP write deadline so multi-hour / 100GB+ downloads run uninterrupted to EOF
	// even if the signed ticket expires after the stream has started.
	if rc := http.NewResponseController(c.Writer); rc != nil {
		_ = rc.SetWriteDeadline(time.Time{})
	}

	// Disable proxy response buffering so multi-GB archives stream first bytes immediately.
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, strings.ReplaceAll(safeName, `"`, "")))
	c.Header("Cache-Control", "no-store")
	c.Header("X-Accel-Buffering", "no")
	http.ServeContent(c.Writer, c.Request, safeName, fi.ModTime(), f)
}

const fineTuningDownloadTicketTTL = 24 * time.Hour

// UserCreateFineTuningDownloadTicket issues a signed download URL (24h sliding TTL) so
// the browser streams the .zip archive directly without Axios RAM buffering or timeouts.
// Once a transfer begins it runs to EOF with no write deadline even if it takes days.
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
		ExpiresAt: now.Add(fineTuningDownloadTicketTTL),
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
			"ttl_seconds":  int64(fineTuningDownloadTicketTTL.Seconds()),
		},
	})
}

// DownloadFineTuningModelWithTicket streams the trained .zip archive directly to
// the browser with HTTP Range/resume support. Each valid resume request slides the
// ticket expiry forward by 24h, and active streams run uninterrupted to EOF.
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
	if ok && t.JobID == jobID && !now.After(t.ExpiresAt) {
		// Slide expiry on active/resumed Range transfers so slow multi-day downloads never break on resume
		t.ExpiresAt = now.Add(fineTuningDownloadTicketTTL)
		ftDownloadTickets[ticketToken] = t
	}
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
	GPU                string `json:"gpu"`
	RamTier            string `json:"ram_tier"`
	IdleTimeoutSeconds int    `json:"idle_timeout_seconds"`
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

func normalizeServingIdleTimeout(sec int) int {
	if sec <= 0 {
		return 0
	}
	if sec < 60 {
		return 60
	}
	if sec > 86400 {
		return 86400
	}
	return sec
}

// launchFineTuningServingRuntime provisions a private NotebookDeployment for a trained
// fine-tuning job. Shared by manual UserStartFineTuningServing and automatic wake-on-request.
func launchFineTuningServingRuntime(ctx context.Context, c *gin.Context, job *model.FineTuningJob, gpuID, ramTier string, idleTimeoutSeconds int) (code string, err error) {
	if !service.IsFineTuningServingEnabled() {
		return "FT_SERVING_DISABLED", fmt.Errorf("Fine-tuned model hosting is disabled in System Settings")
	}
	if job == nil || job.Status != "succeeded" {
		return "FT_MODEL_NOT_READY", fmt.Errorf("Model is not ready yet; training must complete first")
	}
	if job.FilesPurgedAt > 0 || job.ModelFilePath == "" {
		return "FT_MODEL_EXPIRED", fmt.Errorf("Model files have expired and been purged from storage")
	}
	if _, statErr := os.Stat(job.ModelFilePath); statErr != nil {
		return "FT_MODEL_MISSING", fmt.Errorf("Model archive file not found on disk")
	}

	gpuID = strings.TrimSpace(gpuID)
	ramTier = strings.ToLower(strings.TrimSpace(ramTier))
	if ramTier != "high" {
		ramTier = "normal"
	}
	idleTimeoutSeconds = normalizeServingIdleTimeout(idleTimeoutSeconds)

	gpuOpt, ok := service.FindEnabledFineTuningServingGPU(gpuID)
	if !ok || gpuOpt == nil {
		return "FT_SERVING_GPU_DISABLED", fmt.Errorf("Selected accelerator is not enabled for model hosting")
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
	if userQuota, qErr := model.GetUserQuota(job.UserId, true); qErr == nil && userQuota < minQuota {
		return "FT_SERVING_INSUFFICIENT_BALANCE", fmt.Errorf("Insufficient balance to activate this model")
	}

	cand, candErr := resolveServingProviderCandidate(gpuID)
	if candErr != nil || cand == nil {
		return "FT_SERVING_NO_PROVIDER", fmt.Errorf("No enabled compute provider account supports the selected accelerator")
	}

	callbackToken := strings.TrimSpace(job.CallbackToken)
	if callbackToken == "" {
		callbackToken = common.GetUUID()
		_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{"callback_token": callbackToken})
	}

	serverBase := resolvePublicServerBaseURL(c)
	artifactURL := fmt.Sprintf("%s/api/deployments/fine_tuning/%s/artifact_download?token=%s",
		serverBase, url.PathEscape(job.JobID), url.QueryEscape(callbackToken))

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
		if _, tokErr := getValidColabAccessTokenForAccount(ctx, cand.Account.ID); tokErr != nil {
			return "FT_SERVING_NO_PROVIDER", tokErr
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
		OwnerUserID:        job.UserId,
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
		IdleTimeoutSeconds: 0, // Managed by fine-tuning watchdog so sleeping/auto-wake stays synchronized
		Status:             "provisioning",
		StatusMessage:      "Starting private fine-tuned model server...",
		CallbackToken:      depToken,
		CreatedTime:        now,
		UpdatedTime:        now,
	}
	if err := model.CreateNotebookDeployment(dep); err != nil {
		return "FT_SERVING_DB_ERROR", err
	}

	callbackURL := buildDeploymentCallbackURL(c, dep.Id, depToken)
	if callbackURL == "" && serverBase != "" {
		callbackURL = fmt.Sprintf("%s/api/deployments/%d/callback?token=%s", serverBase, dep.Id, url.QueryEscape(depToken))
	}
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
		"serving_status":               "starting",
		"serving_deployment_id":        dep.Id,
		"serving_provider":             cand.Provider,
		"serving_gpu":                  gpuID,
		"serving_ram_tier":             ramTier,
		"serving_idle_timeout_seconds": idleTimeoutSeconds,
		"serving_auto_wake":            idleTimeoutSeconds > 0,
		"serving_endpoint_url":         "",
		"serving_started_at":           0,
		"serving_last_billed_at":       0,
		"serving_last_request_at":      now,
		"serving_error":                "",
		"serving_stop_reason":          "",
		"updated_at":                   now,
	})

	idleDesc := "disabled"
	if idleTimeoutSeconds > 0 {
		idleDesc = fmt.Sprintf("%ds (auto-sleep & auto-wake enabled)", idleTimeoutSeconds)
	}
	service.AppendFineTuningServingLog(job.UserId, job.JobID, "serve",
		fmt.Sprintf("Launching private model runtime for %q (accelerator=%s, ram_tier=%s, idle_sleep=%s)", job.FineTunedModel, gpuID, ramTier, idleDesc))

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
	return "", nil
}

// UserStartFineTuningServing launches a private compute instance hosting the
// user's trained fine-tuned model (visible only in that user's Playground, /v1/models & Model Plaza).
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
	code, launchErr := launchFineTuningServingRuntime(c.Request.Context(), c, job, req.GPU, req.RamTier, req.IdleTimeoutSeconds)
	if launchErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"code":    code,
			"message": launchErr.Error(),
		})
		return
	}

	updated, _ := model.GetFineTuningJobByJobID(job.JobID)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": updated})
}

// UserStopFineTuningServing stops a user's active/starting/sleeping hosted fine-tuned model,
// settles final elapsed compute seconds, tears down remote compute, and removes
// the model from /v1/models, Playground, and Model Plaza immediately.
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
	ftServingOpMu   sync.Mutex
	ftServingWakeMu sync.Mutex
	ftServingWaking = map[string]chan struct{}{}
)

// wakeFineTuningServingAndWait brings a sleeping (or starting) auto-wake fine-tuned
// model online and waits until it reaches "running" with a live endpoint URL.
// Concurrent requests for the same sleeping job coalesce on a single boot sequence.
func wakeFineTuningServingAndWait(ctx context.Context, jobID string) (*model.FineTuningJob, error) {
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || job == nil {
		return nil, fmt.Errorf("Fine-tuning job not found")
	}
	if job.ServingStatus == "running" && strings.TrimSpace(job.ServingEndpointURL) != "" {
		return job, nil
	}

	ftServingWakeMu.Lock()
	waitCh, alreadyWaking := ftServingWaking[jobID]
	if !alreadyWaking {
		waitCh = make(chan struct{})
		ftServingWaking[jobID] = waitCh
		ftServingWakeMu.Unlock()

		// Re-check status inside leader after acquiring wake slot
		fresh, fErr := model.GetFineTuningJobByJobID(jobID)
		if fErr == nil && fresh != nil && fresh.ServingStatus == "sleeping" && fresh.ServingAutoWake {
			service.AppendFineTuningServingLog(fresh.UserId, fresh.JobID, "wake",
				fmt.Sprintf("Incoming request received while sleeping — auto-waking %q...", fresh.FineTunedModel))
			_, launchErr := launchFineTuningServingRuntime(
				context.Background(),
				nil,
				fresh,
				fresh.ServingGPU,
				fresh.ServingRamTier,
				fresh.ServingIdleTimeoutSeconds,
			)
			if launchErr != nil {
				service.AppendFineTuningServingLog(fresh.UserId, fresh.JobID, "error", "Auto-wake failed: "+launchErr.Error())
				ftServingWakeMu.Lock()
				delete(ftServingWaking, jobID)
				close(waitCh)
				ftServingWakeMu.Unlock()
				return nil, launchErr
			}
		}

		// Background watcher closes waitCh once running / failed / stopped / timeout
		go func() {
			defer func() {
				ftServingWakeMu.Lock()
				delete(ftServingWaking, jobID)
				close(waitCh)
				ftServingWakeMu.Unlock()
			}()
			deadline := time.Now().Add(8 * time.Minute)
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for time.Now().Before(deadline) {
				tickFineTuningServingWatchdog(context.Background())
				cur, cErr := model.GetFineTuningJobByJobID(jobID)
				if cErr != nil || cur == nil {
					return
				}
				if (cur.ServingStatus == "running" && strings.TrimSpace(cur.ServingEndpointURL) != "") ||
					cur.ServingStatus == "failed" ||
					cur.ServingStatus == "stopped" {
					return
				}
				<-ticker.C
			}
		}()
	} else {
		ftServingWakeMu.Unlock()
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-waitCh:
	}

	finalJob, fErr := model.GetFineTuningJobByJobID(jobID)
	if fErr != nil || finalJob == nil {
		return nil, fmt.Errorf("Fine-tuning job not found")
	}
	if finalJob.ServingStatus == "running" && strings.TrimSpace(finalJob.ServingEndpointURL) != "" {
		return finalJob, nil
	}
	if strings.TrimSpace(finalJob.ServingError) != "" {
		return nil, fmt.Errorf("%s", finalJob.ServingError)
	}
	return nil, fmt.Errorf("Hosted fine-tuned model did not finish waking up in time")
}

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

// sleepFineTuningServingJob pauses compute after the configured idle timeout while
// preserving auto-wake configuration so the model stays listed and resumes on the next request.
func sleepFineTuningServingJob(jobID string) {
	ftServingOpMu.Lock()
	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || job == nil || job.ServingStatus != "running" {
		ftServingOpMu.Unlock()
		return
	}
	now := time.Now().Unix()
	_ = billFineTuningServingDelta(job, now)
	depID := job.ServingDeploymentID
	userID := job.UserId
	idleSec := job.ServingIdleTimeoutSeconds
	_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
		"serving_status":        "sleeping",
		"serving_deployment_id": 0,
		"serving_endpoint_url":  "",
		"serving_stop_reason":   "idle_sleep",
		"serving_auto_wake":     true,
		"updated_at":            now,
	})
	ftServingOpMu.Unlock()

	service.AppendFineTuningServingLog(userID, jobID, "idle",
		fmt.Sprintf("No requests for %ds — compute paused (sleeping; will auto-wake on next request)", idleSec))

	if depID > 0 {
		if dep, dErr := model.GetNotebookDeploymentByID(depID); dErr == nil && dep != nil {
			if strings.TrimSpace(dep.Logs) != "" {
				for _, ln := range strings.Split(strings.TrimSpace(dep.Logs), "\n") {
					if strings.TrimSpace(ln) != "" {
						service.AppendFineTuningServingLog(userID, jobID, "runtime", ln)
					}
				}
			}
			if stopErr := stopNotebookDeploymentRemote(dep); stopErr != nil {
				common.SysError(fmt.Sprintf("fine-tuning serving remote sleep teardown failed for job %s (dep #%d): %v", jobID, depID, stopErr))
			}
			_ = model.DeleteNotebookDeployment(depID)
		}
	}
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
	userID := job.UserId
	_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
		"serving_status":        "stopped",
		"serving_deployment_id": 0,
		"serving_endpoint_url":  "",
		"serving_stop_reason":   reason,
		"serving_auto_wake":     false,
		"updated_at":            now,
	})
	ftServingOpMu.Unlock()

	service.AppendFineTuningServingLog(userID, jobID, "stop", fmt.Sprintf("Model hosting stopped (reason=%s)", reason))

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
						"serving_status":          "running",
						"serving_endpoint_url":    dep.EndpointURL,
						"serving_started_at":      now,
						"serving_last_billed_at":  now,
						"serving_last_request_at": now,
						"serving_error":           "",
						"updated_at":              now,
					})
					service.AppendFineTuningServingLog(job.UserId, job.JobID, "ready",
						fmt.Sprintf("Private model server %q is online and CUDA-warmed (ready for requests)", job.FineTunedModel))
				}
			case "failed", "stopped":
				errMsg := strings.TrimSpace(dep.ErrorMessage)
				if errMsg == "" {
					errMsg = strings.TrimSpace(dep.StatusMessage)
				}
				if errMsg == "" {
					errMsg = "Hosted model runtime failed to start"
				}
				sanitizedErr := service.SanitizeFineTuningLogLine(errMsg)
				service.AppendFineTuningServingLog(job.UserId, job.JobID, "error", "Runtime failed: "+sanitizedErr)
				_ = stopNotebookDeploymentRemote(dep)
				_ = model.DeleteNotebookDeployment(dep.Id)
				_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
					"serving_status":        "failed",
					"serving_deployment_id": 0,
					"serving_endpoint_url":  "",
					"serving_error":         sanitizedErr,
					"serving_stop_reason":   "provider_error",
					"updated_at":            now,
				})
			}
			continue
		}

		// Running job: bill elapsed seconds, tear down on zero balance, or pause on idle timeout
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
				continue
			}

			if fresh.ServingIdleTimeoutSeconds > 0 {
				lastAct := fresh.ServingLastRequestAt
				if fresh.ServingStartedAt > lastAct {
					lastAct = fresh.ServingStartedAt
				}
				if lastAct > 0 && now-lastAct >= int64(fresh.ServingIdleTimeoutSeconds) {
					common.SysLog(fmt.Sprintf("fine-tuning serving idle auto-sleep for job %s (user #%d, idle %ds >= %ds)", job.JobID, job.UserId, now-lastAct, fresh.ServingIdleTimeoutSeconds))
					sleepFineTuningServingJob(job.JobID)
				}
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
