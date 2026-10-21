//go:build windows

package main

// The graphical interface of the Windows client.
//
// The client is a window of its own, it is shown in the search of the system and
// it can be removed with the applications of the control panel. The worker runs
// in the background, the window only shows the state of the worker.
//
// Every measure of the layout comes from a design of 480 points, the window
// scales the design with the resolution of the screen. The window looks the same
// on every screen and it does not break when the resolution changes.
//
// The functions of the system are called directly, the client has no other
// dependency.

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/QuantumNous/new-api/worker/desktop/installer"
)

// The constants of the system.
const (
	wmDestroy    = 0x0002
	wmClose      = 0x0010
	wmCommand    = 0x0111
	wmTimer      = 0x0113
	wmMouseMove  = 0x0200
	wmLButtonUp  = 0x0202
	wmEraseBkgnd = 0x0014
	wmDpiChanged = 0x02E0
	wmPaint      = 0x000F
	wmTray       = 0x8000 + 1
	notifyClick  = 0x40000000

	csHRedraw = 0x0002
	csVRedraw = 0x0001

	wsOverlappedWindow = 0x00CF0000
	wsCaption          = 0x00C00000
	wsSysMenu          = 0x00080000
	wsMinimizeBox      = 0x00020000
	wsChild            = 0x40000000
	wsVisible          = 0x10000000
	wsBorder           = 0x00800000
	wsClipChildren     = 0x02000000

	esAutoHScroll   = 0x0080
	swShow          = 5
	swHide          = 0
	colorWindow     = 5
	imageIcon       = 1
	lrLoadFromFile  = 0x0010
	lrDefaultSize   = 0x0040
	cwUseDefault    = 0x80000000
	idcHand         = 32512
	resourceArrow   = 32512
	resourceAppIcon = 32512

	iconAdd         = 0x00000000
	iconDelete      = 0x00000002
	iconFlagMessage = 0x00000001
	iconFlagIcon    = 0x00000002
	iconFlagTip     = 0x00000004

	commandShow     = 100
	commandPanel    = 101
	commandQuit     = 102
	commandConnect  = 103
	commandTest     = 104
	commandLanguage = 105
	commandSettings = 106
	commandClose    = 107

	controlServerEdit = 201
	controlPairEdit   = 202
	trayIconID        = 1

	// The measures of the design of the window.
)

// The libraries of the system.
var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	procRegisterClassEx             = user32.NewProc("RegisterClassExW")
	procCreateWindowEx              = user32.NewProc("CreateWindowExW")
	procMessageBox                  = user32.NewProc("MessageBoxW")
	procDefWindowProc               = user32.NewProc("DefWindowProcW")
	procGetMessage                  = user32.NewProc("GetMessageW")
	procTranslateMessage            = user32.NewProc("TranslateMessage")
	procDispatchMessage             = user32.NewProc("DispatchMessageW")
	procPostQuitMessage             = user32.NewProc("PostQuitMessage")
	procSetWindowText               = user32.NewProc("SetWindowTextW")
	procGetWindowText               = user32.NewProc("GetWindowTextW")
	procGetWindowTextLen            = user32.NewProc("GetWindowTextLengthW")
	procLoadIcon                    = user32.NewProc("LoadIconW")
	procLoadCursor                  = user32.NewProc("LoadCursorW")
	procLoadImage                   = user32.NewProc("LoadImageW")
	procGetSysColorBrush            = user32.NewProc("GetSysColorBrush")
	procSetTimer                    = user32.NewProc("SetTimer")
	procShowWindow                  = user32.NewProc("ShowWindow")
	procUpdateWindow                = user32.NewProc("UpdateWindow")
	procInvalidateRect              = user32.NewProc("InvalidateRect")
	procCreatePopupMenu             = user32.NewProc("CreatePopupMenu")
	procAppendMenu                  = user32.NewProc("AppendMenuW")
	procTrackPopupMenu              = user32.NewProc("TrackPopupMenu")
	procDestroyMenu                 = user32.NewProc("DestroyMenu")
	procGetCursorPos                = user32.NewProc("GetCursorPos")
	procSetForeground               = user32.NewProc("SetForegroundWindow")
	procDestroyWindow               = user32.NewProc("DestroyWindow")
	procGetDC                       = user32.NewProc("GetDC")
	procReleaseDC                   = user32.NewProc("ReleaseDC")
	procBeginPaint                  = user32.NewProc("BeginPaint")
	procEndPaint                    = user32.NewProc("EndPaint")
	procDrawText                    = user32.NewProc("DrawTextW")
	procFillRect                    = user32.NewProc("FillRect")
	procSetWindowPos                = user32.NewProc("SetWindowPos")
	procSetCursor                   = user32.NewProc("SetCursor")
	procTrackMouseEvent             = user32.NewProc("TrackMouseEvent")
	procSetProcessDpiAwareContext   = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware          = user32.NewProc("SetProcessDPIAware")
	procGetDeviceCaps               = gdi32.NewProc("GetDeviceCaps")
	procAddFontResourceEx           = gdi32.NewProc("AddFontResourceExW")
	procGetDpiForWindow             = user32.NewProc("GetDpiForWindow")
	procSetProcessDpiAwareContextV2 = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDpiAwareContext1  = user32.NewProc("SetProcessDpiAwarenessContext")
	procGetDpiForSystem             = user32.NewProc("GetDpiForSystem")
	procSendMessage                 = user32.NewProc("SendMessageW")
	procGetCurrentProcess           = kernel32.NewProc("GetCurrentProcessId")
	procShellNotifyIcon             = shell32.NewProc("Shell_NotifyIconW")

	procCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = gdi32.NewProc("SelectObject")
	procDeleteObject           = gdi32.NewProc("DeleteObject")
	procDeleteDC               = gdi32.NewProc("DeleteDC")
	procBitBlt                 = gdi32.NewProc("BitBlt")
	procRoundRect              = gdi32.NewProc("RoundRect")
	procEllipse                = gdi32.NewProc("Ellipse")
	procSetBkMode              = gdi32.NewProc("SetBkMode")
	procSetTextColor           = gdi32.NewProc("SetTextColor")
	procCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	procCreatePen              = gdi32.NewProc("CreatePen")
	procCreateFont             = gdi32.NewProc("CreateFontW")
)

