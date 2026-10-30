package main

// The page of the client lives on the site, the site does not know the state
// of the client of the computer. A small server that listens on the computer
// of the user gives the state of the client to the page.

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/worker/desktop/installer"
)

// localBridge gives the state of the client to the page of the client.
type localBridge struct {
	listener net.Listener
	server   *http.Server
	token    string
	address  string
	mutex    sync.Mutex
	// pairing tells whether the client is registering itself at the moment.
	pairing bool
}

// bridgeState is the state of the client that the page shows.
type bridgeState struct {
	NodeId           string  `json:"node_id"`
	DeviceName       string  `json:"device_name"`
	ServerUrl        string  `json:"server_url"`
	AppName          string  `json:"app_name"`
	SystemName       string  `json:"system_name"`
	Language         string  `json:"language"`
	PairingCode      string  `json:"pairing_code"`
	Paired           bool    `json:"paired"`
	Connected        bool    `json:"connected"`
	ModelName        string  `json:"model_name"`
	ModelStatus      string  `json:"model_status"`
	DownloadProgress int     `json:"download_progress"`
	CurrentTps       float64 `json:"current_tps"`
	TotalTokens      int64               `json:"total_tokens"`
	LastError        string              `json:"last_error"`
	Disks            []DiskPartitionInfo `json:"disks"`
	PhysicalRamMB    int                 `json:"physical_ram_mb"`
	SwapRamMB        int                 `json:"swap_ram_mb"`
	CpuCores         int                 `json:"cpu_cores"`
	CpuModel         string              `json:"cpu_model"`
	GpuModel         string              `json:"gpu_model"`
	VramMB           int                 `json:"vram_mb"`
	SharedVramMB     int                 `json:"shared_vram_mb"`
	TotalDiskGB      int                 `json:"total_disk_gb"`
	FreeDiskGB       int                 `json:"free_disk_gb"`
	BandwidthDown    float64             `json:"bandwidth_down_mbps"`
	BandwidthUp      float64             `json:"bandwidth_up_mbps"`
	PingMs           int                 `json:"ping_ms"`
	DownloadModelId  string              `json:"download_model_id"`
	DownloadedMB     int                 `json:"downloaded_mb"`
	TotalMB          int                 `json:"total_mb"`
	DownloadSpeedBps float64             `json:"download_speed_bps"`
	EtaSeconds       int                 `json:"eta_seconds"`
	DownloadQueue    []string            `json:"download_queue"`
	InstalledModels  []string            `json:"installed_models"`
	HotStartEnabled  bool                `json:"hot_start_enabled"`
	AllocatedDiskGB  int                 `json:"allocated_disk_gb"`
	FaviconBase64    string              `json:"favicon_base64,omitempty"`
}

