package deepseekweb

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

const ChannelName = "deepseekweb"

// DefaultBaseURL is the public chat host; a channel may override it to reach a
// mirror deployment.
const DefaultBaseURL = defaultBaseURL

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

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return resolveBaseURL(info.ChannelBaseUrl) + completionPath, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, header)
	header.Set("User-Agent", browserUserAgent)
	header.Set("Accept", "text/event-stream")
	return nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	ctx := c.Request.Context()

	client, err := newWebClient(ctx, info.ApiKey, info.ChannelBaseUrl)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, err
	}
	var openAIRequest dto.GeneralOpenAIRequest
	if len(bodyBytes) > 0 {
		if err := common.Unmarshal(bodyBytes, &openAIRequest); err != nil {
			return nil, fmt.Errorf("deepseek web: cannot decode relay request: %w", err)
		}
	}

	prompt := buildPrompt(openAIRequest.Messages)
	if prompt == "" {
		prompt = "Hello"
	}

	fileIDs, err := uploadAttachments(ctx, client, collectAttachments(openAIRequest.Messages))
	if err != nil {
		return nil, err
	}

	sessionID, err := client.createSession(ctx)
	if err != nil {
		return nil, err
	}

	return client.completion(ctx, sessionID, prompt, fileIDs, isThinkingModel(info.UpstreamModelName))
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	defer resp.Body.Close()

	// The relay promotes info.IsStream to true as soon as the upstream answers
	// with text/event-stream, and DeepSeek Web always does. The caller's own
	// intent survives under ContextKeyIsStream, so a non-streaming caller still
	// receives one JSON body instead of an event stream.
	isStream := common.GetContextKeyBool(c, constant.ContextKeyIsStream)
	if isStream {
		helper.SetEventStreamHeaders(c)
	}

	responseID := fmt.Sprintf("chatcmpl-%s", common.GetUUID())
	createdTime := time.Now().Unix()

	state, streamErr := readCompletionStream(resp.Body, func(delta streamDelta) {
		if !isStream {
			return
		}
		a.writeStreamDelta(c, info, responseID, createdTime, delta)
	})
	if streamErr != nil {
		return nil, types.NewError(fmt.Errorf("deepseek web: cannot read completion stream: %w", streamErr), types.ErrorCodeBadResponse)
	}

	totalContent := state.content()
	totalReasoning := state.reasoning()
	if strings.TrimSpace(totalContent) == "" && strings.TrimSpace(totalReasoning) == "" {
		return nil, types.NewError(errors.New("deepseek web: boş yanıt alındı (empty response from upstream)"), types.ErrorCodeBadResponse)
	}

	tokenUsage := completionUsage(info, state, totalContent, totalReasoning)
	if isStream {
		c.Writer.Write([]byte("data: [DONE]\n\n"))
		c.Writer.Flush()
	} else {
		var reasoning *string
		if totalReasoning != "" {
			reasoning = &totalReasoning
		}
		content := totalContent
		if strings.TrimSpace(content) == "" {
			content = totalReasoning
			reasoning = nil
		}
		c.JSON(http.StatusOK, dto.OpenAITextResponse{
			Id:      responseID,
			Object:  "chat.completion",
			Created: createdTime,
			Model:   info.OriginModelName,
			Choices: []dto.OpenAITextResponseChoice{
				{
					Index: 0,
					Message: dto.Message{
						Role:             "assistant",
						Content:          content,
						ReasoningContent: reasoning,
					},
					FinishReason: "stop",
				},
			},
			Usage: *tokenUsage,
		})
	}

	return tokenUsage, nil
}

func (a *Adaptor) writeStreamDelta(c *gin.Context, info *relaycommon.RelayInfo, responseID string, createdTime int64, delta streamDelta) {
	choiceDelta := dto.ChatCompletionsStreamResponseChoiceDelta{}
	if delta.content != "" {
		content := delta.content
		choiceDelta.Content = &content
	}
	if delta.reasoning != "" {
		reasoning := delta.reasoning
		choiceDelta.ReasoningContent = &reasoning
	}
	if choiceDelta.Content == nil && choiceDelta.ReasoningContent == nil {
		return
	}

	chunk, marshalErr := common.Marshal(dto.ChatCompletionsStreamResponse{
		Id:      responseID,
		Object:  "chat.completion.chunk",
		Created: createdTime,
		Model:   info.OriginModelName,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: choiceDelta},
		},
	})
	if marshalErr != nil {
		return
	}
	c.Writer.Write([]byte("data: " + string(chunk) + "\n\n"))
	c.Writer.Flush()
}

// completionUsage reports the token usage of one relayed turn. DeepSeek Web
// publishes the true total for the turn in accumulated_token_usage, but the
// field is optional, so the locally counted text is used as the floor to keep a
// successful request from ever being billed as zero. The relay callers assert
// this value to *dto.Usage, so a pointer is returned.
func completionUsage(info *relaycommon.RelayInfo, state *completionStreamState, content, reasoning string) *dto.Usage {
	completionTokens := utf8.RuneCountInString(content) + utf8.RuneCountInString(reasoning)
	promptTokens := info.GetEstimatePromptTokens()
	if state.usage > promptTokens+completionTokens {
		promptTokens = state.usage - completionTokens
	}
	if promptTokens < 1 {
		promptTokens = 1
	}
	if completionTokens < 1 {
		completionTokens = 1
	}
	return &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      promptTokens + completionTokens,
	}
}

func resolveBaseURL(channelBaseURL string) string {
	baseURL := strings.TrimRight(strings.TrimSpace(channelBaseURL), "/")
	if baseURL == "" {
		return DefaultBaseURL
	}
	return baseURL
}