// The structures of the system.
type windowClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconName   *uint16
}

type windowMessage struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   struct{ X, Y int32 }
}

type windowPoint struct {
	X int32
	Y int32
}

type clientRect struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

type paintStruct struct {
	HDC       uintptr
	Erase     int32
	RectPaint clientRect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type trackMouseEvent struct {
	Size        uint32
	Flags       uint32
	TrackWindow uintptr
	HoverTime   uint32
}

type notifyIconData struct {
	Size             uint32
	Window           uintptr
	IconID           uint32
	Flags            uint32
	CallbackMessage  uint32
	Icon             uintptr
	Tip              [128]uint16
	State            uint32
	StateMask        uint32
	Info             [256]uint16
	TimeoutOrVersion uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
	Guid             [16]byte
	BalloonIcon      uintptr
}

// The states of the connection of the worker.
const (
	stateOffline = iota
	stateOnline
	stateDownloading
	stateError
)

// The button of the window.
type uiButton struct {
	command int
	left    int32
	top     int32
	right   int32
	bottom  int32
	label   string
	primary bool
}

// guiWindow holds the state of the window of the client.
type guiWindow struct {
	window    uintptr
	icon      uintptr
	instance  uintptr
	className *uint16
	dpi       int32

	serverEdit uintptr
	pairEdit   uintptr
	// serverFieldShown tells if the field of the address of the installation is
	// shown in the window.
	serverFieldShown bool
	buttons          []uiButton
	hovered          int

	fontTitle uintptr
	fontText  uintptr
	fontSmall uintptr

	statusText  string
	statusState int
	details     [4]string
	// page is the page of the menu that is shown.
	page int
	// testResult is the answer of the test of the model.
	testResult  string
	progress    int
	progressSet bool

	mutex         sync.Mutex
	trayAdded     bool
	lastConnected bool
}

var (
	guiWindowsMutex sync.Mutex
	guiWindows      = map[uintptr]*guiWindow{}
)

// runGui starts the graphical interface of the client.
func runGui(serverFlag string, codeFlag string, deviceFlag string) {
	_ = os.MkdirAll(storageRoot(), 0755)
	redirectOutputToLog()

	config := loadConfig()
	if serverUrl, systemName, appName, faviconBase64, ok := embeddedSettings(); ok {
		// The program carries the settings of the installation, they are used
		// before the user does anything.
		config.ServerUrl = serverUrl
		config.SystemName = systemName
		config.AppName = appName
		config.FaviconBase64 = faviconBase64
		writeApplicationIcon(faviconBase64)
	}
	if serverFlag != "" {
		config.ServerUrl = serverFlag
	}
	if codeFlag != "" {
		config.PairingCode = codeFlag
	}
	if deviceFlag != "" {
		config.DeviceName = deviceFlag
	}
	if config.ServerUrl == "" {
		config.ServerUrl = DefaultServerUrl
	}
	if config.DeviceName == "" {
		config.DeviceName = "Worker-Node-" + generateHWID()[:6]
	}
	if config.StorageDir == "" {
		config.StorageDir = modelDirectory()
	}
	if config.HWID == "" {
		config.HWID = generateHWID()
	}
	// The language of the user and the policies of the installation are asked
	// before the client is used.
	if !runSetup(&config) {
		return
	}
	saveConfig(config)

	applyConfig(config)

	// The entries of the system carry the name and the icon of the installation
	// from the first start.
	if config.SystemName != "" || config.AppName != "" {
		updateInstallationBranding(config.AppName, config.SystemName, config.FaviconBase64)
	}

	window := &guiWindow{statusState: stateOffline, statusText: clientText("connecting"), hovered: -1}
	window.create()

	// The worker starts in the background, the window shows its state.
	go func() {
		if config.PairingCode != "" {
			result, err := registerWorker()
			if err != nil {
				updateStatus(func(status *AppStatus) { status.LastError = err.Error() })
			} else {
				updateInstallationBranding(result.AppName, result.SystemName, result.FaviconB64)
			}
			startTunnel()
		}
		startBackgroundTasks()
	}()

	// The state of the worker is read in the background, the window never waits
	// for the worker.
	go window.watchWorker()

	window.messageLoop()
}

// applyBranding shows the name and the icon of the installation in the window.
//
// The dashboard sends its name and its icon, the window shows them as soon as
// the client is known.
func (w *guiWindow) applyBranding() {
	setText(w.window, windowTitle())
	path := applicationIconPath()
	if path == "" {
		return
	}
	icon, _, _ := procLoadImage.Call(
		0,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))),
		uintptr(imageIcon),
		0,
		0,
		uintptr(lrLoadFromFile|lrDefaultSize),
	)
	if icon == 0 {
		return
	}
	w.icon = icon
	// WM_SETICON
	procSendMessage.Call(w.window, 0x0080, 0, icon)
	procSendMessage.Call(w.window, 0x0080, 1, icon)
	w.addTrayIcon()
	w.invalidate()
}

