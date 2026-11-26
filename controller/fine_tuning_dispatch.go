package controller

import (
	"context"
	"fmt"
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
// (Modal, Google Colab, Kaggle) into service/fine_tuning_runner.go so fine-tuning
// jobs execute real PyTorch + PEFT / Diffusers training on the admin's enabled
// Model Deployment accounts.
func InitFineTuningDispatcher() {
	service.RegisterFineTuningDispatcher(dispatchRealFineTuningJob)
}

func resolveProviderForFineTuningGPU(gpuID string, preferredProvider string) (string, *service.FineTuningGPUOption) {
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

	candidates := []string{}
	if pref != "" {
		candidates = append(candidates, pref)
	}
	if matched != nil {
		candidates = append(candidates, matched.Providers...)
	}
	// Default preference order among providers with direct script execution
	candidates = append(candidates, "modal", "colab", "kaggle")

	seen := map[string]bool{}
	for _, p := range candidates {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if p != "modal" && p != "colab" && p != "kaggle" {
			continue
		}
		common.OptionMapRWMutex.RLock()
		enabled := common.OptionMap["model_deployment."+p+".enabled"] == "true"
		common.OptionMapRWMutex.RUnlock()
		if !enabled {
			continue
		}
		if acc, err := ResolveProviderAccount(p, ""); err == nil && acc != nil {
			return p, matched
		}
	}
	return "", matched
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

func dispatchRealFineTuningJob(ctx context.Context, job *model.FineTuningJob, epochs int) (string, int64, error) {
	provider, _ := resolveProviderForFineTuningGPU(job.GPU, job.Provider)
	if provider == "" {
		return "", 0, fmt.Errorf("no enabled Model Deployment provider (Modal, Google Colab, or Kaggle) with active credentials is configured for accelerator %q. Enable and connect a provider in System Settings > Model Deployment", job.GPU)
	}

	acc, err := ResolveProviderAccount(provider, "")
	if err != nil || acc == nil {
		return "", 0, fmt.Errorf("no active %s account found in System Settings > Model Deployment", provider)
	}

	_ = model.UpdateFineTuningJobStatus(job.JobID, map[string]any{
		"provider":   provider,
		"status":     "running",
		"progress":   3,
		"updated_at": time.Now().Unix(),
	})
	service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "alloc", fmt.Sprintf("Dispatching real PyTorch training workload (accelerator=%s, ram_tier=%s, epochs=%d)", job.GPU, job.RamTier, epochs))

	baseURL := buildFineTuningServerBaseURL()
	callbackURL := ""
	artifactURL := ""
	datasetURL := ""
	baseModelURL := ""
	if baseURL != "" {
		tok := job.CallbackToken
		callbackURL = fmt.Sprintf("%s/api/deployments/fine_tuning/%s/callback", baseURL, job.JobID)
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

	script := finetunescript.Generate(finetunescript.Config{
		JobID:          job.JobID,
		FineTunedModel: job.FineTunedModel,
		BaseModel:      job.Model,
		BaseModelURL:   baseModelURL,
		ModelSource:    job.ModelSource,
		DatasetURL:     datasetURL,
		DatasetName:    dsName,
		TaskType:       job.TaskType,
		Method:         job.Method,
		Epochs:         epochs,
		Accelerator:    job.GPU,
		CallbackURL:    callbackURL,
		ArtifactURL:    artifactURL,
		CallbackToken:  job.CallbackToken,
		HFToken:        hfToken,
		OutputZipPath:  "output.zip",
	})

	storageDir := service.FineTuningStorageDirForUser(job.UserId, job.JobID)
	_ = os.MkdirAll(storageDir, 0o755)
	destZip := filepath.Join(storageDir, job.JobID+".zip")

	switch provider {
	case "modal":
		return runFineTuningOnModal(ctx, job, acc, script, dsName, destZip)
	case "colab":
		return runFineTuningOnColab(ctx, job, acc, script, dsName, destZip)
	case "kaggle":
		return runFineTuningOnKaggle(ctx, job, acc, script, destZip)
	default:
		return "", 0, fmt.Errorf("unsupported fine-tuning execution provider: %s", provider)
	}
}

func runFineTuningOnModal(ctx context.Context, job *model.FineTuningJob, acc *DeploymentAccount, script, dsName, destZip string) (string, int64, error) {
	common.OptionMapRWMutex.RLock()
	volPath := strings.TrimSpace(common.OptionMap["model_deployment.modal.shared_volume_path"])
	common.OptionMapRWMutex.RUnlock()

	err := modaldeploy.RunFineTuneWithLogger(ctx, modaldeploy.FineTuneConfig{
		AppName:          "ft-" + strings.ToLower(job.JobID),
		Accelerator:      job.GPU,
		TokenID:          strings.TrimSpace(acc.TokenID),
		TokenSecret:      strings.TrimSpace(acc.TokenSecret),
		SharedVolumePath: volPath,
		TrainScript:      script,
		DatasetPath:      job.TrainingFilePath,
		DatasetName:      dsName,
		BaseModelPath:    job.BaseModelFilePath,
		OutputZipPath:    destZip,
	}, func(line string) {
		service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "worker", line)
	})
	if err != nil {
		return "", 0, err
	}
	if info, statErr := os.Stat(destZip); statErr != nil || info.Size() == 0 {
		return "", 0, fmt.Errorf("training completed without producing a valid .zip model archive")
	}
	fresh, _ := model.GetFineTuningJobByJobID(job.JobID)
	var trained int64
	if fresh != nil {
		trained = fresh.TrainedTokens
	}
	return destZip, trained, nil
}

func runFineTuningOnColab(ctx context.Context, job *model.FineTuningJob, acc *DeploymentAccount, script, dsName, destZip string) (string, int64, error) {
	tok, err := getValidColabAccessTokenForAccount(ctx, acc.ID)
	if err != nil {
		return "", 0, err
	}
	client := newColabClient(tok)
	highRAM := strings.EqualFold(strings.TrimSpace(job.RamTier), "high")
	rt, err := client.CreateRuntimeWithOptions(job.GPU, highRAM)
	if err != nil {
		return "", 0, fmt.Errorf("failed to allocate compute runtime: %w", err)
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
		return "", 0, fmt.Errorf("compute runtime did not return a connection endpoint")
	}

	// Upload dataset directly to runtime so training works even on localhost/private servers
	if job.TrainingFilePath != "" {
		if dsBytes, rErr := os.ReadFile(job.TrainingFilePath); rErr == nil && len(dsBytes) > 0 {
			dsExt := filepath.Ext(dsName)
			if dsExt == "" {
				dsExt = ".jsonl"
			}
			if upErr := client.UploadBinaryFile(proxyURL, rt.ConnectionInfo.Token, "dataset"+dsExt, dsBytes); upErr != nil {
				service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "warn", "Direct dataset upload fallback to HTTP callback")
			}
		}
	}

	if err := client.UploadFile(proxyURL, rt.ConnectionInfo.Token, "train.py", script); err != nil {
		return "", 0, fmt.Errorf("failed to upload training script: %w", err)
	}
	kernelID, err := client.StartKernel(proxyURL, rt.ConnectionInfo.Token)
	if err != nil {
		return "", 0, fmt.Errorf("failed to start training kernel: %w", err)
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
		return "", 0, fmt.Errorf("failed to start training process: %w", err)
	}

	readRT := func(name string) (string, error) {
		if s, e := client.ReadFile(proxyURL, rt.ConnectionInfo.Token, name); e == nil {
			return s, nil
		}
		return client.ReadFile(proxyURL, rt.ConnectionInfo.Token, "content/"+strings.TrimLeft(name, "/"))
	}

	seenLines := 0
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		case <-ticker.C:
			if stdoutText, sErr := readRT("train_stdout.log"); sErr == nil && stdoutText != "" {
				lines := strings.Split(strings.TrimRight(stdoutText, "\n"), "\n")
				for i := seenLines; i < len(lines); i++ {
					ln := strings.TrimSpace(lines[i])
					if ln != "" {
						service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "worker", ln)
					}
				}
				seenLines = len(lines)
			}

			// Check if callback already delivered the zip archive
			if info, stErr := os.Stat(destZip); stErr == nil && info.Size() > 0 {
				if fresh, _ := model.GetFineTuningJobByJobID(job.JobID); fresh != nil && fresh.Status == "succeeded" {
					return destZip, fresh.TrainedTokens, nil
				}
			}

			if errTxt, eErr := readRT("ft_error.txt"); eErr == nil && strings.TrimSpace(errTxt) != "" {
				return "", 0, fmt.Errorf("%s", strings.TrimSpace(errTxt))
			}

			if doneTxt, dErr := readRT("ft_done.json"); dErr == nil && strings.TrimSpace(doneTxt) != "" {
				var doneMeta struct {
					TrainedTokens int64 `json:"trained_tokens"`
				}
				_ = common.UnmarshalJsonStr(doneTxt, &doneMeta)
				if info, stErr := os.Stat(destZip); stErr != nil || info.Size() == 0 {
					// Download output.zip directly over runtime proxy (works even on localhost!)
					zipBytes, dlErr := client.DownloadBinaryFile(proxyURL, rt.ConnectionInfo.Token, "output.zip")
					if dlErr != nil {
						zipBytes, dlErr = client.DownloadBinaryFile(proxyURL, rt.ConnectionInfo.Token, "content/output.zip")
					}
					if dlErr != nil || len(zipBytes) == 0 {
						return "", 0, fmt.Errorf("training finished but retrieving trained archive failed: %v", dlErr)
					}
					if wErr := os.WriteFile(destZip, zipBytes, 0o644); wErr != nil {
						return "", 0, wErr
					}
				}
				return destZip, doneMeta.TrainedTokens, nil
			}
		}
	}
}

