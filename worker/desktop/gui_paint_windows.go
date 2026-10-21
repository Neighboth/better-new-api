//go:build windows

package main

// The drawing of the window of the client.
//
// Every measure comes from the design of the window, the window scales the
// design with the resolution of the screen. The window looks the same on every
// screen.

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

// uiColor is a color of the window.
type uiColor struct {
	Red   uint8
	Green uint8
	Blue  uint8
}

// The colors of the window.
var (
	colorBackground = uiColor{0xF2, 0xF4, 0xF8}
	colorCard       = uiColor{0xFF, 0xFF, 0xFF}
	colorBorder     = uiColor{0xE4, 0xE7, 0xEC}
	colorText       = uiColor{0x10, 0x18, 0x28}
	colorMuted      = uiColor{0x66, 0x70, 0x85}
	colorAccent     = uiColor{0x2F, 0x6D, 0xF6}
	colorAccentText = uiColor{0xFF, 0xFF, 0xFF}
	colorSuccess    = uiColor{0x12, 0xB7, 0x6A}
	colorWarning    = uiColor{0xF7, 0x90, 0x09}
	colorError      = uiColor{0xF0, 0x44, 0x38}
	colorField      = uiColor{0xF9, 0xFA, 0xFB}
)

// design returns the measure of the design.
//
// The system scales the measures of the design with the resolution of the
// screen, the window of the client is drawn with them.
func design(value int32) int32 {
	return value
}

// layout places the fields and the buttons of the window after a change of the
// resolution of the screen.
func (w *guiWindow) layout() {
	moveField := func(field uintptr, left int32, top int32, width int32) {
		procSetWindowPos.Call(field, 0, uintptr(w.scaleValue(left)), uintptr(w.scaleValue(top)),
			uintptr(w.scaleValue(width)), uintptr(w.scaleValue(30)), 0x0004) // SWP_NOZORDER
	}
	if w.serverFieldShown {
		moveField(w.serverEdit, 48, 196, 464)
		moveField(w.pairEdit, 48, 276, 464)
	} else {
		moveField(w.pairEdit, 48, 166, 464)
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

	// The background of the window.
	fill(dc, 0, 0, width, height, colorBackground)

	w.mutex.Lock()
	statusText := w.statusText
	statusState := w.statusState
	details := w.details
	progress := w.progress
	progressSet := w.progressSet
	hovered := w.hovered
	w.mutex.Unlock()

	// The card of the state of the worker.
	drawRoundRect(dc, design(24), design(24), design(536), design(136), design(16), colorCard, colorBorder, true)
	fill(dc, design(24), design(24), design(31), design(136), colorAccent)

	w.drawText(dc, windowTitle(), w.fontTitle, colorText, design(52), design(34), design(330), design(62), taLeft|taVCenter)

	// The state of the worker with its color.
	stateColor := colorMuted
	switch statusState {
	case stateOnline:
		stateColor = colorSuccess
	case stateDownloading:
		stateColor = colorWarning
	case stateError:
		stateColor = colorError
	}
	drawEllipse(dc, design(54), design(78), design(66), design(90), stateColor)
	w.drawText(dc, statusText, w.fontText, colorText, design(78), design(70), design(330), design(98), taLeft|taVCenter)

	// The details of the worker.
	labels := [4]string{clientText("server"), clientText("device"), clientText("model"), clientText("speed")}
	for index, label := range labels {
		top := design(int32(30 + index*24))
		w.drawText(dc, label, w.fontSmall, colorMuted, design(350), top, design(420), top+design(22), taLeft|taVCenter)
		w.drawText(dc, details[index], w.fontSmall, colorText, design(424), top, design(520), top+design(22), taLeft|taVCenter)
	}

	// The card of the fields.
	drawRoundRect(dc, design(24), design(152), design(536), design(332), design(16), colorCard, colorBorder, true)
	if w.serverFieldShown {
		w.drawText(dc, clientText("server_url"), w.fontSmall, colorMuted, design(48), design(166), design(320), design(190), taLeft|taVCenter)
		w.drawText(dc, clientText("pairing"), w.fontSmall, colorMuted, design(48), design(246), design(320), design(270), taLeft|taVCenter)
	} else {
		w.drawText(dc, clientText("pairing"), w.fontSmall, colorMuted, design(48), design(166), design(320), design(190), taLeft|taVCenter)
	}

	// The bar of the download.
	if progressSet {
		drawRoundRect(dc, design(48), design(344), design(512), design(352), design(4), colorField, colorBorder, true)
		length := int32(464) * int32(progress) / 100
		if length > 0 {
			drawRoundRect(dc, design(48), design(344), design(48+length), design(352), design(4), colorAccent, colorAccent, false)
		}
	}

	// The buttons of the window.
	for index, button := range w.buttons {
		background := colorField
		text := colorText
		if button.primary {
			background = colorAccent
			text = colorAccentText
		} else if index == hovered {
			background = uiColor{0xE8, 0xEE, 0xFD}
		}
		drawRoundRect(dc, design(button.left), design(button.top), design(button.right), design(button.bottom),
			design(8), background, colorBorder, !button.primary)
		w.drawText(dc, button.label, w.fontText, text, design(button.left), design(button.top),
			design(button.right), design(button.bottom), taLeft|taVCenter)
	}

	procBitBlt.Call(hdc, 0, 0, uintptr(width), uintptr(height), dc, 0, 0, srccopy)
	procSelectObject.Call(dc, old)
	procDeleteObject.Call(bitmap)
	procDeleteDC.Call(dc)
}

// windowTitle returns the title of the window of the client.
//
// The title is the name of the installation with the name of the client.
func windowTitle() string {
	status := readStatus()
	if status.SystemName != "" {
		return status.SystemName + " BYOG"
	}
	if status.AppName != "" && status.AppName != "BYOG Worker" {
		return status.AppName
	}
	return "BYOG"
}
