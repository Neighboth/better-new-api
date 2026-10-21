//go:build windows

package main

// The graphical interface of the Windows client.
//
// The client is a window of its own, it is shown in the search of the system and
// it can be removed with the applications of the control panel. The worker runs
// in the background, the window only shows the state of the worker.
//
// The functions of the system are called directly, the client has no other
// dependency.

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/QuantumNous/new-api/worker/desktop/installer"
)

// The constants of the system.
const (
	wmDestroy   = 0x0002
	wmClose     = 0x0010
	wmCommand   = 0x0111
	wmTimer     = 0x0113
	wmTray      = 0x8000 + 1
	wmRefresh   = 0x8000 + 2
	notifyClick = 0x40000000

	csHRedraw = 0x0002
	csVRedraw = 0x0001

	wsOverlappedWindow = 0x00CF0000
	wsCaption          = 0x00C00000
	wsSysMenu          = 0x00080000
	wsMinimizeBox      = 0x00020000
	wsChild            = 0x40000000
	wsVisible          = 0x10000000

	ssLeft          = 0x0000
	bsPushButton    = 0x0000
	bsDefaultPush   = 0x0001
	esAutoHScroll   = 0x0080
	swShow          = 5
	swHide          = 0
	colorWindow     = 5
	imageIcon       = 1
	lrLoadFromFile  = 0x0010
	lrDefaultSize   = 0x0040
	cwUseDefault    = 0x80000000
	iconAdd         = 0x00000000
	iconDelete      = 0x00000002
	iconFlagMessage = 0x00000001
	iconFlagIcon    = 0x00000002
	iconFlagTip     = 0x00000004
	commandShow     = 100
	commandPanel    = 101
	commandQuit     = 102
	commandConnect  = 103
	commandRefresh  = 104

	// The ids of the controls of the window.
	controlPairEdit   = 201
	controlPairButton = 202
	controlPanel      = 203
	controlRefresh    = 204
	controlQuit       = 205
	controlServerEdit = 206

	// The messages of the window.
	wmSetFont = 0x0030

	// The stock objects of the system.
	stockDefaultGuiFont = 17

	// The border of the fields of the window.
	wsBorder         = 0x00800000
	trayIconID       = 1
	resourceArrow    = 32512
	resourceAppIcon  = 32512
	sysMenuSeparator = 0x0800
)

// The libraries of the system.
var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	procRegisterClassEx   = user32.NewProc("RegisterClassExW")
	procCreateWindowEx    = user32.NewProc("CreateWindowExW")
	procDefWindowProc     = user32.NewProc("DefWindowProcW")
	procGetMessage        = user32.NewProc("GetMessageW")
	procTranslateMessage  = user32.NewProc("TranslateMessage")
	procDispatchMessage   = user32.NewProc("DispatchMessageW")
	procPostQuitMessage   = user32.NewProc("PostQuitMessage")
	procSetWindowText     = user32.NewProc("SetWindowTextW")
	procGetWindowText     = user32.NewProc("GetWindowTextW")
	procGetWindowTextLen  = user32.NewProc("GetWindowTextLengthW")
	procLoadIcon          = user32.NewProc("LoadIconW")
	procLoadCursor        = user32.NewProc("LoadCursorW")
	procLoadImage         = user32.NewProc("LoadImageW")
	procGetSysColorBrush  = user32.NewProc("GetSysColorBrush")
	procSetTimer          = user32.NewProc("SetTimer")
	procShowWindow        = user32.NewProc("ShowWindow")
	procUpdateWindow      = user32.NewProc("UpdateWindow")
	procCreatePopupMenu   = user32.NewProc("CreatePopupMenu")
	procAppendMenu        = user32.NewProc("AppendMenuW")
	procTrackPopupMenu    = user32.NewProc("TrackPopupMenu")
	procDestroyMenu       = user32.NewProc("DestroyMenu")
	procGetCursorPos      = user32.NewProc("GetCursorPos")
	procSetForeground     = user32.NewProc("SetForegroundWindow")
	procDestroyWindow     = user32.NewProc("DestroyWindow")
	procGetCurrentProcess = kernel32.NewProc("GetCurrentProcessId")
	procShellNotifyIcon   = shell32.NewProc("Shell_NotifyIconW")
	gdi32                 = syscall.NewLazyDLL("gdi32.dll")

	procGetStockObject            = gdi32.NewProc("GetStockObject")
	procSendMessage               = user32.NewProc("SendMessageW")
	procSetProcessDpiAwareContext = user32.NewProc("SetProcessDpiAwarenessContext")
	procSetProcessDPIAware        = user32.NewProc("SetProcessDPIAware")
	procGetDpiForSystem           = user32.NewProc("GetDpiForSystem")
)

// windowClassEx is the description of the class of the window.
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

