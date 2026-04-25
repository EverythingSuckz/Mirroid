package adb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func loadFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("loadFixture %s: %v", name, err)
	}
	return string(b)
}

func TestFixtureDumpsysWifi(t *testing.T) {
	out := loadFixture(t, "dumpsys_wifi_pixel.txt")
	got := parseWifiSSID(out)
	// must pick the active network from mWifiInfo, not "OldSavedNet" from configured networks
	if got != "MyHomeNet" {
		t.Errorf("parseWifiSSID = %q, want %q", got, "MyHomeNet")
	}
}

func TestFixtureDumpsysBattery(t *testing.T) {
	out := loadFixture(t, "dumpsys_battery.txt")
	level, status, temp, health := parseDumpsysBattery(out)
	if level != 73 {
		t.Errorf("level = %d, want 73", level)
	}
	if got := parseBatteryStatus(status); got != "Charging" {
		t.Errorf("battery status = %q, want %q", got, "Charging")
	}
	if got := parseBatteryHealth(health); got != "Good" {
		t.Errorf("battery health = %q, want %q", got, "Good")
	}
	if got := parseBatteryTemp(temp); got != "31.2°C" {
		t.Errorf("battery temp = %q, want %q", got, "31.2°C")
	}
}

func TestFixtureDfK(t *testing.T) {
	out := loadFixture(t, "df_k_data.txt")
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected >=2 lines, got %d", len(lines))
	}
	total, used, free, pct := parseStorageLine(lines[1])
	if total != "60.0 GB" {
		t.Errorf("total = %q, want %q", total, "60.0 GB")
	}
	if used != "37.3 GB" {
		t.Errorf("used = %q, want %q", used, "37.3 GB")
	}
	if free != "22.7 GB" {
		t.Errorf("free = %q, want %q", free, "22.7 GB")
	}
	if pct < 0.61 || pct > 0.63 {
		t.Errorf("pct = %v, want ~0.62", pct)
	}
}

func TestFixtureMeminfo(t *testing.T) {
	out := loadFixture(t, "meminfo.txt")
	if got := parseMemTotal(out); got != "7.5 GB" {
		t.Errorf("parseMemTotal = %q, want %q", got, "7.5 GB")
	}
}

func TestFixtureCpuinfoArm(t *testing.T) {
	out := loadFixture(t, "cpuinfo_arm.txt")
	// 8 per-core "processor" entries; the ARM "Processor" header line must NOT count
	if got := parseCPUCores(out); got != "8" {
		t.Errorf("parseCPUCores = %q, want %q", got, "8")
	}
}

func TestFixtureWmSizeOverride(t *testing.T) {
	out := loadFixture(t, "wm_size_override.txt")
	// override resolution must win over physical
	if got := parseResolution(out); got != "1080x2340" {
		t.Errorf("parseResolution = %q, want %q", got, "1080x2340")
	}
}

func TestFixtureIpAddrShow(t *testing.T) {
	out := loadFixture(t, "ip_addr_show.txt")
	// must skip loopback (127.x) and link-local (169.254.x), pick wlan0
	if got := parseIPAddress(out); got != "192.168.1.42" {
		t.Errorf("parseIPAddress = %q, want %q", got, "192.168.1.42")
	}
}

func TestFixtureGetprop(t *testing.T) {
	out := loadFixture(t, "getprop.txt")
	props := parseGetProps(out)
	wants := map[string]string{
		"ro.build.version.release": "13",
		"ro.product.model":         "Pixel 7",
		"ro.product.manufacturer":  "Google",
		"ro.sf.lcd_density":        "420",
		"dhcp.wlan0.ipaddress":     "192.168.1.42",
	}
	for k, v := range wants {
		if props[k] != v {
			t.Errorf("props[%q] = %q, want %q", k, props[k], v)
		}
	}
}
