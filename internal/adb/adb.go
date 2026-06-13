package adb

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"mirroid/internal/model"
	"mirroid/internal/platform"
)

// ErrAlreadyConnected: adb found an existing transport for the address and
// skipped the TLS handshake; the transport may be a dead "offline" one.
var ErrAlreadyConnected = errors.New("already connected")

const (
	adbTimeout     = 10 * time.Second // quick queries (devices, connect, disconnect, getprop)
	adbLongTimeout = 30 * time.Second // slow operations (pair, screenshot, device info)
)

// Device represents a connected Android device.
type Device struct {
	Serial       string `json:"serial"`
	Model        string `json:"model"`
	Manufacturer string `json:"manufacturer,omitempty"`
	Source       string `json:"source"` // "usb", "wireless", "mdns"
	// last seen ip of a wireless device; survives the serial being
	// rewritten to an mdns instance name, so host matching keeps working
	Host string `json:"host,omitempty"`
}

// String returns a display-friendly label.
func (d Device) String() string {
	if d.Model != "" {
		return fmt.Sprintf("%s (%s)", d.Model, d.Serial)
	}
	return d.Serial
}

// isIPPort returns true if serial looks like a real IP:port address
// (as opposed to mDNS service names like "adb-xxx._adb-tls-connect._tcp").
func isIPPort(serial string) bool {
	host, _, err := net.SplitHostPort(serial)
	if err != nil {
		return false
	}
	return net.ParseIP(host) != nil
}

// IsInstanceSerial reports whether serial is an mdns instance name
// ("adb-XXXX._adb-tls-connect._tcp") registered by adb's own auto-connect.
func IsInstanceSerial(serial string) bool {
	return strings.HasSuffix(serial, "._adb-tls-connect._tcp")
}

type Client struct {
	adbPath string

	propsMu    sync.Mutex
	propsCache map[string]deviceProps // serial → cached manufacturer/model from getprop
}

type deviceProps struct {
	manufacturer string
	model        string
}

func NewClient(adbPath string) *Client {
	if adbPath == "" {
		adbPath = "adb"
	}
	return &Client{
		adbPath:    adbPath,
		propsCache: make(map[string]deviceProps),
	}
}

// Path returns the configured adb binary path.
func (c *Client) Path() string {
	return c.adbPath
}

// GetDevices runs `adb devices -l` and parses the output.
// GetDevices runs `adb devices -l` once, returning the connected devices
// plus the full serial -> state map (which also covers offline/unauthorized
// transports that the device list filters out).
func (c *Client) GetDevices() ([]Device, map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), adbTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.adbPath, "devices", "-l")
	platform.HideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("adb devices: %w", err)
	}
	states := parseDeviceStates(string(out))

	var devices []Device
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	first := true
	for scanner.Scan() {
		line := scanner.Text()
		// first line is just "List of devices attached" -- thanks for nothing, adb
		if first {
			first = false
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		serial := parts[0]
		if parts[1] != "device" {
			continue
		}

		devModel := ""
		for _, part := range parts[2:] {
			if strings.HasPrefix(part, "model:") {
				devModel = strings.TrimPrefix(part, "model:")
				break
			}
		}

		host := ""
		if isIPPort(serial) {
			host, _, _ = net.SplitHostPort(serial)
		}
		source := model.SourceUSB
		if strings.Contains(serial, ":") || IsInstanceSerial(serial) {
			source = model.SourceWireless
		}

		devices = append(devices, Device{
			Serial: serial,
			Model:  devModel,
			Source: source,
			Host:   host,
		})
	}

	// deduplicate: when same model has both an IP:port and an mDNS serial,
	// keep only the IP:port one (works better with scrcpy).
	seen := make(map[string]int) // model -> index in result
	var result []Device
	for _, d := range devices {
		key := d.Model
		if key == "" {
			key = d.Serial
		}
		if idx, ok := seen[key]; ok {
			// prefer IP:port over mDNS service name
			isIP := isIPPort(d.Serial)
			existingIsIP := isIPPort(result[idx].Serial)
			if isIP && !existingIsIP {
				result[idx] = d
			}
		} else {
			seen[key] = len(result)
			result = append(result, d)
		}
	}

	// fetch manufacturer + model per device in parallel; tolerate failures
	c.fillDeviceProperties(result)

	return result, states, nil
}

// fillDeviceProperties populates the Manufacturer field and refreshes Model
// for each device via getprop, preferring the getprop model (with proper
// spaces) over `adb devices -l`'s underscore-encoded form. Results are cached
// per serial since these props don't change at runtime; cache lookups skip the
// adb roundtrip entirely. Per-device failures are silently ignored.
func (c *Client) fillDeviceProperties(devices []Device) {
	var wg sync.WaitGroup
	for i := range devices {
		// fast path: cached
		c.propsMu.Lock()
		cached, ok := c.propsCache[devices[i].Serial]
		c.propsMu.Unlock()
		if ok {
			if cached.manufacturer != "" {
				devices[i].Manufacturer = cached.manufacturer
			}
			if cached.model != "" {
				devices[i].Model = cached.model
			}
			continue
		}

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), adbTimeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, c.adbPath, "-s", devices[idx].Serial, "shell",
				"getprop ro.product.manufacturer; getprop ro.product.model")
			platform.HideConsole(cmd)
			out, err := cmd.Output()
			if err != nil {
				return
			}
			// strip embedded \r (adb on Windows emits \r\n) up front
			normalized := strings.ReplaceAll(string(out), "\r", "")
			lines := strings.Split(strings.TrimRight(normalized, "\n"), "\n")
			var mfr, mdl string
			if len(lines) > 0 {
				mfr = strings.TrimSpace(lines[0])
				if strings.EqualFold(mfr, "unknown") {
					mfr = "" // treat the literal Android sentinel as missing
				}
			}
			if len(lines) > 1 {
				mdl = strings.TrimSpace(lines[1])
				if strings.EqualFold(mdl, "unknown") {
					mdl = "" // android sentinel; let `adb devices -l` model win
				}
			}
			if mfr != "" {
				devices[idx].Manufacturer = mfr
			}
			if mdl != "" {
				devices[idx].Model = mdl
			}
			c.propsMu.Lock()
			c.propsCache[devices[idx].Serial] = deviceProps{manufacturer: mfr, model: mdl}
			c.propsMu.Unlock()
		}(i)
	}
	wg.Wait()
}

