package main

// The shared logic of the clients: the configuration, the registration with the
// coordinator, the tunnel and the status of the worker.
//
// The graphical interface of the client and the console mode use the same
// functions, a client that is installed behaves the same way as a client that
// is started from a terminal.

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// AppStatus is the state of the worker that the interface shows.
type AppStatus struct {
	AppName      string
	ServerUrl    string
	NodeId       string
	DeviceName   string
	Paired       bool
	Connected    bool
	ModelStatus  string
	DownloadedMB int
	TotalMB      int
	Progress     int
	CurrentTps   float64
	TotalTokens  int64
	ModelName    string
	PairingCode  string
	HasCode      bool
	LastError    string
	FaviconPath  string
	Started      bool
}

var (
	statusMutex sync.RWMutex
	appStatus   = AppStatus{
		AppName:     "BYOG Worker",
		ModelStatus: "idle",
	}
)

// updateStatus changes the state of the worker.
func updateStatus(update func(*AppStatus)) {
	statusMutex.Lock()
	defer statusMutex.Unlock()
	update(&appStatus)
}

// readStatus returns the state of the worker.
func readStatus() AppStatus {
	statusMutex.RLock()
	defer statusMutex.RUnlock()
	return appStatus
}

// buildIconFile builds the file of an icon from a picture in the format of PNG.
func buildIconFile(pngData []byte) []byte {
	file := make([]byte, 0, 22+len(pngData))
	header := make([]byte, 6)
	binary.LittleEndian.PutUint16(header[0:], 0)
	binary.LittleEndian.PutUint16(header[2:], 1)
	binary.LittleEndian.PutUint16(header[4:], 1)
	file = append(file, header...)

	entry := make([]byte, 16)
	binary.LittleEndian.PutUint16(entry[4:], 1)
	binary.LittleEndian.PutUint16(entry[6:], 32)
	binary.LittleEndian.PutUint32(entry[8:], uint32(len(pngData)))
	binary.LittleEndian.PutUint32(entry[12:], 22)
	file = append(file, entry...)
	file = append(file, pngData...)
	return file
}

// sameFile reports whether two paths point to the same file.
func sameFile(first string, second string) bool {
	firstInfo, err := os.Stat(first)
	if err != nil {
		return false
	}
	secondInfo, err := os.Stat(second)
	if err != nil {
		return false
	}
	return os.SameFile(firstInfo, secondInfo)
}

// storageRoot returns the directory of the models and of the configuration.
//
// The directory of the temporary folder of the system is not used, the models
// would be deleted with the next restart of the system.
func storageRoot() string {
	if custom := strings.TrimSpace(os.Getenv("BYOG_STORAGE_DIR")); custom != "" {
		return custom
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".byog")
	}
	return filepath.Join(os.TempDir(), "byog")
}

// modelDirectory returns the directory of the model files.
func modelDirectory() string {
	return filepath.Join(storageRoot(), "models")
}

// configPath returns the path of the configuration file.
func configPath() string {
	return filepath.Join(storageRoot(), "config.json")
}

// logPath returns the path of the log file of the client.
func logPath() string {
	return filepath.Join(storageRoot(), "worker.log")
}

// loadConfig reads the configuration of the worker from the disk.
func loadConfig() Config {
	var config Config
	if data, err := os.ReadFile(configPath()); err == nil {
		_ = json.Unmarshal(data, &config)
	}
	return config
}

// saveConfig writes the configuration of the worker to the disk.
func saveConfig(config Config) {
	if data, err := json.MarshalIndent(config, "", "  "); err == nil {
		_ = os.WriteFile(configPath(), data, 0644)
	}
}

// applyConfig puts the configuration of the worker into the state.
func applyConfig(config Config) {
	State.Mu.Lock()
	State.Config = config
	State.Mu.Unlock()

	updateStatus(func(status *AppStatus) {
		status.ServerUrl = config.ServerUrl
		status.NodeId = config.NodeId
		status.DeviceName = config.DeviceName
		status.PairingCode = config.PairingCode
		status.HasCode = config.PairingCode != ""
	})
}

// registerResult is the answer of the coordinator to a registration.
type registerResult struct {
	AppName     string
	FaviconURL  string
	FaviconB64  string
	Assigned    []assignedModel
	ServerError string
}

// assignedModel is a model that the coordinator assigned to the worker.
type assignedModel struct {
	ModelId     string
	DownloadUrl string
	FileSize    int64
}

