package colab

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestColabClientLifecycle(t *testing.T) {
	var uploadedContent string
	var proxyTokenSeen string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/runtimespecs":
			assert.Equal(t, "Bearer test-oauth-token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"runtimeSpecs":[{"variant":"VARIANT_GPU","accelerator":"NVIDIA_TESLA_T4"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/runtimes":
			body, _ := io.ReadAll(r.Body)
			assert.Contains(t, string(body), `"NVIDIA_L4"`)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"runtimes/rt-123","state":"RUNNING","connectionInfo":{"url":"` + "http://" + r.Host + `/proxy","token":"proxy-tok-1"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/proxy/api/contents/serve.py":
			proxyTokenSeen = r.Header.Get("X-Colab-Runtime-Proxy-Token")
			body, _ := io.ReadAll(r.Body)
			uploadedContent = string(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name":"serve.py"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/proxy/api/kernels":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"kern-42"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/runtimes/rt-123":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient("test-oauth-token")
	client.BaseURL = srv.URL

	specs, err := client.ListRuntimeSpecs()
	require.NoError(t, err)
	require.Len(t, specs.RuntimeSpecs, 1)

	rt, err := client.CreateRuntime("NVIDIA_L4")
	require.NoError(t, err)
	assert.Equal(t, "runtimes/rt-123", rt.Name)
	assert.Equal(t, "proxy-tok-1", rt.ConnectionInfo.Token)

	err = client.UploadFile(rt.ConnectionInfo.URL, rt.ConnectionInfo.Token, "serve.py", "print('ok')")
	require.NoError(t, err)
	assert.Equal(t, "proxy-tok-1", proxyTokenSeen)
	assert.Contains(t, uploadedContent, "print('ok')")

	kernelID, err := client.StartKernel(rt.ConnectionInfo.URL, rt.ConnectionInfo.Token)
	require.NoError(t, err)
	assert.Equal(t, "kern-42", kernelID)

	require.NoError(t, client.DeleteRuntime(rt.Name))
}
