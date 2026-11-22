package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/colab"
	"github.com/QuantumNous/new-api/pkg/hfendpoints"
	"github.com/QuantumNous/new-api/pkg/kaggle"
	"github.com/QuantumNous/new-api/pkg/modaldeploy"
	"github.com/QuantumNous/new-api/pkg/notebookscript"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

var (
	colabBaseURLOverride  string
	kaggleBaseURLOverride string
	colabRefresherOnce    sync.Once
)

func newColabClient(token string) *colab.Client {
	client := colab.NewClient(token)
	if colabBaseURLOverride != "" {
		client.BaseURL = colabBaseURLOverride
	}
	return client
}

func newKaggleClient(username, apiKey string) *kaggle.Client {
	client := kaggle.NewClient(username, apiKey)
	if kaggleBaseURLOverride != "" {
		client.BaseURL = kaggleBaseURLOverride
	}
	return client
}

func persistColabOption(key, value string) {
	common.OptionMapRWMutex.Lock()
	common.OptionMap[key] = value
	common.OptionMapRWMutex.Unlock()
	_ = model.UpdateOption(key, value)
}

// StartColabTokenRefresher starts a background goroutine that periodically
// refreshes the Google Colab OAuth access token before it expires.
func StartColabTokenRefresher() {
	colabRefresherOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(25 * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				common.OptionMapRWMutex.RLock()
				enabled := common.OptionMap["model_deployment.colab.enabled"] == "true"
				refreshTok := strings.TrimSpace(common.OptionMap["model_deployment.colab.refresh_token"])
				common.OptionMapRWMutex.RUnlock()
				if !enabled || refreshTok == "" {
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				_, _ = getValidColabAccessToken(ctx)
				cancel()
			}
		}()
	})
}

func getValidColabAccessToken(ctx context.Context) (string, error) {
	StartColabTokenRefresher()
	common.OptionMapRWMutex.RLock()
	accessTok := strings.TrimSpace(common.OptionMap["model_deployment.colab.access_token"])
	refreshTok := strings.TrimSpace(common.OptionMap["model_deployment.colab.refresh_token"])
	expiresAtStr := strings.TrimSpace(common.OptionMap["model_deployment.colab.token_expires_at"])
	common.OptionMapRWMutex.RUnlock()

	needsRefresh := accessTok == ""
	if !needsRefresh && expiresAtStr != "" {
		if exp, err := time.Parse(time.RFC3339, expiresAtStr); err == nil {
			if time.Until(exp) < 5*time.Minute {
				needsRefresh = true
			}
		}
	}

	if !needsRefresh {
		return accessTok, nil
	}
	if refreshTok == "" {
		if accessTok != "" {
			return accessTok, nil
		}
		return "", fmt.Errorf("Google Colab OAuth credentials missing; please sign in with Google")
	}

	newAccess, newExp, err := service.RefreshColabToken(ctx, refreshTok)
	if err != nil {
		if accessTok != "" {
			return accessTok, nil
		}
		return "", err
	}
	persistColabOption("model_deployment.colab.access_token", newAccess)
	persistColabOption("model_deployment.colab.token_expires_at", newExp.Format(time.RFC3339))
	return newAccess, nil
}

func StartColabOAuth(c *gin.Context) {
	flow, err := channelOAuthService.Generate(service.ChannelOAuthColab)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, flow)
}

