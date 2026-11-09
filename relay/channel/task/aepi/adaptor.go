package aepi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// AepiJobRequest represents the payload for POST /v1/jobs
type AepiJobRequest struct {
	Operation  string `json:"operation"`
	Model      string `json:"model,omitempty"`
	Prompt     string `json:"prompt"`
	Source     string `json:"source,omitempty"`
	Duration   int    `json:"duration,omitempty"`
	Aspect     string `json:"aspect,omitempty"`
	Resolution string `json:"resolution,omitempty"`
	Style      string `json:"style,omitempty"`
	Fit        string `json:"fit,omitempty"`
	Engine     string `json:"engine,omitempty"`
	Audio      *bool  `json:"audio,omitempty"`
}

// AepiJobOutput represents output file info
type AepiJobOutput struct {
	Url  string `json:"url"`
	Name string `json:"name,omitempty"`
	Size int64  `json:"size,omitempty"`
	Kind string `json:"kind,omitempty"`
}

// AepiJobResponse represents the response from POST /v1/jobs and GET /v1/jobs/{id}
type AepiJobResponse struct {
	Id        string         `json:"id"`
	Operation string         `json:"operation,omitempty"`
	Model     string         `json:"model,omitempty"`
	Prompt    string         `json:"prompt,omitempty"`
	Status    string         `json:"status"` // "queued", "processing", "done", "error"
	Progress  int            `json:"progress"`
	Error     string         `json:"error,omitempty"`
	Output    *AepiJobOutput `json:"output,omitempty"`
	CreatedAt int64          `json:"created_at,omitempty"`
	UpdatedAt int64          `json:"updated_at,omitempty"`
	Duration  int            `json:"duration,omitempty"`
}

