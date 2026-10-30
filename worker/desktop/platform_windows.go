//go:build windows

package main

import (
	"encoding/binary"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// hideProcessWindow starts a child process without a console window.
//
// The client is a program of the graphical interface, a console window of a
// child process would appear next to the window of the client and the user
// would see a terminal that does not belong to the application.
func hideProcessWindow(command *exec.Cmd) {
	if command == nil {
		return
	}
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}
}

// detachConsole disconnects the process from any attached console.
//
// The client is a graphical Windows application. If started from a terminal or
// if Windows attached a console window to the process, detaching frees the console
// so no terminal window remains visible.
func detachConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	freeConsole := kernel32.NewProc("FreeConsole")
	freeConsole.Call()
}

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

// displayAdapterClass is the class of the display adapters in the registry.
//
// Every adapter the driver knows has a key with its name and the size of its
// memory there. The number of the key is appended to the path.
const displayAdapterClass = `SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}\`

// displayAdapter is one graphics adapter of the computer.
type displayAdapter struct {
	Name   string
	VramMB int
}

// displayAdapters reads the graphics adapters of the computer.
//
// The size of the memory of an adapter is read from the key of the driver: the
// QWORD value carries the whole size, the DWORD value of the old drivers is
// limited to four gigabytes. An adapter without a driver is skipped, a basic
// display adapter is not a card that can run a model.
func displayAdapters() []displayAdapter {
	var adapters []displayAdapter
	for index := 0; index < 16; index++ {
		key, err := registry.OpenKey(registry.LOCAL_MACHINE, displayAdapterClass+fmt4(index), registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		name, _, _ := key.GetStringValue("DriverDesc")
		vram := 0
		if raw, _, err := key.GetBinaryValue("HardwareInformation.qwMemorySize"); err == nil && len(raw) >= 8 {
			vram = int(binary.LittleEndian.Uint64(raw) / (1024 * 1024))
		} else if value, _, err := key.GetIntegerValue("HardwareInformation.MemorySize"); err == nil {
			vram = int(value / (1024 * 1024))
		}
		key.Close()

		if strings.TrimSpace(name) == "" {
			continue
		}
		lower := strings.ToLower(name)
		if strings.Contains(lower, "microsoft basic display") || strings.Contains(lower, "microsoft remote display") ||
			strings.Contains(lower, "parsec") || strings.Contains(lower, "oray") {
			continue
		}
		adapters = append(adapters, displayAdapter{Name: name, VramMB: vram})
	}
	return adapters
}

// fmt4 formats the number of the key of a display adapter with four digits.
func fmt4(value int) string {
	text := strconv.Itoa(value)
	for len(text) < 4 {
		text = "0" + text
	}
	return text
}

// dedicatedVRAMMB returns the video memory of the dedicated adapters in
// megabytes.
//
// The value is zero when the computer has no dedicated graphics adapter. A
// shared memory of the system is not counted here, the panel shows the shared
// memory separately.
func dedicatedVRAMMB() int {
	total := 0
	for _, adapter := range displayAdapters() {
		total += adapter.VramMB
	}
	return total
}

// gpuModelName returns the name of the graphics adapter of the computer.
func gpuModelName() string {
	best := ""
	bestVram := -1
	for _, adapter := range displayAdapters() {
		if adapter.VramMB > bestVram {
			bestVram = adapter.VramMB
			best = adapter.Name
		}
	}
	return best
}

// cpuModelName returns the name of the processor of the computer.
func cpuModelName() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	name, _, err := key.GetStringValue("ProcessorNameString")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(name)
}

// swapRamMB returns the size of the page file of the system in megabytes.
func swapRamMB() int {
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
	if result == 0 || status.TotalPageFile < status.TotalPhys {
		return 0
	}
	return int((status.TotalPageFile - status.TotalPhys) / (1024 * 1024))
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

// releaseSingleInstance gives the mutex of the client back to the system.
//
// The self-updater starts the program of a new version before this program
// ends: the new program asks for the mutex of the client and it would end at
// once while this program still holds it.
func releaseSingleInstance() {
	if singleInstanceMutex == 0 {
		return
	}
	_ = windows.CloseHandle(singleInstanceMutex)
	singleInstanceMutex = 0
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

type DiskPartitionInfo struct {
	Drive       string `json:"drive"`
	Label       string `json:"label"`
	TotalGB     int    `json:"total_gb"`
	FreeGB      int    `json:"free_gb"`
	AllocatedGB int    `json:"allocated_gb"`
}

func openBrowser(url string) {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	shellExecute := shell32.NewProc("ShellExecuteW")
	verb, _ := syscall.UTF16PtrFromString("open")
	target, _ := syscall.UTF16PtrFromString(url)
	shellExecute.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
}

func getFixedDiskPartitions() []DiskPartitionInfo {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getLogicalDriveStrings := kernel32.NewProc("GetLogicalDriveStringsW")
	getDriveType := kernel32.NewProc("GetDriveTypeW")
	getDiskFreeSpaceEx := kernel32.NewProc("GetDiskFreeSpaceExW")

	buf := make([]uint16, 512)
	n, _, _ := getLogicalDriveStrings.Call(uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	if n == 0 {
		return nil
	}

	var partitions []DiskPartitionInfo
	var current []uint16
	for _, ch := range buf[:n] {
		if ch == 0 {
			if len(current) > 0 {
				driveStr := syscall.UTF16ToString(current)
				drivePtr := uintptr(unsafe.Pointer(&current[0]))
				driveType, _, _ := getDriveType.Call(drivePtr)
				// DRIVE_FIXED = 3 (strictly exclude removable USB drives DRIVE_REMOVABLE = 2, CDROM = 5, NETWORK = 4)
				if driveType == 3 {
					var freeBytesAvail, totalBytes, totalFreeBytes int64
					ret, _, _ := getDiskFreeSpaceEx.Call(
						drivePtr,
						uintptr(unsafe.Pointer(&freeBytesAvail)),
						uintptr(unsafe.Pointer(&totalBytes)),
						uintptr(unsafe.Pointer(&totalFreeBytes)),
					)
					if ret != 0 {
						totalGB := int(totalBytes / (1024 * 1024 * 1024))
						freeGB := int(freeBytesAvail / (1024 * 1024 * 1024))
						partitions = append(partitions, DiskPartitionInfo{
							Drive:   driveStr,
							Label:   "Yerel Disk (" + strings.TrimSuffix(driveStr, `\`) + ")",
							TotalGB: totalGB,
							FreeGB:  freeGB,
						})
					}
				}
				current = nil
			}
		} else {
			current = append(current, ch)
		}
	}
	return partitions
}
