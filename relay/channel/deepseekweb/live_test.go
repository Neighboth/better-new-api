package deepseekweb

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests drive the real chat.deepseek.com endpoints with a live user token.
// They are the only tests that can prove the PoW header, the file upload
// handshake and the streaming parser all still match upstream, so they are kept
// behind DEEPSEEK_USER_TOKEN and skip on a hermetic run.

func liveClient(t *testing.T) (*webClient, context.Context) {
	t.Helper()
	token := deepSeekLiveToken(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	return &webClient{baseURL: defaultBaseURL, token: token, http: &http.Client{Timeout: 2 * time.Minute}}, ctx
}

func TestLiveSessionAndPlainCompletion(t *testing.T) {
	client, ctx := liveClient(t)

	sessionID, err := client.createSession(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, sessionID)

	resp, err := client.completion(ctx, sessionID, "Reply with exactly: STREAM-OK", nil, false)
	require.NoError(t, err)
	defer resp.Body.Close()

	state, err := readCompletionStream(resp.Body, func(streamDelta) {})
	require.NoError(t, err)
	assert.Contains(t, state.content(), "STREAM-OK")
}

func TestLiveTextAttachmentRoundTrip(t *testing.T) {
	client, ctx := liveClient(t)

	const codeword = "ZYPHER-4417"
	fileID, err := client.uploadAttachment(ctx, "secret.txt", "text/plain",
		[]byte(fmt.Sprintf("The secret codeword is %s. Reply with exactly that codeword and nothing else.", codeword)))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(fileID, "file-"), "unexpected file id %q", fileID)

	sessionID, err := client.createSession(ctx)
	require.NoError(t, err)

	resp, err := client.completion(ctx, sessionID, "What is the secret codeword in the attached file?", []string{fileID}, false)
	require.NoError(t, err)
	defer resp.Body.Close()

	state, err := readCompletionStream(resp.Body, func(streamDelta) {})
	require.NoError(t, err)
	assert.Contains(t, state.content(), codeword)
}

func TestLiveImageAttachmentRoundTrip(t *testing.T) {
	client, ctx := liveClient(t)

	fileID, err := client.uploadAttachment(ctx, "swatch.png", "image/png", solidPNG(t, 240, 240, color.RGBA{R: 0xE0, G: 0x10, B: 0xA0, A: 0xFF}))
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(fileID, "file-"), "unexpected file id %q", fileID)

	sessionID, err := client.createSession(ctx)
	require.NoError(t, err)

	resp, err := client.completion(ctx, sessionID,
		"What is the dominant color of the attached image? Answer with a single color word.", []string{fileID}, false)
	require.NoError(t, err)
	defer resp.Body.Close()

	state, err := readCompletionStream(resp.Body, func(streamDelta) {})
	require.NoError(t, err)

	answer := strings.ToLower(state.content())
	require.NotEmpty(t, answer)
	// The swatch is magenta/pink; accept either spelling rather than pinning the
	// model's exact vocabulary.
	assert.True(t, strings.Contains(answer, "magenta") || strings.Contains(answer, "pink") || strings.Contains(answer, "purple"),
		"expected a magenta family answer, got %q", state.content())
}

func TestLiveThinkingStreamSeparatesReasoning(t *testing.T) {
	client, ctx := liveClient(t)

	sessionID, err := client.createSession(ctx)
	require.NoError(t, err)

	resp, err := client.completion(ctx, sessionID, "What is 17*23? Think step by step.", nil, true)
	require.NoError(t, err)
	defer resp.Body.Close()

	state, err := readCompletionStream(resp.Body, func(streamDelta) {})
	require.NoError(t, err)

	assert.NotEmpty(t, state.reasoning(), "thinking was enabled but no THINK fragments arrived")
	assert.Contains(t, state.content(), "391")
	assert.Positive(t, state.usage)
}

// TestLiveAttachmentThroughAdaptorPipeline runs the same path a relayed request
// takes: OpenAI style messages with a base64 data URI image in, uploaded and
// referenced by the completion.
func TestLiveAttachmentThroughAdaptorPipeline(t *testing.T) {
	client, ctx := liveClient(t)

	imageData := base64.StdEncoding.EncodeToString(solidPNG(t, 128, 128, color.RGBA{R: 0x10, G: 0x80, B: 0xF0, A: 0xFF}))
	messages := []dto.Message{
		{
			Role: "user",
			Content: []any{
				map[string]any{"type": "text", "text": "What is the dominant color of this image? One word."},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64," + imageData}},
			},
		},
	}

	attachments := collectAttachments(messages)
	require.Len(t, attachments, 1)

	fileIDs, err := uploadAttachments(ctx, client, attachments)
	require.NoError(t, err)
	require.Len(t, fileIDs, 1)

	sessionID, err := client.createSession(ctx)
	require.NoError(t, err)

	resp, err := client.completion(ctx, sessionID, buildPrompt(messages), fileIDs, false)
	require.NoError(t, err)
	defer resp.Body.Close()

	state, err := readCompletionStream(resp.Body, func(streamDelta) {})
	require.NoError(t, err)

	answer := strings.ToLower(state.content())
	require.NotEmpty(t, answer)
	assert.True(t, strings.Contains(answer, "blue") || strings.Contains(answer, "azure") || strings.Contains(answer, "cyan"),
		"expected a blue family answer, got %q", state.content())
}

func solidPNG(t *testing.T, width, height int, fill color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, fill)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}
