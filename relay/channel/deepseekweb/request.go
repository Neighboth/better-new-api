package deepseekweb

import (
	"context"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// attachment is one caller supplied file that has to be mirrored into the
// DeepSeek account before the completion can reference it.
type attachment struct {
	name   string
	source types.FileSource
}

// buildPrompt flattens an OpenAI style request into the single prompt the
// web endpoint accepts, preserving system instructions, response_format,
// tool definitions, and multi-turn tool call history.
func buildPrompt(req *dto.GeneralOpenAIRequest) string {
	if req == nil {
		return ""
	}

	var sysParts []string
	if instr := strings.TrimSpace(req.Instruction); instr != "" {
		sysParts = append(sysParts, instr)
	}

	var convParts []string
	for _, message := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		text := strings.TrimSpace(message.StringContent())

		if role == "system" || role == "developer" {
			if text != "" {
				sysParts = append(sysParts, text)
			}
			continue
		}

		if role == "assistant" {
			var assistantParts []string
			if text != "" {
				assistantParts = append(assistantParts, text)
			}
			if len(message.ToolCalls) > 0 {
				for _, tc := range message.ParseToolCalls() {
					callPayload := map[string]any{
						"id":        tc.ID,
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					}
					if raw, err := common.Marshal(callPayload); err == nil {
						assistantParts = append(assistantParts, "<tool_call>\n"+string(raw)+"\n</tool_call>")
					}
				}
			}
			if len(assistantParts) > 0 {
				convParts = append(convParts, "Assistant: "+strings.Join(assistantParts, "\n"))
			}
			continue
		}

		if role == "tool" || role == "function" {
			label := "Tool Result"
			if message.ToolCallId != "" {
				label = fmt.Sprintf("Tool Result (tool_call_id=%s)", message.ToolCallId)
			} else if message.Name != nil && strings.TrimSpace(*message.Name) != "" {
				label = fmt.Sprintf("Tool Result (name=%s)", strings.TrimSpace(*message.Name))
			}
			if text == "" {
				text = "{}"
			}
			convParts = append(convParts, label+": "+text)
			continue
		}

		if text != "" {
			convParts = append(convParts, "User: "+text)
		}
	}

	if req.ResponseFormat != nil {
		switch strings.ToLower(strings.TrimSpace(req.ResponseFormat.Type)) {
		case "json_object":
			sysParts = append(sysParts, "You must respond with valid JSON only, with no surrounding markdown or commentary.")
		case "json_schema":
			if len(req.ResponseFormat.JsonSchema) > 0 {
				sysParts = append(sysParts, "You must respond with valid JSON matching this schema:\n"+string(req.ResponseFormat.JsonSchema))
			} else {
				sysParts = append(sysParts, "You must respond with valid JSON only.")
			}
		}
	}

	if len(req.Tools) > 0 && !isToolChoiceNone(req.ToolChoice) {
		sysParts = append(sysParts, buildToolsInstruction(req.Tools, req.ToolChoice))
	}

	var sections []string
	if len(sysParts) > 0 {
		sections = append(sections, "<system_instructions>\n"+strings.Join(sysParts, "\n\n")+"\n</system_instructions>")
	}
	if len(convParts) > 0 {
		sections = append(sections, strings.Join(convParts, "\n\n"))
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func buildToolsInstruction(tools []dto.ToolCallRequest, toolChoice any) string {
	type toolSpec struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters,omitempty"`
	}
	specs := make([]toolSpec, 0, len(tools))
	for _, t := range tools {
		if strings.TrimSpace(t.Function.Name) == "" {
			continue
		}
		specs = append(specs, toolSpec{
			Name:        t.Function.Name,
			Description: t.Function.Description,
			Parameters:  t.Function.Parameters,
		})
	}
	toolsJSON, err := common.Marshal(specs)
	if err != nil {
		toolsJSON = []byte("[]")
	}

	var sb strings.Builder
	sb.WriteString("<tools>\n")
	sb.Write(toolsJSON)
	sb.WriteString("\n</tools>\n")
	sb.WriteString("To call a tool, respond ONLY with one or more <tool_call> blocks in this exact format:\n")
	sb.WriteString("<tool_call>\n{\"name\": \"tool_name\", \"arguments\": {\"arg\": \"value\"}}\n</tool_call>\n")

	if forcedName := extractForcedToolName(toolChoice); forcedName != "" {
		sb.WriteString(fmt.Sprintf("You MUST call the tool %q.", forcedName))
	} else if isToolChoiceRequired(toolChoice) {
		sb.WriteString("You MUST call at least one tool from <tools>.")
	} else {
		sb.WriteString("Call a tool when needed; otherwise answer the user directly without <tool_call> tags.")
	}
	return sb.String()
}

// collectAttachments gathers every image and document the caller attached,
// de-duplicated so a repeated image in the history is uploaded only once.
func collectAttachments(messages []dto.Message) []attachment {
	var out []attachment
	seen := make(map[string]bool)

	for _, message := range messages {
		for _, media := range message.ParseContent() {
			var image *dto.MessageImageUrl
			switch media.Type {
			case dto.ContentTypeImageURL:
				image = media.GetImageMedia()
				if image == nil || image.Url == "" {
					continue
				}
			case dto.ContentTypeFile:
				file := media.GetFile()
				if file == nil || file.FileData == "" {
					continue
				}
			default:
				continue
			}

			source := media.ToFileSource()
			if source == nil || seen[source.GetIdentifier()] {
				continue
			}
			seen[source.GetIdentifier()] = true

			name := ""
			if file := media.GetFile(); file != nil && file.FileName != "" {
				name = file.FileName
			}
			if name == "" {
				name = attachmentName(len(out), image)
			}
			out = append(out, attachment{name: name, source: source})
		}
	}
	return out
}

// uploadAttachments mirrors every attachment into the DeepSeek account and
// returns the file ids the completion request must reference.
func uploadAttachments(ctx context.Context, client *webClient, items []attachment) ([]string, error) {
	if len(items) == 0 {
		return nil, nil
	}

	fileIDs := make([]string, 0, len(items))
	for index, item := range items {
		data, mimeType, err := readAttachment(ctx, item.source)
		if err != nil {
			return nil, err
		}
		name := item.name
		if name == "" {
			name = attachmentName(index, nil)
		}
		fileID, err := client.uploadAttachment(ctx, name, mimeType, data)
		if err != nil {
			return nil, err
		}
		fileIDs = append(fileIDs, fileID)
	}
	return fileIDs, nil
}

// isThinkingModel reports whether the upstream model should run with the
// reasoning (THINK) fragments enabled.
func isThinkingModel(modelName string) bool {
	lower := strings.ToLower(modelName)
	return strings.Contains(lower, "reasoner") || strings.Contains(lower, "r1") || strings.Contains(lower, "-think")
}
