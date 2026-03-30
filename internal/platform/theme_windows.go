//go:build windows

package platform

import (
	"golang.org/x/sys/windows/registry"
)

// SystemThemeIsDark reads the Windows AppsUseLightTheme registry value.
// returns true (dark) on any error as a safe default.
func SystemThemeIsDark() bool {
	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return true
	}
	defer k.Close()

	val, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return true
	}
	return val == 0
}
