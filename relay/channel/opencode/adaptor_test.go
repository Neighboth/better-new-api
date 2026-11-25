package opencode

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestGetRequestURLNormalizesLegacyPaths(t *testing.T) {
	a := &Adaptor{}
	cases := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "empty uses default v1 endpoint",
			baseURL:  "",
			expected: "https://opencode.ai/inference/openai/v1/chat/completions",
		},
		{
			name:     "legacy base without v1 gets normalized",
			baseURL:  "https://opencode.ai/inference/openai",
			expected: "https://opencode.ai/inference/openai/v1/chat/completions",
		},
		{
			name:     "standard v1 base appends chat completions",
			baseURL:  "https://opencode.ai/inference/openai/v1",
			expected: "https://opencode.ai/inference/openai/v1/chat/completions",
		},
		{
			name:     "zen base without v1 gets normalized",
			baseURL:  "https://opencode.ai/zen",
			expected: "https://opencode.ai/zen/v1/chat/completions",
		},
		{
			name:     "full v1 chat completions preserved",
			baseURL:  "https://opencode.ai/zen/v1/chat/completions",
			expected: "https://opencode.ai/zen/v1/chat/completions",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: tc.baseURL}}
			got, err := a.GetRequestURL(info)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestSetupRequestHeaderMatchesOfficialCLIFormat(t *testing.T) {
	a := &Adaptor{}
	h := http.Header{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId: 42,
			ApiKey:    "",
		},
		IsStream: false,
	}
	err := a.SetupRequestHeader(nil, &h, info)
	require.NoError(t, err)

	assert.Equal(t, "Bearer public", h.Get("Authorization"))
	assert.Equal(t, "cli", h.Get("x-opencode-client"))
	assert.Equal(t, "text/event-stream", h.Get("Accept"))
	assert.Equal(t, "opencode/latest/2.1.1/cli", h.Get("User-Agent"))

	ses := h.Get("x-opencode-session")
	assert.True(t, strings.HasPrefix(ses, "ses_"), "session must start with ses_")
	assert.Len(t, ses, 30, "official OpenCode session IDs are 30 chars (ses_ + 12 hex + 14 base62)")
	assert.Equal(t, ses, h.Get("x-opencode-session-id"))
	assert.Equal(t, ses, h.Get("x-session-affinity"))
	assert.Equal(t, ses, h.Get("X-Session-Id"))

	proj := h.Get("x-opencode-project")
	assert.Len(t, proj, 40, "x-opencode-project must be a 40-char hex hash")
	assert.Empty(t, h.Get("x-opencode-request"), "v2 /inference must not send x-opencode-request")
}

func TestCollapseSSEStreamToChatCompletionJSON(t *testing.T) {
	sse := strings.Join([]string{
		`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1700000000,"model":"big-pickle","choices":[{"index":0,"delta":{"role":"assistant","content":"Hello "},"finish_reason":null}]}`,
		``,
		`data: {"id":"chatcmpl-123","object":"chat.completion.chunk","created":1700000000,"model":"big-pickle","choices":[{"index":0,"delta":{"content":"world!"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	out, ok := collapseSSEStreamToJSON([]byte(sse))
	require.True(t, ok)
	assert.Contains(t, string(out), `"object":"chat.completion"`)
	assert.Contains(t, string(out), `"content":"Hello world!"`)
	assert.Contains(t, string(out), `"finish_reason":"stop"`)
	assert.Contains(t, string(out), `"total_tokens":12`)
}
