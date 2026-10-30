// BYOG Desktop Background Worker Client (byog-worker)
// Zero-Window, System Tray, Gamer Mode, IDM+P2P Downloader & Self-Updater

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/worker/desktop/installer"
	"github.com/QuantumNous/new-api/worker/llama"
	"github.com/gorilla/websocket"
)

const (
	WorkerVersion       = "1.9.1"
	DefaultServerUrl    = "http://localhost:3000"
	DefaultChunkSize    = 4194304 // 4 MB
	ParallelConnections = 16
)

type Config struct {
	ServerUrl         string `json:"server_url"`
	SystemName        string `json:"system_name"`
	AppName           string `json:"app_name"`
	FaviconBase64     string `json:"favicon_base64"`
	Language          string `json:"language"`
	LegalAccepted     bool   `json:"legal_accepted"`
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
	// The model that is transferred now and the queue of the models that wait.
	//
	// The progress of the client belongs to one model: the model that is
	// transferred. A model that ends leaves the queue and it is available for
	// the network at once.
	DownloadModelId  string
	DownloadQueue    []string
	DownloadSpeedBps float64
	EtaSeconds       int
	// The moment the current attempt of a transfer started. A transfer that
	// is retried starts again from the beginning, the moment tells the
	// coordinator that the progress of the node begins again.
	DownloadStartedAt int64
	DownloadError     string
	Mu                sync.Mutex
}

var State = &WorkerState{
	InstalledModels: make(map[string]bool),
	ModelStatus:     "ready",
}

// The download of a model is started by the registration of the client. The
// coordinator is asked again while the client runs, a model that is assigned
// after the first registration has to be fetched as well. The mutex keeps one
// transfer of a model, a repeated assignment does not start the file twice.
var (
	wsWriteMu sync.Mutex
	// heartbeatWake carries a request for an immediate heartbeat of the client.
	heartbeatWake = make(chan struct{}, 1)
)

// triggerHeartbeat asks the heartbeat loop for an immediate report.
//
// A model that ends and a setting that changes have to reach the coordinator at
// once, the panel of the network must not wait for the next tick of the loop.
func triggerHeartbeat() {
	select {
	case heartbeatWake <- struct{}{}:
	default:
	}
}

func sendWsFrame(conn *websocket.Conn, v interface{}) error {
	wsWriteMu.Lock()
	defer wsWriteMu.Unlock()
	if conn == nil {
		return errors.New("nil conn")
	}
	return conn.WriteJSON(v)
}

// defaultIconPNG is the icon of the client, it is the picture of the logo of
// the installation.
//
//go:embed assets/logo.png
var defaultIconPNG []byte

// interfaceFont is the font of the interface of the client.
//
//go:embed assets/DMSans-Regular.ttf
var interfaceFont []byte

// interfaceFontBold is the bold shape of the font of the interface.
//
//go:embed assets/DMSans-Bold.ttf
var interfaceFontBold []byte

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
		case <-heartbeatWake:
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

		// The hardware of the computer is measured here and not in the
		// coordinator. The panel of the network shows the values that the
		// computer reports, a value that cannot be measured stays zero.
		hardware := readHardware()
		network, _ := readBandwidth()

		payload := map[string]interface{}{
			// Zero means that the value cannot be measured on this machine, the
			// platform does not award bonuses for unknown sensor values.
			"cpu_temp":            0.0,
			"gpu_temp":            0.0,
			"battery_temp":        0.0,
			"fan_speed":           0,
			"battery_percent":     0,
			"is_charging":         false,
			"current_tps":         State.CurrentTps,
			"current_ttft_ms":     State.CurrentTtftMs,
			"total_tokens":        State.TotalTokens,
			"installed_models":    modelsList,
			"active_vram_model":   State.ActiveVramModel,
			"model_status":        State.ModelStatus,
			"download_status":     State.ModelStatus,
			"downloaded_mb":       State.DownloadedMB,
			"total_mb":            State.TotalMB,
			"download_progress":   State.DownloadProgress,
			"download_model_id":   State.DownloadModelId,
			"download_speed_bps":  State.DownloadSpeedBps,
			"eta_seconds":         State.EtaSeconds,
			"download_started_at": State.DownloadStartedAt,
			"download_queue":      State.DownloadQueue,
			"hot_start_enabled":   State.HotStartEnabled,
			"is_gamer_mode":       State.IsGamerMode,
			"is_throttled":        State.IsThrottled,

			"physical_ram_mb":     hardware.PhysicalRamMB,
			"swap_ram_mb":         hardware.SwapRamMB,
			"vram_mb":             hardware.VramMB,
			"shared_vram_mb":      hardware.SharedVramMB,
			"cpu_model":           hardware.CpuModel,
			"gpu_model":           hardware.GpuModel,
			"cpu_cores":           hardware.CpuCores,
			"disks":               json.RawMessage(hardware.disksJSON()),
			"total_disk_gb":       hardware.TotalDiskGB,
			"free_disk_gb":        hardware.FreeDiskGB,
			"bandwidth_down_mbps": network.DownMbps,
			"bandwidth_up_mbps":   network.UpMbps,
			"ping_ms":             network.PingMs,
		}

		raw, _ := json.Marshal(payload)
		frame := map[string]interface{}{
			"type":      "heartbeat",
			"payload":   json.RawMessage(raw),
			"timestamp": time.Now().UnixMilli(),
		}

		_ = State.WsConn.WriteJSON(frame)
		State.Mu.Unlock()
	}
}

