//go:build windows

package main

// The window of the setup of the client.
//
// The client asks the language of the user at the first start and it shows the
// privacy policy and the terms of service of the installation. The user has to
// accept them before the client is used.

import (
	"encoding/json"
	"net/http"
	"syscall"
	"time"
	"unsafe"
)

// The identifiers of the controls of the window of the setup.
const (
	setupLanguageCombo = 200
	setupPrivacyEdit   = 201
	setupTermsEdit     = 202
	setupPrivacyCheck  = 203
	setupTermsCheck    = 204
	setupContinue      = 205
)

// setupLanguage is a language of the installation.
type setupLanguage struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// setupPolicy holds the privacy policy and the terms of service.
type setupPolicy struct {
	Privacy string `json:"privacy"`
	Terms   string `json:"terms"`
}

// setupBootstrap is the answer of the installation.
type setupBootstrap struct {
	Languages      []setupLanguage        `json:"languages"`
	SystemLanguage string                 `json:"system_language"`
	Policies       map[string]setupPolicy `json:"policies"`
}

// setupWindow is the window of the setup of the client.
type setupWindow struct {
	window     uintptr
	instance   uintptr
	language   []setupLanguage
	policies   map[string]setupPolicy
	selected   int
	combo      uintptr
	privacyBox uintptr
	termsBox   uintptr
	finished   bool
}

// The windows of the setup.
var setupWindows = map[uintptr]*setupWindow{}

// runSetup asks the language of the user and shows the policies of the
// installation.
//
// The setup is skipped when the language of the user is known and the policies
// are accepted.
func runSetup(config *Config) bool {
	if config.Language != "" && config.LegalAccepted {
		clientLanguage = config.Language
		return true
	}

	bootstrap := fetchBootstrap(config.ServerUrl)
	if len(bootstrap.Languages) == 0 {
		// The installation does not answer, the client is used with the language
		// of the system.
		clientLanguage = systemLanguage()
		config.Language = clientLanguage
		config.LegalAccepted = true
		return true
	}

	window := &setupWindow{
		instance: moduleHandle(),
		language: bootstrap.Languages,
		policies: bootstrap.Policies,
	}
	window.selected = selectedLanguage(bootstrap.Languages, bootstrap.SystemLanguage)
	window.create()
	return window.finished
}

// fetchBootstrap reads the languages and the policies of the installation.
func fetchBootstrap(serverUrl string) setupBootstrap {
	var bootstrap setupBootstrap
	if serverUrl == "" {
		return bootstrap
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(serverUrl + "/api/v1/worker/bootstrap")
	if err != nil {
		return bootstrap
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return bootstrap
	}
	if err := json.NewDecoder(response.Body).Decode(&bootstrap); err != nil {
		return setupBootstrap{}
	}
	return bootstrap
}

// selectedLanguage returns the place of the language of the user.
//
// The language of the system is used when it is one of the languages of the
// installation, English is used when it is not.
func selectedLanguage(languages []setupLanguage, systemLanguage string) int {
	for index, item := range languages {
		if item.Code == systemLanguage {
			return index
		}
	}
	for index, item := range languages {
		if item.Code == "en" {
			return index
		}
	}
	return 0
}

// systemLanguage returns the language of the system of the user.
func systemLanguage() string {
	language, _, _ := kernel32.NewProc("GetUserDefaultUILanguage").Call()
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

// create builds the window of the setup and waits for the user.
func (w *setupWindow) create() {
	className, _ := syscall.UTF16PtrFromString("ByogWorkerSetup")
	class := windowClassEx{
		Size:      uint32(unsafe.Sizeof(windowClassEx{})),
		Style:     0x0003 | 0x0008 | 0x0002,
		WndProc:   syscall.NewCallback(setupWindowProcedure),
		Instance:  w.instance,
		ClassName: className,
	}
	procRegisterClassEx.Call(uintptr(unsafe.Pointer(&class)))

	title, _ := syscall.UTF16PtrFromString(clientText("setup"))
	w.window, _, _ = procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		wsOverlappedWindow|wsCaption|wsSysMenu|wsMinimizeBox|wsVisible,
		120, 100, 700, 580,
		0, 0, w.instance, 0,
	)
	if w.window == 0 {
		w.finished = true
		return
	}

	setupWindows[w.window] = w
	w.createControls()
	procShowWindow.Call(w.window, swShow)
	procUpdateWindow.Call(w.window)

	// The window of the setup has its own message loop, it waits for the user.
	for !w.finished {
		var message windowMessage
		result, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&message)))
	}
	procDestroyWindow.Call(w.window)
	delete(setupWindows, w.window)
}

