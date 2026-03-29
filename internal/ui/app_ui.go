package ui

import (
	"context"
	"log/slog"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	fynetooltip "github.com/dweymouth/fyne-tooltip"

	"mirroid/internal/adb"
	"mirroid/internal/deps"
)

// buildMainUI sets up the menu, panels, device refresh goroutines, and window content.
func (a *App) buildMainUI() {
	logsMenu := fyne.NewMenu("Logs",
		fyne.NewMenuItem("View Logs", func() {
			a.logsPanel.ShowWindow()
		}),
		fyne.NewMenuItem("Clear Logs", func() {
			a.logsPanel.Clear()
		}),
	)

	deviceMenu := fyne.NewMenu("Device",
		fyne.NewMenuItem("Pair New Device", func() {
			ShowPairingWindow(a)
		}),
		fyne.NewMenuItem("Refresh Devices", func() {
			go a.devicePanel.refreshDevices()
		}),
	)

	aboutMenu := fyne.NewMenu("About",
		fyne.NewMenuItem("About Mirroid", func() {
			a.showAboutDialog()
		}),
		fyne.NewMenuItem("Check for Updates", func() {
			go a.checkForUpdates(false)
		}),
	)

	mainMenu := fyne.NewMainMenu(deviceMenu, logsMenu, aboutMenu)
	a.window.SetMainMenu(mainMenu)

	// Empty state
	emptyIcon := canvas.NewImageFromResource(theme.ComputerIcon())
	emptyIcon.FillMode = canvas.ImageFillContain
	emptyIcon.SetMinSize(fyne.NewSize(emptyStateIconSize, emptyStateIconSize))

	emptyTitle := canvas.NewText("No devices found", theme.Color(theme.ColorNameForeground))
	emptyTitle.TextSize = 24
	emptyTitle.TextStyle = fyne.TextStyle{Bold: true}
	emptyTitle.Alignment = fyne.TextAlignCenter

	emptySubtitle := widget.NewLabelWithStyle(
		"Connect a device via USB or pair wirelessly to get started.",
		fyne.TextAlignCenter, fyne.TextStyle{})

	pairBtn := widget.NewButton("Pair New Device", func() {
		ShowPairingWindow(a)
	})
	pairBtn.Importance = widget.HighImportance

	refreshBtn := widget.NewButton("Refresh", func() {
		go a.devicePanel.refreshDevices()
	})
	refreshBtn.Importance = widget.MediumImportance

	a.emptyState = container.NewCenter(
		container.NewVBox(
			container.NewCenter(emptyIcon),
			container.NewCenter(emptyTitle),
			container.NewCenter(emptySubtitle),
			widget.NewSeparator(),
			container.NewCenter(container.NewHBox(pairBtn, refreshBtn)),
		),
	)

	// Connected state
	deviceSection := widget.NewCard("Devices", "", a.devicePanel.Build())

	topArea := deviceSection

	// Bottom area: options with inline preset controls in the header
	optionsTitle := widget.NewLabelWithStyle("Options", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	presetControls := a.presetsPanel.Build()
	optionsHeader := container.NewBorder(nil, nil, optionsTitle, presetControls)
	optionsTabs := a.optionsPanel.Build()
	optionsSection := container.NewVBox(optionsHeader, widget.NewSeparator(), optionsTabs)

	a.optionsContent = container.NewVScroll(optionsSection)

	// Disconnected hint (shown when selected device is offline)
	hintIcon := canvas.NewImageFromResource(theme.ComputerIcon())
	hintIcon.FillMode = canvas.ImageFillContain
	hintIcon.SetMinSize(fyne.NewSize(hintIconSize, hintIconSize))
	hintLabel := widget.NewLabel("Select a connected device")
	hintLabel.TextStyle = fyne.TextStyle{Italic: true}
	hintLabel.Alignment = fyne.TextAlignCenter
	a.disconnectedHint = container.NewCenter(container.NewVBox(
		hintIcon,
		hintLabel,
	))
	a.disconnectedHint.Hide()

	a.bottomStack = container.NewStack(a.optionsContent, a.disconnectedHint)

	leftSplit := container.NewVSplit(topArea, a.bottomStack)
	leftSplit.SetOffset(leftSplitOffset)
	leftPanel := leftSplit

	rightPanel := a.deviceInfoPanel.Build()

	split := container.NewHSplit(leftPanel, rightPanel)
	split.SetOffset(mainSplitOffset)

	a.connectedState = split
	a.connectedState.Hide()

	// Root: stack with both states, toggle visibility
	a.rootContainer = container.NewStack(a.emptyState, a.connectedState)
	a.window.SetContent(fynetooltip.AddWindowToolTipLayer(a.rootContainer, a.window.Canvas()))

	ctx, cancel := context.WithCancel(context.Background())
	go adb.WatchDevices(ctx, 1_000_000_000, func(devices []adb.MdnsDevice) {
		a.devicePanel.OnMdnsDevices(devices)
	})

	go a.autoRefreshDevices(ctx)

	a.window.SetOnClosed(func() {
		cancel()
		if a.runner != nil {
			a.runner.StopAll()
		}
	})

	go a.devicePanel.refreshDevices()

	if a.debug {
		slog.Debug("debug mode: opening logs panel")
		go func() {
			time.Sleep(200 * time.Millisecond)
			fyne.Do(func() {
				a.logsPanel.ShowWindow()
			})
		}()
	}

	// Auto-check for updates on startup (with 12-hour cooldown)
	if a.cfg.AppConf.AutoCheckUpdates {
		go func() {
			time.Sleep(5 * time.Second)
			if a.shouldAutoCheck() {
				a.checkForUpdates(true) // timestamp saved inside on success
			}
		}()
	}
}

// setConnectedLayout toggles between empty state and connected (split) layout.
func (a *App) setConnectedLayout(connected bool) {
	if a.emptyState == nil || a.connectedState == nil {
		return
	}
	if connected {
		a.emptyState.Hide()
		a.connectedState.Show()
	} else {
		a.connectedState.Hide()
		a.emptyState.Show()
	}
}

// setOptionsAreaVisible toggles between the options/presets panels and
// the "Select a connected device" hint in the left bottom area.
func (a *App) setOptionsAreaVisible(show bool) {
	if a.bottomStack == nil {
		return
	}
	if show {
		a.disconnectedHint.Hide()
		a.optionsContent.Show()
	} else {
		a.optionsContent.Hide()
		a.disconnectedHint.Show()
	}
}

// showMissingDepsDialog shows an informational dialog listing missing dependencies.
func (a *App) showMissingDepsDialog(adbR, scrcpyR deps.DetectResult) {
	var msg string
	msg = "Some dependencies could not be found:\n\n"
	if !adbR.Found {
		msg += "  • adb — https://developer.android.com/tools/releases/platform-tools\n"
	}
	if !scrcpyR.Found {
		msg += "  • scrcpy — https://github.com/Genymobile/scrcpy/releases\n"
	}
	msg += "\nInstall them and add to PATH, or reinstall Mirroid using the full installer."

	content := widget.NewLabel(msg)
	content.Wrapping = fyne.TextWrapWord

	dlg := dialog.NewCustom("Missing Dependencies", "Close", content, a.window)
	dlg.Resize(fyne.NewSize(500, 250))
	dlg.Show()
}