// startLocalBridge starts the server of the state of the client.
//
// The server listens on the computer of the user only, the page of the client
// uses it with the key of it.
func startLocalBridge() (*localBridge, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	bridge := &localBridge{
		listener: listener,
		token:    randomKey(),
		address:  "http://" + listener.Addr().String(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/state", bridge.handleState)
	mux.HandleFunc("/pair", bridge.handlePair)
	mux.HandleFunc("/test", bridge.handleTest)
	mux.HandleFunc("/settings", bridge.handleSettings)
	// The page of the first start is served by the client itself. The page of
	// the installation is opened when the language and the policies are known.
	mux.HandleFunc("/setup", bridge.handleSetupPage)
	mux.HandleFunc("/marked.js", bridge.handleMarkedJS)
	mux.HandleFunc("/", bridge.handleSetupPage)
	bridge.server = &http.Server{Handler: withBridgeHeaders(mux)}
	go func() {
		_ = bridge.server.Serve(listener)
	}()
	return bridge, nil
}

// pageAddress returns the address of the page of the client with the key of
// the server of the state.
func (b *localBridge) pageAddress(serverUrl string) string {
	target := strings.TrimRight(serverUrl, "/")
	if target == "" {
		return ""
	}
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "http://" + target
	}
	State.Mu.Lock()
	lang := State.Config.Language
	State.Mu.Unlock()
	if lang == "" {
		lang = "en"
	}
	return target + "/client?local=" + b.address + "&token=" + b.token + "&lang=" + lang
}

// handleState gives the state of the client to the page.
func (b *localBridge) handleState(writer http.ResponseWriter, request *http.Request) {
	if !b.allowed(writer, request) {
		return
	}
	status := readStatus()
	State.Mu.Lock()
	language := State.Config.Language
	serverUrl := State.Config.ServerUrl
	faviconBase64 := State.Config.FaviconBase64
	allocatedDisk := State.Config.MaxDiskGB
	hotStart := State.HotStartEnabled
	queue := append([]string{}, State.DownloadQueue...)
	installed := make([]string, 0, len(State.InstalledModels))
	for name := range State.InstalledModels {
		installed = append(installed, name)
	}
	State.Mu.Unlock()
	sort.Strings(installed)
	hardware := readHardware()
	network, _ := readBandwidth()
	writeBridgeJSON(writer, bridgeState{
		NodeId:           status.NodeId,
		DeviceName:       status.DeviceName,
		ServerUrl:        serverUrl,
		AppName:          status.AppName,
		SystemName:       status.SystemName,
		FaviconBase64:    faviconBase64,
		Language:         language,
		PairingCode:      status.PairingCode,
		Paired:           status.Paired,
		Connected:        status.Connected,
		ModelName:        status.ModelName,
		ModelStatus:      status.ModelStatus,
		DownloadProgress: status.Progress,
		CurrentTps:       status.CurrentTps,
		TotalTokens:      status.TotalTokens,
		LastError:        status.LastError,
		Disks:            hardware.Disks,
		PhysicalRamMB:    hardware.PhysicalRamMB,
		SwapRamMB:        hardware.SwapRamMB,
		CpuCores:         hardware.CpuCores,
		CpuModel:         hardware.CpuModel,
		GpuModel:         hardware.GpuModel,
		VramMB:           hardware.VramMB,
		SharedVramMB:     hardware.SharedVramMB,
		TotalDiskGB:      hardware.TotalDiskGB,
		FreeDiskGB:       hardware.FreeDiskGB,
		BandwidthDown:    network.DownMbps,
		BandwidthUp:      network.UpMbps,
		PingMs:           network.PingMs,
		DownloadModelId:  status.ModelName,
		DownloadedMB:     status.DownloadedMB,
		TotalMB:          status.TotalMB,
		DownloadSpeedBps: status.DownloadSpeedBps,
		EtaSeconds:       status.EtaSeconds,
		DownloadQueue:    queue,
		InstalledModels:  installed,
		HotStartEnabled:  hotStart,
		AllocatedDiskGB:  allocatedDisk,
	})
}

// handlePair pairs the client with the code of the installation.
func (b *localBridge) handlePair(writer http.ResponseWriter, request *http.Request) {
	if !b.allowed(writer, request) {
		return
	}
	var payload struct {
		Code      string `json:"code"`
		ServerUrl string `json:"server_url"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeBridgeJSON(writer, map[string]any{"success": false, "message": err.Error()})
		return
	}
	code := strings.ToUpper(strings.TrimSpace(payload.Code))
	if code == "" {
		writeBridgeJSON(writer, map[string]any{"success": false, "message": "the pairing code is empty"})
		return
	}
	State.Mu.Lock()
	State.Config.PairingCode = code
	if payload.ServerUrl != "" {
		State.Config.ServerUrl = payload.ServerUrl
	}
	State.Mu.Unlock()
	saveConfig(State.Config)
	updateStatus(func(status *AppStatus) { status.LastError = "" })

	b.mutex.Lock()
	if b.pairing {
		b.mutex.Unlock()
		writeBridgeJSON(writer, map[string]any{
			"success": false,
			"message": "the pairing of the client is already running",
		})
		return
	}
	b.pairing = true
	b.mutex.Unlock()

	go func() {
		defer func() {
			b.mutex.Lock()
			b.pairing = false
			b.mutex.Unlock()
		}()
		result, err := registerWorker()
		if err != nil {
			updateStatus(func(status *AppStatus) { status.LastError = err.Error() })
			return
		}
		updateInstallationBranding(result.AppName, result.SystemName, result.FaviconB64)
		startTunnel()
	}()
	writeBridgeJSON(writer, map[string]any{"success": true})
}

// handleTest runs the model of the client with the message of the page.
func (b *localBridge) handleTest(writer http.ResponseWriter, request *http.Request) {
	if !b.allowed(writer, request) {
		return
	}
	var payload struct {
		Prompt string `json:"prompt"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeBridgeJSON(writer, map[string]any{"success": false, "message": err.Error()})
		return
	}
	answer, err := runModelTest(payload.Prompt)
	if err != nil {
		writeBridgeJSON(writer, map[string]any{"success": false, "message": err.Error()})
		return
	}
	writeBridgeJSON(writer, map[string]any{"success": true, "data": map[string]any{"reply": answer}})
}

// handleSettings saves the settings of the client that the page changes.
func (b *localBridge) handleSettings(writer http.ResponseWriter, request *http.Request) {
	if !b.allowed(writer, request) {
		return
	}
	var payload struct {
		ServerUrl       string `json:"server_url"`
		Language        string `json:"language"`
		AllocatedDiskGB *int   `json:"allocated_disk_gb"`
		HotStartEnabled *bool  `json:"hot_start_enabled"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		writeBridgeJSON(writer, map[string]any{"success": false, "message": err.Error()})
		return
	}
	changed := false
	State.Mu.Lock()
	if payload.ServerUrl != "" {
		State.Config.ServerUrl = payload.ServerUrl
	}
	if payload.Language != "" {
		State.Config.Language = payload.Language
		clientLanguage = payload.Language
		installer.UpdateUninstallerLanguage(payload.Language)
	}
	// The page of the client and the window of the client show the same
	// settings. A setting that the page changes is applied by the client at
	// once, the coordinator learns it with the next heartbeat.
	if payload.AllocatedDiskGB != nil && *payload.AllocatedDiskGB >= 0 {
		State.Config.MaxDiskGB = *payload.AllocatedDiskGB
		changed = true
	}
	if payload.HotStartEnabled != nil {
		State.HotStartEnabled = *payload.HotStartEnabled
		changed = true
	}
	State.Mu.Unlock()
	saveConfig(State.Config)
	if changed {
		triggerHeartbeat()
	}
	writeBridgeJSON(writer, map[string]any{"success": true})
}

// allowed tells whether the request of the page carries the key of the
// server of the state.
func (b *localBridge) allowed(writer http.ResponseWriter, request *http.Request) bool {
	key := request.Header.Get("X-Client-Key")
	if key == "" {
		key = request.URL.Query().Get("token")
	}
	if key != b.token {
		writeBridgeJSON(writer, map[string]any{"success": false, "message": "the key of the client is not known"})
		return false
	}
	return true
}

// withBridgeHeaders lets the page of the site use the server of the state.
func withBridgeHeaders(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", "*")
		writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Client-Key")
		writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if request.Method == http.MethodOptions {
			writer.WriteHeader(http.StatusNoContent)
			return
		}
		handler.ServeHTTP(writer, request)
	})
}

// writeBridgeJSON writes the answer of the server of the state.
func writeBridgeJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(value)
}

// randomKey returns a key for the server of the state.
func randomKey() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "byog"
	}
	return hex.EncodeToString(buffer)
}
