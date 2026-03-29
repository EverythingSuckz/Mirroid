//go:build windows

package platform

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// CopyImageToClipboard copies an image file to the system clipboard using PowerShell.
func CopyImageToClipboard(filePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	escapedPath := strings.ReplaceAll(filePath, "'", "''")
	psCmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
		`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Clipboard]::SetImage([System.Drawing.Image]::FromFile('`+escapedPath+`'))`)
	HideConsole(psCmd)
	return psCmd.Run()
}
