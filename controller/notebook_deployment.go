package controller

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/colab"
	"github.com/QuantumNous/new-api/pkg/hfendpoints"
	"github.com/QuantumNous/new-api/pkg/kaggle"
	"github.com/QuantumNous/new-api/pkg/notebookscript"
	"github.com/gin-gonic/gin"
)

var (
	colabBaseURLOverride  string
	kaggleBaseURLOverride string
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
		common.OptionMapRWMutex.RLock()
		token = strings.TrimSpace(common.OptionMap["model_deployment.colab.access_token"])
		common.OptionMapRWMutex.RUnlock()
	}
	if token == "" {
		common.ApiErrorMsg(c, "Google Colab OAuth access token is required")
		return
	}

	specs, err := newColabClient(token).ListRuntimeSpecs()
	if err != nil {
		common.ApiErrorMsg(c, fmt.Sprintf("failed to connect to Google Colab: %v", err))
		return
	}
	common.ApiSuccess(c, gin.H{
		"provider":    "colab",
		"status":      "connected",
		"specs_count": len(specs.RuntimeSpecs),
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

func CreateNotebookDeployment(c *gin.Context) {
	var req struct {
		Provider           string `json:"provider"`
		Name               string `json:"name"`
		ModelRepo          string `json:"model_repo"`
		ModelFile          string `json:"model_file"`
		TaskType           string `json:"task_type"`
		Accelerator        string `json:"accelerator"`
		IdleTimeoutSeconds int    `json:"idle_timeout_seconds"`
		EndpointURL        string `json:"endpoint_url"`
		HFToken            string `json:"hf_token"`
	}
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "invalid request payload")
		return
	}

	provider := strings.TrimSpace(strings.ToLower(req.Provider))
	if provider != "colab" && provider != "kaggle" {
		common.ApiErrorMsg(c, "provider must be colab or kaggle")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.ModelRepo = strings.TrimSpace(req.ModelRepo)
	if req.Name == "" || req.ModelRepo == "" {
		common.ApiErrorMsg(c, "name and model_repo are required")
		return
	}

	taskType := notebookscript.NormalizeTaskType(req.TaskType)

	common.OptionMapRWMutex.RLock()
	colabEnabled := common.OptionMap["model_deployment.colab.enabled"] == "true"
	colabToken := strings.TrimSpace(common.OptionMap["model_deployment.colab.access_token"])
	colabDefaultHW := strings.TrimSpace(common.OptionMap["model_deployment.colab.hardware"])
	colabDefaultIdle, _ := strconv.Atoi(common.OptionMap["model_deployment.colab.idle_timeout_seconds"])

	kaggleEnabled := common.OptionMap["model_deployment.kaggle.enabled"] == "true"
	kaggleUser := strings.TrimSpace(common.OptionMap["model_deployment.kaggle.username"])
	kaggleKey := strings.TrimSpace(common.OptionMap["model_deployment.kaggle.api_key"])
	kaggleDefaultHW := strings.TrimSpace(common.OptionMap["model_deployment.kaggle.accelerator"])
	kaggleDefaultIdle, _ := strconv.Atoi(common.OptionMap["model_deployment.kaggle.idle_timeout_seconds"])
	common.OptionMapRWMutex.RUnlock()

	idleTimeout := req.IdleTimeoutSeconds
	if idleTimeout <= 0 {
		if provider == "colab" && colabDefaultIdle > 0 {
			idleTimeout = colabDefaultIdle
		} else if provider == "kaggle" && kaggleDefaultIdle > 0 {
			idleTimeout = kaggleDefaultIdle
		} else {
			idleTimeout = 300
		}
	}

	accelerator := strings.TrimSpace(req.Accelerator)
	if accelerator == "" {
		if provider == "colab" {
			accelerator = colabDefaultHW
			if accelerator == "" {
				accelerator = "NVIDIA_TESLA_T4"
			}
		} else {
			accelerator = kaggleDefaultHW
			if accelerator == "" {
				accelerator = "NvidiaTeslaT4"
			}
		}
	}

	script := notebookscript.Generate(notebookscript.Config{
		Provider:           provider,
		DeploymentName:     req.Name,
		ModelRepo:          req.ModelRepo,
		ModelFile:          strings.TrimSpace(req.ModelFile),
		TaskType:           taskType,
		Accelerator:        accelerator,
		IdleTimeoutSeconds: idleTimeout,
		HFToken:            strings.TrimSpace(req.HFToken),
	})

	now := time.Now().Unix()
	item := &model.NotebookDeployment{
		Provider:           provider,
		Name:               req.Name,
		ModelRepo:          req.ModelRepo,
		ModelFile:          strings.TrimSpace(req.ModelFile),
		TaskType:           taskType,
		Accelerator:        accelerator,
		IdleTimeoutSeconds: idleTimeout,
		Status:             "running",
		EndpointURL:        strings.TrimSpace(req.EndpointURL),
		Script:             script,
		CreatedTime:        now,
		UpdatedTime:        now,
	}

	if provider == "kaggle" {
		if !kaggleEnabled || kaggleKey == "" {
			common.ApiErrorMsg(c, "Kaggle deployment is not enabled or API key is missing")
			return
		}
		if kaggleUser == "" {
			common.ApiErrorMsg(c, "Kaggle username is required in System Settings")
			return
		}
		slug := kaggleUser + "/" + sanitizeSlug(req.Name)
		pushed, err := newKaggleClient(kaggleUser, kaggleKey).PushKernel(kaggle.PushKernelRequest{
			Slug:        slug,
			Title:       req.Name,
			ScriptText:  script,
			Accelerator: accelerator,
			IsPrivate:   true,
		})
		if err != nil {
			common.ApiErrorMsg(c, fmt.Sprintf("failed to push Kaggle kernel: %v", err))
			return
		}
		item.RemoteID = pushed.Ref
		if item.RemoteID == "" {
			item.RemoteID = slug
		}
		item.RemoteURL = pushed.URL
	} else {
		if !colabEnabled || colabToken == "" {
			common.ApiErrorMsg(c, "Google Colab deployment is not enabled or access token is missing")
			return
		}
		client := newColabClient(colabToken)
		rt, err := client.CreateRuntime(accelerator)
		if err != nil {
			common.ApiErrorMsg(c, fmt.Sprintf("failed to provision Google Colab runtime: %v", err))
			return
		}
		item.RemoteID = rt.Name
		proxyURL := strings.TrimSpace(rt.ConnectionInfo.URL)
		if proxyURL == "" {
			proxyURL = strings.TrimSpace(rt.ProxyURL)
		}
		item.RemoteURL = proxyURL
		if proxyURL != "" {
			_ = client.UploadFile(proxyURL, rt.ConnectionInfo.Token, "serve.py", script)
			_, _ = client.StartKernel(proxyURL, rt.ConnectionInfo.Token)
		}
	}

	if err := model.CreateNotebookDeployment(item); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
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
		colabToken := strings.TrimSpace(common.OptionMap["model_deployment.colab.access_token"])
		kaggleUser := strings.TrimSpace(common.OptionMap["model_deployment.kaggle.username"])
		kaggleKey := strings.TrimSpace(common.OptionMap["model_deployment.kaggle.api_key"])
		common.OptionMapRWMutex.RUnlock()

		if item.Provider == "colab" && colabToken != "" {
			_ = newColabClient(colabToken).DeleteRuntime(item.RemoteID)
		} else if item.Provider == "kaggle" && kaggleKey != "" {
			parts := strings.SplitN(item.RemoteID, "/", 2)
			if len(parts) == 2 {
				_ = newKaggleClient(kaggleUser, kaggleKey).DeleteKernel(parts[0], parts[1])
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
		Prompt      string `json:"prompt"`
		EndpointURL string `json:"endpoint_url"`
	}
	rawBody, _ := c.GetRawData()
	if len(bytes.TrimSpace(rawBody)) > 0 {
		_ = common.Unmarshal(rawBody, &req)
	}

	targetURL := strings.TrimSpace(req.EndpointURL)
	if targetURL == "" {
		targetURL = strings.TrimSpace(item.EndpointURL)
	}
	if targetURL == "" {
		common.ApiErrorMsg(c, "endpoint_url is not configured for this deployment yet")
		return
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "Hello! Reply briefly."
	}

	result, testErr := executeModelTest(targetURL, item.ModelRepo, item.TaskType, prompt, "")
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
	client := &http.Client{Timeout: 60 * time.Second}

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
