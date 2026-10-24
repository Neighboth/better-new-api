//go:build windows

package main

// The window of the client of the installation of Windows.
//
// The page of the client is shown in the window of the client itself, the bars
// of a browser are not shown. The window runs in a process of its own: a
// runtime of WebView2 that is not complete on the computer of the user cannot
// stop the worker. The window of the program shows the state of the worker when
// the window of WebView2 cannot be started.
//
// The program carries the runtime of the loader of WebView2, the computer of
// the user does not have to have the package of the developer.

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// webView2Loader is the loader of WebView2 of the package of the developer.
//
// The loader finds the runtime of WebView2 of the system, the program of the
// client does not carry the runtime itself.
//
//go:embed assets/WebView2Loader.dll
var webView2Loader []byte

const (
	webViewWindowClass = "ByogWorkerWebView"
	webViewWindowTitle = "BYOG Worker"

	webViewMessageSize    = 0x0005
	webViewMessageClose   = 0x8001 // WM_APP+1, the client closes the window
	webViewMessageDestroy = 0x0002
	webViewMessageSetIcon = 0x0080
	wmSetText             = 0x000C
	wmSysCommand          = 0x0112
	scClose               = 0xF060
	wsMaximizeBox         = 0x00010000
	wsMaximize            = 0x01000000
	swMaximize            = 3
)

var (
	ole32                     = syscall.NewLazyDLL("ole32.dll")
	advapi32                  = syscall.NewLazyDLL("advapi32.dll")
	procWebViewFindWindow     = user32.NewProc("FindWindowW")
	procWebViewSetForeground  = user32.NewProc("SetForegroundWindow")
	procWebViewPostMessage    = user32.NewProc("PostMessageW")
	procWebViewGetClientRect  = user32.NewProc("GetClientRect")
	procWebViewSystemInfo     = user32.NewProc("SystemParametersInfoW")
	procWebViewCoInitializeEx = ole32.NewProc("CoInitializeEx")
	procWebViewCoUninitialize = ole32.NewProc("CoUninitialize")
	procRegOpenKeyEx          = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueEx       = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey           = advapi32.NewProc("RegCloseKey")

	webViewWindow     uintptr
	webViewController uintptr
	webViewUrl        string
	webViewHandlers   []any
	webViewLoaderDLL  *syscall.DLL
	// webViewReady tells whether the page of the client is shown in the window.
	webViewReady atomic.Bool
)

// runWebViewHelper shows the page of the client in the window of the client.
//
// The program ends with an error when the runtime of WebView2 is not on the
// computer of the user, the window of the program shows the state of the worker
// then.
func runWebViewHelper(url string, title string, iconPath string) (code int) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer func() {
		if recovered := recover(); recovered != nil {
			webViewLog(fmt.Sprintf("panic: %v", recovered))
			code = 4
		}
	}()

	// A fatal error of the runtime of the page is written into the log of the
	// client, the window of the page has no terminal of its own.
	redirectOutputToLog()

	webViewUrl = url
	webViewLog("start " + url)

	// The window of the page uses the real pixels of the screen. Without the
	// awareness the system scales the window and the page does not fill it.
	enableDpiAwareness()

	// The thread of the window is a thread of COM, the runtime of WebView2
	// lives on it.
	result, _, _ := procWebViewCoInitializeEx.Call(0, 2) // COINIT_APARTMENTTHREADED
	if int32(result) < 0 && uint32(result) != 1 {
		webViewLog("the thread of COM could not be started")
		return 2
	}
	defer procWebViewCoUninitialize.Call()

	if title == "" {
		title = webViewWindowTitle
	}
	if createWebViewWindow(title, iconPath) == 0 {
		webViewLog("the window of the client could not be created")
		return 2
	}
	if !createWebViewEnvironment() {
		webViewLog("the runtime of WebView2 could not be started")
		procDestroyWindow.Call(webViewWindow)
		return 3
	}

	// The page of the client is shown when the controller of the page is
	// ready. A runtime that does not answer would leave a window without a
	// page, the window is closed after a time and the window of the program
	// shows the state of the worker.
	time.AfterFunc(15*time.Second, func() {
		if !webViewReady.Load() {
			webViewLog("the page of the client is not ready")
			procWebViewPostMessage.Call(webViewWindow, webViewMessageClose, 0, 0)
		}
	})

	webViewMessageLoop()
	webViewLog("end")
	return 0
}

