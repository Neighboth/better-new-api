package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/finetunescript"
	"github.com/QuantumNous/new-api/pkg/kaggle"
	"github.com/QuantumNous/new-api/pkg/modaldeploy"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// InitFineTuningDispatcher wires the real remote GPU/TPU/CPU training dispatcher
// (Modal, Google Colab, Kaggle, RunPod) with 16MB chunked streaming uploads,
// cross-provider GPU translation, and automatic failover/checkpoint resume.
func InitFineTuningDispatcher() {
	service.RegisterFineTuningDispatcher(dispatchRealFineTuningJob)
	service.RegisterFineTuningServingStopHook(stopFineTuningServingJob)
	service.RegisterFineTuningServingWakeHook(wakeFineTuningServingAndWait)
}

type fineTuningCandidate struct {
	Provider    string
	Account     DeploymentAccount
	Accelerator string // mapped provider-native accelerator ID
	ExactMatch  bool
}

// canonicalAcceleratorFamily normalizes provider-specific GPU/TPU/CPU IDs into
// a common hardware family so failover can translate to equivalent accelerators
// across enabled providers.
func canonicalAcceleratorFamily(gpuID string) string {
	u := strings.ToUpper(strings.TrimSpace(gpuID))
	switch {
	case u == "CPU" || strings.HasPrefix(u, "CPU"):
		return "CPU"
	case strings.Contains(u, "B300"):
		return "B300"
	case strings.Contains(u, "B200"):
		return "B200"
	case strings.Contains(u, "H200"):
		return "H200"
	case strings.Contains(u, "H100"):
		return "H100"
	case strings.Contains(u, "RTX-PRO-6000") || strings.Contains(u, "RTX_PRO_6000"):
		return "RTX-PRO-6000"
	case strings.Contains(u, "A100") && strings.Contains(u, "40"):
		return "A100-40GB"
	case strings.Contains(u, "A100"):
		return "A100-80GB"
	case strings.Contains(u, "L40S"):
		return "L40S"
	case strings.Contains(u, "A10G"):
		return "A10G"
	case strings.Contains(u, "4090"):
		return "RTX-4090"
	case strings.Contains(u, "3090"):
		return "RTX-3090"
	case strings.Contains(u, "L4"):
		return "L4"
	case strings.Contains(u, "T4"):
		return "T4"
	case strings.Contains(u, "V6E"):
		return "TPU_V6E"
	case strings.Contains(u, "V5E"):
		return "TPU_V5E"
	case strings.Contains(u, "V2_8") || strings.Contains(u, "V2-8"):
		return "TPU_V2_8"
	default:
		return u
	}
}

// mapAcceleratorForProvider maps a requested accelerator ID to the target
// provider's native accelerator ID, returning "" when that provider cannot
// supply an equivalent accelerator (e.g. TPU on Modal/RunPod).
func mapAcceleratorForProvider(gpuID string, targetProvider string) string {
	fam := canonicalAcceleratorFamily(gpuID)
	switch strings.ToLower(strings.TrimSpace(targetProvider)) {
	case "modal":
		switch fam {
		case "CPU":
			return "CPU"
		case "T4":
			return "T4"
		case "L4":
			return "L4"
		case "A10G", "RTX-3090", "RTX-4090":
			return "A10G"
		case "L40S":
			return "L40S"
		case "A100-40GB":
			return "A100-40GB"
		case "A100-80GB":
			return "A100-80GB"
		case "RTX-PRO-6000":
			return "RTX-PRO-6000"
		case "H100":
			return "H100"
		case "H200":
			return "H200"
		case "B200":
			return "B200"
		case "B300":
			return "B300"
		default:
			return ""
		}
	case "colab":
		switch fam {
		case "CPU":
			return "CPU"
		case "T4":
			return "NVIDIA_TESLA_T4"
		case "L4":
			return "NVIDIA_L4"
		case "A100-40GB":
			return "NVIDIA_TESLA_A100"
		case "A100-80GB":
			return "NVIDIA_TESLA_A100_80GB"
		case "RTX-PRO-6000":
			return "NVIDIA_RTX_PRO_6000"
		case "H100":
			return "NVIDIA_H100_80GB"
		case "TPU_V2_8":
			return "TPU_V2_8"
		case "TPU_V5E":
			return "TPU_V5E_1"
		case "TPU_V6E":
			return "TPU_V6E_1"
		default:
			return ""
		}
	case "kaggle":
		switch fam {
		case "CPU":
			return "CPU"
		case "T4":
			return "NvidiaTeslaT4"
		case "L4":
			return "NvidiaL4"
		case "TPU_V5E":
			return "TpuV5E8"
		case "TPU_V6E":
			return "TpuV6E8"
		default:
			return ""
		}
	case "runpod":
		switch fam {
		case "CPU":
			return "CPU"
		case "T4", "L4", "A10G", "RTX-3090":
			return "RTX-3090"
		case "RTX-4090":
			return "RTX-4090"
		case "L40S":
			return "L40S"
		case "A100-40GB", "A100-80GB":
			return "A100-80GB"
		case "H100":
			return "H100-SXM"
		case "H200":
			return "H200"
		case "B200":
			return "B200"
		default:
			return ""
		}
	default:
		return ""
	}
}

