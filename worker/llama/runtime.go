// Package llama manages the llama.cpp runtime and the local inference.
//
// The worker runs the inference with the real llama.cpp engine. The runtime is either
// provided by the operator next to the worker binary or downloaded from the official
// llama.cpp release. Every answer is produced by the model itself, the worker never
// fabricates content or throughput numbers.
//
// The worker runs the inference with the real llama.cpp engine. The runtime is either
// provided by the operator next to the worker binary or downloaded from the official
// llama.cpp release. Every answer is produced by the model itself, the worker never
// fabricates content or throughput numbers.

package llama

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// llamaReleaseTag is the llama.cpp release the worker downloads when no runtime is
// available on the machine.
const llamaReleaseTag = "b5231"

// Runtime holds the paths of the llama.cpp binaries.
type Runtime struct {
	Dir        string
	ServerPath string
	CliPath    string
}

// Server is a running llama.cpp server for a single model.
type Server struct {
	cmd      *exec.Cmd
	port     int
	model    string
	started  time.Time
	mutex    sync.Mutex
	stopping bool
}

var (
	runtimeOnce sync.Once
	runtimePath *Runtime
	runtimeErr  error

	serverMutex sync.Mutex
	servers     = map[string]*Server{}
)

// llamaRuntimeDirectory returns the directory the runtime is installed in.
func llamaRuntimeDirectory(storageDir string) string {
	return filepath.Join(storageDir, "llama-runtime")
}

// EnsureRuntime makes sure that the llama.cpp binaries are available.
func EnsureRuntime(storageDir string) (*Runtime, error) {
	runtimeOnce.Do(func() {
		runtimePath, runtimeErr = locateOrInstallLlamaRuntime(storageDir)
	})
	return runtimePath, runtimeErr
}

// locateOrInstallLlamaRuntime searches the well known locations and downloads the
// official release when nothing is found.
func locateOrInstallLlamaRuntime(storageDir string) (*Runtime, error) {
	directories := []string{
		llamaRuntimeDirectory(storageDir),
		filepath.Join(filepath.Dir(os.Args[0]), "llama-runtime"),
		filepath.Join(filepath.Dir(os.Args[0]), "llama"),
	}

	for _, dir := range directories {
		if found := runtimeFromDirectory(dir); found != nil {
			return found, nil
		}
	}

	// A runtime that is installed system wide.
	if path, err := exec.LookPath(ServerBinaryName()); err == nil {
		return &Runtime{Dir: filepath.Dir(path), ServerPath: path}, nil
	}

	downloadDir := llamaRuntimeDirectory(storageDir)
	if err := installLlamaRuntime(downloadDir); err != nil {
		return nil, err
	}
	found := runtimeFromDirectory(downloadDir)
	if found == nil {
		return nil, errors.New("llama.cpp runtime could not be located after the installation")
	}
	return found, nil
}

// runtimeFromDirectory returns the runtime of a directory when it contains the
// llama.cpp binaries.
func runtimeFromDirectory(dir string) *Runtime {
	if dir == "" {
		return nil
	}
	serverPath := findBinaryInTree(dir, ServerBinaryName())
	cliPath := findBinaryInTree(dir, CliBinaryName())
	if serverPath == "" && cliPath == "" {
		return nil
	}
	return &Runtime{Dir: dir, ServerPath: serverPath, CliPath: cliPath}
}

