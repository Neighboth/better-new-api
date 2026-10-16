package controller

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The bundler can inject its own favicon link on top of the one in the
// template; browsers honor the last rel=icon, so RenderIndexPage must rewrite
// every occurrence — otherwise the default icon wins over the site's.
func TestReplaceAllLinkHrefsRewritesEveryIconLink(t *testing.T) {
	page := `<head>` +
		`<link rel="icon" type="image/png" href="/logo.png" />` +
		`<link rel="stylesheet" href="/app.css" />` +
		`<link rel="icon" href="/favicon.ico">` +
		`</head>`

	got := replaceAllLinkHrefs(page, `rel="icon"`, "https://cdn.example.com/icon.png")

	assert.Equal(t, 2, strings.Count(got, "https://cdn.example.com/icon.png"))
	assert.NotContains(t, got, "/favicon.ico")
	assert.NotContains(t, got, `href="/logo.png"`)
	assert.Contains(t, got, `href="/app.css"`)
	// Idempotent: running it again changes nothing further.
	assert.Equal(t, got, replaceAllLinkHrefs(got, `rel="icon"`, "https://cdn.example.com/icon.png"))
}