func accountTierAllowsAccelerator(provider, tier, mappedGPU string) bool {
	if mappedGPU == "" {
		return false
	}
	t := strings.TrimSpace(tier)
	if t == "" {
		t = defaultTierForProvider(provider)
	}
	switch provider {
	case "modal":
		if t == "free_no_card" {
			return mappedGPU == "CPU"
		}
		return true
	case "colab":
		switch t {
		case "free":
			return mappedGPU == "CPU" || mappedGPU == "NVIDIA_TESLA_T4" || mappedGPU == "TPU_V5E_1"
		case "google_ai_pro":
			return mappedGPU == "CPU" || mappedGPU == "NVIDIA_TESLA_T4" || mappedGPU == "TPU_V2_8" || mappedGPU == "TPU_V5E_1"
		case "colab_pro":
			return mappedGPU == "CPU" || mappedGPU == "NVIDIA_TESLA_T4" || mappedGPU == "NVIDIA_L4" || mappedGPU == "NVIDIA_TESLA_A100" || mappedGPU == "TPU_V2_8" || mappedGPU == "TPU_V5E_1"
		default:
			return true
		}
	case "kaggle":
		switch t {
		case "unverified":
			return mappedGPU == "CPU"
		case "verified_free":
			return mappedGPU == "CPU" || mappedGPU == "NvidiaTeslaT4" || mappedGPU == "TpuV5E8"
		default:
			return true
		}
	case "runpod":
		return true
	default:
		return false
	}
}

// resolveFineTuningCandidates returns an ordered list of enabled, configured
// execution accounts across Modal, Google Colab, Kaggle, and RunPod whose plan
// tier supports the requested accelerator (or its exact mapped equivalent).
func resolveFineTuningCandidates(gpuID string, preferredProvider string) []fineTuningCandidate {
	opts := service.GetFineTuningGPUOptions()
	pref := strings.ToLower(strings.TrimSpace(preferredProvider))
	trimmed := strings.TrimSpace(gpuID)

	var matched *service.FineTuningGPUOption
	for i := range opts {
		if opts[i].ID == trimmed {
			optCopy := opts[i]
			matched = &optCopy
			break
		}
	}

	providerOrder := []string{}
	if pref != "" {
		providerOrder = append(providerOrder, pref)
	}
	if matched != nil {
		providerOrder = append(providerOrder, matched.Providers...)
	}
	providerOrder = append(providerOrder, "modal", "colab", "kaggle", "runpod")

	seenProvider := map[string]bool{}
	exactMatches := make([]fineTuningCandidate, 0, 8)
	fallbackMatches := make([]fineTuningCandidate, 0, 8)

	for _, p := range providerOrder {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || seenProvider[p] {
			continue
		}
		seenProvider[p] = true
		if p != "modal" && p != "colab" && p != "kaggle" && p != "runpod" {
			continue
		}
		common.OptionMapRWMutex.RLock()
		enabled := common.OptionMap["model_deployment."+p+".enabled"] == "true"
		common.OptionMapRWMutex.RUnlock()
		if !enabled {
			continue
		}

		mappedGPU := mapAcceleratorForProvider(trimmed, p)
		if mappedGPU == "" {
			continue
		}
		isExact := mappedGPU == trimmed
		accounts := GetProviderAccounts(p)
		for _, acc := range accounts {
			if !acc.Enabled || !isAccountConfigured(p, acc) {
				continue
			}
			if !accountTierAllowsAccelerator(p, acc.Tier, mappedGPU) {
				continue
			}
			cand := fineTuningCandidate{
				Provider:    p,
				Account:     acc,
				Accelerator: mappedGPU,
				ExactMatch:  isExact,
			}
			if isExact {
				exactMatches = append(exactMatches, cand)
			} else {
				fallbackMatches = append(fallbackMatches, cand)
			}
		}
	}
	return append(exactMatches, fallbackMatches...)
}

func buildFineTuningServerBaseURL() string {
	base := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if base == "" {
		return ""
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + base
	}
	return base
}

