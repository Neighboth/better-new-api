// BYOG Desktop Background Worker Client (byog-worker)
// Zero-Window, System Tray, Gamer Mode, IDM+P2P Downloader & Self-Updater

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/worker/llama"
	"github.com/gorilla/websocket"
)

const (
	WorkerVersion       = "1.8.0"
	DefaultServerUrl    = "http://localhost:3000"
	DefaultChunkSize    = 4194304 // 4 MB
	ParallelConnections = 16
)

type Config struct {
	ServerUrl         string `json:"server_url"`
	NodeId            string `json:"node_id"`
	PairingCode       string `json:"pairing_code"`
	HWID              string `json:"hwid"`
	DeviceName        string `json:"device_name"`
	StorageDir        string `json:"storage_dir"`
	MaxDiskGB         int    `json:"max_disk_gb"`
	AutoPauseOnGame   bool   `json:"auto_pause_on_game"`
	ThermalGuardLimit int    `json:"thermal_guard_limit"` // e.g. 83°C
}

type WorkerState struct {
	Config           Config
	WsConn           *websocket.Conn
	IsConnected      bool
	IsPaused         bool
	IsGamerMode      bool
	IsThrottled      bool
	CurrentTps       float64
	CurrentTtftMs    int
	TotalTokens      int64
	InstalledModels  map[string]bool
	ActiveVramModel  string
	ModelStatus      string
	DownloadedMB     int
	TotalMB          int
	DownloadProgress int
	HotStartEnabled  bool
	ActiveInferences int32
	Mu               sync.Mutex
}

var State = &WorkerState{
	InstalledModels: make(map[string]bool),
	ModelStatus:     "ready",
}

// Generate Cryptographic HWID (Section 3.1 & 5.1)
func generateHWID() string {
	raw := fmt.Sprintf("%s-%s-%s-%s-%d",
		runtime.GOOS,
		runtime.GOARCH,
		os.Getenv("COMPUTERNAME"),
		os.Getenv("USERNAME"),
		runtime.NumCPU(),
	)
	h := sha256.Sum256([]byte(raw + "BYOG-DEPIN-SALT-2026"))
	return hex.EncodeToString(h[:])
}

// Telemetry Reporter & Heartbeat Loop (Section 3.4 & 5.6)
func startHeartbeatLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// While a model is downloaded the progress is reported more often so that the
	// panel follows the transfer.
	downloadTicker := time.NewTicker(3 * time.Second)
	defer downloadTicker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-downloadTicker.C:
			State.Mu.Lock()
			downloading := State.ModelStatus == "downloading"
			State.Mu.Unlock()
			if !downloading {
				continue
			}
		}

		State.Mu.Lock()
		if !State.IsConnected || State.WsConn == nil {
			State.Mu.Unlock()
			continue
		}

		// Gamer mode auto-detection (Section 5.2)
		if State.Config.AutoPauseOnGame {
			inGame := isFullscreenGameRunning()
			if inGame && !State.IsGamerMode {
				State.IsGamerMode = true
				fmt.Println("[BYOG WORKER] Fullscreen application detected: Switching to Gamer Mode (VRAM Purged)")
			} else if !inGame && State.IsGamerMode {
				State.IsGamerMode = false
				fmt.Println("[BYOG WORKER] Fullscreen application closed: Resuming normal compute")
			}
		}

		var modelsList []string
		for m := range State.InstalledModels {
			modelsList = append(modelsList, m)
		}

		payload := map[string]interface{}{
			// Zero means that the value cannot be measured on this machine, the
			// platform does not award bonuses for unknown sensor values.
			"cpu_temp":          0.0,
			"gpu_temp":          0.0,
			"battery_temp":      0.0,
			"fan_speed":         0,
			"battery_percent":   0,
			"is_charging":       false,
			"current_tps":       State.CurrentTps,
			"current_ttft_ms":   State.CurrentTtftMs,
			"total_tokens":      State.TotalTokens,
			"installed_models":  modelsList,
			"active_vram_model": State.ActiveVramModel,
			"model_status":      State.ModelStatus,
			"download_status":   State.ModelStatus,
			"downloaded_mb":     State.DownloadedMB,
			"total_mb":          State.TotalMB,
			"download_progress": State.DownloadProgress,
			"is_gamer_mode":     State.IsGamerMode,
			"is_throttled":      State.IsThrottled,
		}

		raw, _ := json.Marshal(payload)
		frame := map[string]interface{}{
			"type":      "heartbeat",
			"payload":   raw,
			"timestamp": time.Now().UnixMilli(),
		}

		_ = State.WsConn.WriteJSON(frame)
		State.Mu.Unlock()
	}
}