// AepiUploadResponse represents the upload endpoint response
type AepiUploadResponse struct {
	Id   string `json:"id"`
	Name string `json:"name,omitempty"`
	Size int64  `json:"size,omitempty"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
	proxy       string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	if a.baseURL == "" {
		a.baseURL = "https://videoapi.aerobox.eu.cc"
	}
	a.apiKey = info.ApiKey
	a.proxy = info.ChannelSetting.Proxy
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	contentType := c.GetHeader("Content-Type")
	var req relaycommon.TaskSubmitReq

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if mf, mfErr := c.MultipartForm(); mfErr == nil && mf != nil {
			if prompts, ok := mf.Value["prompt"]; ok && len(prompts) > 0 {
				req.Prompt = prompts[0]
			}
			if models, ok := mf.Value["model"]; ok && len(models) > 0 {
				req.Model = models[0]
			}
			if durations, ok := mf.Value["duration"]; ok && len(durations) > 0 {
				if d, err := strconv.Atoi(durations[0]); err == nil {
					req.Duration = d
				}
			}
			if sizes, ok := mf.Value["size"]; ok && len(sizes) > 0 {
				req.Size = sizes[0]
			}
			if seconds, ok := mf.Value["seconds"]; ok && len(seconds) > 0 {
				req.Seconds = seconds[0]
			}
			if refs, ok := mf.Value["input_reference"]; ok && len(refs) > 0 {
				req.InputReference = refs[0]
			}
			if sources, ok := mf.Value["source"]; ok && len(sources) > 0 {
				req.InputReference = sources[0]
			}
		}
	} else {
		if err := common.UnmarshalBodyReusable(c, &req); err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
	}

	// Rescale operations do not require a prompt
	isRescale := req.Model == "rescale"
	if req.Metadata != nil {
		if op, ok := req.Metadata["operation"].(string); ok && op == "rescale" {
			isRescale = true
		}
	}

	if strings.TrimSpace(req.Prompt) == "" {
		if isRescale {
			req.Prompt = "Rescale media"
		} else {
			return service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest)
		}
	}

	if req.Duration > 30 {
		req.Duration = 30
	}

	c.Set("task_request", req)
	info.Action = constant.TaskActionGenerate
	return nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	ratios := make(map[string]float64)
	v, exists := c.Get("task_request")
	if exists {
		if req, ok := v.(relaycommon.TaskSubmitReq); ok {
			seconds := req.Duration
			if seconds <= 0 && req.Seconds != "" {
				if s, err := strconv.Atoi(req.Seconds); err == nil {
					seconds = s
				}
			}
			if seconds > 0 {
				ratios["seconds"] = float64(seconds)
			}
			if req.Size == "1920x1080" || req.Size == "1080x1920" || strings.Contains(req.Size, "1080p") {
				ratios["size"] = 1.5
			} else if strings.Contains(req.Size, "4k") {
				ratios["size"] = 2.0
			}
		}
	}
	if len(ratios) == 0 {
		return nil
	}
	return ratios
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/v1/jobs", a.baseURL), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("X-API-Key", a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil, fmt.Errorf("invalid task request type")
	}

	jobReq, err := a.convertToAepiJobRequest(c, &req, info)
	if err != nil {
		return nil, err
	}

	data, err := common.Marshal(jobReq)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		taskErr = service.TaskErrorWrapperLocal(fmt.Errorf("upstream error %d: %s", resp.StatusCode, string(responseBody)), "upstream_error", resp.StatusCode)
		return
	}

	var aepiResp AepiJobResponse
	if err := common.Unmarshal(responseBody, &aepiResp); err != nil {
		taskErr = service.TaskErrorWrapper(err, "unmarshal_response_failed", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)
	return aepiResp.Id, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	url := fmt.Sprintf("%s/v1/jobs/%s", strings.TrimRight(baseUrl, "/"), taskID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("X-API-Key", key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	taskInfo := &relaycommon.TaskInfo{}
	var job AepiJobResponse
	if err := common.Unmarshal(respBody, &job); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal response body")
	}

	taskInfo.TaskID = job.Id
	taskInfo.Reason = job.Error

	switch strings.ToLower(job.Status) {
	case "queued":
		taskInfo.Status = model.TaskStatusSubmitted
		taskInfo.Progress = fmt.Sprintf("%d%%", job.Progress)
	case "processing":
		taskInfo.Status = model.TaskStatusInProgress
		taskInfo.Progress = fmt.Sprintf("%d%%", job.Progress)
	case "done":
		taskInfo.Status = model.TaskStatusSuccess
		taskInfo.Progress = "100%"
		if job.Output != nil && job.Output.Url != "" {
			u := job.Output.Url
			if strings.HasPrefix(u, "/") {
				u = "https://videoapi.aerobox.eu.cc" + u
			}
			taskInfo.Url = u
		}
	case "error":
		taskInfo.Status = model.TaskStatusFailure
		taskInfo.Progress = "100%"
		if job.Error != "" {
			taskInfo.Reason = job.Error
		} else {
			taskInfo.Reason = "generation failed on aepi engine"
		}
	default:
		taskInfo.Status = model.TaskStatusInProgress
		taskInfo.Progress = fmt.Sprintf("%d%%", job.Progress)
	}

	return taskInfo, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var job AepiJobResponse
	if len(originTask.Data) > 0 {
		_ = common.Unmarshal(originTask.Data, &job)
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)

	if job.CreatedAt > 0 {
		openAIVideo.CreatedAt = job.CreatedAt
	} else if originTask.SubmitTime > 0 {
		openAIVideo.CreatedAt = originTask.SubmitTime
	}
	if job.UpdatedAt > 0 {
		openAIVideo.CompletedAt = job.UpdatedAt
	} else if originTask.FinishTime > 0 {
		openAIVideo.CompletedAt = originTask.FinishTime
	}

	resultUrl := originTask.GetResultURL()
	if resultUrl == "" && job.Output != nil && job.Output.Url != "" {
		resultUrl = job.Output.Url
		if strings.HasPrefix(resultUrl, "/") {
			resultUrl = "https://videoapi.aerobox.eu.cc" + resultUrl
		}
	}

	if resultUrl != "" {
		openAIVideo.SetMetadata("url", resultUrl)
	}

	if job.Duration > 0 {
		openAIVideo.Seconds = fmt.Sprintf("%d", job.Duration)
	}

	if originTask.Status == model.TaskStatusFailure {
		failReason := originTask.FailReason
		if failReason == "" && job.Error != "" {
			failReason = job.Error
		}
		if failReason == "" {
			failReason = "Media generation failed"
		}
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: failReason,
			Code:    "task_failed",
		}
	}

	return common.Marshal(openAIVideo)
}

func (a *TaskAdaptor) GetModelList() []string {
	return AllModels
}

func (a *TaskAdaptor) GetChannelName() string {
	return "aepi"
}

// ============================================================================
// Helpers & Payload Converters
// ============================================================================

func (a *TaskAdaptor) convertToAepiJobRequest(c *gin.Context, req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo) (*AepiJobRequest, error) {
	job := &AepiJobRequest{
		Prompt: req.Prompt,
	}

	// 1. Resolve source input if any (file upload, URL, base64)
	rawSource := req.InputReference
	if rawSource == "" && req.Image != "" {
		rawSource = req.Image
	}
	if rawSource == "" && len(req.Images) > 0 {
		rawSource = req.Images[0]
	}
	if req.Metadata != nil {
		if s, ok := req.Metadata["source"].(string); ok && s != "" {
			rawSource = s
		}
	}

	sourceID, err := a.resolveSource(c, rawSource)
	if err != nil {
		return nil, fmt.Errorf("failed to process reference source: %w", err)
	}
	job.Source = sourceID

	// 2. Determine operation
	operation := ""
	if req.Metadata != nil {
		if op, ok := req.Metadata["operation"].(string); ok && op != "" {
			operation = strings.ToLower(op)
		}
	}

	modelName := strings.ToLower(info.UpstreamModelName)
	if modelName == "" {
		modelName = strings.ToLower(req.Model)
	}

	if operation == "" {
		if modelName == "music" {
			operation = "music"
		} else if modelName == "rescale" {
			operation = "rescale"
		} else if IsImageModel(modelName) {
			if job.Source != "" {
				operation = "image_edit"
			} else {
				operation = "image"
			}
		} else {
			operation = "create"
		}
	}
	job.Operation = operation

	// 3. Determine Model (Music and Rescale must have NO model!)
	if operation == "music" || operation == "rescale" {
		job.Model = ""
	} else if modelName != "" && modelName != "music" && modelName != "rescale" {
		job.Model = modelName
	} else {
		if strings.HasPrefix(operation, "image") {
			job.Model = "gpt-image-2.6-sunburst"
		} else {
			job.Model = "seedance-2.0-fast"
		}
	}

	// 4. Duration
	duration := req.Duration
	if duration <= 0 && req.Seconds != "" {
		if s, err := strconv.Atoi(req.Seconds); err == nil {
			duration = s
		}
	}
	if req.Metadata != nil {
		if d, ok := req.Metadata["duration"].(float64); ok && d > 0 {
			duration = int(d)
		} else if d, ok := req.Metadata["duration"].(int); ok && d > 0 {
			duration = d
		}
	}
	if duration > 0 {
		if duration < 2 {
			duration = 2
		} else if duration > 30 {
			duration = 30
		}
		job.Duration = duration
	}

	// 5. Aspect and Resolution
	aspect, resolution := parseAspectAndResolution(req.Size)
	if req.Metadata != nil {
		if asp, ok := req.Metadata["aspect"].(string); ok && asp != "" {
			aspect = asp
		}
		if res, ok := req.Metadata["resolution"].(string); ok && res != "" {
			resolution = res
		}
		if style, ok := req.Metadata["style"].(string); ok && style != "" {
			job.Style = style
		}
		if fit, ok := req.Metadata["fit"].(string); ok && fit != "" {
			job.Fit = fit
		}
		if engine, ok := req.Metadata["engine"].(string); ok && engine != "" {
			job.Engine = engine
		}
		if audio, ok := req.Metadata["audio"].(bool); ok {
			job.Audio = &audio
		}
	}
	if aspect != "" {
		job.Aspect = aspect
	}
	if resolution != "" {
		job.Resolution = resolution
	}

	return job, nil
}

func parseAspectAndResolution(size string) (aspect string, resolution string) {
	size = strings.TrimSpace(strings.ToLower(size))
	switch size {
	case "1920x1080":
		return "16:9", "1080p"
	case "1280x720":
		return "16:9", "720p"
	case "1080x1920":
		return "9:16", "1080p"
	case "720x1280":
		return "9:16", "720p"
	case "1024x1024":
		return "1:1", "1080p"
	case "720x720":
		return "1:1", "720p"
	case "2520x1080":
		return "21:9", "1080p"
	case "1680x720":
		return "21:9", "720p"
	case "1440x1080":
		return "4:3", "1080p"
	case "960x720":
		return "4:3", "720p"
	case "1080x1440":
		return "3:4", "1080p"
	case "720x960":
		return "3:4", "720p"
	case "1620x1080":
		return "3:2", "1080p"
	case "1080x720":
		return "3:2", "720p"
	case "1080x1620":
		return "2:3", "1080p"
	case "720x1080":
		return "2:3", "720p"
	case "1080x1350":
		return "4:5", "1080p"
	case "720x900":
		return "4:5", "720p"
	case "1350x1080":
		return "5:4", "1080p"
	case "900x720":
		return "5:4", "720p"
	case "16:9", "9:16", "1:1", "4:3", "3:4", "3:2", "2:3", "21:9", "4:5", "5:4", "auto":
		return size, "720p"
	case "720p", "1080p", "4k":
		return "16:9", size
	default:
		return "", ""
	}
}

func (a *TaskAdaptor) resolveSource(c *gin.Context, rawSource string) (string, error) {
	rawSource = strings.TrimSpace(rawSource)
	if strings.HasPrefix(rawSource, "upload_") {
		return rawSource, nil
	}

	// 1. Check if multipart form contains uploaded file
	if c != nil && c.Request != nil && strings.HasPrefix(c.GetHeader("Content-Type"), "multipart/form-data") {
		if mf, err := c.MultipartForm(); err == nil && mf != nil {
			for _, key := range []string{"input_reference", "image", "source", "file"} {
				if files, ok := mf.File[key]; ok && len(files) > 0 {
					fh := files[0]
					f, err := fh.Open()
					if err == nil {
						defer f.Close()
						data, readErr := io.ReadAll(f)
						if readErr == nil && len(data) > 0 {
							contentType := fh.Header.Get("Content-Type")
							if contentType == "" {
								contentType = http.DetectContentType(data)
							}
							return a.uploadBinary(fh.Filename, contentType, data)
						}
					}
				}
			}
		}
	}

	if rawSource == "" {
		return "", nil
	}

	// 2. Base64 Data URI
	if strings.HasPrefix(rawSource, "data:") {
		idx := strings.Index(rawSource, ",")
		if idx == -1 {
			return "", errors.New("invalid data URI")
		}
		header := rawSource[:idx]
		payload := rawSource[idx+1:]
		mimeType := "image/png"
		ext := ".png"
		if strings.HasPrefix(header, "data:") && strings.Contains(header, ";") {
			mimeType = strings.TrimPrefix(strings.Split(header, ";")[0], "data:")
			if strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg") {
				ext = ".jpg"
			} else if strings.Contains(mimeType, "mp4") {
				ext = ".mp4"
			} else if strings.Contains(mimeType, "mp3") {
				ext = ".mp3"
			} else if strings.Contains(mimeType, "webp") {
				ext = ".webp"
			}
		}
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 source: %w", err)
		}
		return a.uploadBinary("source"+ext, mimeType, data)
	}

	// 3. HTTP / HTTPS URL
	if strings.HasPrefix(rawSource, "http://") || strings.HasPrefix(rawSource, "https://") {
		client := service.GetSSRFProtectedHTTPClient()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawSource, nil)
		if err != nil {
			return "", err
		}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("failed to fetch remote source URL: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("failed to download remote source URL, status: %d", resp.StatusCode)
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read remote source data: %w", err)
		}
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		name := filepath.Base(rawSource)
		if name == "" || name == "." || name == "/" {
			name = "source.png"
		}
		return a.uploadBinary(name, contentType, data)
	}

	return rawSource, nil
}

func (a *TaskAdaptor) uploadBinary(filename, contentType string, data []byte) (string, error) {
	client, err := service.GetHttpClientWithProxy(a.proxy)
	if err != nil {
		return "", err
	}
	if filename == "" {
		filename = "source.png"
	}
	uploadURL := fmt.Sprintf("%s/v1/uploads?name=%s", a.baseURL, url.QueryEscape(filename))
	req, err := http.NewRequest(http.MethodPost, uploadURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("X-API-Key", a.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	} else {
		req.Header.Set("Content-Type", "application/octet-stream")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to upload binary to Aepi: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read upload response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("aepi upload returned %d: %s", resp.StatusCode, string(respBody))
	}

	var upRes AepiUploadResponse
	if err := json.Unmarshal(respBody, &upRes); err != nil {
		return "", fmt.Errorf("failed to parse aepi upload response: %w", err)
	}
	if upRes.Id == "" {
		return "", fmt.Errorf("aepi upload returned empty id: %s", string(respBody))
	}
	return upRes.Id, nil
}
