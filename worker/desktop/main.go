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
	"github.com/QuantumNous/new-api/worker/workerurl"
	"github.com/gorilla/websocket"
)

const (
	WorkerVersion       = "1.10.0"
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
	WorkerSecret      string `json:"worker_secret"`
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
	AllocatedRamGB    float64
	AllocatedVramGB   int
	AllocatedCores    int
	Disks             string
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
	// panel follows the transfer in real time without lag.
	downloadTicker := time.NewTicker(1 * time.Second)
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

		conn := State.WsConn
		State.Mu.Unlock()
		_ = sendWsFrame(conn, frame)
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

func getSystemModelDirName() string {
	State.Mu.Lock()
	sysName := strings.TrimSpace(State.Config.SystemName)
	if sysName == "" {
		sysName = strings.TrimSpace(State.Config.AppName)
	}
	State.Mu.Unlock()
	if sysName == "" {
		sysName = "PixRouter"
	}
	return sysName + " BYOG Models"
}

func pruneDriveModels(driveRoot string) {
	driveRoot = strings.TrimSpace(driveRoot)
	if driveRoot == "" {
		return
	}
	if strings.HasSuffix(driveRoot, ":") {
		driveRoot += `\`
	} else if !strings.HasSuffix(driveRoot, `\`) && !strings.HasSuffix(driveRoot, `/`) {
		driveRoot += `\`
	}
	name := getSystemModelDirName()
	candidates := []string{
		filepath.Join(driveRoot, "models"),
		filepath.Join(driveRoot, name),
		filepath.Join(driveRoot, "PixRouter BYOG Models"),
		filepath.Join(driveRoot, "BYOG Models"),
	}
	for _, c := range candidates {
		_ = removeDirectoryRobust(c)
	}
}

func pruneAllModelsIfZeroDisk() {
	State.Mu.Lock()
	maxDisk := State.Config.MaxDiskGB
	disksJson := State.Disks
	State.Mu.Unlock()
	totalAlloc := maxDisk
	if disksJson != "" {
		var disks []DiskPartitionInfo
		if err := json.Unmarshal([]byte(disksJson), &disks); err == nil && len(disks) > 0 {
			totalAlloc = 0
			for _, d := range disks {
				if d.AllocatedGB > 0 {
					totalAlloc += d.AllocatedGB
				} else {
					pruneDriveModels(d.Drive)
				}
			}
		}
	}
	if totalAlloc <= 0 {
		_ = removeDirectoryRobust(modelDirectory())
	}
}

func candidateModelDirs() []string {
	dirs := []string{}
	if State.Config.StorageDir != "" {
		dirs = append(dirs, State.Config.StorageDir)
	}
	name := getSystemModelDirName()

	State.Mu.Lock()
	disksJson := State.Disks
	State.Mu.Unlock()

	var disks []DiskPartitionInfo
	if disksJson != "" {
		_ = json.Unmarshal([]byte(disksJson), &disks)
	}
	if len(disks) == 0 {
		disks = readHardware().Disks
	}

	for _, d := range disks {
		drive := strings.TrimSpace(d.Drive)
		if drive == "" {
			continue
		}
		driveRoot := drive
		if strings.HasSuffix(driveRoot, ":") {
			driveRoot += `\`
		} else if !strings.HasSuffix(driveRoot, `\`) && !strings.HasSuffix(driveRoot, `/`) {
			driveRoot += `\`
		}
		dir := filepath.Join(driveRoot, name)
		if d.AllocatedGB <= 0 {
			// User set this drive to 0 GB: prune model directory and models folder robustly
			pruneDriveModels(driveRoot)
			continue
		}
		_ = os.MkdirAll(dir, 0755)
		setHiddenFolder(dir)
		dirs = append(dirs, dir)
	}
	if len(dirs) == 0 {
		pruneAllModelsIfZeroDisk()
		dirs = append(dirs, modelDirectory())
	}
	return dirs
}

func findInstalledModelPath(modelId string) (string, bool) {
	for _, dir := range candidateModelDirs() {
		p := filepath.Join(dir, modelId+".gguf")
		if info, err := os.Stat(p); err == nil && info.Size() > 10*1024*1024 {
			return p, true
		}
	}
	return "", false
}

func bestDownloadDir(fileSize int64) string {
	dirs := candidateModelDirs()
	if len(dirs) == 0 {
		return State.Config.StorageDir
	}
	for _, dir := range dirs {
		if strings.Contains(dir, getSystemModelDirName()) {
			_ = os.MkdirAll(dir, 0755)
			setHiddenFolder(dir)
			return dir
		}
	}
	return dirs[0]
}

// queueModelDownload adds a model of the coordinator to the queue of the client.
//
// A model that is already installed, already queued or already transferred is
// not queued again: the coordinator repeats the assignment while the client
// runs.
func queueModelDownload(modelId string, downloadUrl string, fileSize int64) {
	if strings.TrimSpace(modelId) == "" {
		return
	}
	if _, ok := findInstalledModelPath(modelId); ok {
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
		if percent < State.DownloadProgress && State.DownloadModelId == modelId {
			percent = State.DownloadProgress
		}
		if downloadedMB < State.DownloadedMB && State.DownloadModelId == modelId {
			downloadedMB = State.DownloadedMB
		}
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
		if totalMB > 0 {
			status.TotalMB = totalMB
		}
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

// errRangeUnsupported reports a model host that answers a range request with
// the whole file. The parallel download stops at once and the caller falls
// back to the transfer with a single connection.
var errRangeUnsupported = errors.New("the server does not support the range requests")

// chunkAttempts is the number of tries of a single channel of the parallel
// download. A channel resumes the bytes it already holds, a stalled mirror
// does not fail the whole transfer.
const chunkAttempts = 3

// resolveRemoteFileSize probes the real size of a model file.
//
// The size of the assignment of the coordinator can be out of date, a
// parallel download that is cut from a wrong size produces a model that is
// missing its tail and crashes the inference. The probe follows the redirects
// of the coordinator to the model host and asks the host itself.
func resolveRemoteFileSize(remoteUrl string) int64 {
	client := &http.Client{Timeout: 30 * time.Second}
	request, err := http.NewRequest(http.MethodHead, remoteUrl, nil)
	if err != nil {
		return 0
	}
	request.Header.Set("User-Agent", "BYOG-Worker/1.0")
	request.Header.Set("Accept-Encoding", "identity")
	response, err := client.Do(request)
	if err == nil {
		size := response.ContentLength
		response.Body.Close()
		if size > 0 {
			return size
		}
	}
	// A host that refuses HEAD is asked with the first byte of the file, the
	// answer of the range request carries the real size.
	request, err = http.NewRequest(http.MethodGet, remoteUrl, nil)
	if err != nil {
		return 0
	}
	request.Header.Set("User-Agent", "BYOG-Worker/1.0")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Range", "bytes=0-0")
	response, err = client.Do(request)
	if err != nil {
		return 0
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusPartialContent {
		contentRange := response.Header.Get("Content-Range")
		if slash := strings.LastIndex(contentRange, "/"); slash >= 0 && slash+1 < len(contentRange) {
			if total, parseErr := strconv.ParseInt(strings.TrimSpace(contentRange[slash+1:]), 10, 64); parseErr == nil && total > 0 {
				return total
			}
		}
	}
	if response.StatusCode == http.StatusOK {
		return response.ContentLength
	}
	return 0
}

// fetchChunkRange transfers one channel of the parallel download and appends
// the bytes to the part file of the channel. A part that already holds bytes
// resumes where it stopped.
func fetchChunkRange(ctx context.Context, remoteUrl string, partPath string, start int64, end int64, counter *int64, mutex *sync.Mutex) error {
	have := int64(0)
	if info, err := os.Stat(partPath); err == nil && info.Size() > 0 {
		have = info.Size()
	}
	expected := end - start + 1
	if have >= expected {
		mutex.Lock()
		*counter += have
		mutex.Unlock()
		return nil
	}
	if have > 0 {
		mutex.Lock()
		*counter += have
		mutex.Unlock()
	}

	request, err := http.NewRequest(http.MethodGet, remoteUrl, nil)
	if err != nil {
		return err
	}
	request = request.WithContext(ctx)
	request.Header.Set("User-Agent", "BYOG-Worker/1.0")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start+have, end))

	// The transfer of a model takes minutes, the timeout covers the whole
	// download and not a single read.
	client := &http.Client{Timeout: 6 * time.Hour}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	switch response.StatusCode {
	case http.StatusPartialContent:
	case http.StatusOK:
		// The host sends the whole file for every channel, the parallel
		// download is not possible and the single connection fallback is used.
		return errRangeUnsupported
	case http.StatusRequestedRangeNotSatisfiable:
		// The part holds more bytes than the file has, the size of the
		// assignment does not match the file of the host.
		return fmt.Errorf("the range %d-%d is not satisfiable, the size of the model is out of date", start+have, end)
	default:
		return fmt.Errorf("the server answered with %d", response.StatusCode)
	}

	out, err := os.OpenFile(partPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	written, err := io.Copy(&progressWriter{writer: out, counter: counter, mutex: mutex}, response.Body)
	if err != nil {
		return err
	}
	if have+written != expected {
		return fmt.Errorf("the piece of the file is incomplete: %d of %d bytes", have+written, expected)
	}
	return nil
}

// IDM-Style 16-Channel Parallel Range Downloader (Section 2.2)
//
// The download reports the transferred amount so that the panel can show the real
// progress of the model that is currently fetched. A channel resumes the bytes
// it holds and retries a stalled mirror, only the missing bytes are fetched
// again.
func downloadModelIDM(remoteUrl string, targetPath string, fileSize int64, modelId string) error {
	if fileSize <= 0 {
		return fmt.Errorf("the size of the model is not known")
	}
	connections := ParallelConnections
	// A small file does not profit from sixteen connections, four channels
	// keep the overhead of the requests low.
	if fileSize < int64(connections)*8*1024*1024 {
		connections = 4
	}
	partSize := fileSize / int64(connections)
	if partSize <= 0 {
		return fmt.Errorf("the model is too small for a parallel download")
	}

	var wg sync.WaitGroup
	errs := make([]error, connections)

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

	// The channels share one context: a host that does not support the range
	// requests stops every channel at once, the fallback of the single
	// connection does not wait for fifteen more full answers.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("[BYOG IDM] Starting %d-channel saturated download for %s (%d bytes)\n", connections, targetPath, fileSize)

	for i := 0; i < connections; i++ {
		wg.Add(1)
		go func(channelIdx int) {
			defer wg.Done()
			startByte := int64(channelIdx) * partSize
			endByte := startByte + partSize - 1
			if channelIdx == connections-1 {
				endByte = fileSize - 1
			}
			partFile := fmt.Sprintf("%s.part%d", targetPath, channelIdx)

			var lastErr error
			for attempt := 0; attempt < chunkAttempts; attempt++ {
				if ctx.Err() != nil {
					lastErr = ctx.Err()
					break
				}
				lastErr = fetchChunkRange(ctx, remoteUrl, partFile, startByte, endByte, &downloaded, &downloadMutex)
				if lastErr == nil {
					return
				}
				if errors.Is(lastErr, errRangeUnsupported) || errors.Is(lastErr, context.Canceled) {
					break
				}
				fmt.Printf("[BYOG IDM] Channel %d attempt %d failed: %v\n", channelIdx, attempt+1, lastErr)
				time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
			}
			if errors.Is(lastErr, errRangeUnsupported) {
				cancel()
			}
			errs[channelIdx] = lastErr
		}(i)
	}

	wg.Wait()

	for _, err := range errs {
		if err != nil {
			if errors.Is(err, errRangeUnsupported) {
				return errRangeUnsupported
			}
			return fmt.Errorf("chunk download failed: %v", err)
		}
	}

	// Merge all parts into final model file, the parts are streamed through a
	// small buffer: a piece of a model of many gigabytes is never held in the
	// memory of the client.
	finalFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}

	mergeFailed := false
	for i := 0; i < connections && !mergeFailed; i++ {
		partFile := fmt.Sprintf("%s.part%d", targetPath, i)
		partHandle, openErr := os.Open(partFile)
		if openErr != nil {
			err = fmt.Errorf("the part %d of the model could not be read: %w", i, openErr)
			mergeFailed = true
			break
		}
		_, copyErr := io.Copy(finalFile, partHandle)
		partHandle.Close()
		if copyErr != nil {
			err = fmt.Errorf("the part %d of the model could not be merged: %w", i, copyErr)
			mergeFailed = true
		}
	}
	closeErr := finalFile.Close()
	if mergeFailed {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	for i := 0; i < connections; i++ {
		_ = os.Remove(fmt.Sprintf("%s.part%d", targetPath, i))
	}

	// A model that is not complete can not be used for an inference.
	info, err := os.Stat(targetPath)
	if err != nil {
		return err
	}
	if info.Size() != fileSize {
		return fmt.Errorf("the model file is incomplete: %d of %d bytes", info.Size(), fileSize)
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
	if foundPath, ok := findInstalledModelPath(modelId); ok {
		targetPath = foundPath
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
	} else {
		targetDir := bestDownloadDir(job.FileSize)
		_ = os.MkdirAll(targetDir, 0755)
		setHiddenFolder(targetDir)
		targetPath = filepath.Join(targetDir, modelId+".gguf")
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
		if job.FileSize > 0 {
			status.TotalMB = int(job.FileSize / (1024 * 1024))
		}
	})

	primary := ""
	if server := strings.TrimRight(State.Config.ServerUrl, "/"); server != "" {
		primary = server + "/api/v1/worker/models/" + url.PathEscape(modelId) + "/file"
	}
	if primary == "" {
		primary = job.DownloadUrl
	}

	// The real size of the file is probed at the host. The size of the
	// assignment of the coordinator can be out of date, a parallel download
	// that is cut from a wrong size produces a model that is missing its tail
	// and the inference of that model crashes the engine.
	probeUrl := primary
	if probeUrl == "" {
		probeUrl = job.DownloadUrl
	}
	if probeUrl != "" {
		if realSize := resolveRemoteFileSize(probeUrl); realSize > 0 && realSize != job.FileSize {
			if job.FileSize > 0 {
				fmt.Printf("[BYOG IDM] The size of the assignment (%d) does not match the size of the host (%d), the size of the host is used\n", job.FileSize, realSize)
			}
			job.FileSize = realSize
			totalMB := int(realSize / (1024 * 1024))
			State.Mu.Lock()
			State.TotalMB = totalMB
			State.Mu.Unlock()
			updateStatus(func(status *AppStatus) { status.TotalMB = totalMB })
		} else if realSize > 0 {
			job.FileSize = realSize
		}
	}

	failed := primary == ""
	var lastErr error
	if primary != "" {
		if err := downloadModelIDM(primary, targetPath, job.FileSize, modelId); err != nil {
			fmt.Printf("[BYOG IDM] Download of %s failed: %v\n", modelId, err)
			cleanupModelParts(targetPath)
			State.Mu.Lock()
			State.DownloadStartedAt = time.Now().UnixMilli()
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
// requests, the file is downloaded as a whole then. A transfer that is
// interrupted resumes the bytes of the part file when the host supports the
// range requests, only the missing tail is fetched again.
func downloadModelSingle(downloadUrl string, targetPath string, fileSize int64, modelId string) error {
	partFile := targetPath + ".part"
	var lastErr error

	for attempt := 0; attempt < chunkAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}
		lastErr = downloadModelSingleAttempt(downloadUrl, partFile, targetPath, fileSize, modelId)
		if lastErr == nil {
			return nil
		}
		fmt.Printf("[BYOG IDM] Single connection attempt %d failed: %v\n", attempt+1, lastErr)
	}
	return lastErr
}

// downloadModelSingleAttempt runs one transfer of the single connection
// fallback and verifies the size of the file before it is renamed.
func downloadModelSingleAttempt(downloadUrl string, partFile string, targetPath string, fileSize int64, modelId string) error {
	have := int64(0)
	if info, err := os.Stat(partFile); err == nil && info.Size() > 0 {
		have = info.Size()
	}

	request, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "BYOG-Worker/1.0")
	request.Header.Set("Accept-Encoding", "identity")
	if have > 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	client := &http.Client{Timeout: 6 * time.Hour}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	appendPart := false
	total := fileSize
	switch response.StatusCode {
	case http.StatusPartialContent:
		appendPart = true
		total = have
		if response.ContentLength > 0 {
			total += response.ContentLength
		}
		if contentRange := response.Header.Get("Content-Range"); contentRange != "" {
			if slash := strings.LastIndex(contentRange, "/"); slash >= 0 && slash+1 < len(contentRange) {
				if parsed, parseErr := strconv.ParseInt(strings.TrimSpace(contentRange[slash+1:]), 10, 64); parseErr == nil && parsed > 0 {
					total = parsed
				}
			}
		}
	case http.StatusOK:
		// The host does not resume, the transfer starts from the first byte.
		have = 0
		total = fileSize
		if response.ContentLength > 0 {
			total = response.ContentLength
		}
	case http.StatusRequestedRangeNotSatisfiable:
		// The part already holds the whole file or more, it is verified below.
		if info, statErr := os.Stat(partFile); statErr == nil && fileSize > 0 && info.Size() == fileSize {
			return os.Rename(partFile, targetPath)
		}
		_ = os.Remove(partFile)
		return fmt.Errorf("the part file does not match the size of the model")
	default:
		return fmt.Errorf("the server answered with %d", response.StatusCode)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if appendPart {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	out, err := os.OpenFile(partFile, flags, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	var downloaded int64
	var mutex sync.Mutex
	if appendPart {
		mutex.Lock()
		downloaded = have
		mutex.Unlock()
	}
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
	if total > 0 && info.Size() != total {
		return fmt.Errorf("the model file is incomplete: %d of %d bytes", info.Size(), total)
	}
	meter.report(info.Size())
	if err := out.Close(); err != nil {
		return err
	}
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
	if workerurl.Validate(State.Config.ServerUrl) != nil {
		return
	}
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
	if workerurl.Validate(State.Config.ServerUrl) != nil {
		return
	}
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
	req, err := http.NewRequest(http.MethodPost, crashUrl, bytes.NewReader(body))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+State.Config.WorkerSecret)
		if resp, err := (&http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req); err == nil {
			resp.Body.Close()
		}
	}
}

// LocalTransformerEngine executes the inference with the local llama.cpp runtime.
type LocalTransformerEngine struct {
	ModelId  string
	GgufPath string
}

func NewLocalTransformerEngine(modelId string, storageDir string) *LocalTransformerEngine {
	ggufPath := filepath.Join(storageDir, modelId+".gguf")
	if foundPath, ok := findInstalledModelPath(modelId); ok {
		ggufPath = foundPath
	}
	return &LocalTransformerEngine{
		ModelId:  modelId,
		GgufPath: ggufPath,
	}
}

// GenerateStreamMessages runs llama.cpp generation with full conversation history and streams tokens.
func (e *LocalTransformerEngine) GenerateStreamMessages(ctx context.Context, messages []llama.LlamaChatMessage, maxTokens int, temperature float64, onChunk func(piece string, tokenId int) bool) (*llama.Result, error) {
	if maxTokens <= 0 {
		maxTokens = -1
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

	return runtime.InferMessages(ctx, e.GgufPath, filepath.Dir(e.GgufPath), messages, maxTokens, temperature, onChunk)
}

// GenerateStream runs a real llama.cpp generation and streams the produced tokens.
// The measured token count and throughput come from the engine itself, nothing is
// estimated or fabricated.
func (e *LocalTransformerEngine) GenerateStream(ctx context.Context, prompt string, maxTokens int, temperature float64, onChunk func(piece string, tokenId int) bool) (*llama.Result, error) {
	return e.GenerateStreamMessages(ctx, []llama.LlamaChatMessage{{Role: "user", Content: prompt}}, maxTokens, temperature, onChunk)
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
			_ = sendWsFrame(conn, map[string]string{"type": "pong"})

		case "device_removed", "pairing_revoked":
			// The device was removed from the network, the pairing is dropped.
			State.Mu.Lock()
			State.IsConnected = false
			State.Config.NodeId = ""
			State.Config.PairingCode = ""
			cfg := State.Config
			State.Mu.Unlock()
			saveConfig(cfg)
			updateStatus(func(status *AppStatus) {
				status.Connected = false
				status.Paired = false
				status.NodeId = ""
			})
			fmt.Println("[BYOG WORKER] The device was removed from the network, the pairing has to be done again")
			_ = sendWsFrame(conn, map[string]string{"type": "device_removed_ack"})
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
				HotStartEnabled *bool    `json:"hot_start_enabled"`
				AllocatedRamGB  *float64 `json:"allocated_ram_gb"`
				AllocatedVramGB *int     `json:"allocated_vram_gb"`
				AllocatedCores  *int     `json:"allocated_cores"`
				AllocatedDiskGB *int     `json:"allocated_disk_gb"`
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

		case "update_resources":
			var resUpdate struct {
				AllocatedRamGB  *float64        `json:"allocated_ram_gb"`
				AllocatedVramGB *int            `json:"allocated_vram_gb"`
				AllocatedCores  *int            `json:"allocated_cores"`
				AllocatedDiskGB *int            `json:"allocated_disk_gb"`
				HotStartEnabled *bool           `json:"hot_start_enabled"`
				Disks           json.RawMessage `json:"disks"`
			}
			if err := json.Unmarshal(frame.Payload, &resUpdate); err == nil {
				changed := false
				State.Mu.Lock()
				if resUpdate.AllocatedRamGB != nil && State.AllocatedRamGB != *resUpdate.AllocatedRamGB {
					State.AllocatedRamGB = *resUpdate.AllocatedRamGB
					changed = true
				}
				if resUpdate.AllocatedVramGB != nil && State.AllocatedVramGB != *resUpdate.AllocatedVramGB {
					State.AllocatedVramGB = *resUpdate.AllocatedVramGB
					changed = true
				}
				if resUpdate.AllocatedCores != nil && State.AllocatedCores != *resUpdate.AllocatedCores {
					State.AllocatedCores = *resUpdate.AllocatedCores
					changed = true
				}
				if resUpdate.AllocatedDiskGB != nil && State.Config.MaxDiskGB != *resUpdate.AllocatedDiskGB {
					State.Config.MaxDiskGB = *resUpdate.AllocatedDiskGB
					changed = true
				}
				if resUpdate.HotStartEnabled != nil && State.HotStartEnabled != *resUpdate.HotStartEnabled {
					State.HotStartEnabled = *resUpdate.HotStartEnabled
					changed = true
				}
				if len(resUpdate.Disks) > 0 {
					disksStr := string(resUpdate.Disks)
					if strings.HasPrefix(disksStr, `"`) && strings.HasSuffix(disksStr, `"`) {
						var unquoted string
						if err := json.Unmarshal(resUpdate.Disks, &unquoted); err == nil {
							disksStr = unquoted
						}
					}
					if State.Disks != disksStr {
						State.Disks = disksStr
						changed = true
					}
					var disks []DiskPartitionInfo
					if err := json.Unmarshal([]byte(disksStr), &disks); err == nil {
						for _, d := range disks {
							if d.AllocatedGB <= 0 {
								pruneDriveModels(d.Drive)
							}
						}
						pruneAllModelsIfZeroDisk()
					}
				}
				State.Mu.Unlock()
				if changed {
					saveConfig(State.Config)
					updateStatus(func(status *AppStatus) {
						status.HotStartEnabled = State.HotStartEnabled
					})
					_ = candidateModelDirs()
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
						_ = sendWsFrame(conn, map[string]interface{}{
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
						_ = sendWsFrame(conn, map[string]interface{}{
							"type":    "byte_probe_response",
							"id":      frame.Id,
							"payload": json.RawMessage(respPayload),
						})
						return
					}
					respPayload, _ := json.Marshal(map[string]string{
						"data_hex": hex.EncodeToString(buf),
					})
					_ = sendWsFrame(conn, map[string]interface{}{
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
					_ = sendWsFrame(conn, map[string]interface{}{
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
						State.Mu.Lock()
						if !State.HotStartEnabled {
							State.ActiveVramModel = ""
						}
						targetStatus := "ready"
						if State.DownloadModelId != "" {
							targetStatus = "downloading"
						}
						State.ModelStatus = targetStatus
						State.CurrentTps = 0.0
						State.Mu.Unlock()
						updateStatus(func(status *AppStatus) {
							status.ModelStatus = targetStatus
							status.CurrentTps = 0.0
						})
						triggerHeartbeat()
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

					var chatMessages []llama.LlamaChatMessage
					if len(infReq.Messages) > 0 {
						var rawMsgs []map[string]interface{}
						if err := json.Unmarshal(infReq.Messages, &rawMsgs); err == nil {
							for _, m := range rawMsgs {
								role, _ := m["role"].(string)
								if role == "" {
									role = "user"
								}
								var contentStr string
								if c, ok := m["content"].(string); ok {
									contentStr = c
								} else if m["content"] != nil {
									cb, _ := json.Marshal(m["content"])
									contentStr = string(cb)
								}
								chatMessages = append(chatMessages, llama.LlamaChatMessage{
									Role:    role,
									Content: contentStr,
								})
							}
						}
					}
					if len(chatMessages) == 0 && strings.TrimSpace(infReq.Prompt) != "" {
						chatMessages = []llama.LlamaChatMessage{{
							Role:    "user",
							Content: strings.TrimSpace(infReq.Prompt),
						}}
					}

					fmt.Printf("[INFERENCE] Request %s: model=%s, messages=%d turns, maxTokens=%d\n", infReq.RequestId, infReq.ModelId, len(chatMessages), infReq.MaxTokens)
					engine := NewLocalTransformerEngine(infReq.ModelId, State.Config.StorageDir)
					result, err := engine.GenerateStreamMessages(
						context.Background(),
						chatMessages,
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
	patchIconsFlag := flag.Bool("patch-icons", false, "Patch the installed executables with site branding icons")
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
	if *patchIconsFlag {
		_ = installer.PatchInstalledBinariesIcons()
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
	go func() {
		_ = installer.PatchInstalledBinariesIcons()
	}()

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
	exe, err := os.Executable()
	if err == nil {
		dir := strings.ToLower(filepath.Dir(exe))
		if strings.Contains(dir, "programs") || strings.Contains(dir, "binaries") {
			return false
		}
		if _, err := os.Stat(filepath.Join(filepath.Dir(exe), "unins000.exe")); err == nil {
			return false
		}
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
		if serverFlag != config.ServerUrl {
			config.NodeId = ""
			config.WorkerSecret = ""
			config.PairingCode = ""
		}
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