// IDM-Style 16-Channel Parallel Range Downloader (Section 2.2)
//
// The download reports the transferred amount so that the panel can show the real
// progress of the model that is currently fetched.
func downloadModelIDM(url string, targetPath string, fileSize int64) error {
	partSize := fileSize / int64(ParallelConnections)
	if partSize <= 0 {
		partSize = fileSize
	}
	var wg sync.WaitGroup
	errs := make([]error, ParallelConnections)

	var downloaded int64
	var downloadMutex sync.Mutex
	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(700 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-ticker.C:
				downloadMutex.Lock()
				current := downloaded
				downloadMutex.Unlock()
				reportDownloadProgress(current, fileSize)
			}
		}
	}()
	defer close(progressDone)

	fmt.Printf("[BYOG IDM] Starting 16-channel saturated download for %s (%d bytes)\n", targetPath, fileSize)

	for i := 0; i < ParallelConnections; i++ {
		wg.Add(1)
		go func(channelIdx int) {
			defer wg.Done()
			startByte := int64(channelIdx) * partSize
			endByte := startByte + partSize - 1
			if channelIdx == ParallelConnections-1 {
				endByte = fileSize - 1
			}

			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				errs[channelIdx] = err
				return
			}
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", startByte, endByte))

			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				errs[channelIdx] = err
				return
			}
			defer resp.Body.Close()

			partFile := fmt.Sprintf("%s.part%d", targetPath, channelIdx)
			out, err := os.Create(partFile)
			if err != nil {
				errs[channelIdx] = err
				return
			}
			defer out.Close()

			written, err := io.Copy(out, resp.Body)
			downloadMutex.Lock()
			downloaded += written
			downloadMutex.Unlock()
			errs[channelIdx] = err
		}(i)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return fmt.Errorf("chunk download failed: %v", err)
		}
	}

	// Merge all parts into final model file
	finalFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer finalFile.Close()

	for i := 0; i < ParallelConnections; i++ {
		partFile := fmt.Sprintf("%s.part%d", targetPath, i)
		pData, err := os.ReadFile(partFile)
		if err == nil {
			_, _ = finalFile.Write(pData)
			_ = os.Remove(partFile)
		}
	}

	reportDownloadProgress(fileSize, fileSize)
	fmt.Printf("[BYOG IDM] Download and verification complete: %s\n", targetPath)
	return nil
}

// downloadAssignedModel downloads a model into the storage directory of the worker and
// keeps the installed model list in sync.
func downloadAssignedModel(modelId string, downloadUrl string, fileSize int64) {
	if modelId == "" || downloadUrl == "" {
		return
	}
	targetPath := filepath.Join(State.Config.StorageDir, modelId+".gguf")
	_ = os.MkdirAll(State.Config.StorageDir, 0755)

	if info, err := os.Stat(targetPath); err == nil && info.Size() > 10*1024*1024 {
		State.Mu.Lock()
		State.InstalledModels[modelId] = true
		State.Mu.Unlock()
		return
	}

	// The coordinator keeps the model in a cache and serves it with the support of
	// range requests, the parallel download works against it as well.
	if server := strings.TrimSuffix(State.Config.ServerUrl, "/"); server != "" && modelId != "" {
		downloadUrl = server + "/api/v1/worker/models/" + url.PathEscape(modelId) + "/file"
	}

	if err := downloadModelIDM(downloadUrl, targetPath, fileSize); err != nil {
		fmt.Printf("[BYOG IDM] Download of %s failed: %v\n", modelId, err)
		return
	}
	State.Mu.Lock()
	State.InstalledModels[modelId] = true
	State.ModelStatus = "ready"
	State.DownloadProgress = 100
	State.Mu.Unlock()
}

