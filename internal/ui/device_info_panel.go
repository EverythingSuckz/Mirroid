package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	ttwidget "github.com/dweymouth/fyne-tooltip/widget"

	"mirroid/internal/adb"
	"mirroid/internal/icons"
	"mirroid/internal/platform"
	"mirroid/internal/scrcpy"
)

var (
	pillGreen = color.NRGBA{R: 76, G: 175, B: 80, A: 255}
	pillTeal  = color.NRGBA{R: 38, G: 166, B: 154, A: 255}
	pillRed   = color.NRGBA{R: 239, G: 83, B: 80, A: 255}
	pillGray  = color.NRGBA{R: 158, G: 158, B: 158, A: 255}
)

const (
	progressBarHeight float32 = 6

	pillRadius  float32 = 4
	cardRadius  float32 = 8
	badgeRadius float32 = 15

	sectionIconSize float32 = 14
	pillIconSize    float32 = 11
	heroIconSmall   float32 = 24
	heroIconLarge   float32 = 38
	heroBoxSmall    float32 = 48
	heroBoxLarge    float32 = 50

	badgeTextSize float32 = 10
	pillTextSize  float32 = 11
	wifiDotSize   float32 = 12

	statusBadgePadX float32 = 9
	pillPadX        float32 = 8
	badgePadY       float32 = 4
)

