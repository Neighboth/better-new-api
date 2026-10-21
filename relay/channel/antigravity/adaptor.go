package antigravity

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Adaptor struct{}

type V1InternalRequest struct {
	Project     string                 `json:"project,omitempty"`
	RequestID   string                 `json:"requestId"`
	UserAgent   string                 `json:"userAgent"`
	RequestType string                 `json:"requestType"`
	Model       string                 `json:"model"`
	Request     *dto.GeminiChatRequest `json:"request"`
}

type V1InternalResponse struct {
	ResponseID   string                  `json:"responseId,omitempty"`
	ModelVersion string                  `json:"modelVersion,omitempty"`
	Response     *dto.GeminiChatResponse `json:"response,omitempty"`
}

type AntigravityKey struct {
	AccessToken   string `json:"access_token"`
	RefreshToken  string `json:"refresh_token"`
	Token         string `json:"token"`
	ProjectID     string `json:"project_id"`
	Project       string `json:"project"`
	Expired       string `json:"expired"`
	Plan          string `json:"plan,omitempty"`           // "pro" (Google AI Pro), "plus" (Google AI Plus), "ultra" (Google AI Ultra), "free"
	ThinkingLevel string `json:"thinking_level,omitempty"` // "none", "low", "medium", "high" (or empty for default low on flash)
}

