// Portions adapted from github.com/jasonxu114514/opencode2api (GPL-3.0) by
// jasonxu114514. The embedded prompt text and tool vocabulary are the official
// OpenCode CLI's own assets; they are required for the upstream free lane to
// accept requests. See THIRD-PARTY-LICENSES.md.
package opencode

import (
	_ "embed"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

//go:embed opencode_prompt_v2.txt
var opencodePromptV2 string

//go:embed opencode_tools.json
var opencodeToolsJSON []byte

const opencodeSkills = `Skills provide specialized instructions and workflows for specific tasks.
Use the skill tool to load a skill when a task matches its description.
<available_skills>
  <skill>
    <name>customize-opencode</name>
    <description>Use ONLY when the user is editing or creating opencode's own configuration: opencode.json, opencode.jsonc, files under .opencode/, or files under ~/.config/opencode/. Also use when creating or fixing opencode agents, subagents, skills, plugins, MCP servers, or permission rules. Do not use for the user's own application code, or for any project that is not configuring opencode itself.</description>
    <location>&lt;built-in&gt;</location>
  </skill>
</available_skills>`

// opencodeEnvironment renders the <env> block the CLI puts after its base prompt.
func opencodeEnvironment(model string) string {
	apiModel := strings.TrimSpace(strings.TrimPrefix(model, "opencode/"))
	if apiModel == "" {
		apiModel = "unknown"
	}
	return strings.Join([]string{
		"You are powered by the model named " + apiModel + ". The exact model ID is opencode/" + apiModel,
		"Here is some useful information about the environment you are running in:",
		"<env>",
		"  Working directory: /workspace",
		"  Workspace root folder: /workspace",
		"  Is directory a git repo: yes",
		"  Platform: linux",
		"  Today's date: " + time.Now().UTC().Format("Mon Jan 02 2006"),
		"</env>",
	}, "\n")
}

// officialSystemPrompt assembles the full CLI system prompt for model.
func officialSystemPrompt(model string) string {
	return opencodePromptV2 + opencodeEnvironment(model) + "\n" + opencodeSkills
}

// ShapeChatRequest injects the official OpenCode CLI prompt into a Chat
// Completions body (unless it already carries the official prompt), folds all
// system/developer messages into one leading system message, then renames
// client tools and appends the official tool set. Returns the renames needed
// to restore the client names in the response.
func ShapeChatRequest(payload map[string]any) *toolRenames {
	model, _ := payload["model"].(string)
	injectChatPrompt(payload, officialSystemPrompt(model))
	return renameAndCompleteTools(payload, "linux")
}

// RestoreChatResponse maps the wire tool names in a non-streaming Chat
// Completions response back to the names the client declared.
func RestoreChatResponse(body []byte, renames *toolRenames) []byte {
	if renames == nil || renames.empty() {
		return body
	}
	var decoded map[string]any
	if err := common.Unmarshal(body, &decoded); err != nil {
		return body
	}
	for _, rawChoice := range jsonSliceAt(decoded, "choices") {
		choice, _ := rawChoice.(map[string]any)
		message, _ := choice["message"].(map[string]any)
		restoreToolCalls(message, renames)
	}
	out, err := common.Marshal(decoded)
	if err != nil {
		return body
	}
	return out
}

func restoreToolCalls(message map[string]any, renames *toolRenames) {
	for _, rawCall := range jsonSliceAt(message, "tool_calls") {
		call, _ := rawCall.(map[string]any)
		function, _ := call["function"].(map[string]any)
		if name, _ := function["name"].(string); name != "" {
			function["name"] = renames.restore(name)
		}
	}
}

// RestoreChatStreamLine rewrites one SSE `data:` line if it decodes as a Chat
// Completions stream chunk; every other line passes through untouched.
func RestoreChatStreamLine(line []byte, renames *toolRenames) []byte {
	if renames == nil || renames.empty() {
		return line
	}
	trimmed := string(line)
	const prefix = "data:"
	idx := strings.Index(trimmed, prefix)
	if idx == -1 {
		return line
	}
	payload := strings.TrimSpace(trimmed[idx+len(prefix):])
	if payload == "[DONE]" {
		return line
	}
	var decoded map[string]any
	if err := common.Unmarshal([]byte(payload), &decoded); err != nil {
		return line
	}
	choices, ok := decoded["choices"].([]any)
	if !ok {
		return line
	}
	changed := false
	for _, rawChoice := range choices {
		choice, _ := rawChoice.(map[string]any)
		delta, _ := choice["delta"].(map[string]any)
		for _, rawCall := range jsonSliceAt(delta, "tool_calls") {
			call, _ := rawCall.(map[string]any)
			function, _ := call["function"].(map[string]any)
			name, _ := function["name"].(string)
			if name == "" {
				continue
			}
			restored := renames.restore(name)
			if restored != name {
				function["name"] = restored
				changed = true
			}
		}
	}
	if !changed {
		return line
	}
	out, err := common.Marshal(decoded)
	if err != nil {
		return line
	}
	return append([]byte("data: "), append(out, '\n')...)
}
