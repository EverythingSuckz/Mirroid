package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/adb"
	"mirroid/internal/platform"
	"mirroid/internal/scrcpy"
)

// DeviceInfoPanel shows detailed information about the selected device.
type DeviceInfoPanel struct {
	app           *App
	container     *fyne.Container
	activity      *widget.Activity
	currentSerial string         // track which device is displayed
	mirrorBtn     *widget.Button // stored for reactive state updates
	stopBtn       *widget.Button // stored for reactive state updates
}

func NewDeviceInfoPanel(app *App) *DeviceInfoPanel {
	return &DeviceInfoPanel{app: app}
}

func (dip *DeviceInfoPanel) Build() fyne.CanvasObject {
	dip.activity = widget.NewActivity()
	placeholder := widget.NewLabel("Select a device to view info")
	placeholder.TextStyle = fyne.TextStyle{Italic: true}

	dip.container = container.NewStack(container.NewCenter(placeholder))
	return container.NewPadded(dip.container)
}

// LoadDeviceInfo fetches and displays device info for the given serial.
func (dip *DeviceInfoPanel) LoadDeviceInfo(serial string) {
	if serial == "" {
		fyne.Do(func() {
			dip.currentSerial = ""
			dip.mirrorBtn = nil
			dip.stopBtn = nil
			dip.app.setOptionsAreaVisible(true)
			placeholder := widget.NewLabel("Select a device to view info")
			placeholder.TextStyle = fyne.TextStyle{Italic: true}
			dip.container.Objects = []fyne.CanvasObject{container.NewCenter(placeholder)}
			dip.container.Refresh()
		})
		return
	}

	// Show disconnected view if the device is not connected
	if dip.app.devicePanel != nil && !dip.app.devicePanel.IsConnected(serial) {
		fyne.Do(func() {
			dip.currentSerial = serial
			dip.mirrorBtn = nil
			dip.stopBtn = nil
			dip.app.setOptionsAreaVisible(false)
			dip.container.Objects = []fyne.CanvasObject{dip.buildDisconnectedView(serial)}
			dip.container.Refresh()
		})
		return
	}

	fyne.Do(func() {
		dip.currentSerial = serial
		dip.app.setOptionsAreaVisible(true)
		dip.activity.Start()
		dip.activity.Show()
		loading := container.NewCenter(container.NewVBox(
			dip.activity,
			widget.NewLabel("Loading device info..."),
		))
		dip.container.Objects = []fyne.CanvasObject{loading}
		dip.container.Refresh()
	})

	go func() {
		info := dip.app.adbClient.GetDeviceInfo(serial)
		fyne.Do(func() {
			// Discard stale result if user has since selected a different device
			if dip.currentSerial != serial {
				return
			}
			dip.activity.Stop()
			dip.container.Objects = []fyne.CanvasObject{dip.buildInfoView(serial, info)}
			dip.container.Refresh()
		})
	}()
}

// RefreshActions updates only the Mirror/Stop button states without
// re-fetching device info from ADB. Safe to call from any goroutine.
func (dip *DeviceInfoPanel) RefreshActions() {
	fyne.Do(func() {
		serial := dip.currentSerial
		if serial == "" || dip.mirrorBtn == nil || dip.stopBtn == nil {
			return
		}
		state := dip.app.runner.StateFor(serial)
		switch state {
		case scrcpy.StateLaunching, scrcpy.StateMirroring:
			dip.mirrorBtn.Disable()
			dip.stopBtn.Enable()
		default:
			dip.mirrorBtn.Enable()
			dip.stopBtn.Disable()
		}
	})
}