// watchWorker reads the state of the worker and shows it in the window.
func (w *guiWindow) watchWorker() {
	for {
		time.Sleep(time.Second)
		syncStatus()
		status := readStatus()

		connection := "Bağlı değil"
		state := stateOffline
		switch {
		case status.LastError != "":
			connection = status.LastError
			state = stateError
		case status.ModelStatus == "downloading":
			connection = "Model indiriliyor"
			state = stateDownloading
		case status.Connected:
			connection = clientText("online")
			state = stateOnline
		}

		w.mutex.Lock()
		changed := connection != w.statusText || state != w.statusState || status.Connected != w.lastConnected
		w.statusText = connection
		w.statusState = state
		w.lastConnected = status.Connected
		w.progress = status.Progress
		w.progressSet = status.ModelStatus == "downloading"
		w.details = [4]string{status.ServerUrl, status.NodeId, modelDescription(status), speedDescription(status)}
		w.mutex.Unlock()

		if changed {
			w.applyBranding()
			w.invalidate()
		}
	}
}

// redirectOutputToLog writes the output of the client into its log file.
//
// The client is built as a program of the graphical user interface, it has no
// terminal.
func redirectOutputToLog() {
	file, err := os.OpenFile(logPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	os.Stdout = file
	os.Stderr = file
}

// enableDpiAwareness lets the window use the real sizes of the screen.
//
// Without the awareness of the resolution, the system scales the window of the
// client and the text of the window is blurred.
func enableDpiAwareness() {
	// PER_MONITOR_AWARE_V2
	if result, _, _ := procSetProcessDpiAwareContextV2.Call(uintptr(0xFFFFFFFC)); result != 0 {
		return
	}
	// PER_MONITOR_AWARE
	if result, _, _ := procSetProcessDpiAwareContext1.Call(uintptr(0xFFFFFFFD)); result != 0 {
		return
	}
	procSetProcessDPIAware.Call()
}

// screenDpi returns the resolution of the screen of the window in dots per inch.
//
// The value is read from the window, the window can be on a screen with another
// resolution than the screen of the system.
func (w *guiWindow) screenDpi() int32 {
	if w.window != 0 {
		dpi, _, _ := procGetDpiForWindow.Call(w.window)
		if dpi > 0 {
			return int32(dpi)
		}
	}
	if w.dpi > 0 {
		return w.dpi
	}
	return systemDpi()
}

// scaleValue returns the measure of the screen for a measure of the design.
func (w *guiWindow) scaleValue(value int32) int32 {
	return value * w.dpi / 96
}

// windowDpi returns the resolution of the screen of the window in dots per inch.
func (w *guiWindow) windowDpi() int32 {
	dpi, _, _ := procGetDpiForWindow.Call(w.window)
	return int32(dpi)
}

// systemDpi returns the resolution of the screen of the system in dots per inch.
func systemDpi() int32 {
	if dpi, _, _ := procGetDpiForSystem.Call(); dpi > 0 {
		return int32(dpi)
	}
	device, _, _ := procGetDC.Call(0)
	if device == 0 {
		return 96
	}
	defer procReleaseDC.Call(0, device)
	// LOGPIXELSX
	dpi, _, _ := procGetDeviceCaps.Call(device, 88)
	if dpi == 0 {
		return 96
	}
	return int32(dpi)
}

// create builds the window of the client.
func (w *guiWindow) create() {
	enableDpiAwareness()
	processId, _, _ := procGetCurrentProcess.Call()
	w.instance = processId
	w.className, _ = syscall.UTF16PtrFromString("ByogWorkerWindow")
	w.icon = w.loadApplicationIcon()
	w.dpi = systemDpi()

	brush, _, _ := procGetSysColorBrush.Call(uintptr(colorWindow))
	cursor, _, _ := procLoadCursor.Call(0, uintptr(resourceArrow))

	class := windowClassEx{
		Size:       uint32(unsafe.Sizeof(windowClassEx{})),
		Style:      csHRedraw | csVRedraw,
		WndProc:    syscall.NewCallback(wndProc),
		Instance:   w.instance,
		Icon:       w.icon,
		Cursor:     cursor,
		Background: brush,
		ClassName:  w.className,
	}
	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&class)))

	window, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(w.className)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("BYOG"))),
		wsOverlappedWindow|wsCaption|wsSysMenu|wsMinimizeBox|wsClipChildren,
		uintptr(cwUseDefault),
		uintptr(cwUseDefault),
		uintptr(w.scaleValue(designWidth)),
		uintptr(w.scaleValue(designHeight)),
		0,
		0,
		w.instance,
		0,
	)
	w.window = window
	if window == 0 {
		return
	}

	guiWindowsMutex.Lock()
	guiWindows[window] = w
	guiWindowsMutex.Unlock()

	loadInterfaceFont()
	// The window knows its resolution now, it gets the size of the design.
	w.dpi = w.screenDpi()
	procSetWindowPos.Call(w.window, 0, 0, 0, uintptr(w.scaleValue(designWidth+16)), uintptr(w.scaleValue(designHeight+55)), 0x0004)

	w.createFonts()
	w.createControls()
	w.layout()
	w.addTrayIcon()
	procSetTimer.Call(w.window, 1, 1000, 0)
	procShowWindow.Call(w.window, swShow)
	procUpdateWindow.Call(w.window)
}

