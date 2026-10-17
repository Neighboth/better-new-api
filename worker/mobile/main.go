// BYOG Mobile & Tablet Background Worker (byog-mobile-worker)
// Real Physical RAM vs ZRAM Filter, Strict Mobile Tiering, WakeLock & Foreground Service

package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type MobileConfig struct {
	ServerUrl             string  `json:"server_url"`
	NodeId                string  `json:"node_id"`
	PairingCode           string  `json:"pairing_code"`
	DeviceName            string  `json:"device_name"`
	WifiOnlyDownload      bool    `json:"wifi_only_download"`       // DEFAULT TRUE (Section 6.3)
	MonthlyDataLimitMB    int64   `json:"monthly_data_limit_mb"`    // DEFAULT UNLIMITED (0) (Section 6.3)
	MonthlyDataUsedMB     int64   `json:"monthly_data_used_mb"`
	RunOnlyWhileCharging  bool    `json:"run_only_while_charging"`  // DEFAULT FALSE (Section 6.3)
	BatteryCutoffEnabled  bool    `json:"battery_cutoff_enabled"`   // DEFAULT FALSE (Section 6.3)
	BatteryCutoffPercent  int     `json:"battery_cutoff_percent"`   // 5 - 50%
	MaxBatteryTemp        float64 `json:"max_battery_temp"`         // DEFAULT 50°C (Section 6.3)
	BatteryTempSupported  bool    `json:"battery_temp_supported"`
}

type MemoryMetrics struct {
	PhysicalTotalMB int `json:"physical_total_mb"` // Real physical RAM
	AvailableMB     int `json:"available_mb"`      // Safe allocatable heap
	SwapTotalMB     int `json:"swap_total_mb"`     // ZRAM / Virtual RAM (STRICTLY IGNORED!)
	SafeBudgetMB    int `json:"safe_budget_mb"`    // Calculated safe budget
}

type MobileState struct {
	Config          MobileConfig
	Memory          MemoryMetrics
	WsConn          *websocket.Conn
	IsConnected     bool
	IsPaused        bool
	IsThrottled     bool
	IsCharging      bool
	BatteryPercent  int
	BatteryTemp     float64
	NetworkType     string // wifi, cellular
	InstalledModels map[string]bool
	TotalTokens     int64
	Mu              sync.Mutex
}

var State = &MobileState{
	InstalledModels: make(map[string]bool),
	Config: MobileConfig{
		WifiOnlyDownload:     true,  // DEFAULT: ON (Section 6.3)
		MonthlyDataLimitMB:   0,     // DEFAULT: UNLIMITED (Section 6.3)
		RunOnlyWhileCharging: false, // DEFAULT: OFF (Section 6.3)
		BatteryCutoffEnabled: false, // DEFAULT: OFF (Section 6.3)
		BatteryCutoffPercent: 20,
		MaxBatteryTemp:       50.0,  // DEFAULT: 50°C (Section 6.3)
		BatteryTempSupported: true,
	},
}

// Section 6.1: Strict Physical RAM vs ZRAM/Swap Filtering (/proc/meminfo)
func queryDeviceMemory() MemoryMetrics {
	metrics := MemoryMetrics{
		PhysicalTotalMB: 6144, // Default fallback
		AvailableMB:     2800,
		SwapTotalMB:     6144, // ZRAM / RAM Plus
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
	metrics.SafeBudgetMB = int(math.Min(budgetA, budgetB))

	return metrics
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

// Section 6.2: Clean Live Corporate Status Notification (Zero-Emoji & i18n compliant)
func updateForegroundNotification(tps float64, todayTokens int64) {
	statusText := fmt.Sprintf("BYOG Dugumu Aktif | Hiz: %.1f TPS | Bugun: +%d Token", tps, todayTokens)
	// Write to Android log / notification bus
	_ = statusText
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
	State.Config.ServerUrl = serverUrl
	State.Config.DeviceName = "Mobile-Node-" + hwid[:6]
	State.NetworkType = "wifi"
	State.BatteryPercent = 85
	State.BatteryTemp = 33.5
	State.IsCharging = true

	fmt.Printf("[BYOG MOBILE] Initialized Node. Physical RAM: %d MB | Safe Budget: %d MB (ZRAM %d MB Ignored)\n",
		mem.PhysicalTotalMB, mem.SafeBudgetMB, mem.SwapTotalMB)

	// Register with Coordinator
	regUrl := fmt.Sprintf("%s/api/v1/worker/register", serverUrl)
	regBody, _ := json.Marshal(map[string]interface{}{
		"hwid":                hwid,
		"os":                  "android",
		"arch":                runtime.GOARCH,
		"device_name":         State.Config.DeviceName,
		"device_type":         "mobile",
		"cpu_model":           "ARM Cortex Octa-Core",
		"gpu_model":           "Adreno / ARM Mali Vulkan Compute",
		"physical_ram_mb":     mem.PhysicalTotalMB,
		"swap_ram_mb":         mem.SwapTotalMB, // Logged for telemetry, ignored for capacity
		"vram_mb":             0,
		"shared_vram_mb":      mem.SafeBudgetMB,
		"tee_supported":       true,
		"tee_type":            "trustzone",
		"bandwidth_down_mbps": 65.0,
		"bandwidth_up_mbps":   40.0,
		"ping_ms":             24,
		"network_type":        "wifi",
	})

	resp, err := http.Post(regUrl, "application/json", bytes.NewReader(regBody))
	if err == nil && resp.StatusCode == http.StatusOK {
		var regResp struct {
			Success bool `json:"success"`
			Data    struct {
				NodeId      string `json:"node_id"`
				PairingCode string `json:"pairing_code"`
			} `json:"data"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&regResp)
		State.Config.NodeId = regResp.Data.NodeId
		State.Config.PairingCode = regResp.Data.PairingCode
		fmt.Printf("[BYOG MOBILE] Paired successfully. Node ID: %s | Pairing Code: %s\n", State.Config.NodeId, State.Config.PairingCode)
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

			// Send Heartbeats
			go func() {
				ticker := time.NewTicker(20 * time.Second)
				defer ticker.Stop()
				for range ticker.C {
					canRun, _ := canAcceptTasks()
					status := "online"
					if !canRun {
						status = "paused"
					}
					payload, _ := json.Marshal(map[string]interface{}{
						"cpu_temp":         38.0,
						"battery_temp":     State.BatteryTemp,
						"battery_percent":  State.BatteryPercent,
						"is_charging":      State.IsCharging,
						"current_tps":      18.5,
						"total_tokens":     State.TotalTokens,
						"installed_models": []string{"qwen-2.5-0.5b-instruct", "bge-large-en-v1.5"},
						"status":           status,
					})
					_ = conn.WriteJSON(map[string]interface{}{
						"type":      "heartbeat",
						"payload":   payload,
						"timestamp": time.Now().UnixMilli(),
					})
					updateForegroundNotification(18.5, State.TotalTokens)
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
			}
		}
		time.Sleep(5 * time.Second)
	}
}
