//go:build !windows && !darwin

package platform

import "os/exec"

// OpenFolder opens the given path in the OS file manager.
func OpenFolder(path string) error {
	return exec.Command("xdg-open", path).Start()
}