// findBinaryInTree searches a directory tree for a binary.
func findBinaryInTree(dir string, name string) string {
	direct := filepath.Join(dir, name)
	if info, err := os.Stat(direct); err == nil && !info.IsDir() {
		return direct
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(dir, entry.Name(), name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// ServerBinaryName returns the name of the llama.cpp server binary.
func ServerBinaryName() string {
	if runtime.GOOS == "windows" {
		return "llama-server.exe"
	}
	return "llama-server"
}

// CliBinaryName returns the name of the llama.cpp cli binary.
func CliBinaryName() string {
	if runtime.GOOS == "windows" {
		return "llama-cli.exe"
	}
	return "llama-cli"
}

// llamaReleaseAsset returns the name of the official release archive for the platform.
func llamaReleaseAsset() (string, error) {
	switch {
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		return fmt.Sprintf("llama-%s-bin-win-avx2-x64.zip", llamaReleaseTag), nil
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		return fmt.Sprintf("llama-%s-bin-ubuntu-x64.zip", llamaReleaseTag), nil
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		return fmt.Sprintf("llama-%s-bin-macos-arm64.zip", llamaReleaseTag), nil
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		return fmt.Sprintf("llama-%s-bin-macos-x64.zip", llamaReleaseTag), nil
	}
	return "", fmt.Errorf("no llama.cpp release for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// installLlamaRuntime downloads and extracts the official llama.cpp release.
func installLlamaRuntime(targetDir string) error {
	asset, err := llamaReleaseAsset()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://github.com/ggml-org/llama.cpp/releases/download/%s/%s", llamaReleaseTag, asset)
	fmt.Printf("[BYOG LLAMA] Downloading the llama.cpp runtime: %s\n", url)

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}

	client := &http.Client{Timeout: 20 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("llama.cpp runtime download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("llama.cpp runtime download failed with status %d", resp.StatusCode)
	}

	archivePath := filepath.Join(targetDir, asset)
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return err
	}
	out.Close()
	defer os.Remove(archivePath)

	if err := extractZip(archivePath, targetDir); err != nil {
		return fmt.Errorf("llama.cpp runtime extraction failed: %w", err)
	}
	fmt.Printf("[BYOG LLAMA] llama.cpp runtime installed at %s\n", targetDir)
	return nil
}

// extractZip extracts an archive into a directory.
func extractZip(archivePath string, targetDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		name := filepath.ToSlash(file.Name)
		if strings.Contains(name, "..") {
			continue
		}
		destination := filepath.Join(targetDir, filepath.FromSlash(name))
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		mode := file.Mode()
		if mode == 0 {
			mode = 0o644
		}
		target, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
		if err != nil {
			source.Close()
			return err
		}
		if _, err := io.Copy(target, source); err != nil {
			source.Close()
			target.Close()
			return err
		}
		source.Close()
		target.Close()
	}
	return nil
}

// StartServer starts a llama.cpp server for the given model and waits until it
// answers on the health endpoint.
func (r *Runtime) StartServer(modelPath string, storageDir string) (*Server, error) {
	if r.ServerPath == "" {
		return nil, errors.New("llama-server is not available")
	}
	port, err := freeLocalPort()
	if err != nil {
		return nil, err
	}

	ramMB := systemRamMB()
	ctxSize := "4096"
	batchSize := "512"
	useMmap := true

	if ramMB <= 1500 {
		ctxSize = "512"
		batchSize = "128"
		useMmap = false
	} else if ramMB <= 2500 {
		ctxSize = "1024"
		batchSize = "256"
	} else if ramMB <= 4500 {
		ctxSize = "2048"
		batchSize = "512"
	}

	threads := runtime.NumCPU()
	if threads > 1 {
		threads-- // Leave 1 core for the OS and UI responsiveness
	}

	args := []string{
		"-m", modelPath,
		"--host", "127.0.0.1",
		"--port", strconv.Itoa(port),
		"-c", ctxSize,
		"-b", batchSize,
		"-t", strconv.Itoa(threads),
		"--no-warmup",
		"-np", "1",
		"--log-disable",
	}
	if !useMmap {
		args = append(args, "--no-mmap")
	}
	logFile, _ := os.OpenFile(filepath.Join(storageDir, "llama_server.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	cmd := exec.Command(r.ServerPath, args...)
	cmd.Dir = storageDir
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}
	hideProcessWindow(cmd)
	if err := cmd.Start(); err != nil {
		if logFile != nil {
			logFile.Close()
		}
		return nil, fmt.Errorf("llama-server could not be started: %w", err)
	}

	process := &Server{cmd: cmd, port: port, model: modelPath, started: time.Now()}
	if err := waitForLlamaServer(cmd, port, 90*time.Second); err != nil {
		_ = cmd.Process.Kill()
		if logFile != nil {
			logFile.Close()
		}
		return nil, err
	}
	return process, nil
}

// freeLocalPort reserves a free TCP port on the loopback interface.
func freeLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// waitForLlamaServer waits until the server accepts requests.
func waitForLlamaServer(cmd *exec.Cmd, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		if cmd != nil && cmd.ProcessState != nil && cmd.ProcessState.Exited() {
			return errors.New("llama-server process exited unexpectedly")
		}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health", port))
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return errors.New("llama-server did not become ready in time")
}

// serverForModel returns a running server for the model, starting it when needed.
func (r *Runtime) serverForModel(modelPath string, storageDir string) (*Server, error) {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	if process, ok := servers[modelPath]; ok {
		process.mutex.Lock()
		alive := process.cmd.ProcessState == nil || !process.cmd.ProcessState.Exited()
		process.mutex.Unlock()
		if alive {
			return process, nil
		}
		delete(servers, modelPath)
	}

	process, err := r.StartServer(modelPath, storageDir)
	if err != nil {
		return nil, err
	}
	servers[modelPath] = process
	return process, nil
}

// StopServers stops every running llama.cpp server.
func StopServers() {
	serverMutex.Lock()
	defer serverMutex.Unlock()
	for path, process := range servers {
		process.mutex.Lock()
		if !process.stopping && process.cmd.Process != nil {
			_ = process.cmd.Process.Kill()
		}
		process.stopping = true
		process.mutex.Unlock()
		delete(servers, path)
	}
}

// llamaChatRequest is the payload of an OpenAI compatible chat completion.
type llamaChatRequest struct {
	Model         string             `json:"model"`
	Messages      []llamaChatMessage `json:"messages"`
	MaxTokens     int                `json:"max_tokens"`
	Temperature   float64            `json:"temperature"`
	Stream        bool               `json:"stream"`
	StreamOptions map[string]bool    `json:"stream_options,omitempty"`
	CachePrompt   bool               `json:"cache_prompt"`
}

// llamaChatMessage is a single chat message.
type llamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// llamaChatResponse is a chunk of the streamed chat completion.
type llamaChatResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Result carries the measured results of a real inference run.
type Result struct {
	TotalTokens   int
	PromptTokens  int
	TtftMs        int
	Tps           float64
	Text          string
	PromptApplied bool
}

// Infer performs a real inference with the llama.cpp server and streams the produced
// tokens to the callback.
func (r *Runtime) Infer(ctx context.Context, modelPath string, storageDir string, prompt string, maxTokens int, temperature float64, onChunk func(piece string, tokenId int) bool) (*Result, error) {
	process, err := r.serverForModel(modelPath, storageDir)
	if err != nil {
		return nil, err
	}

	payload := llamaChatRequest{
		Model:       modelPath,
		Messages:    []llamaChatMessage{{Role: "user", Content: prompt}},
		MaxTokens:   maxTokens,
		Temperature: temperature,
		Stream:      true,
		StreamOptions: map[string]bool{
			"include_usage": true,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", process.port)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("llama-server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}

	result := &Result{}
	var builder strings.Builder
	var firstTokenAt time.Time
	var lastTokenAt time.Time
	streamedPieces := 0

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk llamaChatResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return nil, fmt.Errorf("llama-server error: %s", chunk.Error.Message)
		}
		if chunk.Usage != nil {
			result.PromptTokens = chunk.Usage.PromptTokens
			result.TotalTokens = chunk.Usage.CompletionTokens
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			if firstTokenAt.IsZero() {
				firstTokenAt = time.Now()
			}
			lastTokenAt = time.Now()
			streamedPieces++
			builder.WriteString(choice.Delta.Content)
			if onChunk != nil && !onChunk(choice.Delta.Content, 1000+streamedPieces) {
				return nil, errors.New("generation cancelled")
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	result.Text = builder.String()
	if result.TotalTokens == 0 {
		result.TotalTokens = streamedPieces
	}
	if !firstTokenAt.IsZero() {
		result.TtftMs = int(firstTokenAt.Sub(start).Milliseconds())
	}
	generation := lastTokenAt.Sub(firstTokenAt)
	if generation <= 0 {
		generation = time.Since(start)
	}
	if generation > 0 && result.TotalTokens > 0 {
		result.Tps = float64(result.TotalTokens) / generation.Seconds()
	}
	result.Tps = float64(int(result.Tps*10)) / 10
	return result, nil
}