// isUnrecoverableUserTrainingError returns true for errors caused by the user's
// model, dataset, or insufficient VRAM (OOM) where failing over to another node
// would repeat the same error instead of recovering from a node shutdown.
func isUnrecoverableUserTrainingError(err error) bool {
	if err == nil {
		return false
	}
	low := strings.ToLower(err.Error())
	markers := []string{
		"out of memory",
		"cuda error: out of memory",
		"cublas_status_alloc_failed",
		"oom",
		"repo id must be",
		"not a valid model identifier",
		"is not a local folder and is not a valid model identifier",
		"gated repo",
		"repository not found",
		"dataset contains no usable",
		"missing valid openai",
		"invalid openai dpo",
		"no training images",
		"invalid dataset format",
		"jsondecodeerror",
		"unicodedecodeerror",
		"size mismatch",
		"shape mismatch",
		"401 client error",
		"403 client error",
		"404 client error",
	}
	for _, m := range markers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

// isPostTrainingUploadError detects errors that happened AFTER training finished
// (when transferring output.zip), so we never blindly restart training from
// epoch 1 when the problem was a server upload/storage issue.
func isPostTrainingUploadError(err error) bool {
	if err == nil {
		return false
	}
	low := strings.ToLower(err.Error())
	return strings.Contains(low, "artifact upload failed after training completed") ||
		strings.Contains(low, "http error 413") ||
		strings.Contains(low, "payload too large")
}

func dispatchRealFineTuningJob(ctx context.Context, job *model.FineTuningJob, epochs int) (service.FineTuningDispatchResult, error) {
	candidates := resolveFineTuningCandidates(job.GPU, job.Provider)
	if len(candidates) == 0 {
		return service.FineTuningDispatchResult{}, fmt.Errorf("no enabled Model Deployment provider (Modal, Google Colab, Kaggle, or RunPod) with an active account supporting accelerator %q is configured. Enable and connect a provider in System Settings > Model Deployment", job.GPU)
	}

	baseURL := buildFineTuningServerBaseURL()
	callbackURL := ""
	checkpointURL := ""
	uploadChunkURL := ""
	resumeCheckpointURL := ""
	artifactURL := ""
	datasetURL := ""
	baseModelURL := ""
	if baseURL != "" {
		tok := job.CallbackToken
		callbackURL = fmt.Sprintf("%s/api/deployments/fine_tuning/%s/callback", baseURL, job.JobID)
		checkpointURL = fmt.Sprintf("%s/api/deployments/fine_tuning/%s/checkpoint?token=%s", baseURL, job.JobID, tok)
		uploadChunkURL = fmt.Sprintf("%s/api/deployments/fine_tuning/%s/upload_chunk?token=%s", baseURL, job.JobID, tok)
		resumeCheckpointURL = fmt.Sprintf("%s/api/deployments/fine_tuning/%s/checkpoint?token=%s", baseURL, job.JobID, tok)
		artifactURL = fmt.Sprintf("%s/api/deployments/fine_tuning/%s/artifact?token=%s", baseURL, job.JobID, tok)
		datasetURL = fmt.Sprintf("%s/api/deployments/fine_tuning/%s/dataset?token=%s", baseURL, job.JobID, tok)
		if strings.TrimSpace(job.BaseModelFilePath) != "" {
			baseModelURL = fmt.Sprintf("%s/api/deployments/fine_tuning/%s/base_model?token=%s", baseURL, job.JobID, tok)
		}
	}

	hfToken := ""
	if hfAcc, hfErr := ResolveProviderAccount("huggingface", ""); hfErr == nil && hfAcc != nil {
		hfToken = strings.TrimSpace(hfAcc.Token)
	}

	dsName := filepath.Base(job.TrainingFile)
	if dsName == "" || dsName == "." {
		dsName = "dataset.jsonl"
	}

	storageDir := service.FineTuningStorageDirForUser(job.UserId, job.JobID)
	_ = os.MkdirAll(storageDir, 0o755)
	destZip := filepath.Join(storageDir, job.JobID+".zip")
	ckptZip := service.FineTuningCheckpointZipPath(job.UserId, job.JobID)

	// Allow up to 4 attempts when multiple accounts/providers exist, or up to 3
	// clean retries on a fresh runtime of the single available provider when a
	// checkpoint has been saved (only 2 tries if single provider fails before compute).
	maxAttempts := 4
	if len(candidates) == 1 {
		maxAttempts = 3
	}

	var (
		lastErr           error
		anyComputeStarted bool
		totalComputeSec   int64
	)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return service.FineTuningDispatchResult{
				ComputeStarted:        anyComputeStarted,
				ElapsedComputeSeconds: totalComputeSec,
			}, ctx.Err()
		}

		// If the trained archive already arrived on disk (e.g. via chunked upload), finish immediately!
		if info, stErr := os.Stat(destZip); stErr == nil && info.Size() > 0 {
			fresh, _ := model.GetFineTuningJobByJobID(job.JobID)
			var tok int64
			if fresh != nil {
				tok = fresh.TrainedTokens
			}
			return service.FineTuningDispatchResult{
				ArtifactZipPath:       destZip,
				TrainedTokens:         tok,
				ComputeStarted:        true,
				ElapsedComputeSeconds: maxInt64(1, totalComputeSec),
			}, nil
		}

		cand := candidates[attempt%len(candidates)]
		hasCheckpoint := false
		if fi, sErr := os.Stat(ckptZip); sErr == nil && fi.Size() > 0 {
			hasCheckpoint = true
		}

		freshJob, _ := model.GetFineTuningJobByJobID(job.JobID)
		if freshJob != nil && freshJob.Status == "cancelled" {
			return service.FineTuningDispatchResult{
				ComputeStarted:        anyComputeStarted,
				ElapsedComputeSeconds: totalComputeSec,
			}, context.Canceled
		}

		_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
			"provider":      cand.Provider,
			"status":        "running",
			"attempt_count": attempt + 1,
			"updated_at":    time.Now().Unix(),
		})

		if attempt == 0 {
			service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "alloc", fmt.Sprintf("Dispatching real PyTorch training workload (accelerator=%s, ram_tier=%s, epochs=%d)", cand.Accelerator, job.RamTier, epochs))
		} else if hasCheckpoint && freshJob != nil {
			targetDesc := "backup compute node"
			if len(candidates) == 1 {
				targetDesc = "fresh compute instance"
			}
			service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "failover", fmt.Sprintf("Compute node interrupted — seamlessly resuming from saved checkpoint (epoch %d, step %d) on %s (attempt %d/%d)...", freshJob.CheckpointEpoch, freshJob.CheckpointStep, targetDesc, attempt+1, maxAttempts))
		} else {
			service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "failover", fmt.Sprintf("Compute node unavailable — retrying on next available compute instance (attempt %d/%d)...", attempt+1, maxAttempts))
		}

		activeResumeURL := ""
		activeResumeLocal := ""
		if hasCheckpoint {
			activeResumeURL = resumeCheckpointURL
			activeResumeLocal = ckptZip
		}

		script := finetunescript.Generate(finetunescript.Config{
			JobID:               job.JobID,
			FineTunedModel:      job.FineTunedModel,
			BaseModel:           job.Model,
			BaseModelURL:        baseModelURL,
			ModelSource:         job.ModelSource,
			DatasetURL:          datasetURL,
			DatasetName:         dsName,
			TaskType:            job.TaskType,
			Method:              job.Method,
			Quantization:        job.Quantization,
			Epochs:              epochs,
			Accelerator:         cand.Accelerator,
			CallbackURL:         callbackURL,
			CheckpointURL:       checkpointURL,
			UploadChunkURL:      uploadChunkURL,
			ResumeCheckpointURL: activeResumeURL,
			ArtifactURL:         artifactURL,
			CallbackToken:       job.CallbackToken,
			HFToken:             hfToken,
			OutputZipPath:       "output.zip",
		})

		var (
			zipPath        string
			trainedTokens  int64
			computeStarted bool
			computeSec     int64
			runErr         error
		)
		accCopy := cand.Account
		switch cand.Provider {
		case "modal":
			zipPath, trainedTokens, computeStarted, computeSec, runErr = runFineTuningOnModal(ctx, job, cand.Accelerator, &accCopy, script, dsName, activeResumeLocal, destZip)
		case "colab":
			zipPath, trainedTokens, computeStarted, computeSec, runErr = runFineTuningOnColab(ctx, job, cand.Accelerator, &accCopy, script, dsName, activeResumeLocal, ckptZip, destZip, baseURL != "")
		case "kaggle":
			zipPath, trainedTokens, computeStarted, computeSec, runErr = runFineTuningOnKaggle(ctx, job, cand.Accelerator, &accCopy, script, destZip)
		case "runpod":
			zipPath, trainedTokens, computeStarted, computeSec, runErr = runFineTuningOnRunPod(ctx, job, cand.Accelerator, &accCopy, script, destZip)
		default:
			runErr = fmt.Errorf("unsupported fine-tuning execution provider: %s", cand.Provider)
		}

		if computeStarted {
			anyComputeStarted = true
			totalComputeSec += computeSec
		}

		if runErr == nil {
			return service.FineTuningDispatchResult{
				ArtifactZipPath:       zipPath,
				TrainedTokens:         trainedTokens,
				ComputeStarted:        true,
				ElapsedComputeSeconds: maxInt64(1, totalComputeSec),
			}, nil
		}

		lastErr = runErr
		if ctx.Err() != nil {
			return service.FineTuningDispatchResult{
				ComputeStarted:        anyComputeStarted,
				ElapsedComputeSeconds: totalComputeSec,
			}, ctx.Err()
		}

		// Never discard completed training or retry from scratch on post-training upload/413 errors
		if isPostTrainingUploadError(runErr) {
			return service.FineTuningDispatchResult{
				ComputeStarted:        anyComputeStarted,
				ElapsedComputeSeconds: totalComputeSec,
				BilledOnFailure:       false,
			}, runErr
		}

		// User/model/dataset/OOM errors cannot be fixed by switching servers
		if isUnrecoverableUserTrainingError(runErr) {
			return service.FineTuningDispatchResult{
				ComputeStarted:        anyComputeStarted,
				ElapsedComputeSeconds: totalComputeSec,
				BilledOnFailure:       anyComputeStarted,
			}, runErr
		}

		// If only 1 provider/account supports this GPU and it failed before any checkpoint
		// was saved, try at most once more (2 total tries) and then stop cleanly without looping.
		hasAnyCkptNow := false
		if fi, sErr := os.Stat(ckptZip); sErr == nil && fi.Size() > 0 {
			hasAnyCkptNow = true
		}
		if len(candidates) == 1 && !hasAnyCkptNow && attempt >= 1 {
			break
		}

		backoff := time.Duration(3*(attempt+1)) * time.Second
		select {
		case <-ctx.Done():
			return service.FineTuningDispatchResult{
				ComputeStarted:        anyComputeStarted,
				ElapsedComputeSeconds: totalComputeSec,
			}, ctx.Err()
		case <-time.After(backoff):
		}
	}

	return service.FineTuningDispatchResult{
		ComputeStarted:        anyComputeStarted,
		ElapsedComputeSeconds: totalComputeSec,
		BilledOnFailure:       anyComputeStarted && isUnrecoverableUserTrainingError(lastErr),
	}, lastErr
}