type DeviceInfoPanel struct {
	app           *App
	container     *fyne.Container
	activity      *widget.Activity
	currentSerial string
	mirrorBtn     *widget.Button
	stopBtn       *widget.Button
	mirrorStopBox *fyne.Container

	cancelLoad context.CancelFunc // mutated only on the fyne ui thread
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

func (dip *DeviceInfoPanel) LoadDeviceInfo(serial string) {
	if serial == "" {
		fyne.Do(func() {
			dip.cancelInflightLocked()
			dip.currentSerial = ""
			dip.mirrorBtn = nil
			dip.stopBtn = nil
			dip.mirrorStopBox = nil
			dip.app.setOptionsAreaVisible(true)
			placeholder := widget.NewLabel("Select a device to view info")
			placeholder.TextStyle = fyne.TextStyle{Italic: true}
			dip.container.Objects = []fyne.CanvasObject{container.NewCenter(placeholder)}
			dip.container.Refresh()
		})
		return
	}

	if dip.app.devicePanel != nil && !dip.app.devicePanel.IsConnected(serial) {
		fyne.Do(func() {
			dip.cancelInflightLocked()
			dip.currentSerial = serial
			dip.mirrorBtn = nil
			dip.stopBtn = nil
			dip.mirrorStopBox = nil
			dip.app.setOptionsAreaVisible(false)
			dip.container.Objects = []fyne.CanvasObject{dip.buildDisconnectedView(serial)}
			dip.container.Refresh()
		})
		return
	}

	fyne.Do(func() {
		dip.cancelInflightLocked()
		ctx, cancel := context.WithCancel(context.Background())
		dip.cancelLoad = cancel
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

		go func() {
			info, err := dip.app.adbClient.GetDeviceInfo(ctx, serial)
			fyne.Do(func() {
				if dip.currentSerial != serial || ctx.Err() != nil {
					return
				}
				dip.activity.Stop()
				switch {
				case err == nil:
					dip.container.Objects = []fyne.CanvasObject{dip.buildInfoView(serial, info)}
				case errors.Is(err, context.Canceled):
					return
				default:
					log.Printf("device_info: load %s: %v", serial, err)
					dip.container.Objects = []fyne.CanvasObject{dip.buildErrorView(serial, err)}
				}
				dip.container.Refresh()
			})
		}()
	})
}

// must be called on the fyne ui thread
func (dip *DeviceInfoPanel) cancelInflightLocked() {
	if dip.cancelLoad != nil {
		dip.cancelLoad()
		dip.cancelLoad = nil
	}
}

func (dip *DeviceInfoPanel) RefreshActions() {
	fyne.Do(dip.refreshActionsLocked)
}

// must be called on the fyne ui thread
func (dip *DeviceInfoPanel) refreshActionsLocked() {
	serial := dip.currentSerial
	if serial == "" || dip.mirrorBtn == nil || dip.stopBtn == nil || dip.mirrorStopBox == nil {
		return
	}
	state := dip.app.runner.StateFor(serial)
	mirroring := state == scrcpy.StateLaunching || state == scrcpy.StateMirroring
	if mirroring {
		dip.mirrorBtn.Hide()
		dip.stopBtn.Show()
	} else {
		dip.mirrorBtn.Show()
		dip.stopBtn.Hide()
	}
}

func (dip *DeviceInfoPanel) deviceDisplayName(serial string) string {
	if dev, ok := dip.app.devicePanel.GetDevice(serial); ok && dev.Model != "" {
		return dev.Model
	}
	return serial
}

func (dip *DeviceInfoPanel) buildInfoView(serial string, info adb.DeviceInfo) fyne.CanvasObject {
	heroHeader := buildHeroHeader(info)

	refreshBtn := ttwidget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		go dip.LoadDeviceInfo(serial)
	})
	refreshBtn.Importance = widget.LowImportance
	refreshBtn.SetToolTip("Refresh device info")

	stickyTop := container.NewVBox(
		container.NewBorder(nil, nil, nil, refreshBtn, container.NewPadded(heroHeader)),
		widget.NewSeparator(),
	)

	batteryPctText := widget.NewRichText(&widget.TextSegment{
		Text: info.Battery,
		Style: widget.RichTextStyle{
			ColorName: theme.ColorNameForeground,
			SizeName:  theme.SizeNameHeadingText,
			TextStyle: fyne.TextStyle{Bold: true},
		},
	})
	batteryBar := buildThinBar(info.BatteryPct)
	batteryPills := container.NewHBox(
		buildInfoPill(icons.ZapIcon, info.BatteryStatus, batteryStatusColor(info.BatteryStatus)),
		buildInfoPill(icons.ThermometerIcon, info.BatteryTemp, pillTeal),
		buildInfoPill(icons.HeartIcon, info.BatteryHealth, pillTeal),
	)
	batteryTopRow := container.NewBorder(nil, nil, batteryPctText, nil, batteryBar)
	batteryCard := buildCard(container.NewVBox(batteryTopRow, batteryPills))
	batterySection := container.NewVBox(buildSectionLabel(icons.BatteryMediumIcon, "Battery"), batteryCard)

	storagePctDisplay := fmt.Sprintf("%.0f%%", info.StoragePct*100)
	storagePctText := widget.NewRichText(&widget.TextSegment{
		Text: storagePctDisplay,
		Style: widget.RichTextStyle{
			ColorName: theme.ColorNameForeground,
			SizeName:  theme.SizeNameHeadingText,
			TextStyle: fyne.TextStyle{Bold: true},
		},
	})
	storageBar := buildThinBar(info.StoragePct)
	storageTopRow := container.NewBorder(nil, nil, storagePctText, nil, storageBar)
	storageKV := container.NewVBox(
		buildCardKV("Used", info.StorageUsed),
		buildCardKV("Free", info.StorageFree),
		buildCardKV("Total", info.StorageTotal),
	)
	storageCard := buildCard(container.NewVBox(storageTopRow, storageKV))
	storageSection := container.NewVBox(buildSectionLabel(icons.HardDriveIcon, "Storage"), storageCard)

	displayGrid := container.NewGridWithColumns(2,
		buildStatBlock(info.Resolution, "Resolution"),
		buildStatBlock(info.DensityDisplay, "Density"),
	)
	displayCard := buildCard(displayGrid)
	displaySection := container.NewVBox(buildSectionLabel(icons.MonitorIcon, "Display"), displayCard)

	hardwareCard := buildCard(container.NewVBox(
		buildCardKV("CPU", info.CPUPlatform),
		buildCardKV("Cores", info.CPUCores),
		buildCardKV("RAM", info.RAM),
	))
	hardwareSection := container.NewVBox(buildSectionLabel(icons.CPUIcon, "Hardware"), hardwareCard)

	wifiDot := canvas.NewText("●", pillGreen)
	wifiDot.TextSize = wifiDotSize
	wifiLabel := widget.NewRichText(&widget.TextSegment{
		Text: "Wi-Fi",
		Style: widget.RichTextStyle{
			Inline:    true,
			ColorName: theme.ColorNameForeground,
		},
	})
	wifiValue := widget.NewRichText(&widget.TextSegment{
		Text: info.WifiSSID,
		Style: widget.RichTextStyle{
			Inline:    true,
			ColorName: theme.ColorNameForeground,
			Alignment: fyne.TextAlignTrailing,
		},
	})
	wifiRow := container.NewBorder(nil, nil, container.NewHBox(wifiDot, wifiLabel), nil, wifiValue)
	networkCard := buildCard(container.NewVBox(
		wifiRow,
		buildCardKV("IP Address", info.IPAddress),
	))
	networkSection := container.NewVBox(buildSectionLabel(icons.WifiIcon, "Network"), networkCard)

	systemGrid := container.NewGridWithColumns(2,
		buildStatBlock(info.AndroidDisplay, "Android"),
		buildStatBlock(info.Uptime, "Uptime"),
		buildStatBlock(info.AppCount, "Apps Installed"),
		buildStatBlock(info.BuildID, "Build"),
	)
	systemCard := buildCard(systemGrid)
	systemSection := container.NewVBox(buildSectionLabel(icons.SettingsIcon, "System"), systemCard)

	sections := container.NewVBox(
		batterySection,
		storageSection,
		displaySection,
		hardwareSection,
		networkSection,
		systemSection,
	)
	rightSpacer := canvas.NewRectangle(color.Transparent)
	rightSpacer.SetMinSize(fyne.NewSize(theme.Padding()*2, 0))
	scrollInner := container.NewBorder(nil, nil, nil, rightSpacer, container.NewPadded(sections))
	scrollContent := container.NewVScroll(scrollInner)

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
	})
	dip.mirrorBtn.Importance = widget.HighImportance

	dip.stopBtn = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), func() {
		dip.app.runner.StopFor(serial)
		dip.app.logsPanel.Log("Stopped " + serial)
	})
	dip.stopBtn.Importance = widget.DangerImportance

	state := dip.app.runner.StateFor(serial)
	if state == scrcpy.StateLaunching || state == scrcpy.StateMirroring {
		dip.mirrorBtn.Hide()
	} else {
		dip.stopBtn.Hide()
	}
	dip.mirrorStopBox = container.NewStack(dip.mirrorBtn, dip.stopBtn)

	disconnectBtn := widget.NewButtonWithIcon("Disconnect", theme.LogoutIcon(), func() {
		go func() {
			dip.app.runner.StopFor(serial)
			if err := dip.app.adbClient.Disconnect(serial); err != nil {
				dip.app.logsPanel.Log("[ERROR]Disconnect: " + err.Error())
			} else {
				dip.app.logsPanel.Log("[OK]Disconnected " + serial)
				dip.app.ignoreDevice(serial, info.DeviceID)
				dip.app.disconnectAliases(map[string]bool{info.DeviceID: true})
			}
			dip.app.devicePanel.refreshDevices() // triggers LoadDeviceInfo via the panel's refresh path
		}()
	})
	disconnectBtn.Importance = widget.DangerImportance

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

	primaryActions := container.NewGridWithColumns(2, dip.mirrorStopBox, disconnectBtn)
	secondaryActions := container.NewGridWithColumns(2, screenshotBtn, openShellBtn)
	actions := container.NewVBox(widget.NewSeparator(), primaryActions, secondaryActions)

	return container.NewBorder(stickyTop, actions, nil, nil, scrollContent)
}

