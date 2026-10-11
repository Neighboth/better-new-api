package deepseekweb

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

const (
	ChannelName = "deepseekweb"
	DefaultBaseURL = "https://chat.deepseek.com"
)

var ModelList = []string{
	"deepseek-chat",
	"deepseek-reasoner",
}

var (
	tokenCacheMu sync.RWMutex
	tokenCache   = make(map[string]cachedToken)
)

type cachedToken struct {
	token     string
	expiresAt time.Time
}

type Adaptor struct{}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("deepseek web: claude endpoint not supported directly")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("deepseek web: gemini endpoint not supported directly")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("deepseek web: audio endpoint not supported")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	return nil, errors.New("deepseek web: image endpoint not supported")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("deepseek web: rerank endpoint not supported")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("deepseek web: embedding endpoint not supported")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("deepseek web: responses endpoint not supported")
}

func resolveToken(ctx context.Context, apiKey, baseURL string) (string, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", errors.New("deepseek web credentials missing: enter username and password or user token")
	}
	if strings.HasPrefix(apiKey, "{") {
		var tokenObj map[string]any
		if err := json.Unmarshal([]byte(apiKey), &tokenObj); err == nil {
			if val, ok := tokenObj["value"].(string); ok && val != "" {
				return val, nil
			}
			if val, ok := tokenObj["token"].(string); ok && val != "" {
				return val, nil
			}
			if val, ok := tokenObj["userToken"].(string); ok && val != "" {
				return val, nil
			}
		}
	}
	apiKey = strings.Trim(apiKey, "\"")
	if !strings.Contains(apiKey, ":") {
		// Already a user token
		return apiKey, nil
	}
	parts := strings.SplitN(apiKey, ":", 2)
	email, password := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])

	tokenCacheMu.RLock()
	cached, ok := tokenCache[email]
	tokenCacheMu.RUnlock()
	if ok && time.Now().Before(cached.expiresAt) {
		return cached.token, nil
	}

	loginURL := baseURL + "/api/v0/users/login"
	payload, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
		"mobile":   email,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, loginURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://chat.deepseek.com")
	req.Header.Set("Referer", "https://chat.deepseek.com/")
	req.Header.Set("x-client-platform", "web")
	req.Header.Set("x-app-version", "20241129.1")
	req.Header.Set("x-client-locale", "zh_CN")
	req.Header.Set("x-client-version", "1.0.0-always")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("deepseek login failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusAccepted { // 202
		return "", errors.New("deepseek web: sunucu CAPTCHA/güvenlik doğrulaması istedi (HTTP 202). Tarayıcıdan chat.deepseek.com'a giriş yapıp F12 -> Application -> LocalStorage -> userToken kopyalayarak doğrudan Key alanına yapıştırınız.")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("deepseek login failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var loginResp struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return "", fmt.Errorf("failed to decode login response: %w", err)
	}
	if loginResp.Data.Token == "" {
		if loginResp.Msg != "" {
			return "", fmt.Errorf("deepseek login failed: %s", loginResp.Msg)
		}
		return "", errors.New("deepseek login returned empty token")
	}

	tokenCacheMu.Lock()
	tokenCache[email] = cachedToken{
		token:     loginResp.Data.Token,
		expiresAt: time.Now().Add(12 * time.Hour),
	}
	tokenCacheMu.Unlock()

	return loginResp.Data.Token, nil
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	baseUrl := strings.TrimRight(info.ChannelBaseUrl, "/")
	if baseUrl == "" {
		baseUrl = DefaultBaseURL
	}
	return baseUrl + "/api/v0/chat/completion", nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, header)
	header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	header.Set("Accept", "text/event-stream")
	return nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	baseUrl := strings.TrimRight(info.ChannelBaseUrl, "/")
	if baseUrl == "" {
		baseUrl = DefaultBaseURL
	}
	token, err := resolveToken(c.Request.Context(), info.ApiKey, baseUrl)
	if err != nil {
		return nil, err
	}

	// Create session first
	createSessionURL := baseUrl + "/api/v0/chat_session/create"
	createReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, createSessionURL, strings.NewReader(`{}`))
	if err != nil {
		return nil, fmt.Errorf("failed to create session request: %w", err)
	}
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	createReq.Header.Set("Origin", "https://chat.deepseek.com")
	createReq.Header.Set("Referer", "https://chat.deepseek.com/")
	createReq.Header.Set("x-client-platform", "web")
	createReq.Header.Set("x-app-version", "20241129.1")
	createReq.Header.Set("x-client-version", "1.0.0-always")
	client := &http.Client{Timeout: 15 * time.Second}
	sResp, sErr := client.Do(createReq)
	if sErr != nil {
		return nil, fmt.Errorf("deepseek session creation request failed: %w", sErr)
	}
	defer sResp.Body.Close()
	if sResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(sResp.Body)
		return nil, fmt.Errorf("deepseek session creation failed (HTTP %d): %s", sResp.StatusCode, string(body))
	}
	var sessResp struct {
		Data struct {
			BizData struct {
				ID string `json:"id"`
			} `json:"biz_data"`
		} `json:"data"`
	}
	_ = json.NewDecoder(sResp.Body).Decode(&sessResp)
	sessID := sessResp.Data.BizData.ID
	if sessID == "" {
		return nil, errors.New("deepseek web: session id not returned by server")
	}

	// Read input request
	var oaiReq dto.GeneralOpenAIRequest
	bodyBytes, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(bodyBytes, &oaiReq)

	prompt := ""
	for _, m := range oaiReq.Messages {
		if contentStr, ok := m.Content.(string); ok {
			prompt += m.Role + ": " + contentStr + "\n"
		}
	}
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = "Hello"
	}

	thinkingEnabled := strings.Contains(strings.ToLower(info.UpstreamModelName), "reasoner") || strings.Contains(strings.ToLower(info.UpstreamModelName), "r1")

	// Get PoW challenge and solve it
	var powHeaderVal string
	powReqURL := baseUrl + "/api/v0/chat/create_pow_challenge"
	powReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, powReqURL, strings.NewReader(`{"target_path":"/api/v0/chat/completion"}`))
	if err != nil {
		return nil, fmt.Errorf("failed to create pow challenge request: %w", err)
	}
	powReq.Header.Set("Authorization", "Bearer "+token)
	powReq.Header.Set("Content-Type", "application/json")
	powReq.Header.Set("Origin", "https://chat.deepseek.com")
	powReq.Header.Set("Referer", "https://chat.deepseek.com/")
	powReq.Header.Set("x-client-platform", "web")
	powReq.Header.Set("x-app-version", "20241129.1")
	powReq.Header.Set("x-client-version", "1.0.0-always")
	powReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")

	powClient := &http.Client{Timeout: 15 * time.Second}
	powResp, powErr := powClient.Do(powReq)
	if powErr != nil {
		return nil, fmt.Errorf("deepseek pow challenge request failed: %w", powErr)
	}
	defer powResp.Body.Close()
	if powResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(powResp.Body)
		return nil, fmt.Errorf("deepseek pow challenge failed (HTTP %d): %s", powResp.StatusCode, string(body))
	}

	var powData struct {
		Data struct {
			BizData struct {
				Challenge PowChallenge `json:"challenge"`
			} `json:"biz_data"`
		} `json:"data"`
	}
	if decErr := json.NewDecoder(powResp.Body).Decode(&powData); decErr != nil {
		return nil, fmt.Errorf("failed to decode pow challenge: %w", decErr)
	}
	if powData.Data.BizData.Challenge.Challenge == "" {
		return nil, errors.New("deepseek web: empty pow challenge returned")
	}

	solvedStr, sErr := SolvePoWChallenge(&powData.Data.BizData.Challenge)
	if sErr != nil {
		return nil, fmt.Errorf("failed to solve pow challenge: %w", sErr)
	}
	powHeaderVal = solvedStr

	webReqPayload := map[string]any{
		"chat_session_id":   sessID,
		"parent_message_id": nil,
		"prompt":            prompt,
		"ref_file_ids":      []any{},
		"thinking_enabled":  thinkingEnabled,
	}
	payloadBytes, _ := json.Marshal(webReqPayload)

	fullURL, _ := a.GetRequestURL(info)
	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, fullURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Origin", "https://chat.deepseek.com")
	httpReq.Header.Set("Referer", "https://chat.deepseek.com/")
	httpReq.Header.Set("x-client-platform", "web")
	httpReq.Header.Set("x-app-version", "20241129.1")
	httpReq.Header.Set("x-client-version", "1.0.0-always")
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	if powHeaderVal != "" {
		httpReq.Header.Set("x-ds-pow-response", powHeaderVal)
	}

	client = &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("deepseek web upstream error (%d): %s", resp.StatusCode, string(body)), types.ErrorCodeBadResponseStatusCode, resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	isStream := info.IsStream
	if isStream {
		helper.SetEventStreamHeaders(c)
	}

	var totalResponse strings.Builder
	var reasoningResponse strings.Builder
	responseId := fmt.Sprintf("chatcmpl-%s", common.GetUUID())
	createdTime := time.Now().Unix()

	var activeFrag string
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				dataContent := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if dataContent == "[DONE]" {
					break
				}

				var contentChunk string
				var reasoningChunk string

				// 1. Try DeepSeek Web patch format:
				// - New fragment appended: {"p":"response/fragments","o":"APPEND","v":[{"type":"RESPONSE","content":"Hi"}]}
				// - Fragment content updated: {"p":"response/fragments/-1/content","v":"!"}
				var patch struct {
					P string          `json:"p"`
					O string          `json:"o"`
					V json.RawMessage `json:"v"`
				}
				if json.Unmarshal([]byte(dataContent), &patch) == nil && patch.P != "" {
					if patch.P == "response/status" {
						var statusStr string
						if json.Unmarshal(patch.V, &statusStr) == nil && statusStr == "FINISHED" {
							break
						}
					} else if patch.P == "response/fragments" && patch.O == "APPEND" {
						var newFrags []struct {
							Type    string `json:"type"`
							Content string `json:"content"`
						}
						if json.Unmarshal(patch.V, &newFrags) == nil && len(newFrags) > 0 {
							for _, nf := range newFrags {
								fragType := strings.ToUpper(nf.Type)
								activeFrag = fragType
								if fragType == "THINK" || fragType == "THINKING" {
									reasoningChunk += nf.Content
								} else {
									contentChunk += nf.Content
								}
							}
						}
					} else if patch.P == "response/fragments/-1/content" || strings.HasSuffix(patch.P, "/content") {
						var vStr string
						if json.Unmarshal(patch.V, &vStr) == nil && vStr != "" {
							if activeFrag == "THINK" || activeFrag == "THINKING" {
								reasoningChunk = vStr
							} else {
								contentChunk = vStr
							}
						}
					} else if patch.P == "response/thinking_content" || strings.HasSuffix(patch.P, "/thinking_content") {
						var vStr string
						if json.Unmarshal(patch.V, &vStr) == nil && vStr != "" {
							reasoningChunk = vStr
						}
					}
				} else {
					// 1b. Try initial response object: {"v":{"response":{"fragments":[{"type":"THINK","content":"..."}]}}}
					var initV struct {
						V struct {
							Response struct {
								Fragments []struct {
									Type    string `json:"type"`
									Content string `json:"content"`
								} `json:"fragments"`
							} `json:"response"`
						} `json:"v"`
					}
					if json.Unmarshal([]byte(dataContent), &initV) == nil && len(initV.V.Response.Fragments) > 0 {
						for _, f := range initV.V.Response.Fragments {
							fragType := strings.ToUpper(f.Type)
							activeFrag = fragType
							if fragType == "THINK" || fragType == "THINKING" {
								reasoningChunk += f.Content
							} else {
								contentChunk += f.Content
							}
						}
					} else {
						// 2. Try simple {"v": "..."} token (very common in DeepSeek SSE streams)
						var simpleV struct {
							V string `json:"v"`
						}
						if json.Unmarshal([]byte(dataContent), &simpleV) == nil && simpleV.V != "" {
							if activeFrag == "THINK" || activeFrag == "THINKING" {
								reasoningChunk = simpleV.V
							} else {
								contentChunk = simpleV.V
							}
						} else {
							// 3. Fallback: OpenAI chunk format
							var oaiChunk struct {
								Choices []struct {
									Delta struct {
										Content          string `json:"content"`
										ReasoningContent string `json:"reasoning_content"`
									} `json:"delta"`
								} `json:"choices"`
							}
							if json.Unmarshal([]byte(dataContent), &oaiChunk) == nil && len(oaiChunk.Choices) > 0 {
								contentChunk = oaiChunk.Choices[0].Delta.Content
								reasoningChunk = oaiChunk.Choices[0].Delta.ReasoningContent
							}
						}
					}
				}

				if contentChunk != "" || reasoningChunk != "" {
					if contentChunk != "" {
						totalResponse.WriteString(contentChunk)
					}
					if reasoningChunk != "" {
						reasoningResponse.WriteString(reasoningChunk)
					}

					if isStream {
						deltaChoice := dto.ChatCompletionsStreamResponseChoiceDelta{}
						if contentChunk != "" {
							deltaChoice.Content = &contentChunk
						}
						if reasoningChunk != "" {
							deltaChoice.ReasoningContent = &reasoningChunk
						}
						chunkResp := dto.ChatCompletionsStreamResponse{
							Id:      responseId,
							Object:  "chat.completion.chunk",
							Created: createdTime,
							Model:   info.OriginModelName,
							Choices: []dto.ChatCompletionsStreamResponseChoice{
								{
									Index: 0,
									Delta: deltaChoice,
								},
							},
						}
						chunkBytes, _ := json.Marshal(chunkResp)
						c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", string(chunkBytes))))
						c.Writer.Flush()
					}
				}
			}
		}
		if readErr != nil {
			break
		}
	}

	if totalResponse.Len() == 0 && reasoningResponse.Len() == 0 {
		return nil, types.NewError(errors.New("deepseek web: boş yanıt alındı (empty response from upstream)"), types.ErrorCodeBadResponse)
	}

	if isStream {
		c.Writer.Write([]byte("data: [DONE]\n\n"))
		c.Writer.Flush()
	} else {
		finalContent := totalResponse.String()
		if finalContent == "" && reasoningResponse.Len() > 0 {
			finalContent = reasoningResponse.String()
		}
		var finalReasoning *string
		if reasoningResponse.Len() > 0 {
			rStr := reasoningResponse.String()
			finalReasoning = &rStr
		}
		fullResp := dto.OpenAITextResponse{
			Id:      responseId,
			Object:  "chat.completion",
			Created: createdTime,
			Model:   info.OriginModelName,
			Choices: []dto.OpenAITextResponseChoice{
				{
					Index: 0,
					Message: dto.Message{
						Role:             "assistant",
						Content:          finalContent,
						ReasoningContent: finalReasoning,
					},
					FinishReason: "stop",
				},
			},
			Usage: dto.Usage{
				PromptTokens:     10,
				CompletionTokens: len([]rune(totalResponse.String())),
				TotalTokens:      10 + len([]rune(totalResponse.String())),
			},
		}
		c.JSON(http.StatusOK, fullResp)
	}

	tokenUsage := &dto.Usage{
		PromptTokens:     10,
		CompletionTokens: len([]rune(totalResponse.String())),
		TotalTokens:      10 + len([]rune(totalResponse.String())),
	}
	return tokenUsage, nil
}
