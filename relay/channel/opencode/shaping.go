// Portions adapted from github.com/jasonxu114514/opencode2api (GPL-3.0) by
// jasonxu114514. See THIRD-PARTY-LICENSES.md.
package opencode

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// AssetPath returns the embedded official CLI prompt and tool vocabulary.
var opencodePromptMarker = "You are opencode, an interactive CLI tool"

type toolRenames struct {
	toWire   map[string]string
	toClient map[string]string
}

func newToolRenames() *toolRenames {
	return &toolRenames{toWire: map[string]string{}, toClient: map[string]string{}}
}

func (r *toolRenames) add(client, wire string) {
	if client == "" || wire == "" || client == wire {
		return
	}
	if existing, ok := r.toWire[client]; ok {
		if existing != wire {
			return
		}
	}
	if _, taken := r.toClient[wire]; taken {
		return
	}
	r.toWire[client] = wire
	r.toClient[wire] = client
}

func (r *toolRenames) wireName(client string) (string, bool) {
	w, ok := r.toWire[client]
	return w, ok
}

func (r *toolRenames) clientName(wire string) (string, bool) {
	c, ok := r.toClient[wire]
	return c, ok
}

func (r *toolRenames) restore(wire string) string {
	if c, ok := r.toClient[wire]; ok {
		return c
	}
	return wire
}

func (r *toolRenames) empty() bool { return len(r.toWire) == 0 }

const clientToolPrefix = "mcp_"
const clientToolDigestLength = 12
const toolWireNameLimit = 64

func clientWireName(client string, used map[string]bool) string {
	digest := sha256.Sum256([]byte(client))
	clean := hex.EncodeToString(digest[:])[:clientToolDigestLength]
	max := toolWireNameLimit - len(clientToolPrefix)
	if len(clean) > max {
		clean = clean[:max]
	}
	base := clientToolPrefix + clean
	if !used[base] {
		used[base] = true
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s_%d", base, suffix)
		if len(candidate) > toolWireNameLimit {
			candidate = candidate[:toolWireNameLimit]
		}
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
	}
}

// officialToolOrder is the key order of opencode_tools.json, made explicit
// because Go maps do not preserve it.
var officialToolOrder = []string{
	"edit", "execute", "glob", "grep", "question", "read", "shell",
	"skill", "subagent", "webfetch", "websearch", "write", "patch",
}

