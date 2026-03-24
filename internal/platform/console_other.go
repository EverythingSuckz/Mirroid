//go:build !windows

package platform

import "os/exec"

// HideConsole is a no-op on non-Windows platforms.
func HideConsole(cmd *exec.Cmd) {}

// SuppressConsole is a no-op on non-Windows platforms.
func SuppressConsole(cmd *exec.Cmd) {}
