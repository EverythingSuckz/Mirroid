package ui

import (
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/platform"
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

// NewDeviceInfoPanel creates a new device info panel.
func NewDeviceInfoPanel(app *App) *DeviceInfoPanel {
	return &DeviceInfoPanel{app: app}
}

// Build creates the info panel UI.
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
			placeholder := widget.NewLabel("Select a device to view info")
			placeholder.TextStyle = fyne.TextStyle{Italic: true}
			dip.container.Objects = []fyne.CanvasObject{container.NewCenter(placeholder)}
			dip.container.Refresh()
		})
		return
	}

	fyne.Do(func() {
		dip.currentSerial = serial
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
		info := dip.fetchDeviceInfo(serial)
		fyne.Do(func() {
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
		running := dip.app.runner.IsRunningFor(serial)
		if running {
			dip.stopBtn.Enable()
		} else {
			dip.stopBtn.Disable()
		}
	})
}

type deviceInfo struct {
	Model          string
	Manufacturer   string
	AndroidVersion string
	SDK            string
	BuildID        string
	Serial         string
	Resolution     string
	Density        string
	Battery        string
}

func (dip *DeviceInfoPanel) adbPath() string {
	if dip.app.cfg != nil && dip.app.cfg.AppConf.ADBPath != "" {
		return dip.app.cfg.AppConf.ADBPath
	}
	return "adb"
}

func (dip *DeviceInfoPanel) fetchDeviceInfo(serial string) deviceInfo {
	adb := dip.adbPath()

	getProp := func(prop string) string {
		cmd := exec.Command(adb, "-s", serial, "shell", "getprop", prop)
		platform.HideConsole(cmd)
		out, err := cmd.Output()
		if err != nil {
			return "-"
		}
		return strings.TrimSpace(string(out))
	}

	battCmd := exec.Command(adb, "-s", serial, "shell", "dumpsys", "battery")
	platform.HideConsole(battCmd)
	battOut, _ := battCmd.Output()
	battery := "-"
	for _, line := range strings.Split(string(battOut), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "level:") {
			battery = strings.TrimPrefix(line, "level:") + "%"
			battery = strings.TrimSpace(battery)
			break
		}
	}

	resCmd := exec.Command(adb, "-s", serial, "shell", "wm", "size")
	platform.HideConsole(resCmd)
	resOut, _ := resCmd.Output()
	resolution := "-"
	resParts := strings.Split(strings.TrimSpace(string(resOut)), ":")
	if len(resParts) == 2 {
		resolution = strings.TrimSpace(resParts[1])
	}

	return deviceInfo{
		Model:          getProp("ro.product.model"),
		Manufacturer:   getProp("ro.product.manufacturer"),
		AndroidVersion: getProp("ro.build.version.release"),
		SDK:            getProp("ro.build.version.sdk"),
		BuildID:        getProp("ro.build.display.id"),
		Serial:         serial,
		Resolution:     resolution,
		Density:        getProp("ro.sf.lcd_density"),
		Battery:        battery,
	}
}

