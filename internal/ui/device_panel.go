package ui

import (
	"fmt"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	ttwidget "github.com/dweymouth/fyne-tooltip/widget"

	"mirroid/internal/adb"
	"mirroid/internal/model"
	"mirroid/internal/scrcpy"
)

// DevicePanel manages device selection via a multi-select table.
type DevicePanel struct {
	app            *App
	deviceList     *widget.List
	selectAllCheck *widget.Check
	statusLabel    *widget.Label
	devices        []adb.Device      // all known devices (connected + disconnected)
	connectedSet   map[string]bool   // serials currently reported by adb
	checkedSerials map[string]bool   // serials checked for batch actions
	reconnectingSet map[string]bool  // serials currently reconnecting
	reconnectErrors map[string]string // serials that failed to reconnect
	lastSelected   string
	mu             sync.Mutex

	// bulk action buttons — context-sensitive based on checked devices
	mirrorBtn     *ttwidget.Button
	stopBtn       *ttwidget.Button
	disconnectBtn *ttwidget.Button
	previewBtn    *ttwidget.Button
	removeBtn     *ttwidget.Button // remove checked (disconnected) devices
	reconnectBtn  *ttwidget.Button // reconnect checked disconnected devices
	actionSep     *widget.Separator
}

// NewDevicePanel creates a new device panel.
func NewDevicePanel(app *App) *DevicePanel {
	dp := &DevicePanel{
		app:             app,
		connectedSet:    make(map[string]bool),
		checkedSerials:  make(map[string]bool),
		reconnectingSet: make(map[string]bool),
		reconnectErrors: make(map[string]string),
	}
	// Load previously known devices from config
	if app.cfg != nil {
		if saved, err := app.cfg.LoadKnownDevices(); err == nil && len(saved) > 0 {
			dp.devices = saved
		}
	}
	return dp
}

