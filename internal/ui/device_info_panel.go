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

	info       *infoView          // built once per (re)connect, refreshes update in place
	cancelLoad context.CancelFunc // mutated only on the fyne ui thread
}

// infoView holds refs to the dynamic widgets in the connected info layout so refreshes can update text/values in place instead of rebuilding the tree.
type infoView struct {
	serial string
	root   fyne.CanvasObject

	heroName    *widget.Label
	heroAddress *styledText

	batteryHeading *styledText
	batteryBar     *fyne.Container
	batteryLayout  *progressBarLayout
	batteryPills   *fyne.Container

	storageHeading *styledText
	storageBar     *fyne.Container
	storageLayout  *progressBarLayout
	storageUsed    *styledText
	storageFree    *styledText
	storageTotal   *styledText

	resolution *styledText
	density    *styledText

	cpuPlatform *styledText
	cpuCores    *styledText
	ram         *styledText

	wifiSSID *styledText
	ip       *styledText

	androidVer *styledText
	uptime     *styledText
	appCount   *styledText
	buildID    *styledText

	refreshBtn  *ttwidget.Button
	refreshSpin *rotatingIcon

	currentDeviceID string // refreshed by apply, read by click closures
}

func (v *infoView) startRefresh() {
	v.refreshBtn.Hide()
	v.refreshSpin.Show()
	v.refreshSpin.Start()
}

