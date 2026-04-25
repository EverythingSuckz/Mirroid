package adb

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"

	"mirroid/internal/platform"
)

// fieldUnknown is the sentinel string used in DeviceInfo display fields when
// a value could not be obtained.
const fieldUnknown = "-"

// DeviceInfo holds detailed properties fetched from a connected device.
// Display fields use fieldUnknown ("-") when a value is unavailable.
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

	// BatteryPct is the battery level in the range [0,1]. Zero when unknown.
	BatteryPct    float64
	BatteryStatus string
	BatteryTemp   string
	BatteryHealth string

	StorageTotal string
	StorageUsed  string
	StorageFree  string
	// StoragePct is used storage as a fraction in [0,1]. Zero when unknown.
	StoragePct     float64
	StorageDisplay string

	RAM         string
	CPUPlatform string
	CPUCores    string

	WifiSSID  string
	IPAddress string

	Uptime         string
	AppCount       string
	AndroidDisplay string
	DensityDisplay string
}

// shellOutput runs `adb -s <serial> shell <args...>` under ctx and returns
// the captured stdout. Stderr is silently dropped to avoid noisy output from
// commands that exit non-zero on some devices.
func (c *Client) shellOutput(ctx context.Context, serial string, args ...string) ([]byte, error) {
	full := append([]string{"-s", serial, "shell"}, args...)
	cmd := exec.CommandContext(ctx, c.adbPath, full...)
	platform.HideConsole(cmd)
	return cmd.Output()
}

// logShellErr logs a per-field shell failure with context. Cancellation
// errors are intentionally suppressed — they're not informative when the
// caller has explicitly cancelled.
func logShellErr(field string, err error) {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	log.Printf("device_info: %s: %v", field, err)
}

