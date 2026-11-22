package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteModelTestVariants(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello from colab"}}]}`))
		case "/v1/videos/generations":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/out.mp4"}]}`))
		case "/v1/images/generations":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":[{"url":"https://cdn.example.com/out.png"}]}`))
		case "/v1/audio/speech":
			w.Header().Set("Content-Type", "audio/wav")
			_, _ = w.Write([]byte("RIFFWAVE"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	textRes, err := executeModelTest(srv.URL, "org/llm", "text", "hi", "")
	require.NoError(t, err)
	assert.Equal(t, "text", textRes.Type)
	assert.Equal(t, "hello from colab", textRes.Text)

	videoRes, err := executeModelTest(srv.URL, "org/vid", "video", "cat", "")
	require.NoError(t, err)
	assert.Equal(t, "video", videoRes.Type)
	assert.Equal(t, "https://cdn.example.com/out.mp4", videoRes.MediaURL)

	imgRes, err := executeModelTest(srv.URL, "org/img", "image", "cat", "")
	require.NoError(t, err)
	assert.Equal(t, "image", imgRes.Type)
	assert.Equal(t, "https://cdn.example.com/out.png", imgRes.MediaURL)

	audRes, err := executeModelTest(srv.URL, "org/tts", "audio", "hello", "")
	require.NoError(t, err)
	assert.Equal(t, "audio", audRes.Type)
	assert.Contains(t, audRes.MediaURL, "data:audio/wav;base64,")
}
