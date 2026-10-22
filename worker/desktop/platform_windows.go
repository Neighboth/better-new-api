//go:build windows

package main

import (
	"errors"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
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

// systemLanguage returns the language of the system of the user.
//
// The language decides the texts of the page of the first start and the texts
// of the interface of the client.
func systemLanguage() string {
	language, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage").Call()
	if language == 0 {
		return "en"
	}
	switch uint32(language) {
	case 0x041F: // Turkish
		return "tr"
	case 0x0804: // Chinese Simplified
		return "zh_CN"
	case 0x0404: // Chinese Traditional
		return "zh_TW"
	case 0x040C: // French
		return "fr"
	case 0x0419: // Russian
		return "ru"
	case 0x0411: // Japanese
		return "ja"
	case 0x042A: // Vietnamese
		return "vi"
	}
	return "en"
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

// singleInstanceMutex is the handle of the mutex that makes sure that one
// client runs for the user.
var singleInstanceMutex windows.Handle

// acquireSingleInstance tells whether this process is the first client of the
// user.
//
// The client stays in the notification area after its window is closed, the
// file of the client is started again to show the window. A second worker would
// serve the tasks of the computer twice.
func acquireSingleInstance() bool {
	name, err := windows.UTF16PtrFromString(`Local\ByogWorkerClient`)
	if err != nil {
		return true
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if err != nil {
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			_ = windows.CloseHandle(handle)
			return false
		}
		return true
	}
	singleInstanceMutex = handle
	return true
}

// showRunningInstance brings the window of the client that runs to the front.
func showRunningInstance() {
	user32 := syscall.NewLazyDLL("user32.dll")
	findWindow := user32.NewProc("FindWindowW")
	showWindow := user32.NewProc("ShowWindow")
	setForeground := user32.NewProc("SetForegroundWindow")

	// The window of the client is the interface of the installation.
	className, err := syscall.UTF16PtrFromString(webViewWindowClass)
	if err == nil {
		window, _, _ := findWindow.Call(uintptr(unsafe.Pointer(className)), 0)
		if window != 0 {
			showWindow.Call(window, 9) // SW_RESTORE
			setForeground.Call(window)
			return
		}
	}

	// The window of the program opens the page of the client again.
	className, err = syscall.UTF16PtrFromString("ByogWorkerWindow")
	if err != nil {
		return
	}
	window, _, _ := findWindow.Call(uintptr(unsafe.Pointer(className)), 0)
	if window == 0 {
		return
	}
	procWebViewPostMessage.Call(window, 0x0111, commandShow, 0) // WM_COMMAND
}