func (dip *DeviceInfoPanel) buildInfoView(serial string, info adb.DeviceInfo) fyne.CanvasObject {
	titleText := fmt.Sprintf("%s %s", info.Manufacturer, info.Model)
	title := widget.NewLabelWithStyle(titleText, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	form := widget.NewForm(
		widget.NewFormItem("Model", widget.NewLabel(info.Model)),
		widget.NewFormItem("Brand", widget.NewLabel(info.Manufacturer)),
		widget.NewFormItem("Android", widget.NewLabel(info.AndroidVersion+" (SDK "+info.SDK+")")),
		widget.NewFormItem("Build", widget.NewLabel(info.BuildID)),
		widget.NewFormItem("Address", widget.NewLabel(info.Serial)),
		widget.NewFormItem("Display", widget.NewLabel(info.Resolution+" @ "+info.Density+" dpi")),
		widget.NewFormItem("Battery", widget.NewLabel(info.Battery)),
	)

	// Primary device actions
	dip.mirrorBtn = widget.NewButtonWithIcon("Mirror", theme.MediaPlayIcon(), func() {
		dip.app.optionsPanel.SyncToModel(&dip.app.options)
		if err := dip.app.options.Validate(); err != nil {
			dip.app.logsPanel.Log("[WARN]" + err.Error())
			return
		}
		preview := dip.app.runner.CommandPreview(serial, dip.app.options)
		dip.app.logsPanel.Log(">" + preview)
		err := dip.app.runner.Launch(serial, dip.app.options, func(line string) {
			dip.app.logsPanel.Log(line)
		}, "")
		if err != nil {
			dip.app.logsPanel.Log("[ERROR]" + err.Error())
		}
		// OnStateChange callback handles deviceList.Refresh() and RefreshActions()
	})
	dip.mirrorBtn.Importance = widget.HighImportance

	dip.stopBtn = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), func() {
		dip.app.runner.StopFor(serial)
		dip.app.logsPanel.Log("Stopped " + serial)
		// OnStateChange callback handles deviceList.Refresh() and RefreshActions()
	})
	dip.stopBtn.Importance = widget.DangerImportance
	state := dip.app.runner.StateFor(serial)
	if state == scrcpy.StateLaunching || state == scrcpy.StateMirroring {
		dip.mirrorBtn.Disable()
		dip.stopBtn.Enable()
	} else {
		dip.stopBtn.Disable()
	}

	disconnectBtn := widget.NewButtonWithIcon("Disconnect", theme.LogoutIcon(), func() {
		go func() {
			dip.app.runner.StopFor(serial)
			if err := dip.app.adbClient.Disconnect(serial); err != nil {
				dip.app.logsPanel.Log("[ERROR]Disconnect: " + err.Error())
			} else {
				dip.app.logsPanel.Log("[OK]Disconnected " + serial)
				// block this device from reconnecting via serial, host/IP,
				// and hardware device ID (ro.serialno).
				dip.app.ignoredAddrs.Store(serial, true)
				ip := parseHostFromAddr(serial)
				dip.app.ignoredAddrs.Store(ip, true)
				if info.DeviceID != "" && info.DeviceID != "-" {
					dip.app.ignoredAddrs.Store("devid:"+info.DeviceID, true)
				}

				// sweep remaining ADB entries to disconnect mDNS aliases.
				// match by hardware device ID for precision and avoids
				// disconnecting unrelated devices that share the same model.
				remaining, _ := dip.app.adbClient.GetDevices()
				for _, d := range remaining {
					rid := dip.app.adbClient.GetDeviceID(d.Serial)
					if rid != "" && info.DeviceID != "" && info.DeviceID != "-" && rid == info.DeviceID {
						dip.app.ignoredAddrs.Store(d.Serial, true)
						_ = dip.app.adbClient.Disconnect(d.Serial)
					}
				}
			}
			dip.app.devicePanel.refreshDevices()
			dip.LoadDeviceInfo(serial)
		}()
	})
	disconnectBtn.Importance = widget.DangerImportance

	// Secondary actions
	screenshotBtn := widget.NewButtonWithIcon("Screenshot", theme.MediaPhotoIcon(), func() {
		go dip.takeScreenshot(serial)
	})

	openShellBtn := widget.NewButtonWithIcon("ADB Shell", theme.SettingsIcon(), func() {
		go func() {
			dip.app.logsPanel.Log("Opening ADB shell...")
			if err := platform.OpenTerminal(dip.app.adbClient.Path(), "-s", serial, "shell"); err != nil {
				dip.app.logsPanel.Log("[ERROR]Shell: " + err.Error())
			}
		}()
	})

	refreshInfoBtn := widget.NewButtonWithIcon("Refresh Info", theme.ViewRefreshIcon(), func() {
		go dip.LoadDeviceInfo(serial)
	})

	primaryActions := container.NewGridWithColumns(3, dip.mirrorBtn, dip.stopBtn, disconnectBtn)

	secondaryActions := container.NewGridWithColumns(3,
		screenshotBtn,
		openShellBtn,
		refreshInfoBtn,
	)

	return container.NewVBox(
		widget.NewSeparator(),
		container.NewCenter(title),
		widget.NewSeparator(),
		form,
		layout.NewSpacer(),
		widget.NewSeparator(),
		primaryActions,
		secondaryActions,
	)
}

// buildDisconnectedView creates the info panel content for a disconnected device.
func (dip *DeviceInfoPanel) buildDisconnectedView(serial string) fyne.CanvasObject {
	// Try to get the device name from known devices
	name := serial
	if dev, ok := dip.app.devicePanel.GetDevice(serial); ok && dev.Model != "" {
		name = dev.Model
	}

	title := widget.NewLabelWithStyle(name, fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	reconnecting := dip.app.devicePanel.IsReconnecting(serial)

	msg := widget.NewLabel("This device is not connected")
	if reconnecting {
		msg.SetText("Reconnecting...")
	}
	msg.TextStyle = fyne.TextStyle{Italic: true}
	msg.Alignment = fyne.TextAlignCenter

	var reconnectBtn *widget.Button
	reconnectBtn = widget.NewButtonWithIcon("Reconnect", theme.MediaReplayIcon(), func() {
		reconnectBtn.Disable()
		msg.SetText("Reconnecting...")
		dip.app.devicePanel.ReconnectDevice(serial)
	})
	reconnectBtn.Importance = widget.HighImportance
	if reconnecting {
		reconnectBtn.Disable()
	}

	removeBtn := widget.NewButtonWithIcon("Remove", theme.ContentRemoveIcon(), func() {
		dip.app.devicePanel.RemoveDevice(serial)
		dip.LoadDeviceInfo("")
	})
	removeBtn.Importance = widget.DangerImportance

	actions := container.NewGridWithColumns(2, reconnectBtn, removeBtn)

	return container.NewVBox(
		widget.NewSeparator(),
		container.NewCenter(title),
		widget.NewSeparator(),
		layout.NewSpacer(),
		container.NewCenter(msg),
		layout.NewSpacer(),
		widget.NewSeparator(),
		actions,
	)
}
