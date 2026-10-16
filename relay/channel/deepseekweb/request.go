package deepseekweb

import (
	"context"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

// attachment is one caller supplied file that has to be mirrored into the
// DeepSeek account before the completion can reference it.
type attachment struct {
	name   string
	source types.FileSource
}

// buildPrompt flattens an OpenAI style conversation into the single prompt the
// web endpoint accepts. DeepSeek Web keeps conversation state server side, so a
// stateless relay has to resend the whole history as text.
func buildPrompt(messages []dto.Message) string {
	var builder strings.Builder
	for _, message := range messages {
		text := strings.TrimSpace(message.StringContent())
		if text == "" {
			continue
		}
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(roleLabel(message.Role))
		builder.WriteString(": ")
		builder.WriteString(text)
	}
	return strings.TrimSpace(builder.String())
}

func roleLabel(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "system", "developer":
		return "System"
	case "assistant":
		return "Assistant"
	case "tool", "function":
		return "Tool"
	default:
		return "User"
	}
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
