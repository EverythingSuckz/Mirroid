//go:build !windows

package platform

import (
	"os"
	"os/exec"
)

// OpenTerminal opens a new terminal window running the given command.
func OpenTerminal(command string, args ...string) error {
	term := os.Getenv("TERMINAL")
	if term == "" {
		term = "xterm"
	}
	allArgs := append([]string{"-e", command}, args...)
	return exec.Command(term, allArgs...).Start()
}