func (dp *DevicePanel) Build() fyne.CanvasObject {
	dp.selectAllCheck = widget.NewCheck("", func(checked bool) {
		dp.mu.Lock()
		if checked {
			for _, d := range dp.devices {
				dp.checkedSerials[d.Serial] = true
			}
		} else {
			dp.checkedSerials = make(map[string]bool)
		}
		dp.mu.Unlock()
		dp.deviceList.Refresh()
		dp.syncActionVisibility()
	})

	headerRow := container.NewBorder(nil, nil,
		dp.selectAllCheck, nil,
		container.NewGridWithColumns(4,
			widget.NewLabelWithStyle("Model", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Address", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Type", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			widget.NewLabelWithStyle("Status", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		),
	)

	dp.deviceList = widget.NewList(
		func() int {
			dp.mu.Lock()
			defer dp.mu.Unlock()
			return len(dp.devices)
		},
		func() fyne.CanvasObject {
			modelLabel := widget.NewLabel("")
			modelLabel.Truncation = fyne.TextTruncateEllipsis
			addrLabel := widget.NewLabel("")
			addrLabel.Truncation = fyne.TextTruncateEllipsis
			typeLabel := widget.NewLabel("")
			typeLabel.Truncation = fyne.TextTruncateEllipsis
			statusLabel := ttwidget.NewLabel("")
			statusLabel.Truncation = fyne.TextTruncateEllipsis
			return container.NewBorder(nil, nil,
				widget.NewCheck("", nil), nil,
				container.NewGridWithColumns(4,
					modelLabel,
					addrLabel,
					typeLabel,
					statusLabel,
				),
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			row := item.(*fyne.Container)
			check := row.Objects[1].(*widget.Check)
			cols := row.Objects[0].(*fyne.Container)

			dp.mu.Lock()
			if id >= len(dp.devices) {
				dp.mu.Unlock()
				return
			}
			d := dp.devices[id]
			isChecked := dp.checkedSerials[d.Serial]
			connected := dp.connectedSet[d.Serial]
			dp.mu.Unlock()

			cols.Objects[0].(*widget.Label).SetText(func() string {
				if d.Model != "" {
					return d.Model
				}
				return d.Serial
			}())
			cols.Objects[1].(*widget.Label).SetText(d.Serial)
			cols.Objects[2].(*widget.Label).SetText(func() string {
				if d.Source == model.SourceWireless || d.Source == model.SourceMDNS {
					return "Wi-Fi"
				}
				return "USB"
			}())

			// Status priority:
			// Connected:    Error > Launching/Mirroring > Connected
			// Disconnected: Reconnecting > Error > Disconnected
			status := model.StatusDisconnected
			statusTip := ""
			if connected {
				status = model.StatusConnected
				if dp.app.runner != nil {
					switch dp.app.runner.StateFor(d.Serial) {
					case scrcpy.StateError:
						status = model.StatusError
						if errMsg := dp.app.runner.LastErrorFor(d.Serial); errMsg != "" {
							statusTip = errMsg + " (check logs)"
						}
					case scrcpy.StateLaunching:
						status = model.StatusLaunching
					case scrcpy.StateMirroring:
						status = model.StatusMirroring
					}
				}
			} else {
				dp.mu.Lock()
				reconnecting := dp.reconnectingSet[d.Serial]
				reconnectErr := dp.reconnectErrors[d.Serial]
				dp.mu.Unlock()
				if reconnecting {
					status = model.StatusReconnecting
				} else if reconnectErr != "" {
					status = model.StatusError
					statusTip = reconnectErr + " (check logs)"
				}
			}
			statusLbl := cols.Objects[3].(*ttwidget.Label)
			statusLbl.SetText(string(status))
			statusLbl.SetToolTip(statusTip)

			serial := d.Serial
			check.OnChanged = nil
			check.Enable()
			check.SetChecked(isChecked)
			check.OnChanged = func(checked bool) {
				dp.mu.Lock()
				if checked {
					dp.checkedSerials[serial] = true
				} else {
					delete(dp.checkedSerials, serial)
				}
				dp.mu.Unlock()
				dp.syncSelectAllCheck()
				dp.syncActionVisibility()
			}
		},
	)

	dp.deviceList.OnSelected = func(id widget.ListItemID) {
		dp.mu.Lock()
		serial := ""
		if id >= 0 && id < len(dp.devices) {
			serial = dp.devices[id].Serial
		}
		changed := serial != "" && serial != dp.lastSelected
		if changed {
			dp.lastSelected = serial
		}
		dp.mu.Unlock()

		if changed && dp.app.deviceInfoPanel != nil {
			go dp.app.deviceInfoPanel.LoadDeviceInfo(serial)
		}
		if changed && dp.app.presetsPanel != nil {
			dp.app.presetsPanel.LoadPresetForDevice(serial)
		}
	}

	dp.statusLabel = widget.NewLabel("")

	//  Bulk action buttons (shown when 2+ devices checked)
	dp.mirrorBtn = ttwidget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
		dp.app.onLaunch()
		dp.deviceList.Refresh()
	})
	dp.mirrorBtn.Importance = widget.LowImportance
	dp.mirrorBtn.SetToolTip("Mirror checked devices")

	dp.stopBtn = ttwidget.NewButtonWithIcon("", theme.MediaStopIcon(), func() {
		for _, s := range dp.SelectedDevices() {
			dp.app.runner.StopFor(s)
			dp.app.logsPanel.Log("Stopped " + s)
		}
		dp.deviceList.Refresh()
	})
	dp.stopBtn.Importance = widget.LowImportance
	dp.stopBtn.SetToolTip("Stop checked devices")

	dp.disconnectBtn = ttwidget.NewButtonWithIcon("", theme.LogoutIcon(), func() {
		go func() {
			disconnectedDevIDs := make(map[string]bool)

			for _, s := range dp.SelectedDevices() {
				dp.app.runner.StopFor(s)

				// fetch hardware device ID while still connected
				devID := dp.app.adbClient.GetDeviceID(s)

				if err := dp.app.adbClient.Disconnect(s); err != nil {
					dp.app.logsPanel.Log("[ERROR]Disconnect: " + err.Error())
				} else {
					dp.app.logsPanel.Log("[OK]Disconnected " + s)
				}

				dp.app.ignoreDevice(s, devID)
				if devID != "" {
					disconnectedDevIDs[devID] = true
				}
			}

			dp.app.disconnectAliases(disconnectedDevIDs)
			dp.refreshDevices()
		}()
	})
	dp.disconnectBtn.Importance = widget.LowImportance
	dp.disconnectBtn.SetToolTip("Disconnect checked devices")

	dp.previewBtn = ttwidget.NewButtonWithIcon("", theme.DocumentIcon(), func() {
		dp.app.onShowCommand()
	})
	dp.previewBtn.Importance = widget.LowImportance
	dp.previewBtn.SetToolTip("Preview command")

	// Disconnected-device bulk actions
	dp.removeBtn = ttwidget.NewButtonWithIcon("", theme.ContentRemoveIcon(), func() {
		for _, s := range dp.SelectedDisconnectedDevices() {
			dp.app.logsPanel.Log("[OK]Removed " + s)
		}
		dp.mu.Lock()
		var keep []adb.Device
		for _, d := range dp.devices {
			if !dp.checkedSerials[d.Serial] || dp.connectedSet[d.Serial] {
				keep = append(keep, d)
			} else {
				delete(dp.checkedSerials, d.Serial)
				if dp.lastSelected == d.Serial {
					dp.lastSelected = ""
				}
			}
		}
		dp.devices = keep
		dp.mu.Unlock()
		dp.saveKnownDevices()
		dp.updateList()
	})
	dp.removeBtn.Importance = widget.LowImportance
	dp.removeBtn.SetToolTip("Remove checked devices")

	dp.reconnectBtn = ttwidget.NewButtonWithIcon("", theme.MediaReplayIcon(), func() {
		for _, s := range dp.SelectedDisconnectedDevices() {
			dp.ReconnectDevice(s)
		}
	})
	dp.reconnectBtn.Importance = widget.LowImportance
	dp.reconnectBtn.SetToolTip("Reconnect checked devices")

	dp.actionSep = widget.NewSeparator()

	// Start hidden
	dp.mirrorBtn.Hide()
	dp.stopBtn.Hide()
	dp.disconnectBtn.Hide()
	dp.previewBtn.Hide()
	dp.removeBtn.Hide()
	dp.reconnectBtn.Hide()
	dp.actionSep.Hide()

	//  Always-visible toolbar buttons
	refreshBtn := ttwidget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		dp.mu.Lock()
		dp.lastSelected = ""
		dp.mu.Unlock()
		go dp.refreshDevices()
	})
	refreshBtn.Importance = widget.LowImportance
	refreshBtn.SetToolTip("Refresh device list")

	pairBtn := ttwidget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
		ShowPairingWindow(dp.app)
	})
	pairBtn.Importance = widget.LowImportance
	pairBtn.SetToolTip("Pair new device")

	moreMenu := fyne.NewMenu("",
		fyne.NewMenuItem("Clear Disconnected", func() {
			dp.clearDisconnected()
		}),
	)
	moreBtn := ttwidget.NewButtonWithIcon("", theme.MoreVerticalIcon(), func() {
		c := fyne.CurrentApp().Driver().CanvasForObject(dp.deviceList)
		widget.ShowPopUpMenuAtRelativePosition(moreMenu, c,
			fyne.NewPos(0, refreshBtn.Size().Height), refreshBtn)
	})
	moreBtn.Importance = widget.LowImportance
	moreBtn.SetToolTip("More options")

	toolbar := container.NewHBox(
		dp.mirrorBtn,
		dp.stopBtn,
		dp.disconnectBtn,
		dp.previewBtn,
		dp.removeBtn,
		dp.reconnectBtn,
		dp.actionSep,
		refreshBtn,
		pairBtn,
		moreBtn,
	)

	topSection := container.NewVBox(
		container.NewBorder(nil, nil, nil, toolbar, dp.statusLabel),
		headerRow,
		canvas.NewLine(theme.Color(theme.ColorNameSeparator)),
	)

	return container.NewBorder(
		topSection, nil, nil, nil,
		dp.deviceList,
	)
}

