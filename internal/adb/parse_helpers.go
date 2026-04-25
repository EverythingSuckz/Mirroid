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

func parseStorageLine(line string) (total, used, free string, pct float64) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 5 {
		return "-", "-", "-", 0.0
	}

	total = fields[1]
	used = fields[2]
	free = fields[3]

	pctStr := strings.TrimSuffix(fields[4], "%")
	pctVal, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		return total, used, free, 0.0
	}

	pct = pctVal / 100.0
	if pct < 0.0 {
		pct = 0.0
	}
	if pct > 1.0 {
		pct = 1.0
	}
	return total, used, free, pct
}

func parseMemTotal(memInfoOutput string) string {
	for _, line := range strings.Split(memInfoOutput, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
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

// parseCPUCores counts per-core entries in /proc/cpuinfo. Lines look like
// "processor\t: 0". On ARM the file also contains a header line
// "Processor\t: AArch64 Processor rev 4" — the leading capital distinguishes
// the header from the per-core entries, so a case-sensitive prefix match plus
// a numeric value check yields the correct count.
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

// findSSIDToken locates the start of an "SSID:" token in s without matching
// the trailing "SSID:" inside "BSSID:". Returns the index of the value
// (i.e. the character after "SSID: ") or -1 if not found.
func findSSIDToken(s string) int {
	rest := s
	offset := 0
	for {
		i := strings.Index(rest, "SSID:")
		if i < 0 {
			return -1
		}
		absolute := offset + i
		// Reject matches preceded by a letter/digit (e.g. the "B" in "BSSID:").
		if absolute > 0 {
			prev := s[absolute-1]
			if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') || (prev >= '0' && prev <= '9') {
				offset = absolute + len("SSID:")
				rest = s[offset:]
				continue
			}
		}
		valStart := absolute + len("SSID:")
		// Skip a single space if present (matches both "SSID: x" and "SSID:x").
		if valStart < len(s) && s[valStart] == ' ' {
			valStart++
		}
		return valStart
	}
}

func parseWifiSSID(dumpsysOutput string) string {
	for _, line := range strings.Split(dumpsysOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		valStart := findSSIDToken(trimmed)
		if valStart < 0 {
			continue
		}
		after := trimmed[valStart:]

		// Handle SSID: "name" (with quotes)
		if strings.HasPrefix(after, "\"") {
			end := strings.Index(after[1:], "\"")
			if end < 0 {
				continue
			}
			ssid := after[1 : end+1]
			if ssid == "" || ssid == "<unknown ssid>" {
				return "-"
			}
			return ssid
		}

		// Handle SSID: name (no quotes) — take until comma or end of line
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
	return "-"
}

// parseGetProps parses the output of `adb shell getprop` (no args). Each
// line has the form `[key]: [value]`; multi-line values (extremely rare for
// the keys we care about) are not handled — only the first line of such a
// value is captured.
func parseGetProps(output string) map[string]string {
	props := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Expected: [key]: [value]
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

// parseResolution extracts the device resolution from `wm size` output.
// When an override is set, the output is two lines:
//
//	Physical size: 1440x3120
//	Override size: 1080x1920
//
// The override is preferred when present.
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

func parseIPAddress(ipAddrOutput string) string {
	for _, line := range strings.Split(ipAddrOutput, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "inet ") {
			continue
		}
		parts := strings.Fields(trimmed)
		if len(parts) < 2 {
			continue
		}
		addr := parts[1]
		if slashIdx := strings.Index(addr, "/"); slashIdx >= 0 {
			addr = addr[:slashIdx]
		}
		return addr
	}
	return "-"
}