func (dip *DeviceInfoPanel) buildInfoView(serial string, info deviceInfo) fyne.CanvasObject {
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
	if !dip.app.runner.IsRunningFor(serial) {
		dip.stopBtn.Disable()
	}

	disconnectBtn := widget.NewButtonWithIcon("Disconnect", theme.LogoutIcon(), func() {
		go func() {
			dip.app.runner.StopFor(serial)
			if err := dip.app.adbClient.Disconnect(serial); err != nil {
				dip.app.logsPanel.Log("[ERROR]Disconnect: " + err.Error())
			} else {
				dip.app.logsPanel.Log("[OK]Disconnected " + serial)
				// Block this device from reconnecting via IP, model name,
				// and serial (covers mDNS alias serials too).
				dip.app.ignoredAddrs[serial] = true
				ip := serial
				if idx := strings.Index(ip, ":"); idx > 0 {
					ip = ip[:idx]
				}
				dip.app.ignoredAddrs[ip] = true

				if info.Model != "" {
					// Store both space and underscore variants
					dip.app.ignoredAddrs[info.Model] = true
					dip.app.ignoredAddrs[strings.ReplaceAll(info.Model, " ", "_")] = true
				}

				// Also disconnect any other ADB entries for the same device
				// (catches mDNS aliases like adb-xxx._adb-tls-connect._tcp)
				remaining, _ := dip.app.adbClient.GetDevices()
				for _, d := range remaining {
					if d.Model != "" && (d.Model == info.Model || d.Model == strings.ReplaceAll(info.Model, " ", "_")) {
						dip.app.ignoredAddrs[d.Serial] = true
						_ = dip.app.adbClient.Disconnect(d.Serial)
					}
				}
			}
			dip.app.devicePanel.refreshDevices()
			dip.LoadDeviceInfo("")
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
			cmd := exec.Command("cmd", "/c", "start", "cmd", "/k", dip.adbPath(), "-s", serial, "shell")
			if err := cmd.Start(); err != nil {
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

// takeScreenshot captures the screen, pulls to temp, and shows preview window.
func (dip *DeviceInfoPanel) takeScreenshot(serial string) {
	dip.app.logsPanel.Log("Taking screenshot...")
	adb := dip.adbPath()

	capCmd := exec.Command(adb, "-s", serial, "shell", "screencap", "-p", "/sdcard/mirroid_screenshot.png")
	platform.HideConsole(capCmd)
	out, err := capCmd.CombinedOutput()
	if err != nil {
		dip.app.logsPanel.Log("[ERROR]Screenshot failed: " + string(out))
		return
	}

	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "mirroid_screenshot.png")

	pullCmd := exec.Command(adb, "-s", serial, "pull", "/sdcard/mirroid_screenshot.png", tmpFile)
	platform.HideConsole(pullCmd)
	pullOut, err := pullCmd.CombinedOutput()
	if err != nil {
		dip.app.logsPanel.Log("[ERROR]Pull failed: " + string(pullOut))
		return
	}

	cleanCmd := exec.Command(adb, "-s", serial, "shell", "rm", "/sdcard/mirroid_screenshot.png")
	platform.HideConsole(cleanCmd)
	cleanCmd.Run()

	dip.app.logsPanel.Log("[OK]Screenshot captured")

	f, err := os.Open(tmpFile)
	if err != nil {
		dip.app.logsPanel.Log("[ERROR]Failed to open screenshot: " + err.Error())
		return
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		dip.app.logsPanel.Log("[ERROR]Failed to decode screenshot: " + err.Error())
		return
	}

	fyne.Do(func() {
		screenshotWin := dip.app.fyneApp.NewWindow("Screenshot")
		screenshotWin.Resize(fyne.NewSize(500, 700))

		imgWidget := canvas.NewImageFromImage(img)
		imgWidget.FillMode = canvas.ImageFillContain
		imgWidget.SetMinSize(fyne.NewSize(400, 600))

		copyBtn := widget.NewButtonWithIcon("Copy to Clipboard", theme.ContentCopyIcon(), func() {
			go func() {
				// use PowerShell to copy image to clipboard
				psCmd := exec.Command("powershell", "-NoProfile", "-Command",
					fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.Clipboard]::SetImage([System.Drawing.Image]::FromFile('%s'))`, tmpFile))
				platform.HideConsole(psCmd)
				if err := psCmd.Run(); err != nil {
					dip.app.logsPanel.Log("[ERROR]Copy failed: " + err.Error())
				} else {
					dip.app.logsPanel.Log("[OK]Screenshot copied to clipboard")
				}
			}()
		})

		saveBtn := widget.NewButtonWithIcon("Save As...", theme.DocumentSaveIcon(), func() {
			fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
				if err != nil || writer == nil {
					return
				}
				defer writer.Close()

				data, readErr := os.ReadFile(tmpFile)
				if readErr != nil {
					dip.app.logsPanel.Log("[ERROR]Read failed: " + readErr.Error())
					return
				}
				if _, writeErr := writer.Write(data); writeErr != nil {
					dip.app.logsPanel.Log("[ERROR]Save failed: " + writeErr.Error())
					return
				}
				dip.app.logsPanel.Log("[OK]Screenshot saved to " + writer.URI().Path())
			}, screenshotWin)
			fd.SetFilter(storage.NewExtensionFileFilter([]string{".png"}))
			fd.SetFileName("screenshot.png")
			fd.Show()
		})
		saveBtn.Importance = widget.HighImportance

		toolbar := container.NewHBox(layout.NewSpacer(), copyBtn, saveBtn, layout.NewSpacer())

		screenshotWin.SetContent(container.NewBorder(
			nil,
			container.NewPadded(toolbar),
			nil, nil,
			container.NewPadded(imgWidget),
		))
		screenshotWin.Show()
	})
}
