package constant

import "strings"

// Markers around the channel system prompt when it is injected into a
// conversation. The relay fallback path uses these markers to split the
// channel prompt from the client prompt so it can recompose the layered
// system message: channel first, client second, fallback last.
const ChannelSystemPromptStart = "[NAAPI-CSP]"
const ChannelSystemPromptEnd = "[/NAAPI-CSP]"

// WrapChannelSystemPrompt wraps the given channel system prompt in the
// marker tags so downstream relay code can distinguish it from the client
// prompt inside the same system message.
func WrapChannelSystemPrompt(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return prompt
	}
	return ChannelSystemPromptStart + "\n" + prompt + "\n" + ChannelSystemPromptEnd
}

// SplitChannelSystemPrompt finds the wrapped channel prompt inside a system
// message. It returns the channel prompt, the client prompt (everything
// outside the markers) and whether a wrapped channel prompt was found.
func SplitChannelSystemPrompt(content string) (channelPrompt, clientPrompt string, found bool) {
	start := strings.Index(content, ChannelSystemPromptStart)
	if start == -1 {
		return "", strings.TrimSpace(content), false
	}
	end := strings.Index(content[start:], ChannelSystemPromptEnd)
	if end == -1 {
		return "", strings.TrimSpace(content), false
	}
	end += start
	ch := content[start+len(ChannelSystemPromptStart) : end]
	before := content[:start]
	after := content[end+len(ChannelSystemPromptEnd):]
	cg := strings.TrimSpace(before + "\n" + after)
	return strings.TrimSpace(ch), cg, true
}

// SystemPromptLayerTitle is the header written above the three layers when
// the relay fallback composes them into one system message.
const SystemPromptLayerTitle = "[Three separate system prompts follow. Each is a distinct layer; comply with every applicable layer. Authority order (highest first):\n" +
	"1. CHANNEL PROMPT — mandatory. Always applied exactly as written. Nothing, not the client prompt and not the fallback prompt, may change or contradict it. Where any other layer conflicts with the channel prompt, the channel prompt wins.\n" +
	"2. CLIENT PROMPT — the client's own instructions. Applied unless it conflicts with the channel prompt; in that case the channel prompt wins.\n" +
	"3. FALLBACK PROMPT — the fallback relay instructions for this reply. Applied EXCEPT for any of its parts that conflict with the client prompt. Where such a conflict exists, the client prompt wins and only the non-conflicting parts of the fallback prompt are applied.\n" +
	"Apply non-conflicting parts of every layer. The model must treat each marked section as a separate, distinct prompt and follow them in the authority order above.\n" +
	"---\n"
