//go:build windows

package main

// The drawing of the window of the client.
//
// The window has a menu on the left, the page of the menu is shown on the right.
// Every measure comes from the design of the window, the window scales the
// design with the resolution of the screen.

import (
	"runtime"
	"syscall"
	"unsafe"
)

// The flags of the drawing of the system.
const (
	srccopy       = 0x00CC0020
	transparent   = 0x00000001
	taLeft        = 0x00000000
	taVCenter     = 0x00000004
	taSingleLine  = 0x00000020
	taEndEllipsis = 0x00008000
)

// The pages of the window of the client.
const (
	pageStatus   = 0
	pagePair     = 1
	pageSettings = 2
	pageTest     = 3
)

// The measures of the design of the window.
const (
	designWidth  = 680
	designHeight = 262
	sidebarWidth = 164
	menuTop      = 28
	menuStep     = 46
	contentLeft  = 188
	contentRight = 656
)

// uiColor is a color of the window.
type uiColor struct {
	Red   uint8
	Green uint8
	Blue  uint8
}

// The colors of the window.
var (
	colorBackground = uiColor{0xF4, 0xF6, 0xFA}
	colorSidebar    = uiColor{0xFF, 0xFF, 0xFF}
	colorCard       = uiColor{0xFF, 0xFF, 0xFF}
	colorBorder     = uiColor{0xE3, 0xE8, 0xF0}
	colorText       = uiColor{0x10, 0x18, 0x28}
	colorMuted      = uiColor{0x64, 0x71, 0x87}
	colorAccent     = uiColor{0x2F, 0x6D, 0xF6}
	colorAccentSoft = uiColor{0xE8, 0xEF, 0xFD}
	colorAccentText = uiColor{0xFF, 0xFF, 0xFF}
	colorSuccess    = uiColor{0x12, 0xB7, 0x6A}
	colorWarning    = uiColor{0xF7, 0x90, 0x09}
	colorError      = uiColor{0xF0, 0x44, 0x38}
	colorField      = uiColor{0xF8, 0xFA, 0xFC}
)

// design returns the measure of the design.
func design(value int32) int32 {
	return value
}

// menuItems returns the entries of the menu of the window.
func menuItems() []struct {
	key   string
	label string
} {
	return []struct {
		key   string
		label string
	}{
		{"menu_status", clientText("menu_status")},
		{"menu_pair", clientText("pairing")},
		{"menu_settings", clientText("menu_settings")},
		{"menu_test", clientText("test")},
	}
}

// layout places the fields and the buttons of the window for the page of the
// menu.
func (w *guiWindow) layout() {
	moveField := func(field uintptr, left int32, top int32, width int32) {
		procSetWindowPos.Call(field, 0, uintptr(w.scaleValue(left)), uintptr(w.scaleValue(top)),
			uintptr(w.scaleValue(width)), uintptr(w.scaleValue(30)), 0x0004) // SWP_NOZORDER
	}
	showField := func(field uintptr, visible bool) {
		if visible {
			procShowWindow.Call(field, swShow)
			return
		}
		procShowWindow.Call(field, swHide)
	}

	w.buttons = nil
	switch w.page {
	case pagePair:
		showField(w.serverEdit, false)
		showField(w.pairEdit, true)
		moveField(w.pairEdit, contentLeft, 68, contentRight-contentLeft)
		w.buttons = []uiButton{
			{command: commandConnect, left: contentLeft, top: 118, right: contentLeft + 150, bottom: 156, label: clientText("connect"), primary: true},
		}
	case pageSettings:
		showField(w.serverEdit, true)
		showField(w.pairEdit, false)
		moveField(w.serverEdit, contentLeft, 68, contentRight-contentLeft)
		w.buttons = []uiButton{
			{command: commandLanguage, left: contentLeft, top: 118, right: contentLeft + 200, bottom: 156, label: clientText("change_language")},
		}
	case pageTest:
		showField(w.serverEdit, false)
		showField(w.pairEdit, false)
		w.buttons = []uiButton{
			{command: commandTest, left: contentLeft, top: 68, right: contentLeft + 150, bottom: 106, label: clientText("test"), primary: true},
		}
	default:
		showField(w.serverEdit, false)
		showField(w.pairEdit, false)
	}
	w.invalidate()
}

// fill draws a rectangle with a color.
func fill(dc uintptr, left int32, top int32, right int32, bottom int32, color uiColor) {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(uint32(color.Blue)<<16 | uint32(color.Green)<<8 | uint32(color.Red)))
	rect := [4]int32{left, top, right, bottom}
	procFillRect.Call(dc, uintptr(unsafe.Pointer(&rect[0])), brush)
	// The measure of the rectangle must stay in the memory until the drawing is
	// done.
	runtime.KeepAlive(&rect)
	procDeleteObject.Call(brush)
}

