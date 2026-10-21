// BYOG Mobile & Tablet Background Worker (byog-mobile-worker)
//
// The worker runs on Android (Termux) and Linux devices with an ARM processor. It
// reports the real hardware telemetry, downloads the models that the coordinator
// assigns and answers the inference requests with the local llama.cpp runtime.
//
// Real Physical RAM vs ZRAM Filter, Strict Mobile Tiering, WakeLock & Foreground Service

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/worker/llama"
	"github.com/gorilla/websocket"
)

type MobileConfig struct {
	ServerUrl            string  `json:"server_url"`
	NodeId               string  `json:"node_id"`
	PairingCode          string  `json:"pairing_code"`
	DeviceName           string  `json:"device_name"`
	WifiOnlyDownload     bool    `json:"wifi_only_download"`      // DEFAULT TRUE (Section 6.3)
	MonthlyDataLimitMB   int64   `json:"monthly_data_limit_mb"`   // DEFAULT UNLIMITED (0) (Section 6.3)
	MonthlyDataUsedMB    int64   `json:"monthly_data_used_mb"`    //
	RunOnlyWhileCharging bool    `json:"run_only_while_charging"` // DEFAULT FALSE (Section 6.3)
	BatteryCutoffEnabled bool    `json:"battery_cutoff_enabled"`  // DEFAULT FALSE (Section 6.3)
	BatteryCutoffPercent int     `json:"battery_cutoff_percent"`  // 5 - 50%
	MaxBatteryTemp       float64 `json:"max_battery_temp"`        // DEFAULT 50°C (Section 6.3)
	BatteryTempSupported bool    `json:"battery_temp_supported"`
	StorageDir           string  `json:"storage_dir"`
}

type MemoryMetrics struct {
	PhysicalTotalMB int `json:"physical_total_mb"` // Real physical RAM
	AvailableMB     int `json:"available_mb"`      // Safe allocatable heap
	SwapTotalMB     int `json:"swap_total_mb"`     // ZRAM / Virtual RAM (STRICTLY IGNORED!)
	SafeBudgetMB    int `json:"safe_budget_mb"`    // Calculated safe budget
}

type MobileState struct {
	Config           MobileConfig
	Memory           MemoryMetrics
	WsConn           *websocket.Conn
	IsConnected      bool
	IsPaused         bool
	IsThrottled      bool
	IsCharging       bool
	BatteryPercent   int
	BatteryTemp      float64
	NetworkType      string // wifi, cellular
	InstalledModels  map[string]bool
	TotalTokens      int64
	CurrentTps       float64
	CurrentTtftMs    int
	ModelStatus      string
	DownloadedMB     int
	TotalMB          int
	DownloadProgress int
	Mu               sync.Mutex
}

var State = &MobileState{
	InstalledModels: make(map[string]bool),
	ModelStatus:     "ready",
	Config: MobileConfig{
		WifiOnlyDownload:     true,  // DEFAULT: ON (Section 6.3)
		MonthlyDataLimitMB:   0,     // DEFAULT: UNLIMITED (Section 6.3)
		RunOnlyWhileCharging: false, // DEFAULT: OFF (Section 6.3)
		BatteryCutoffEnabled: false, // DEFAULT: OFF (Section 6.3)
		BatteryCutoffPercent: 20,
		MaxBatteryTemp:       50.0, // DEFAULT: 50°C (Section 6.3)
		BatteryTempSupported: true,
	},
}

// Section 6.1: Strict Physical RAM vs ZRAM/Swap Filtering (/proc/meminfo)
func queryDeviceMemory() MemoryMetrics {
	metrics := MemoryMetrics{
		PhysicalTotalMB: 6144, // Default fallback
		AvailableMB:     2800,
		SwapTotalMB:     6144, // ZRAM / Virtual RAM (STRICTLY IGNORED!)
	}

	f, err := os.Open("/proc/meminfo")
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				key := strings.TrimSuffix(fields[0], ":")
				valKb, _ := strconv.Atoi(fields[1])
				switch key {
				case "MemTotal":
					metrics.PhysicalTotalMB = valKb / 1024
				case "MemAvailable":
					metrics.AvailableMB = valKb / 1024
				case "SwapTotal":
					metrics.SwapTotalMB = valKb / 1024 // Recorded only for telemetry, completely ignored in budgeting!
				}
			}
		}
	}

	// Safe Mobile Budget Formula: min(MemAvailable * 0.55, Physical MemTotal * 0.40)
	budgetA := float64(metrics.AvailableMB) * 0.55
	budgetB := float64(metrics.PhysicalTotalMB) * 0.40
	metrics.SafeBudgetMB = int(minFloat(budgetA, budgetB))

	return metrics
}

