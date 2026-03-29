//go:build windows

package platform

import "os/exec"

// OpenTerminal opens a new terminal window running the given command.
func OpenTerminal(command string, args ...string) error {
	allArgs := append([]string{"/c", "start", "cmd", "/k", command}, args...)
	return exec.Command("cmd", allArgs...).Start()
}
