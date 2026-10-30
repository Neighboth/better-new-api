package main

// The hardware of the computer is measured on the computer and not in the
// coordinator. The panel shows the values that this file reads, therefore a
// value that cannot be measured stays unknown instead of being invented.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"
)

// hardwareProfile is the hardware of the computer that the coordinator needs.
type hardwareProfile struct {
	CpuModel      string              `json:"cpu_model"`
	GpuModel      string              `json:"gpu_model"`
	CpuCores      int                 `json:"cpu_cores"`
	PhysicalRamMB int                 `json:"physical_ram_mb"`
	SwapRamMB     int                 `json:"swap_ram_mb"`
	VramMB        int                 `json:"vram_mb"`
	SharedVramMB  int                 `json:"shared_vram_mb"`
	Disks         []DiskPartitionInfo `json:"disks"`
	TotalDiskGB   int                 `json:"total_disk_gb"`
	FreeDiskGB    int                 `json:"free_disk_gb"`
	MeasuredAt    time.Time           `json:"-"`
}

var (
	hardwareMutex sync.Mutex
	hardwareCache = hardwareProfile{}
)

// readHardware reads the hardware of the computer.
//
// The memory of the disks changes while the client runs, the values are read
// again after half a minute. The rest of the hardware does not change, the
// values are kept for the whole run of the client.
func readHardware() hardwareProfile {
	hardwareMutex.Lock()
	defer hardwareMutex.Unlock()

	refresh := hardwareCache.MeasuredAt.IsZero() || time.Since(hardwareCache.MeasuredAt) > 30*time.Second
	if !refresh {
		return hardwareCache
	}

	if hardwareCache.CpuModel == "" {
		hardwareCache.CpuModel = cpuModelName()
		if hardwareCache.CpuModel == "" {
			hardwareCache.CpuModel = runtime.GOARCH + " " + runtime.GOOS
		}
		hardwareCache.GpuModel = gpuModelName()
		hardwareCache.CpuCores = runtime.NumCPU()
		hardwareCache.PhysicalRamMB = physicalRamMB()
		hardwareCache.SwapRamMB = swapRamMB()
		hardwareCache.VramMB = dedicatedVRAMMB()
		// A graphics adapter without its own memory runs from the memory of the
		// system. The system gives half of the memory of the computer to the
		// adapter, the value is reported as the shared memory of the adapter.
		if hardwareCache.VramMB <= 0 {
			shared := hardwareCache.PhysicalRamMB / 2
			if shared > 8192 {
				shared = 8192
			}
			if shared < 0 {
				shared = 0
			}
			hardwareCache.SharedVramMB = shared
			hardwareCache.VramMB = 0
		} else {
			hardwareCache.SharedVramMB = 0
		}
	}

	disks := getFixedDiskPartitions()
	total := 0
	free := 0
	for index := range disks {
		total += disks[index].TotalGB
		free += disks[index].FreeGB
	}
	hardwareCache.Disks = disks
	hardwareCache.TotalDiskGB = total
	hardwareCache.FreeDiskGB = free
	hardwareCache.MeasuredAt = time.Now()
	return hardwareCache
}

