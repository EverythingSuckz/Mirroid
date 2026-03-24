package ui

import (
	"fmt"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/adb"
)

// DevicePanel manages device selection.
type DevicePanel struct {
	app          *App
	deviceSelect *widget.Select
	statusLabel  *widget.Label
	pairBtn      *widget.Button
	devices      []adb.Device
	lastSelected string // track to avoid redundant LoadDeviceInfo on auto-polls
	mu           sync.Mutex
}

// NewDevicePanel creates a new device panel.
func NewDevicePanel(app *App) *DevicePanel {
	return &DevicePanel{app: app}
}

// Build creates the device panel UI.
func (dp *DevicePanel) Build() fyne.CanvasObject {
	dp.deviceSelect = widget.NewSelect([]string{}, func(selected string) {
		serial := dp.SelectedDevice()
		// only reload device info if the selection actually changed
		if serial != dp.lastSelected && dp.app.deviceInfoPanel != nil {
			dp.lastSelected = serial
			go dp.app.deviceInfoPanel.LoadDeviceInfo(serial)
		}
	})
	dp.deviceSelect.PlaceHolder = "Select a device..."

	dp.statusLabel = widget.NewLabel("No devices found")
	dp.statusLabel.TextStyle = fyne.TextStyle{Italic: true}

	refreshBtn := widget.NewButton("Refresh", func() {
		// Manual refresh: clear lastSelected to force device info reload
		dp.lastSelected = ""
		go dp.refreshDevices()
	})

	dp.pairBtn = widget.NewButton("Pair New Device", func() {
		ShowPairingWindow(dp.app)
	})
	dp.pairBtn.Importance = widget.HighImportance

	return container.NewVBox(
		dp.deviceSelect,
		container.NewHBox(dp.statusLabel, refreshBtn, dp.pairBtn),
	)
}

// refreshDevices queries ADB for connected devices.
func (dp *DevicePanel) refreshDevices() {
	devices, err := dp.app.adbClient.GetDevices()
	if err != nil {
		fyne.Do(func() {
			dp.statusLabel.SetText("Error: " + err.Error())
		})
		return
	}

	// filter out devices the user explicitly disconnected
	var filtered []adb.Device
	for _, d := range devices {
		if d.Model != "" && dp.app.ignoredAddrs[d.Model] {
			_ = dp.app.adbClient.Disconnect(d.Serial)
			continue
		}
		filtered = append(filtered, d)
	}
	devices = filtered

	dp.mu.Lock()
	dp.devices = devices
	dp.mu.Unlock()

	dp.updateDropdown()
}

// OnMdnsDevices handles devices discovered via mDNS.
func (dp *DevicePanel) OnMdnsDevices(mdnsDevices []adb.MdnsDevice) {
	for _, md := range mdnsDevices {
		// skip if this address was explicitly disconnected by the user
		ip := md.Addr
		if idx := strings.Index(ip, ":"); idx > 0 {
			ip = ip[:idx]
		}
		if dp.app.ignoredAddrs[ip] {
			continue
		}

		dp.mu.Lock()
		alreadyConnected := false
		for _, d := range dp.devices {
			if d.Serial == md.Addr {
				alreadyConnected = true
				break
			}
		}
		dp.mu.Unlock()

		if !alreadyConnected {
			if err := dp.app.adbClient.Connect(md.Addr); err == nil {
				dp.app.logsPanel.Log(fmt.Sprintf("mDNS: Connected to %s", md.Addr))
				go dp.refreshDevices()
			}
		}
	}
}

// updateDropdown refreshes the dropdown options.
func (dp *DevicePanel) updateDropdown() {
	dp.mu.Lock()
	devices := dp.devices
	dp.mu.Unlock()

	options := make([]string, len(devices))
	for i, d := range devices {
		options[i] = d.String()
	}

	fyne.Do(func() {
		dp.deviceSelect.Options = options
		if len(options) > 0 {
			dp.deviceSelect.SetSelected(options[0])
			dp.statusLabel.SetText(fmt.Sprintf("[OK] %d device(s)", len(options)))
			dp.pairBtn.Hide()
			// auto close pairing window only if it was auto-opened at first boot
			if dp.app.autoOpenedPairingWindow != nil {
				dp.app.autoOpenedPairingWindow.Close()
				dp.app.autoOpenedPairingWindow = nil
			}
		} else {
			dp.deviceSelect.ClearSelected()
			dp.statusLabel.SetText("No devices found")
			dp.pairBtn.Show()
			if dp.app.deviceInfoPanel != nil {
				dp.app.deviceInfoPanel.LoadDeviceInfo("")
			}
		}
	})
}

// SelectedDevice returns the serial of the currently selected device.
func (dp *DevicePanel) SelectedDevice() string {
	dp.mu.Lock()
	defer dp.mu.Unlock()

	selected := dp.deviceSelect.Selected
	for _, d := range dp.devices {
		if d.String() == selected {
			return d.Serial
		}
	}
	return ""
}
