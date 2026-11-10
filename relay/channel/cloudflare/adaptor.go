package cloudflare

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	//TODO implement me
	panic("implement me")
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	switch info.RelayMode {
	case constant.RelayModeChatCompletions:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/v1/chat/completions", info.ChannelBaseUrl, info.ApiVersion), nil
	case constant.RelayModeEmbeddings:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/v1/embeddings", info.ChannelBaseUrl, info.ApiVersion), nil
	case constant.RelayModeResponses:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/v1/responses", info.ChannelBaseUrl, info.ApiVersion), nil
	default:
		return fmt.Sprintf("%s/client/v4/accounts/%s/ai/run/%s", info.ChannelBaseUrl, info.ApiVersion, info.UpstreamModelName), nil
	}
}

func usesCfJsonAudioPayload(modelName string) bool {
	return strings.Contains(strings.ToLower(modelName), "whisper-large-v3")
}

func isCfDeepgramModel(modelName string) bool {
	lower := strings.ToLower(modelName)
	return strings.Contains(lower, "deepgram") || strings.Contains(lower, "nova-2") || strings.Contains(lower, "nova-3")
}

func detectAudioContentType(c *gin.Context) string {
	if c != nil && c.Request != nil {
		reqCT := strings.TrimSpace(c.Request.Header.Get("Content-Type"))
		if strings.HasPrefix(strings.ToLower(reqCT), "audio/") {
			return reqCT
		}
		if strings.HasPrefix(strings.ToLower(reqCT), "multipart/form-data") {
			if _, fh, err := c.Request.FormFile("file"); err == nil && fh != nil {
				partCT := strings.TrimSpace(fh.Header.Get("Content-Type"))
				if strings.HasPrefix(strings.ToLower(partCT), "audio/") {
					return partCT
				}
				switch strings.ToLower(filepath.Ext(fh.Filename)) {
				case ".mp3":
					return "audio/mpeg"
				case ".wav":
					return "audio/wav"
				case ".ogg":
					return "audio/ogg"
				case ".flac":
					return "audio/flac"
				case ".webm":
					return "audio/webm"
				case ".m4a", ".mp4":
					return "audio/mp4"
				case ".aac":
					return "audio/aac"
				case ".opus":
					return "audio/opus"
				}
			}
		}
	}
	return "audio/mpeg"
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", fmt.Sprintf("Bearer %s", info.ApiKey))
	if info.RelayMode == constant.RelayModeAudioTranscription || info.RelayMode == constant.RelayModeAudioTranslation {
		if usesCfJsonAudioPayload(info.UpstreamModelName) {
			req.Set("Content-Type", "application/json")
		} else if isCfDeepgramModel(info.UpstreamModelName) {
			req.Set("Content-Type", detectAudioContentType(c))
		} else {
			req.Set("Content-Type", "application/octet-stream")
		}
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	switch info.RelayMode {
	case constant.RelayModeCompletions:
		return convertCf2CompletionsRequest(*request), nil
	default:
		return request, nil
	}
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	var bodyBytes []byte
	if c.Request != nil && c.Request.MultipartForm != nil {
		file, _, err := c.Request.FormFile("file")
		if err == nil && file != nil {
			defer file.Close()
			bodyBytes, _ = io.ReadAll(file)
		}
	} else if c.Request != nil {
		file, _, err := c.Request.FormFile("file")
		if err == nil && file != nil {
			defer file.Close()
			bodyBytes, _ = io.ReadAll(file)
		} else if c.Request.Body != nil {
			bodyBytes, _ = io.ReadAll(c.Request.Body)
		}
	}
	if len(bodyBytes) == 0 {
		return nil, errors.New("file is required")
	}

	if !usesCfJsonAudioPayload(info.UpstreamModelName) {
		return bytes.NewBuffer(bodyBytes), nil
	}

	task := "transcribe"
	if info.RelayMode == constant.RelayModeAudioTranslation {
		task = "translate"
	}
	cfPayload := map[string]any{
		"audio": base64.StdEncoding.EncodeToString(bodyBytes),
		"task":  task,
	}
	if len(request.Language) > 0 {
		var langStr string
		if err := json.Unmarshal(request.Language, &langStr); err == nil && langStr != "" {
			cfPayload["language"] = langStr
		}
	}
	jsonBytes, err := json.Marshal(cfPayload)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(jsonBytes), nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case constant.RelayModeEmbeddings:
		fallthrough
	case constant.RelayModeChatCompletions:
		if info.IsStream {
			err, usage = cfStreamHandler(c, info, resp)
		} else {
			err, usage = cfHandler(c, info, resp)
		}
	case constant.RelayModeResponses:
		if info.IsStream {
			usage, err = openai.OaiResponsesStreamHandler(c, info, resp)
		} else {
			usage, err = openai.OaiResponsesHandler(c, info, resp)
		}
	case constant.RelayModeAudioTranslation:
		fallthrough
	case constant.RelayModeAudioTranscription:
		err, usage = cfSTTHandler(c, info, resp)
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