// loadInterfaceFont adds the font of the interface to the system.
//
// The font belongs to the client, the user does not have to install it. The
// shapes of the font are written to a file of the temporary directory, the
// system of the drawing reads the font from a file.
func loadInterfaceFont() {
	addFont := func(font []byte, name string) {
		if len(font) == 0 {
			return
		}
		directory := filepath.Join(os.TempDir(), "byog-fonts")
		if err := os.MkdirAll(directory, 0755); err != nil {
			return
		}
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, font, 0644); err != nil {
			return
		}
		file, _ := syscall.UTF16PtrFromString(path)
		procAddFontResourceEx.Call(
			uintptr(unsafe.Pointer(file)),
			0x00000010, // FR_PRIVATE
			0,
		)
		// The name of the file must stay in the memory until the system is done
		// with it.
		runtime.KeepAlive(file)
	}
	addFont(interfaceFont, "DMSans-Regular.ttf")
	addFont(interfaceFontBold, "DMSans-Bold.ttf")
}

// createFonts builds the fonts of the window.
func (w *guiWindow) createFonts() {
	makeFont := func(weight int32, height int32) uintptr {
		name := syscall.StringToUTF16Ptr("DM Sans")
		font, _, _ := procCreateFont.Call(
			uintptr(-height),
			0,
			0,
			0,
			uintptr(weight),
			0,
			0,
			0,
			1, // DEFAULT_CHARSET
			0,
			0,
			5, // CLEARTYPE_QUALITY
			0,
			uintptr(unsafe.Pointer(name)),
		)
		// The name of the font must stay in the memory until the system is done
		// with it.
		runtime.KeepAlive(name)
		return font
	}
	w.fontTitle = makeFont(700, 18)
	w.fontText = makeFont(400, 14)
	w.fontSmall = makeFont(400, 13)
}

