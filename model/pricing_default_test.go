package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchesKeyword(t *testing.T) {
	cases := []struct {
		name     string
		haystack string
		needle   string
		want     bool
	}{
		{"exact match at start", "kling-v2", "kling", true},
		{"match after separator", "meta/kling-2.1", "kling", true},
		{"match after underscore", "some_kling_model", "kling", true},
		{"match in middle of name", "claude-sonnet", "claude", true},
		{"short pattern inside longer token", "sao10k/l3-lunaris-8b", "o1", false},
		{"pattern inside word", "thinkingmachines/inkling", "kling", false},
		// a hyphen is a token boundary, so this does match; "muse-spark-open" is
		// only kept off the iFlytek vendor by the higher-priority prefix rules
		{"pattern after hyphen is a boundary", "muse-spark-open", "spark", true},
		{"pattern absent", "qwen3-8b", "claude", false},
		{"empty needle never matches", "openai/gpt-4o", "", false},
		{"empty haystack", "", "gpt", false},
		{"digit boundary", "gpt-4o-mini", "o1", false},
		{"token at end of string", "z-ai/glm-5.2", "glm-", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, matchesKeyword(tc.haystack, tc.needle))
		})
	}
}
