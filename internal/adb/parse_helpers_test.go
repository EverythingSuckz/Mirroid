package adb

import (
	"math"
	"testing"
)

func TestParseBatteryStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2", "Charging"},
		{"3", "Discharging"},
		{"4", "Not charging"},
		{"5", "Full"},
		{"1", "Unknown"},
		{"", "Unknown"},
		{"garbage", "Unknown"},
		{"99", "Unknown"},
	}
	for _, tt := range tests {
		if got := parseBatteryStatus(tt.input); got != tt.want {
			t.Errorf("parseBatteryStatus(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseBatteryHealth(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1", "Unknown"},
		{"2", "Good"},
		{"3", "Overheat"},
		{"4", "Dead"},
		{"5", "Over voltage"},
		{"6", "Failure"},
		{"7", "Cold"},
		{"", "Unknown"},
		{"garbage", "Unknown"},
		{"99", "Unknown"},
	}
	for _, tt := range tests {
		if got := parseBatteryHealth(tt.input); got != tt.want {
			t.Errorf("parseBatteryHealth(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseBatteryTemp(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"312", "31.2°C"},
		{"0", "0.0°C"},
		{"250", "25.0°C"},
		{"", "-"},
		{"abc", "-"},
	}
	for _, tt := range tests {
		if got := parseBatteryTemp(tt.input); got != tt.want {
			t.Errorf("parseBatteryTemp(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseUptime(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{185432.12, "2d 3h 30m"},
		{3600, "1h 0m"},
		{45, "<1m"},
		{0, "<1m"},
		{-1, "<1m"},
		{86400, "1d 0h 0m"},
		{3661, "1h 1m"},
		{60, "1m"},
	}
	for _, tt := range tests {
		if got := parseUptime(tt.input); got != tt.want {
			t.Errorf("parseUptime(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseStorageLine(t *testing.T) {
	tests := []struct {
		input               string
		wantTotal           string
		wantUsed            string
		wantFree            string
		wantPctApprox       float64
		wantPctApproxMargin float64
	}{
		// df -k output: 62914556 1k-blocks ≈ 60.0G
		{
			"/dev/block/dm-48    62914556    39061408    23853148  62% /data",
			"60.0G", "37.3G", "22.7G", 0.62, 0.01,
		},
		{"", "-", "-", "-", 0.0, 0.0},
		{"garbage", "-", "-", "-", 0.0, 0.0},
		{"short line", "-", "-", "-", 0.0, 0.0},
		// non-numeric size column rejects (would otherwise pass through as bogus strings)
		{"/dev/block/x abc def ghi 50% /data", "-", "-", "-", 0.0, 0.0},
		// zero total rejects
		{"/dev/block/x 0 0 0 0% /data", "-", "-", "-", 0.0, 0.0},
	}
	for _, tt := range tests {
		total, used, free, pct := parseStorageLine(tt.input)
		if total != tt.wantTotal || used != tt.wantUsed || free != tt.wantFree {
			t.Errorf("parseStorageLine(%q) = (%q, %q, %q, _), want (%q, %q, %q, _)",
				tt.input, total, used, free, tt.wantTotal, tt.wantUsed, tt.wantFree)
		}
		if math.Abs(pct-tt.wantPctApprox) > tt.wantPctApproxMargin {
			t.Errorf("parseStorageLine(%q) pct = %v, want ~%v", tt.input, pct, tt.wantPctApprox)
		}
	}
}

func TestParseMemTotal(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MemTotal:        3905424 kB\nMemFree:         1234567 kB", "3.7 GB"},
		{"", "-"},
		{"MemTotal:", "-"},
		{"SomethingElse: 12345 kB", "-"},
		{"MemTotal:        notanumber kB", "-"},
		// missing kB unit must reject (would otherwise be silently 1024x wrong)
		{"MemTotal:        3905424", "-"},
		// MB unit must reject
		{"MemTotal:        3905424 MB", "-"},
	}
	for _, tt := range tests {
		if got := parseMemTotal(tt.input); got != tt.want {
			t.Errorf("parseMemTotal(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseCPUCores(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"two cores", "processor\t: 0\nBogoMIPS\t: 52.00\nprocessor\t: 1\n", "2"},
		// ARM header line "Processor : AArch64 ..." must NOT be counted —
		// the device has 2 cores, not 3.
		{"arm header excluded", "Processor\t: AArch64 Processor rev 4 (aarch64)\nprocessor\t: 0\nprocessor\t: 1\n", "2"},
		{"empty", "", "-"},
		{"no matches", "no processor lines here", "-"},
		// Non-numeric value should be ignored.
		{"non-numeric value", "processor\t: foo\nprocessor\t: 0\n", "1"},
		{"eight cores", "processor\t: 0\nprocessor\t: 1\nprocessor\t: 2\nprocessor\t: 3\nprocessor\t: 4\nprocessor\t: 5\nprocessor\t: 6\nprocessor\t: 7\n", "8"},
	}
	for _, tt := range tests {
		if got := parseCPUCores(tt.input); got != tt.want {
			t.Errorf("parseCPUCores(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseWifiSSID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`SSID: "MyNetwork", BSSID: aa:bb`, "MyNetwork"},
		{"some line\n    SSID: \"HomeWifi\", security: WPA", "HomeWifi"},
		{`SSID: <unknown ssid>`, "-"},
		{`SSID: "<unknown ssid>"`, "-"},
		{"", "-"},
		{"no ssid here", "-"},
		{`SSID: OpenNet, BSSID: cc:dd`, "OpenNet"},
		// BSSID-only line must not be mis-parsed as SSID.
		{"   BSSID: 00:11:22:33:44:55, RSSI: -50", "-"},
		// BSSID first, real SSID later in the same line — must pick SSID.
		{`BSSID: aa:bb:cc:dd:ee:ff SSID: "RealNet"`, "RealNet"},
		// BSSID line followed by SSID line — must pick SSID line.
		{"BSSID: aa:bb:cc:dd:ee:ff\nSSID: \"NextNet\"", "NextNet"},
		// mWifiInfo wins over an earlier configured network entry.
		{
			"Configured networks:\n  SSID: \"OldSaved\"\nmWifiInfo SSID: \"Active\", BSSID: aa:bb",
			"Active",
		},
	}
	for _, tt := range tests {
		if got := parseWifiSSID(tt.input); got != tt.want {
			t.Errorf("parseWifiSSID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseGetProps(t *testing.T) {
	in := "[ro.product.model]: [Pixel 7]\n" +
		"[ro.build.version.release]: [13]\n" +
		"[ro.build.version.sdk]: [33]\n" +
		"\n" +
		"[ro.serialno]: [ABC123]\n" +
		"garbage line\n" +
		"[malformed.no.value]\n" +
		"[empty.value]: []\n"
	got := parseGetProps(in)
	wants := map[string]string{
		"ro.product.model":          "Pixel 7",
		"ro.build.version.release":  "13",
		"ro.build.version.sdk":      "33",
		"ro.serialno":               "ABC123",
		"empty.value":               "",
	}
	for k, v := range wants {
		if got[k] != v {
			t.Errorf("parseGetProps[%q] = %q, want %q", k, got[k], v)
		}
	}
	if _, exists := got["malformed.no.value"]; exists {
		t.Errorf("expected malformed.no.value to be skipped, but it was parsed")
	}
	if got["nonexistent"] != "" {
		t.Errorf("expected zero value for missing key, got %q", got["nonexistent"])
	}
}

func TestParseResolution(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"physical only", "Physical size: 1440x3120", "1440x3120"},
		{"override preferred", "Physical size: 1440x3120\nOverride size: 1080x1920", "1080x1920"},
		{"override no physical", "Override size: 1080x1920", "1080x1920"},
		{"trailing newline", "Physical size: 1080x2400\n", "1080x2400"},
		{"empty", "", "-"},
		{"garbage", "no colon here", "-"},
		{"empty value", "Physical size:", "-"},
		{"unknown keys", "Display size: 1080x1920", "-"},
	}
	for _, tt := range tests {
		if got := parseResolution(tt.input); got != tt.want {
			t.Errorf("parseResolution(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestParseIPAddress(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"multi-line wlan0", "3: wlan0: ...\n    inet 192.168.2.48/24 brd ... wlan0", "192.168.2.48"},
		{"oneline wlan0", "3: wlan0    inet 192.168.2.48/24 brd 192.168.2.255 scope global wlan0", "192.168.2.48"},
		{"empty", "", "-"},
		{"no inet", "no inet line", "-"},
		{"v6 only", "inet6 ::1/128 scope host", "-"},
		{"loopback skipped", "1: lo    inet 127.0.0.1/8 scope host lo", "-"},
		{"link-local skipped", "5: wlan0    inet 169.254.1.2/16 scope link wlan0", "-"},
		// wlan0 preferred over wlan1
		{
			"wlan0 over wlan1",
			"3: wlan1    inet 10.0.0.2/24 scope global wlan1\n4: wlan0    inet 10.0.0.1/24 scope global wlan0",
			"10.0.0.1",
		},
	}
	for _, tt := range tests {
		if got := parseIPAddress(tt.input); got != tt.want {
			t.Errorf("parseIPAddress(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
