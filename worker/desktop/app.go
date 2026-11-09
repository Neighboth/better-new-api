package main

// The shared logic of the clients: the configuration, the registration with the
// coordinator, the tunnel and the status of the worker.
//
// The graphical interface of the client and the console mode use the same
// functions, a client that is installed behaves the same way as a client that
// is started from a terminal.

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/pkg/embedcfg"
	"github.com/QuantumNous/new-api/worker/workerurl"

	"github.com/gorilla/websocket"
)

// AppStatus is the state of the worker that the interface shows.
type AppStatus struct {
	AppName      string
	SystemName   string
	ServerUrl    string
	NodeId       string
	DeviceName   string
	Paired       bool
	Connected    bool
	ModelStatus  string
	DownloadedMB int
	TotalMB      int
	Progress     int
	// The speed of the transfer and the time that remains follow the bytes
	// that arrive, the window shows them next to the progress.
	DownloadSpeedBps float64
	EtaSeconds       int
	CurrentTps       float64
	TotalTokens      int64
	ModelName        string
	// The model of the running transfer. The name above falls back to the
	// model of the memory when no download runs, the identity of the
	// progress needs the honest value: an empty text means "no transfer".
	DownloadModelId string
	HotStartEnabled bool
	PairingCode     string
	HasCode         bool
	LastError       string
	FaviconPath     string
	Started         bool
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

// embeddedSettings reads the settings of the installation from the program of
// the client.
//
// The dashboard writes the name, the icon and the address of the installation
// into the program, the client does not ask the user for them.
func embeddedSettings() (serverUrl string, systemName string, appName string, faviconBase64 string, ok bool) {
	executable, err := os.Executable()
	if err != nil {
		return "", "", "", "", false
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		return "", "", "", "", false
	}
	raw, found := embedcfg.Extract(data)
	if !found {
		return "", "", "", "", false
	}
	var payload struct {
		ServerUrl     string `json:"server_url"`
		SystemName    string `json:"system_name"`
		AppName       string `json:"app_name"`
		FaviconBase64 string `json:"favicon_base64"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", "", "", "", false
	}
	return payload.ServerUrl, payload.SystemName, payload.AppName, payload.FaviconBase64, true
}

// loadConfig reads the configuration of the worker from the disk.
func loadConfig() Config {
	var config Config
	if data, err := os.ReadFile(configPath()); err == nil {
		_ = json.Unmarshal(data, &config)
	}
	if config.Language == "" {
		// The installer of the client asks the language of the user, the page
		// of the first start shows the same language.
		config.Language = installerLanguage()
	}
	return config
}

// installerLanguage returns the language that the installer of the client
// chose.
//
// The installation writes the language into a small file, the language of the
// user is known before the page of the first start is shown.
func installerLanguage() string {
	if root := os.Getenv("BYOG_STORAGE_DIR"); root != "" {
		// The installation of the system writes the file into the folder of
		// the user, a custom folder is not written by it.
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".byog", "installer.json"))
	if err != nil {
		return ""
	}
	var payload struct {
		Language string `json:"language"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	language := strings.TrimSpace(payload.Language)
	for _, code := range clientLanguages {
		if code == language {
			return language
		}
	}
	return ""
}

// saveConfig writes the configuration of the worker to the disk.
func saveConfig(config Config) {
	if data, err := json.MarshalIndent(config, "", "  "); err == nil {
		_ = os.WriteFile(configPath(), data, 0600)
		_ = os.Chmod(configPath(), 0600)
	}
}

// applyConfig puts the configuration of the worker into the state.
func applyConfig(config Config) {
	State.Mu.Lock()
	State.Config = config
	if config.AllocatedRamGB > 0 {
		State.AllocatedRamGB = config.AllocatedRamGB
	}
	if config.AllocatedVramGB > 0 {
		State.AllocatedVramGB = config.AllocatedVramGB
	}
	if config.AllocatedCores > 0 {
		State.AllocatedCores = config.AllocatedCores
	}
	if config.Disks != "" {
		State.Disks = config.Disks
	}
	State.HotStartEnabled = config.HotStartEnabled
	State.Mu.Unlock()

	if config.Language != "" {
		// The text of the interface follows the language of the user.
		clientLanguage = config.Language
	}

	updateStatus(func(status *AppStatus) {
		if config.AppName != "" {
			status.AppName = config.AppName
		}
		if config.SystemName != "" {
			status.SystemName = config.SystemName
		}
		status.ServerUrl = config.ServerUrl
		status.NodeId = config.NodeId
		status.DeviceName = config.DeviceName
		status.PairingCode = config.PairingCode
		status.HasCode = config.PairingCode != ""
		status.Paired = config.PairingCode != "" && config.NodeId != ""
	})
}

// registerResult is the answer of the coordinator to a registration.
type registerResult struct {
	AppName     string
	SystemName  string
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
	nodeId := State.Config.NodeId
	secret := State.Config.WorkerSecret
	deviceName := State.Config.DeviceName
	hwid := State.Config.HWID
	State.Mu.Unlock()

	if serverUrl == "" {
		result.ServerError = "Sunucu adresi eksik"
		return result, fmt.Errorf("%s", result.ServerError)
	}
	if err := workerurl.Validate(serverUrl); err != nil {
		result.ServerError = err.Error()
		return result, err
	}
	if pairingCode == "" {
		result.ServerError = "Eşleştirme kodu eksik"
		return result, fmt.Errorf("%s", result.ServerError)
	}

	// The hardware of the computer is read on the computer. A value that cannot
	// be measured is reported as zero and not as a number that the coordinator
	// would show in the panel of the network.
	hardware := readHardware()
	network, _ := readBandwidth()

	body, _ := json.Marshal(map[string]interface{}{
		"hwid":                hwid,
		"node_id":             nodeId,
		"pairing_code":        pairingCode,
		"os":                  runtime.GOOS,
		"arch":                runtime.GOARCH,
		"device_name":         deviceName,
		"device_type":         "desktop",
		"cpu_model":           hardware.CpuModel,
		"cpu_cores":           hardware.CpuCores,
		"gpu_model":           hardware.GpuModel,
		"physical_ram_mb":     hardware.PhysicalRamMB,
		"swap_ram_mb":         hardware.SwapRamMB,
		"vram_mb":             hardware.VramMB,
		"shared_vram_mb":      hardware.SharedVramMB,
		"tee_supported":       false,
		"tee_type":            "none",
		"bandwidth_down_mbps": network.DownMbps,
		"bandwidth_up_mbps":   network.UpMbps,
		"ping_ms":             network.PingMs,
		"network_type":        "ethernet",
		"total_disk_gb":       hardware.TotalDiskGB,
		"free_disk_gb":        hardware.FreeDiskGB,
		"disks":               json.RawMessage(hardware.disksJSON()),
		"installed_models":    installedModelsJSON(),
		"hot_start_enabled":   State.HotStartEnabled,
	})

	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/worker/register", serverUrl), bytes.NewReader(body))
	if err != nil {
		return result, err
	}
	request.Header.Set("Content-Type", "application/json")
	if secret != "" {
		request.Header.Set("Authorization", "Bearer "+secret)
	}
	response, err := (&http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(request)
	if err != nil {
		result.ServerError = "Sunucuya ulaşılamadı: " + err.Error()
		return result, err
	}
	defer response.Body.Close()

	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			NodeId       string `json:"node_id"`
			WorkerSecret string `json:"worker_secret"`
			PairingCode  string `json:"pairing_code"`
			UserId       int    `json:"user_id"`
			IsPaired     bool   `json:"is_paired"`
			AppName      string `json:"app_name"`
			SystemName   string `json:"system_name"`
			Favicon      string `json:"favicon"`
			FaviconB64   string `json:"favicon_base64"`
			Assigned     []struct {
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
	if payload.Data.WorkerSecret != "" {
		State.Config.WorkerSecret = payload.Data.WorkerSecret
	}
	if payload.Data.PairingCode != "" {
		State.Config.PairingCode = payload.Data.PairingCode
	}
	State.Mu.Unlock()
	saveConfig(State.Config)

	result.AppName = installationDisplayName(payload.Data.AppName, payload.Data.SystemName)
	result.SystemName = payload.Data.SystemName
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
		status.SystemName = result.SystemName
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
var connectionWatchdogOnce sync.Once

func startConnectionWatchdog() {
	connectionWatchdogOnce.Do(func() {
		recoverForever("connectionWatchdog", func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			httpClient := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			for range ticker.C {
				State.Mu.Lock()
				serverUrl := strings.TrimRight(State.Config.ServerUrl, "/")
				nodeId := State.Config.NodeId
				secret := State.Config.WorkerSecret
				State.Mu.Unlock()

				if serverUrl == "" || nodeId == "" || workerurl.Validate(serverUrl) != nil {
					continue
				}

				checkUrl := fmt.Sprintf("%s/api/v1/worker/status?node_id=%s", serverUrl, nodeId)
				req, err := http.NewRequest(http.MethodGet, checkUrl, nil)
				if err != nil {
					continue
				}
				req.Header.Set("Authorization", "Bearer "+secret)
				resp, err := httpClient.Do(req)
				if err == nil {
					if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized {
						fmt.Printf("[BYOG Worker] Watchdog detected node %s was unlinked/removed on server (HTTP %d). Resetting pairing...\n", nodeId, resp.StatusCode)
						State.Mu.Lock()
						State.Config.NodeId = ""
						State.Config.WorkerSecret = ""
						State.Config.PairingCode = ""
						State.IsConnected = false
						cfg := State.Config
						wsConn := State.WsConn
						State.WsConn = nil
						State.Mu.Unlock()
						if wsConn != nil {
							_ = wsConn.Close()
						}
						saveConfig(cfg)
						updateStatus(func(status *AppStatus) {
							status.Connected = false
							status.Paired = false
							status.NodeId = ""
						})
					}
					_ = resp.Body.Close()
				}
			}
		})
	})
}

func startTunnel() {
	updateStatus(func(status *AppStatus) { status.Started = true })
	startConnectionWatchdog()
	recoverForever("tunnel", func() {
		for {
			State.Mu.Lock()
			serverUrl := strings.TrimRight(State.Config.ServerUrl, "/")
			nodeId := State.Config.NodeId
			secret := State.Config.WorkerSecret
			State.Mu.Unlock()

			if serverUrl != "" && nodeId != "" && workerurl.Validate(serverUrl) == nil {
				wsUrl := fmt.Sprintf("ws%s/api/v1/worker/tunnel?node_id=%s",
					strings.TrimPrefix(serverUrl, "http"), nodeId)
				conn, resp, err := websocket.DefaultDialer.Dial(wsUrl, http.Header{"Authorization": []string{"Bearer " + secret}})
				if err != nil {
					if resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized) {
						fmt.Println("[BYOG Worker] Device was removed on server (404/401). Resetting pairing...")
						State.Mu.Lock()
						State.Config.NodeId = ""
						State.Config.WorkerSecret = ""
						State.Config.PairingCode = ""
						State.IsConnected = false
						cfg := State.Config
						State.Mu.Unlock()
						saveConfig(cfg)
						updateStatus(func(status *AppStatus) {
							status.Connected = false
							status.Paired = false
							status.NodeId = ""
						})
					}
				} else {
					State.Mu.Lock()
					State.WsConn = conn
					State.IsConnected = true
					State.Mu.Unlock()
					updateStatus(func(status *AppStatus) { status.Connected = true })
					fmt.Println("[BYOG Worker] Reverse tunnel connected. Listening for inference tasks...")

					// The heartbeat of the client is one loop for the life of
					// the program. A loop for every connection of the tunnel
					// sent a heartbeat of every connection that the client had
					// ever opened: after a night of reconnections the client
					// held a hundred loops and the coordinator throttled the
					// device.
					startHeartbeatOnce()
					handleWebSocketMessages(conn)

					State.Mu.Lock()
					State.IsConnected = false
					State.WsConn = nil
					State.Mu.Unlock()
					updateStatus(func(status *AppStatus) { status.Connected = false })
				}
			}
			time.Sleep(3 * time.Second)
		}
	})
}

// stopTunnel closes the active websocket connection if open.
func stopTunnel() {
	State.Mu.Lock()
	if State.WsConn != nil {
		_ = State.WsConn.Close()
		State.WsConn = nil
	}
	State.IsConnected = false
	State.Mu.Unlock()
	updateStatus(func(status *AppStatus) {
		status.Connected = false
		status.Started = false
	})
}

// heartbeatOnce guards the single heartbeat loop of the client.
var heartbeatOnce sync.Once

// startHeartbeatOnce starts the heartbeat of the client one time.
func startHeartbeatOnce() {
	heartbeatOnce.Do(func() {
		recoverForever("heartbeat", startHeartbeatLoop)
	})
}

// startBackgroundTasks starts the heartbeat of the worker and the search for
// updates.
func startBackgroundTasks() {
	startDownloadLoop()
	startBandwidthLoop()
	// The hardware of the computer is read before the first registration, the
	// values are cached and they are refreshed while the client runs.
	safeGo("hardware", func() {
		_ = readHardware()
	})
	recoverForever("self-update", func() {
		time.Sleep(2 * time.Second)
		checkSelfUpdate()
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			checkSelfUpdate()
		}
	})

	// The coordinator is asked for the assignment of the models again. A model
	// that the administrator assigns after the first registration of the client
	// would otherwise never reach the device. The registration is idempotent,
	// the node is found by the hardware id.
	recoverForever("registration", func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			State.Mu.Lock()
			registered := State.Config.NodeId != "" && State.Config.PairingCode != ""
			State.Mu.Unlock()
			if !registered {
				continue
			}
			if _, err := registerWorker(); err != nil {
				// The tunnel of the client reports the connection state. A
				// failed refresh must not replace the message of the page with
				// a stale error.
				continue
			}
		}
	})
}

