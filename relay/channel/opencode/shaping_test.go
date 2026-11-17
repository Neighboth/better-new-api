// Portions adapted from github.com/jasonxu114514/opencode2api (GPL-3.0) by
// jasonxu114514. See THIRD-PARTY-LICENSES.md.
package opencode

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
)

func TestShapeChatRequestInjectsOfficialPromptAndRenamesTools(t *testing.T) {
	payload := map[string]any{
		"model": "inkling-2-small-free",
		"messages": []any{
			map[string]any{"role": "system", "content": "Be terse."},
			map[string]any{"role": "user", "content": "hi"},
		},
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        "get_time",
					"description": "Get the current time.",
					"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
				},
			},
		},
	}

	renames := ShapeChatRequest(payload)

	messages, ok := payload["messages"].([]any)
	require.True(t, ok)
	require.Len(t, messages, 2, "system messages fold into the leading official system prompt")
	first, ok := messages[0].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "system", first["role"])
	content, ok := first["content"].(string)
	require.True(t, ok)
	require.True(t, strings.HasPrefix(content, "You are opencode, an interactive CLI tool"), "official prompt must lead")
	require.Contains(t, content, "Be terse.", "the client's own system prompt is preserved after the official one")

	// Client tool renamed to an opaque digest; official tools appended.
	tools, ok := payload["tools"].([]any)
	require.True(t, ok)
	var names []string
	for _, item := range tools {
		entry, ok := item.(map[string]any)
		require.True(t, ok)
		function, ok := entry["function"].(map[string]any)
		require.True(t, ok)
		name, _ := function["name"].(string)
		names = append(names, name)
	}
	var mcpDigest string
	for _, n := range names {
		if strings.HasPrefix(n, "mcp_") {
			mcpDigest = n
		}
	}
	require.Len(t, mcpDigest, len("mcp_")+clientToolDigestLength, "client tool must be renamed to an opaque digest")
	require.NotContains(t, mcpDigest, "get_time", "client tool names must not leak upstream")
	for _, official := range officialToolOrder {
		require.Contains(t, names, official, "official tool set must be appended for the free lane")
	}
	require.Equal(t, names[:len(names)], names, "renames are deterministic")

	// Restoring the response maps the wire names back to the client's names.
	response := map[string]any{
		"choices": []any{
			map[string]any{
				"message": map[string]any{
					"tool_calls": []any{
						map[string]any{
							"function": map[string]any{"name": mcpDigest},
						},
					},
				},
			},
		},
	}
	body, err := common.Marshal(response)
	require.NoError(t, err)
	restored := RestoreChatResponse(body, renames)
	require.Contains(t, string(restored), "get_time")
}

func TestRestoreChatStreamLineRewritesDataChunks(t *testing.T) {
	renames := newToolRenames()
	renames.add("get_time", "mcp_deadbeefdead")

	line := []byte(`data: {"choices":[{"delta":{"tool_calls":[{"function":{"name":"mcp_deadbeefdead"}}]}}]}` + "\n")
	out := RestoreChatStreamLine(line, renames)
	require.Contains(t, string(out), "get_time")

	done := []byte("data: [DONE]\n")
	require.Equal(t, done, RestoreChatStreamLine(done, renames))

	unrelated := []byte(": comment\n")
	require.Equal(t, unrelated, RestoreChatStreamLine(unrelated, renames))
}

func TestShapeChatRequestLeavesCompliantRequestUntouched(t *testing.T) {
	prompt := "user message that already carries the official prompt marker: You are opencode, an interactive CLI tool"
	payload := map[string]any{
		"model": "inkling-2-small-free",
		"messages": []any{
			map[string]any{"role": "system", "content": prompt},
		},
	}
	ShapeChatRequest(payload)
	messages := payload["messages"].([]any)
	require.Len(t, messages, 1, "a request already carrying the official prompt must not get a second one prepended")
}
