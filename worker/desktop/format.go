package main

// The window and the page of the client show the time that remains of a
// download. The time is shown in hours, minutes and seconds, a time of an hour
// and a half is not shown as ninety minutes.

import (
	"fmt"
	"strings"
)

// formatRemainingTime formats a duration in seconds as `1h 2m 3s`.
//
// The parts that are zero are not shown, `1s` is a short wait and `1h 2m 3s` is
// a long one. The letters of the units are the same in every language of the
// client, the number carries the meaning.
func formatRemainingTime(seconds int) string {
	if seconds <= 0 {
		return ""
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	rest := seconds % 60

	parts := make([]string, 0, 3)
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || hours > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	parts = append(parts, fmt.Sprintf("%ds", rest))
	return strings.Join(parts, " ")
}

// formatTransferSpeed formats a transfer rate in bytes per second.
func formatTransferSpeed(bytesPerSecond float64) string {
	if bytesPerSecond <= 0 {
		return ""
	}
	units := []string{"B/s", "KB/s", "MB/s", "GB/s"}
	value := bytesPerSecond
	unit := 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f %s", value, units[unit])
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}