// createWebViewWindow builds the window of the client.
func createWebViewWindow(title string, iconPath string) uintptr {
	className, _ := syscall.UTF16PtrFromString(webViewWindowClass)
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	brush, _, _ := procGetSysColorBrush.Call(uintptr(colorWindow))
	class := windowClassEx{
		Size:       uint32(unsafe.Sizeof(windowClassEx{})),
		Style:      0x0003 | 0x0008 | 0x0002, // CS_HREDRAW, CS_VREDRAW, CS_DBLCLKS
		WndProc:    syscall.NewCallback(webViewWindowProcedure),
		Instance:   instance,
		Background: brush,
		ClassName:  className,
	}

	// The window is placed in the work area of the screen and it takes most of
	// it. The page of the client is a full application, a small window would
	// hide the content.
	x, y := uintptr(0x80000000), uintptr(0x80000000)
	width, height := uintptr(1100), uintptr(760)
	var workArea clientRect
	if result, _, _ := procWebViewSystemInfo.Call(0x0030, 0, uintptr(unsafe.Pointer(&workArea)), 0); result != 0 {
		availableWidth := workArea.Right - workArea.Left
		availableHeight := workArea.Bottom - workArea.Top
		if availableWidth > 640 && availableHeight > 480 {
			width = uintptr(availableWidth - 80)
			height = uintptr(availableHeight - 80)
			x = uintptr(workArea.Left + 40)
			y = uintptr(workArea.Top + 40)
		}
	}

	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&class)))

	titlePtr, _ := syscall.UTF16PtrFromString(title)
	webViewWindow, _, _ = procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(titlePtr)),
		wsOverlappedWindow|wsCaption|wsSysMenu|wsMinimizeBox|wsMaximizeBox|wsClipChildren|wsMaximize,
		uintptr(x), uintptr(y), uintptr(width), uintptr(height),
		0, 0, instance, 0,
	)
	if webViewWindow == 0 {
		return 0
	}

	if iconPath != "" {
		icon, _, _ := procLoadImage.Call(
			0,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(iconPath))),
			uintptr(imageIcon),
			0, 0,
			uintptr(lrLoadFromFile|lrDefaultSize),
		)
		if icon != 0 {
			procSendMessage.Call(webViewWindow, webViewMessageSetIcon, 0, icon)
			procSendMessage.Call(webViewWindow, webViewMessageSetIcon, 1, icon)
		}
	}
	return webViewWindow
}