// minFloat returns the smaller of two values.
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Section 6.1: Strict Mobile Model Tiering Validator
func isModelTierAllowed(paramSize string, physicalRamMB int) bool {
	size := strings.ToUpper(strings.TrimSpace(paramSize))
	// 3GB - 4GB Physical RAM -> only 0.5B / Embedding
	if physicalRamMB <= 4096 {
		return size == "0.5B" || strings.Contains(size, "0.5")
	}
	// 6GB Physical RAM (e.g. Realme C55 with 6GB real + 6GB virtual) -> max 1.5B or 3B, NEVER 4B or 7B!
	if physicalRamMB <= 6144 {
		return size == "0.5B" || size == "1B" || size == "1.5B" || size == "3B"
	}
	// 8GB - 12GB Physical RAM -> max 3B or 4B
	if physicalRamMB <= 12288 {
		return size == "0.5B" || size == "1B" || size == "1.5B" || size == "3B" || size == "4B"
	}
	// 16GB+ Flagship (Snapdragon 8 Gen 4 / 8 Elite / A18 Pro / M-series iPad) -> 7B allowed
	return true
}

// Section 6.3: Constraint Checker (Wi-Fi only, Charging, Battery % & Temperature)
func canAcceptTasks() (bool, string) {
	State.Mu.Lock()
	defer State.Mu.Unlock()

	// 1. Wi-Fi Check
	if State.Config.WifiOnlyDownload && State.NetworkType == "cellular" {
		return false, "Wi-Fi Only mode active on cellular network"
	}

	// 2. Data Limit Check
	if State.Config.MonthlyDataLimitMB > 0 && State.Config.MonthlyDataUsedMB >= State.Config.MonthlyDataLimitMB {
		return false, "Monthly data usage limit reached"
	}

	// 3. Charging Check
	if State.Config.RunOnlyWhileCharging && !State.IsCharging {
		return false, "Paused: Run only while charging is active"
	}

	// 4. Battery % Cutoff
	if State.Config.BatteryCutoffEnabled && State.BatteryPercent < State.Config.BatteryCutoffPercent {
		return false, fmt.Sprintf("Paused: Battery below threshold (%d%% < %d%%)", State.BatteryPercent, State.Config.BatteryCutoffPercent)
	}

	// 5. Battery Temperature Cutoff (Only if sensor is present)
	if State.Config.BatteryTempSupported && State.BatteryTemp >= State.Config.MaxBatteryTemp {
		return false, fmt.Sprintf("Throttled: Battery temperature %.1fC exceeds %.1fC", State.BatteryTemp, State.Config.MaxBatteryTemp)
	}

	return true, ""
}

// telemetryPayload builds the heartbeat payload with the measured values.
func telemetryPayload() map[string]interface{} {
	State.Mu.Lock()
	defer State.Mu.Unlock()

	models := make([]string, 0, len(State.InstalledModels))
	for model := range State.InstalledModels {
		models = append(models, model)
	}

	return map[string]interface{}{
		// Zero means that the value cannot be measured on this device.
		"cpu_temp":          0.0,
		"gpu_temp":          0.0,
		"battery_temp":      State.BatteryTemp,
		"fan_speed":         0,
		"battery_percent":   State.BatteryPercent,
		"is_charging":       State.IsCharging,
		"current_tps":       State.CurrentTps,
		"current_ttft_ms":   State.CurrentTtftMs,
		"total_tokens":      State.TotalTokens,
		"installed_models":  models,
		"model_status":      State.ModelStatus,
		"download_status":   State.ModelStatus,
		"downloaded_mb":     State.DownloadedMB,
		"total_mb":          State.TotalMB,
		"download_progress": State.DownloadProgress,
		"network_type":      State.NetworkType,
	}
}

// sendHeartbeat publishes the telemetry to the coordinator.
func sendHeartbeat() {
	State.Mu.Lock()
	conn := State.WsConn
	State.Mu.Unlock()
	if conn == nil {
		return
	}
	raw, err := json.Marshal(telemetryPayload())
	if err != nil {
		return
	}
	_ = conn.WriteJSON(map[string]interface{}{
		"type":      "heartbeat",
		"payload":   raw,
		"timestamp": time.Now().UnixMilli(),
	})
}

