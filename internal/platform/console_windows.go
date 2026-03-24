//go:build windows

package platform

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// HideConsole fully hides the subprocess: no console window AND the process
// starts hidden (SW_HIDE). Use for console-only tools like adb where you
// never want any window to appear.
func HideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

// SuppressConsole prevents a console window from being allocated but does NOT
// hide the process's own GUI window. Use for GUI apps like scrcpy that need
// their SDL window visible but shouldn't spawn a console.
func SuppressConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNoWindow,
	}
}