// progressWriter counts the bytes that a channel of the download receives.
//
// The progress of the download is reported while the transfer runs, the bytes are
// counted as they arrive and not at the end of a channel.
type progressWriter struct {
	writer  io.Writer
	counter *int64
	mutex   *sync.Mutex
}

func (w *progressWriter) Write(data []byte) (int, error) {
	written, err := w.writer.Write(data)
	w.mutex.Lock()
	*w.counter += int64(written)
	w.mutex.Unlock()
	return written, err
}

// downloadJob is one model that the coordinator assigned to the client.
type downloadJob struct {
	ModelId     string
	DownloadUrl string
	FileSize    int64
}

// The client downloads one model at a time.
//
// A model that ends is available at once: the model is marked as installed
// before the next model of the queue starts, therefore a request of the network
// is served with the model that is finished while the next model is fetched in
// the background.
var (
	downloadQueueMutex sync.Mutex
	downloadQueue      []downloadJob
	downloadQueueKnown = map[string]bool{}
	downloadRunMutex   sync.Mutex
	downloadLoopStart  sync.Once
)

// queueModelDownload adds a model of the coordinator to the queue of the client.
//
// A model that is already installed, already queued or already transferred is
// not queued again: the coordinator repeats the assignment while the client
// runs.
func queueModelDownload(modelId string, downloadUrl string, fileSize int64) {
	if strings.TrimSpace(modelId) == "" {
		return
	}
	targetPath := filepath.Join(State.Config.StorageDir, modelId+".gguf")
	if info, err := os.Stat(targetPath); err == nil && info.Size() > 10*1024*1024 {
		State.Mu.Lock()
		State.InstalledModels[modelId] = true
		if State.ActiveVramModel == "" || State.ActiveVramModel == modelId {
			State.ActiveVramModel = modelId
		}
		State.Mu.Unlock()
		return
	}

	downloadQueueMutex.Lock()
	if downloadQueueKnown[modelId] {
		downloadQueueMutex.Unlock()
		return
	}
	downloadQueueKnown[modelId] = true
	downloadQueue = append(downloadQueue, downloadJob{
		ModelId:     modelId,
		DownloadUrl: downloadUrl,
		FileSize:    fileSize,
	})
	downloadQueueMutex.Unlock()

	State.Mu.Lock()
	queued := make([]string, 0, len(downloadQueue))
	for _, job := range downloadQueue {
		queued = append(queued, job.ModelId)
	}
	State.DownloadQueue = queued
	State.Mu.Unlock()

	startDownloadLoop()
}

// downloadAssignedModel queues the model that the coordinator assigned.
//
// The name is kept for the callers of the client, the transfer itself is made
// by the queue of the client.
func downloadAssignedModel(modelId string, downloadUrl string, fileSize int64) {
	queueModelDownload(modelId, downloadUrl, fileSize)
}

// startDownloadLoop starts the queue of the downloads of the client.
func startDownloadLoop() {
	downloadLoopStart.Do(func() {
		recoverForever("download", runDownloadLoop)
	})
}

// runDownloadLoop transfers the models of the queue one after the other.
func runDownloadLoop() {
	for {
		downloadQueueMutex.Lock()
		if len(downloadQueue) == 0 {
			downloadQueueMutex.Unlock()
			time.Sleep(time.Second)
			continue
		}
		job := downloadQueue[0]
		downloadQueue = downloadQueue[1:]
		downloadQueueMutex.Unlock()

		performModelDownload(job)

		downloadQueueMutex.Lock()
		delete(downloadQueueKnown, job.ModelId)
		queued := make([]string, 0, len(downloadQueue))
		for _, item := range downloadQueue {
			queued = append(queued, item.ModelId)
		}
		downloadQueueMutex.Unlock()

		State.Mu.Lock()
		State.DownloadQueue = queued
		State.Mu.Unlock()

		// The coordinator learns that the model is available now and not with
		// the next heartbeat, the first token of the model must not wait.
		triggerHeartbeat()
	}
}

// downloadMeter measures the speed of the transfer of one model.
//
// The speed is the average of the transfer, the time that remains is the size
// that is still missing divided by the speed. A transfer that stalls keeps the
// progress of the model, the panel never shows a value that goes back.
type downloadMeter struct {
	modelId   string
	total     int64
	started   time.Time
	lastAt    time.Time
	lastBytes int64
	speed     float64
	percent   int
	mutex     sync.Mutex
}

