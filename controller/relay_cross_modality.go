package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

// tryCrossModalityFallback attempts other modality endpoints (image, tts, stt, video)
// when a chat/playground request receives a 400, in case the user requested an image or audio model.
func tryCrossModalityFallback(c *gin.Context, relayInfo *relaycommon.RelayInfo) bool {
	if c.Writer != nil && c.Writer.Written() {
		return false
	}
	prompt := ""
	if req, ok := relayInfo.Request.(*dto.GeneralOpenAIRequest); ok && len(req.Messages) > 0 {
		for i := len(req.Messages) - 1; i >= 0; i-- {
			if content := req.Messages[i].StringContent(); content != "" {
				prompt = content
				break
			}
		}
	}
	if prompt == "" {
		return false
	}

	// 1. Try Image Generation
	if tryImageCrossModality(c, relayInfo, prompt) {
		return true
	}

	// 2. Try Audio Speech (TTS)
	if tryTTSCrossModality(c, relayInfo, prompt) {
		return true
	}

	return false
}

func tryImageCrossModality(c *gin.Context, relayInfo *relaycommon.RelayInfo, prompt string) bool {
	w := httptest.NewRecorder()
	tempCtx, _ := gin.CreateTestContext(w)
	tempCtx.Request = c.Request.Clone(c.Request.Context())
	for k, v := range c.Keys {
		tempCtx.Set(k, v)
	}

	imgReq := &dto.ImageRequest{
		Model:  relayInfo.OriginModelName,
		Prompt: prompt,
		N:      common.GetPointer(uint(1)),
		Size:   "1024x1024",
	}

	tempInfo := *relayInfo
	tempInfo.RelayMode = relayconstant.RelayModeImagesGenerations
	tempInfo.RelayFormat = types.RelayFormatOpenAIImage
	tempInfo.Request = imgReq
	tempInfo.IsStream = false

	err := relay.ImageHelper(tempCtx, &tempInfo)
	if err != nil || w.Code != http.StatusOK {
		return false
	}

	var imgResp struct {
		Data []struct {
			Url     string `json:"url"`
			B64Json string `json:"b64_json"`
		} `json:"data"`
	}

	if err := common.Unmarshal(w.Body.Bytes(), &imgResp); err != nil || len(imgResp.Data) == 0 {
		return false
	}

	imgUrl := imgResp.Data[0].Url
	if imgUrl == "" && imgResp.Data[0].B64Json != "" {
		imgUrl = "data:image/png;base64," + imgResp.Data[0].B64Json
	}
	if imgUrl == "" {
		return false
	}

	replyText := fmt.Sprintf("![image](%s)", imgUrl)
	sendCrossModalityReply(c, relayInfo, replyText)
	logger.LogInfo(c, fmt.Sprintf("cross-modality fallback succeeded: %s served via image API", relayInfo.OriginModelName))
	return true
}

func tryTTSCrossModality(c *gin.Context, relayInfo *relaycommon.RelayInfo, prompt string) bool {
	w := httptest.NewRecorder()
	tempCtx, _ := gin.CreateTestContext(w)
	tempCtx.Request = c.Request.Clone(c.Request.Context())
	for k, v := range c.Keys {
		tempCtx.Set(k, v)
	}

	audioReq := &dto.AudioRequest{
		Model: relayInfo.OriginModelName,
		Input: prompt,
		Voice: "alloy",
	}

	tempInfo := *relayInfo
	tempInfo.RelayMode = relayconstant.RelayModeAudioSpeech
	tempInfo.Request = audioReq
	tempInfo.IsStream = false

	err := relay.AudioHelper(tempCtx, &tempInfo)
	if err != nil || w.Code != http.StatusOK {
		return false
	}

	replyText := fmt.Sprintf("[Audio generated successfully for model %s]", relayInfo.OriginModelName)
	sendCrossModalityReply(c, relayInfo, replyText)
	logger.LogInfo(c, fmt.Sprintf("cross-modality fallback succeeded: %s served via TTS API", relayInfo.OriginModelName))
	return true
}

func sendCrossModalityReply(c *gin.Context, relayInfo *relaycommon.RelayInfo, replyText string) {
	targetModel := relayInfo.RequestedModelName
	if targetModel == "" {
		targetModel = c.GetString("client_requested_model")
	}
	if targetModel == "" {
		targetModel = relayInfo.OriginModelName
	}

	if relayInfo.IsStream {
		helper.SetEventStreamHeaders(c)
		chunk := dto.ChatCompletionsStreamResponse{
			Id:      "chatcmpl-" + common.GetUUID(),
			Object:  "chat.completion.chunk",
			Created: common.GetTimestamp(),
			Model:   targetModel,
			Choices: []dto.ChatCompletionsStreamResponseChoice{
				{
					Index: 0,
					Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
						Role:    "assistant",
						Content: common.GetPointer(replyText),
					},
				},
			},
		}
		_ = helper.ObjectData(c, chunk)
		helper.Done(c)
		return
	}

	c.JSON(http.StatusOK, dto.OpenAITextResponse{
		Id:      "chatcmpl-" + common.GetUUID(),
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Model:   targetModel,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Index: 0,
				Message: dto.Message{
					Role:    "assistant",
					Content: replyText,
				},
				FinishReason: "stop",
			},
		},
	})
}