// GetDeviceInfo fetches detailed device properties via parallel adb shell
// invocations. Returns an error only when the anchor `getprop` call fails
// (which indicates the device is offline, unauthorized, or disconnected).
// Per-field failures are logged but do not fail the whole call; affected
// fields fall back to fieldUnknown.
func (c *Client) GetDeviceInfo(ctx context.Context, serial string) (DeviceInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, adbLongTimeout)
	defer cancel()

	var (
		propMap  map[string]string
		propsErr error

		battOut, resOut, dfOut, memOut, cpuOut []byte
		wifiOut, ipOut, upOut, appOut          []byte
	)

	g, gctx := errgroup.WithContext(ctx)

	// Anchor: full getprop dump. If this fails, the device is unreachable.
	g.Go(func() error {
		out, err := c.shellOutput(gctx, serial, "getprop")
		if err != nil {
			propsErr = err
			return nil
		}
		propMap = parseGetProps(string(out))
		return nil
	})

	g.Go(func() error {
		out, err := c.shellOutput(gctx, serial, "dumpsys", "battery")
		logShellErr("dumpsys battery", err)
		battOut = out
		return nil
	})
	g.Go(func() error {
		out, err := c.shellOutput(gctx, serial, "wm", "size")
		logShellErr("wm size", err)
		resOut = out
		return nil
	})
	g.Go(func() error {
		out, err := c.shellOutput(gctx, serial, "df", "-h", "/data")
		logShellErr("df -h /data", err)
		dfOut = out
		return nil
	})
	g.Go(func() error {
		out, err := c.shellOutput(gctx, serial, "cat", "/proc/meminfo")
		logShellErr("cat /proc/meminfo", err)
		memOut = out
		return nil
	})
	g.Go(func() error {
		out, err := c.shellOutput(gctx, serial, "cat", "/proc/cpuinfo")
		logShellErr("cat /proc/cpuinfo", err)
		cpuOut = out
		return nil
	})
	g.Go(func() error {
		out, err := c.shellOutput(gctx, serial, "dumpsys", "wifi")
		logShellErr("dumpsys wifi", err)
		wifiOut = out
		return nil
	})
	g.Go(func() error {
		out, err := c.shellOutput(gctx, serial, "ip", "addr", "show", "wlan0")
		logShellErr("ip addr show wlan0", err)
		ipOut = out
		return nil
	})
	g.Go(func() error {
		out, err := c.shellOutput(gctx, serial, "cat", "/proc/uptime")
		logShellErr("cat /proc/uptime", err)
		upOut = out
		return nil
	})
	g.Go(func() error {
		out, err := c.shellOutput(gctx, serial, "pm", "list", "packages")
		logShellErr("pm list packages", err)
		appOut = out
		return nil
	})

	// errgroup.Go callbacks never return errors here, so Wait cannot fail
	// for any reason other than ctx cancellation — handled below.
	_ = g.Wait()

	if err := ctx.Err(); err != nil {
		return DeviceInfo{Serial: serial}, fmt.Errorf("device info cancelled: %w", err)
	}
	if propsErr != nil {
		return DeviceInfo{Serial: serial}, fmt.Errorf("getprop failed (device offline?): %w", propsErr)
	}

	// Battery fields
	battery := fieldUnknown
	var batteryStatus, batteryTemp, batteryHealth string
	for _, line := range strings.Split(string(battOut), "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch key {
		case "level":
			battery = val + "%"
		case "status":
			batteryStatus = val
		case "temperature":
			batteryTemp = val
		case "health":
			batteryHealth = val
		}
	}

	// Storage
	storageTotal, storageUsed, storageFree, storagePct := fieldUnknown, fieldUnknown, fieldUnknown, 0.0
	dfLines := strings.Split(strings.TrimSpace(string(dfOut)), "\n")
	if len(dfLines) >= 2 {
		storageTotal, storageUsed, storageFree, storagePct = parseStorageLine(dfLines[1])
	}

	// Uptime
	uptime := fieldUnknown
	if fields := strings.Fields(strings.TrimSpace(string(upOut))); len(fields) >= 1 {
		if secs, err := strconv.ParseFloat(fields[0], 64); err == nil {
			uptime = parseUptime(secs)
		}
	}

	// App count: filter to "package:" prefix to avoid counting noise lines.
	appCount := fieldUnknown
	if len(appOut) > 0 {
		count := 0
		for _, line := range strings.Split(string(appOut), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "package:") {
				count++
			}
		}
		appCount = strconv.Itoa(count)
	}

	prop := func(key string) string {
		if v, ok := propMap[key]; ok && v != "" {
			return v
		}
		return fieldUnknown
	}

	androidVersion := prop("ro.build.version.release")
	sdk := prop("ro.build.version.sdk")
	density := prop("ro.sf.lcd_density")

	storageDisplay := fieldUnknown
	if storageUsed != fieldUnknown && storageTotal != fieldUnknown {
		storageDisplay = storageUsed + " / " + storageTotal
	}

	androidDisplay := androidVersion
	if androidVersion != fieldUnknown && sdk != fieldUnknown {
		androidDisplay = androidVersion + " (SDK " + sdk + ")"
	}

	densityDisplay := density
	if density != fieldUnknown {
		densityDisplay = density + " dpi"
	}

	return DeviceInfo{
		Model:          prop("ro.product.model"),
		Manufacturer:   prop("ro.product.manufacturer"),
		AndroidVersion: androidVersion,
		SDK:            sdk,
		BuildID:        prop("ro.build.display.id"),
		Serial:         serial,
		DeviceID:       prop("ro.serialno"),
		Resolution:     parseResolution(string(resOut)),
		Density:        density,
		Battery:        battery,
		BatteryPct:     parseBatteryPct(battery),
		BatteryStatus:  parseBatteryStatus(batteryStatus),
		BatteryTemp:    parseBatteryTemp(batteryTemp),
		BatteryHealth:  parseBatteryHealth(batteryHealth),
		StorageTotal:   storageTotal,
		StorageUsed:    storageUsed,
		StorageFree:    storageFree,
		StoragePct:     storagePct,
		StorageDisplay: storageDisplay,
		RAM:            parseMemTotal(string(memOut)),
		CPUPlatform:    prop("ro.board.platform"),
		CPUCores:       parseCPUCores(string(cpuOut)),
		WifiSSID:       parseWifiSSID(string(wifiOut)),
		IPAddress:      parseIPAddress(string(ipOut)),
		Uptime:         uptime,
		AppCount:       appCount,
		AndroidDisplay: androidDisplay,
		DensityDisplay: densityDisplay,
	}, nil
}

// GetDeviceID returns the hardware serial (ro.serialno) for a connected device.
func (c *Client) GetDeviceID(serial string) string {
	ctx, cancel := context.WithTimeout(context.Background(), adbTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.adbPath, "-s", serial, "shell", "getprop", "ro.serialno")
	platform.HideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
