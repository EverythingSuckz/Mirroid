package ui

import (
	"fmt"
	"net"
	"strings"

	"mirroid/internal/adb"
)

// parseHostFromAddr extracts the host/IP from an address that may include a port.
// Returns addr unchanged if no port is present.
func parseHostFromAddr(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// deviceFriendlyName returns a human-readable name, prefixing manufacturer
// when it isn't already part of the model string.
func deviceFriendlyName(d adb.Device) string {
	name := d.Model
	if name == "" {
		name = d.Serial
	}
	if d.Manufacturer != "" &&
		!strings.HasPrefix(strings.ToLower(name), strings.ToLower(d.Manufacturer)) {
		name = d.Manufacturer + " " + name
	}
	return name
}

// ReconnectDevice attempts to reconnect a disconnected device.
// It tracks the reconnecting state, disables buttons, and updates the UI.
func (dp *DevicePanel) ReconnectDevice(serial string) {
	dp.mu.Lock()
	if dp.reconnectingSet[serial] {
		dp.mu.Unlock()
		return // already reconnecting
	}
	dp.reconnectingSet[serial] = true
	delete(dp.reconnectErrors, serial)
	dp.mu.Unlock()

	// clear ignoredAddrs so refreshDevices won't filter the device back out
	dp.app.ignoredAddrs.Delete(serial)
	if host := parseHostFromAddr(serial); host != serial {
		dp.app.ignoredAddrs.Delete(host)
	}

	dp.updateList()

	go func() {
		dp.app.logsPanel.Log("Reconnecting to " + serial + "...")
		err := dp.app.adbClient.Connect(serial)

		dp.mu.Lock()
		delete(dp.reconnectingSet, serial)
		if err != nil {
			dp.reconnectErrors[serial] = err.Error()
		}
		dp.mu.Unlock()

		if err != nil {
			dp.app.logsPanel.Log("[ERROR]Reconnect " + serial + ": " + err.Error())
		} else {
			dp.app.logsPanel.Log("[OK]Reconnected " + serial)
		}

		dp.refreshDevices()

		// reload info panel if this device is selected
		dp.mu.Lock()
		selected := dp.lastSelected == serial
		dp.mu.Unlock()
		if selected && dp.app.deviceInfoPanel != nil {
			dp.app.deviceInfoPanel.LoadDeviceInfo(serial)
		}
	}()
}

// RemoveDevice permanently removes a device from the known list.
func (dp *DevicePanel) RemoveDevice(serial string) {
	dp.mu.Lock()
	for i, d := range dp.devices {
		if d.Serial == serial {
			dp.devices = append(dp.devices[:i], dp.devices[i+1:]...)
			break
		}
	}
	delete(dp.connectedSet, serial)
	delete(dp.checkedSerials, serial)
	if dp.lastSelected == serial {
		dp.lastSelected = ""
	}
	dp.mu.Unlock()

	dp.saveKnownDevices()
	dp.updateList()
}

// OnMdnsDevices handles devices discovered via mDNS.
func (dp *DevicePanel) OnMdnsDevices(mdnsDevices []adb.MdnsDevice) {
	for _, md := range mdnsDevices {
		ip := parseHostFromAddr(md.Addr)
		if dp.app.isIgnored(ip) {
			continue
		}

		dp.mu.Lock()
		alreadyConnected := dp.connectedSet[md.Addr]
		dp.mu.Unlock()

		if !alreadyConnected {
			if err := dp.app.adbClient.Connect(md.Addr); err == nil {
				// verify this isn't an alias of a device the user disconnected
				if devID := dp.app.adbClient.GetDeviceID(md.Addr); devID != "" && dp.app.isIgnored("devid:"+devID) {
					_ = dp.app.adbClient.Disconnect(md.Addr)
					continue
				}
				dp.app.logsPanel.Log(fmt.Sprintf("mDNS: Connected to %s", md.Addr))
				go dp.refreshDevices()
			}
		}
	}
}

// saveKnownDevices persists the known device list to config.
func (dp *DevicePanel) saveKnownDevices() {
	if dp.app.cfg == nil {
		return
	}
	dp.mu.Lock()
	devices := make([]adb.Device, len(dp.devices))
	copy(devices, dp.devices)
	dp.mu.Unlock()

	if err := dp.app.cfg.SaveKnownDevices(devices); err != nil {
		dp.app.logsPanel.Log("[WARN]Failed to save known devices: " + err.Error())
	}
}

// clearDisconnected removes all disconnected devices from the known list.
func (dp *DevicePanel) clearDisconnected() {
	dp.mu.Lock()
	var keep []adb.Device
	for _, d := range dp.devices {
		if dp.connectedSet[d.Serial] {
			keep = append(keep, d)
		} else {
			delete(dp.checkedSerials, d.Serial)
			if dp.lastSelected == d.Serial {
				dp.lastSelected = ""
			}
		}
	}
	removed := len(dp.devices) - len(keep)
	dp.devices = keep
	dp.mu.Unlock()

	if removed > 0 {
		dp.app.logsPanel.Log(fmt.Sprintf("[OK]Cleared %d disconnected device(s)", removed))
		dp.saveKnownDevices()
		dp.updateList()
	}
}
