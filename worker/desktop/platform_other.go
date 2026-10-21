//go:build !windows

package main

import (
	"os"
	"strconv"
	"strings"
)

func checkDebuggerPresent() bool {
	return false
}

func isFullscreenGameRunning() bool {
	return false
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
