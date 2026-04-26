package ui

import (
	"fyne.io/fyne/v2"

	"mirroid/internal/adb"
)

// refreshDevices queries ADB and merges results with known devices.
// Connected devices are updated; previously known devices that are absent
// are kept in the table as "Disconnected".
func (dp *DevicePanel) refreshDevices() {
	if dp.app.adbClient == nil {
		return
	}
	liveDevices, err := dp.app.adbClient.GetDevices()
	if err != nil {
		fyne.Do(func() {
			dp.statusLabel.SetText("Error: " + err.Error())
		})
		return
	}

	// filter out explicitly ignored devices (by serial or host/IP)
	var filtered []adb.Device
	for _, d := range liveDevices {
		if dp.app.isIgnored(d.Serial) {
			_ = dp.app.adbClient.Disconnect(d.Serial)
			continue
		}
		if host := parseHostFromAddr(d.Serial); host != d.Serial && dp.app.isIgnored(host) {
			_ = dp.app.adbClient.Disconnect(d.Serial)
			continue
		}
		filtered = append(filtered, d)
	}
	liveDevices = filtered

	dp.mu.Lock()

	// capture old connection state for selected device
	selectedSerial := dp.lastSelected
	wasConnected := dp.connectedSet[selectedSerial]

	// build connected set from live ADB results
	dp.connectedSet = make(map[string]bool, len(liveDevices))
	for _, d := range liveDevices {
		dp.connectedSet[d.Serial] = true
		// clear reconnect errors for devices that are now connected
		delete(dp.reconnectErrors, d.Serial)
	}

	// merge live devices into known devices list.
	// match by serial first, then by model to avoid duplicates when
	// ADB reports the same device under a different serial (mDNS alias vs IP:port).
	knownBySerial := make(map[string]int, len(dp.devices))
	knownByModel := make(map[string]int, len(dp.devices))
	for i, d := range dp.devices {
		knownBySerial[d.Serial] = i
		if d.Model != "" {
			knownByModel[d.Model] = i
		}
	}
	for _, d := range liveDevices {
		if idx, exists := knownBySerial[d.Serial]; exists {
			// exact serial match and update metadata
			dp.devices[idx].Model = d.Model
			dp.devices[idx].Source = d.Source
			if d.Manufacturer != "" {
				dp.devices[idx].Manufacturer = d.Manufacturer
			}
		} else if idx, exists := knownByModel[d.Model]; d.Model != "" && exists {
			// same model, different serial (mDNS alias changed) update the existing entry
			oldSerial := dp.devices[idx].Serial
			dp.devices[idx].Serial = d.Serial
			dp.devices[idx].Source = d.Source
			if d.Manufacturer != "" {
				dp.devices[idx].Manufacturer = d.Manufacturer
			}
			// migrate serial-keyed state
			delete(dp.connectedSet, oldSerial)
			dp.connectedSet[d.Serial] = true
			if dp.checkedSerials[oldSerial] {
				delete(dp.checkedSerials, oldSerial)
				dp.checkedSerials[d.Serial] = true
			}
			if dp.reconnectingSet[oldSerial] {
				delete(dp.reconnectingSet, oldSerial)
				dp.reconnectingSet[d.Serial] = true
			}
			if errMsg, ok := dp.reconnectErrors[oldSerial]; ok {
				delete(dp.reconnectErrors, oldSerial)
				dp.reconnectErrors[d.Serial] = errMsg
			}
			if dp.lastSelected == oldSerial {
				dp.lastSelected = d.Serial
				selectedSerial = d.Serial
			}
		} else {
			// truly new device
			dp.devices = append(dp.devices, d)
		}
	}

	// capture new connection state
	nowConnected := dp.connectedSet[selectedSerial]

	dp.mu.Unlock()

	dp.saveKnownDevices()

	// auto-clear errors from exited processes so "Error" shows for one
	// refresh cycle (~3 s) then transitions back to "Connected"
	if dp.app.runner != nil {
		dp.app.runner.ClearExitedErrors()
	}

	dp.updateList()

	// reload info panel if selected device's connection state changed
	if selectedSerial != "" && wasConnected != nowConnected && dp.app.deviceInfoPanel != nil {
		fyne.Do(func() {
			dp.app.deviceInfoPanel.LoadDeviceInfo(selectedSerial)
		})
	}
}

// SelectedDevice returns the serial of the currently highlighted device (single).
func (dp *DevicePanel) SelectedDevice() string {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	if dp.lastSelected == "" {
		return ""
	}
	for _, d := range dp.devices {
		if d.Serial == dp.lastSelected {
			return d.Serial
		}
	}
	return ""
}

// SelectedDevices returns the serials of all checked connected devices (multi-select).
func (dp *DevicePanel) SelectedDevices() []string {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	var serials []string
	for _, d := range dp.devices {
		if dp.checkedSerials[d.Serial] && dp.connectedSet[d.Serial] {
			serials = append(serials, d.Serial)
		}
	}
	return serials
}

// SelectedDisconnectedDevices returns the serials of all checked disconnected devices.
func (dp *DevicePanel) SelectedDisconnectedDevices() []string {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	var serials []string
	for _, d := range dp.devices {
		if dp.checkedSerials[d.Serial] && !dp.connectedSet[d.Serial] {
			serials = append(serials, d.Serial)
		}
	}
	return serials
}

// HasDevices returns whether any devices are known (connected or previously seen).
func (dp *DevicePanel) HasDevices() bool {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	return len(dp.devices) > 0
}

// IsConnected returns whether a device serial is currently connected.
func (dp *DevicePanel) IsConnected(serial string) bool {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	return dp.connectedSet[serial]
}

// GetDevice returns the known device entry for the given serial.
func (dp *DevicePanel) GetDevice(serial string) (adb.Device, bool) {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	for _, d := range dp.devices {
		if d.Serial == serial {
			return d, true
		}
	}
	return adb.Device{}, false
}

// IsReconnecting returns whether a device is currently reconnecting.
func (dp *DevicePanel) IsReconnecting(serial string) bool {
	dp.mu.Lock()
	defer dp.mu.Unlock()
	return dp.reconnectingSet[serial]
}
