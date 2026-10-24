//go:build windows

package llama

import (
	"os/exec"
	"syscall"
	"unsafe"
)

// hideProcessWindow starts the llama.cpp server without a console window.
//
// The worker is a graphical program, the engine of the model has to run in the
// background without a terminal of its own.
func hideProcessWindow(command *exec.Cmd) {
	if command == nil {
		return
	}
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

func systemRamMB() int {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	globalMemoryStatusEx := kernel32.NewProc("GlobalMemoryStatusEx")
	var status struct {
		Length               uint32
		MemoryLoad           uint32
		TotalPhys            uint64
		AvailPhys            uint64
		TotalPageFile        uint64
		AvailPageFile        uint64
		TotalVirtual         uint64
		AvailVirtual         uint64
		AvailExtendedVirtual uint64
	}
	status.Length = uint32(unsafe.Sizeof(status))
	ret, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if ret == 0 || status.TotalPhys == 0 {
		return 8192
	}
	return int(status.TotalPhys / (1024 * 1024))
}
