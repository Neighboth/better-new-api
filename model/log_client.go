package model

import (
	"net/http"
	"strings"
	"unicode"
)

// requestClientName extracts an application name, not a version or a raw
// user agent. Headers are client supplied and must never be treated as proof
// of identity. X-Title is used by OpenRouter-compatible clients such as Roo.
func requestClientName(header http.Header) string {
	userAgent := strings.ToLower(header.Get("User-Agent"))
	known := []struct{ marker, name string }{
		{"claude-code", "Claude Code"}, {"claude-cli", "Claude Code"},
		{"roocode", "Roo Code"}, {"roo-code", "Roo Code"},
		{"codex_cli_rs", "Codex"}, {"codex-cli", "Codex"},
		{"opencode", "OpenCode"}, {"cline", "Cline"},
		{"cursor", "Cursor"}, {"aider", "Aider"},
		{"continue", "Continue"},
	}
	// A specific product UA is more reliable than an arbitrary title.
	for _, item := range known {
		if strings.Contains(userAgent, item.marker) {
			return item.name
		}
	}
	title := strings.TrimSpace(header.Get("X-Title"))
	if title == "" {
		return ""
	}
	// Do not persist control characters or unbounded user-supplied strings.
	clean := make([]rune, 0, 80)
	for _, r := range title {
		if unicode.IsControl(r) {
			continue
		}
		if len(clean) == 80 {
			break
		}
		clean = append(clean, r)
	}
	return strings.TrimSpace(string(clean))
}
