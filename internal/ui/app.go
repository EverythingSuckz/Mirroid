package ui

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	fynetooltip "github.com/dweymouth/fyne-tooltip"

	"mirroid/internal/adb"
	"mirroid/internal/config"
	"mirroid/internal/deps"
	"mirroid/internal/model"
	"mirroid/internal/scrcpy"
	"mirroid/internal/updater"
)

// App is the top-level application struct.
type App struct {
	fyneApp    fyne.App
	window     fyne.Window
	cfg        *config.Config
	adbClient  *adb.Client
	runner     *scrcpy.Runner
	options    model.ScrcpyOptions
	mdnsCancel context.CancelFunc
	debug      bool

	// addresses explicitly disconnected by the user --mDNS won't auto-reconnect these.
	ignoredAddrs map[string]bool

	// ui panels
	devicePanel             *DevicePanel
	optionsPanel            *OptionsPanel
	presetsPanel            *PresetsPanel
	logsPanel               *LogsPanel
	deviceInfoPanel         *DeviceInfoPanel
	// layout states: empty (no devices) vs connected (has devices)
	emptyState     fyne.CanvasObject
	connectedState fyne.CanvasObject
	rootContainer  *fyne.Container

	// bottom area: swaps between options/presets and disconnected hint
	optionsContent    fyne.CanvasObject
	disconnectedHint  fyne.CanvasObject
	bottomStack       *fyne.Container
}

// NewApp creates and configures the application.
func NewApp(debug bool) *App {
	fyneApp := app.NewWithID("com.mirroid.app")
	fyneApp.Settings().SetTheme(theme.DarkTheme())

	cfg, err := config.New()
	if err != nil {
		slog.Error("failed to initialize config", "error", err)
	}

	a := &App{
		fyneApp:      fyneApp,
		window:       fyneApp.NewWindow("Mirroid"),
		cfg:          cfg,
		adbClient:    nil,
		runner:       nil,
		options:      model.DefaultOptions(),
		debug:        debug,
		ignoredAddrs: make(map[string]bool),
	}

	a.logsPanel = NewLogsPanel()
	a.logsPanel.SetApp(a)
	a.devicePanel = NewDevicePanel(a)
	a.optionsPanel = NewOptionsPanel(a)
	a.presetsPanel = NewPresetsPanel(a)
	a.deviceInfoPanel = NewDeviceInfoPanel(a)

	return a
}