func runFineTuningOnModal(ctx context.Context, job *model.FineTuningJob, accelerator string, acc *DeploymentAccount, script, dsName, resumeCkptPath, destZip string) (string, int64, bool, int64, error) {
	common.OptionMapRWMutex.RLock()
	volPath := strings.TrimSpace(common.OptionMap["model_deployment.modal.shared_volume_path"])
	common.OptionMapRWMutex.RUnlock()

	computeStart := time.Now()
	computeStarted := false
	err := modaldeploy.RunFineTuneWithLogger(ctx, modaldeploy.FineTuneConfig{
		AppName:              "ft-" + strings.ToLower(job.JobID),
		Accelerator:          accelerator,
		TokenID:              strings.TrimSpace(acc.TokenID),
		TokenSecret:          strings.TrimSpace(acc.TokenSecret),
		SharedVolumePath:     volPath,
		TrainScript:          script,
		DatasetPath:          job.TrainingFilePath,
		DatasetName:          dsName,
		BaseModelPath:        job.BaseModelFilePath,
		ResumeCheckpointPath: resumeCkptPath,
		OutputZipPath:        destZip,
	}, func(line string) {
		if !computeStarted && (strings.Contains(line, "[boot]") || strings.Contains(line, "[env]") || strings.Contains(line, "[model]") || strings.Contains(line, "[train]")) {
			computeStarted = true
			computeStart = time.Now()
		}
		service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "worker", line)
	})
	elapsedSec := int64(time.Since(computeStart).Seconds())
	if !computeStarted {
		elapsedSec = 0
	}
	if err != nil {
		return "", 0, computeStarted, elapsedSec, err
	}
	if info, statErr := os.Stat(destZip); statErr != nil || info.Size() == 0 {
		return "", 0, computeStarted, elapsedSec, fmt.Errorf("training completed without producing a valid .zip model archive")
	}
	fresh, _ := model.GetFineTuningJobByJobID(job.JobID)
	var trained int64
	if fresh != nil {
		trained = fresh.TrainedTokens
	}
	return destZip, trained, true, maxInt64(1, elapsedSec), nil
}

