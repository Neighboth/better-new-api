package deepseekweb

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

var toolCallBlockRe = regexp.MustCompile(`(?is)<tool_call>\s*(.*?)\s*</tool_call>`)

// isToolChoiceNone reports whether the caller explicitly disabled tool calls.
func isToolChoiceNone(choice any) bool {
	if choice == nil {
		return false
	}
	s, ok := choice.(string)
	return ok && strings.EqualFold(strings.TrimSpace(s), "none")
}

// isToolChoiceRequired reports whether the caller requires at least one tool call.
func isToolChoiceRequired(choice any) bool {
	if choice == nil {
		return false
	}
	s, ok := choice.(string)
	return ok && strings.EqualFold(strings.TrimSpace(s), "required")
}

// extractForcedToolName returns the function name when tool_choice forces a
// specific function, e.g. {"type": "function", "function": {"name": "..."}}.
func extractForcedToolName(choice any) string {
	m, ok := choice.(map[string]any)
	if !ok {
		return ""
	}
	fn, ok := m["function"].(map[string]any)
	if !ok {
		return ""
	}
	name, _ := fn["name"].(string)
	return strings.TrimSpace(name)
}

// parseToolCalls extracts <tool_call>...</tool_call> blocks from assistant
// output and returns the remaining prose plus OpenAI-compatible tool calls.
func parseToolCalls(content string) (string, []dto.ToolCallResponse) {
	matches := toolCallBlockRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content, nil
	}

	var calls []dto.ToolCallResponse
	var cleanBuilder strings.Builder
	lastEnd := 0

	for _, match := range matches {
		blockStart, blockEnd := match[0], match[1]
		innerStart, innerEnd := match[2], match[3]

		cleanBuilder.WriteString(content[lastEnd:blockStart])
		lastEnd = blockEnd

		rawInner := stripMarkdownFence(strings.TrimSpace(content[innerStart:innerEnd]))
		call, ok := decodeToolCallJSON(rawInner, len(calls))
		if ok {
			calls = append(calls, call)
		} else {
			// Keep unparseable blocks in text so nothing is silently lost.
			cleanBuilder.WriteString(content[blockStart:blockEnd])
		}
	}
	cleanBuilder.WriteString(content[lastEnd:])

	return strings.TrimSpace(cleanBuilder.String()), calls
}

func stripMarkdownFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	lines = lines[1:]
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func decodeToolCallJSON(raw string, index int) (dto.ToolCallResponse, bool) {
	if raw == "" {
		return dto.ToolCallResponse{}, false
	}
	var payload struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := common.Unmarshal([]byte(raw), &payload); err != nil {
		return dto.ToolCallResponse{}, false
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return dto.ToolCallResponse{}, false
	}

	args := normalizeToolArguments(payload.Arguments)
	idx := index
	return dto.ToolCallResponse{
		Index: &idx,
		ID:    "call_" + strings.ReplaceAll(common.GetUUID(), "-", "")[:24],
		Type:  "function",
		Function: dto.FunctionResponse{
			Name:      name,
			Arguments: args,
		},
	}, true
}

func normalizeToolArguments(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "{}"
	}
	if trimmed[0] == '"' {
		var s string
		if err := common.Unmarshal(trimmed, &s); err == nil {
			s = strings.TrimSpace(s)
			if s == "" {
				return "{}"
			}
			return s
		}
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, trimmed); err == nil {
		return compacted.String()
	}
	return string(trimmed)
}