// syncStatus copies the state of the worker into the state of the interface.
func syncStatus() {
	State.Mu.Lock()
	connected := State.IsConnected
	modelStatus := State.ModelStatus
	downloaded := State.DownloadedMB
	total := State.TotalMB
	progress := State.DownloadProgress
	speed := State.DownloadSpeedBps
	eta := State.EtaSeconds
	tps := State.CurrentTps
	tokens := State.TotalTokens
	model := State.ActiveVramModel
	downloading := State.DownloadModelId
	hotStart := State.HotStartEnabled
	State.Mu.Unlock()

	updateStatus(func(status *AppStatus) {
		status.Connected = connected
		status.ModelStatus = modelStatus
		status.DownloadedMB = downloaded
		status.TotalMB = total
		status.Progress = progress
		status.DownloadSpeedBps = speed
		status.EtaSeconds = eta
		status.CurrentTps = tps
		status.TotalTokens = tokens
		status.HotStartEnabled = hotStart
		status.DownloadModelId = downloading
		if downloading != "" {
			status.ModelName = downloading
		} else {
			status.ModelName = model
		}
	})
}

// installedModelsJSON returns the models of the client as a JSON array.
func installedModelsJSON() string {
	State.Mu.Lock()
	models := make([]string, 0, len(State.InstalledModels))
	for name := range State.InstalledModels {
		models = append(models, name)
	}
	State.Mu.Unlock()
	sort.Strings(models)
	encoded, err := json.Marshal(models)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// installationDisplayName returns the name of the installation.
//
// The name of the installation and the name of the client are shown together,
// the user knows the installation from the name of it.
func installationDisplayName(appName string, systemName string) string {
	name := strings.TrimSpace(systemName)
	if name == "" || name == "New API" || strings.EqualFold(name, "byog") {
		name = strings.TrimSpace(appName)
	}
	s := name
	for {
		lower := strings.ToLower(strings.TrimSpace(s))
		if strings.HasSuffix(lower, "worker") {
			s = strings.TrimSpace(s[:len(s)-6])
		} else if strings.HasSuffix(lower, "byog") {
			s = strings.TrimSpace(s[:len(s)-4])
		} else {
			break
		}
	}
	if s == "" || strings.EqualFold(s, "new api") || strings.EqualFold(s, "byog") {
		return "BYOG Worker"
	}
	return s + " BYOG Worker"
}

// runModelTest runs the model of the client with a message and returns the
// answer of it.
func runModelTest(prompt string) (string, error) {
	status := readStatus()
	if status.ModelName == "" {
		return "", errors.New("the model of the client is not downloaded yet")
	}
	if strings.TrimSpace(prompt) == "" {
		prompt = "Merhaba! Kendini tek cümleyle tanıt."
	}
	State.Mu.Lock()
	storageDir := State.Config.StorageDir
	State.Mu.Unlock()
	engine := NewLocalTransformerEngine(status.ModelName, storageDir)
	result, err := engine.GenerateStream(
		context.Background(),
		prompt,
		96,
		0.7,
		func(piece string, tokenId int) bool { return true },
	)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Text), nil
}