// webViewWindowProcedure is the procedure of the window of the client.
func webViewWindowProcedure(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
	switch message {
	case webViewMessageSize, 0x0003, 0x0047, 0x0232: // WM_SIZE, WM_MOVE, WM_WINDOWPOSCHANGED, WM_EXITSIZEMOVE
		resizeWebView()
		return 0
	case 0x02E0: // WM_DPICHANGED
		if lParam != 0 {
			rect := (*struct{ Left, Top, Right, Bottom int32 })(unsafe.Pointer(lParam))
			procSetWindowPos.Call(hwnd, 0,
				uintptr(rect.Left), uintptr(rect.Top),
				uintptr(rect.Right-rect.Left), uintptr(rect.Bottom-rect.Top),
				0x0004) // SWP_NOZORDER
		}
		resizeWebView()
		return 0
	case webViewMessageClose:
		webViewLog("the window of the page is closed by the client")
		procDestroyWindow.Call(hwnd)
		return 0
	case wmSetText:
		// The title of the window is the name of the installation. After the
		// page is shown, the page cannot replace it with the title of the
		// document.
		if webViewReady.Load() {
			return 0
		}
	case wmClose:
		// A close message that the client did not send is not obeyed. The page
		// of the client is the window of the application, a message of the
		// system or of the page must not close it.
		webViewLog("the window of the page received WM_CLOSE")
		return 0
	case wmSysCommand:
		if wParam == scClose {
			webViewLog("the window of the page is closed by the user")
			procDestroyWindow.Call(hwnd)
			return 0
		}
	case webViewMessageDestroy:
		webViewLog("the window of the page is destroyed")
		procPostQuitMessage.Call(0)
		return 0
	}
	result, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

// webViewMessageLoop waits for the messages of the window of the client.
func webViewMessageLoop() {
	var message windowMessage
	for {
		result, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			webViewLog(fmt.Sprintf("the message loop of the page ended: %d", int32(result)))
			return
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
}

// createWebViewEnvironment starts the runtime of WebView2.
func createWebViewEnvironment() bool {
	directory := webViewDataDirectory()
	if err := os.MkdirAll(directory, 0755); err != nil {
		webViewLog("the folder of the page could not be created: " + err.Error())
		return false
	}

	loaderPath := ensureWebViewLoader()
	loader, err := syscall.LoadDLL(loaderPath)
	if err != nil {
		webViewLog("the loader of WebView2 could not be loaded: " + err.Error())
		return false
	}
	webViewLoaderDLL = loader
	create, err := loader.FindProc("CreateCoreWebView2EnvironmentWithOptions")
	if err != nil {
		webViewLog("the loader of WebView2 has no environment: " + err.Error())
		return false
	}

	// The page follows the theme of the system, the runtime of WebView2 is
	// told the theme before it starts.
	if systemThemeIsDark() {
		_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--force-dark-mode")
	}

	var folder uintptr
	if custom := os.Getenv("BYOG_WEBVIEW2_DIR"); custom != "" {
		if _, err := os.Stat(custom); err == nil {
			path, _ := syscall.UTF16PtrFromString(custom)
			folder = uintptr(unsafe.Pointer(path))
			runtime.KeepAlive(path)
		}
	}
	dataPath, _ := syscall.UTF16PtrFromString(directory)
	result, _, _ := create.Call(folder, uintptr(unsafe.Pointer(dataPath)), 0, newWebViewEnvironmentHandler())
	runtime.KeepAlive(dataPath)
	if int32(result) < 0 {
		webViewLog("the environment of WebView2 could not be created")
		return false
	}
	return true
}

// ensureWebViewLoader writes the loader of WebView2 to the computer of the
// user and returns its path.
func ensureWebViewLoader() string {
	if custom := os.Getenv("BYOG_WEBVIEW2_LOADER"); custom != "" {
		if _, err := os.Stat(custom); err == nil {
			return custom
		}
	}
	path := filepath.Join(webViewDataDirectory(), "WebView2Loader.dll")
	if data, err := os.ReadFile(path); err == nil && len(data) == len(webView2Loader) {
		return path
	}
	if err := os.WriteFile(path, webView2Loader, 0644); err != nil {
		// The folder of the program is used when the folder of the user
		// cannot be written.
		if executable, err := os.Executable(); err == nil {
			return filepath.Join(filepath.Dir(executable), "WebView2Loader.dll")
		}
	}
	return path
}

// webViewDataDirectory returns the folder of the page of the client.
func webViewDataDirectory() string {
	if custom := os.Getenv("BYOG_STORAGE_DIR"); custom != "" {
		return filepath.Join(custom, "webview")
	}
	if root := os.Getenv("LOCALAPPDATA"); root != "" {
		return filepath.Join(root, "BYOG Worker")
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, ".byog", "webview")
	}
	return filepath.Join(os.TempDir(), "byog-webview")
}

// systemThemeIsDark tells whether the system uses the dark theme.
func systemThemeIsDark() bool {
	path, err := syscall.UTF16PtrFromString(`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`)
	if err != nil {
		return false
	}
	var key uintptr
	result, _, _ := procRegOpenKeyEx.Call(0x80000001, uintptr(unsafe.Pointer(path)), 0, 0x20019, uintptr(unsafe.Pointer(&key)))
	if int32(result) != 0 || key == 0 {
		return false
	}
	defer procRegCloseKey.Call(key)
	var value uint32
	size := uint32(4)
	name, _ := syscall.UTF16PtrFromString("AppsUseLightTheme")
	result, _, _ = procRegQueryValueEx.Call(key, uintptr(unsafe.Pointer(name)), 0, 0, uintptr(unsafe.Pointer(&value)), uintptr(unsafe.Pointer(&size)))
	if int32(result) != 0 {
		return false
	}
	return value == 0
}

// resizeWebView gives the page the size of the window.
func resizeWebView() {
	webViewLog(fmt.Sprintf("resize called controller=%d window=%d", webViewController, webViewWindow))
	if webViewController == 0 || webViewWindow == 0 {
		return
	}
	var rect struct{ Left, Top, Right, Bottom int32 }
	procWebViewGetClientRect.Call(webViewWindow, uintptr(unsafe.Pointer(&rect)))
	width := rect.Right - rect.Left
	height := rect.Bottom - rect.Top
	webViewLog(fmt.Sprintf("resize %dx%d", width, height))
	bounds := struct{ Left, Top, Right, Bottom int32 }{0, 0, width, height}
	callWebViewMethod(webViewController, 6, uintptr(unsafe.Pointer(&bounds))) // put_Bounds
	callWebViewMethod(webViewController, 23)                                  // NotifyParentWindowPositionChanged
}

// callWebViewMethod calls a method of an interface of COM.
func callWebViewMethod(object uintptr, index int, args ...uintptr) uintptr {
	if object == 0 {
		return 0
	}
	table := *(*uintptr)(unsafe.Pointer(object))
	method := *(*uintptr)(unsafe.Pointer(table + uintptr(index)*unsafe.Sizeof(uintptr(0))))
	if method == 0 {
		return 0
	}
	call := append([]uintptr{object}, args...)
	result, _, _ := syscall.SyscallN(method, call...)
	return result
}

// newWebViewEnvironmentHandler returns the object that receives the
// environment of WebView2.
func newWebViewEnvironmentHandler() uintptr {
	handler := &webViewHandler{
		invoke: func(self uintptr, result uintptr, environment uintptr) uintptr {
			if int32(result) < 0 || environment == 0 {
				webViewLog("the environment of WebView2 is not ready")
				procPostQuitMessage.Call(3)
				return 0
			}
			webViewLog("the environment of WebView2 is ready")
			// CreateCoreWebView2Controller(environment, window, handler)
			callWebViewMethod(environment, 3, webViewWindow, newWebViewControllerHandler())
			return 0
		},
	}
	handler.vtbl = &webViewVtbl{
		QueryInterface: syscall.NewCallback(webViewQueryInterface),
		AddRef:         syscall.NewCallback(webViewAddRef),
		Release:        syscall.NewCallback(webViewRelease),
		Invoke:         syscall.NewCallback(webViewInvoke),
	}
	webViewHandlers = append(webViewHandlers, handler)
	webViewHandlers = append(webViewHandlers, handler.vtbl)
	return uintptr(unsafe.Pointer(handler))
}

// newWebViewControllerHandler returns the object that receives the controller
// of the page.
func newWebViewControllerHandler() uintptr {
	handler := &webViewHandler{
		invoke: func(self uintptr, result uintptr, controller uintptr) uintptr {
			if int32(result) < 0 || controller == 0 {
				webViewLog("the controller of the page is not ready")
				procPostQuitMessage.Call(3)
				return 0
			}
			// The controller is owned by the answer of WebView2. The window
			// keeps it after the handler returns, so the reference of the
			// answer is taken before the object is stored.
			callWebViewMethod(controller, 1) // AddRef
			webViewController = controller
			resizeWebView()
			callWebViewMethod(controller, 4, 1) // put_IsVisible
			procShowWindow.Call(webViewWindow, swMaximize)
			// The window is shown with the flag as well. A window that is
			// created while the process of the page starts can stay hidden
			// when only ShowWindow is used.
			procSetWindowPos.Call(webViewWindow, 0, 0, 0, 0, 0, 0x0001|0x0002|0x0040) // SWP_NOSIZE | SWP_NOMOVE | SWP_SHOWWINDOW
			webViewLog("the window of the page is shown")
			procUpdateWindow.Call(webViewWindow)
			// The window is shown before the page is loaded. The size of the
			// page is set again when the system has placed the window. The
			// message is posted to the thread of COM, the methods of WebView2
			// are not called from a timer thread.
			time.AfterFunc(250*time.Millisecond, func() {
				procWebViewPostMessage.Call(webViewWindow, webViewMessageSize, 0, 0)
			})

			var page uintptr
			callWebViewMethod(controller, 25, uintptr(unsafe.Pointer(&page))) // get_CoreWebView2
			if page == 0 {
				webViewLog("the page of the client is not available")
				procWebViewPostMessage.Call(webViewWindow, webViewMessageClose, 0, 0)
				return 0
			}
			webViewLog("the page of the client is available")
			var settings uintptr
			callWebViewMethod(page, 3, uintptr(unsafe.Pointer(&settings))) // get_Settings
			if settings != 0 {
				callWebViewMethod(settings, 10, 0) // put_AreDefaultContextMenusEnabled
				callWebViewMethod(settings, 12, 0) // put_AreDevToolsEnabled
				callWebViewMethod(settings, 14, 0) // put_IsStatusBarEnabled
				callWebViewMethod(settings, 26, 1) // put_IsZoomControlEnabled
			}
			webViewLog("the settings of the page are ready")

			// The page is ready when the navigation completed, not when the
			// controller was created. A page that cannot be loaded closes the
			// window so that the window of the program shows the state of the
			// worker instead of a blank page.
			navHandler := newWebViewNavigationHandler()
			var token int64
			handlerResult := callWebViewMethod(page, 15, navHandler, uintptr(unsafe.Pointer(&token))) // add_NavigationCompleted
			webViewLog(fmt.Sprintf("the navigation handler is ready: %#x", handlerResult))

			target, err := syscall.UTF16PtrFromString(webViewUrl)
			if err == nil {
				handlerResult = callWebViewMethod(page, 5, uintptr(unsafe.Pointer(target))) // Navigate
				webViewLog(fmt.Sprintf("the page of the client is started: %#x", handlerResult))
				runtime.KeepAlive(target)
			}
			return 0
		},
	}
	handler.vtbl = &webViewVtbl{
		QueryInterface: syscall.NewCallback(webViewQueryInterface),
		AddRef:         syscall.NewCallback(webViewAddRef),
		Release:        syscall.NewCallback(webViewRelease),
		Invoke:         syscall.NewCallback(webViewInvoke),
	}
	webViewHandlers = append(webViewHandlers, handler)
	webViewHandlers = append(webViewHandlers, handler.vtbl)
	return uintptr(unsafe.Pointer(handler))
}

// newWebViewNavigationHandler returns the object that receives the result of
// the page navigation.
func newWebViewNavigationHandler() uintptr {
	handler := &webViewHandler{
		invoke: func(self uintptr, sender uintptr, args uintptr) uintptr {
			var success int32
			callWebViewMethod(args, 3, uintptr(unsafe.Pointer(&success))) // get_IsSuccess
			if success == 0 {
				webViewLog("the page of the client could not be loaded")
				procWebViewPostMessage.Call(webViewWindow, webViewMessageClose, 0, 0)
				return 0
			}
			webViewReady.Store(true)
			webViewLog("the page of the client is shown")
			resizeWebView()
			return 0
		},
	}
	handler.vtbl = &webViewVtbl{
		QueryInterface: syscall.NewCallback(webViewQueryInterface),
		AddRef:         syscall.NewCallback(webViewAddRef),
		Release:        syscall.NewCallback(webViewRelease),
		Invoke:         syscall.NewCallback(webViewInvoke),
	}
	webViewHandlers = append(webViewHandlers, handler)
	webViewHandlers = append(webViewHandlers, handler.vtbl)
	return uintptr(unsafe.Pointer(handler))
}

// webViewVtbl holds the functions of a COM object of WebView2.
type webViewVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	Invoke         uintptr
}