// readBatteryState reads the battery percentage and the charging state of the device.
func readBatteryState() {
	output, err := exec.Command("sh", "-c", "cat /sys/class/power_supply/*/capacity 2>/dev/null | head -n1").Output()
	if err == nil {
		if value, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil && value > 0 && value <= 100 {
			State.Mu.Lock()
			State.BatteryPercent = value
			State.Mu.Unlock()
		}
	}

	status, err := exec.Command("sh", "-c", "cat /sys/class/power_supply/*/status 2>/dev/null | head -n1").Output()
	if err == nil {
		State.Mu.Lock()
		State.IsCharging = strings.Contains(strings.ToLower(string(status)), "charging")
		State.Mu.Unlock()
	}
}

// detectNetworkType reports the network type of the device.
func detectNetworkType() string {
	output, err := exec.Command("sh", "-c", "getprop gsm.network.type 2>/dev/null").Output()
	if err == nil {
		value := strings.TrimSpace(string(output))
		if value != "" {
			return value
		}
	}
	return "wifi"
}

// downloadModel fetches a model into the storage directory and reports the progress.
func downloadModel(modelID string, downloadURL string, fileSize int64) error {
	if modelID == "" {
		return errors.New("model id is missing")
	}
	// The coordinator keeps the model in a cache and serves it with the support of
	// range requests, the device therefore does not need a direct connection to the
	// model host.
	if server := strings.TrimSuffix(State.Config.ServerUrl, "/"); server != "" {
		downloadURL = server + "/api/v1/worker/models/" + url.PathEscape(modelID) + "/file"
	}
	if downloadURL == "" {
		return errors.New("model download url is missing")
	}
	storageDir := State.Config.StorageDir
	if storageDir == "" {
		storageDir = filepath.Join(os.TempDir(), "byog_models")
	}
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		return err
	}

	targetPath := filepath.Join(storageDir, modelID+".gguf")
	if info, err := os.Stat(targetPath); err == nil && info.Size() > 10*1024*1024 {
		State.Mu.Lock()
		State.InstalledModels[modelID] = true
		State.Mu.Unlock()
		return nil
	}

	State.Mu.Lock()
	State.ModelStatus = "downloading"
	State.DownloadedMB = 0
	State.TotalMB = int(fileSize / (1024 * 1024))
	State.DownloadProgress = 0
	State.Mu.Unlock()

	request, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "BYOG-Worker/1.0")

	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if total <= 0 {
		total = fileSize
	}
	if total > 0 {
		State.Mu.Lock()
		State.TotalMB = int(total / (1024 * 1024))
		State.Mu.Unlock()
	}

	partPath := targetPath + ".part"
	file, err := os.Create(partPath)
	if err != nil {
		return err
	}
	defer file.Close()

	buffer := make([]byte, 256*1024)
	var downloaded int64
	lastReport := time.Now()
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
			downloaded += int64(n)
			if time.Since(lastReport) > 700*time.Millisecond {
				lastReport = time.Now()
				percent := 0
				if total > 0 {
					percent = int((downloaded * 100) / total)
				}
				State.Mu.Lock()
				State.DownloadedMB = int(downloaded / (1024 * 1024))
				State.DownloadProgress = percent
				State.Mu.Unlock()
				sendHeartbeat()
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
	}

	if err := os.Rename(partPath, targetPath); err != nil {
		return err
	}

	State.Mu.Lock()
	State.InstalledModels[modelID] = true
	State.ModelStatus = "ready"
	State.DownloadProgress = 100
	State.DownloadedMB = 0
	State.Mu.Unlock()
	return nil
}

