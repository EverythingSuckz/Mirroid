//go:build darwin

package platform

import (
	"os/exec"
	"strings"
)

// SystemThemeIsDark checks the macOS AppleInterfaceStyle user default.
// returns true when set to "Dark", false otherwise.
func SystemThemeIsDark() bool {
	out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		return false // missing key means light mode
	}
	return strings.TrimSpace(string(out)) == "Dark"
}
