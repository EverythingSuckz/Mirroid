//go:build !windows

package platform

import "fmt"

// CopyImageToClipboard is not supported on this platform.
func CopyImageToClipboard(filePath string) error {
	return fmt.Errorf("clipboard image copy not supported on this platform")
}