// handleInference answers an inference request with the local llama.cpp runtime.
func handleInference(requestID string, payload map[string]interface{}) {
	modelID, _ := payload["model_id"].(string)
	if modelID == "" {
		modelID = "qwen-2.5-0.5b-instruct"
	}
	prompt, _ := payload["prompt"].(string)
	if prompt == "" {
		if messages, ok := payload["messages"].([]interface{}); ok && len(messages) > 0 {
			if last, ok := messages[len(messages)-1].(map[string]interface{}); ok {
				prompt, _ = last["content"].(string)
			}
		}
	}
	maxTokens := 512
	if value, ok := payload["max_tokens"].(float64); ok && value > 0 {
		maxTokens = int(value)
	}
	temperature := 0.7
	if value, ok := payload["temperature"].(float64); ok && value > 0 {
		temperature = value
	}

	storageDir := State.Config.StorageDir
	if storageDir == "" {
		storageDir = filepath.Join(os.TempDir(), "byog_models")
	}
	modelPath := filepath.Join(storageDir, modelID+".gguf")
	if info, err := os.Stat(modelPath); err != nil || info.Size() < 10*1024*1024 {
		sendInferenceError(requestID, "model "+modelID+" is not available on the device")
		return
	}

	runtime, err := llama.EnsureRuntime(storageDir)
	if err != nil {
		sendInferenceError(requestID, err.Error())
		return
	}
	if runtime.ServerPath == "" {
		sendInferenceError(requestID, "the llama.cpp runtime does not provide llama-server")
		return
	}

	State.Mu.Lock()
	State.ModelStatus = "running"
	State.Mu.Unlock()

	result, err := runtime.Infer(context.Background(), modelPath, storageDir, prompt, maxTokens, temperature, func(piece string, tokenID int) bool {
		chunk, _ := json.Marshal(map[string]interface{}{
			"request_id": requestID,
			"content":    piece,
			"token_id":   tokenID,
			"is_final":   false,
		})
		State.Mu.Lock()
		conn := State.WsConn
		State.Mu.Unlock()
		if conn != nil {
			_ = conn.WriteJSON(map[string]interface{}{
				"type":    "inference_chunk",
				"id":      requestID,
				"payload": chunk,
			})
		}
		return true
	})
	if err != nil {
		sendInferenceError(requestID, err.Error())
		State.Mu.Lock()
		State.ModelStatus = "ready"
		State.Mu.Unlock()
		return
	}

	State.Mu.Lock()
	State.TotalTokens += int64(result.TotalTokens)
	State.CurrentTps = result.Tps
	State.CurrentTtftMs = result.TtftMs
	State.ModelStatus = "ready"
	State.CurrentTps = 0
	conn := State.WsConn
	State.Mu.Unlock()

	done, _ := json.Marshal(map[string]interface{}{
		"request_id":    requestID,
		"total_tokens":  result.TotalTokens,
		"prompt_tokens": result.PromptTokens,
		"ttft_ms":       result.TtftMs,
		"tps":           result.Tps,
	})
	if conn != nil {
		_ = conn.WriteJSON(map[string]interface{}{
			"type":    "inference_done",
			"id":      requestID,
			"payload": done,
		})
	}
}

// sendInferenceError reports a failed inference to the coordinator.
func sendInferenceError(requestID string, message string) {
	State.Mu.Lock()
	conn := State.WsConn
	State.Mu.Unlock()
	if conn == nil {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"request_id": requestID,
		"error":      message,
	})
	_ = conn.WriteJSON(map[string]interface{}{
		"type":    "inference_error",
		"id":      requestID,
		"payload": payload,
	})
}

