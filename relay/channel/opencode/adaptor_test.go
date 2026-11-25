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
		IsStream: true,
	}
	err := a.SetupRequestHeader(nil, &h, info)
	require.NoError(t, err)

	assert.Equal(t, "Bearer public", h.Get("Authorization"))
	assert.Equal(t, "cli", h.Get("x-opencode-client"))
	assert.Equal(t, "text/event-stream", h.Get("Accept"))
	assert.True(t, strings.HasPrefix(h.Get("User-Agent"), "opencode/"))

	ses := h.Get("x-opencode-session")
	reqID := h.Get("x-opencode-request")
	assert.True(t, strings.HasPrefix(ses, "ses_"), "session must start with ses_")
	assert.Len(t, ses, 30, "official OpenCode session IDs are 30 chars (ses_ + 12 hex + 14 base62)")
	assert.True(t, strings.HasPrefix(reqID, "msg_"), "request must start with msg_")
	assert.Len(t, reqID, 30, "official OpenCode request IDs are 30 chars (msg_ + 12 hex + 14 base62)")
	assert.Equal(t, ses, h.Get("x-session-affinity"))
	assert.Empty(t, h.Get("x-opencode-session-id"))
}