func runFineTuningOnColab(ctx context.Context, job *model.FineTuningJob, accelerator string, acc *DeploymentAccount, script, dsName, resumeCkptPath, localCkptDst, destZip string, hasRemoteChunkUpload bool) (string, int64, bool, int64, error) {
	tok, err := getValidColabAccessTokenForAccount(ctx, acc.ID)
	if err != nil {
		return "", 0, false, 0, err
	}
	client := newColabClient(tok)
	highRAM := strings.EqualFold(strings.TrimSpace(job.RamTier), "high")
	rt, err := client.CreateRuntimeWithOptions(accelerator, highRAM)
	if err != nil {
		return "", 0, false, 0, fmt.Errorf("failed to allocate compute runtime: %w", err)
	}
	proxyURL := strings.TrimSpace(rt.ConnectionInfo.URL)
	if proxyURL == "" {
		proxyURL = strings.TrimSpace(rt.ProxyURL)
	}
	defer func() {
		if rt.Name != "" {
			_ = client.DeleteRuntime(rt.Name)
		}
	}()
	if proxyURL == "" {
		return "", 0, false, 0, fmt.Errorf("compute runtime did not return a connection endpoint")
	}

	// Upload dataset directly to runtime when small (<= 40 MB) so training works even on localhost
	if job.TrainingFilePath != "" {
		if fi, sErr := os.Stat(job.TrainingFilePath); sErr == nil && fi.Size() > 0 && fi.Size() <= 40<<20 {
			if dsBytes, rErr := os.ReadFile(job.TrainingFilePath); rErr == nil && len(dsBytes) > 0 {
				dsExt := filepath.Ext(dsName)
				if dsExt == "" {
					dsExt = ".jsonl"
				}
				if upErr := client.UploadBinaryFile(proxyURL, rt.ConnectionInfo.Token, "dataset"+dsExt, dsBytes); upErr != nil {
					service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "warn", "Direct dataset upload fallback to HTTP download")
				}
			}
		}
	}

	// Only push small (<= 40 MB) resume checkpoints directly via Jupyter JSON/base64;
	// larger checkpoints are downloaded directly by train.py via RESUME_CHECKPOINT_URL!
	if resumeCkptPath != "" {
		if fi, sErr := os.Stat(resumeCkptPath); sErr == nil && fi.Size() > 0 && fi.Size() <= 40<<20 {
			if ckBytes, rErr := os.ReadFile(resumeCkptPath); rErr == nil && len(ckBytes) > 0 {
				if upErr := client.UploadBinaryFile(proxyURL, rt.ConnectionInfo.Token, "resume_checkpoint.zip", ckBytes); upErr == nil {
					service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "resume", fmt.Sprintf("Uploaded saved checkpoint (%.2f MB) to compute runtime", float64(len(ckBytes))/(1024*1024)))
				}
			}
		}
	}

	if err := client.UploadFile(proxyURL, rt.ConnectionInfo.Token, "train.py", script); err != nil {
		return "", 0, false, 0, fmt.Errorf("failed to upload training script: %w", err)
	}
	kernelID, err := client.StartKernel(proxyURL, rt.ConnectionInfo.Token)
	if err != nil {
		return "", 0, false, 0, fmt.Errorf("failed to start training kernel: %w", err)
	}

	launchSnippet := `import os, subprocess, sys
script_path = next((p for p in ['train.py', '/content/train.py', '/train.py'] if os.path.exists(p)), None)
if not script_path:
    open('ft_error.txt', 'w').write('train.py not found')
else:
    script_dir = os.path.dirname(os.path.abspath(script_path)) or '.'
    log_path = os.path.join(script_dir, 'train_stdout.log')
    runner = (
        f"import os, subprocess, sys\n"
        f"with open({log_path!r}, 'w') as lf:\n"
        f"    rc = subprocess.call([sys.executable, '-u', {os.path.abspath(script_path)!r}], cwd={script_dir!r}, stdout=lf, stderr=subprocess.STDOUT)\n"
        f"if rc != 0 and not os.path.exists({os.path.join(script_dir, 'ft_error.txt')!r}):\n"
        f"    tail = open({log_path!r}).read()[-600:] if os.path.exists({log_path!r}) else ''\n"
        f"    open({os.path.join(script_dir, 'ft_error.txt')!r}, 'w').write(f'exit {{rc}}: {{tail}}')\n"
    )
    subprocess.Popen([sys.executable, '-c', runner], cwd=script_dir, start_new_session=True, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
`
	if err := client.ExecuteCode(proxyURL, rt.ConnectionInfo.Token, kernelID, launchSnippet); err != nil {
		return "", 0, false, 0, fmt.Errorf("failed to start training process: %w", err)
	}
	computeStart := time.Now()

	readRT := func(name string) (string, error) {
		if s, e := client.ReadFile(proxyURL, rt.ConnectionInfo.Token, name); e == nil {
			return s, nil
		}
		return client.ReadFile(proxyURL, rt.ConnectionInfo.Token, "content/"+strings.TrimLeft(name, "/"))
	}

	seenLines := 0
	lastPulledCkptStep := -1
	consecutiveReadFailures := 0
	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", 0, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), ctx.Err()
		case <-ticker.C:
			fresh, _ := model.GetFineTuningJobByJobID(job.JobID)
			if fresh != nil {
				if fresh.Status == "cancelled" {
					return "", 0, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), context.Canceled
				}
				// If chunked HTTP upload + succeeded callback already finished cleanly, return immediately!
				if fresh.Status == "succeeded" {
					if info, stErr := os.Stat(destZip); stErr == nil && info.Size() > 0 {
						return destZip, fresh.TrainedTokens, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), nil
					}
				}
				if fresh.Status == "failed" && strings.TrimSpace(fresh.Error) != "" {
					return "", 0, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), fmt.Errorf("%s", fresh.Error)
				}
			}

			stdoutText, sErr := readRT("train_stdout.log")
			if sErr == nil {
				consecutiveReadFailures = 0
				if stdoutText != "" {
					lines := strings.Split(strings.TrimRight(stdoutText, "\n"), "\n")
					for i := seenLines; i < len(lines); i++ {
						ln := strings.TrimSpace(lines[i])
						if ln != "" {
							service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "worker", ln)
						}
					}
					seenLines = len(lines)
				}
			} else {
				// If HTTP callbacks updated the job within the last 60s, the training
				// process is still alive (Jupyter proxy may just be momentarily busy
				// during heavy disk I/O / packaging) — NEVER falsely kill or failover!
				if fresh != nil && time.Now().Unix()-fresh.UpdatedAt < 60 {
					consecutiveReadFailures = 0
				} else {
					consecutiveReadFailures++
					if consecutiveReadFailures >= 12 {
						return "", 0, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), fmt.Errorf("compute runtime disconnected unexpectedly (%v)", sErr)
					}
				}
			}

			// Localhost fallback ONLY: pull small checkpoints (<= 40 MB) via Jupyter proxy
			// when remote chunked HTTP upload did not already sync them. Never call
			// Jupyter /api/contents base64 JSON on 600 MB+ files (which crashes Jupyter with HTTP 500!).
			if !hasRemoteChunkUpload {
				if metaTxt, mErr := readRT("checkpoint_meta.json"); mErr == nil && strings.TrimSpace(metaTxt) != "" {
					var ckMeta struct {
						CompletedEpoch int   `json:"completed_epoch"`
						GlobalStep     int   `json:"global_step"`
						TrainedTokens  int64 `json:"trained_tokens"`
						Progress       int   `json:"progress"`
						SizeBytes      int64 `json:"size_bytes"`
						Uploaded       bool  `json:"uploaded"`
					}
					if common.UnmarshalJsonStr(metaTxt, &ckMeta) == nil &&
						!ckMeta.Uploaded &&
						ckMeta.GlobalStep > lastPulledCkptStep &&
						ckMeta.SizeBytes > 0 && ckMeta.SizeBytes <= 40<<20 {
						ckBytes, dlErr := client.DownloadBinaryFile(proxyURL, rt.ConnectionInfo.Token, "checkpoint_latest.zip")
						if dlErr != nil {
							ckBytes, dlErr = client.DownloadBinaryFile(proxyURL, rt.ConnectionInfo.Token, "content/checkpoint_latest.zip")
						}
						if dlErr == nil && len(ckBytes) > 0 {
							tmpCk := localCkptDst + ".tmp"
							if wErr := os.WriteFile(tmpCk, ckBytes, 0o644); wErr == nil && os.Rename(tmpCk, localCkptDst) == nil {
								lastPulledCkptStep = ckMeta.GlobalStep
								_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
									"checkpoint_path":  localCkptDst,
									"checkpoint_epoch": ckMeta.CompletedEpoch,
									"checkpoint_step":  ckMeta.GlobalStep,
									"trained_tokens":   ckMeta.TrainedTokens,
									"progress":         ckMeta.Progress,
									"updated_at":       time.Now().Unix(),
								})
							}
						}
					}
				}
			}

			if errTxt, eErr := readRT("ft_error.txt"); eErr == nil && strings.TrimSpace(errTxt) != "" {
				return "", 0, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), fmt.Errorf("%s", strings.TrimSpace(errTxt))
			}

			if doneTxt, dErr := readRT("ft_done.json"); dErr == nil && strings.TrimSpace(doneTxt) != "" {
				var doneMeta struct {
					Status         string `json:"status"`
					Uploaded       bool   `json:"uploaded"`
					OutputZipBytes int64  `json:"output_zip_bytes"`
					UploadError    string `json:"upload_error"`
					TrainedTokens  int64  `json:"trained_tokens"`
				}
				_ = common.UnmarshalJsonStr(doneTxt, &doneMeta)
				if info, stErr := os.Stat(destZip); stErr == nil && info.Size() > 0 {
					return destZip, doneMeta.TrainedTokens, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), nil
				}
				if doneMeta.OutputZipBytes > 0 && doneMeta.OutputZipBytes <= 40<<20 {
					zipBytes, dlErr := client.DownloadBinaryFile(proxyURL, rt.ConnectionInfo.Token, "output.zip")
					if dlErr != nil {
						zipBytes, dlErr = client.DownloadBinaryFile(proxyURL, rt.ConnectionInfo.Token, "content/output.zip")
					}
					if dlErr == nil && len(zipBytes) > 0 {
						if wErr := os.WriteFile(destZip, zipBytes, 0o644); wErr == nil {
							return destZip, doneMeta.TrainedTokens, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), nil
						}
					}
				}
				if doneMeta.UploadError != "" {
					return "", 0, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), fmt.Errorf("artifact upload failed after training completed: %s", doneMeta.UploadError)
				}
				return "", 0, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), fmt.Errorf("artifact upload failed after training completed: ensure ServerAddress is reachable for chunked upload")
			}
		}
	}
}

