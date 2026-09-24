package antigravity

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

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
	AccessToken string `json:"access_token"`
	Token       string `json:"token"`
	ProjectID   string `json:"project_id"`
	Project     string `json:"project"`
}

func parseAntigravityKey(key string) (token string, project string) {
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
			if t != "" {
				return t, p
			}
		}
	}
	return key, ""
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
		baseUrl = "https://cloudcode-pa.googleapis.com"
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
	token, _ := parseAntigravityKey(info.ApiKey)
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
	_, project := parseAntigravityKey(info.ApiKey)
	modelName := info.UpstreamModelName
	if modelName == "" {
		modelName = info.OriginModelName
	}
	// normalize model name (e.g., strip vendor prefix if needed)
	if strings.Contains(modelName, "/") {
		parts := strings.Split(modelName, "/")
		modelName = parts[len(parts)-1]
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
	return channel.DoApiRequest(a, c, info, requestBody)
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