func ExchangeColabOAuth(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
		Callback  string `json:"callback" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	credJSON, err := channelOAuthService.Exchange(context.WithoutCancel(c.Request.Context()), service.ChannelOAuthColab, req.SessionID, req.Callback)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var cred struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		Expired      string `json:"expired"`
		Email        string `json:"email"`
		ProjectID    string `json:"project_id"`
	}
	if err := common.Unmarshal([]byte(credJSON), &cred); err != nil {
		common.ApiErrorMsg(c, "failed to parse OAuth credentials")
		return
	}
	persistColabOption("model_deployment.colab.access_token", cred.AccessToken)
	persistColabOption("model_deployment.colab.refresh_token", cred.RefreshToken)
	persistColabOption("model_deployment.colab.token_expires_at", cred.Expired)
	persistColabOption("model_deployment.colab.account_email", cred.Email)
	if cred.ProjectID != "" {
		persistColabOption("model_deployment.colab.project_id", cred.ProjectID)
	}
	StartColabTokenRefresher()

	common.ApiSuccess(c, gin.H{
		"email":      cred.Email,
		"expires_at": cred.Expired,
		"project_id": cred.ProjectID,
	})
}

func DisconnectColabOAuth(c *gin.Context) {
	persistColabOption("model_deployment.colab.access_token", "")
	persistColabOption("model_deployment.colab.refresh_token", "")
	persistColabOption("model_deployment.colab.token_expires_at", "")
	persistColabOption("model_deployment.colab.account_email", "")
	common.ApiSuccess(c, gin.H{"disconnected": true})
}

func TestColabConnection(c *gin.Context) {
	var req struct {
		AccessToken string `json:"access_token"`
	}
	rawBody, _ := c.GetRawData()
	if len(bytes.TrimSpace(rawBody)) > 0 {
		_ = common.Unmarshal(rawBody, &req)
	}
	token := strings.TrimSpace(req.AccessToken)
	if token == "" {
		var err error
		token, err = getValidColabAccessToken(c.Request.Context())
		if err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
	}

	count, err := newColabClient(token).TestConnection()
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("failed to connect to Google Colab: %v", err))
		return
	}
	common.ApiSuccess(c, gin.H{
		"provider":       "colab",
		"status":         "connected",
		"runtimes_count": count,
	})
}

func TestKaggleConnection(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		ApiKey   string `json:"api_key"`
	}
	rawBody, _ := c.GetRawData()
	if len(bytes.TrimSpace(rawBody)) > 0 {
		_ = common.Unmarshal(rawBody, &req)
	}
	username := strings.TrimSpace(req.Username)
	apiKey := strings.TrimSpace(req.ApiKey)

	common.OptionMapRWMutex.RLock()
	if username == "" {
		username = strings.TrimSpace(common.OptionMap["model_deployment.kaggle.username"])
	}
	if apiKey == "" {
		apiKey = strings.TrimSpace(common.OptionMap["model_deployment.kaggle.api_key"])
	}
	common.OptionMapRWMutex.RUnlock()

	if apiKey == "" {
		common.ApiErrorMsg(c, "Kaggle API key is required")
		return
	}

	kernels, err := newKaggleClient(username, apiKey).TestConnection()
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("failed to connect to Kaggle: %v", err))
		return
	}
	common.ApiSuccess(c, gin.H{
		"provider":      "kaggle",
		"status":        "connected",
		"kernels_count": len(kernels),
	})
}

func ListNotebookDeployments(c *gin.Context) {
	provider := strings.TrimSpace(strings.ToLower(c.Query("provider")))
	items, err := model.ListNotebookDeployments(provider)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func GetNotebookDeployment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid deployment id")
		return
	}
	item, err := model.GetNotebookDeploymentByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

var slugSanitizerRe = regexp.MustCompile(`[^a-z0-9-]+`)

func sanitizeSlug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugSanitizerRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "model-server"
	}
	return s
}

func isValidDeploymentProvider(p string) bool {
	switch p {
	case "colab", "kaggle", "modal", "huggingface", "ionet", "hostinger", "runpod":
		return true
	default:
		return false
	}
}

func getProviderIdleTimeoutSeconds(provider string) int {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	raw := strings.TrimSpace(common.OptionMap["model_deployment."+provider+".idle_timeout_seconds"])
	if raw == "" {
		return 300
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func buildAcceleratorCandidates(primary string, fallbacks []string) []string {
	seen := make(map[string]bool)
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	add(primary)
	for _, fb := range fallbacks {
		add(fb)
	}
	return out
}

func setDeploymentStatus(id int, status, message string) {
	_ = model.UpdateNotebookDeploymentFields(id, map[string]any{
		"status":         status,
		"status_message": message,
		"updated_time":   time.Now().Unix(),
	})
}

func setDeploymentFailed(id int, errMsg string) {
	_ = model.UpdateNotebookDeploymentFields(id, map[string]any{
		"status":         "failed",
		"status_message": errMsg,
		"error_message":  errMsg,
		"updated_time":   time.Now().Unix(),
	})
}

func CreateNotebookDeployment(c *gin.Context) {
	var req struct {
		Provider             string   `json:"provider"`
		Name                 string   `json:"name"`
		ModelRepo            string   `json:"model_repo"`
		ModelFile            string   `json:"model_file"`
		TaskType             string   `json:"task_type"`
		Accelerator          string   `json:"accelerator"`
		FallbackAccelerators []string `json:"fallback_accelerators"`
		HFToken              string   `json:"hf_token"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request payload")
		return
	}

	provider := strings.TrimSpace(strings.ToLower(req.Provider))
	if !isValidDeploymentProvider(provider) {
		common.ApiErrorMsg(c, "unsupported deployment provider")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.ModelRepo = strings.TrimSpace(req.ModelRepo)
	if req.Name == "" || req.ModelRepo == "" {
		common.ApiErrorMsg(c, "name and model_repo are required")
		return
	}

	// Validate provider credentials upfront before creating DB record
	common.OptionMapRWMutex.RLock()
	modalEnabled := common.OptionMap["model_deployment.modal.enabled"] == "true"
	modalTokenID := strings.TrimSpace(common.OptionMap["model_deployment.modal.token_id"])
	modalTokenSecret := strings.TrimSpace(common.OptionMap["model_deployment.modal.token_secret"])
	modalVolPath := strings.TrimSpace(common.OptionMap["model_deployment.modal.shared_volume_path"])

	kaggleEnabled := common.OptionMap["model_deployment.kaggle.enabled"] == "true"
	kaggleUser := strings.TrimSpace(common.OptionMap["model_deployment.kaggle.username"])
	kaggleKey := strings.TrimSpace(common.OptionMap["model_deployment.kaggle.api_key"])

	colabEnabled := common.OptionMap["model_deployment.colab.enabled"] == "true"
	hfEnabled := common.OptionMap["model_deployment.huggingface.enabled"] == "true"
	hfGlobalToken := strings.TrimSpace(common.OptionMap["model_deployment.huggingface.token"])
	if hfGlobalToken == "" {
		hfGlobalToken = strings.TrimSpace(common.OptionMap["model_deployment.huggingface.api_key"])
	}
	hfDefaultOrg := strings.TrimSpace(common.OptionMap["model_deployment.huggingface.org"])
	common.OptionMapRWMutex.RUnlock()

	switch provider {
	case "modal":
		if !modalEnabled || modalTokenID == "" || modalTokenSecret == "" {
			common.ApiErrorMsg(c, "Modal deployment is not enabled or Token ID / Token Secret is missing in System Settings")
			return
		}
	case "kaggle":
		if !kaggleEnabled || kaggleKey == "" || kaggleUser == "" {
			common.ApiErrorMsg(c, "Kaggle deployment is not enabled or Username / API Key is missing in System Settings")
			return
		}
	case "colab":
		if !colabEnabled {
			common.ApiErrorMsg(c, "Google Colab deployment is not enabled in System Settings")
			return
		}
		if _, err := getValidColabAccessToken(c.Request.Context()); err != nil {
			common.ApiErrorMsg(c, err.Error())
			return
		}
	case "huggingface":
		if !hfEnabled || hfGlobalToken == "" {
			common.ApiErrorMsg(c, "Hugging Face deployment is not enabled or token is missing in System Settings")
			return
		}
	}

	taskType := notebookscript.NormalizeTaskType(req.TaskType)
	idleTimeout := getProviderIdleTimeoutSeconds(provider)
	accelerator := strings.TrimSpace(req.Accelerator)
	if accelerator == "" {
		accelerator = "CPU"
	}
	candidates := buildAcceleratorCandidates(accelerator, req.FallbackAccelerators)
	fallbacksJSON, _ := common.Marshal(req.FallbackAccelerators)

	script := notebookscript.Generate(notebookscript.Config{
		Provider:             provider,
		DeploymentName:       req.Name,
		ModelRepo:            req.ModelRepo,
		ModelFile:            strings.TrimSpace(req.ModelFile),
		TaskType:             taskType,
		Accelerator:          accelerator,
		FallbackAccelerators: req.FallbackAccelerators,
		IdleTimeoutSeconds:   idleTimeout,
		HFToken:              strings.TrimSpace(req.HFToken),
	})

	now := time.Now().Unix()
	item := &model.NotebookDeployment{
		Provider:             provider,
		Name:                 req.Name,
		ModelRepo:            req.ModelRepo,
		ModelFile:            strings.TrimSpace(req.ModelFile),
		TaskType:             taskType,
		Accelerator:          accelerator,
		FallbackAccelerators: string(fallbacksJSON),
		IdleTimeoutSeconds:   idleTimeout,
		Status:               "provisioning",
		StatusMessage:        "Initializing deployment...",
		Script:               script,
		CreatedTime:          now,
		UpdatedTime:          now,
	}

	if err := model.CreateNotebookDeployment(item); err != nil {
		common.ApiError(c, err)
		return
	}

	go provisionDeploymentAsync(provisionParams{
		ID:                 item.Id,
		Provider:           provider,
		Name:               req.Name,
		ModelRepo:          req.ModelRepo,
		ModelFile:          strings.TrimSpace(req.ModelFile),
		TaskType:           taskType,
		PrimaryAccelerator: accelerator,
		Fallbacks:          req.FallbackAccelerators,
		Candidates:         candidates,
		IdleTimeoutSeconds: idleTimeout,
		HFToken:            strings.TrimSpace(req.HFToken),
		Script:             script,
		ModalTokenID:       modalTokenID,
		ModalTokenSecret:   modalTokenSecret,
		ModalVolPath:       modalVolPath,
		KaggleUser:         kaggleUser,
		KaggleKey:          kaggleKey,
		HFGlobalToken:      hfGlobalToken,
		HFDefaultOrg:       hfDefaultOrg,
	})

	common.ApiSuccess(c, item)
}

