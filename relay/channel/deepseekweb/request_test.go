package deepseekweb

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPromptFlattensConversation(t *testing.T) {
	prompt := buildPrompt([]dto.Message{
		{Role: "system", Content: "Be terse."},
		{Role: "user", Content: "Hi"},
		{Role: "assistant", Content: "Hello"},
		{Role: "user", Content: []any{
			map[string]any{"type": "text", "text": "What is this?"},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "http://example.com/a.png"}},
		}},
	})

	assert.Equal(t, "System: Be terse.\n\nUser: Hi\n\nAssistant: Hello\n\nUser: What is this?", prompt)
}

func TestBuildPromptSkipsEmptyMessages(t *testing.T) {
	prompt := buildPrompt([]dto.Message{
		{Role: "user", Content: "  "},
		{Role: "user"},
		{Role: "assistant", Content: "answer"},
	})
	assert.Equal(t, "Assistant: answer", prompt)
}

func TestCollectAttachmentsDeduplicatesAndNames(t *testing.T) {
	dataURI := "data:image/png;base64,iVBORw0KGgo="
	messages := []dto.Message{
		{Role: "user", Content: []any{
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": dataURI}},
		}},
		{Role: "user", Content: []any{
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": dataURI}},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.com/photo.jpg"}},
			map[string]any{"type": "input_audio", "input_audio": map[string]any{"data": "AAAA", "format": "wav"}},
		}},
	}

	attachments := collectAttachments(messages)
	require.Len(t, attachments, 2, "the repeated image must be uploaded once and audio must be ignored")
	assert.Equal(t, "attachment-1.png", attachments[0].name)
	assert.Equal(t, "attachment-2.jpg", attachments[1].name)
}

func TestCollectAttachmentsUsesCallerFileName(t *testing.T) {
	messages := []dto.Message{{Role: "user", Content: []any{
		map[string]any{"type": "file", "file": map[string]any{"filename": "report.pdf", "file_data": "data:application/pdf;base64,AAAA"}},
	}}}

	attachments := collectAttachments(messages)
	require.Len(t, attachments, 1)
	assert.Equal(t, "report.pdf", attachments[0].name)
}

func TestCollectAttachmentsIgnoresMalformedParts(t *testing.T) {
	messages := []dto.Message{{Role: "user", Content: []any{
		map[string]any{"type": "image_url"},
		map[string]any{"type": "image_url", "image_url": map[string]any{"url": ""}},
		map[string]any{"type": "file", "file": map[string]any{"filename": "x.pdf"}},
	}}}

	assert.Empty(t, collectAttachments(messages))
}

func TestAttachmentNameFromMimeAndURL(t *testing.T) {
	assert.Equal(t, "attachment-1.jpg", attachmentName(0, &dto.MessageImageUrl{Url: "x", MimeType: "image/jpeg"}))
	assert.Equal(t, "attachment-2.gif", attachmentName(1, &dto.MessageImageUrl{Url: "https://e.com/a.gif"}))
	assert.Equal(t, "attachment-3.pdf", attachmentName(2, &dto.MessageImageUrl{Url: "https://e.com/a.pdf?v=2"}))
	assert.Equal(t, "attachment-4.png", attachmentName(3, &dto.MessageImageUrl{Url: "https://e.com/unknown"}))
}

func TestEscapeFilenameStripsHeaderBreakers(t *testing.T) {
	assert.Equal(t, "ab.txt", escapeFilename("a\"b\r\n.txt"))
}

func TestIsThinkingModel(t *testing.T) {
	assert.True(t, isThinkingModel("deepseek-reasoner"))
	assert.True(t, isThinkingModel("deepseek-r1"))
	assert.False(t, isThinkingModel("deepseek-chat"))
}

func TestResolveBaseURL(t *testing.T) {
	assert.Equal(t, DefaultBaseURL, resolveBaseURL(""))
	assert.Equal(t, DefaultBaseURL, resolveBaseURL("   "))
	assert.Equal(t, "https://mirror.example.com", resolveBaseURL("https://mirror.example.com/"))
}