func newDownloadMeter(modelId string, total int64) *downloadMeter {
	now := time.Now()
	return &downloadMeter{
		modelId: modelId,
		total:   total,
		started: now,
		lastAt:  now,
	}
}

// report publishes the transferred size of the model.
func (m *downloadMeter) report(downloaded int64) {
	m.mutex.Lock()
	now := time.Now()
	elapsed := now.Sub(m.lastAt).Seconds()
	if elapsed >= 0.25 {
		delta := downloaded - m.lastBytes
		if delta > 0 {
			current := float64(delta) / elapsed
			if m.speed <= 0 {
				m.speed = current
			} else {
				// The average of the last samples, a single slow sample does not
				// move the estimate of the remaining time.
				m.speed = m.speed*0.7 + current*0.3
			}
		}
		m.lastAt = now
		m.lastBytes = downloaded
		if downloaded <= 0 {
			m.lastBytes = 0
		}
	}

	percent := 0
	if m.total > 0 {
		percent = int((downloaded * 100) / m.total)
	}
	if percent < m.percent {
		percent = m.percent
	}
	if percent > 100 {
		percent = 100
	}
	m.percent = percent

	eta := 0
	if m.speed > 1024 && m.total > downloaded {
		eta = int(float64(m.total-downloaded) / m.speed)
		if eta < 0 {
			eta = 0
		}
	}
	speed := m.speed
	m.mutex.Unlock()

	publishDownloadProgress(m.modelId, downloaded, m.total, percent, speed, eta)
}

// publishDownloadProgress publishes the progress of the running transfer.
//
// Only the model that is transferred now is published, the progress of a model
// that waits in the queue is not the progress of the client.
func publishDownloadProgress(modelId string, downloaded int64, total int64, percent int, speed float64, eta int) {
	downloadedMB := int(downloaded / (1024 * 1024))
	totalMB := int(total / (1024 * 1024))
	if total > 0 && totalMB <= 0 {
		totalMB = 1
	}

	State.Mu.Lock()
	active := State.DownloadModelId == "" || State.DownloadModelId == modelId
	if active {
		State.DownloadModelId = modelId
		State.ModelStatus = "downloading"
		State.DownloadedMB = downloadedMB
		if totalMB > 0 {
			State.TotalMB = totalMB
		}
		State.DownloadProgress = percent
		State.DownloadSpeedBps = speed
		State.EtaSeconds = eta
	}
	State.Mu.Unlock()

	if !active {
		return
	}
	updateStatus(func(status *AppStatus) {
		status.ModelStatus = "downloading"
		status.ModelName = modelId
		status.DownloadedMB = downloadedMB
		status.TotalMB = totalMB
		status.Progress = percent
		status.DownloadSpeedBps = speed
		status.EtaSeconds = eta
	})
}

// finishDownload publishes the state of a model that is transferred.
func finishDownload(modelId string, failed bool, err error) {
	State.Mu.Lock()
	State.DownloadModelId = ""
	if failed {
		State.ModelStatus = "error"
		State.DownloadError = ""
		if err != nil {
			State.DownloadError = err.Error()
		}
	} else {
		State.InstalledModels[modelId] = true
		State.ActiveVramModel = modelId
		State.ModelStatus = "ready"
		State.DownloadProgress = 100
		State.DownloadSpeedBps = 0
		State.EtaSeconds = 0
		if info, statErr := os.Stat(filepath.Join(State.Config.StorageDir, modelId+".gguf")); statErr == nil {
			State.DownloadedMB = int(info.Size() / (1024 * 1024))
			State.TotalMB = State.DownloadedMB
		}
	}
	State.Mu.Unlock()
	updateStatus(func(status *AppStatus) {
		status.ModelStatus = State.ModelStatus
		status.DownloadSpeedBps = 0
		status.EtaSeconds = 0
		if !failed {
			status.Progress = 100
		}
	})
}