// windowMessage is a message of the window.
type windowMessage struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   struct{ X, Y int32 }
}

// windowPoint is a point of the screen.
type windowPoint struct {
	X int32
	Y int32
}

// notifyIconData is the structure of the icon in the notification area.
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

// guiWindow holds the controls of the window of the client.
type guiWindow struct {
	window      uintptr
	statusLabel uintptr
	nodeLabel   uintptr
	modelLabel  uintptr
	serverEdit  uintptr
	pairEdit    uintptr
	icon        uintptr
	trayAdded   bool
	instance    uintptr
	className   *uint16
	mutex       sync.Mutex
	lastStatus  string
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
	applyConfig(config)
	saveConfig(config)

	window := &guiWindow{}
	window.create()

	// The worker starts in the background, the window shows its state.
	go func() {
		if config.PairingCode != "" {
			result, err := registerWorker()
			if err != nil {
				updateStatus(func(status *AppStatus) { status.LastError = err.Error() })
			} else {
				updateInstallationBranding(result.AppName, result.FaviconB64)
			}
			startTunnel()
		}
		startBackgroundTasks()
	}()

	window.messageLoop()
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
// Without the awareness of the resolution, the system draws the text of the
// controls bigger than the controls themselves.
func enableDpiAwareness() {
	// PER_MONITOR_AWARE_V2
	context, _, _ := procSetProcessDpiAwareContext.Call(uintptr(0xFFFFFFFC))
	if context == 0 {
		procSetProcessDPIAware.Call()
	}
}

// scale returns the measure of the screen for a value of the layout.
//
// The controls of the window keep their size on a screen with a high
// resolution.
func scale(value int32) int32 {
	dpi, _, _ := procGetDpiForSystem.Call()
	if dpi == 0 {
		dpi = 96
	}
	return value * int32(dpi) / 96
}

// create builds the window of the client.
func (w *guiWindow) create() {
	enableDpiAwareness()
	processId, _, _ := procGetCurrentProcess.Call()
	w.instance = processId
	w.className, _ = syscall.UTF16PtrFromString("ByogWorkerWindow")
	w.icon = w.loadApplicationIcon()

	class := windowClassEx{
		Size:       uint32(unsafe.Sizeof(windowClassEx{})),
		Style:      csHRedraw | csVRedraw,
		WndProc:    syscall.NewCallback(wndProc),
		Instance:   w.instance,
		Icon:       w.icon,
		Cursor:     w.loadCursor(),
		Background: w.loadBrush(),
		ClassName:  w.className,
	}
	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&class)))

	window, _, _ := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(w.className)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("BYOG Worker"))),
		wsOverlappedWindow|wsCaption|wsSysMenu|wsMinimizeBox,
		uintptr(cwUseDefault),
		uintptr(cwUseDefault),
		uintptr(scale(460)),
		uintptr(scale(360)),
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

	w.createControls()
	w.fillFields()
	w.addTrayIcon()
	procSetTimer.Call(w.window, 1, 1000, 0)
	procShowWindow.Call(w.window, swShow)
	procUpdateWindow.Call(w.window)
	w.refresh()
}

// loadCursor loads the pointer of the mouse of the window.
func (w *guiWindow) loadCursor() uintptr {
	cursor, _, _ := procLoadCursor.Call(0, uintptr(resourceArrow))
	return cursor
}

// loadBrush loads the brush of the background of the window.
func (w *guiWindow) loadBrush() uintptr {
	brush, _, _ := procGetSysColorBrush.Call(uintptr(colorWindow))
	return brush
}

// createControls builds the controls of the window.
func (w *guiWindow) createControls() {
	font, _, _ := procGetStockObject.Call(uintptr(stockDefaultGuiFont))

	child := func(id int, class string, text string, style uint32, x int32, y int32, width int32, height int32) uintptr {
		control, _, _ := procCreateWindowEx.Call(
			0,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(class))),
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
			uintptr(wsChild|wsVisible|style),
			uintptr(scale(x)),
			uintptr(scale(y)),
			uintptr(scale(width)),
			uintptr(scale(height)),
			w.window,
			uintptr(id),
			w.instance,
			0,
		)
		procSendMessage.Call(control, wmSetFont, font, 1)
		return control
	}

	w.statusLabel = child(0, "STATIC", "Bağlanılıyor...", ssLeft, 20, 18, 400, 22)
	child(0, "STATIC", "Cihaz:", ssLeft, 20, 54, 90, 20)
	w.nodeLabel = child(0, "STATIC", "-", ssLeft, 120, 54, 300, 20)
	child(0, "STATIC", "Model:", ssLeft, 20, 82, 90, 20)
	w.modelLabel = child(0, "STATIC", "-", ssLeft, 120, 82, 300, 20)

	child(0, "STATIC", "Sunucu adresi:", ssLeft, 20, 116, 100, 20)
	w.serverEdit = child(controlServerEdit, "EDIT", "", esAutoHScroll|wsBorder, 20, 138, 362, 26)
	child(0, "STATIC", "Eşleştirme kodu:", ssLeft, 20, 172, 120, 20)
	w.pairEdit = child(controlPairEdit, "EDIT", "", esAutoHScroll|wsBorder, 20, 194, 250, 26)
	child(controlPairButton, "BUTTON", "Bağlan", bsDefaultPush, 282, 192, 100, 30)
	child(controlPanel, "BUTTON", "Panele git", bsPushButton, 20, 240, 120, 30)
	child(controlRefresh, "BUTTON", "Yeniden bağlan", bsPushButton, 150, 240, 140, 30)
	child(controlQuit, "BUTTON", "Çıkış", bsPushButton, 300, 240, 100, 30)
	child(0, "STATIC", "Bilgiler worker.log dosyasına yazılır.", ssLeft, 20, 282, 400, 20)
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