type officialToolDefinition struct {
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

func loadOfficialTools() map[string]officialToolDefinition {
	var tools map[string]officialToolDefinition
	if err := json.Unmarshal(opencodeToolsJSON, &tools); err != nil {
		return map[string]officialToolDefinition{}
	}
	return tools
}

func v2ShellDescription(base, platform string) string {
	switch platform {
	case "win32", "windows":
		return strings.Replace(base, "on Linux using bash", "on Windows using powershell", 1)
	case "darwin":
		return strings.Replace(base, "on Linux using bash", "on macOS using bash", 1)
	default:
		return base
	}
}

// renameAndCompleteTools renames client tool names to opaque digest names,
// appends the official agent toolset, and returns the renames needed to
// restore the client names in the response.
func renameAndCompleteTools(payload map[string]any, platform string) *toolRenames {
	renames := newToolRenames()
	used := map[string]bool{}
	runtimePlatform := platform
	if runtimePlatform == "" || runtimePlatform == "unknown" {
		runtimePlatform = "linux"
	}

	items, _ := payload["tools"].([]any)
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		function, ok := entry["function"].(map[string]any)
		if !ok {
			continue
		}
		name, _ := function["name"].(string)
		if name == "" {
			continue
		}
		wire := clientWireName(name, used)
		renames.add(name, wire)
		function["name"] = wire
	}

	// Append the official v2 tool set in canonical order.
	tools := loadOfficialTools()
	for _, name := range officialToolOrder {
		def, ok := tools[name]
		if !ok {
			def = officialToolDefinition{
				Description: "Agent tool " + name,
				Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
			}
		}
		description := def.Description
		if name == "shell" {
			description = v2ShellDescription(description, runtimePlatform)
		}
		parameters := def.Parameters
		if parameters == nil {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		items = append(items, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        name,
				"description": description,
				"parameters":  parameters,
				"strict":      false,
			},
		})
	}
	payload["tools"] = items

	// Rewrite tool_choice references to the renamed names.
	if choice, ok := payload["tool_choice"].(map[string]any); ok {
		switch choice["type"].(string) {
		case "function":
			if function, ok := choice["function"].(map[string]any); ok {
				if name, ok := function["name"].(string); ok {
					if wire, ok := renames.wireName(name); ok {
						function["name"] = wire
					}
				}
			}
		case "tool", "custom":
			if name, ok := choice["name"].(string); ok {
				if wire, ok := renames.wireName(name); ok {
					choice["name"] = wire
				}
			}
		}
	}

	// Rewrite tool_calls and function_call in the message history.
	droppedToolCallIDs := map[string]bool{}
	for _, rawMessage := range jsonSliceAt(payload, "messages") {
		message, _ := rawMessage.(map[string]any)
		if calls, ok := message["tool_calls"].([]any); ok {
			keptCalls := make([]any, 0, len(calls))
			for _, rawCall := range calls {
				call, _ := rawCall.(map[string]any)
				function, _ := call["function"].(map[string]any)
				name, _ := function["name"].(string)
				if wire, ok := renames.wireName(name); ok {
					function["name"] = wire
					keptCalls = append(keptCalls, call)
					continue
				}
				if isOfficialToolName(name) {
					keptCalls = append(keptCalls, call)
					continue
				}
				if toolCallID, _ := call["id"].(string); toolCallID != "" {
					droppedToolCallIDs[toolCallID] = true
				}
			}
			if len(keptCalls) == 0 {
				delete(message, "tool_calls")
			} else {
				message["tool_calls"] = keptCalls
			}
			continue
		}
		if call, ok := message["function_call"].(map[string]any); ok {
			if name, _ := call["name"].(string); name != "" {
				if wire, ok := renames.wireName(name); ok {
					call["name"] = wire
				}
			}
		}
	}
	// Drop tool messages that answered a dropped call.
	if len(droppedToolCallIDs) > 0 {
		var keptMessages []any
		for _, rawMessage := range jsonSliceAt(payload, "messages") {
			message, _ := rawMessage.(map[string]any)
			role, _ := message["role"].(string)
			if role == "tool" {
				if id, _ := message["tool_call_id"].(string); id != "" && droppedToolCallIDs[id] {
					continue
				}
			}
			keptMessages = append(keptMessages, rawMessage)
		}
		payload["messages"] = keptMessages
	}

	return renames
}

func isOfficialToolName(name string) bool {
	for _, official := range officialToolOrder {
		if name == official {
			return true
		}
	}
	return false
}

func jsonSliceAt(value any, key string) []any {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	list, ok := obj[key].([]any)
	if !ok {
		return nil
	}
	return list
}

func textFragments(value any) []string {
	switch typed := value.(type) {
	case string:
		return []string{typed}
	case []any:
		var parts []string
		for _, item := range typed {
			parts = append(parts, textFragments(item)...)
		}
		return parts
	case map[string]any:
		if text, ok := typed["text"].(string); ok {
			return textFragments(text)
		}
		if content, ok := typed["content"]; ok {
			return textFragments(content)
		}
	}
	return nil
}

func carriesOpenCodePrompt(value any) bool {
	for _, fragment := range textFragments(value) {
		if strings.Contains(fragment, opencodePromptMarker) {
			return true
		}
	}
	return false
}

// injectChatPrompt folds every system/developer message into one leading
// system message with the official CLI prompt prepended.
func injectChatPrompt(payload map[string]any, prompt string) bool {
	messages, ok := payload["messages"].([]any)
	if !ok {
		return false
	}
	kept := make([]any, 0, len(messages)+1)
	var instructions []string
	for _, raw := range messages {
		message, ok := raw.(map[string]any)
		if !ok {
			kept = append(kept, raw)
			continue
		}
		role, _ := message["role"].(string)
		if role != "system" && role != "developer" {
			kept = append(kept, raw)
			continue
		}
		if carriesOpenCodePrompt(message["content"]) {
			return false
		}
		for _, part := range textFragments(message["content"]) {
			if t := strings.TrimSpace(part); t != "" {
				instructions = append(instructions, t)
			}
		}
	}
	systemPrompt := prompt
	if len(instructions) > 0 {
		systemPrompt += "\n\n" + strings.Join(instructions, "\n")
	}
	payload["messages"] = append([]any{map[string]any{"role": "system", "content": systemPrompt}}, kept...)
	return true
}