type provisionParams struct {
	ID                 int
	Provider           string
	Name               string
	ModelRepo          string
	ModelFile          string
	TaskType           string
	PrimaryAccelerator string
	Fallbacks          []string
	Candidates         []string
	IdleTimeoutSeconds int
	HFToken            string
	Script             string
	ModalTokenID       string
	ModalTokenSecret   string
	ModalVolPath       string
	KaggleUser         string
	KaggleKey          string
	HFGlobalToken      string
	HFDefaultOrg       string
}

func provisionDeploymentAsync(p provisionParams) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	switch p.Provider {
	case "modal":
		setDeploymentStatus(p.ID, "downloading", "Deploying container & downloading model on Modal...")
		appName := sanitizeSlug(p.Name)
		endpointURL, modalScript, err := modaldeploy.Deploy(ctx, modaldeploy.DeployConfig{
			AppName:              appName,
			ModelRepo:            p.ModelRepo,
			ModelFile:            p.ModelFile,
			TaskType:             p.TaskType,
			Accelerator:          p.PrimaryAccelerator,
			FallbackAccelerators: p.Fallbacks,
			IdleTimeoutSeconds:   p.IdleTimeoutSeconds,
			TokenID:              p.ModalTokenID,
			TokenSecret:          p.ModalTokenSecret,
			HFToken:              p.HFToken,
			SharedVolumePath:     p.ModalVolPath,
		})
		if err != nil {
			setDeploymentFailed(p.ID, err.Error())
			return
		}
		_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
			"remote_id":      appName,
			"remote_url":     endpointURL,
			"endpoint_url":   endpointURL,
			"script":         modalScript,
			"status":         "starting",
			"status_message": "Warming up Modal endpoint...",
			"updated_time":   time.Now().Unix(),
		})
		probeHealth(endpointURL)
		setDeploymentStatus(p.ID, "running", "Running")

	case "kaggle":
		setDeploymentStatus(p.ID, "downloading", "Pushing kernel to Kaggle...")
		slug := p.KaggleUser + "/" + sanitizeSlug(p.Name)
		client := newKaggleClient(p.KaggleUser, p.KaggleKey)
		var pushed *kaggle.PushKernelResponse
		var chosenAcc string
		var lastErr error
		for _, cand := range p.Candidates {
			res, err := client.PushKernel(kaggle.PushKernelRequest{
				Slug:        slug,
				Title:       p.Name,
				ScriptText:  p.Script,
				Accelerator: cand,
				IsPrivate:   true,
			})
			if err == nil {
				pushed = res
				chosenAcc = cand
				break
			}
			lastErr = err
		}
		if pushed == nil {
			setDeploymentFailed(p.ID, fmt.Sprintf("Kaggle push failed (tried %v): %v", p.Candidates, lastErr))
			return
		}
		remoteID := pushed.Ref
		if remoteID == "" {
			remoteID = slug
		}
		_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
			"accelerator":    chosenAcc,
			"remote_id":      remoteID,
			"remote_url":     pushed.URL,
			"status":         "starting",
			"status_message": "Kernel queued/running on Kaggle...",
			"updated_time":   time.Now().Unix(),
		})
		setDeploymentStatus(p.ID, "running", "Running on Kaggle")

	case "colab":
		setDeploymentStatus(p.ID, "starting", "Provisioning Google Colab runtime...")
		tok, err := getValidColabAccessToken(ctx)
		if err != nil {
			setDeploymentFailed(p.ID, err.Error())
			return
		}
		client := newColabClient(tok)
		var rt *colab.Runtime
		var chosenAcc string
		var lastErr error
		for _, cand := range p.Candidates {
			r, err := client.CreateRuntime(cand)
			if err == nil {
				rt = r
				chosenAcc = cand
				break
			}
			lastErr = err
		}
		if rt == nil {
			setDeploymentFailed(p.ID, fmt.Sprintf("Google Colab provisioning failed (tried %v): %v", p.Candidates, lastErr))
			return
		}
		proxyURL := strings.TrimSpace(rt.ConnectionInfo.URL)
		if proxyURL == "" {
			proxyURL = strings.TrimSpace(rt.ProxyURL)
		}
		if proxyURL != "" {
			setDeploymentStatus(p.ID, "downloading", "Uploading model server & starting Colab kernel...")
			_ = client.UploadFile(proxyURL, rt.ConnectionInfo.Token, "serve.py", p.Script)
			_, _ = client.StartKernel(proxyURL, rt.ConnectionInfo.Token)
		}
		_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
			"accelerator":    chosenAcc,
			"remote_id":      rt.Name,
			"remote_url":     proxyURL,
			"status":         "running",
			"status_message": "Running on Google Colab",
			"updated_time":   time.Now().Unix(),
		})

	case "huggingface":
		setDeploymentStatus(p.ID, "downloading", "Creating Hugging Face Inference Endpoint...")
		ns := p.HFDefaultOrg
		if ns == "" {
			if resolved, err := hfWhoamiName(p.HFGlobalToken); err == nil {
				ns = resolved
			}
		}
		if ns == "" {
			setDeploymentFailed(p.ID, "Hugging Face organization/username could not be resolved")
			return
		}
		scaleToZeroMin := 0
		if p.IdleTimeoutSeconds > 0 {
			scaleToZeroMin = (p.IdleTimeoutSeconds + 59) / 60
		}
		client := hfendpoints.NewClient(p.HFGlobalToken)
		var ep *hfendpoints.Endpoint
		var chosenAcc string
		var lastErr error
		for _, cand := range p.Candidates {
			created, err := client.CreateEndpoint(hfendpoints.CreateEndpointRequest{
				Name:                      sanitizeSlug(p.Name),
				Namespace:                 ns,
				Repository:                p.ModelRepo,
				Task:                      p.TaskType,
				Hardware:                  cand,
				Region:                    "us-east-1",
				Protected:                 true,
				ScaleToZeroTimeoutMinutes: scaleToZeroMin,
			})
			if err == nil {
				ep = created
				chosenAcc = cand
				break
			}
			lastErr = err
		}
		if ep == nil {
			setDeploymentFailed(p.ID, fmt.Sprintf("Hugging Face endpoint creation failed (tried %v): %v", p.Candidates, lastErr))
			return
		}
		_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
			"accelerator":    chosenAcc,
			"remote_id":      ep.Namespace + "/" + ep.Name,
			"remote_url":     ep.Status.URL,
			"endpoint_url":   ep.Status.URL,
			"status":         "starting",
			"status_message": "Building Hugging Face container...",
			"updated_time":   time.Now().Unix(),
		})
		// Poll briefly for URL if not immediately returned
		for i := 0; i < 20; i++ {
			time.Sleep(6 * time.Second)
			latest, err := client.GetEndpoint(ns, ep.Name)
			if err != nil {
				continue
			}
			st := strings.ToLower(latest.Status.State)
			if latest.Status.URL != "" && strings.Contains(st, "run") {
				_ = model.UpdateNotebookDeploymentFields(p.ID, map[string]any{
					"remote_url":     latest.Status.URL,
					"endpoint_url":   latest.Status.URL,
					"status":         "running",
					"status_message": "Running",
					"updated_time":   time.Now().Unix(),
				})
				return
			}
			if strings.Contains(st, "fail") || strings.Contains(st, "error") {
				setDeploymentFailed(p.ID, latest.Status.Message)
				return
			}
		}
		setDeploymentStatus(p.ID, "running", "Provisioned on Hugging Face")

	default:
		setDeploymentStatus(p.ID, "running", "Configured")
	}
}

