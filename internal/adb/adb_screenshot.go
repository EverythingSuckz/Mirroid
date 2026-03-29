package adb

import (
	"context"
	"fmt"
	"log"
	"os/exec"

	"mirroid/internal/platform"
)

// TakeScreenshot captures the device screen to a local file.
// It runs screencap on the device, pulls the file, and cleans up.
func (c *Client) TakeScreenshot(serial, destPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), adbLongTimeout)
	defer cancel()

	capCmd := exec.CommandContext(ctx, c.adbPath, "-s", serial, "shell", "screencap", "-p", "/sdcard/mirroid_screenshot.png")
	platform.HideConsole(capCmd)
	out, err := capCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("screencap failed: %s (%w)", string(out), err)
	}

	pullCmd := exec.CommandContext(ctx, c.adbPath, "-s", serial, "pull", "/sdcard/mirroid_screenshot.png", destPath)
	platform.HideConsole(pullCmd)
	pullOut, pullErr := pullCmd.CombinedOutput()

	// always attempt to remove the temp file from the device (fresh context
	// so cleanup runs even if the screencap/pull context expired)
	cleanCtx, cleanCancel := context.WithTimeout(context.Background(), adbTimeout)
	defer cleanCancel()
	cleanupCmd := exec.CommandContext(cleanCtx, c.adbPath, "-s", serial, "shell", "rm", "/sdcard/mirroid_screenshot.png")
	platform.HideConsole(cleanupCmd)
	if cleanupErr := cleanupCmd.Run(); cleanupErr != nil {
		log.Printf("warning: failed to remove device temp file /sdcard/mirroid_screenshot.png: %v", cleanupErr)
	}

	if pullErr != nil {
		return fmt.Errorf("pull failed: %s (%w)", string(pullOut), pullErr)
	}

	return nil
}
