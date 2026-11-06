package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerBridgeUsesPrivateCredentialAndOwnNode(t *testing.T) {
	var upstreamAuth, upstreamNode string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamAuth = r.Header.Get("Authorization")
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		upstreamNode, _ = body["node_id"].(string)
		_, _ = io.WriteString(w, `{"success":true}`)
	}))
	defer server.Close()

	State.Mu.Lock()
	oldConfig := State.Config
	State.Config = Config{ServerUrl: server.URL, NodeId: "own-node", WorkerSecret: "private-token"}
	State.Mu.Unlock()
	t.Cleanup(func() { State.Mu.Lock(); State.Config = oldConfig; State.Mu.Unlock() })

	bridge := &localBridge{token: "bridge-token"}
	denied := httptest.NewRecorder()
	bridge.handleWorkerProxy(denied, httptest.NewRequest(http.MethodPost, "/worker/resources", strings.NewReader(`{}`)))
	assert.Empty(t, upstreamAuth)

	request := httptest.NewRequest(http.MethodPost, "/worker/resources", strings.NewReader(`{"node_id":"another-node","allocated_cores":1}`))
	request.Header.Set("X-Client-Key", "bridge-token")
	response := httptest.NewRecorder()
	bridge.handleWorkerProxy(response, request)
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "Bearer private-token", upstreamAuth)
	assert.Equal(t, "own-node", upstreamNode)
	assert.NotContains(t, response.Body.String(), "private-token")
}