// disksJSON returns the disks of the computer as the JSON array the coordinator
// stores in the record of the node.
func (p hardwareProfile) disksJSON() string {
	if p.Disks == nil {
		return "[]"
	}
	encoded, err := json.Marshal(p.Disks)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// bandwidthSample is the result of the measurement of the network of the
// computer.
//
// The values are measured against the coordinator of the client, the distance
// to the coordinator decides the latency of the requests of the inference.
type bandwidthSample struct {
	DownMbps   float64   `json:"bandwidth_down_mbps"`
	UpMbps     float64   `json:"bandwidth_up_mbps"`
	PingMs     int       `json:"ping_ms"`
	MeasuredAt time.Time `json:"-"`
}

var (
	bandwidthMutex  sync.Mutex
	bandwidthResult bandwidthSample
	bandwidthErr    string
)

// readBandwidth returns the last measurement of the network.
func readBandwidth() (bandwidthSample, string) {
	bandwidthMutex.Lock()
	defer bandwidthMutex.Unlock()
	return bandwidthResult, bandwidthErr
}

// measureBandwidth measures the download, the upload and the latency of the
// network of the computer against the coordinator.
//
// The measurement is small: two megabytes are downloaded and one megabyte is
// uploaded. The measurement is repeated while the client runs, a network that
// changes during the day is reported with the value of the last measurement.
func measureBandwidth() {
	server := strings.TrimRight(State.Config.ServerUrl, "/")
	if server == "" {
		return
	}

	sample := bandwidthSample{MeasuredAt: time.Now()}
	failure := ""

	// The latency of three small requests, the smallest value is reported: the
	// other values carry the load of the network stack of the system.
	for attempt := 0; attempt < 3; attempt++ {
		started := time.Now()
		response, err := http.Get(server + "/api/v1/worker/ping")
		if err != nil {
			failure = err.Error()
			break
		}
		_, _ = io.Copy(io.Discard, response.Body)
		response.Body.Close()
		elapsed := time.Since(started).Milliseconds()
		if elapsed <= 0 {
			elapsed = 1
		}
		ping := int(elapsed)
		if sample.PingMs == 0 || ping < sample.PingMs {
			sample.PingMs = ping
		}
	}

	if failure == "" {
		const probeBytes = 2 * 1024 * 1024
		started := time.Now()
		response, err := http.Get(fmt.Sprintf("%s/api/v1/worker/bandwidth?bytes=%d", server, probeBytes))
		if err != nil {
			failure = err.Error()
		} else {
			read, _ := io.Copy(io.Discard, response.Body)
			response.Body.Close()
			elapsed := time.Since(started).Seconds()
			if read > 64*1024 && elapsed > 0.05 {
				sample.DownMbps = roundMbps(float64(read) * 8 / elapsed / 1_000_000)
			} else {
				failure = "the download measurement was too short"
			}
		}
	}

	if failure == "" {
		const uploadBytes = 512 * 1024
		payload := bytes.Repeat([]byte{0x5A}, uploadBytes)
		started := time.Now()
		response, err := http.Post(
			fmt.Sprintf("%s/api/v1/worker/bandwidth", server),
			"application/octet-stream",
			bytes.NewReader(payload),
		)
		if err != nil {
			failure = err.Error()
		} else {
			_, _ = io.Copy(io.Discard, response.Body)
			response.Body.Close()
			elapsed := time.Since(started).Seconds()
			if elapsed > 0.05 {
				sample.UpMbps = roundMbps(float64(uploadBytes) * 8 / elapsed / 1_000_000)
			} else {
				failure = "the upload measurement was too short"
			}
		}
	}

	bandwidthMutex.Lock()
	defer bandwidthMutex.Unlock()
	if failure != "" {
		bandwidthErr = failure
	}
	if sample.PingMs > 0 {
		bandwidthResult.PingMs = sample.PingMs
	}
	if sample.DownMbps > 0 {
		bandwidthResult.DownMbps = sample.DownMbps
	}
	if sample.UpMbps > 0 {
		bandwidthResult.UpMbps = sample.UpMbps
	}
	bandwidthResult.MeasuredAt = time.Now()
}

// roundMbps rounds a measured rate to one decimal.
func roundMbps(value float64) float64 {
	if value <= 0 {
		return 0
	}
	return float64(int(value*10+0.5)) / 10
}

// startBandwidthLoop measures the network of the computer while the client
// runs.
func startBandwidthLoop() {
	recoverForever("bandwidth", func() {
		time.Sleep(5 * time.Second)
		for {
			measureBandwidth()
			time.Sleep(30 * time.Minute)
		}
	})
}