// syncSelectAllCheck updates the header "select all" checkbox to reflect
// whether every device is individually checked, without triggering its handler.
func (dp *DevicePanel) syncSelectAllCheck() {
	dp.mu.Lock()
	allChecked := len(dp.devices) > 0 && len(dp.checkedSerials) == len(dp.devices)
	dp.mu.Unlock()

	savedHandler := dp.selectAllCheck.OnChanged
	dp.selectAllCheck.OnChanged = nil
	dp.selectAllCheck.SetChecked(allChecked)
	dp.selectAllCheck.OnChanged = savedHandler
}

// syncActionVisibility shows context-sensitive bulk action buttons based on checked devices.
func (dp *DevicePanel) syncActionVisibility() {
	dp.mu.Lock()
	hasConnected := false
	hasDisconnected := false
	anyReconnecting := false
	for serial := range dp.checkedSerials {
		if dp.connectedSet[serial] {
			hasConnected = true
		} else {
			hasDisconnected = true
			if dp.reconnectingSet[serial] {
				anyReconnecting = true
			}
		}
	}
	dp.mu.Unlock()

	// Connected-device actions
	if hasConnected {
		dp.mirrorBtn.Show()
		dp.stopBtn.Show()
		dp.disconnectBtn.Show()
		dp.previewBtn.Show()
	} else {
		dp.mirrorBtn.Hide()
		dp.stopBtn.Hide()
		dp.disconnectBtn.Hide()
		dp.previewBtn.Hide()
	}

	// Disconnected-device actions
	if hasDisconnected {
		dp.removeBtn.Show()
		dp.reconnectBtn.Show()
		if anyReconnecting {
			dp.reconnectBtn.Disable()
		} else {
			dp.reconnectBtn.Enable()
		}
	} else {
		dp.removeBtn.Hide()
		dp.reconnectBtn.Hide()
	}

	// Separator between context actions and permanent buttons
	if hasConnected || hasDisconnected {
		dp.actionSep.Show()
	} else {
		dp.actionSep.Hide()
	}
}

