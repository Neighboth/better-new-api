//go:build !windows

package main

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// hideProcessWindow is a no-op outside Windows.
func hideProcessWindow(command *exec.Cmd) {
	_ = command
}

func detachConsole() {}

func redirectOutputToLog() {}

func checkDebuggerPresent() bool {
	return false
}

func isFullscreenGameRunning() bool {
	return false
}

// systemLanguage returns the language of the system of the user.
//
// The language decides the texts of the page of the first start and the texts
// of the interface of the client.
func systemLanguage() string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		value := strings.TrimSpace(os.Getenv(name))
		if value == "" {
			continue
		}
		value = strings.ToLower(strings.Split(value, ".")[0])
		switch {
		case strings.HasPrefix(value, "tr"):
			return "tr"
		case strings.HasPrefix(value, "zh_tw"), strings.HasPrefix(value, "zh_hk"), strings.HasPrefix(value, "zh_hant"):
			return "zh_TW"
		case strings.HasPrefix(value, "zh"):
			return "zh_CN"
		case strings.HasPrefix(value, "fr"):
			return "fr"
		case strings.HasPrefix(value, "ru"):
			return "ru"
		case strings.HasPrefix(value, "ja"):
			return "ja"
		case strings.HasPrefix(value, "vi"):
			return "vi"
		case strings.HasPrefix(value, "en"):
			return "en"
		}
	}
	return "en"
}

// physicalRamMB returns the physical memory of the computer in megabytes.
//
// The value decides which models the installation assigns to the node, a value
// that is too high would promise a capacity that the computer does not have.
func physicalRamMB() int {
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "MemTotal:") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				break
			}
			value, err := strconv.Atoi(fields[1])
			if err != nil {
				break
			}
			return value / 1024
		}
	}
	return 8192
}

// dedicatedVRAMMB returns the video memory of the dedicated adapters.
//
// The other systems do not report the memory of the graphics adapter without a
// driver of the vendor, the value is unknown there and it is not invented.
func dedicatedVRAMMB() int {
	return 0
}

// gpuModelName returns the name of the graphics adapter of the computer.
func gpuModelName() string {
	return ""
}

// cpuModelName returns the name of the processor of the computer.
func cpuModelName() string {
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "model name") {
				continue
			}
			if index := strings.Index(line, ":"); index >= 0 {
				return strings.TrimSpace(line[index+1:])
			}
		}
	}
	return ""
}

// swapRamMB returns the size of the swap of the system in megabytes.
func swapRamMB() int {
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "SwapTotal:") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				break
			}
			value, err := strconv.Atoi(fields[1])
			if err != nil {
				break
			}
			return value / 1024
		}
	}
	return 0
}

// acquireSingleInstance tells whether this process is the first client of the
// user.
func acquireSingleInstance() bool {
	return true
}

// showRunningInstance brings the window of the client that runs to the front.
func showRunningInstance() {}

type DiskPartitionInfo struct {
	Drive       string `json:"drive"`
	Label       string `json:"label"`
	TotalGB     int    `json:"total_gb"`
	FreeGB      int    `json:"free_gb"`
	AllocatedGB int    `json:"allocated_gb"`
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	cmd = exec.Command("xdg-open", url)
	_ = cmd.Start()
}

func getFixedDiskPartitions() []DiskPartitionInfo {
	return []DiskPartitionInfo{
		{
			Drive:   "/",
			Label:   "Root Partition",
			TotalGB: 100,
			FreeGB:  50,
		},
	}
}
