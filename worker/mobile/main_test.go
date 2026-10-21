package main

import (
	"encoding/json"
	"testing"

	"github.com/QuantumNous/new-api/worker/llama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInferenceChunkFrameCarriesAJSONObject protects the format of the tunnel frames.
//
// The payload of a frame has to be a JSON object. A byte slice would be encoded as a
// base64 string, the coordinator would not be able to read the answer and the client
// would receive an empty completion.
func TestInferenceChunkFrameCarriesAJSONObject(t *testing.T) {
	frame := inferenceChunkFrame("request-1", "merhaba", 7, false)

	encoded, err := json.Marshal(frame)
	require.NoError(t, err)

	var decoded struct {
		Type    string          `json:"type"`
		Id      string          `json:"id"`
		Payload json.RawMessage `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, "inference_chunk", decoded.Type)
	assert.Equal(t, "request-1", decoded.Id)

	var chunk struct {
		RequestId string `json:"request_id"`
		Content   string `json:"content"`
		TokenId   int    `json:"token_id"`
		IsFinal   bool   `json:"is_final"`
	}
	require.NoError(t, json.Unmarshal(decoded.Payload, &chunk))
	assert.Equal(t, "request-1", chunk.RequestId)
	assert.Equal(t, "merhaba", chunk.Content)
	assert.Equal(t, 7, chunk.TokenId)
	assert.False(t, chunk.IsFinal)
}

// TestInferenceDoneFrameCarriesTheMeasuredValues protects the closing frame of an
// inference, the token usage of the answer comes from it.
func TestInferenceDoneFrameCarriesTheMeasuredValues(t *testing.T) {
	result := &llama.Result{
		TotalTokens:  42,
		PromptTokens: 11,
		TtftMs:       830,
		Tps:          17.5,
	}
	frame := inferenceDoneFrame("request-2", result)

	encoded, err := json.Marshal(frame)
	require.NoError(t, err)

	var decoded struct {
		Payload json.RawMessage `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(encoded, &decoded))

	var done struct {
		RequestId    string  `json:"request_id"`
		TotalTokens  int     `json:"total_tokens"`
		PromptTokens int     `json:"prompt_tokens"`
		TtftMs       int     `json:"ttft_ms"`
		Tps          float64 `json:"tps"`
	}
	require.NoError(t, json.Unmarshal(decoded.Payload, &done))
	assert.Equal(t, "request-2", done.RequestId)
	assert.Equal(t, 42, done.TotalTokens)
	assert.Equal(t, 11, done.PromptTokens)
	assert.Equal(t, 830, done.TtftMs)
	assert.Equal(t, 17.5, done.Tps)
}