func runFineTuningOnKaggle(ctx context.Context, job *model.FineTuningJob, accelerator string, acc *DeploymentAccount, script, destZip string) (string, int64, bool, int64, error) {
	if buildFineTuningServerBaseURL() == "" {
		return "", 0, false, 0, fmt.Errorf("ServerAddress must be configured (public URL) so the remote worker can download the dataset and upload the trained .zip archive")
	}
	kUser := strings.TrimSpace(acc.Username)
	kKey := strings.TrimSpace(acc.APIKey)
	client := newKaggleClient(kUser, kKey)
	kernelSlug := "ft-" + strings.ToLower(strings.ReplaceAll(job.JobID, "_", "-"))
	slug := kUser + "/" + kernelSlug
	pushed, err := client.PushKernel(kaggle.PushKernelRequest{
		Slug:        slug,
		Title:       strings.ReplaceAll(kernelSlug, "-", " "),
		ScriptText:  script,
		Accelerator: accelerator,
		IsPrivate:   true,
	})
	if err != nil {
		return "", 0, false, 0, fmt.Errorf("failed to submit training kernel: %w", err)
	}
	service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "worker", fmt.Sprintf("Training kernel queued (v%d, accelerator=%s)", pushed.VersionNumber, accelerator))

	computeStarted := false
	computeStart := time.Now()
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			elapsed := int64(0)
			if computeStarted {
				elapsed = maxInt64(1, int64(time.Since(computeStart).Seconds()))
			}
			return "", 0, computeStarted, elapsed, ctx.Err()
		case <-ticker.C:
			if fresh, _ := model.GetFineTuningJobByJobID(job.JobID); fresh != nil {
				if fresh.Progress > 3 && !computeStarted {
					computeStarted = true
					computeStart = time.Now()
				}
				if fresh.Status == "succeeded" {
					if info, stErr := os.Stat(destZip); stErr == nil && info.Size() > 0 {
						return destZip, fresh.TrainedTokens, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), nil
					}
				}
				if fresh.Status == "failed" {
					elapsed := int64(0)
					if computeStarted {
						elapsed = maxInt64(1, int64(time.Since(computeStart).Seconds()))
					}
					return "", 0, computeStarted, elapsed, fmt.Errorf("%s", fresh.Error)
				}
				if fresh.Status == "cancelled" {
					elapsed := int64(0)
					if computeStarted {
						elapsed = maxInt64(1, int64(time.Since(computeStart).Seconds()))
					}
					return "", 0, computeStarted, elapsed, context.Canceled
				}
			}
			ks, stErr := client.GetKernelStatus(kUser, kernelSlug)
			if stErr == nil && ks != nil {
				st := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ks.Status)), "kernelworkerstatus.")
				switch st {
				case "running":
					if !computeStarted {
						computeStarted = true
						computeStart = time.Now()
					}
				case "error":
					msg := strings.TrimSpace(ks.FailureMessage)
					if msg == "" {
						msg = "remote training kernel failed"
					}
					elapsed := int64(0)
					if computeStarted {
						elapsed = maxInt64(1, int64(time.Since(computeStart).Seconds()))
					}
					return "", 0, computeStarted, elapsed, fmt.Errorf("%s", msg)
				case "cancel_requested", "cancel_acknowledged":
					return "", 0, computeStarted, 0, context.Canceled
				case "complete":
					if info, sErr := os.Stat(destZip); sErr == nil && info.Size() > 0 {
						fresh, _ := model.GetFineTuningJobByJobID(job.JobID)
						var tok int64
						if fresh != nil {
							tok = fresh.TrainedTokens
						}
						return destZip, tok, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), nil
					}
					return "", 0, computeStarted, maxInt64(1, int64(time.Since(computeStart).Seconds())), fmt.Errorf("artifact upload failed after training completed: ensure ServerAddress is publicly reachable")
				}
			}
		}
	}
}

