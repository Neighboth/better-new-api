package aepi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	taskaepi "github.com/QuantumNous/new-api/relay/channel/task/aepi"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	apiKey  string
	baseURL string
	proxy   string
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	if a.baseURL == "" {
		a.baseURL = "https://videoapi.aerobox.eu.cc"
	}
	a.apiKey = info.ApiKey
	a.proxy = info.ChannelSetting.Proxy
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/v1/jobs", a.baseURL), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	header.Set("Authorization", "Bearer "+a.apiKey)
	header.Set("X-API-Key", a.apiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("aepi provider supports media generation: video, image, music, rescale")
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	aspect := "1:1"
	resolution := "1080p"
	switch request.Size {
	case "1024x1024":
		aspect = "1:1"
		resolution = "1080p"
	case "1792x1024", "1920x1080":
		aspect = "16:9"
		resolution = "1080p"
	case "1024x1792", "1080x1920":
		aspect = "9:16"
		resolution = "1080p"
	}

	modelName := request.Model
	if modelName == "" {
		modelName = "gpt-image-2.6-sunburst"
	}

	jobReq := taskaepi.AepiJobRequest{
		Operation:  "image",
		Model:      modelName,
		Prompt:     request.Prompt,
		Aspect:     aspect,
		Resolution: resolution,
	}

	return jobReq, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	client, err := service.GetHttpClientWithProxy(a.proxy)
	if err != nil {
		return nil, err
	}
	url, err := a.GetRequestURL(info)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, requestBody)
	if err != nil {
		return nil, err
	}
	if err := a.SetupRequestHeader(c, &req.Header, info); err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	defer resp.Body.Close()
	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewOpenAIError(readErr, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return nil, types.NewOpenAIError(fmt.Errorf("aepi error: %s", string(respBody)), types.ErrorCodeBadResponseStatusCode, resp.StatusCode)
	}

	var submitResp taskaepi.AepiJobResponse
	if unmarshalErr := json.Unmarshal(respBody, &submitResp); unmarshalErr != nil {
		return nil, types.NewOpenAIError(unmarshalErr, types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
	}

	jobID := submitResp.Id
	if jobID == "" {
		return nil, types.NewOpenAIError(errors.New("empty job id from aepi"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
	}

	// Poll until done (timeout 60 seconds)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	pollURL := fmt.Sprintf("%s/v1/jobs/%s", a.baseURL, jobID)
	client, clientErr := service.GetHttpClientWithProxy(a.proxy)
	if clientErr != nil {
		return nil, types.NewOpenAIError(clientErr, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, types.NewOpenAIError(errors.New("timeout waiting for image render to complete"), types.ErrorCodeDoRequestFailed, http.StatusGatewayTimeout)
		case <-ticker.C:
			pollReq, pErr := http.NewRequestWithContext(ctx, http.MethodGet, pollURL, nil)
			if pErr != nil {
				continue
			}
			pollReq.Header.Set("Authorization", "Bearer "+a.apiKey)
			pollReq.Header.Set("X-API-Key", a.apiKey)
			pollReq.Header.Set("Accept", "application/json")

			pResp, pErr := client.Do(pollReq)
			if pErr != nil {
				continue
			}

			pBody, _ := io.ReadAll(pResp.Body)
			pResp.Body.Close()

			var job taskaepi.AepiJobResponse
			if err := json.Unmarshal(pBody, &job); err != nil {
				continue
			}

			if strings.EqualFold(job.Status, "done") {
				outURL := ""
				if job.Output != nil && job.Output.Url != "" {
					outURL = job.Output.Url
					if strings.HasPrefix(outURL, "/") {
						outURL = a.baseURL + outURL
					}
				}
				openAIResp := dto.ImageResponse{
					Created: time.Now().Unix(),
					Data: []dto.ImageData{
						{
							Url: outURL,
						},
					},
				}
				c.JSON(http.StatusOK, openAIResp)
				return nil, nil
			}

			if strings.EqualFold(job.Status, "error") {
				errMsg := job.Error
				if errMsg == "" {
					errMsg = "image generation failed on aepi engine"
				}
				return nil, types.NewOpenAIError(errors.New(errMsg), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest)
			}
		}
	}
}

func (a *Adaptor) GetModelList() []string {
	return taskaepi.AllModels
}

func (a *Adaptor) GetChannelName() string {
	return "aepi"
}