// createControls builds the fields and the buttons of the window.
func (w *guiWindow) createControls() {

	createField := func(id int, text string, left int32, top int32, width int32) uintptr {
		class := syscall.StringToUTF16Ptr("EDIT")
		title := syscall.StringToUTF16Ptr(text)
		control, _, _ := procCreateWindowEx.Call(
			0,
			uintptr(unsafe.Pointer(class)),
			uintptr(unsafe.Pointer(title)),
			uintptr(wsChild|wsVisible|esAutoHScroll),
			uintptr(w.scaleValue(left)),
			uintptr(w.scaleValue(top)),
			uintptr(w.scaleValue(width)),
			uintptr(w.scaleValue(30)),
			w.window,
			uintptr(id),
			w.instance,
			0,
		)
		// The characters of the field must stay in the memory until the system is
		// done with them.
		runtime.KeepAlive(class)
		runtime.KeepAlive(title)
		return control
	}

	// The address of the installation is known when the client comes from the
	// dashboard, the field of it is hidden then.
	w.serverFieldShown = readStatus().ServerUrl == ""
	if w.serverFieldShown {
		w.serverEdit = createField(controlServerEdit, "", 48, 196, 464)
		w.pairEdit = createField(controlPairEdit, "", 48, 276, 464)
	} else {
		w.pairEdit = createField(controlPairEdit, "", 48, 166, 464)
	}
	procSendMessage.Call(w.serverEdit, 0x0030, w.fontText, 1)
	procSendMessage.Call(w.pairEdit, 0x0030, w.fontText, 1)
	w.fillFields()
}

// setText writes a text into a window of the system.
//
// The characters of the text must stay in the memory until the system is done
// with them.
func setText(window uintptr, text string) {
	chars := syscall.StringToUTF16Ptr(text)
	procSetWindowText.Call(window, uintptr(unsafe.Pointer(chars)))
	runtime.KeepAlive(chars)
}

// fillFields writes the known values into the fields of the window.
func (w *guiWindow) fillFields() {
	status := readStatus()
	setText(w.serverEdit, status.ServerUrl)
	if status.Paired {
		setText(w.pairEdit, status.PairingCode)
	} else {
		setText(w.pairEdit, "")
	}
}

// loadApplicationIcon loads the icon of the client.
//
// The icon of the installation is used, it is the icon of the dashboard of the
// installation.
func (w *guiWindow) loadApplicationIcon() uintptr {
	path := applicationIconPath()
	if path != "" {
		icon, _, _ := procLoadImage.Call(
			0,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))),
			uintptr(imageIcon),
			0,
			0,
			uintptr(lrLoadFromFile|lrDefaultSize),
		)
		if icon != 0 {
			return icon
		}
	}
	icon, _, _ := procLoadIcon.Call(0, uintptr(resourceAppIcon))
	return icon
}

