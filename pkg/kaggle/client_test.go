package kaggle

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupAccelerator(t *testing.T) {
	cpu := LookupAccelerator("CPU")
	assert.False(t, cpu.EnableGPU)
	assert.False(t, cpu.EnableTPU)
	assert.Empty(t, cpu.MachineShape)

	t4 := LookupAccelerator("NvidiaTeslaT4")
	assert.True(t, t4.EnableGPU)
	assert.False(t, t4.EnableTPU)
	assert.Equal(t, "NvidiaTeslaT4", t4.MachineShape)

	tpu := LookupAccelerator("TpuV5E8")
	assert.False(t, tpu.EnableGPU)
	assert.True(t, tpu.EnableTPU)
	assert.Equal(t, "TpuV5E8", tpu.MachineShape)
}

func TestKaggleClientLifecycle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		assert.Equal(t, "alice", user)
		assert.Equal(t, "secret-key", pass)

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/kernels/list":
			assert.Equal(t, "mine", r.URL.Query().Get("group"))
			assert.Empty(t, r.URL.Query().Get("mine"))
			_, _ = w.Write([]byte(`[{"ref":"alice/test"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/kernels/push":
			body, _ := io.ReadAll(r.Body)
			assert.Contains(t, string(body), `"enableTpu":true`)
			assert.Contains(t, string(body), `"machineShape":"TpuV5E8"`)
			_, _ = w.Write([]byte(`{"ref":"alice/serve-qwen","url":"https://www.kaggle.com/code/alice/serve-qwen","versionNumber":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/kernels/status":
			_, _ = w.Write([]byte(`{"slug":"serve-qwen","status":"running"}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/kernels/alice/serve-qwen":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient("alice", "secret-key")
	client.BaseURL = srv.URL

	list, err := client.TestConnection()
	require.NoError(t, err)
	require.Len(t, list, 1)

	pushed, err := client.PushKernel(PushKernelRequest{
		Slug:        "alice/serve-qwen",
		Title:       "Serve Qwen",
		ScriptText:  "print('hello')",
		Accelerator: "TpuV5E8",
		IsPrivate:   true,
	})
	require.NoError(t, err)
	assert.Equal(t, "alice/serve-qwen", pushed.Ref)

	st, err := client.GetKernelStatus("alice", "serve-qwen")
	require.NoError(t, err)
	assert.Equal(t, "running", st.Status)

	require.NoError(t, client.DeleteKernel("alice", "serve-qwen"))
}

func TestKaggleBearerTokenAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer KGAT_abc123", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	client := NewClient("alice", "KGAT_abc123")
	client.BaseURL = srv.URL
	_, err := client.TestConnection()
	require.NoError(t, err)
}