// createControls builds the fields of the window of the setup.
func (w *setupWindow) createControls() {
	create := func(class string, text string, id int, style uintptr, left int32, top int32, width int32, height int32) uintptr {
		className, _ := syscall.UTF16PtrFromString(class)
		title, _ := syscall.UTF16PtrFromString(text)
		control, _, _ := procCreateWindowEx.Call(
			0,
			uintptr(unsafe.Pointer(className)),
			uintptr(unsafe.Pointer(title)),
			wsChild|wsVisible|style,
			uintptr(left), uintptr(top), uintptr(width), uintptr(height),
			w.window, uintptr(id), w.instance, 0,
		)
		return control
	}

	create("STATIC", clientText("language"), 0, 0, 24, 20, 240, 24)
	w.combo = create("COMBOBOX", "", setupLanguageCombo, 0x03, 24, 48, 260, 300)
	for _, item := range w.language {
		text, _ := syscall.UTF16PtrFromString(item.Name)
		procSendMessage.Call(w.combo, 0x0143, 0, uintptr(unsafe.Pointer(text))) // CB_ADDSTRING
	}
	procSendMessage.Call(w.combo, 0x014E, uintptr(w.selected), 0) // CB_SETCURSEL

	create("STATIC", clientText("privacy"), 0, 0, 24, 100, 240, 24)
	privacy := create("EDIT", "", setupPrivacyEdit, 0x0800|0x0400|0x0020, 24, 126, 640, 130)
	setControlText(privacy, w.policyText("privacy"))

	create("STATIC", clientText("terms"), 0, 0, 24, 270, 240, 24)
	terms := create("EDIT", "", setupTermsEdit, 0x0800|0x0400|0x0020, 24, 296, 640, 130)
	setControlText(terms, w.policyText("terms"))

	w.privacyBox = create("BUTTON", clientText("accept_privacy"), setupPrivacyCheck, 0x03, 24, 442, 640, 26)
	w.termsBox = create("BUTTON", clientText("accept_terms"), setupTermsCheck, 0x03, 24, 474, 640, 26)

	create("BUTTON", clientText("continue"), setupContinue, 0x01, 544, 516, 120, 32)
}

// setControlText writes a text into a control of the system.
func setControlText(control uintptr, text string) {
	if text == "" {
		return
	}
	chars, err := syscall.UTF16FromString(text)
	if err != nil {
		return
	}
	procSendMessage.Call(control, 0x000C, uintptr(unsafe.Pointer(&chars[0])), 1)
}

// policyText returns the policy of the chosen language.
//
// The policy of English is used when the language has no policy, the policy of
// any language is used when English has no policy either.
func (w *setupWindow) policyText(kind string) string {
	if len(w.language) == 0 {
		return ""
	}
	code := w.language[w.selected].Code
	if policy, ok := w.policies[code]; ok {
		if value := policyValue(policy, kind); value != "" {
			return value
		}
	}
	if policy, ok := w.policies["en"]; ok {
		if value := policyValue(policy, kind); value != "" {
			return value
		}
	}
	for _, item := range w.language {
		if policy, ok := w.policies[item.Code]; ok {
			if value := policyValue(policy, kind); value != "" {
				return value
			}
		}
	}
	return ""
}

// policyValue returns the privacy policy or the terms of service.
func policyValue(policy setupPolicy, kind string) string {
	if kind == "terms" {
		return policy.Terms
	}
	return policy.Privacy
}

// setupWindowProcedure is the procedure of the window of the setup.
func setupWindowProcedure(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
	switch message {
	case 0x0111: // WM_COMMAND
		setup := setupWindows[hwnd]
		if setup == nil {
			break
		}
		if lParam&0xFFFF != setupContinue {
			break
		}
		privacy, _, _ := procSendMessage.Call(setup.privacyBox, 0x00F0, 0, 0) // BM_GETCHECK
		terms, _, _ := procSendMessage.Call(setup.termsBox, 0x00F0, 0, 0)
		if privacy != 1 || terms != 1 {
			break
		}
		current, _, _ := procSendMessage.Call(setup.combo, 0x0147, 0, 0) // CB_GETCURSEL
		index := int(current)
		if index >= 0 && index < len(setup.language) {
			setup.selected = index
			clientLanguage = setup.language[index].Code
		}
		setup.finished = true
		procDestroyWindow.Call(hwnd)
		return 0
	case 0x0002: // WM_DESTROY
		if setup := setupWindows[hwnd]; setup != nil {
			setup.finished = true
		}
		procPostQuitMessage.Call(0)
		return 0
	}
	result, _, _ := procDefWindowProc.Call(hwnd, uintptr(message), wParam, lParam)
	return result
}

// moduleHandle returns the handle of the program of the client.
func moduleHandle() uintptr {
	handle, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	return handle
}