// IDM-Style 16-Channel Parallel Range Downloader (Section 2.2)
//
// The download reports the transferred amount so that the panel can show the real
// progress of the model that is currently fetched.
func downloadModelIDM(url string, targetPath string, fileSize int64, modelId string) error {
	if fileSize <= 0 {
		return fmt.Errorf("the size of the model is not known")
	}
	partSize := fileSize / int64(ParallelConnections)
	if partSize <= 0 {
		return fmt.Errorf("the model is too small for a parallel download")
	}
	var wg sync.WaitGroup
	errs := make([]error, ParallelConnections)

	var downloaded int64
	var downloadMutex sync.Mutex
	meter := newDownloadMeter(modelId, fileSize)
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
				meter.report(current)
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

			// The transfer of a model takes minutes, the timeout covers the whole
			// download and not a single read.
			client := &http.Client{Timeout: 6 * time.Hour}
			resp, err := client.Do(req)
			if err != nil {
				errs[channelIdx] = err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusPartialContent {
				// The server does not give the piece of the file, it sends the
				// whole file for every request. The download of the file has
				// to be done with a single connection.
				errs[channelIdx] = fmt.Errorf("the server does not support the range: %d", resp.StatusCode)
				return
			}

			partFile := fmt.Sprintf("%s.part%d", targetPath, channelIdx)
			out, err := os.Create(partFile)
			if err != nil {
				errs[channelIdx] = err
				return
			}
			defer out.Close()

			// The bytes are counted while they arrive, the progress of the
			// download follows the transfer. Only the bytes of the piece are
			// read, a server that sends more does not fill the disk.
			expected := endByte - startByte + 1
			written, err := io.CopyN(&progressWriter{writer: out, counter: &downloaded, mutex: &downloadMutex}, resp.Body, expected)
			if err == nil && written != expected {
				err = fmt.Errorf("the piece of the file is incomplete: %d of %d bytes", written, expected)
			}
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
		if err != nil {
			return fmt.Errorf("the part %d of the model could not be read: %w", i, err)
		}
		if _, err := finalFile.Write(pData); err != nil {
			return err
		}
		_ = os.Remove(partFile)
	}

	// A model that is not complete can not be used for an inference.
	if fileSize > 0 {
		info, err := finalFile.Stat()
		if err != nil {
			return err
		}
		if info.Size() != fileSize {
			return fmt.Errorf("the model file is incomplete: %d of %d bytes", info.Size(), fileSize)
		}
	}

	meter.report(fileSize)
	fmt.Printf("[BYOG IDM] Download and verification complete: %s\n", targetPath)
	return nil
}

// performModelDownload transfers a model of the queue into the storage of the
// client.
func performModelDownload(job downloadJob) {
	modelId := job.ModelId
	if modelId == "" {
		return
	}

	downloadRunMutex.Lock()
	defer downloadRunMutex.Unlock()

	targetPath := filepath.Join(State.Config.StorageDir, modelId+".gguf")
	_ = os.MkdirAll(State.Config.StorageDir, 0755)

	if info, err := os.Stat(targetPath); err == nil && info.Size() > 10*1024*1024 {
		State.Mu.Lock()
		State.InstalledModels[modelId] = true
		if State.ActiveVramModel == "" || State.ActiveVramModel == modelId {
			State.ActiveVramModel = modelId
		}
		State.ModelStatus = "ready"
		State.DownloadProgress = 100
		State.DownloadedMB = int(info.Size() / (1024 * 1024))
		State.TotalMB = State.DownloadedMB
		State.Mu.Unlock()
		return
	}

	State.Mu.Lock()
	State.DownloadModelId = modelId
	State.ModelStatus = "downloading"
	State.DownloadedMB = 0
	State.DownloadProgress = 0
	State.DownloadSpeedBps = 0
	State.EtaSeconds = 0
	State.DownloadStartedAt = time.Now().UnixMilli()
	if job.FileSize > 0 {
		State.TotalMB = int(job.FileSize / (1024 * 1024))
	}
	State.Mu.Unlock()
	updateStatus(func(status *AppStatus) {
		status.ModelName = modelId
		status.ModelStatus = "downloading"
		status.Progress = 0
		status.EtaSeconds = 0
	})

	// The coordinator caches the model and gives it to every worker of the
	// installation. When the coordinator does not know the model, the model
	// host of the assignment is used directly.
	primary := ""
	if server := strings.TrimRight(State.Config.ServerUrl, "/"); server != "" {
		primary = server + "/api/v1/worker/models/" + url.PathEscape(modelId) + "/file"
	}
	if primary == "" {
		primary = job.DownloadUrl
	}

	failed := primary == ""
	var lastErr error
	if primary != "" {
		if err := downloadModelIDM(primary, targetPath, job.FileSize, modelId); err != nil {
			fmt.Printf("[BYOG IDM] Download of %s failed: %v\n", modelId, err)
			cleanupModelParts(targetPath)
			// The server does not give the pieces of the file, the file is
			// downloaded with a single connection. The transfer starts again
			// from the first byte, the coordinator is told that the progress
			// begins again.
			State.Mu.Lock()
			State.DownloadStartedAt = time.Now().UnixMilli()
			State.DownloadedMB = 0
			State.DownloadProgress = 0
			State.Mu.Unlock()
			if singleErr := downloadModelSingle(primary, targetPath, job.FileSize, modelId); singleErr != nil {
				lastErr = singleErr
				fmt.Printf("[BYOG IDM] Download of %s failed: %v\n", modelId, singleErr)
				failed = true
			} else {
				failed = false
			}
		}
	}
	if failed && job.DownloadUrl != "" && job.DownloadUrl != primary {
		fmt.Printf("[BYOG IDM] Download of %s is retried from the model host\n", modelId)
		cleanupModelParts(targetPath)
		State.Mu.Lock()
		State.DownloadStartedAt = time.Now().UnixMilli()
		State.DownloadedMB = 0
		State.DownloadProgress = 0
		State.Mu.Unlock()
		if err := downloadModelSingle(job.DownloadUrl, targetPath, job.FileSize, modelId); err != nil {
			lastErr = err
			fmt.Printf("[BYOG IDM] Download of %s from the model host failed: %v\n", modelId, err)
		} else {
			failed = false
		}
	}
	if failed {
		fmt.Printf("[BYOG IDM] The model %s could not be downloaded: %v\n", modelId, lastErr)
		finishDownload(modelId, true, lastErr)
		return
	}

	finishDownload(modelId, false, nil)
}