// updateList refreshes the list widget and toggles empty/connected layout.
func (dp *DevicePanel) updateList() {
	dp.mu.Lock()
	devices := dp.devices
	lastSel := dp.lastSelected
	connectedCount := len(dp.connectedSet)
	dp.mu.Unlock()

	fyne.Do(func() {
		if len(devices) > 0 {
			if connectedCount == len(devices) {
				dp.statusLabel.SetText(fmt.Sprintf("%d device(s)", len(devices)))
			} else {
				dp.statusLabel.SetText(fmt.Sprintf("%d device(s), %d connected", len(devices), connectedCount))
			}

			if lastSel == "" {
				// Force fresh selection — UnselectAll first so Select(0) fires OnSelected
				dp.deviceList.UnselectAll()
				dp.deviceList.Select(0)
			} else {
				found := false
				for i, d := range devices {
					if d.Serial == lastSel {
						dp.deviceList.Select(i)
						found = true
						break
					}
				}
				if !found {
					dp.mu.Lock()
					dp.lastSelected = ""
					dp.mu.Unlock()
					// Force fresh selection — UnselectAll first so Select(0) fires OnSelected
					dp.deviceList.UnselectAll()
					dp.deviceList.Select(0)
				}
			}

			dp.app.setConnectedLayout(true)
		} else {
			dp.mu.Lock()
			dp.lastSelected = ""
			dp.mu.Unlock()
			dp.deviceList.UnselectAll()
			if dp.app.deviceInfoPanel != nil {
				dp.app.deviceInfoPanel.LoadDeviceInfo("")
			}

			dp.app.setConnectedLayout(false)
		}
		dp.deviceList.Refresh()
		dp.syncSelectAllCheck()
		dp.syncActionVisibility()
	})
}

