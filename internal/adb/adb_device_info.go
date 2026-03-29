package adb

import (
	"os/exec"
	"strings"

	"mirroid/internal/platform"
)

// DeviceInfo holds detailed properties fetched from a connected device.
type DeviceInfo struct {
	Model          string
	Manufacturer   string
	AndroidVersion string
	SDK            string
	BuildID        string
	Serial         string
	DeviceID       string
	Resolution     string
	Density        string
	Battery        string
}

// GetDeviceInfo fetches detailed device properties via adb shell getprop,
// dumpsys battery, and wm size.
func (c *Client) GetDeviceInfo(serial string) DeviceInfo {
	getProp := func(prop string) string {
		cmd := exec.Command(c.adbPath, "-s", serial, "shell", "getprop", prop)
		platform.HideConsole(cmd)
		out, err := cmd.Output()
		if err != nil {
			return "-"
		}
		return strings.TrimSpace(string(out))
	}

	battCmd := exec.Command(c.adbPath, "-s", serial, "shell", "dumpsys", "battery")
	platform.HideConsole(battCmd)
	battOut, _ := battCmd.Output()
	battery := "-"
	for _, line := range strings.Split(string(battOut), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "level:") {
			battery = strings.TrimPrefix(line, "level:") + "%"
			battery = strings.TrimSpace(battery)
			break
		}
	}

	resCmd := exec.Command(c.adbPath, "-s", serial, "shell", "wm", "size")
	platform.HideConsole(resCmd)
	resOut, _ := resCmd.Output()
	resolution := "-"
	resParts := strings.Split(strings.TrimSpace(string(resOut)), ":")
	if len(resParts) == 2 {
		resolution = strings.TrimSpace(resParts[1])
	}

	return DeviceInfo{
		Model:          getProp("ro.product.model"),
		Manufacturer:   getProp("ro.product.manufacturer"),
		AndroidVersion: getProp("ro.build.version.release"),
		SDK:            getProp("ro.build.version.sdk"),
		BuildID:        getProp("ro.build.display.id"),
		Serial:         serial,
		DeviceID:       getProp("ro.serialno"),
		Resolution:     resolution,
		Density:        getProp("ro.sf.lcd_density"),
		Battery:        battery,
	}
}

// GetDeviceID returns the hardware serial (ro.serialno) for a connected device.
func (c *Client) GetDeviceID(serial string) string {
	cmd := exec.Command(c.adbPath, "-s", serial, "shell", "getprop", "ro.serialno")
	platform.HideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
