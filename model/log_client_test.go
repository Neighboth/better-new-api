package model

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestClientName(t *testing.T) {
	cases := []struct{ ua, title, want string }{
		{"claude-code/2.1.45", "", "Claude Code"},
		{"claude-cli/1.0", "", "Claude Code"},
		{"RooCode/3.5", "", "Roo Code"},
		{"codex_cli_rs/1", "", "Codex"},
		{"node", "Roo Code", "Roo Code"},
		{"python/3", "", ""},
		{"", " abc\nxyz ", "abcxyz"},
		{"", strings.Repeat("a", 90), strings.Repeat("a", 80)},
		{"claude-code/2", "Misleading title", "Claude Code"},
	}
	for _, tc := range cases {
		h := http.Header{}
		h.Set("User-Agent", tc.ua)
		h.Set("X-Title", tc.title)
		assert.Equal(t, tc.want, requestClientName(h))
	}
}
