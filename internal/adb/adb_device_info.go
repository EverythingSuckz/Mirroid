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

const fieldUnknown = "-"

// DeviceInfo holds detailed properties for a connected device. Display fields use "-" when unavailable.
type DeviceInfo struct {
	Model          string
	Manufacturer   string
	AndroidVersion string
	SDK            string
	BuildID        string
	Serial         string
	DeviceID       string
	Resolution     string

	BatteryPct    float64 // [0,1] when known, negative when unknown

	BatteryStatus BatteryStatus
	BatteryTemp   string
	BatteryHealth BatteryHealth

	StorageTotal   string
	StorageUsed    string
	StorageFree    string
	StoragePct     float64 // used fraction [0,1], zero when unknown
	StorageDisplay string

	RAM         string
	CPUPlatform string
	CPUCores    int // -1 when unknown

	WifiSSID  string
	IPAddress string

	Uptime         string
	AppCount       int // -1 when unknown
	AndroidDisplay string
	DensityDisplay string
}

func (c *Client) shellOutput(ctx context.Context, serial string, args ...string) ([]byte, error) {
	full := append([]string{"-s", serial, "shell"}, args...)
	cmd := exec.CommandContext(ctx, c.adbPath, full...)
	platform.HideConsole(cmd)
	return cmd.Output()
}

func logShellErr(field string, err error) {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return
	}
	log.Printf("device_info: %s: %v", field, err)
}

// GetDeviceInfo fans out adb queries in parallel. Returns an error only when the anchor getprop fails (device offline/unauthorized); per-field failures are logged and fall back to "-".
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

	// anchor: getprop failure means device is unreachable. propagate err to
	// short-circuit the other goroutines via gctx cancellation.
	g.Go(func() error {
		out, err := c.shellOutput(gctx, serial, "getprop")
		if err != nil {
			propsErr = err
			return err
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
		// raw 1k-blocks; format ourselves to avoid toybox/busybox column drift
		out, err := c.shellOutput(gctx, serial, "df", "-k", "/data")
		logShellErr("df -k /data", err)
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
		out, err := c.shellOutput(gctx, serial, "ip", "-4", "-o", "addr", "show")
		logShellErr("ip -4 -o addr show", err)
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

	_ = g.Wait()

	if err := ctx.Err(); err != nil {
		return DeviceInfo{Serial: serial}, fmt.Errorf("device info cancelled: %w", err)
	}
	if propsErr != nil {
		return DeviceInfo{Serial: serial}, fmt.Errorf("getprop failed (device offline?): %w", propsErr)
	}

	batteryLevel, batteryStatus, batteryTemp, batteryHealth := parseDumpsysBattery(string(battOut))
	batteryPct := -1.0 // unknown sentinel; preserves the 0% vs. unknown distinction
	if batteryLevel >= 0 {
		batteryPct = float64(batteryLevel) / 100.0
		if batteryPct > 1.0 {
			batteryPct = 1.0
		}
	}

	storageTotal, storageUsed, storageFree, storagePct := fieldUnknown, fieldUnknown, fieldUnknown, 0.0
	dfLines := strings.Split(strings.TrimSpace(string(dfOut)), "\n")
	if len(dfLines) >= 2 {
		storageTotal, storageUsed, storageFree, storagePct = parseStorageLine(dfLines[1])
	}

	uptime := fieldUnknown
	if fields := strings.Fields(strings.TrimSpace(string(upOut))); len(fields) >= 1 {
		if secs, err := strconv.ParseFloat(fields[0], 64); err == nil {
			uptime = parseUptime(secs)
		}
	}

	appCount := -1
	if len(appOut) > 0 {
		count := 0
		for _, line := range strings.Split(string(appOut), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "package:") {
				count++
			}
		}
		appCount = count
	}

	prop := func(key string) string {
		if v, ok := propMap[key]; ok && v != "" {
			return v
		}
		return fieldUnknown
	}

	androidVersion := prop("ro.build.version.release")
	sdk := prop("ro.build.version.sdk")

	storageDisplay := fieldUnknown
	if storageUsed != fieldUnknown && storageTotal != fieldUnknown {
		storageDisplay = storageUsed + " / " + storageTotal
	}

	androidDisplay := androidVersion
	if androidVersion != fieldUnknown && sdk != fieldUnknown {
		androidDisplay = androidVersion + " (SDK " + sdk + ")"
	}

	densityDisplay := prop("ro.sf.lcd_density")
	if densityDisplay == fieldUnknown {
		densityDisplay = prop("ro.lcd_density")
	}
	if densityDisplay != fieldUnknown {
		densityDisplay += " dpi"
	}

	ipAddress := parseIPAddress(string(ipOut))
	if ipAddress == fieldUnknown {
		if v := prop("dhcp.wlan0.ipaddress"); v != fieldUnknown {
			ipAddress = v
		}
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
		BatteryPct:     batteryPct,
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
		IPAddress:      ipAddress,
		Uptime:         uptime,
		AppCount:       appCount,
		AndroidDisplay: androidDisplay,
		DensityDisplay: densityDisplay,
	}, nil
}

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
