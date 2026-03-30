//go:build !windows && !darwin && !linux

package platform

// SystemThemeIsDark returns false (light) on unsupported platforms.
func SystemThemeIsDark() bool {
	return false
}
