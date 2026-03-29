//go:build windows

package platform

import (
	"os/exec"
	"strings"
)

// CopyImageToClipboard copies an image file to the system clipboard using PowerShell.
func CopyImageToClipboard(filePath string) error {
	escapedPath := strings.ReplaceAll(filePath, "'", "''")
	psCmd := exec.Command("powershell", "-NoProfile", "-Command",
		`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Clipboard]::SetImage([System.Drawing.Image]::FromFile('`+escapedPath+`'))`)
	HideConsole(psCmd)
	return psCmd.Run()
}