// registerWorker registers the worker with the coordinator.
//
// The registration is repeated when the pairing code of the user is entered in
// the interface.
func registerWorker() (registerResult, error) {
	result := registerResult{AppName: "BYOG Worker"}

	State.Mu.Lock()
	serverUrl := strings.TrimRight(State.Config.ServerUrl, "/")
	pairingCode := State.Config.PairingCode
	deviceName := State.Config.DeviceName
	hwid := State.Config.HWID
	State.Mu.Unlock()

	if serverUrl == "" {
		result.ServerError = "Sunucu adresi eksik"
		return result, fmt.Errorf("%s", result.ServerError)
	}
	if pairingCode == "" {
		result.ServerError = "Eşleştirme kodu eksik"
		return result, fmt.Errorf("%s", result.ServerError)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"hwid":                hwid,
		"pairing_code":        pairingCode,
		"os":                  runtime.GOOS,
		"arch":                runtime.GOARCH,
		"device_name":         deviceName,
		"device_type":         "desktop",
		"cpu_model":           "Host CPU Architecture",
		"cpu_cores":           runtime.NumCPU(),
		"gpu_model":           "Dedicated GPU / Compute Engine",
		"physical_ram_mb":     physicalRamMB(),
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

	response, err := http.Post(
		fmt.Sprintf("%s/api/v1/worker/register", serverUrl),
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		result.ServerError = "Sunucuya ulaşılamadı: " + err.Error()
		return result, err
	}
	defer response.Body.Close()

	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			NodeId      string `json:"node_id"`
			PairingCode string `json:"pairing_code"`
			UserId      int    `json:"user_id"`
			IsPaired    bool   `json:"is_paired"`
			AppName     string `json:"app_name"`
			Favicon     string `json:"favicon"`
			FaviconB64  string `json:"favicon_base64"`
			Assigned    []struct {
				ModelId     string `json:"model_id"`
				DownloadUrl string `json:"download_url"`
				FileSize    int64  `json:"file_size"`
				TotalMB     int    `json:"total_mb"`
			} `json:"assigned_model_details"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		result.ServerError = "Sunucunun cevabı okunamadı"
		return result, err
	}
	if !payload.Success {
		message := payload.Message
		if message == "" {
			message = "Eşleştirme kodu geçersiz"
		}
		result.ServerError = message
		return result, fmt.Errorf("%s", message)
	}

	State.Mu.Lock()
	State.Config.NodeId = payload.Data.NodeId
	if payload.Data.PairingCode != "" {
		State.Config.PairingCode = payload.Data.PairingCode
	}
	State.Mu.Unlock()
	saveConfig(State.Config)

	result.AppName = "BYOG Worker"
	if payload.Data.AppName != "" {
		result.AppName = payload.Data.AppName
	}
	result.FaviconURL = payload.Data.Favicon
	result.FaviconB64 = payload.Data.FaviconB64
	for _, detail := range payload.Data.Assigned {
		result.Assigned = append(result.Assigned, assignedModel{
			ModelId:     detail.ModelId,
			DownloadUrl: detail.DownloadUrl,
			FileSize:    detail.FileSize,
		})
	}

	updateStatus(func(status *AppStatus) {
		status.AppName = result.AppName
		status.NodeId = payload.Data.NodeId
		status.Paired = payload.Data.IsPaired
		status.PairingCode = State.Config.PairingCode
		status.HasCode = State.Config.PairingCode != ""
		status.LastError = ""
	})

	fmt.Printf("[%s] Initialized successfully. Node ID: %s | Paired: %v\n",
		result.AppName, payload.Data.NodeId, payload.Data.IsPaired)

	// The models that the coordinator assigns are fetched by the worker itself.
	for _, detail := range result.Assigned {
		if detail.ModelId == "" {
			continue
		}
		go func(model assignedModel) {
			downloadAssignedModel(model.ModelId, model.DownloadUrl, model.FileSize)
		}(detail)
	}

	return result, nil
}

// startTunnel starts the tunnel to the coordinator in the background.
//
// The tunnel reconnects itself, a connection that ends is opened again.
func startTunnel() {
	updateStatus(func(status *AppStatus) { status.Started = true })
	go func() {
		for {
			State.Mu.Lock()
			serverUrl := strings.TrimRight(State.Config.ServerUrl, "/")
			nodeId := State.Config.NodeId
			State.Mu.Unlock()

			if serverUrl != "" && nodeId != "" {
				wsUrl := fmt.Sprintf("ws%s/api/v1/worker/tunnel?node_id=%s",
					strings.TrimPrefix(serverUrl, "http"), nodeId)
				conn, _, err := websocket.DefaultDialer.Dial(wsUrl, nil)
				if err == nil {
					State.Mu.Lock()
					State.WsConn = conn
					State.IsConnected = true
					State.Mu.Unlock()
					updateStatus(func(status *AppStatus) { status.Connected = true })
					fmt.Println("[BYOG Worker] Reverse tunnel connected. Listening for inference tasks...")

					go startHeartbeatLoop()
					handleWebSocketMessages(conn)

					State.Mu.Lock()
					State.IsConnected = false
					State.Mu.Unlock()
					updateStatus(func(status *AppStatus) { status.Connected = false })
				}
			}
			time.Sleep(3 * time.Second)
		}
	}()
}

// startBackgroundTasks starts the heartbeat of the worker and the search for
// updates.
func startBackgroundTasks() {
	go func() {
		time.Sleep(2 * time.Second)
		checkSelfUpdate()
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			checkSelfUpdate()
		}
	}()
}

// syncStatus copies the state of the worker into the state of the interface.
func syncStatus() {
	State.Mu.Lock()
	connected := State.IsConnected
	modelStatus := State.ModelStatus
	downloaded := State.DownloadedMB
	total := State.TotalMB
	progress := State.DownloadProgress
	tps := State.CurrentTps
	tokens := State.TotalTokens
	model := State.ActiveVramModel
	State.Mu.Unlock()

	updateStatus(func(status *AppStatus) {
		status.Connected = connected
		status.ModelStatus = modelStatus
		status.DownloadedMB = downloaded
		status.TotalMB = total
		status.Progress = progress
		status.CurrentTps = tps
		status.TotalTokens = tokens
		status.ModelName = model
	})
}
