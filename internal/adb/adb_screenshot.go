package adb

import (
	"fmt"
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
	pullOut, err := pullCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pull failed: %s (%w)", string(pullOut), err)
	}

	cleanCmd := exec.Command(c.adbPath, "-s", serial, "shell", "rm", "/sdcard/mirroid_screenshot.png")
	platform.HideConsole(cleanCmd)
	_ = cleanCmd.Run()

	return nil
}