// Pair runs `adb pair <addr> <password>` and returns the device guid from
// adb's output (the device's mdns instance name, stable across ports).
func (c *Client) Pair(addr, password string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), adbLongTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.adbPath, "pair", addr, password)
	platform.HideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("adb pair %s: %s (%w)", addr, string(out), err)
	}
	outStr := string(out)
	if !strings.Contains(outStr, "Successfully paired") {
		return "", fmt.Errorf("adb pair %s: unexpected output: %s", addr, outStr)
	}
	return parsePairGuid(outStr), nil
}

// parses "Successfully paired to <ip:port> [guid=<guid>]"; "" when absent.
func parsePairGuid(out string) string {
	_, after, ok := strings.Cut(out, "[guid=")
	if !ok {
		return ""
	}
	guid, _, ok := strings.Cut(after, "]")
	if !ok {
		return ""
	}
	return strings.TrimSpace(guid)
}

// DeviceStates returns serial -> state ("device", "offline", "unauthorized",
// ...) from raw `adb devices`, including transports GetDevices filters out.
func (c *Client) DeviceStates() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), adbTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.adbPath, "devices")
	platform.HideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return parseDeviceStates(string(out))
}

// parseDeviceStates handles both plain and -l output (-l pads with spaces,
// not tabs, so splitting on whitespace is the only format-safe option).
// multi word statuses like "no permissions (verify udev rules)" truncate to
// their first word; that's fine because callers only match the single word
// states "device" and "offline".
func parseDeviceStates(out string) map[string]string {
	states := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		// skip the "List of devices attached" header and "* daemon ..." banners
		if len(fields) < 2 || fields[0] == "List" || strings.HasPrefix(fields[0], "*") {
			continue
		}
		states[fields[0]] = fields[1]
	}
	return states
}

// TransportState returns the raw `adb devices` state for a serial, or "" when absent.
func (c *Client) TransportState(serial string) string {
	return c.DeviceStates()[serial]
}

// DropTransport disconnects a serial and waits out adb's asynchronous
// transport removal so the next connect does a fresh handshake.
func (c *Client) DropTransport(serial string) {
	_ = c.Disconnect(serial)
	// overall deadline, not a fixed poll count: each query has its own 10s
	// timeout, so counting iterations could block for minutes when adb hangs
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		// a nil snapshot means the query failed, not that the transport
		// is gone; keep polling rather than report success
		if states := c.DeviceStates(); states != nil {
			if _, ok := states[serial]; !ok {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// MdnsConnectAddr returns the ip:port that adb's own mdns resolver holds for
// the _adb-tls-connect service of the given guid instance, or "". adb's
// resolver sees announcements our zeroconf browse can miss (multi-nic
// windows), so this is the reliable source for the connect port after pairing.
func (c *Client) MdnsConnectAddr(guid string) string {
	if guid == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), adbTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.adbPath, "mdns", "services")
	platform.HideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return parseMdnsConnectAddr(string(out), guid)
}

// parses `adb mdns services` lines ("<instance>\t<service>\t<ip:port>") for
// the connect-service address of the given guid instance.
func parseMdnsConnectAddr(out, guid string) string {
	if guid == "" {
		return ""
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || !strings.Contains(fields[1], "_adb-tls-connect") {
			continue
		}
		if (fields[0] == guid || strings.HasPrefix(fields[0], guid)) && isIPPort(fields[2]) {
			return fields[2]
		}
	}
	return ""
}

// Connect runs `adb connect <addr>`. Returns ErrAlreadyConnected when adb
// reused an existing transport instead of handshaking.
func (c *Client) Connect(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), adbTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.adbPath, "connect", addr)
	platform.HideConsole(cmd)
	out, err := cmd.CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	slog.Debug("adb connect", "addr", addr, "output", outStr, "error", err)
	if err != nil {
		return fmt.Errorf("adb connect %s: %s (%w)", addr, outStr, err)
	}
	// adb exits 0 even on failure ("cannot connect to X:Y: ..."),
	// so detect outcomes positively
	switch {
	case strings.Contains(outStr, "already connected to"):
		return ErrAlreadyConnected
	case strings.Contains(outStr, "connected to"):
		return nil
	}
	return fmt.Errorf("adb connect %s: %s", addr, outStr)
}

// Disconnect runs `adb disconnect <addr>`.
func (c *Client) Disconnect(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), adbTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.adbPath, "disconnect", addr)
	platform.HideConsole(cmd)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb disconnect %s: %w", addr, err)
	}
	return nil
}