func (dip *DeviceInfoPanel) buildDisconnectedView(serial string) fyne.CanvasObject {
	name := dip.deviceDisplayName(serial)
	heroArea := buildSmallHero(name, "● Disconnected", pillGray)

	reconnecting := dip.app.devicePanel.IsReconnecting(serial)

	msg := widget.NewLabel("This device is not connected")
	msg.Alignment = fyne.TextAlignCenter
	msg.TextStyle = fyne.TextStyle{Italic: true}
	if reconnecting {
		msg.SetText("Reconnecting...")
	}

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

	centerContent := container.NewCenter(container.NewVBox(heroArea, msg))
	bottomArea := container.NewVBox(widget.NewSeparator(), actions)

	return container.NewBorder(nil, bottomArea, nil, nil, centerContent)
}

func (dip *DeviceInfoPanel) buildErrorView(serial string, loadErr error) fyne.CanvasObject {
	name := dip.deviceDisplayName(serial)
	heroArea := buildSmallHero(name, "● Error", pillRed)

	msg := widget.NewLabel("Couldn't read device info — is the screen unlocked?")
	msg.Alignment = fyne.TextAlignCenter
	msg.TextStyle = fyne.TextStyle{Italic: true}
	msg.Wrapping = fyne.TextWrapWord

	detail := widget.NewLabel(loadErr.Error())
	detail.Alignment = fyne.TextAlignCenter
	detail.Wrapping = fyne.TextWrapWord
	detail.Importance = widget.LowImportance

	retryBtn := widget.NewButtonWithIcon("Retry", theme.ViewRefreshIcon(), func() {
		go dip.LoadDeviceInfo(serial)
	})
	retryBtn.Importance = widget.HighImportance

	centerContent := container.NewCenter(container.NewVBox(heroArea, msg, detail))
	bottomArea := container.NewVBox(widget.NewSeparator(), retryBtn)

	return container.NewBorder(nil, bottomArea, nil, nil, centerContent)
}
