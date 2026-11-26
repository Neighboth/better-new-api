package service

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// FineTuningDispatchResult reports execution telemetry from a remote provider
// run so billing can distinguish pre-compute provisioning failures ($0) from
// mid-training OOM / runtime errors (billed for actual consumed compute time).
type FineTuningDispatchResult struct {
	ArtifactZipPath       string
	TrainedTokens         int64
	ComputeStarted        bool
	ElapsedComputeSeconds int64
	BilledOnFailure       bool
}

type FineTuningDispatchFunc func(ctx context.Context, job *model.FineTuningJob, epochs int) (FineTuningDispatchResult, error)

var (
	ftRunnersMu    sync.Mutex
	ftActiveCancel = map[string]context.CancelFunc{}
	ftDispatcher   FineTuningDispatchFunc

	ftLogMu          sync.Mutex
	ftRecentLogLines = map[string][]string{}

	ftProviderLeakRe = regexp.MustCompile(`(?i)\b(google\s+colab|colab|modal|kaggle|runpod|hostinger|io\.net|ionet|huggingface\s+spaces|trycloudflare\.com)\b`)
	ftURLRe          = regexp.MustCompile(`https?://[^\s"']+`)
	ftIPv4Re         = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b`)
	ftSecretRe       = regexp.MustCompile(`(?i)(sk-[a-zA-Z0-9_-]{8,}|hf_[a-zA-Z0-9]{8,}|ak-[a-zA-Z0-9_-]{8,}|bearer\s+[a-zA-Z0-9._-]{8,}|token=[a-zA-Z0-9._-]+)`)
	ftStampedLineRe  = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}\s+\[[^\]]+\]\s+`)
)

// RegisterFineTuningDispatcher sets the real compute provider dispatcher
// (Modal / Google Colab / Kaggle) invoked when a fine-tuning job starts.
func RegisterFineTuningDispatcher(fn FineTuningDispatchFunc) {
	ftRunnersMu.Lock()
	ftDispatcher = fn
	ftRunnersMu.Unlock()
}

// AppendFineTuningLogFromWorker writes a sanitized log line emitted by a remote
// fine-tuning worker into the job's persistent console log.
func AppendFineTuningLogFromWorker(userID int, jobID, stage, msg string) {
	appendFineTuningLog(userID, jobID, stage, msg)
}

// InspectFineTuningDatasetFile returns sample count, byte size, format, and
// estimated tokens for an uploaded dataset file on disk.
func InspectFineTuningDatasetFile(path string, fallbackName string) (samples int64, byteSize int64, format string, estTokens int64) {
	ds := inspectFineTuningDataset(path, fallbackName)
	return int64(ds.SampleCount), ds.ByteSize, ds.Format, ds.EstTokens
}

// SanitizeFineTuningLogLine strips any upstream provider names, URLs, host IPs,
// and secret tokens so the user-facing console shows detailed training progress
// without ever leaking backend infrastructure or credentials.
func SanitizeFineTuningLogLine(line string) string {
	clean := ftSecretRe.ReplaceAllString(line, "[redacted-secret]")
	clean = ftURLRe.ReplaceAllString(clean, "[redacted-endpoint]")
	clean = ftIPv4Re.ReplaceAllString(clean, "[redacted-host]")
	clean = ftProviderLeakRe.ReplaceAllString(clean, "compute-node")
	return clean
}

func FineTuningLogPath(userID int, jobID string) string {
	return filepath.Join(FineTuningStorageDirForUser(userID, jobID), "train.log")
}

// FineTuningCheckpointZipPath returns the canonical path where the latest
// resumable training checkpoint archive is persisted on the server.
func FineTuningCheckpointZipPath(userID int, jobID string) string {
	return filepath.Join(FineTuningStorageDirForUser(userID, jobID), "checkpoint_latest.zip")
}

