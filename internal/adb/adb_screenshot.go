package adb

import (
	"fmt"
	"log"
	"os/exec"

	"mirroid/internal/platform"
)

// TakeScreenshot captures the device screen to a local file.
// It runs screencap on the device, pulls the file, and cleans up.
func (c *Client) TakeScreenshot(serial, destPath string) error {
	capCmd := exec.Command(c.adbPath, "-s", serial, "shell", "screencap", "-p", "/sdcard/mirroid_screenshot.png")
	platform.HideConsole(capCmd)
	out, err := capCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("screencap failed: %s (%w)", string(out), err)
	}

	pullCmd := exec.Command(c.adbPath, "-s", serial, "pull", "/sdcard/mirroid_screenshot.png", destPath)
	platform.HideConsole(pullCmd)
	pullOut, pullErr := pullCmd.CombinedOutput()

	// always attempt to remove the temp file from the device
	cleanupCmd := exec.Command(c.adbPath, "-s", serial, "shell", "rm", "/sdcard/mirroid_screenshot.png")
	platform.HideConsole(cleanupCmd)
	if cleanupErr := cleanupCmd.Run(); cleanupErr != nil {
		log.Printf("warning: failed to remove device temp file /sdcard/mirroid_screenshot.png: %v", cleanupErr)
	}

	if pullErr != nil {
		return fmt.Errorf("pull failed: %s (%w)", string(pullOut), pullErr)
	}

	return nil
}