// webViewHandler is the object of the answers of WebView2.
type webViewHandler struct {
	vtbl   *webViewVtbl
	invoke func(self, result, object uintptr) uintptr
}

// webViewQueryInterface answers the interface of a COM object of WebView2.
func webViewQueryInterface(self uintptr, riid uintptr, object uintptr) uintptr {
	if object != 0 {
		*(*uintptr)(unsafe.Pointer(object)) = self
	}
	return 0
}

// webViewAddRef keeps a COM object of WebView2 alive.
func webViewAddRef(self uintptr) uintptr {
	return 1
}

// webViewRelease releases a COM object of WebView2.
func webViewRelease(self uintptr) uintptr {
	return 1
}

// webViewInvoke calls the function of a COM object of WebView2.
func webViewInvoke(self uintptr, result uintptr, object uintptr) uintptr {
	handler := (*webViewHandler)(unsafe.Pointer(self))
	if handler != nil && handler.invoke != nil {
		return handler.invoke(self, result, object)
	}
	return 0
}

// showWebViewWindow brings the window of the client to the front.
func showWebViewWindow() bool {
	className, _ := syscall.UTF16PtrFromString(webViewWindowClass)
	window, _, _ := procWebViewFindWindow.Call(uintptr(unsafe.Pointer(className)), 0)
	runtime.KeepAlive(className)
	if window == 0 {
		return false
	}
	procShowWindow.Call(window, swMaximize)
	procWebViewSetForeground.Call(window)
	return true
}

// closeWebViewWindow closes the window of the client.
func closeWebViewWindow() bool {
	className, _ := syscall.UTF16PtrFromString(webViewWindowClass)
	window, _, _ := procWebViewFindWindow.Call(uintptr(unsafe.Pointer(className)), 0)
	runtime.KeepAlive(className)
	if window == 0 {
		return false
	}
	procWebViewPostMessage.Call(window, webViewMessageClose, 0, 0)
	return true
}

// webViewLog writes a message of the window of the client to the log file.
func webViewLog(message string) {
	path := filepath.Join(storageRoot(), "webview.log")
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	if file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		_, _ = file.WriteString(time.Now().Format("2006-01-02 15:04:05") + " " + message + "\n")
		_ = file.Close()
	}
}
