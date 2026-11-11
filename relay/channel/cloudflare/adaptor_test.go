package cloudflare

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func buildMultipartAudioContext(filename string, rawAudio []byte) *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	buf := &bytes.Buffer{}
	writer := multipart.NewWriter(buf)
	part, _ := writer.CreateFormFile("file", filename)
	_, _ = part.Write(rawAudio)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request = req
	return c
}

func TestCloudflareSTTHeadersAndPayloads(t *testing.T) {
	adaptor := &Adaptor{}
	sampleBytes := []byte("ID3\x04\x00\x00\x00\x00\x00\x00fake-mp3-audio-frames")

	t.Run("whisper-large-v3-turbo uses JSON base64 payload", func(t *testing.T) {
		c := buildMultipartAudioContext("test.mp3", sampleBytes)
		info := &relaycommon.RelayInfo{
			RelayMode: constant.RelayModeAudioTranscription,
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName: "@cf/openai/whisper-large-v3-turbo",
				ApiKey:            "test-key",
			},
		}
		header := http.Header{}
		require.NoError(t, adaptor.SetupRequestHeader(c, &header, info))
		require.Equal(t, "application/json", header.Get("Content-Type"))

		reader, err := adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{Model: info.UpstreamModelName})
		require.NoError(t, err)
		bodyBytes, err := io.ReadAll(reader)
		require.NoError(t, err)

		var payload map[string]any
		require.NoError(t, json.Unmarshal(bodyBytes, &payload))
		require.NotEmpty(t, payload["audio"])
		require.Equal(t, "transcribe", payload["task"])
	})

	t.Run("whisper and whisper-tiny-en use raw binary octet-stream", func(t *testing.T) {
		for _, modelName := range []string{"@cf/openai/whisper", "@cf/openai/whisper-tiny-en"} {
			c := buildMultipartAudioContext("test.mp3", sampleBytes)
			info := &relaycommon.RelayInfo{
				RelayMode: constant.RelayModeAudioTranscription,
				ChannelMeta: &relaycommon.ChannelMeta{
					UpstreamModelName: modelName,
					ApiKey:            "test-key",
				},
			}
			header := http.Header{}
			require.NoError(t, adaptor.SetupRequestHeader(c, &header, info))
			require.Equal(t, "application/octet-stream", header.Get("Content-Type"))

			reader, err := adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{Model: modelName})
			require.NoError(t, err)
			bodyBytes, err := io.ReadAll(reader)
			require.NoError(t, err)
			require.Equal(t, sampleBytes, bodyBytes)
		}
	})

	t.Run("deepgram nova-3 uses raw binary audio content-type", func(t *testing.T) {
		cMp3 := buildMultipartAudioContext("speech.mp3", sampleBytes)
		info := &relaycommon.RelayInfo{
			RelayMode: constant.RelayModeAudioTranscription,
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName: "@cf/deepgram/nova-3",
				ApiKey:            "test-key",
			},
		}
		header := http.Header{}
		require.NoError(t, adaptor.SetupRequestHeader(cMp3, &header, info))
		require.Equal(t, "audio/mpeg", header.Get("Content-Type"))

		reader, err := adaptor.ConvertAudioRequest(cMp3, info, dto.AudioRequest{Model: info.UpstreamModelName})
		require.NoError(t, err)
		bodyBytes, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, sampleBytes, bodyBytes)

		cWav := buildMultipartAudioContext("speech.wav", sampleBytes)
		wavHeader := http.Header{}
		require.NoError(t, adaptor.SetupRequestHeader(cWav, &wavHeader, info))
		require.Equal(t, "audio/wav", wavHeader.Get("Content-Type"))
	})

	t.Run("cfSTTHandler extracts deepgram results.channels shape", func(t *testing.T) {
		raw := []byte(`{"result":{"results":{"channels":[{"alternatives":[{"transcript":"Hello world from Nova 3"}]}]}}}`)
		var parsed any
		require.NoError(t, json.Unmarshal(raw, &parsed))
		require.Equal(t, "Hello world from Nova 3", extractTranscriptFromRaw(parsed))

		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(raw)),
		}
		info := &relaycommon.RelayInfo{
			RelayMode: constant.RelayModeAudioTranscription,
			ChannelMeta: &relaycommon.ChannelMeta{
				UpstreamModelName: "@cf/deepgram/nova-3",
			},
		}
		apiErr, _ := cfSTTHandler(c, info, resp)
		require.Nil(t, apiErr)
		var out map[string]string
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		require.Equal(t, "Hello world from Nova 3", out["text"])
	})
}

func TestCloudflareDeepgramRequestsPunctuation(t *testing.T) {
	adaptor := &Adaptor{}

	info := &relaycommon.RelayInfo{
		RelayMode: constant.RelayModeAudioTranscription,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "@cf/deepgram/nova-3",
			BaseUrl:           "https://api.cloudflare.com",
			ApiVersion:        "account-id",
		},
	}
	info.ChannelBaseUrl = "https://api.cloudflare.com"

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	// Deepgram keeps punctuation and smart formatting off unless the decoding
	// options are passed: without them Nova 3 returned plain lowercase text.
	require.Contains(t, url, "punctuate=true")
	require.Contains(t, url, "smart_format=true")

	whisper := &relaycommon.RelayInfo{
		RelayMode: constant.RelayModeAudioTranscription,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "@cf/openai/whisper-large-v3-turbo",
		},
	}
	whisper.ChannelBaseUrl = "https://api.cloudflare.com"
	whisperURL, err := adaptor.GetRequestURL(whisper)
	require.NoError(t, err)
	require.NotContains(t, whisperURL, "punctuate")
}