// messageLoop runs the messages of the window.
func (w *guiWindow) messageLoop() {
	var message windowMessage
	for {
		result, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
}

// invalidate asks the window to draw itself again.
func (w *guiWindow) invalidate() {
	procInvalidateRect.Call(w.window, 0, 0)
}

// readField reads the value of a field of the window.
func (w *guiWindow) readField(field uintptr) string {
	var buffer [512]uint16
	length, _, _ := procGetWindowTextLen.Call(field)
	if length > uintptr(len(buffer)-1) {
		length = uintptr(len(buffer) - 1)
	}
	procGetWindowText.Call(field, uintptr(unsafe.Pointer(&buffer[0])), length+1)
	return strings.TrimSpace(syscall.UTF16ToString(buffer[:length+1]))
}

// modelDescription describes the model of the worker for the interface.
func modelDescription(status AppStatus) string {
	if status.ModelName != "" {
		return status.ModelName
	}
	if status.ModelStatus == "downloading" {
		return fmt.Sprintf("indiriliyor (%d / %d MB)", status.DownloadedMB, status.TotalMB)
	}
	if status.TotalTokens > 0 {
		return fmt.Sprintf("%d token işlendi", status.TotalTokens)
	}
	return "model bekleniyor"
}

// wndProc handles the messages of the window of the client.
func wndProc(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	guiWindowsMutex.Lock()
	window := guiWindows[hwnd]
	guiWindowsMutex.Unlock()

	switch message {
	case wmTimer:
		if window != nil {
			window.invalidate()
		}
		return 0

	case wmPaint:
		if window != nil {
			var surface paintStruct
			device, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&surface)))
			window.paint(device)
			procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&surface)))
		}
		return 0

	case wmEraseBkgnd:
		// The window draws its background itself.
		return 1

	case wmMouseMove:
		if window != nil {
			window.trackMouse(lparam)
		}
		return 0

	case wmLButtonUp:
		if window != nil {
			window.click(lparam)
		}
		return 0

	case wmCommand:
		if window != nil {
			switch wparam & 0xFFFF {
			case controlPairEdit:
				window.connect()
			case commandShow:
				window.show()
			case commandPanel:
				window.openPanel()
			case commandSettings:
				window.openSiteSettings()
			case commandClose:
				window.closeApp()
			case commandQuit:
				window.quit()
			case commandLanguage:
				window.changeLanguage()
			}
		}
		return 0

	case wmTray:
		if window != nil {
			switch lparam {
			case notifyClick:
				window.showTrayMenu()
			case commandShow:
				window.show()
			case commandPanel:
				window.openPanel()
			case commandQuit:
				window.quit()
			}
		}
		return 0

	case wmDpiChanged:
		if window != nil {
			// The window takes the size of the design with the new resolution of
			// the screen, the measures of the design never change.
			window.dpi = window.screenDpi()
			window.createFonts()
			procSetWindowPos.Call(window.window, 0, 0, 0,
				uintptr(window.scaleValue(designWidth+16)),
				uintptr(window.scaleValue(designHeight+55)), 0x0004)
			window.layout()
		}
		return 0

	case wmClose:
		if window != nil {
			// The client stays in the notification area, the worker keeps running.
			procShowWindow.Call(window.window, swHide)
			return 0
		}
		return 0

	case wmDestroy:
		if window != nil {
			window.removeTrayIcon()
			guiWindowsMutex.Lock()
			delete(guiWindows, hwnd)
			guiWindowsMutex.Unlock()
		}
		procPostQuitMessage.Call(0)
		return 0
	}

	result, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wparam, lparam)
	return result
}

// trackMouse follows the pointer of the mouse over the buttons of the window.
func (w *guiWindow) trackMouse(lparam uintptr) {
	x := int32(lparam & 0xFFFF)
	y := int32((lparam >> 16) & 0xFFFF)
	hovered := -1
	if x < w.scaleValue(sidebarWidth) {
		for index := range menuItems() {
			top := w.scaleValue(int32(menuTop + index*menuStep))
			bottom := top + w.scaleValue(34)
			if y >= top && y <= bottom {
				hovered = index
				break
			}
		}
	} else {
		for index, button := range w.buttons {
			if x >= w.scaleValue(button.left) && x <= w.scaleValue(button.right) &&
				y >= w.scaleValue(button.top) && y <= w.scaleValue(button.bottom) {
				hovered = index
				break
			}
		}
	}
	if hovered == w.hovered {
		return
	}
	w.hovered = hovered
	if hovered >= 0 {
		cursor, _, _ := procLoadCursor.Call(0, idcHand)
		procSetCursor.Call(cursor)
		event := trackMouseEvent{
			Size:        uint32(unsafe.Sizeof(trackMouseEvent{})),
			Flags:       0x00000001 | 0x00000002, // TME_LEAVE | TME_HOVER
			TrackWindow: w.window,
			HoverTime:   100,
		}
		procTrackMouseEvent.Call(uintptr(unsafe.Pointer(&event)))
	} else {
		cursor, _, _ := procLoadCursor.Call(0, uintptr(resourceArrow))
		procSetCursor.Call(cursor)
	}
	w.invalidate()
}

