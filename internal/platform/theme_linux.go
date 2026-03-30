//go:build linux

package platform

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// SystemThemeIsDark queries the freedesktop color-scheme setting via
// the org.freedesktop.portal.Settings D-Bus interface.
// returns true when the desktop prefers dark mode.
func SystemThemeIsDark() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx,
		"dbus-send", "--session", "--print-reply=literal",
		"--dest=org.freedesktop.portal.Desktop",
		"/org/freedesktop/portal/desktop",
		"org.freedesktop.portal.Settings.Read",
		"string:org.freedesktop.appearance",
		"string:color-scheme",
	).Output()
	if err != nil {
		return false
	}
	// color-scheme: 0 = no preference, 1 = prefer dark, 2 = prefer light
	return strings.Contains(string(out), "uint32 1")
}
