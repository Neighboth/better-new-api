//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

func checkDebuggerPresent() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	isDebuggerPresent := kernel32.NewProc("IsDebuggerPresent")
	res, _, _ := isDebuggerPresent.Call()
	return res != 0
}

func isFullscreenGameRunning() bool {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	queryState := shell32.NewProc("SHQueryUserNotificationState")
	var state uint32
	ret, _, _ := queryState.Call(uintptr(unsafe.Pointer(&state)))
	if ret == 0 {
		// QUNS_BUSY = 2, QUNS_RUNNING_D3D_FULL_SCREEN = 3, QUNS_PRESENTATION_MODE = 4
		if state == 2 || state == 3 || state == 4 {
			return true
		}
	}
	return false
}
