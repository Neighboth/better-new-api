//go:build !windows

package main

func checkDebuggerPresent() bool {
	return false
}

func isFullscreenGameRunning() bool {
	return false
}