func probeHealth(baseURL string) {
	if strings.TrimSpace(baseURL) == "" {
		return
	}
	client := &http.Client{Timeout: 10 * time.Second}
	target := strings.TrimRight(baseURL, "/") + "/health"
	for i := 0; i < 6; i++ {
		resp, err := client.Get(target)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func UpdateNotebookDeployment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid deployment id")
		return
	}
	item, err := model.GetNotebookDeploymentByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req struct {
		EndpointURL string `json:"endpoint_url"`
		Status      string `json:"status"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request payload")
		return
	}
	if u := strings.TrimSpace(req.EndpointURL); u != "" {
		item.EndpointURL = u
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		item.Status = st
	}
	item.UpdatedTime = time.Now().Unix()
	if err := model.UpdateNotebookDeployment(item); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func DeleteNotebookDeployment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid deployment id")
		return
	}
	item, err := model.GetNotebookDeploymentByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if item.RemoteID != "" {
		common.OptionMapRWMutex.RLock()
		kaggleUser := strings.TrimSpace(common.OptionMap["model_deployment.kaggle.username"])
		kaggleKey := strings.TrimSpace(common.OptionMap["model_deployment.kaggle.api_key"])
		hfToken := strings.TrimSpace(common.OptionMap["model_deployment.huggingface.token"])
		modalTokenID := strings.TrimSpace(common.OptionMap["model_deployment.modal.token_id"])
		modalTokenSecret := strings.TrimSpace(common.OptionMap["model_deployment.modal.token_secret"])
		common.OptionMapRWMutex.RUnlock()

		switch item.Provider {
		case "modal":
			_ = modaldeploy.StopApp(c.Request.Context(), item.RemoteID, modalTokenID, modalTokenSecret)
		case "colab":
			if colabToken, tokErr := getValidColabAccessToken(c.Request.Context()); tokErr == nil && colabToken != "" {
				_ = newColabClient(colabToken).DeleteRuntime(item.RemoteID)
			}
		case "kaggle":
			if kaggleKey != "" {
				parts := strings.SplitN(item.RemoteID, "/", 2)
				if len(parts) == 2 {
					_ = newKaggleClient(kaggleUser, kaggleKey).DeleteKernel(parts[0], parts[1])
				}
			}
		case "huggingface":
			if hfToken != "" {
				parts := strings.SplitN(item.RemoteID, "/", 2)
				if len(parts) == 2 {
					_ = hfendpoints.NewClient(hfToken).DeleteEndpoint(parts[0], parts[1])
				}
			}
		}
	}

	if err := model.DeleteNotebookDeployment(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"deleted": true})
}

func TestNotebookDeployment(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid deployment id")
		return
	}
	item, err := model.GetNotebookDeploymentByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req struct {
		Prompt string `json:"prompt"`
	}
	rawBody, _ := c.GetRawData()
	if len(bytes.TrimSpace(rawBody)) > 0 {
		_ = common.Unmarshal(rawBody, &req)
	}

	targetURL := strings.TrimSpace(item.EndpointURL)
	if targetURL == "" {
		if item.ErrorMessage != "" {
			common.ApiErrorMsg(c, fmt.Sprintf("deployment failed: %s", item.ErrorMessage))
		} else {
			common.ApiErrorMsg(c, fmt.Sprintf("deployment is currently %s (%s); endpoint is not ready yet", item.Status, item.StatusMessage))
		}
		return
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Hello! Reply briefly."
	}

	prevStatus := item.Status
	prevMsg := item.StatusMessage
	setDeploymentStatus(item.Id, "testing", "Testing model...")
	result, testErr := executeModelTest(targetURL, item.ModelRepo, item.TaskType, prompt, "")
	if prevStatus == "" || prevStatus == "testing" {
		prevStatus = "running"
	}
	if prevMsg == "" || prevMsg == "Testing model..." {
		prevMsg = "Running"
	}
	setDeploymentStatus(item.Id, prevStatus, prevMsg)

	if testErr != nil {
		common.ApiErrorMsg(c, testErr.Error())
		return
	}
	common.ApiSuccess(c, result)
}

func TestHfEndpoint(c *gin.Context) {
	token, ok := getHfToken(c)
	if !ok {
		return
	}
	namespace, err := hfResolveNamespace(c, token, c.Query("namespace"))
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		common.ApiErrorMsg(c, "endpoint name is required")
		return
	}
	ep, err := hfendpoints.NewClient(token).GetEndpoint(namespace, name)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(ep.Status.URL) == "" {
		common.ApiErrorMsg(c, "endpoint URL is not ready yet")
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
	}
	rawBody, _ := c.GetRawData()
	if len(bytes.TrimSpace(rawBody)) > 0 {
		_ = common.Unmarshal(rawBody, &req)
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Hello!"
	}

	result, testErr := executeModelTest(ep.Status.URL, ep.Model.Repository, ep.Model.Task, prompt, token)
	if testErr != nil {
		common.ApiErrorMsg(c, testErr.Error())
		return
	}
	common.ApiSuccess(c, result)
}

type modelTestResult struct {
	Type      string `json:"type"` // text | image | video | audio
	Text      string `json:"text,omitempty"`
	MediaURL  string `json:"media_url,omitempty"`
	LatencyMs int64  `json:"latency_ms"`
	Raw       string `json:"raw,omitempty"`
}

func executeModelTest(baseURL, modelRepo, rawTask, prompt, bearerToken string) (*modelTestResult, error) {
	task := notebookscript.NormalizeTaskType(rawTask)
	base := strings.TrimRight(baseURL, "/")
	start := time.Now()
	client := &http.Client{Timeout: 120 * time.Second}

	var endpointPath string
	var payload any

	switch task {
	case "video":
		endpointPath = "/v1/videos/generations"
		payload = map[string]any{"model": modelRepo, "prompt": prompt}
	case "image":
		endpointPath = "/v1/images/generations"
		payload = map[string]any{"model": modelRepo, "prompt": prompt}
	case "audio":
		endpointPath = "/v1/audio/speech"
		payload = map[string]any{"model": modelRepo, "input": prompt, "prompt": prompt}
	case "embeddings":
		endpointPath = "/v1/embeddings"
		payload = map[string]any{"model": modelRepo, "input": prompt}
	default:
		endpointPath = "/v1/chat/completions"
		payload = map[string]any{
			"model": modelRepo,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
			"max_tokens": 128,
		}
	}

	rawBody, err := common.Marshal(payload)
	if err != nil {
		return nil, err
	}
	target := base
	if !strings.HasSuffix(base, endpointPath) {
		target = base + endpointPath
	}
	httpReq, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(rawBody))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("model test request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read model test response: %w", err)
	}
	latency := time.Since(start).Milliseconds()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBytes))
		if len(msg) > 400 {
			msg = msg[:400] + "..."
		}
		return nil, fmt.Errorf("model endpoint returned HTTP %d: %s", resp.StatusCode, msg)
	}

	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if task == "audio" && (strings.HasPrefix(ct, "audio/") || strings.Contains(ct, "octet-stream")) {
		mime := strings.Split(ct, ";")[0]
		if mime == "" || mime == "application/octet-stream" {
			mime = "audio/mpeg"
		}
		return &modelTestResult{
			Type:      "audio",
			MediaURL:  "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(respBytes),
			LatencyMs: latency,
		}, nil
	}

	var parsed map[string]any
	if err := common.Unmarshal(respBytes, &parsed); err != nil {
		return &modelTestResult{
			Type:      "text",
			Text:      strings.TrimSpace(string(respBytes)),
			LatencyMs: latency,
			Raw:       strings.TrimSpace(string(respBytes)),
		}, nil
	}

	res := &modelTestResult{
		Type:      task,
		LatencyMs: latency,
		Raw:       string(respBytes),
	}

	switch task {
	case "video", "image", "audio":
		res.MediaURL = extractMediaURL(parsed, task)
		if res.MediaURL == "" {
			res.Type = "text"
			res.Text = string(respBytes)
		}
	case "embeddings":
		res.Type = "text"
		if data, ok := parsed["data"].([]any); ok && len(data) > 0 {
			if first, ok := data[0].(map[string]any); ok {
				if emb, ok := first["embedding"].([]any); ok {
					res.Text = fmt.Sprintf("Embedding generated (%d dimensions)", len(emb))
					return res, nil
				}
			}
		}
		res.Text = string(respBytes)
	default:
		res.Type = "text"
		res.Text = extractTextFromResponse(parsed)
		if res.Text == "" {
			res.Text = string(respBytes)
		}
	}

	return res, nil
}

func extractMediaURL(parsed map[string]any, task string) string {
	for _, key := range []string{"url", "video_url", "image_url", "audio_url"} {
		if v, ok := parsed[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if data, ok := parsed["data"].([]any); ok && len(data) > 0 {
		if item, ok := data[0].(map[string]any); ok {
			for _, key := range []string{"url", "video_url", "image_url", "audio_url"} {
				if v, ok := item[key].(string); ok && strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			}
			if b64, ok := item["b64_json"].(string); ok && strings.TrimSpace(b64) != "" {
				mime := "image/png"
				if task == "video" {
					mime = "video/mp4"
				} else if task == "audio" {
					mime = "audio/wav"
				}
				return "data:" + mime + ";base64," + strings.TrimSpace(b64)
			}
		}
	}
	return ""
}

func extractTextFromResponse(parsed map[string]any) string {
	if choices, ok := parsed["choices"].([]any); ok && len(choices) > 0 {
		if first, ok := choices[0].(map[string]any); ok {
			if msg, ok := first["message"].(map[string]any); ok {
				if content, ok := msg["content"].(string); ok {
					return content
				}
			}
			if text, ok := first["text"].(string); ok {
				return text
			}
		}
	}
	if gen, ok := parsed["generated_text"].(string); ok {
		return gen
	}
	return ""
}
