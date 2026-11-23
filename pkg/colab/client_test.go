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
	var postAssignSawXSRF bool
	var postUnassignSawXSRF bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/ccu-info":
			assert.Equal(t, "Bearer test-oauth-token", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(")]}'\n" + `{"currentBalance":100,"eligibleGpus":["T4","L4"]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/assign":
			assert.Equal(t, "Bearer test-oauth-token", r.Header.Get("Authorization"))
			assert.Equal(t, "GPU", r.URL.Query().Get("variant"))
			assert.Equal(t, "L4", r.URL.Query().Get("accelerator"))
			assert.Regexp(t, `^[0-9a-f]{8}_[0-9a-f]{4}_[0-9a-f]{4}_[0-9a-f]{4}_[0-9a-f]{12}\.{8}$`, r.URL.Query().Get("nbh"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(")]}'\n" + `{"token":"xsrf-assign-1"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/assign":
			postAssignSawXSRF = r.Header.Get("X-Goog-Colab-Token") == "xsrf-assign-1"
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(")]}'\n" + `{"endpoint":"m-s-abc123","runtimeProxyInfo":{"url":"http://` + r.Host + `/proxy","token":"proxy-tok-1"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/proxy/api/contents/serve.py":
			proxyTokenSeen = r.Header.Get("X-Colab-Runtime-Proxy-Token")
			body, _ := io.ReadAll(r.Body)
			uploadedContent = string(body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"name":"serve.py"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/proxy/api/contents/endpoint_url.txt":
			assert.Equal(t, "proxy-tok-1", r.Header.Get("X-Colab-Runtime-Proxy-Token"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"endpoint_url.txt","content":"https://demo-colab.trycloudflare.com\n"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/proxy/api/kernels":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"kern-42"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/unassign/m-s-abc123":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(")]}'\n" + `{"token":"xsrf-unassign-1"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/unassign/m-s-abc123":
			postUnassignSawXSRF = r.Header.Get("X-Goog-Colab-Token") == "xsrf-unassign-1"
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(")]}'\n{}"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient("test-oauth-token")
	client.BaseURL = srv.URL
	client.TunnelBaseURL = srv.URL

	count, err := client.TestConnection()
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	rt, err := client.CreateRuntime("NVIDIA_L4")
	require.NoError(t, err)
	assert.True(t, postAssignSawXSRF)
	assert.Equal(t, "m-s-abc123", rt.Name)
	assert.Equal(t, "proxy-tok-1", rt.ConnectionInfo.Token)

	err = client.UploadFile(rt.ConnectionInfo.URL, rt.ConnectionInfo.Token, "serve.py", "print('ok')")
	require.NoError(t, err)
	assert.Equal(t, "proxy-tok-1", proxyTokenSeen)
	assert.Contains(t, uploadedContent, "print('ok')")

	epContent, err := client.ReadFile(rt.ConnectionInfo.URL, rt.ConnectionInfo.Token, "endpoint_url.txt")
	require.NoError(t, err)
	assert.Equal(t, "https://demo-colab.trycloudflare.com\n", epContent)

	kernelID, err := client.StartKernel(rt.ConnectionInfo.URL, rt.ConnectionInfo.Token)
	require.NoError(t, err)
	assert.Equal(t, "kern-42", kernelID)

	require.NoError(t, client.DeleteRuntime(rt.Name))
	assert.True(t, postUnassignSawXSRF)
}

func TestColabClientCPUAssignParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.URL.Query().Get("variant"))
		assert.Empty(t, r.URL.Query().Get("accelerator"))
		assert.Empty(t, r.URL.Query().Get("shape"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(")]}'\n" + `{"endpoint":"m-s-cpu","runtimeProxyInfo":{"url":"http://` + r.Host + `/proxy","token":"tok-cpu"}}`))
	}))
	defer srv.Close()

	client := NewClient("test-oauth-token")
	client.TunnelBaseURL = srv.URL

	rt, err := client.CreateRuntime("CPU")
	require.NoError(t, err)
	assert.Equal(t, "m-s-cpu", rt.Name)
}

func TestColabClientHighRAMOption(t *testing.T) {
	var seenShapes []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/assign" {
			seenShapes = append(seenShapes, r.URL.Query().Get("shape"))
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(")]}'\n" + `{"token":"xsrf-1"}`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/assign" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(")]}'\n" + `{"endpoint":"m-s-hm","runtimeProxyInfo":{"url":"http://` + r.Host + `/proxy","token":"tok"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient("test-oauth-token")
	client.TunnelBaseURL = srv.URL

	_, err := client.CreateRuntimeWithOptions("CPU", true)
	require.NoError(t, err)
	_, err = client.CreateRuntimeWithOptions("NVIDIA_TESLA_T4", false)
	require.NoError(t, err)
	require.Equal(t, []string{"hm", ""}, seenShapes)
}

func TestColabClientTPUSingleShapeAndFallback(t *testing.T) {
	var seenQueries []string
	rejectAll := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/assign" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodGet {
			assert.Empty(t, r.URL.Query().Get("shape"), "TPU must never send shape=hm")
			assert.Equal(t, "TPU", r.URL.Query().Get("variant"))
			acc := r.URL.Query().Get("accelerator")
			seenQueries = append(seenQueries, acc)
			if rejectAll || acc == "V5E1" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("<!DOCTYPE html><html><title>Error 400 (Bad Request)!!1</title></html>"))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(")]}'\n" + `{"token":"xsrf-tpu"}`))
			return
		}
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(")]}'\n" + `{"endpoint":"m-s-tpu","runtimeProxyInfo":{"url":"http://` + r.Host + `/proxy","token":"tok-tpu"}}`))
			return
		}
	}))
	defer srv.Close()

	client := NewClient("test-oauth-token")
	client.TunnelBaseURL = srv.URL

	rt, err := client.CreateRuntimeWithOptions("TPU_V5E_1", true)
	require.NoError(t, err)
	assert.Equal(t, "m-s-tpu", rt.Name)
	assert.Equal(t, []string{"V5E1", "V6E1"}, seenQueries)

	rejectAll = true
	seenQueries = nil
	_, err = client.CreateRuntimeWithOptions("TPU_V5E_1", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bad request")
	assert.NotContains(t, err.Error(), "<!DOCTYPE html>")
	assert.Equal(t, []string{"V5E1", "V6E1", "V28", ""}, seenQueries)
}