func runFineTuningOnKaggle(ctx context.Context, job *model.FineTuningJob, acc *DeploymentAccount, script, destZip string) (string, int64, error) {
	if buildFineTuningServerBaseURL() == "" {
		return "", 0, fmt.Errorf("ServerAddress must be configured (public URL) so the remote worker can download the dataset and upload the trained .zip archive")
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
		Accelerator: job.GPU,
		IsPrivate:   true,
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to submit training kernel: %w", err)
	}
	service.AppendFineTuningLogFromWorker(job.UserId, job.JobID, "worker", fmt.Sprintf("Training kernel queued (v%d, accelerator=%s)", pushed.VersionNumber, job.GPU))

	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return "", 0, ctx.Err()
		case <-ticker.C:
			if fresh, _ := model.GetFineTuningJobByJobID(job.JobID); fresh != nil {
				if fresh.Status == "succeeded" {
					if info, stErr := os.Stat(destZip); stErr == nil && info.Size() > 0 {
						return destZip, fresh.TrainedTokens, nil
					}
				}
				if fresh.Status == "failed" {
					return "", 0, fmt.Errorf("%s", fresh.Error)
				}
				if fresh.Status == "cancelled" {
					return "", 0, context.Canceled
				}
			}
			ks, stErr := client.GetKernelStatus(kUser, kernelSlug)
			if stErr == nil && ks != nil {
				st := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(ks.Status)), "kernelworkerstatus.")
				switch st {
				case "error":
					msg := strings.TrimSpace(ks.FailureMessage)
					if msg == "" {
						msg = "remote training kernel failed"
					}
					return "", 0, fmt.Errorf("%s", msg)
				case "cancel_requested", "cancel_acknowledged":
					return "", 0, context.Canceled
				case "complete":
					if info, sErr := os.Stat(destZip); sErr == nil && info.Size() > 0 {
						fresh, _ := model.GetFineTuningJobByJobID(job.JobID)
						var tok int64
						if fresh != nil {
							tok = fresh.TrainedTokens
						}
						return destZip, tok, nil
					}
					return "", 0, fmt.Errorf("training kernel completed without uploading trained model archive; ensure ServerAddress is publicly reachable")
				}
			}
		}
	}
}