func mapRunPodGPUTypeID(gpuID string) string {
	switch strings.TrimSpace(gpuID) {
	case "B200":
		return "NVIDIA B200"
	case "H200":
		return "NVIDIA H200"
	case "H100-SXM", "H100", "NVIDIA_H100_80GB":
		return "NVIDIA H100 80GB HBM3"
	case "A100-80GB", "NVIDIA_TESLA_A100_80GB", "A100":
		return "NVIDIA A100 80GB PCIe"
	case "A100-40GB", "NVIDIA_TESLA_A100":
		return "NVIDIA A100-SXM4-40GB"
	case "L40S":
		return "NVIDIA L40S"
	case "L4", "NVIDIA_L4":
		return "NVIDIA L4"
	case "RTX-4090":
		return "NVIDIA GeForce RTX 4090"
	case "RTX-3090":
		return "NVIDIA GeForce RTX 3090"
	default:
		return "NVIDIA GeForce RTX 4090"
	}
}

func runPodGraphQL(ctx context.Context, apiKey string, query string, variables map[string]any, out any) error {
	payload := map[string]any{
		"query":     query,
		"variables": variables,
	}
	bodyBytes, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.runpod.io/graphql?api_key="+apiKey, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("runpod api HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		if err := common.Unmarshal(raw, out); err != nil {
			return err
		}
	}
	return nil
}

