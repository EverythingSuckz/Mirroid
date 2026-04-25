package adb

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func parseBatteryStatus(code string) string {
	switch code {
	case "1":
		return "Unknown"
	case "2":
		return "Charging"
	case "3":
		return "Discharging"
	case "4":
		return "Not charging"
	case "5":
		return "Full"
	default:
		return "Unknown"
	}
}

func parseBatteryHealth(code string) string {
	switch code {
	case "1":
		return "Unknown"
	case "2":
		return "Good"
	case "3":
		return "Overheat"
	case "4":
		return "Dead"
	case "5":
		return "Over voltage"
	case "6":
		return "Failure"
	case "7":
		return "Cold"
	default:
		return "Unknown"
	}
}

func parseBatteryPct(battery string) float64 {
	s := strings.TrimSuffix(strings.TrimSpace(battery), "%")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0.0
	}
	pct := v / 100.0
	if pct > 1.0 {
		pct = 1.0
	}
	return pct
}

func parseBatteryTemp(raw string) string {
	val, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return "-"
	}
	celsius := float64(val) / 10.0
	return fmt.Sprintf("%.1f°C", celsius)
}

func parseUptime(seconds float64) string {
	if seconds < 0 {
		return "-"
	}
	total := int(math.Floor(seconds))
	if total < 60 {
		return "<1m"
	}

	days := total / 86400
	remaining := total % 86400
	hours := remaining / 3600
	remaining = remaining % 3600
	minutes := remaining / 60

	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if days > 0 || hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	parts = append(parts, fmt.Sprintf("%dm", minutes))

	return strings.Join(parts, " ")
}

// parses a `df -k` data line: filesystem 1K-blocks used available use% mountpoint.
// values are 1k-blocks; converted to human-readable strings here.
func parseStorageLine(line string) (total, used, free string, pct float64) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 5 {
		return "-", "-", "-", 0.0
	}

	totalKB, err1 := strconv.ParseInt(fields[1], 10, 64)
	usedKB, err2 := strconv.ParseInt(fields[2], 10, 64)
	freeKB, err3 := strconv.ParseInt(fields[3], 10, 64)
	if err1 != nil || err2 != nil || err3 != nil || totalKB <= 0 {
		return "-", "-", "-", 0.0
	}

	pctStr := strings.TrimSuffix(fields[4], "%")
	pctVal, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		return "-", "-", "-", 0.0
	}

	pct = pctVal / 100.0
	if pct < 0.0 {
		pct = 0.0
	}
	if pct > 1.0 {
		pct = 1.0
	}
	return formatKB(totalKB), formatKB(usedKB), formatKB(freeKB), pct
}

func formatKB(kb int64) string {
	switch {
	case kb >= 1<<30:
		return fmt.Sprintf("%.1fT", float64(kb)/(1<<30))
	case kb >= 1<<20:
		return fmt.Sprintf("%.1fG", float64(kb)/(1<<20))
	case kb >= 1<<10:
		return fmt.Sprintf("%.0fM", float64(kb)/(1<<10))
	default:
		return fmt.Sprintf("%dK", kb)
	}
}

func parseMemTotal(memInfoOutput string) string {
	for _, line := range strings.Split(memInfoOutput, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 || parts[2] != "kB" {
			return "-"
		}
		kb, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return "-"
		}
		gb := kb / 1024.0 / 1024.0
		return fmt.Sprintf("%.1f GB", gb)
	}
	return "-"
}

// case-sensitive "processor" prefix + numeric value excludes the ARM header line "Processor : AArch64 ...".
func parseCPUCores(cpuInfoOutput string) string {
	count := 0
	for _, line := range strings.Split(cpuInfoOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "processor") {
			continue
		}
		_, val, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		if _, err := strconv.Atoi(strings.TrimSpace(val)); err != nil {
			continue
		}
		count++
	}
	if count == 0 {
		return "-"
	}
	return strconv.Itoa(count)
}