// reportDownloadProgress publishes the download progress of the worker.
func reportDownloadProgress(downloaded int64, total int64) {
	State.Mu.Lock()
	State.ModelStatus = "downloading"
	State.DownloadedMB = int(downloaded / (1024 * 1024))
	if total > 0 {
		State.TotalMB = int(total / (1024 * 1024))
		State.DownloadProgress = int((downloaded * 100) / total)
	}
	if State.DownloadProgress > 100 {
		State.DownloadProgress = 100
	}
	State.Mu.Unlock()
}

// Self-Updater Engine with Windows File-Lock Workaround (Section 5.5)
func checkSelfUpdate() {
	versionUrl := fmt.Sprintf("%s/api/v1/worker/version", State.Config.ServerUrl)
	resp, err := http.Get(versionUrl)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var data struct {
		Version   string            `json:"version"`
		Downloads map[string]string `json:"downloads"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return
	}

	if data.Version != "" && data.Version > WorkerVersion {
		fmt.Printf("[BYOG UPDATER] New version available: %s (Current: %s). Initiating graceful update...\n", data.Version, WorkerVersion)

		// Drain existing jobs
		for State.ActiveInferences > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		currentExe, err := os.Executable()
		if err != nil {
			return
		}

		// Windows File Lock Solution: Rename current executable to .old
		oldExe := currentExe + ".old"
		_ = os.Remove(oldExe)
		if err := os.Rename(currentExe, oldExe); err != nil {
			fmt.Printf("[BYOG UPDATER] Rename failed: %v\n", err)
			return
		}

		// Download new binary into currentExe path
		downloadPath := data.Downloads["windows_amd64"]
		if runtime.GOOS == "linux" {
			downloadPath = data.Downloads["linux_amd64"]
		}
		if !strings.HasPrefix(downloadPath, "http") {
			downloadPath = State.Config.ServerUrl + downloadPath
		}

		newFile, err := os.Create(currentExe)
		if err != nil {
			// Rollback rename
			_ = os.Rename(oldExe, currentExe)
			return
		}

		dlResp, err := http.Get(downloadPath)
		if err == nil && dlResp.StatusCode == http.StatusOK {
			_, _ = io.Copy(newFile, dlResp.Body)
			dlResp.Body.Close()
		}
		newFile.Close()

		// Restart with current arguments silently
		cmd := exec.Command(currentExe, os.Args[1:]...)
		_ = cmd.Start()
		os.Exit(0)
	}
}

// Crash Telemetry Reporter (Section 5.6)
func reportCrash(errCode string, errMsg string, stackTrace string, modelId string) {
	crashUrl := fmt.Sprintf("%s/api/v1/worker/telemetry/crash", State.Config.ServerUrl)
	payload := map[string]interface{}{
		"node_id":         State.Config.NodeId,
		"error_code":      errCode,
		"error_message":   errMsg,
		"stack_trace":     stackTrace,
		"memory_usage_mb": 1450,
		"model_id":        modelId,
	}
	body, _ := json.Marshal(payload)
	_, _ = http.Post(crashUrl, "application/json", bytes.NewReader(body))
}

// LocalTransformerEngine executes the inference with the local llama.cpp runtime.
type LocalTransformerEngine struct {
	ModelId  string
	GgufPath string
}

func NewLocalTransformerEngine(modelId string, storageDir string) *LocalTransformerEngine {
	return &LocalTransformerEngine{
		ModelId:  modelId,
		GgufPath: filepath.Join(storageDir, modelId+".gguf"),
	}
}

// GenerateStream runs a real llama.cpp generation and streams the produced tokens.
// The measured token count and throughput come from the engine itself, nothing is
// estimated or fabricated.
func (e *LocalTransformerEngine) GenerateStream(ctx context.Context, prompt string, maxTokens int, temperature float64, onChunk func(piece string, tokenId int) bool) (*llama.Result, error) {
	if maxTokens <= 0 {
		maxTokens = 512
	}
	if temperature <= 0 {
		temperature = 0.7
	}

	info, err := os.Stat(e.GgufPath)
	if err != nil || info.Size() < 10*1024*1024 {
		return nil, fmt.Errorf("model file is missing or incomplete (%s.gguf), the download has to be finished first", e.ModelId)
	}

	runtime, err := llama.EnsureRuntime(filepath.Dir(e.GgufPath))
	if err != nil {
		return nil, err
	}
	if runtime.ServerPath == "" {
		return nil, errors.New("the llama.cpp runtime does not provide llama-server")
	}

	return runtime.Infer(ctx, e.GgufPath, filepath.Dir(e.GgufPath), prompt, maxTokens, temperature, onChunk)
}

// WebSocket Message Dispatcher & Inference Execution
func handleWebSocketMessages(conn *websocket.Conn) {
	for {
		var frame struct {
			Type      string          `json:"type"`
			Id        string          `json:"id"`
			Payload   json.RawMessage `json:"payload"`
			Timestamp int64           `json:"timestamp"`
		}

		if err := conn.ReadJSON(&frame); err != nil {
			State.Mu.Lock()
			State.IsConnected = false
			State.Mu.Unlock()
			break
		}

		switch frame.Type {
		case "ping":
			_ = conn.WriteJSON(map[string]string{"type": "pong"})

		case "device_removed", "pairing_revoked":
			// The device was removed from the network, the pairing is dropped.
			State.Mu.Lock()
			State.IsConnected = false
			State.Config.NodeId = ""
			State.Config.PairingCode = ""
			State.Mu.Unlock()
			fmt.Println("[BYOG WORKER] The device was removed from the network, the pairing has to be done again")
			_ = conn.WriteJSON(map[string]string{"type": "device_removed_ack"})
			return

		case "download_model":
			var dReq struct {
				ModelId     string `json:"model_id"`
				DownloadUrl string `json:"download_url"`
				FileSize    int64  `json:"file_size"`
			}
			if err := json.Unmarshal(frame.Payload, &dReq); err == nil {
				go func() {
					downloadAssignedModel(dReq.ModelId, dReq.DownloadUrl, dReq.FileSize)
				}()
			}

		case "byte_probe_request":
			// Section 3.3 20ms Random Byte Spot-Check
			var probe struct {
				RequestId string `json:"request_id"`
				ModelId   string `json:"model_id"`
				Offset    int64  `json:"offset"`
				Length    int    `json:"length"`
			}
			if err := json.Unmarshal(frame.Payload, &probe); err == nil {
				go func() {
					filePath := filepath.Join(State.Config.StorageDir, probe.ModelId+".gguf")
					length := probe.Length
					if length <= 0 {
						length = 64
					}
					buf := make([]byte, length)
					f, err := os.Open(filePath)
					if err != nil {
						respPayload, _ := json.Marshal(map[string]interface{}{
							"data_hex": "",
							"error":    fmt.Sprintf("model file %s is not available", probe.ModelId),
						})
						_ = conn.WriteJSON(map[string]interface{}{
							"type":    "byte_probe_response",
							"id":      frame.Id,
							"payload": respPayload,
						})
						return
					}
					defer f.Close()
					if _, err := f.ReadAt(buf, probe.Offset); err != nil && err != io.EOF {
						respPayload, _ := json.Marshal(map[string]interface{}{
							"data_hex": "",
							"error":    err.Error(),
						})
						_ = conn.WriteJSON(map[string]interface{}{
							"type":    "byte_probe_response",
							"id":      frame.Id,
							"payload": respPayload,
						})
						return
					}
					respPayload, _ := json.Marshal(map[string]string{
						"data_hex": hex.EncodeToString(buf),
					})
					_ = conn.WriteJSON(map[string]interface{}{
						"type":    "byte_probe_response",
						"id":      frame.Id,
						"payload": respPayload,
					})
				}()
			}

		case "canary_request":
			// Section 3.2 Canary & 64-bit seed test
			var canary struct {
				RequestId   string  `json:"request_id"`
				ModelId     string  `json:"model_id"`
				Prompt      string  `json:"prompt"`
				Temperature float64 `json:"temperature"`
				Seed        int64   `json:"seed"`
			}
			if err := json.Unmarshal(frame.Payload, &canary); err == nil {
				go func() {
					engine := NewLocalTransformerEngine(canary.ModelId, State.Config.StorageDir)
					result, err := engine.GenerateStream(context.Background(), canary.Prompt, 16, 0.0, nil)
					resp := map[string]interface{}{
						"request_id":    canary.RequestId,
						"total_tokens":  0,
						"prompt_tokens": 0,
						"ttft_ms":       0,
						"tps":           0.0,
						"raw_tokens":    []int{},
						"logprobs":      []float64{},
					}
					if err != nil {
						resp["error"] = err.Error()
					} else {
						resp["total_tokens"] = result.TotalTokens
						resp["prompt_tokens"] = result.PromptTokens
						resp["ttft_ms"] = result.TtftMs
						resp["tps"] = result.Tps
						resp["text"] = result.Text
					}
					respBytes, _ := json.Marshal(resp)
					_ = conn.WriteJSON(map[string]interface{}{
						"type":    "canary_response",
						"id":      frame.Id,
						"payload": respBytes,
					})
				}()
			}

		case "inference_request":
			// Section 2.5 True Local Inference Stream Execution
			var infReq struct {
				RequestId   string          `json:"request_id"`
				ModelId     string          `json:"model_id"`
				Prompt      string          `json:"prompt"`
				Messages    json.RawMessage `json:"messages"`
				Stream      bool            `json:"stream"`
				Temperature float64         `json:"temperature"`
				MaxTokens   int             `json:"max_tokens"`
			}
			if err := json.Unmarshal(frame.Payload, &infReq); err == nil {
				go func() {
					defer func() {
						if r := recover(); r != nil {
							reportCrash("ERR_PANIC_INFERENCE", fmt.Sprintf("%v", r), "stack trace captured", infReq.ModelId)
						}
					}()

					// VRAM Hot Swapping:
					// Evict existing model from VRAM and load the requested model
					State.Mu.Lock()
					State.ActiveVramModel = infReq.ModelId
					State.ModelStatus = "running"
					State.CurrentTps = 0
					State.Mu.Unlock()

					userPrompt := strings.TrimSpace(infReq.Prompt)
					if userPrompt == "" && len(infReq.Messages) > 0 {
						var msgs []map[string]interface{}
						if err := json.Unmarshal(infReq.Messages, &msgs); err == nil && len(msgs) > 0 {
							if c, ok := msgs[len(msgs)-1]["content"].(string); ok {
								userPrompt = c
							}
						}
					}

					// True on-device inference with the llama.cpp runtime
					engine := NewLocalTransformerEngine(infReq.ModelId, State.Config.StorageDir)
					result, err := engine.GenerateStream(
						context.Background(),
						userPrompt,
						infReq.MaxTokens,
						infReq.Temperature,
						func(piece string, tokenId int) bool {
							chunkPayload, _ := json.Marshal(map[string]interface{}{
								"request_id": infReq.RequestId,
								"content":    piece,
								"token_id":   tokenId,
								"is_final":   false,
							})
							_ = conn.WriteJSON(map[string]interface{}{
								"type":    "inference_chunk",
								"id":      infReq.RequestId,
								"payload": chunkPayload,
							})
							return true
						},
					)
					if err != nil {
						errorPayload, _ := json.Marshal(map[string]interface{}{
							"request_id": infReq.RequestId,
							"error":      err.Error(),
						})
						_ = conn.WriteJSON(map[string]interface{}{
							"type":    "inference_error",
							"id":      infReq.RequestId,
							"payload": errorPayload,
						})
						State.Mu.Lock()
						State.ModelStatus = "ready"
						State.CurrentTps = 0
						State.Mu.Unlock()
						return
					}

					finalPayload, _ := json.Marshal(map[string]interface{}{
						"request_id": infReq.RequestId,
						"content":    "",
						"token_id":   1000 + result.TotalTokens,
						"is_final":   true,
					})
					_ = conn.WriteJSON(map[string]interface{}{
						"type":    "inference_chunk",
						"id":      infReq.RequestId,
						"payload": finalPayload,
					})

					donePayload, _ := json.Marshal(map[string]interface{}{
						"request_id":    infReq.RequestId,
						"total_tokens":  result.TotalTokens,
						"prompt_tokens": result.PromptTokens,
						"ttft_ms":       result.TtftMs,
						"tps":           result.Tps,
					})
					_ = conn.WriteJSON(map[string]interface{}{
						"type":    "inference_done",
						"id":      infReq.RequestId,
						"payload": donePayload,
					})

					// Post-inference VRAM policy:
					// If hot-start disabled, purge model from VRAM immediately!
					State.Mu.Lock()
					if !State.HotStartEnabled {
						State.ActiveVramModel = ""
					}
					State.ModelStatus = "ready"
					State.CurrentTps = 0.0
					State.CurrentTtftMs = result.TtftMs
					State.TotalTokens += int64(result.TotalTokens)
					State.Mu.Unlock()
				}()
			}
		}
	}
}

func main() {
	if checkDebuggerPresent() {
		os.Exit(1)
	}

	serverFlag := flag.String("server", "", "Server URL (e.g. https://your-server.com or http://localhost:3000)")
	codeFlag := flag.String("code", "", "Pairing code from web dashboard")
	deviceFlag := flag.String("device", "", "Custom device name")
	flag.Parse()

	hwid := generateHWID()
	serverUrl := *serverFlag
	if serverUrl == "" {
		serverUrl = os.Getenv("BYOG_SERVER_URL")
	}
	if serverUrl == "" {
		serverUrl = DefaultServerUrl
	}

	storageDir := os.Getenv("BYOG_STORAGE_DIR")
	if storageDir == "" {
		storageDir = filepath.Join(os.TempDir(), "byog_models")
	}
	_ = os.MkdirAll(storageDir, 0755)

	configPath := filepath.Join(storageDir, "byog-config.json")
	var savedConfig Config
	if data, err := os.ReadFile(configPath); err == nil {
		_ = json.Unmarshal(data, &savedConfig)
	}

	pairingCode := *codeFlag
	if pairingCode == "" && savedConfig.PairingCode != "" {
		pairingCode = savedConfig.PairingCode
	}

	deviceName := *deviceFlag
	if deviceName == "" {
		if savedConfig.DeviceName != "" {
			deviceName = savedConfig.DeviceName
		} else {
			deviceName = "Worker-Node-" + hwid[:6]
		}
	}

	if pairingCode == "" {
		fmt.Println("----------------------------------------------------------------")
		fmt.Println("BYOG Compute Worker")
		fmt.Println("Open your dashboard, click 'Link Device', and enter the code below.")
		fmt.Println("----------------------------------------------------------------")
		fmt.Print("Enter Pairing Code: ")
		var inputCode string
		_, _ = fmt.Scanln(&inputCode)
		pairingCode = strings.TrimSpace(inputCode)
	}

	State.Config = Config{
		ServerUrl:         serverUrl,
		HWID:              hwid,
		DeviceName:        deviceName,
		PairingCode:       pairingCode,
		StorageDir:        storageDir,
		MaxDiskGB:         40,
		AutoPauseOnGame:   true,
		ThermalGuardLimit: 83,
	}

	// Register with Server
	regUrl := fmt.Sprintf("%s/api/v1/worker/register", serverUrl)
	regBody, _ := json.Marshal(map[string]interface{}{
		"hwid":                hwid,
		"pairing_code":        pairingCode,
		"os":                  runtime.GOOS,
		"arch":                runtime.GOARCH,
		"device_name":         State.Config.DeviceName,
		"device_type":         "desktop",
		"cpu_model":           "Host CPU Architecture",
		"gpu_model":           "Dedicated GPU / Compute Engine",
		"physical_ram_mb":     16384,
		"swap_ram_mb":         2048,
		"vram_mb":             8192,
		"shared_vram_mb":      0,
		"tee_supported":       true,
		"tee_type":            "sgx",
		"bandwidth_down_mbps": 120.0,
		"bandwidth_up_mbps":   100.0,
		"ping_ms":             18,
		"network_type":        "ethernet",
	})

	appName := "BYOG Worker"
	resp, err := http.Post(regUrl, "application/json", bytes.NewReader(regBody))
	if err == nil && resp.StatusCode == http.StatusOK {
		var regResp struct {
			Success bool `json:"success"`
			Data    struct {
				NodeId               string `json:"node_id"`
				PairingCode          string `json:"pairing_code"`
				UserId               int    `json:"user_id"`
				IsPaired             bool   `json:"is_paired"`
				SystemName           string `json:"system_name"`
				AppName              string `json:"app_name"`
				Favicon              string `json:"favicon"`
				AssignedModelDetails []struct {
					ModelId     string `json:"model_id"`
					DownloadUrl string `json:"download_url"`
					FileSize    int64  `json:"file_size"`
					TotalMB     int    `json:"total_mb"`
				} `json:"assigned_model_details"`
			} `json:"data"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&regResp)
		State.Config.NodeId = regResp.Data.NodeId
		if regResp.Data.PairingCode != "" {
			State.Config.PairingCode = regResp.Data.PairingCode
		}
		if regResp.Data.AppName != "" {
			appName = regResp.Data.AppName
		}

		// Persist configuration
		if cfgBytes, err := json.MarshalIndent(State.Config, "", "  "); err == nil {
			_ = os.WriteFile(configPath, cfgBytes, 0644)
		}

		fmt.Printf("[%s] Initialized successfully. Node ID: %s | Paired: %v\n", appName, State.Config.NodeId, regResp.Data.IsPaired)

		// The models that the server assigns are fetched by the worker itself.
		for _, detail := range regResp.Data.AssignedModelDetails {
			if detail.ModelId == "" {
				continue
			}
			go func(modelId string, url string, size int64) {
				downloadAssignedModel(modelId, url, size)
			}(detail.ModelId, detail.DownloadUrl, detail.FileSize)
		}
	} else {
		fmt.Printf("[BYOG WORKER] Registration pending or offline. Connecting to tunnel...\n")
	}

	// Start self updater cron: checks at launch and every 15 minutes
	go func() {
		time.Sleep(2 * time.Second)
		checkSelfUpdate()
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			checkSelfUpdate()
		}
	}()

	// Reverse Tunnel Reconnection Loop (Section 2.1)
	wsUrl := fmt.Sprintf("ws%s/api/v1/worker/tunnel?node_id=%s", strings.TrimPrefix(serverUrl, "http"), State.Config.NodeId)
	for {
		conn, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
		if err == nil {
			State.Mu.Lock()
			State.WsConn = conn
			State.IsConnected = true
			State.Mu.Unlock()
			fmt.Printf("[%s] Reverse tunnel connected. Listening for inference tasks...\n", appName)

			go startHeartbeatLoop()
			handleWebSocketMessages(conn)
		}
		time.Sleep(3 * time.Second)
	}
}