func appendFineTuningLog(userID int, jobID, stage, msg string) {
	trimmed := strings.TrimSpace(msg)
	if trimmed == "" {
		return
	}
	sanitized := SanitizeFineTuningLogLine(trimmed)
	var formatted string
	// Avoid double timestamps / double tags when the remote script already formatted "HH:MM:SS [stage] ..."
	if ftStampedLineRe.MatchString(sanitized) {
		formatted = sanitized
	} else {
		ts := time.Now().UTC().Format("15:04:05")
		formatted = fmt.Sprintf("%s [%s] %s", ts, stage, sanitized)
	}

	// Deduplicate identical lines that arrive via both HTTP callback and stdout polling
	dedupKey := formatted
	if ftStampedLineRe.MatchString(dedupKey) {
		// Strip leading timestamp so a 1-second skew between callback and stdout still deduplicates
		dedupKey = strings.TrimSpace(dedupKey[8:])
	}

	ftLogMu.Lock()
	recent := ftRecentLogLines[jobID]
	for _, prev := range recent {
		if prev == dedupKey {
			ftLogMu.Unlock()
			return
		}
	}
	recent = append(recent, dedupKey)
	if len(recent) > 24 {
		recent = recent[len(recent)-24:]
	}
	ftRecentLogLines[jobID] = recent
	ftLogMu.Unlock()

	dir := FineTuningStorageDirForUser(userID, jobID)
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(FineTuningLogPath(userID, jobID), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(formatted + "\n")
	_ = f.Close()
}

// ReadFineTuningJobLogs returns the sanitized live console log for a job.
func ReadFineTuningJobLogs(userID int, jobID string) string {
	paths := []string{
		FineTuningLogPath(userID, jobID),
		filepath.Join("data", "fine_tuning", sanitizeFineTuningSegment(jobID), "train.log"), // legacy fallback
	}
	var raw []byte
	for _, p := range paths {
		if b, err := os.ReadFile(p); err == nil && len(b) > 0 {
			raw = b
			break
		}
	}
	if len(raw) == 0 {
		return ""
	}
	lines := strings.Split(string(raw), "\n")
	for i := range lines {
		lines[i] = SanitizeFineTuningLogLine(lines[i])
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// CancelFineTuningJobRunner signals any active background training worker for jobID to stop.
func CancelFineTuningJobRunner(jobID string) {
	ftRunnersMu.Lock()
	cancel, ok := ftActiveCancel[jobID]
	delete(ftActiveCancel, jobID)
	ftRunnersMu.Unlock()
	if ok && cancel != nil {
		cancel()
	}
}

type datasetStats struct {
	SampleCount int
	ByteSize    int64
	Format      string
	EstTokens   int64
	SHA256      string
}

func inspectFineTuningDataset(path string, fallbackName string) datasetStats {
	stats := datasetStats{
		SampleCount: 16,
		Format:      strings.TrimPrefix(strings.ToLower(filepath.Ext(fallbackName)), "."),
		EstTokens:   4096,
	}
	if stats.Format == "" {
		stats.Format = "jsonl"
	}
	if path == "" {
		return stats
	}
	fi, err := os.Stat(path)
	if err != nil {
		return stats
	}
	stats.ByteSize = fi.Size()
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if ext != "" {
		stats.Format = ext
	}
	if f, hErr := os.Open(path); hErr == nil {
		h := sha256.New()
		buf := make([]byte, 32*1024)
		for i := 0; i < 64; i++ {
			n, rErr := f.Read(buf)
			if n > 0 {
				_, _ = h.Write(buf[:n])
			}
			if rErr != nil {
				break
			}
		}
		stats.SHA256 = hex.EncodeToString(h.Sum(nil))
		_ = f.Close()
	}

	switch stats.Format {
	case "zip":
		if zr, err := zip.OpenReader(path); err == nil {
			count := 0
			for _, f := range zr.File {
				if !f.FileInfo().IsDir() {
					count++
				}
			}
			_ = zr.Close()
			if count > 0 {
				stats.SampleCount = count
			}
		}
		stats.EstTokens = int64(stats.SampleCount) * 512
	case "jsonl", "csv", "txt":
		if f, err := os.Open(path); err == nil {
			scanner := bufio.NewScanner(f)
			scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
			lines := 0
			for scanner.Scan() {
				if strings.TrimSpace(scanner.Text()) != "" {
					lines++
				}
			}
			_ = f.Close()
			if stats.Format == "csv" && lines > 1 {
				lines-- // exclude header
			}
			if lines > 0 {
				stats.SampleCount = lines
			}
		}
		est := stats.ByteSize / 4
		if est < int64(stats.SampleCount)*64 {
			est = int64(stats.SampleCount) * 64
		}
		stats.EstTokens = est
	case "json":
		if raw, err := os.ReadFile(path); err == nil {
			var arr []any
			if common.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
				stats.SampleCount = len(arr)
			}
		}
		est := stats.ByteSize / 4
		if est < 512 {
			est = 512
		}
		stats.EstTokens = est
	default:
		estSamples := int(stats.ByteSize / 512)
		if estSamples < 8 {
			estSamples = 8
		}
		stats.SampleCount = estSamples
		stats.EstTokens = int64(estSamples) * 256
	}
	return stats
}

// StartFineTuningJobRunner launches an asynchronous training execution worker
// for a newly created job so it transitions queued -> running -> succeeded/failed/cancelled,
// persists epoch checkpoints + final downloadable .zip archive (with a valid
// binary .safetensors model file, checkpoints, detailed metadata, and logs),
// and streams sanitized logs to uploads/fine_tuning/<user_id>/<job_id>/train.log.
func StartFineTuningJobRunner(jobID string) {
	ftRunnersMu.Lock()
	if _, exists := ftActiveCancel[jobID]; exists {
		ftRunnersMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	ftActiveCancel[jobID] = cancel
	ftRunnersMu.Unlock()

	go runFineTuningJob(ctx, jobID)
}

func runFineTuningJob(ctx context.Context, jobID string) {
	defer func() {
		ftRunnersMu.Lock()
		delete(ftActiveCancel, jobID)
		ftRunnersMu.Unlock()
		ftLogMu.Lock()
		delete(ftRecentLogLines, jobID)
		ftLogMu.Unlock()
	}()

	job, err := model.GetFineTuningJobByJobID(jobID)
	if err != nil || job == nil {
		return
	}
	if job.Status == "cancelled" || job.Status == "succeeded" || job.Status == "failed" {
		return
	}

	userID := job.UserId
	taskType := nonEmptyOr(job.TaskType, "text")
	method := nonEmptyOr(job.Method, "lora")
	gpuLabel := job.GPU
	for _, opt := range GetFineTuningGPUOptions() {
		if opt.ID == job.GPU {
			gpuLabel = opt.Label
			break
		}
	}

	epochs := 3
	var hyper map[string]any
	if common.UnmarshalJsonStr(job.Hyperparameters, &hyper) == nil && hyper != nil {
		if ep, ok := hyper["n_epochs"].(float64); ok && int(ep) >= 1 && int(ep) <= 50 {
			epochs = int(ep)
		}
	}

	appendFineTuningLog(userID, jobID, "queue", fmt.Sprintf("Job accepted: task=%s method=%s accelerator=%s epochs=%d", taskType, strings.ToUpper(method), gpuLabel, epochs))

	ds := inspectFineTuningDataset(job.TrainingFilePath, job.TrainingFile)
	appendFineTuningLog(userID, jobID, "dataset", fmt.Sprintf("Dataset verified: %q (%s format, %d samples, %.2f MB)", filepath.Base(job.TrainingFile), strings.ToUpper(ds.Format), ds.SampleCount, float64(ds.ByteSize)/(1024.0*1024.0)))

	ftRunnersMu.Lock()
	dispatcher := ftDispatcher
	ftRunnersMu.Unlock()

	if dispatcher == nil {
		errMsg := "Real fine-tuning compute dispatcher is not initialized. Configure an enabled provider in System Settings > Model Deployment."
		appendFineTuningLog(userID, jobID, "error", errMsg)
		_ = model.UpdateFineTuningJobStatus(jobID, map[string]any{
			"status":      "failed",
			"error":       errMsg,
			"finished_at": time.Now().Unix(),
			"updated_at":  time.Now().Unix(),
		})
		return
	}

	startedAt := time.Now().Unix()
	_ = model.UpdateFineTuningJobStatus(jobID, map[string]any{
		"status":     "running",
		"progress":   2,
		"updated_at": startedAt,
	})

	billConsumedCompute := func(elapsedSec int64) (float64, int64) {
		if elapsedSec <= 0 {
			return 0, 0
		}
		costUSD := EstimateFineTuningCostUSDWithRAM(job.GPU, job.RamTier, elapsedSec)
		if costUSD <= 0 {
			return 0, 0
		}
		q := common.QuotaRound(costUSD * common.QuotaPerUnit)
		if q <= 0 {
			return costUSD, 0
		}
		// Never underflow user quota below zero when billing partial OOM/failure compute
		if avail, qErr := model.GetUserQuota(userID, false); qErr == nil && avail > 0 && q > avail {
			q = avail
		}
		if err := model.DecreaseUserQuota(userID, q, false); err == nil {
			model.UpdateUserUsedQuotaAndRequestCount(userID, q)
			return costUSD, int64(q)
		}
		return costUSD, 0
	}

	dispatchRes, runErr := dispatcher(ctx, job, epochs)
	if runErr != nil {
		if ctx.Err() != nil {
			handleRunnerCancelled(userID, jobID)
			return
		}
		finishedAt := time.Now().Unix()
		errMsg := SanitizeFineTuningLogLine(runErr.Error())

		// If training compute actually started and failed due to OOM / runtime
		// error (especially after acknowledging a hardware compatibility warning),
		// charge for the consumed compute time instead of giving free GPU time.
		shouldBill := dispatchRes.ComputeStarted && (dispatchRes.BilledOnFailure || job.CompatWarningAcknowledged)
		if shouldBill {
			billedSec := dispatchRes.ElapsedComputeSeconds
			if billedSec < 1 {
				billedSec = finishedAt - startedAt
			}
			if billedSec < 1 {
				billedSec = 1
			}
			costUSD, billedQuota := billConsumedCompute(billedSec)
			appendFineTuningLog(userID, jobID, "error", fmt.Sprintf("Training failed after %ds of compute (charged $%.4f for consumed compute): %s", billedSec, costUSD, errMsg))
			_ = model.UpdateFineTuningJobStatus(jobID, map[string]any{
				"status":             "failed",
				"error":              errMsg,
				"duration_seconds":   billedSec,
				"billed_seconds":     billedSec,
				"estimated_cost_usd": costUSD,
				"billed_quota":       billedQuota,
				"finished_at":        finishedAt,
				"updated_at":         finishedAt,
			})
			return
		}

		appendFineTuningLog(userID, jobID, "error", "Real training failed before compute started (no balance charged): "+errMsg)
		_ = model.UpdateFineTuningJobStatus(jobID, map[string]any{
			"status":      "failed",
			"error":       errMsg,
			"finished_at": finishedAt,
			"updated_at":  finishedAt,
		})
		return
	}

	artifactZipPath := dispatchRes.ArtifactZipPath
	trainedTokens := dispatchRes.TrainedTokens

	if fresh, gErr := model.GetFineTuningJobByJobID(jobID); gErr == nil && fresh != nil && fresh.Status == "cancelled" {
		handleRunnerCancelled(userID, jobID)
		return
	}

	if info, statErr := os.Stat(artifactZipPath); statErr != nil || info.Size() == 0 {
		errMsg := "Training worker finished without producing a downloadable .zip model archive"
		appendFineTuningLog(userID, jobID, "error", errMsg)
		_ = model.UpdateFineTuningJobStatus(jobID, map[string]any{
			"status":      "failed",
			"error":       errMsg,
			"finished_at": time.Now().Unix(),
			"updated_at":  time.Now().Unix(),
		})
		return
	}

	finishedAt := time.Now().Unix()
	elapsedSec := dispatchRes.ElapsedComputeSeconds
	if elapsedSec < 1 {
		elapsedSec = finishedAt - startedAt
	}
	if elapsedSec < 1 {
		elapsedSec = 1
	}
	retentionDays := GetFineTuningRetentionDays()
	expiresAt := finishedAt + int64(retentionDays)*86400

	actualCostUSD, billedQuota := billConsumedCompute(elapsedSec)

	_ = model.UpdateFineTuningJobStatus(jobID, map[string]any{
		"status":             "succeeded",
		"progress":           100,
		"trained_tokens":     trainedTokens,
		"duration_seconds":   elapsedSec,
		"billed_seconds":     elapsedSec,
		"estimated_cost_usd": actualCostUSD,
		"billed_quota":       billedQuota,
		"model_file_path":    artifactZipPath,
		"finished_at":        finishedAt,
		"expires_at":         expiresAt,
		"updated_at":         finishedAt,
	})
	appendFineTuningLog(userID, jobID, "done", fmt.Sprintf("Real fine-tuning complete in %ds! Model %s ready for download (.zip, trained_tokens=%d, cost=$%.4f, expires=%s).", elapsedSec, job.FineTunedModel, trainedTokens, actualCostUSD, time.Unix(expiresAt, 0).UTC().Format("2006-01-02")))
}

func handleRunnerCancelled(userID int, jobID string) {
	appendFineTuningLog(userID, jobID, "cancelled", "Training cancelled by user. Completed epoch checkpoints remain preserved until deletion.")
	_ = model.UpdateFineTuningJobStatus(jobID, map[string]any{
		"status":      "cancelled",
		"finished_at": time.Now().Unix(),
		"updated_at":  time.Now().Unix(),
	})
}

func nonEmptyOr(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

// StartFineTuningRetentionWorker periodically warns users before their trained
// fine-tuning models expire, purges expired model files after retention_days,
// and cleans up orphaned staged model uploads older than 6 hours.
func StartFineTuningRetentionWorker() {
	go func() {
		time.Sleep(2 * time.Minute)
		runFineTuningRetentionSweep()
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			runFineTuningRetentionSweep()
		}
	}()
}

func runFineTuningRetentionSweep() {
	if model.DB == nil {
		return
	}
	now := time.Now().Unix()
	warnDays := GetFineTuningExpiryWarningDays()

	// 1. Send pre-expiry warning emails
	if warnDays > 0 {
		warnHorizon := now + int64(warnDays)*86400
		if jobs, err := model.ListFineTuningJobsExpiringSoon(warnHorizon, now); err == nil {
			for i := range jobs {
				sendFineTuningExpiryWarningEmail(&jobs[i])
				_ = model.UpdateFineTuningJobStatus(jobs[i].JobID, map[string]any{
					"expiry_warning_sent_at": now,
					"updated_at":             now,
				})
			}
		}
	}

	// 2. Purge expired model files (keep DB record for audit/stats)
	if expired, err := model.ListFineTuningJobsExpired(now); err == nil {
		for i := range expired {
			CleanupFineTuningStorageForUser(expired[i].UserId, expired[i].JobID)
			_ = model.UpdateFineTuningJobStatus(expired[i].JobID, map[string]any{
				"files_purged_at": now,
				"model_file_path": "",
				"updated_at":      now,
			})
		}
	}

	// 3. Clean orphaned staged uploads older than 6 hours
	cleanupOrphanedStagedFineTuningUploads(6 * time.Hour)
}

func sendFineTuningExpiryWarningEmail(job *model.FineTuningJob) {
	cache, err := model.GetUserCache(job.UserId)
	if err != nil || strings.TrimSpace(cache.Email) == "" {
		return
	}
	var userSetting dto.UserSetting
	lang := "en"
	if raw := strings.TrimSpace(cache.Setting); raw != "" {
		if err := common.Unmarshal([]byte(raw), &userSetting); err == nil && strings.TrimSpace(userSetting.Language) != "" {
			lang = NormalizeEmailLang(userSetting.Language)
		}
	}
	expiresAtStr := time.Unix(job.ExpiresAt, 0).UTC().Format("2006-01-02")
	daysLeft := int(math.Ceil(float64(job.ExpiresAt-time.Now().Unix()) / 86400.0))
	if daysLeft < 1 {
		daysLeft = 1
	}
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	downloadURL := base + "/custom-models"
	subject, content := RenderEmail("fine_tuning_expiry", lang, map[string]string{
		"username":     cache.Username,
		"model":        nonEmptyOr(job.FineTunedModel, job.JobID),
		"job_id":       job.JobID,
		"days_left":    strconv.Itoa(daysLeft),
		"expires_at":   expiresAtStr,
		"download_url": downloadURL,
	})
	if err := common.SendEmail(subject, cache.Email, content); err != nil {
		common.SysError(fmt.Sprintf("fine-tuning expiry warning mail failed for user %d (job %s): %s", job.UserId, job.JobID, err.Error()))
	}
}

func cleanupOrphanedStagedFineTuningUploads(maxAge time.Duration) {
	root := filepath.Join("uploads", "fine_tuning")
	userDirs, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, uDir := range userDirs {
		if !uDir.IsDir() {
			continue
		}
		stagedRoot := filepath.Join(root, uDir.Name(), "_staged")
		entries, err := os.ReadDir(stagedRoot)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if info, iErr := entry.Info(); iErr == nil && info.ModTime().Before(cutoff) {
				_ = os.RemoveAll(filepath.Join(stagedRoot, entry.Name()))
			}
		}
	}
}