// returns the index of the value after "SSID:" in s, rejecting matches inside "BSSID:". -1 if not found.
func findSSIDToken(s string) int {
	rest := s
	offset := 0
	for {
		i := strings.Index(rest, "SSID:")
		if i < 0 {
			return -1
		}
		absolute := offset + i
		if absolute > 0 {
			prev := s[absolute-1]
			if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') || (prev >= '0' && prev <= '9') {
				offset = absolute + len("SSID:")
				rest = s[offset:]
				continue
			}
		}
		valStart := absolute + len("SSID:")
		if valStart < len(s) && s[valStart] == ' ' {
			valStart++
		}
		return valStart
	}
}

func parseWifiSSID(dumpsysOutput string) string {
	lines := strings.Split(dumpsysOutput, "\n")
	// prefer ssid from the active connection (mWifiInfo) when present
	for _, line := range lines {
		if !strings.Contains(line, "mWifiInfo") {
			continue
		}
		if v := extractSSID(strings.TrimSpace(line)); v != "" {
			return v
		}
	}
	for _, line := range lines {
		if v := extractSSID(strings.TrimSpace(line)); v != "" {
			return v
		}
	}
	return "-"
}

// extracts the ssid from a single line; "" means no usable ssid found.
func extractSSID(trimmed string) string {
	valStart := findSSIDToken(trimmed)
	if valStart < 0 {
		return ""
	}
	after := trimmed[valStart:]

	if strings.HasPrefix(after, "\"") {
		end := strings.Index(after[1:], "\"")
		if end < 0 {
			return ""
		}
		ssid := after[1 : end+1]
		if ssid == "" || ssid == "<unknown ssid>" {
			return "-"
		}
		return ssid
	}

	val := after
	if ci := strings.Index(val, ","); ci >= 0 {
		val = val[:ci]
	}
	val = strings.TrimSpace(val)
	if val == "" || val == "<unknown ssid>" {
		return "-"
	}
	return val
}

// parses lines of the form `[key]: [value]` from `adb shell getprop`.
func parseGetProps(output string) map[string]string {
	props := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "[") {
			continue
		}
		keyEnd := strings.Index(line, "]")
		if keyEnd < 2 {
			continue
		}
		key := line[1:keyEnd]
		rest := line[keyEnd+1:]
		colonIdx := strings.Index(rest, ":")
		if colonIdx < 0 {
			continue
		}
		valPart := strings.TrimSpace(rest[colonIdx+1:])
		if !strings.HasPrefix(valPart, "[") || !strings.HasSuffix(valPart, "]") {
			continue
		}
		value := valPart[1 : len(valPart)-1]
		props[key] = value
	}
	return props
}

// prefers "Override size:" over "Physical size:" when both are present.
func parseResolution(wmSizeOutput string) string {
	var physical, override string
	for _, line := range strings.Split(wmSizeOutput, "\n") {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}
		switch key {
		case "Physical size":
			physical = val
		case "Override size":
			override = val
		}
	}
	if override != "" {
		return override
	}
	if physical != "" {
		return physical
	}
	return "-"
}

// handles both `ip addr show <iface>` (multi-line) and `ip -o addr show` (one line per addr).
// prefers wlan0 > wlan1 > any other wlanN; ignores loopback and link-local.
func parseIPAddress(ipAddrOutput string) string {
	var wlan0, wlan1, otherWlan string
	for _, line := range strings.Split(ipAddrOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		// look for "inet 1.2.3.4/24" anywhere in the line
		i := strings.Index(trimmed, "inet ")
		if i < 0 {
			continue
		}
		rest := strings.Fields(trimmed[i+len("inet "):])
		if len(rest) == 0 {
			continue
		}
		addr := rest[0]
		if slashIdx := strings.Index(addr, "/"); slashIdx >= 0 {
			addr = addr[:slashIdx]
		}
		if addr == "" || strings.HasPrefix(addr, "127.") || strings.HasPrefix(addr, "169.254.") {
			continue
		}
		switch {
		case strings.Contains(trimmed, "wlan0"):
			if wlan0 == "" {
				wlan0 = addr
			}
		case strings.Contains(trimmed, "wlan1"):
			if wlan1 == "" {
				wlan1 = addr
			}
		case strings.Contains(trimmed, "wlan"):
			if otherWlan == "" {
				otherWlan = addr
			}
		}
	}
	switch {
	case wlan0 != "":
		return wlan0
	case wlan1 != "":
		return wlan1
	case otherWlan != "":
		return otherWlan
	}
	return "-"
}