func (v *infoView) stopRefresh() {
	v.refreshSpin.Stop()
	v.refreshSpin.Hide()
	v.refreshBtn.Show()
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
			dip.info = nil
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
			dip.info = nil
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

		inPlace := dip.info != nil && dip.info.serial == serial
		if inPlace {
			dip.info.startRefresh()
		} else {
			dip.activity.Start()
			dip.activity.Show()
			loading := container.NewCenter(container.NewVBox(
				dip.activity,
				widget.NewLabel("Loading device info..."),
			))
			dip.container.Objects = []fyne.CanvasObject{loading}
			dip.container.Refresh()
		}

		go func() {
			info, err := dip.app.adbClient.GetDeviceInfo(ctx, serial)
			fyne.Do(func() {
				if dip.currentSerial != serial || ctx.Err() != nil {
					return
				}
				if !inPlace {
					dip.activity.Stop()
				}
				if dip.info != nil && dip.info.serial == serial {
					dip.info.stopRefresh()
				}
				switch {
				case err == nil:
					if dip.info != nil && dip.info.serial == serial {
						dip.info.apply(info)
						dip.refreshActionsLocked()
					} else {
						dip.info = dip.newInfoView(serial, info)
					}
					dip.container.Objects = []fyne.CanvasObject{dip.info.root}
					dip.container.Refresh()
				case errors.Is(err, context.Canceled):
					return
				default:
					log.Printf("device_info: load %s: %v", serial, err)
					dip.info = nil
					dip.container.Objects = []fyne.CanvasObject{dip.buildErrorView(serial, err)}
					dip.container.Refresh()
				}
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
	if dip.activity != nil {
		dip.activity.Stop()
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

func (dip *DeviceInfoPanel) newInfoView(serial string, info adb.DeviceInfo) *infoView {
	v := &infoView{serial: serial}

	heroHeader, heroName, heroAddress := buildHeroHeader()
	v.heroName = heroName
	v.heroAddress = heroAddress

	v.refreshBtn = ttwidget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		go dip.LoadDeviceInfo(serial)
	})
	v.refreshBtn.Importance = widget.LowImportance
	v.refreshBtn.SetToolTip("Refresh device info")

	v.refreshSpin = newRotatingIcon(theme.ViewRefreshIcon())
	v.refreshSpin.Hide()
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(v.refreshBtn.MinSize())
	spinSlot := container.NewStack(spacer, container.NewCenter(v.refreshSpin))
	refreshSlot := container.NewStack(v.refreshBtn, spinSlot)

	stickyTop := container.NewVBox(
		container.NewBorder(nil, nil, nil, refreshSlot, container.NewPadded(heroHeader)),
		widget.NewSeparator(),
	)

	v.batteryHeading = newStyledText("", styleHeading())
	v.batteryBar, v.batteryLayout = buildThinBar(0)
	v.batteryPills = container.NewHBox()
	batteryTopRow := container.NewBorder(nil, nil, v.batteryHeading.rt, nil, v.batteryBar)
	batteryCard := buildCard(container.NewVBox(batteryTopRow, v.batteryPills))
	batterySection := container.NewVBox(buildSectionLabel(icons.BatteryMediumIcon, "Battery"), batteryCard)

	v.storageHeading = newStyledText("", styleHeading())
	v.storageBar, v.storageLayout = buildThinBar(0)
	var usedRow, freeRow, totalRow fyne.CanvasObject
	v.storageUsed, usedRow = kvRow("Used")
	v.storageFree, freeRow = kvRow("Free")
	v.storageTotal, totalRow = kvRow("Total")
	storageTopRow := container.NewBorder(nil, nil, v.storageHeading.rt, nil, v.storageBar)
	storageCard := buildCard(container.NewVBox(storageTopRow, usedRow, freeRow, totalRow))
	storageSection := container.NewVBox(buildSectionLabel(icons.HardDriveIcon, "Storage"), storageCard)

	var resolutionBlock, densityBlock fyne.CanvasObject
	v.resolution, resolutionBlock = statBlock("Resolution")
	v.density, densityBlock = statBlock("Density")
	displayCard := buildCard(container.NewGridWithColumns(2, resolutionBlock, densityBlock))
	displaySection := container.NewVBox(buildSectionLabel(icons.MonitorIcon, "Display"), displayCard)

	var cpuRow, coresRow, ramRow fyne.CanvasObject
	v.cpuPlatform, cpuRow = kvRow("CPU")
	v.cpuCores, coresRow = kvRow("Cores")
	v.ram, ramRow = kvRow("RAM")
	hardwareCard := buildCard(container.NewVBox(cpuRow, coresRow, ramRow))
	hardwareSection := container.NewVBox(buildSectionLabel(icons.CPUIcon, "Hardware"), hardwareCard)

	wifiDot := canvas.NewText("●", pillGreen)
	wifiDot.TextSize = wifiDotSize
	wifiLabel := newStyledText("Wi-Fi", styleKey())
	v.wifiSSID = newStyledText("", styleValue())
	wifiRow := container.NewBorder(nil, nil, container.NewHBox(wifiDot, wifiLabel.rt), nil, v.wifiSSID.rt)
	var ipAddrRow fyne.CanvasObject
	v.ip, ipAddrRow = kvRow("IP Address")
	networkCard := buildCard(container.NewVBox(wifiRow, ipAddrRow))
	networkSection := container.NewVBox(buildSectionLabel(icons.WifiIcon, "Network"), networkCard)

	var androidBlock, uptimeBlock, appsBlock, buildBlock fyne.CanvasObject
	v.androidVer, androidBlock = statBlock("Android")
	v.uptime, uptimeBlock = statBlock("Uptime")
	v.appCount, appsBlock = statBlock("Apps Installed")
	v.buildID, buildBlock = statBlockTruncating("Build")
	systemCard := buildCard(container.NewGridWithColumns(2, androidBlock, uptimeBlock, appsBlock, buildBlock))
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
			deviceID := v.currentDeviceID // read at click time so a refreshed value wins over the initial one
			if err := dip.app.adbClient.Disconnect(serial); err != nil {
				dip.app.logsPanel.Log("[ERROR]Disconnect: " + err.Error())
			} else {
				dip.app.logsPanel.Log("[OK]Disconnected " + serial)
				if deviceID != "" && deviceID != "-" {
					dip.app.ignoreDevice(serial, deviceID)
					dip.app.disconnectAliases(map[string]bool{deviceID: true})
				}
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

	v.root = container.NewBorder(stickyTop, actions, nil, nil, scrollContent)
	v.apply(info)
	return v
}

func (v *infoView) apply(info adb.DeviceInfo) {
	v.currentDeviceID = info.DeviceID
	v.heroName.SetText(fmt.Sprintf("%s %s", info.Manufacturer, info.Model))
	v.heroAddress.Set(info.Serial)

	batteryDisplay := "-"
	if info.BatteryPct >= 0 {
		batteryDisplay = fmt.Sprintf("%d%%", int(info.BatteryPct*100))
	}
	v.batteryHeading.Set(batteryDisplay)
	v.batteryLayout.pct = info.BatteryPct
	v.batteryBar.Refresh()

	v.batteryPills.Objects = []fyne.CanvasObject{
		buildInfoPill(icons.ZapIcon, info.BatteryStatus, batteryStatusColor(info.BatteryStatus)),
		buildInfoPill(icons.ThermometerIcon, info.BatteryTemp, pillTeal),
		buildInfoPill(icons.HeartIcon, info.BatteryHealth, pillTeal),
	}
	v.batteryPills.Refresh()

	v.storageHeading.Set(fmt.Sprintf("%.0f%%", info.StoragePct*100))
	v.storageLayout.pct = info.StoragePct
	v.storageBar.Refresh()
	v.storageUsed.Set(info.StorageUsed)
	v.storageFree.Set(info.StorageFree)
	v.storageTotal.Set(info.StorageTotal)

	v.resolution.Set(info.Resolution)
	v.density.Set(info.DensityDisplay)

	v.cpuPlatform.Set(info.CPUPlatform)
	v.cpuCores.Set(info.CPUCores)
	v.ram.Set(info.RAM)

	v.wifiSSID.Set(info.WifiSSID)
	v.ip.Set(info.IPAddress)

	v.androidVer.Set(info.AndroidDisplay)
	v.uptime.Set(info.Uptime)
	v.appCount.Set(info.AppCount)
	v.buildID.Set(info.BuildID)
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
