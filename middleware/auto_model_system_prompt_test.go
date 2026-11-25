package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectAliasSystemPrompt_Formats(t *testing.T) {
	t.Run("OpenAI chat prepends new system message when absent", func(t *testing.T) {
		in := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}]}`)
		out := injectAliasSystemPrompt("/v1/chat/completions", in, "You are concise.")
		var parsed map[string]any
		require.NoError(t, common.Unmarshal(out, &parsed))
		msgs, ok := parsed["messages"].([]any)
		require.True(t, ok)
		require.Len(t, msgs, 2)
		first := msgs[0].(map[string]any)
		assert.Equal(t, "system", first["role"])
		assert.Equal(t, "You are concise.", first["content"])
	})

	t.Run("OpenAI chat prepends to existing system message", func(t *testing.T) {
		in := []byte(`{"model":"gpt-4o","messages":[{"role":"system","content":"Existing."},{"role":"user","content":"hi"}]}`)
		out := injectAliasSystemPrompt("/v1/chat/completions", in, "Alias rule.")
		var parsed map[string]any
		require.NoError(t, common.Unmarshal(out, &parsed))
		msgs := parsed["messages"].([]any)
		require.Len(t, msgs, 2)
		assert.Equal(t, "Alias rule.\n\nExisting.", msgs[0].(map[string]any)["content"])
	})

	t.Run("Claude messages prepends to system", func(t *testing.T) {
		in := []byte(`{"model":"claude-3-5-sonnet","system":"Base.","messages":[{"role":"user","content":"hi"}]}`)
		out := injectAliasSystemPrompt("/v1/messages", in, "Injected.")
		var parsed map[string]any
		require.NoError(t, common.Unmarshal(out, &parsed))
		assert.Equal(t, "Injected.\n\nBase.", parsed["system"])
	})

	t.Run("Responses API prepends to instructions", func(t *testing.T) {
		in := []byte(`{"model":"gpt-4.1","input":"hello","instructions":"Be brief."}`)
		out := injectAliasSystemPrompt("/v1/responses", in, "Alias instruction.")
		var parsed map[string]any
		require.NoError(t, common.Unmarshal(out, &parsed))
		assert.Equal(t, "Alias instruction.\n\nBe brief.", parsed["instructions"])
	})

	t.Run("Gemini prepends to systemInstruction", func(t *testing.T) {
		in := []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`)
		out := injectAliasSystemPrompt("/v1beta/models/gemini-2.5-flash:generateContent", in, "Gemini system.")
		var parsed map[string]any
		require.NoError(t, common.Unmarshal(out, &parsed))
		si := parsed["systemInstruction"].(map[string]any)
		parts := si["parts"].([]any)
		require.Len(t, parts, 1)
		assert.Equal(t, "Gemini system.", parts[0].(map[string]any)["text"])
	})
}
