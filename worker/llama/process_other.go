//go:build !windows

package llama

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

func systemRamMB() int {
	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					if val, err := strconv.Atoi(fields[1]); err == nil {
						return val / 1024
					}
				}
			}
		}
	}
	return 4096
}
