package main

// The first start of the client is the contract of the installation: the user
// chooses the language and accepts the policies before the client is used. The
// page of the first start is served by the client itself, the test checks the
// answer of the page and the saved answer of the user.

import (
	"net/http"
	"net/http/httptest"
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
	require.Equal(t, http.StatusSeeOther, recorder.Code)
	loc := recorder.Header().Get("Location")
	assert.True(t, strings.HasPrefix(loc, "http://127.0.0.1:1/client"))
	assert.Contains(t, loc, "local=http://127.0.0.1:2")
	assert.Contains(t, loc, "token=token")
	assert.True(t, State.Config.LegalAccepted)
}
