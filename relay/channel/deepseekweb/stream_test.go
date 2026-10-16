package deepseekweb

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The payloads below are captured verbatim from chat.deepseek.com while relaying
// a thinking-enabled request, so the decoder is pinned against the real wire
// protocol rather than an invented fixture.
func TestReadCompletionStreamReplaysCapturedThinkingTurn(t *testing.T) {
	stream := strings.Join([]string{
		"event: ready\n",
		`data: {"request_message_id":1,"response_message_id":2,"model_type":"default"}` + "\n",
		"\n",
		"event: update_session\n",
		`data: {"updated_at":1791834438.0296009}` + "\n",
		"\n",
		`data: {"v":{"response":{"message_id":2,"parent_id":1,"role":"ASSISTANT","thinking_enabled":true,"status":"WIP","accumulated_token_usage":0,"fragments":[{"id":2,"type":"THINK","content":"We","references":[],"stage_id":1}]}}}` + "\n",
		"\n",
		`data: {"p":"response/fragments/-1/content","o":"APPEND","v":" need"}` + "\n",
		"\n",
		`data: {"v":" 391"}` + "\n",
		"\n",
		`data: {"v":"."}` + "\n",
		"\n",
		`data: {"p":"response/fragments","o":"APPEND","v":[{"id":3,"type":"RESPONSE","content":"17","references":[],"stage_id":1}]}` + "\n",
		"\n",
		`data: {"v":"*"}` + "\n",
		`data: {"v":"23"}` + "\n",
		`data: {"v":" = 391"}` + "\n",
		`data: {"p":"response","o":"BATCH","v":[{"p":"accumulated_token_usage","v":108},{"p":"quasi_status","v":"FINISHED"}]}` + "\n",
		`data: {"p":"response/status","o":"SET","v":"FINISHED"}` + "\n",
		"\n",
		"event: close\n",
		`data: {"click_behavior":"none","auto_resume":false}` + "\n",
		"\n",
	}, "")

	var deltas []streamDelta
	state, err := readCompletionStream(strings.NewReader(stream), func(delta streamDelta) {
		deltas = append(deltas, delta)
	})
	require.NoError(t, err)

	assert.Equal(t, "We need 391.", state.reasoning())
	assert.Equal(t, "17*23 = 391", state.content())
	assert.Equal(t, 108, state.usage)
	assert.True(t, state.finished)

	// Every delta must be reconstructable into the accumulated message, because
	// that is what the relayed OpenAI chunks have already sent to the client.
	var replayedContent, replayedReasoning strings.Builder
	for _, delta := range deltas {
		replayedContent.WriteString(delta.content)
		replayedReasoning.WriteString(delta.reasoning)
	}
	assert.Equal(t, state.content(), replayedContent.String())
	assert.Equal(t, state.reasoning(), replayedReasoning.String())
}

func TestReadCompletionStreamHandlesSetAndLegacyThinkingContent(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"v":{"response":{"fragments":[{"id":1,"type":"RESPONSE","content":"Hello"}]}}}` + "\n",
		`data: {"p":"response/fragments/0/content","o":"SET","v":"Hello there"}` + "\n",
		`data: {"p":"response/thinking_content","o":"APPEND","v":"thinking hard"}` + "\n",
		`data: {"p":"response/status","o":"SET","v":"FINISHED"}` + "\n",
	}, "")

	state, err := readCompletionStream(strings.NewReader(stream), func(streamDelta) {})
	require.NoError(t, err)

	// A SET carries the whole fragment, so only the unseen suffix is new.
	assert.Equal(t, "Hello there", state.content())
	assert.Equal(t, "thinking hard", state.reasoning())
	assert.True(t, state.finished)
}

func TestReadCompletionStreamIgnoresUnrelatedPatches(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"v":{"response":{"fragments":[{"id":1,"type":"RESPONSE","content":"Hi"}]}}}` + "\n",
		`data: {"p":"response/search_status","o":"SET","v":"SEARCHING"}` + "\n",
		`data: {"p":"response/fragments/7/content","o":"APPEND","v":"ignored"}` + "\n",
		`data: {"v":"!"}` + "\n",
		`data: {"p":"response/status","o":"SET","v":"FINISHED"}` + "\n",
	}, "")

	state, err := readCompletionStream(strings.NewReader(stream), func(streamDelta) {})
	require.NoError(t, err)
	assert.Equal(t, "Hi!", state.content())
}

func TestReadCompletionStreamStopsAtFinished(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"v":{"response":{"fragments":[{"id":1,"type":"RESPONSE","content":"done"}]}}}` + "\n",
		`data: {"p":"response/status","o":"SET","v":"FINISHED"}` + "\n",
		`data: {"v":" after-finish"}` + "\n",
	}, "")

	state, err := readCompletionStream(strings.NewReader(stream), func(streamDelta) {})
	require.NoError(t, err)
	assert.Equal(t, "done", state.content())
}

func TestReadCompletionStreamRejectsMalformedPayload(t *testing.T) {
	_, err := readCompletionStream(strings.NewReader("data: {not json}\n"), func(streamDelta) {})
	require.Error(t, err)
}

func TestReadCompletionStreamToleratesTruncatedBody(t *testing.T) {
	// Upstream closes the socket without the FINISHED patch; the partial text
	// must still be relayed instead of failing the request.
	stream := `data: {"v":{"response":{"fragments":[{"id":1,"type":"RESPONSE","content":"partial"}]}}}` + "\n"
	state, err := readCompletionStream(strings.NewReader(stream), func(streamDelta) {})
	require.NoError(t, err)
	assert.Equal(t, "partial", state.content())
	assert.False(t, state.finished)
}

func TestDeltaForKindSeparatesThinking(t *testing.T) {
	assert.Equal(t, streamDelta{reasoning: "why"}, deltaForKind("THINK", "why"))
	assert.Equal(t, streamDelta{reasoning: "why"}, deltaForKind("thinking", "why"))
	assert.Equal(t, streamDelta{content: "answer"}, deltaForKind("RESPONSE", "answer"))
	assert.Equal(t, streamDelta{content: "answer"}, deltaForKind("", "answer"))
}