// refresh reads the state of the worker and shows it.
func (w *guiWindow) refresh() {
	syncStatus()
	status := readStatus()

	connection := "Bağlı değil"
	switch {
	case status.ModelStatus == "downloading":
		connection = fmt.Sprintf("Çevrimiçi (İndiriliyor %%%d)", status.Progress)
	case status.Connected:
		connection = "Çevrimiçi"
	}
	if status.LastError != "" {
		connection = status.LastError
	}

	w.mutex.Lock()
	changed := connection != w.lastStatus
	w.lastStatus = connection
	w.mutex.Unlock()

	if changed {
		procSetWindowText.Call(w.statusLabel, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(connection))))
	}
	procSetWindowText.Call(w.nodeLabel, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(status.NodeId))))
	procSetWindowText.Call(w.modelLabel, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(modelDescription(status)))))
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
	tip, _ := syscall.UTF16FromString("BYOG Worker")
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
	procAppendMenu.Call(menu, 0, commandShow, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Göster"))))
	procAppendMenu.Call(menu, 0, commandPanel, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Panele git"))))
	procAppendMenu.Call(menu, 0, commandQuit, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Çıkış"))))

	var point windowPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	procSetForeground.Call(w.window)
	procTrackPopupMenu.Call(menu, 0, uintptr(point.X), uintptr(point.Y), 0, w.window, 0)
	procDestroyMenu.Call(menu)
}

// wndProc handles the messages of the window of the client.
func wndProc(hwnd uintptr, message uint32, wparam, lparam uintptr) uintptr {
	guiWindowsMutex.Lock()
	window := guiWindows[hwnd]
	guiWindowsMutex.Unlock()

	switch message {
	case wmTimer:
		if window != nil {
			window.refresh()
		}
		return 0

	case wmCommand:
		if window != nil {
			switch wparam & 0xFFFF {
			case controlPairButton:
				window.connect()
			case controlPanel:
				window.openPanel()
			case controlRefresh:
				window.reconnect()
			case controlQuit:
				window.quit()
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

	case wmRefresh:
		if window != nil {
			window.show()
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

// show brings the window of the client to the front.
func (w *guiWindow) show() {
	procShowWindow.Call(w.window, swShow)
	procUpdateWindow.Call(w.window)
	w.refresh()
}

// fillFields writes the known values into the fields of the window.
func (w *guiWindow) fillFields() {
	status := readStatus()
	procSetWindowText.Call(w.serverEdit, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(status.ServerUrl))))
	procSetWindowText.Call(w.pairEdit, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(status.PairingCode))))
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

// connect pairs the client with the coordinator.
func (w *guiWindow) connect() {
	code := w.readField(w.pairEdit)
	server := w.readField(w.serverEdit)

	if code == "" {
		updateStatus(func(status *AppStatus) { status.LastError = "Eşleştirme kodu girilmedi" })
		w.refresh()
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
		updateInstallationBranding(result.AppName, result.FaviconB64)
		startTunnel()
	}()
}

// reconnect registers the worker again and opens the tunnel.
func (w *guiWindow) reconnect() {
	updateStatus(func(status *AppStatus) { status.LastError = "" })
	go func() {
		if _, err := registerWorker(); err != nil {
			updateStatus(func(status *AppStatus) { status.LastError = err.Error() })
			return
		}
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

// quit ends the client.
func (w *guiWindow) quit() {
	w.removeTrayIcon()
	os.Exit(0)
}

// updateInstallationBranding writes the name and the icon of the installation
// into the entries of the system.
//
// The dashboard sends its name and its icon, the client shows them in the
// control panel and in the search of the system.
func updateInstallationBranding(appName string, faviconBase64 string) {
	writeApplicationIcon(faviconBase64)
	_ = installer.UpdateBranding(appName, WorkerVersion)
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
