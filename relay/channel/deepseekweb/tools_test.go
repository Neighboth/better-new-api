package deepseekweb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveToken(t *testing.T) {
	t.Run("raw token", func(t *testing.T) {
		tok, err := resolveToken("  eyJhbGciOiJIUzI1NiJ9.abc  ")
		require.NoError(t, err)
		assert.Equal(t, "eyJhbGciOiJIUzI1NiJ9.abc", tok)
	})

	t.Run("json wrapper", func(t *testing.T) {
		tok, err := resolveToken(`{"value":"tok-123","other":1}`)
		require.NoError(t, err)
		assert.Equal(t, "tok-123", tok)
	})

	t.Run("quoted token", func(t *testing.T) {
		tok, err := resolveToken(`"tok-quoted"`)
		require.NoError(t, err)
		assert.Equal(t, "tok-quoted", tok)
	})

	t.Run("token containing colon does not trigger login", func(t *testing.T) {
		tok, err := resolveToken("prefix:token-value")
		require.NoError(t, err)
		assert.Equal(t, "prefix:token-value", tok)
	})

	t.Run("empty returns error", func(t *testing.T) {
		_, err := resolveToken("   ")
		require.Error(t, err)
	})
}

func TestParseToolCalls(t *testing.T) {
	t.Run("extracts single tool call with markdown fence", func(t *testing.T) {
		input := "Let me check.\n<tool_call>\n```json\n{\"name\": \"get_weather\", \"arguments\": {\"city\": \"Istanbul\"}}\n```\n</tool_call>"
		clean, calls := parseToolCalls(input)
		assert.Equal(t, "Let me check.", clean)
		require.Len(t, calls, 1)
		assert.Equal(t, "function", calls[0].Type)
		assert.Equal(t, "get_weather", calls[0].Function.Name)
		assert.JSONEq(t, `{"city":"Istanbul"}`, calls[0].Function.Arguments)
		assert.True(t, len(calls[0].ID) > 5)
		require.NotNil(t, calls[0].Index)
		assert.Equal(t, 0, *calls[0].Index)
	})

	t.Run("extracts multiple tool calls", func(t *testing.T) {
		input := `<tool_call>{"name":"a","arguments":{"x":1}}</tool_call><tool_call>{"name":"b","arguments":"{\"y\":2}"}</tool_call>`
		clean, calls := parseToolCalls(input)
		assert.Empty(t, clean)
		require.Len(t, calls, 2)
		assert.Equal(t, "a", calls[0].Function.Name)
		assert.Equal(t, 0, *calls[0].Index)
		assert.Equal(t, "b", calls[1].Function.Name)
		assert.Equal(t, 1, *calls[1].Index)
		assert.JSONEq(t, `{"y":2}`, calls[1].Function.Arguments)
	})

	t.Run("no tool calls returns original text", func(t *testing.T) {
		clean, calls := parseToolCalls("Just a normal answer.")
		assert.Equal(t, "Just a normal answer.", clean)
		assert.Empty(t, calls)
	})
}