// Run builds the main window UI and starts the event loop.
func (a *App) Run() {
	a.window.Resize(fyne.NewSize(900, 700))

	appDir := getAppDir()
	adbR, scrcpyR := deps.DetectAll(appDir, a.cfg.AppConf.ADBPath, a.cfg.AppConf.ScrcpyPath)

	if adbR.Found && scrcpyR.Found {
		a.initClients(adbR.Path, scrcpyR.Path)
	} else {
		a.showMissingDepsDialog(adbR, scrcpyR)
		adbPath := depFallback(adbR, a.cfg.AppConf.ADBPath, "adb")
		scrcpyPath := depFallback(scrcpyR, a.cfg.AppConf.ScrcpyPath, "scrcpy")
		a.initClients(adbPath, scrcpyPath)
	}

	exe, _ := os.Executable()
	if exe != "" {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		go updater.Cleanup(exe)
	}

	a.buildMainUI()
	a.window.ShowAndRun()
}

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
	emptyIcon.SetMinSize(fyne.NewSize(96, 96))

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
	hintIcon.SetMinSize(fyne.NewSize(64, 64))
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
	leftSplit.SetOffset(0.35)
	leftPanel := leftSplit

	rightPanel := a.deviceInfoPanel.Build()

	split := container.NewHSplit(leftPanel, rightPanel)
	split.SetOffset(0.5)

	a.connectedState = split
	a.connectedState.Hide()

	// Root: stack with both states, toggle visibility
	a.rootContainer = container.NewStack(a.emptyState, a.connectedState)
	a.window.SetContent(fynetooltip.AddWindowToolTipLayer(a.rootContainer, a.window.Canvas()))

	ctx, cancel := context.WithCancel(context.Background())
	a.mdnsCancel = cancel
	go adb.WatchDevices(ctx, 1_000_000_000, func(devices []adb.MdnsDevice) {
		a.devicePanel.OnMdnsDevices(devices)
	})

	go a.autoRefreshDevices(ctx)

	a.window.SetOnClosed(func() {
		cancel()
		a.runner.StopAll()
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

// initClients creates the adb and scrcpy clients with resolved paths and persists them.
func (a *App) initClients(adbPath, scrcpyPath string) {
	a.adbClient = adb.NewClient(adbPath)
	a.runner = scrcpy.NewRunner(scrcpyPath)
	a.runner.OnStateChange = func(serial string) {
		fyne.Do(func() {
			a.devicePanel.deviceList.Refresh()
		})
		a.deviceInfoPanel.RefreshActions()
	}

	a.cfg.AppConf.ADBPath = adbPath
	a.cfg.AppConf.ScrcpyPath = scrcpyPath
	_ = a.cfg.SaveAppConfig()
}

// getAppDir returns the directory containing the running executable.
func getAppDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// depFallback returns the detected path if found, otherwise falls back to the
// config value, and finally to the bare binary name.
func depFallback(r deps.DetectResult, configPath, bare string) string {
	if r.Found {
		return r.Path
	}
	if configPath != "" {
		return configPath
	}
	return bare
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

// autoRefreshDevices polls adb devices every 3 seconds.
func (a *App) autoRefreshDevices(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.devicePanel.refreshDevices()
		}
	}
}

// onShowCommand shows the scrcpy command(s) in a dialog with copy.
// If multiple devices are checked, shows all commands; otherwise shows the highlighted device.
func (a *App) onShowCommand() {
	a.optionsPanel.SyncToModel(&a.options)

	// Collect devices: use checked if any, else the highlighted device
	devices := a.devicePanel.SelectedDevices()
	if len(devices) == 0 {
		if d := a.devicePanel.SelectedDevice(); d != "" {
			devices = []string{d}
		}
	}
	if len(devices) == 0 {
		dialog.ShowInformation("No Device", "Select a device first.", a.window)
		return
	}

	var lines string
	for i, d := range devices {
		if i > 0 {
			lines += "\n\n"
		}
		lines += a.runner.CommandPreview(d, a.options)
	}

	cmdEntry := widget.NewMultiLineEntry()
	cmdEntry.SetText(lines)
	cmdEntry.Disable()
	cmdEntry.SetMinRowsVisible(len(devices) * 2)

	copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		a.window.Clipboard().SetContent(lines)
	})
	copyBtn.Importance = widget.HighImportance

	content := container.NewBorder(nil, copyBtn, nil, nil, cmdEntry)

	dlg := dialog.NewCustom("Command Preview", "Close", content, a.window)
	dlg.Resize(fyne.NewSize(600, float32(120+len(devices)*60)))
	dlg.Show()
}

// onLaunch validates options and starts scrcpy for all checked devices.
func (a *App) onLaunch() {
	devices := a.devicePanel.SelectedDevices()
	if len(devices) == 0 {
		a.logsPanel.Log("[WARN]No devices selected")
		return
	}

	a.optionsPanel.SyncToModel(&a.options)

	if err := a.options.Validate(); err != nil {
		a.logsPanel.Log("[WARN]" + err.Error())
		return
	}

	for _, device := range devices {
		preview := a.runner.CommandPreview(device, a.options)
		a.logsPanel.Log(">" + preview)

		err := a.runner.Launch(device, a.options, func(line string) {
			a.logsPanel.Log(line)
		}, "")
		if err != nil {
			a.logsPanel.Log("[ERROR]" + err.Error())
		}
	}
}

