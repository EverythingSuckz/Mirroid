//go:build !windows

package platform

// SetTitleBarDarkMode is a no-op on non-Windows platforms where the
// window manager controls title bar appearance automatically.
func SetTitleBarDarkMode(hwnd uintptr, dark bool) {}