// click handles a click of the mouse over a button of the window.
func (w *guiWindow) click(lparam uintptr) {
	x := int32(lparam & 0xFFFF)
	y := int32((lparam >> 16) & 0xFFFF)
	if x < w.scaleValue(sidebarWidth) {
		for index := range menuItems() {
			top := w.scaleValue(int32(menuTop + index*menuStep))
			bottom := top + w.scaleValue(34)
			if y >= top && y <= bottom {
				w.mutex.Lock()
				w.page = index
				w.mutex.Unlock()
				w.layout()
				return
			}
		}
		return
	}
	for _, button := range w.buttons {
		if x >= w.scaleValue(button.left) && x <= w.scaleValue(button.right) &&
			y >= w.scaleValue(button.top) && y <= w.scaleValue(button.bottom) {
			w.runCommand(button.command)
			return
		}
	}
}

// runCommand runs the command of a button of the window.
func (w *guiWindow) runCommand(command int) {
	switch command {
	case commandConnect:
		w.connect()
	case commandTest:
		w.testModel()
	case commandPanel:
		w.openPanel()
	case commandQuit:
		w.quit()
	}
}

// testModel runs a test inference with the model of the worker and shows the
// answer with the measurements of the inference.
//
// The inference runs in the background, the window stays open while it runs.
func (w *guiWindow) testModel() {
	status := readStatus()
	if status.ModelName == "" {
		w.showMessage("Test", "Test için önce bir model indirilmelidir.")
		return
	}
	go func() {
		started := time.Now()
		engine := NewLocalTransformerEngine(status.ModelName, State.Config.StorageDir)
		result, err := engine.GenerateStream(
			context.Background(),
			"Merhaba! Kendini tek cümleyle tanıt.",
			96,
			0.7,
			func(piece string, tokenId int) bool { return true },
		)
		elapsed := time.Since(started)
		if err != nil {
			w.showMessage("Test", "Çıkarım yapılamadı:\n"+err.Error())
			return
		}
		speed := result.Tps
		if speed <= 0 && elapsed.Seconds() > 0 {
			speed = float64(result.TotalTokens) / elapsed.Seconds()
		}
		answer := fmt.Sprintf("%s\n\nSüre: %s\nToken: %d\nHız: %.1f token/s",
			strings.TrimSpace(result.Text), elapsed.Round(time.Millisecond), result.TotalTokens, speed)
		w.mutex.Lock()
		w.testResult = answer
		w.page = pageTest
		w.mutex.Unlock()
		w.layout()
	}()
}

// showMessage shows a text of the client in a window of the system.
func (w *guiWindow) showMessage(title string, text string) {
	procMessageBox.Call(w.window, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(title))), 0x00000040)
}

// speedDescription returns the speed of the worker for the window.
func speedDescription(status AppStatus) string {
	if status.CurrentTps <= 0 {
		return clientText("idle")
	}
	return fmt.Sprintf("%.1f token/s", status.CurrentTps)
}

// changeLanguage asks the language of the user again.
//
// The language of the user is used by the texts of the interface and it is kept
// for the next start of the client.
func (w *guiWindow) changeLanguage() {
	config := loadConfig()
	if !runSetup(&config) {
		return
	}
	saveConfig(config)
	w.invalidate()
}

// connect pairs the client with the coordinator.
func (w *guiWindow) connect() {
	code := w.readField(w.pairEdit)
	server := w.readField(w.serverEdit)

	if code == "" {
		updateStatus(func(status *AppStatus) { status.LastError = "Eşleştirme kodu girilmedi" })
		w.invalidate()
		return
	}

	State.Mu.Lock()
	State.Config.PairingCode = strings.ToUpper(code)
	if server != "" {
		State.Config.ServerUrl = server
	}
	State.Mu.Unlock()
	saveConfig(State.Config)

	updateStatus(func(status *AppStatus) { status.LastError = "" })

	go func() {
		result, err := registerWorker()
		if err != nil {
			updateStatus(func(status *AppStatus) { status.LastError = err.Error() })
			return
		}
		updateInstallationBranding(result.AppName, result.SystemName, result.FaviconB64)
		startTunnel()
	}()
}

// openPanel opens the dashboard of the installation in the browser.
func (w *guiWindow) openPanel() {
	status := readStatus()
	target := strings.TrimRight(status.ServerUrl, "/")
	if target == "" {
		return
	}
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "http://" + target
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
}

