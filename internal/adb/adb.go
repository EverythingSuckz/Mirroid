package adb

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
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
	Serial string `json:"serial"`
	Model  string `json:"model"`
	Source string `json:"source"` // "usb", "wireless", "mdns"
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
}

func NewClient(adbPath string) *Client {
	if adbPath == "" {
		adbPath = "adb"
	}
	return &Client{adbPath: adbPath}
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
			Source:  source,
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
	return result, nil
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

