package adb

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"

	"mirroid/internal/model"
	"mirroid/internal/platform"
)

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
func (c *Client) GetDevices() ([]Device, error) {
	ctx, cancel := context.WithTimeout(context.Background(), adbTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.adbPath, "devices", "-l")
	platform.HideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("adb devices: %w", err)
	}

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

		source := model.SourceUSB
		if strings.Contains(serial, ":") {
			source = model.SourceWireless
		}

		devices = append(devices, Device{
			Serial: serial,
			Model:  devModel,
			Source: source,
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

	return result, nil
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

// Pair runs `adb pair <addr> <password>`.
func (c *Client) Pair(addr, password string) error {
	ctx, cancel := context.WithTimeout(context.Background(), adbLongTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.adbPath, "pair", addr, password)
	platform.HideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb pair %s: %s (%w)", addr, string(out), err)
	}
	outStr := string(out)
	if !strings.Contains(outStr, "Successfully paired") {
		return fmt.Errorf("adb pair %s: unexpected output: %s", addr, outStr)
	}
	return nil
}

// Connect runs `adb connect <addr>`.
func (c *Client) Connect(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), adbTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.adbPath, "connect", addr)
	platform.HideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb connect %s: %s (%w)", addr, string(out), err)
	}
	outStr := string(out)
	if strings.Contains(outStr, "failed") || strings.Contains(outStr, "error") {
		return fmt.Errorf("adb connect %s: %s", addr, outStr)
	}
	return nil
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

// VerifyConnection checks if a device is actually reachable by running a shell command.
func (c *Client) VerifyConnection(serial string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), adbTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.adbPath, "-s", serial, "shell", "echo", "ok")
	platform.HideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "ok"
}
