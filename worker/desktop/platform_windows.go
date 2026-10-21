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

// physicalRamMB returns the physical memory of the computer in megabytes.
//
// The value decides which models the installation assigns to the node, a value
// that is too high would promise a capacity that the computer does not have.
func physicalRamMB() int {
	type memoryStatusEx struct {
		Length            uint32
		MemoryLoad        uint32
		TotalPhys         uint64
		AvailablePhys     uint64
		TotalPageFile     uint64
		AvailablePageFile uint64
		TotalVirtual      uint64
		AvailableVirtual  uint64
		AvailableExtended uint64
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	globalMemoryStatusEx := kernel32.NewProc("GlobalMemoryStatusEx")
	status := memoryStatusEx{}
	status.Length = uint32(unsafe.Sizeof(status))
	result, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if result == 0 {
		return 8192
	}
	return int(status.TotalPhys / (1024 * 1024))
}