func runFineTuningOnRunPod(ctx context.Context, job *model.FineTuningJob, accelerator string, acc *DeploymentAccount, script, destZip string) (string, int64, bool, int64, error) {
	if buildFineTuningServerBaseURL() == "" {
		return "", 0, false, 0, fmt.Errorf("ServerAddress must be configured (public URL) so the remote worker can download the dataset and upload checkpoints/artifacts")
	}
	apiKey := strings.TrimSpace(acc.APIKey)
	if apiKey == "" {
		return "", 0, false, 0, fmt.Errorf("RunPod API key is missing")
	}

	b64Script := base64.StdEncoding.EncodeToString([]byte(script))
	gpuTypeID := mapRunPodGPUTypeID(accelerator)

	createMutation := `mutation($input: PodFindAndDeployOnDemandInput!) {
		podFindAndDeployOnDemand(input: $input) {
			id
			desiredStatus
			machineId
		}
	}`
	var createResp struct {
		Data struct {
			PodFindAndDeployOnDemand struct {
				ID            string `json:"id"`
				DesiredStatus string `json:"desiredStatus"`
			} `json:"podFindAndDeployOnDemand"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	podInput := map[string]any{
		"cloudType":         "ALL",
		"gpuCount":          1,
		"volumeInGb":        60,
		"containerDiskInGb": 40,
		"minVcpuCount":      4,
		"minMemoryInGb":     24,
		"gpuTypeId":         gpuTypeID,
		"name":              "ft-" + strings.ToLower(job.JobID),
		"imageName":         "pytorch/pytorch:2.4.0-cuda12.4-cudnn9-runtime",
		"dockerArgs":        `bash -lc "printf '%s' \"$FT_SCRIPT_B64\" | base64 -d > /workspace/train.py && cd /workspace && python3 -u /workspace/train.py"`,
		"env": []map[string]string{
			{"key": "FT_SCRIPT_B64", "value": b64Script},
		},
	}
	if err := runPodGraphQL(ctx, apiKey, createMutation, map[string]any{"input": podInput}, &createResp); err != nil {
		return "", 0, false, 0, fmt.Errorf("failed to allocate GPU pod: %w", err)
	}
	if len(createResp.Errors) > 0 {
		return "", 0, false, 0, fmt.Errorf("failed to allocate GPU pod: %s", createResp.Errors[0].Message)
	}
	podID := strings.TrimSpace(createResp.Data.PodFindAndDeployOnDemand.ID)
	if podID == "" {
		return "", 0, false, 0, fmt.Errorf("GPU pod provisioning returned empty pod ID")
	}

	defer func() {
		termCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		termMutation := `mutation($input: PodTerminateInput!) { podTerminate(input: $input) }`
		_ = runPodGraphQL(termCtx, apiKey, termMutation, map[string]any{"input": map[string]any{"podId": podID}}, nil)
	}()

	service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "worker", fmt.Sprintf("GPU training pod allocated (%s), booting container...", gpuTypeID))

	computeStarted := false
	computeStart := time.Now()
	statusQuery := `query($input: PodFilter!) { pod(input: $input) { id desiredStatus runtime { uptimeInSeconds } } }`
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			elapsed := int64(0)
			if computeStarted {
				elapsed = maxInt64(1, int64(time.Since(computeStart).Seconds()))
			}
			return "", 0, computeStarted, elapsed, ctx.Err()
		case <-ticker.C:
			if fresh, _ := model.GetFineTuningJobByJobID(job.JobID); fresh != nil {
				if fresh.Progress > 3 && !computeStarted {
					computeStarted = true
					computeStart = time.Now()
				}
				if fresh.Status == "succeeded" {
					if info, stErr := os.Stat(destZip); stErr == nil && info.Size() > 0 {
						return destZip, fresh.TrainedTokens, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), nil
					}
				}
				if fresh.Status == "failed" {
					elapsed := int64(0)
					if computeStarted {
						elapsed = maxInt64(1, int64(time.Since(computeStart).Seconds()))
					}
					return "", 0, computeStarted, elapsed, fmt.Errorf("%s", fresh.Error)
				}
				if fresh.Status == "cancelled" {
					elapsed := int64(0)
					if computeStarted {
						elapsed = maxInt64(1, int64(time.Since(computeStart).Seconds()))
					}
					return "", 0, computeStarted, elapsed, context.Canceled
				}
			}

			var podResp struct {
				Data struct {
					Pod *struct {
						ID            string `json:"id"`
						DesiredStatus string `json:"desiredStatus"`
						Runtime       *struct {
							UptimeInSeconds int64 `json:"uptimeInSeconds"`
						} `json:"runtime"`
					} `json:"pod"`
				} `json:"data"`
			}
			if qErr := runPodGraphQL(ctx, apiKey, statusQuery, map[string]any{"input": map[string]any{"podId": podID}}, &podResp); qErr == nil {
				p := podResp.Data.Pod
				if p == nil {
					elapsed := int64(0)
					if computeStarted {
						elapsed = maxInt64(1, int64(time.Since(computeStart).Seconds()))
					}
					return "", 0, computeStarted, elapsed, fmt.Errorf("GPU pod terminated unexpectedly before completion")
				}
				if p.Runtime != nil && p.Runtime.UptimeInSeconds > 0 && !computeStarted {
					computeStarted = true
					computeStart = time.Now()
				}
				st := strings.ToUpper(strings.TrimSpace(p.DesiredStatus))
				if st == "EXITED" || st == "TERMINATED" {
					if info, stErr := os.Stat(destZip); stErr == nil && info.Size() > 0 {
						fresh, _ := model.GetFineTuningJobByJobID(job.JobID)
						var tok int64
						if fresh != nil {
							tok = fresh.TrainedTokens
						}
						return destZip, tok, true, maxInt64(1, int64(time.Since(computeStart).Seconds())), nil
					}
					elapsed := int64(0)
					if computeStarted {
						elapsed = maxInt64(1, int64(time.Since(computeStart).Seconds()))
					}
					return "", 0, computeStarted, elapsed, fmt.Errorf("GPU pod exited unexpectedly before uploading trained model archive")
				}
			}
		}
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