// openSiteSettings opens the settings of the installation on the site.
func (w *guiWindow) openSiteSettings() {
	status := readStatus()
	target := strings.TrimRight(status.ServerUrl, "/")
	if target == "" {
		return
	}
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		target = "http://" + target
	}
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", target+"/console/personal").Start()
}

// closeApp closes the window of the client.
//
// The worker keeps serving the tasks of the coordinator, the window of the
// client stays closed until the client is started again.
func (w *guiWindow) closeApp() {
	w.removeTrayIcon()
	procShowWindow.Call(w.window, swHide)
}

// quit ends the client.
func (w *guiWindow) quit() {
	w.removeTrayIcon()
	os.Exit(0)
}

// show brings the window of the client to the front.
func (w *guiWindow) show() {
	procShowWindow.Call(w.window, swShow)
	procUpdateWindow.Call(w.window)
	w.invalidate()
}

// addTrayIcon shows the client in the notification area of the system.
func (w *guiWindow) addTrayIcon() {
	data := notifyIconData{
		Size:            uint32(unsafe.Sizeof(notifyIconData{})),
		Window:          w.window,
		IconID:          trayIconID,
		Flags:           iconFlagMessage | iconFlagIcon | iconFlagTip,
		CallbackMessage: wmTray,
		Icon:            w.icon,
	}
	tip, _ := syscall.UTF16FromString("BYOG")
	copy(data.Tip[:], tip)
	procShellNotifyIcon.Call(uintptr(iconAdd), uintptr(unsafe.Pointer(&data)))
	w.trayAdded = true
}

// removeTrayIcon removes the icon of the client from the notification area.
func (w *guiWindow) removeTrayIcon() {
	if !w.trayAdded {
		return
	}
	data := notifyIconData{
		Size:   uint32(unsafe.Sizeof(notifyIconData{})),
		Window: w.window,
		IconID: trayIconID,
	}
	procShellNotifyIcon.Call(uintptr(iconDelete), uintptr(unsafe.Pointer(&data)))
	w.trayAdded = false
}

// showTrayMenu shows the menu of the icon in the notification area.
func (w *guiWindow) showTrayMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	procAppendMenu.Call(menu, 0, commandShow, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(clientText("show")))))
	procAppendMenu.Call(menu, 0, commandPanel, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(clientText("go_site")))))
	procAppendMenu.Call(menu, 0, commandSettings, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(clientText("site_settings")))))
	procAppendMenu.Call(menu, 0, commandClose, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(clientText("close_app")))))

	var point windowPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	procSetForeground.Call(w.window)
	procTrackPopupMenu.Call(menu, 0, uintptr(point.X), uintptr(point.Y), 0, w.window, 0)
	procDestroyMenu.Call(menu)
}

// updateInstallationBranding writes the name and the icon of the installation
// into the entries of the system.
//
// The dashboard sends its name and its icon, the client shows them in the
// control panel and in the search of the system.
func updateInstallationBranding(appName string, systemName string, faviconBase64 string) {
	writeApplicationIcon(faviconBase64)
	_ = installer.UpdateBranding(installationDisplayName(appName, systemName), WorkerVersion)
}

// installationDisplayName returns the name of the installation.
//
// The name of the installation and the name of the client are shown together,
// the user knows the installation from the name of it.
func installationDisplayName(appName string, systemName string) string {
	name := strings.TrimSpace(systemName)
	if name == "" {
		return appName
	}
	return name + " BYOG Worker"
}

// applicationIconPath returns the path of the icon of the client.
//
// The icon is written into the program directory of the installation, it is
// the icon of the dashboard of the installation.
func applicationIconPath() string {
	candidates := []string{
		filepath.Join(installer.Directory(), "byog.ico"),
		filepath.Join(storageRoot(), "byog.ico"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// writeApplicationIcon writes the icon of the client from a picture of the
// installation.
//
// The dashboard sends the icon as a picture in the base64 format, the file of
// the icon is written from it.
func writeApplicationIcon(base64Image string) {
	if strings.TrimSpace(base64Image) == "" {
		return
	}
	payload := base64Image
	if index := strings.Index(payload, ","); index >= 0 {
		payload = payload[index+1:]
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(data) < 8 {
		return
	}
	// Only a picture in the format of PNG can be used as an icon.
	if string(data[1:4]) != "PNG" {
		return
	}
	_ = os.MkdirAll(storageRoot(), 0755)
	_ = os.WriteFile(filepath.Join(storageRoot(), "byog.ico"), buildIconFile(data), 0644)
}