// cleanupModelParts removes the parts of a failed parallel download.
func cleanupModelParts(targetPath string) {
	_ = os.Remove(targetPath + ".part")
	for index := 0; index < ParallelConnections; index++ {
		_ = os.Remove(fmt.Sprintf("%s.part%d", targetPath, index))
	}
}

// downloadModelSingle downloads a model with a single connection.
//
// The coordinator does not always give the pieces of the file with the range
// requests, the file is downloaded as a whole then. The size of the response is
// used when the size of the assignment is not known or out of date.
func downloadModelSingle(downloadUrl string, targetPath string, fileSize int64, modelId string) error {
	request, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 6 * time.Hour}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("the server answered with %d", response.StatusCode)
	}

	partFile := targetPath + ".part"
	out, err := os.Create(partFile)
	if err != nil {
		return err
	}
	defer out.Close()

	total := fileSize
	if response.ContentLength > 0 {
		total = response.ContentLength
	}
	var downloaded int64
	var mutex sync.Mutex
	writer := &progressWriter{writer: out, counter: &downloaded, mutex: &mutex}
	meter := newDownloadMeter(modelId, total)

	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-ticker.C:
				mutex.Lock()
				current := downloaded
				mutex.Unlock()
				meter.report(current)
			}
		}
	}()
	_, copyErr := io.Copy(writer, response.Body)
	close(progressDone)
	if copyErr != nil {
		return copyErr
	}
	info, err := out.Stat()
	if err != nil {
		return err
	}
	if info.Size() <= 10*1024*1024 {
		return fmt.Errorf("the model file is too small: %d bytes", info.Size())
	}
	if response.ContentLength > 0 && info.Size() != response.ContentLength {
		return fmt.Errorf("the model file is incomplete: %d of %d bytes", info.Size(), response.ContentLength)
	}
	if response.ContentLength <= 0 && fileSize > 0 && info.Size() != fileSize {
		return fmt.Errorf("the model file is incomplete: %d of %d bytes", info.Size(), fileSize)
	}
	meter.report(info.Size())
	return os.Rename(partFile, targetPath)
}

// versionGreater reports whether a version is newer than another version.
func versionGreater(candidate string, current string) bool {
	parse := func(value string) []int {
		parts := strings.Split(strings.TrimSpace(value), ".")
		numbers := make([]int, len(parts))
		for index, part := range parts {
			if end := strings.IndexFunc(part, func(character rune) bool {
				return character < '0' || character > '9'
			}); end >= 0 {
				part = part[:end]
			}
			numbers[index], _ = strconv.Atoi(part)
		}
		return numbers
	}
	left, right := parse(candidate), parse(current)
	for index := 0; index < len(left) || index < len(right); index++ {
		var first, second int
		if index < len(left) {
			first = left[index]
		}
		if index < len(right) {
			second = right[index]
		}
		if first != second {
			return first > second
		}
	}
	return false
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

	if data.Version == "" || !versionGreater(data.Version, WorkerVersion) {
		return
	}
	fmt.Printf("[BYOG UPDATER] New version available: %s (Current: %s). Initiating graceful update...\n", data.Version, WorkerVersion)

	// Drain existing jobs
	for State.ActiveInferences > 0 {
		time.Sleep(500 * time.Millisecond)
	}

	currentExe, err := os.Executable()
	if err != nil {
		return
	}

	downloadPath := data.Downloads[runtime.GOOS+"_"+runtime.GOARCH]
	if downloadPath == "" {
		return
	}
	if !strings.HasPrefix(downloadPath, "http") {
		downloadPath = State.Config.ServerUrl + downloadPath
	}

	// The new program is written next to the program of the client and it is
	// checked before the program of the client is replaced. The installer of
	// Windows is not used by the self-updater, it downloads the program of the
	// client and keeps the branding of the installation.
	newExe := currentExe + ".new"
	_ = os.Remove(newExe)
	newFile, err := os.Create(newExe)
	if err != nil {
		return
	}
	dlResp, err := http.Get(downloadPath)
	if err != nil || dlResp.StatusCode != http.StatusOK {
		if dlResp != nil {
			dlResp.Body.Close()
		}
		newFile.Close()
		_ = os.Remove(newExe)
		return
	}
	written, copyErr := io.Copy(newFile, dlResp.Body)
	dlResp.Body.Close()
	newFile.Close()
	if copyErr != nil || written < 1<<20 {
		_ = os.Remove(newExe)
		return
	}

	// Windows File Lock Solution: Rename current executable to .old
	oldExe := currentExe + ".old"
	_ = os.Remove(oldExe)
	if err := os.Rename(currentExe, oldExe); err != nil {
		_ = os.Remove(newExe)
		logLine("[BYOG UPDATER] the program of the client can not be replaced: %v", err)
		return
	}
	if err := os.Rename(newExe, currentExe); err != nil {
		_ = os.Rename(oldExe, currentExe)
		_ = os.Remove(newExe)
		logLine("[BYOG UPDATER] the new program can not be placed: %v", err)
		return
	}

	// The client of the new version is started before this program ends.
	//
	// The client used to end as soon as the file of the new program was placed,
	// whatever the new program did: a file that the antivirus of the computer
	// quarantined, a program that a policy of the system blocked, and the
	// computer was left without a client until somebody noticed. The new
	// program has to run for a few seconds before this program ends, otherwise
	// the previous program is put back and the client keeps serving.
	if err := startAndVerifyUpdate(currentExe, oldExe); err != nil {
		logLine("[BYOG UPDATER] the new version did not start, the previous version is restored: %v", err)
		_ = os.Remove(currentExe)
		_ = os.Rename(oldExe, currentExe)
		return
	}
	os.Exit(0)
}

