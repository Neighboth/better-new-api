package aepi

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestParseAspectAndResolution(t *testing.T) {
	tests := []struct {
		size           string
		wantAspect     string
		wantResolution string
	}{
		{"1920x1080", "16:9", "1080p"},
		{"1280x720", "16:9", "720p"},
		{"1080x1920", "9:16", "1080p"},
		{"720x1280", "9:16", "720p"},
		{"1024x1024", "1:1", "1080p"},
		{"720x720", "1:1", "720p"},
		{"2520x1080", "21:9", "1080p"},
		{"1680x720", "21:9", "720p"},
		{"16:9", "16:9", "720p"},
		{"9:16", "9:16", "720p"},
		{"1:1", "1:1", "720p"},
		{"720p", "16:9", "720p"},
		{"1080p", "16:9", "1080p"},
		{"4k", "16:9", "4k"},
	}

	for _, tt := range tests {
		t.Run(tt.size, func(t *testing.T) {
			aspect, res := parseAspectAndResolution(tt.size)
			assert.Equal(t, tt.wantAspect, aspect)
			assert.Equal(t, tt.wantResolution, res)
		})
	}
}

func TestConvertToAepiJobRequest_Video(t *testing.T) {
	adaptor := &TaskAdaptor{
		baseURL: "https://videoapi.aerobox.eu.cc",
		apiKey:  "sk_live_test",
	}

	req := &relaycommon.TaskSubmitReq{
		Prompt:   "A cinematic cyber city shot",
		Size:     "1920x1080",
		Duration: 6,
		Metadata: map[string]interface{}{
			"style": "cinematic",
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "seedance-2.5",
		},
	}

	job, err := adaptor.convertToAepiJobRequest(nil, req, info)
	assert.NoError(t, err)
	assert.Equal(t, "create", job.Operation)
	assert.Equal(t, "seedance-2.5", job.Model)
	assert.Equal(t, "A cinematic cyber city shot", job.Prompt)
	assert.Equal(t, 6, job.Duration)
	assert.Equal(t, "16:9", job.Aspect)
	assert.Equal(t, "1080p", job.Resolution)
	assert.Equal(t, "cinematic", job.Style)
}

func TestConvertToAepiJobRequest_Music(t *testing.T) {
	adaptor := &TaskAdaptor{
		baseURL: "https://videoapi.aerobox.eu.cc",
		apiKey:  "sk_live_test",
	}

	req := &relaycommon.TaskSubmitReq{
		Prompt: "A warm ambient drone",
		Model:  "music",
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "music",
		},
	}

	job, err := adaptor.convertToAepiJobRequest(nil, req, info)
	assert.NoError(t, err)
	assert.Equal(t, "music", job.Operation)
	// Music must omit model!
	assert.Equal(t, "", job.Model)
	assert.Equal(t, "A warm ambient drone", job.Prompt)
}

func TestConvertToAepiJobRequest_Rescale(t *testing.T) {
	adaptor := &TaskAdaptor{
		baseURL: "https://videoapi.aerobox.eu.cc",
		apiKey:  "sk_live_test",
	}

	req := &relaycommon.TaskSubmitReq{
		Model: "rescale",
		Size:  "1:1",
		Metadata: map[string]interface{}{
			"source": "upload_abc123",
			"fit":    "cover",
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "rescale",
		},
	}

	job, err := adaptor.convertToAepiJobRequest(nil, req, info)
	assert.NoError(t, err)
	assert.Equal(t, "rescale", job.Operation)
	assert.Equal(t, "", job.Model)
	assert.Equal(t, "upload_abc123", job.Source)
	assert.Equal(t, "cover", job.Fit)
	assert.Equal(t, "1:1", job.Aspect)
}

func TestConvertToOpenAIVideo(t *testing.T) {
	adaptor := &TaskAdaptor{}
	originTask := &model.Task{
		TaskID:     "task_999",
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		SubmitTime: 1731180000,
		FinishTime: 1731180030,
		Data: []byte(`{
			"id": "job_123",
			"status": "done",
			"progress": 100,
			"duration": 5,
			"output": {
				"url": "/files/out-123.mp4",
				"name": "out-123.mp4"
			}
		}`),
	}

	bytes, err := adaptor.ConvertToOpenAIVideo(originTask)
	assert.NoError(t, err)

	var video dto.OpenAIVideo
	err = jsonUnmarshal(bytes, &video)
	assert.NoError(t, err)
	assert.Equal(t, "task_999", video.ID)
	assert.Equal(t, "completed", video.Status)
	assert.Equal(t, "https://videoapi.aerobox.eu.cc/files/out-123.mp4", video.Metadata["url"])
	assert.Equal(t, "5", video.Seconds)
}

func jsonUnmarshal(data []byte, v any) error {
	gin.SetMode(gin.TestMode)
	return json.Unmarshal(data, v)
}
