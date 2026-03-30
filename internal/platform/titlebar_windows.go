//go:build windows

package platform

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const dwmwaUseImmersiveDarkMode = 20

const (
	swpNoMove       = 0x0002
	swpNoSize       = 0x0001
	swpNoZOrder     = 0x0004
	swpFrameChanged = 0x0020
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procSetWindowPos = user32.NewProc("SetWindowPos")
)

// SetTitleBarDarkMode sets the DWM immersive dark mode attribute on the
// window and forces a frame repaint via SetWindowPos.
func SetTitleBarDarkMode(hwnd uintptr, dark bool) {
	var value int32
	if dark {
		value = 1
	}
	_ = windows.DwmSetWindowAttribute(
		windows.HWND(hwnd),
		dwmwaUseImmersiveDarkMode,
		unsafe.Pointer(&value),
		uint32(unsafe.Sizeof(value)),
	)
	procSetWindowPos.Call(
		hwnd, 0,
		0, 0, 0, 0,
		swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged,
	)
}