func refreshAntigravityKey(ak *AntigravityKey) (string, error) {
	if ak.RefreshToken == "" {
		return "", errors.New("antigravity: no refresh token available")
	}
	clientID := service.GetAntigravityOAuthClientID()
	clientSecret := service.GetAntigravityOAuthClientSecret()
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {clientID},
		"refresh_token": {ak.RefreshToken},
	}
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth2.googleapis.com/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var res struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK || res.AccessToken == "" {
		msg := res.Description
		if msg == "" {
			msg = res.Error
		}
		return "", fmt.Errorf("Google token refresh failed (HTTP %d): %s", resp.StatusCode, msg)
	}
	ak.AccessToken = res.AccessToken
	if res.ExpiresIn > 0 {
		ak.Expired = time.Now().Add(time.Duration(res.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	return res.AccessToken, nil
}

func fetchAntigravityInfo(token string, baseURL string) (project string, detectedPlan string) {
	if token == "" {
		return "", ""
	}
	if baseURL == "" {
		baseUrl := "https://daily-cloudcode-pa.googleapis.com"
		baseURL = baseUrl
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1internal:loadCodeAssist", strings.NewReader(`{"metadata":{"ideType":"ANTIGRAVITY"}}`))
	if err != nil {
		return "", ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "antigravity/1.10.4 windows/amd64")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	var res struct {
		CloudaicompanionProject string `json:"cloudaicompanionProject"`
		Project                 string `json:"project"`
		PaidTier                struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"paidTier"`
		CurrentTier struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"currentTier"`
		Data struct {
			Project string `json:"project"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err == nil {
		p := res.CloudaicompanionProject
		if p == "" {
			p = res.Project
		}
		if p == "" {
			p = res.Data.Project
		}

		plan := "pro"
		tierStr := strings.ToLower(res.PaidTier.Name + " " + res.PaidTier.ID + " " + res.CurrentTier.Name + " " + res.CurrentTier.ID)
		if strings.Contains(tierStr, "ultra") {
			plan = "ultra"
		} else if strings.Contains(tierStr, "plus") {
			plan = "plus"
		} else if strings.Contains(tierStr, "pro") {
			plan = "pro"
		} else if strings.Contains(tierStr, "free") {
			plan = "free"
		}

		return p, plan
	}
	return "", ""
}

func fetchAntigravityProject(token string, baseURL string) string {
	p, _ := fetchAntigravityInfo(token, baseURL)
	return p
}

func parseAntigravityKey(key string) (token string, project string, plan string, thinkingLevel string) {
	key = strings.TrimSpace(key)
	if strings.HasPrefix(key, "{") {
		var ak AntigravityKey
		if err := json.Unmarshal([]byte(key), &ak); err == nil {
			t := ak.AccessToken
			if t == "" {
				t = ak.Token
			}
			p := ak.ProjectID
			if p == "" {
				p = ak.Project
			}
			pl := ak.Plan
			tl := strings.ToLower(strings.TrimSpace(ak.ThinkingLevel))
			needsRefresh := t == ""
			if ak.Expired != "" {
				if expTime, errE := time.Parse(time.RFC3339, ak.Expired); errE == nil {
					if time.Now().After(expTime.Add(-5 * time.Minute)) {
						needsRefresh = true
					}
				}
			}
			if needsRefresh && ak.RefreshToken != "" {
				if newTok, errR := refreshAntigravityKey(&ak); errR == nil && newTok != "" {
					t = newTok
					if updatedBytes, errM := json.Marshal(ak); errM == nil {
						_ = model.DB.Model(&model.Channel{}).Where("`key` = ?", key).Update("key", string(updatedBytes)).Error
					}
				}
			}
			if (p == "" || pl == "") && t != "" {
				newP, newPl := fetchAntigravityInfo(t, "")
				changed := false
				if p == "" && newP != "" {
					p = newP
					ak.ProjectID = p
					changed = true
				}
				if pl == "" && newPl != "" {
					pl = newPl
					ak.Plan = pl
					changed = true
				}
				if changed {
					if updatedBytes, errM := json.Marshal(ak); errM == nil {
						_ = model.DB.Model(&model.Channel{}).Where("`key` = ?", key).Update("key", string(updatedBytes)).Error
					}
				}
			}
			if pl == "" {
				pl = "pro"
			}
			if t != "" {
				return t, p, pl, tl
			}
		}
	}
	return key, "", "pro", ""
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseUrl := info.ChannelBaseUrl
	if baseUrl == "" {
		baseUrl = "https://daily-cloudcode-pa.googleapis.com"
	}
	action := "generateContent"
	if info.IsStream {
		action = "streamGenerateContent?alt=sse"
	}
	path := fmt.Sprintf("/v1internal:%s", action)
	return relaycommon.GetFullRequestURL(baseUrl, path, info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	token, _, _, _ := parseAntigravityKey(info.ApiKey)
	if token == "" {
		return errors.New("antigravity channel: api key or access token is required")
	}
	req.Set("Authorization", "Bearer "+token)
	req.Set("Content-Type", "application/json")
	req.Set("User-Agent", "antigravity/1.10.4 windows/amd64")
	if info.IsStream {
		req.Set("Accept", "text/event-stream")
	} else {
		req.Set("Accept", "application/json")
	}
	return nil
}

func wrapV1InternalRequest(info *relaycommon.RelayInfo, geminiReq *dto.GeminiChatRequest) *V1InternalRequest {
	_, project, _, channelThinkingLevel := parseAntigravityKey(info.ApiKey)
	if project == "" {
		project = "aicode-consumers"
	}
	modelName := info.UpstreamModelName
	if modelName == "" {
		modelName = info.OriginModelName
	}
	// normalize model name (strip vendor prefix e.g. google/ or antigravity/)
	if strings.Contains(modelName, "/") {
		parts := strings.Split(modelName, "/")
		modelName = parts[len(parts)-1]
	}

	// Determine desired thinking level:
	// If think level parameter is provided in request or channel config, send it directly.
	// If none/off or empty, do not attach thinkingConfig. No model name checks!
	targetThinkingLevel := channelThinkingLevel
	if geminiReq != nil && geminiReq.GenerationConfig.ThinkingConfig != nil && geminiReq.GenerationConfig.ThinkingConfig.ThinkingLevel != "" {
		targetThinkingLevel = strings.ToLower(geminiReq.GenerationConfig.ThinkingConfig.ThinkingLevel)
	}

	if geminiReq != nil {
		if targetThinkingLevel == "" || targetThinkingLevel == "none" || targetThinkingLevel == "off" {
			geminiReq.GenerationConfig.ThinkingConfig = nil
		} else {
			geminiReq.GenerationConfig.ThinkingConfig = &dto.GeminiThinkingConfig{
				ThinkingLevel: targetThinkingLevel,
			}
		}
	}

	return &V1InternalRequest{
		Project:     project,
		RequestID:   "agent-" + uuid.New().String(),
		UserAgent:   "antigravity",
		RequestType: "agent",
		Model:       modelName,
		Request:     geminiReq,
	}
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	if len(request.Contents) > 0 && request.Contents[0].Role == "" {
		request.Contents[0].Role = "user"
	}
	return wrapV1InternalRequest(info, request), nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	result, err := relayconvert.ConvertRequest(c, info, types.RelayFormatGemini, request)
	if err != nil {
		return nil, err
	}
	geminiReq, ok := result.Value.(*dto.GeminiChatRequest)
	if !ok {
		return nil, fmt.Errorf("expected Gemini generateContent request, got %T", result.Value)
	}
	return wrapV1InternalRequest(info, geminiReq), nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	result, err := relayconvert.ConvertRequest(c, info, types.RelayFormatGemini, request)
	if err != nil {
		return nil, err
	}
	geminiReq, ok := result.Value.(*dto.GeminiChatRequest)
	if !ok {
		return nil, fmt.Errorf("expected Gemini generateContent request, got %T", result.Value)
	}
	return wrapV1InternalRequest(info, geminiReq), nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("antigravity channel: audio endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("antigravity channel: image generation endpoint not supported")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("antigravity channel: rerank endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("antigravity channel: embedding endpoint not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	result, err := relayconvert.ConvertRequest(c, info, types.RelayFormatGemini, &request)
	if err != nil {
		return nil, err
	}
	geminiRequest, ok := result.Value.(*dto.GeminiChatRequest)
	if !ok {
		return nil, fmt.Errorf("expected Gemini generateContent request, got %T", result.Value)
	}
	return a.ConvertGeminiRequest(c, info, geminiRequest)
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	var bodyBytes []byte
	if requestBody != nil {
		bodyBytes, _ = io.ReadAll(requestBody)
		requestBody = bytes.NewReader(bodyBytes)
	}
	resp, err := channel.DoApiRequest(a, c, info, requestBody)
	if resp != nil && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusTooManyRequests) {
		var ak AntigravityKey
		keyParsed := json.Unmarshal([]byte(info.ApiKey), &ak) == nil
		if keyParsed && ak.RefreshToken != "" && resp.StatusCode == http.StatusUnauthorized {
			if newTok, errR := refreshAntigravityKey(&ak); errR == nil && newTok != "" {
				_ = resp.Body.Close()
				if updatedBytes, errM := json.Marshal(ak); errM == nil {
					newKeyStr := string(updatedBytes)
					info.ApiKey = newKeyStr
					if info.ChannelId > 0 {
						_ = model.DB.Model(&model.Channel{}).Where("id = ?", info.ChannelId).Update("key", newKeyStr).Error
					}
				}
				var retryBody io.Reader
				if bodyBytes != nil {
					retryBody = bytes.NewReader(bodyBytes)
				}
				return channel.DoApiRequest(a, c, info, retryBody)
			}
		}
		// If 429 RESOURCE_EXHAUSTED or 401 and base URL was daily, try switching to prod or vice-versa
		if resp.StatusCode == http.StatusTooManyRequests {
			currentBase := info.ChannelBaseUrl
			if currentBase == "" || strings.Contains(currentBase, "daily") {
				_ = resp.Body.Close()
				info.ChannelBaseUrl = "https://cloudcode-pa.googleapis.com"
				var retryBody io.Reader
				if bodyBytes != nil {
					retryBody = bytes.NewReader(bodyBytes)
				}
				secondResp, secondErr := channel.DoApiRequest(a, c, info, retryBody)
				if secondResp != nil && secondResp.StatusCode == http.StatusOK {
					return secondResp, secondErr
				}
				if secondResp != nil {
					return secondResp, secondErr
				}
			}
		}
	}
	return resp, err
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewError(errors.New("invalid upstream response"), types.ErrorCodeBadResponse)
	}

	if info.IsStream {
		// Transform SSE stream: unwraps v1internal "response" envelope into native gemini SSE
		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			defer service.CloseResponseBodyGracefully(resp)

			scanner := bufio.NewScanner(resp.Body)
			buf := make([]byte, 64*1024)
			scanner.Buffer(buf, 10*1024*1024)

			for scanner.Scan() {
				line := scanner.Text()
				trimmed := strings.TrimSpace(line)
				if !strings.HasPrefix(trimmed, "data:") {
					pw.Write([]byte(line + "\n"))
					continue
				}
				data := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
				if data == "" || data == "[DONE]" {
					pw.Write([]byte(line + "\n"))
					continue
				}

				// Check if enclosed in v1internal envelope
				var v1Resp V1InternalResponse
				if jsonErr := json.Unmarshal([]byte(data), &v1Resp); jsonErr == nil && v1Resp.Response != nil {
					if innerBytes, mErr := json.Marshal(v1Resp.Response); mErr == nil {
						pw.Write([]byte("data: " + string(innerBytes) + "\n"))
						continue
					}
				}
				pw.Write([]byte(line + "\n"))
			}
		}()

		resp.Body = pr
		return gemini.GeminiChatStreamHandler(c, info, resp)
	}

	// Non-streaming: read body, unwrap if enclosed in v1internal envelope
	bodyBytes, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, types.NewError(readErr, types.ErrorCodeReadResponseBodyFailed)
	}

	var v1Resp V1InternalResponse
	if jsonErr := json.Unmarshal(bodyBytes, &v1Resp); jsonErr == nil && v1Resp.Response != nil {
		if innerBytes, mErr := json.Marshal(v1Resp.Response); mErr == nil {
			bodyBytes = innerBytes
		}
	}

	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return gemini.GeminiChatHandler(c, info, resp)
}