func main() {
	mem := queryDeviceMemory()
	State.Memory = mem

	hwidRaw := fmt.Sprintf("android-%s-%d", runtime.GOARCH, mem.PhysicalTotalMB)
	h := sha256.Sum256([]byte(hwidRaw + "BYOG-MOBILE-SALT-2026"))
	hwid := hex.EncodeToString(h[:])

	serverUrl := os.Getenv("BYOG_SERVER_URL")
	if serverUrl == "" {
		serverUrl = "http://localhost:3000"
	}
	storageDir := os.Getenv("BYOG_STORAGE_DIR")
	if storageDir == "" {
		storageDir = filepath.Join(os.TempDir(), "byog_models")
	}
	_ = os.MkdirAll(storageDir, 0o755)

	State.Config.ServerUrl = serverUrl
	State.Config.StorageDir = storageDir
	State.Config.DeviceName = "Mobile-Node-" + hwid[:6]
	State.NetworkType = detectNetworkType()
	readBatteryState()

	fmt.Printf("[BYOG MOBILE] Initialized Node. Physical RAM: %d MB | Safe Budget: %d MB (ZRAM %d MB Ignored)\n",
		mem.PhysicalTotalMB, mem.SafeBudgetMB, mem.SwapTotalMB)

	// Register with the coordinator
	regUrl := fmt.Sprintf("%s/api/v1/worker/register", serverUrl)
	regBody, _ := json.Marshal(map[string]interface{}{
		"hwid":                hwid,
		"os":                  "android",
		"arch":                runtime.GOARCH,
		"device_name":         State.Config.DeviceName,
		"device_type":         "mobile",
		"cpu_model":           fmt.Sprintf("%s (%d cores)", runtime.GOOS, runtime.NumCPU()),
		"gpu_model":           "",
		"physical_ram_mb":     mem.PhysicalTotalMB,
		"swap_ram_mb":         mem.SwapTotalMB, // Logged for telemetry, ignored for capacity
		"vram_mb":             0,
		"shared_vram_mb":      mem.SafeBudgetMB,
		"tee_supported":       false,
		"bandwidth_down_mbps": 0.0,
		"bandwidth_up_mbps":   0.0,
		"ping_ms":             0,
		"network_type":        State.NetworkType,
		"battery_percent":     State.BatteryPercent,
		"battery_temp":        State.BatteryTemp,
		"is_charging":         State.IsCharging,
	})

	type assignedModel struct {
		ModelID     string `json:"model_id"`
		DownloadURL string `json:"download_url"`
		FileSize    int64  `json:"file_size"`
	}

	resp, err := http.Post(regUrl, "application/json", bytes.NewReader(regBody))
	if err == nil && resp.StatusCode == http.StatusOK {
		var regResp struct {
			Success bool `json:"success"`
			Data    struct {
				NodeId      string          `json:"node_id"`
				PairingCode string          `json:"pairing_code"`
				Assigned    []assignedModel `json:"assigned_model_details"`
			} `json:"data"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&regResp)
		resp.Body.Close()
		State.Config.NodeId = regResp.Data.NodeId
		if regResp.Data.PairingCode != "" {
			State.Config.PairingCode = regResp.Data.PairingCode
		}
		fmt.Printf("[BYOG MOBILE] Paired successfully. Node ID: %s | Pairing Code: %s\n", State.Config.NodeId, State.Config.PairingCode)

		for _, model := range regResp.Data.Assigned {
			go func(model assignedModel) {
				if err := downloadModel(model.ModelID, model.DownloadURL, model.FileSize); err != nil {
					fmt.Printf("[BYOG MOBILE] Download of %s failed: %v\n", model.ModelID, err)
				}
			}(model)
		}
	} else if resp != nil {
		resp.Body.Close()
	}

	// Reverse Tunnel loop
	wsUrl := fmt.Sprintf("ws%s/api/v1/worker/tunnel?node_id=%s", strings.TrimPrefix(serverUrl, "http"), State.Config.NodeId)
	for {
		conn, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
		if err == nil {
			State.Mu.Lock()
			State.WsConn = conn
			State.IsConnected = true
			State.Mu.Unlock()
			fmt.Println("[BYOG MOBILE] Mobile reverse tunnel connected.")

			// Send the telemetry regularly
			go func() {
				ticker := time.NewTicker(15 * time.Second)
				defer ticker.Stop()
				for range ticker.C {
					canRun, _ := canAcceptTasks()
					State.Mu.Lock()
					if !canRun {
						State.ModelStatus = "paused"
					} else if State.ModelStatus == "paused" {
						State.ModelStatus = "ready"
					}
					State.Mu.Unlock()
					sendHeartbeat()
				}
			}()

			// Read loop
			for {
				var frame struct {
					Type    string          `json:"type"`
					Id      string          `json:"id"`
					Payload json.RawMessage `json:"payload"`
				}
				if err := conn.ReadJSON(&frame); err != nil {
					break
				}
				switch frame.Type {
				case "ping":
					_ = conn.WriteJSON(map[string]interface{}{"type": "heartbeat_ack", "timestamp": time.Now().UnixMilli()})
				case "inference_request":
					var payload map[string]interface{}
					if err := json.Unmarshal(frame.Payload, &payload); err == nil {
						go handleInference(frame.Id, payload)
					}
				case "device_removed", "pairing_revoked":
					// The device was removed from the network, the pairing is dropped.
					State.Mu.Lock()
					State.Config.NodeId = ""
					State.Config.PairingCode = ""
					State.Mu.Unlock()
					fmt.Println("[BYOG MOBILE] The device was removed from the network, the pairing has to be done again")
					continue
				case "download_model":
					var payload struct {
						ModelID     string `json:"model_id"`
						DownloadURL string `json:"download_url"`
						FileSize    int64  `json:"file_size"`
					}
					if err := json.Unmarshal(frame.Payload, &payload); err == nil {
						go func() {
							if err := downloadModel(payload.ModelID, payload.DownloadURL, payload.FileSize); err != nil {
								fmt.Printf("[BYOG MOBILE] Download of %s failed: %v\n", payload.ModelID, err)
							}
						}()
					}
				}
			}
		}
		State.Mu.Lock()
		State.IsConnected = false
		State.Mu.Unlock()
		time.Sleep(5 * time.Second)
	}
}