// drawRoundRect draws a rectangle with rounded corners.
func drawRoundRect(dc uintptr, left int32, top int32, right int32, bottom int32, radius int32, color uiColor, border uiColor, withBorder bool) {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(uint32(color.Blue)<<16 | uint32(color.Green)<<8 | uint32(color.Red)))
	pen := uintptr(0)
	if withBorder {
		pen, _, _ = procCreatePen.Call(1, 0, uintptr(uint32(border.Blue)<<16|uint32(border.Green)<<8|uint32(border.Red)))
		procSelectObject.Call(dc, pen)
	}
	procSelectObject.Call(dc, brush)
	procRoundRect.Call(dc, uintptr(left), uintptr(top), uintptr(right), uintptr(bottom), uintptr(radius), uintptr(radius))
	procSelectObject.Call(dc, brush)
	if withBorder {
		procSelectObject.Call(dc, pen)
		procDeleteObject.Call(pen)
	}
	procDeleteObject.Call(brush)
}

// drawEllipse draws an ellipse with a color.
func drawEllipse(dc uintptr, left int32, top int32, right int32, bottom int32, color uiColor) {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(uint32(color.Blue)<<16 | uint32(color.Green)<<8 | uint32(color.Red)))
	pen, _, _ := procCreatePen.Call(1, 0, uintptr(uint32(color.Blue)<<16|uint32(color.Green)<<8|uint32(color.Red)))
	procSelectObject.Call(dc, pen)
	procSelectObject.Call(dc, brush)
	procEllipse.Call(dc, uintptr(left), uintptr(top), uintptr(right), uintptr(bottom))
	procSelectObject.Call(dc, brush)
	procDeleteObject.Call(pen)
	procDeleteObject.Call(brush)
}

// drawText draws a text of the window.
func (w *guiWindow) drawText(dc uintptr, text string, font uintptr, color uiColor, left int32, top int32, right int32, bottom int32, flags uint32) {
	procSetBkMode.Call(dc, transparent)
	procSetTextColor.Call(dc, uintptr(uint32(color.Blue)<<16|uint32(color.Green)<<8|uint32(color.Red)))
	procSelectObject.Call(dc, font)
	text16 := syscall.StringToUTF16Ptr(text)
	rect := [4]int32{left, top, right, bottom}
	procDrawText.Call(dc,
		uintptr(unsafe.Pointer(text16)),
		uintptr(^uintptr(0)), // The length of the text is computed by the system.
		uintptr(unsafe.Pointer(&rect[0])),
		uintptr(flags|taSingleLine|taEndEllipsis))
	// The text and the measure of the text must stay in the memory until the
	// drawing is done.
	runtime.KeepAlive(text16)
	runtime.KeepAlive(&rect)
}

// paint draws the window of the client.
func (w *guiWindow) paint(hdc uintptr) {
	// The measures of the design are the measures of the window, the system
	// scales them with the resolution of the screen.
	width := int32(designWidth)
	height := int32(designHeight)

	dc, _, _ := procCreateCompatibleDC.Call(hdc)
	bitmap, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(width), uintptr(height))
	old, _, _ := procSelectObject.Call(dc, bitmap)

	fill(dc, 0, 0, width, height, colorBackground)

	w.mutex.Lock()
	statusText := w.statusText
	statusState := w.statusState
	details := w.details
	progress := w.progress
	progressSet := w.progressSet
	hovered := w.hovered
	page := w.page
	testResult := w.testResult
	w.mutex.Unlock()

	w.paintSidebar(dc, page, hovered)

	switch page {
	case pagePair:
		w.paintPair(dc, statusText)
	case pageSettings:
		w.paintSettings(dc)
	case pageTest:
		w.paintTest(dc, details, testResult)
	default:
		w.paintStatus(dc, statusText, statusState, details, progress, progressSet)
	}

	procBitBlt.Call(hdc, 0, 0, uintptr(width), uintptr(height), dc, 0, 0, srccopy)
	procSelectObject.Call(dc, old)
	procDeleteObject.Call(bitmap)
	procDeleteDC.Call(dc)
}

// paintSidebar draws the menu of the window.
func (w *guiWindow) paintSidebar(dc uintptr, page int, hovered int) {
	fill(dc, 0, 0, design(sidebarWidth), design(designHeight), colorSidebar)
	fill(dc, design(sidebarWidth)-1, 0, design(sidebarWidth), design(designHeight), colorBorder)

	for index, item := range menuItems() {
		top := design(int32(menuTop + index*menuStep))
		bottom := top + design(34)
		if index == page {
			drawRoundRect(dc, design(12), top, design(sidebarWidth-12), bottom, design(8), colorAccentSoft, colorAccentSoft, false)
			fill(dc, design(12), top+design(10), design(16), bottom-design(10), colorAccent)
		} else if index == hovered {
			drawRoundRect(dc, design(12), top, design(sidebarWidth-12), bottom, design(8), colorField, colorField, false)
		}
		color := colorText
		if index == page {
			color = colorAccent
		}
		w.drawText(dc, item.label, w.fontText, color, design(28), top, design(sidebarWidth-14), bottom, taLeft|taVCenter)
	}
}

