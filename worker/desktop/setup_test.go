package main

// The first start of the client is the contract of the installation: the user
// chooses the language and accepts the policies before the client is used. The
// page of the first start is served by the client itself, the test checks the
// answer of the page and the saved answer of the user.

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupPage(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BYOG_STORAGE_DIR", root)

	State = &WorkerState{Config: Config{
		ServerUrl: "http://127.0.0.1:1",
		AppName:   "Example",
	}}
	bridge := &localBridge{address: "http://127.0.0.1:2", token: "token"}

	recorder := httptest.NewRecorder()
	bridge.handleSetupPage(recorder, httptest.NewRequest(http.MethodGet, "/?lang=fr", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	assert.Contains(t, body, "Türkçe")
	assert.Contains(t, body, "简体中文")
	assert.Contains(t, body, `name="privacy"`)
	assert.Contains(t, body, `name="terms"`)
	assert.Contains(t, body, `<option value="fr" selected>`)

	form := url.Values{"language": {"tr"}, "privacy": {"1"}, "terms": {"1"}}
	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	bridge.handleSetupPage(recorder, request)
	require.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.Equal(t, "tr", State.Config.Language)
	assert.True(t, State.Config.LegalAccepted)

	data, err := os.ReadFile(filepath.Join(root, "config.json"))
	require.NoError(t, err)
	assert.Contains(t, string(data), `"language": "tr"`)
	assert.Contains(t, string(data), `"legal_accepted": true`)

	// The page of the installation is shown after the language and the
	// policies are accepted.
	recorder = httptest.NewRecorder()
	bridge.handleSetupPage(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.True(t, strings.HasPrefix(recorder.Header().Get("Location"), "http://127.0.0.1:1/client"))
}

func TestSetupPageRejectsUnacceptedPolicies(t *testing.T) {
	root := t.TempDir()
	t.Setenv("BYOG_STORAGE_DIR", root)

	State = &WorkerState{Config: Config{ServerUrl: "http://127.0.0.1:1"}}
	bridge := &localBridge{address: "http://127.0.0.1:2", token: "token"}

	form := url.Values{"language": {"en"}, "privacy": {"1"}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	bridge.handleSetupPage(recorder, request)

	require.Equal(t, http.StatusSeeOther, recorder.Code)
	assert.False(t, State.Config.LegalAccepted)
	assert.Contains(t, recorder.Header().Get("Location"), "lang=en")
}
