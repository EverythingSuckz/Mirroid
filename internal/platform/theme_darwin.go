//go:build darwin

package platform

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// SystemThemeIsDark checks the macOS AppleInterfaceStyle user default.
// returns true when set to "Dark", false otherwise.
func SystemThemeIsDark() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "Dark"
}
