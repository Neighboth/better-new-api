package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitTranslateSegmentsProtectsCodeAndPlaceholders(t *testing.T) {
	text := "Use `gpt-4o` with {{count}} requests. See https://api.example.com/v1 for details.\n\n```\nconst x = 1\n```"
	segments := splitTranslateSegments(text)
	var protected []string
	for _, seg := range segments {
		if seg.protected {
			protected = append(protected, seg.text)
		}
	}
	joined := strings.Join(protected, "|")
	assert.Contains(t, joined, "`gpt-4o`")
	assert.Contains(t, joined, "{{count}}")
	assert.Contains(t, joined, "https://api.example.com/v1")
	assert.Contains(t, joined, "```")

	var rebuilt strings.Builder
	for _, seg := range segments {
		rebuilt.WriteString(seg.text)
	}
	assert.Equal(t, text, rebuilt.String(), "segments must reassemble into the original text")
}

func TestIsTranslatableSkipsTechnicalTokens(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{"gpt-4o-mini", false},
		{"claude-sonnet-4-20250514", false},
		{"v1/chat/completions", false},
		{"https://api.example.com", false},
		{"v1.2.3", false},
		{"   ", false},
		{"12345", false},
		{"Hello world", true},
		{"This model is fast", true},
		{"Merhaba dünya", true},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, isTranslatable(tc.text), "text: %q", tc.text)
	}
}

func TestSplitLongTextRespectsLimitAndBoundaries(t *testing.T) {
	var paragraphs []string
	for i := 0; i < 20; i++ {
		paragraphs = append(paragraphs, strings.Repeat("word ", 60))
	}
	text := strings.Join(paragraphs, "\n\n")
	chunks := splitLongText(text, 1500)
	require.Greater(t, len(chunks), 1)
	for _, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk), 1500)
	}
	assert.Equal(t, text, strings.Join(chunks, ""))
}

func TestTranslateTextSameLanguagePassthrough(t *testing.T) {
	assert.Equal(t, "Hello", TranslateText("en", "en", "Hello"))
	assert.Equal(t, "", TranslateText("en", "tr", ""))
}
