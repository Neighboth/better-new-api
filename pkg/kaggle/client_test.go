package kaggle

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLookupAccelerator(t *testing.T) {
	cpu := LookupAccelerator("CPU")
	assert.False(t, cpu.EnableGPU)
	assert.False(t, cpu.EnableTPU)
	assert.Empty(t, cpu.MachineShape)

	t4 := LookupAccelerator("NvidiaP100")
	assert.True(t, t4.EnableGPU)
	assert.False(t, t4.EnableTPU)
	assert.Equal(t, "NvidiaTeslaT4", t4.MachineShape)

	tpu := LookupAccelerator("TpuV5E8")
	assert.False(t, tpu.EnableGPU)
	assert.True(t, tpu.EnableTPU)
	assert.Equal(t, "TpuV5E8", tpu.MachineShape)

	tpuLegacy := LookupAccelerator("TpuV38")
	assert.True(t, tpuLegacy.EnableTPU)
	assert.Equal(t, "TpuV5E8", tpuLegacy.MachineShape)
}

func TestKaggleClientLifecycle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		assert.Equal(t, "alice", user)
		assert.Equal(t, "secret-key", pass)

		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/kernels/list":
			assert.Equal(t, "profile", r.URL.Query().Get("group"))
			assert.Empty(t, r.URL.Query().Get("mine"))
			_, _ = w.Write([]byte(`[{"ref":"alice/test"}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/kernels.KernelsApiService/SaveKernel":
			body, _ := io.ReadAll(r.Body)
			assert.Contains(t, string(body), `"enableTpu":true`)
			assert.Contains(t, string(body), `"machineShape":"TpuV5E8"`)
			assert.Contains(t, string(body), `"kernelExecutionType":"SAVE_AND_RUN_ALL"`)
			_, _ = w.Write([]byte(`{"ref":"alice/serve-qwen","url":"https://www.kaggle.com/code/alice/serve-qwen","versionNumber":1}`))
		case r.Method == http.MethodPost && r.URL.Path == "/kernels.KernelsApiService/GetKernelSessionStatus":
			_, _ = w.Write([]byte(`{"slug":"serve-qwen","status":"RUNNING"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/kernels.KernelsApiService/DeleteKernel":
			body, _ := io.ReadAll(r.Body)
			assert.Contains(t, string(body), `"userName":"alice"`)
			assert.Contains(t, string(body), `"kernelSlug":"serve-qwen"`)
			_, _ = w.Write([]byte(`{}`))
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

func TestStreamKernelLogsSSEAndBlob(t *testing.T) {
	t.Run("SSE", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/kernels/logs/stream/alice/serve-qwen", r.URL.Path)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"stream_name\":\"stdout\",\"data\":\"line 1\\nline 2\"}\n\n"))
			_, _ = w.Write([]byte("data: END_OF_LOG\n\n"))
		}))
		defer srv.Close()

		client := NewClient("alice", "KGAT_abc123")
		client.BaseURL = srv.URL
		var got []string
		err := client.StreamKernelLogs(context.Background(), "alice", "serve-qwen", func(line string) {
			got = append(got, line)
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"line 1", "line 2"}, got)
	})

	t.Run("JSONBlob", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"data":"booting\n"},{"data":"ready"}]`))
		}))
		defer srv.Close()

		client := NewClient("alice", "KGAT_abc123")
		client.BaseURL = srv.URL
		var got []string
		err := client.StreamKernelLogs(context.Background(), "alice", "serve-qwen", func(line string) {
			got = append(got, line)
		})
		require.NoError(t, err)
		assert.Equal(t, "booting\nready", strings.Join(got, "\n"))
	})
}