// startAndVerifyUpdate starts the program of a new version and reports whether
// it runs.
//
// The new program gets the arguments of this program and it runs for five
// seconds. A program that ends in that time with an error did not start, the
// caller restores the previous program. The wait happens in the loop of the
// update of the client: the worker keeps serving the tasks of the network while
// the new program starts.
func startAndVerifyUpdate(executable string, previous string) error {
	// The mutex of the client is released before the new program starts, the
	// new program is the client of the user from now on.
	releaseSingleInstance()
	command := exec.Command(executable, os.Args[1:]...)
	command.Dir = filepath.Dir(executable)
	hideProcessWindow(command)
	if err := command.Start(); err != nil {
		_ = acquireSingleInstance()
		return fmt.Errorf("the new program can not be started: %w", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- command.Wait()
	}()
	select {
	case err := <-done:
		// The program ended before the check: a working client stays in the
		// background of the computer.
		if err != nil {
			return fmt.Errorf("the new program ended at once: %w", err)
		}
		return fmt.Errorf("the new program ended at once without an error")
	case <-time.After(5 * time.Second):
		// The program runs, the previous program is not needed any more and it
		// is removed when the system releases the file.
		go func() {
			time.Sleep(30 * time.Second)
			_ = os.Remove(previous)
		}()
		return nil
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

// inferenceChunkFrame builds the frame of a generated part of the answer.
//
// The payload has to be a JSON object, a byte slice would be encoded as a base64
// string and the coordinator would not be able to read the answer.
func inferenceChunkFrame(requestID string, content string, tokenID int, final bool) map[string]interface{} {
	payload, _ := json.Marshal(map[string]interface{}{
		"request_id": requestID,
		"content":    content,
		"token_id":   tokenID,
		"is_final":   final,
	})
	return map[string]interface{}{
		"type":    "inference_chunk",
		"id":      requestID,
		"payload": json.RawMessage(payload),
	}
}

// inferenceDoneFrame builds the frame that closes an inference.
func inferenceDoneFrame(requestID string, result *llama.Result) map[string]interface{} {
	payload, _ := json.Marshal(map[string]interface{}{
		"request_id":    requestID,
		"total_tokens":  result.TotalTokens,
		"prompt_tokens": result.PromptTokens,
		"ttft_ms":       result.TtftMs,
		"tps":           result.Tps,
	})
	return map[string]interface{}{
		"type":    "inference_done",
		"id":      requestID,
		"payload": json.RawMessage(payload),
	}
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
				queueModelDownload(dReq.ModelId, dReq.DownloadUrl, dReq.FileSize)
			}

		case "purge_model":
			// The coordinator takes a model out of the memory of the client, the
			// model stays on the disk and it can be used again at once.
			var purgeReq struct {
				ModelId string `json:"model_id"`
			}
			if err := json.Unmarshal(frame.Payload, &purgeReq); err == nil {
				State.Mu.Lock()
				if purgeReq.ModelId == "" || State.ActiveVramModel == purgeReq.ModelId {
					State.ActiveVramModel = ""
				}
				State.Mu.Unlock()
				triggerHeartbeat()
			}

		case "heartbeat_ack":
			// The coordinator decides the settings of the node, the client shows
			// and applies the settings of the panel. The window of the client and
			// the page of the installation must agree on the hot start of the
			// model and on the resources of the computer.
			var ack struct {
				HotStartEnabled *bool `json:"hot_start_enabled"`
				AllocatedRamGB  *int  `json:"allocated_ram_gb"`
				AllocatedVramGB *int  `json:"allocated_vram_gb"`
				AllocatedCores  *int  `json:"allocated_cores"`
				AllocatedDiskGB *int  `json:"allocated_disk_gb"`
			}
			if err := json.Unmarshal(frame.Payload, &ack); err == nil {
				changed := false
				State.Mu.Lock()
				if ack.HotStartEnabled != nil && State.HotStartEnabled != *ack.HotStartEnabled {
					State.HotStartEnabled = *ack.HotStartEnabled
					changed = true
				}
				if ack.AllocatedDiskGB != nil && State.Config.MaxDiskGB != *ack.AllocatedDiskGB {
					State.Config.MaxDiskGB = *ack.AllocatedDiskGB
					changed = true
				}
				State.Mu.Unlock()
				if changed {
					saveConfig(State.Config)
					updateStatus(func(status *AppStatus) { status.HotStartEnabled = State.HotStartEnabled })
				}
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
							"payload": json.RawMessage(respPayload),
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
							"payload": json.RawMessage(respPayload),
						})
						return
					}
					respPayload, _ := json.Marshal(map[string]string{
						"data_hex": hex.EncodeToString(buf),
					})
					_ = conn.WriteJSON(map[string]interface{}{
						"type":    "byte_probe_response",
						"id":      frame.Id,
						"payload": json.RawMessage(respPayload),
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
						"payload": json.RawMessage(respBytes),
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

					fmt.Printf("[INFERENCE] Request %s: model=%s, prompt=%q, maxTokens=%d\n", infReq.RequestId, infReq.ModelId, userPrompt, infReq.MaxTokens)
					engine := NewLocalTransformerEngine(infReq.ModelId, State.Config.StorageDir)
					result, err := engine.GenerateStream(
						context.Background(),
						userPrompt,
						infReq.MaxTokens,
						infReq.Temperature,
						func(piece string, tokenId int) bool {
							_ = sendWsFrame(conn, inferenceChunkFrame(infReq.RequestId, piece, tokenId, false))
							return true
						},
					)
					if err != nil {
						fmt.Printf("[INFERENCE] Error for %s: %v\n", infReq.RequestId, err)
						errorPayload, _ := json.Marshal(map[string]interface{}{
							"request_id": infReq.RequestId,
							"error":      err.Error(),
						})
						_ = sendWsFrame(conn, map[string]interface{}{
							"type":    "inference_error",
							"id":      infReq.RequestId,
							"payload": json.RawMessage(errorPayload),
						})
						State.Mu.Lock()
						State.ModelStatus = "ready"
						State.CurrentTps = 0
						State.Mu.Unlock()
						return
					}

					fmt.Printf("[INFERENCE] Success for %s: tokens=%d, tps=%.2f\n", infReq.RequestId, result.TotalTokens, result.Tps)
					finalPayload, _ := json.Marshal(map[string]interface{}{
						"request_id": infReq.RequestId,
						"content":    "",
						"token_id":   1000 + result.TotalTokens,
						"is_final":   true,
					})
					_ = sendWsFrame(conn, map[string]interface{}{
						"type":    "inference_chunk",
						"id":      infReq.RequestId,
						"payload": json.RawMessage(finalPayload),
					})

					_ = sendWsFrame(conn, inferenceDoneFrame(infReq.RequestId, result))

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

	installFlag := flag.Bool("install", false, "Install the client on this computer")
	uninstallFlag := flag.Bool("uninstall", false, "Remove the client from this computer")
	consoleFlag := flag.Bool("console", false, "Run the client in the terminal")
	serverFlag := flag.String("server", "", "Server URL (e.g. https://your-server.com or http://localhost:3000)")
	codeFlag := flag.String("code", "", "Pairing code from web dashboard")
	deviceFlag := flag.String("device", "", "Custom device name")
	trayFlag := flag.Bool("tray", false, "Start minimized to system tray")
	webViewHelperFlag := flag.Bool("webview-helper", false, "Show the page of the client in the window of the client")
	webViewUrlFlag := flag.String("webview-url", "", "The page of the window of the client")
	webViewTitleFlag := flag.String("webview-title", "", "The name of the window of the client")
	webViewIconFlag := flag.String("webview-icon", "", "The icon of the window of the client")
	flag.Parse()

	if !*consoleFlag {
		detachConsole()
	}

	_ = os.MkdirAll(storageRoot(), 0755)
	redirectOutputToLog()
	fmt.Printf("[MAIN] Starting BYOG Worker v%s, args=%v\n", WorkerVersion, os.Args)

	// The window of the client runs in a process of its own. The runtime of
	// WebView2 of the computer cannot stop the worker, the window of the
	// program shows the state of the worker when the window cannot be opened.
	if *webViewHelperFlag {
		os.Exit(runWebViewHelper(*webViewUrlFlag, *webViewTitleFlag, *webViewIconFlag))
	}

	if *installFlag {
		installApp()
		return
	}
	if *uninstallFlag {
		_ = installer.Uninstall()
		fmt.Println("BYOG Worker was removed from this computer.")
		return
	}

	if *consoleFlag {
		runConsole(*serverFlag, *codeFlag, *deviceFlag)
		return
	}

	// The file of the installation installs the client and starts it.
	if installIfNeeded() {
		fmt.Println("[MAIN] Installed and launched, exiting launcher instance")
		return
	}

	// The client stays in the notification area after its window is closed.
	// Starting the file again shows the window of the client that runs, a
	// second worker would serve the tasks of the computer twice.
	// If started in tray mode (e.g. at system boot/restart), do not bring the
	// running window to front; exit silently.
	if !acquireSingleInstance() {
		fmt.Printf("[MAIN] Single instance already running (trayFlag=%v)\n", *trayFlag)
		if !*trayFlag {
			showRunningInstance()
		}
		return
	}
	fmt.Printf("[MAIN] Single instance acquired, entering runGui (trayFlag=%v)\n", *trayFlag)

	runGui(*serverFlag, *codeFlag, *deviceFlag, *trayFlag)
}

// installIfNeeded installs the client when it is started outside of the program
// directory of the user.
//
// The file of the installation is a copy of the client, it installs the client
// and starts the installed program. The function returns true when the client
// was installed, the program of the installation ends then.
func installIfNeeded() bool {
	if installer.IsInstalled() {
		return false
	}
	installApp()
	return true
}

// installApp installs the client on this computer.
//
// The program of the client is copied into the program directory of the user,
// the entries of the control panel and the shortcut of the start menu are
// written and the client is started.
func installApp() {
	source, err := os.Executable()
	if err != nil {
		fmt.Println("The client can not be installed:", err)
		return
	}
	directory := installer.Directory()
	if err := os.MkdirAll(directory, 0755); err != nil {
		fmt.Println("The directory of the client could not be created:", err)
		return
	}
	target := installer.Executable()
	data, err := os.ReadFile(source)
	if err == nil && !sameFile(source, target) {
		if err := os.WriteFile(target, data, 0755); err != nil {
			fmt.Println("The client could not be copied:", err)
			return
		}
	}
	_ = os.MkdirAll(modelDirectory(), 0755)

	// The icon of the client is the icon of the installation.
	icon := filepath.Join(directory, "byog.ico")
	_ = os.WriteFile(icon, buildIconFile(defaultIconPNG), 0644)

	if err := installer.Install("BYOG Worker", WorkerVersion); err != nil {
		fmt.Println("The entries of the client could not be written:", err)
		return
	}
	fmt.Println("BYOG Worker was installed in", directory)

	command := exec.Command(target)
	command.Dir = directory
	hideProcessWindow(command)
	_ = command.Start()
}

// runConsole starts the client in the terminal.
//
// The clients of the systems that have no graphical interface of their own use
// this mode.
func runConsole(serverFlag string, codeFlag string, deviceFlag string) {
	if err := os.MkdirAll(storageRoot(), 0755); err != nil {
		_ = err
	}

	config := loadConfig()
	if serverFlag != "" {
		config.ServerUrl = serverFlag
	}
	if codeFlag != "" {
		config.PairingCode = codeFlag
	}
	if deviceFlag != "" {
		config.DeviceName = deviceFlag
	}
	if config.ServerUrl == "" {
		config.ServerUrl = DefaultServerUrl
	}
	if config.DeviceName == "" {
		config.DeviceName = "Worker-Node-" + generateHWID()[:6]
	}
	if config.StorageDir == "" {
		config.StorageDir = modelDirectory()
	}
	if config.HWID == "" {
		config.HWID = generateHWID()
	}
	applyConfig(config)
	saveConfig(config)

	if config.PairingCode == "" {
		fmt.Println("----------------------------------------------------------------")
		fmt.Println("BYOG Compute Worker")
		fmt.Println("Open your dashboard, click 'Link Device', and enter the code below.")
		fmt.Println("----------------------------------------------------------------")
		fmt.Print("Enter Pairing Code: ")
		var inputCode string
		_, _ = fmt.Scanln(&inputCode)
		State.Mu.Lock()
		State.Config.PairingCode = strings.TrimSpace(inputCode)
		State.Mu.Unlock()
		saveConfig(State.Config)
	}

	result, err := registerWorker()
	if err != nil {
		fmt.Println("[BYOG WORKER] Registration pending or offline. Connecting to tunnel...")
	} else {
		updateInstallationBranding(result.AppName, result.SystemName, result.FaviconB64)
	}

	startTunnel()
	startBackgroundTasks()

	// The client stays in the foreground.
	for {
		time.Sleep(time.Second)
	}
}