// paintStatus draws the page of the state of the worker.
func (w *guiWindow) paintStatus(dc uintptr, statusText string, statusState int, details [4]string, progress int, progressSet bool) {
	drawRoundRect(dc, design(contentLeft), design(24), design(contentRight), design(214), design(14), colorCard, colorBorder, true)

	w.drawText(dc, windowTitle(), w.fontTitle, colorText, design(contentLeft+24), design(38), design(contentLeft+300), design(66), taLeft|taVCenter)

	stateColor := colorMuted
	switch statusState {
	case stateOnline:
		stateColor = colorSuccess
	case stateDownloading:
		stateColor = colorWarning
	case stateError:
		stateColor = colorError
	}
	drawEllipse(dc, design(contentLeft+26), design(90), design(contentLeft+38), design(102), stateColor)
	w.drawText(dc, statusText, w.fontText, colorText, design(contentLeft+50), design(82), design(contentLeft+290), design(110), taLeft|taVCenter)

	labels := [4]string{clientText("server"), clientText("device"), clientText("model"), clientText("speed")}
	for index, label := range labels {
		top := design(int32(88 + index*26))
		w.drawText(dc, label, w.fontSmall, colorMuted, design(contentLeft+300), top, design(contentLeft+368), top+design(22), taLeft|taVCenter)
		w.drawText(dc, details[index], w.fontSmall, colorText, design(contentLeft+372), top, design(contentRight-24), top+design(22), taLeft|taVCenter)
	}

	if progressSet {
		drawRoundRect(dc, design(contentLeft), design(236), design(contentRight), design(244), design(4), colorField, colorBorder, true)
		length := int32(contentRight-contentLeft) * int32(progress) / 100
		if length > 0 {
			drawRoundRect(dc, design(contentLeft), design(236), design(contentLeft+length), design(244), design(4), colorAccent, colorAccent, false)
		}
	}
}

// paintPair draws the page of the pairing of the client.
func (w *guiWindow) paintPair(dc uintptr, statusText string) {
	drawRoundRect(dc, design(contentLeft), design(24), design(contentRight), design(214), design(14), colorCard, colorBorder, true)

	w.drawText(dc, clientText("pairing"), w.fontTitle, colorText, design(contentLeft+24), design(40), design(contentRight-24), design(68), taLeft|taVCenter)
	w.drawText(dc, clientText("pairing"), w.fontSmall, colorMuted, design(contentLeft+24), design(70), design(contentRight-24), design(92), taLeft|taVCenter)
	w.drawText(dc, statusText, w.fontSmall, colorMuted, design(contentLeft+24), design(176), design(contentRight-24), design(200), taLeft|taVCenter)
}

// paintSettings draws the page of the settings of the client.
func (w *guiWindow) paintSettings(dc uintptr) {
	drawRoundRect(dc, design(contentLeft), design(24), design(contentRight), design(214), design(14), colorCard, colorBorder, true)

	w.drawText(dc, clientText("menu_settings"), w.fontTitle, colorText, design(contentLeft+24), design(40), design(contentRight-24), design(68), taLeft|taVCenter)
	w.drawText(dc, clientText("server_url"), w.fontSmall, colorMuted, design(contentLeft+24), design(70), design(contentRight-24), design(92), taLeft|taVCenter)
}

// paintTest draws the page of the test of the model.
func (w *guiWindow) paintTest(dc uintptr, details [4]string, result string) {
	drawRoundRect(dc, design(contentLeft), design(24), design(contentRight), design(264), design(14), colorCard, colorBorder, true)

	w.drawText(dc, clientText("test"), w.fontTitle, colorText, design(contentLeft+24), design(40), design(contentRight-24), design(68), taLeft|taVCenter)
	w.drawText(dc, details[2], w.fontSmall, colorMuted, design(contentLeft+24), design(70), design(contentRight-24), design(92), taLeft|taVCenter)

	if result != "" {
		drawRoundRect(dc, design(contentLeft+24), design(120), design(contentRight-24), design(244), design(10), colorField, colorBorder, true)
		w.drawText(dc, result, w.fontSmall, colorText, design(contentLeft+36), design(132), design(contentRight-36), design(234), taLeft|taVCenter)
	}
}

// windowTitle returns the title of the window of the client.
//
// The title is the name of the installation with the name of the client.
func windowTitle() string {
	status := readStatus()
	if title := installationDisplayName(status.AppName, status.SystemName); title != "" {
		return title
	}
	return "BYOG Worker"
}
