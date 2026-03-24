package ui

import (
	"context"
	"log/slog"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/adb"
	"mirroid/internal/config"
	"mirroid/internal/model"
	"mirroid/internal/scrcpy"
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
	// cleared on manual pair or app restart.
	ignoredAddrs map[string]bool

	// ui panels
	devicePanel             *DevicePanel
	optionsPanel            *OptionsPanel
	presetsPanel            *PresetsPanel
	logsPanel               *LogsPanel
	deviceInfoPanel         *DeviceInfoPanel
	autoOpenedPairingWindow fyne.Window // only set when auto-opened at first boot
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
		adbClient:    adb.NewClient(cfg.AppConf.ADBPath),
		runner:       scrcpy.NewRunner(cfg.AppConf.ScrcpyPath),
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

	mainMenu := fyne.NewMainMenu(deviceMenu, logsMenu)
	a.window.SetMainMenu(mainMenu)

	deviceSection := widget.NewCard("Device", "", a.devicePanel.Build())
	optionsSection := widget.NewCard("Options", "", a.optionsPanel.Build())
	presetsSection := widget.NewCard("Presets", "", a.presetsPanel.Build())

	launchBtn := widget.NewButtonWithIcon("Launch scrcpy", theme.MediaPlayIcon(), a.onLaunch)
	launchBtn.Importance = widget.HighImportance

	stopBtn := widget.NewButtonWithIcon("Stop All", theme.MediaStopIcon(), a.onStopAll)
	stopBtn.Importance = widget.DangerImportance

	cmdBtn := widget.NewButtonWithIcon("Preview Command", theme.DocumentIcon(), a.onShowCommand)

	buttons := container.NewGridWithColumns(3, launchBtn, stopBtn, cmdBtn)

	leftPanel := container.NewVScroll(container.NewVBox(
		deviceSection,
		optionsSection,
		presetsSection,
		layout.NewSpacer(),
		widget.NewSeparator(),
		container.NewPadded(buttons),
	))

	rightPanel := a.deviceInfoPanel.Build()

	split := container.NewHSplit(leftPanel, rightPanel)
	split.SetOffset(0.5)

	a.window.SetContent(split)

	ctx, cancel := context.WithCancel(context.Background())
	a.mdnsCancel = cancel
	go adb.WatchDevices(ctx, 1_000_000_000, func(devices []adb.MdnsDevice) {
		a.devicePanel.OnMdnsDevices(devices)
	})

	//auto-refresh device list every 3 seconds
	go a.autoRefreshDevices(ctx)

	a.window.SetOnClosed(func() {
		cancel()
		a.runner.StopAll()
	})

	go func() {
		devices, err := a.adbClient.GetDevices()
		if err != nil || len(devices) == 0 {
			fyne.Do(func() {
				// track this as auto-opened so it gets auto-closed when a device connects
				a.autoOpenedPairingWindow = ShowPairingWindow(a)
			})
		} else {
			a.devicePanel.refreshDevices()
		}
	}()

	// auto-open logs panel in debug mode
	if a.debug {
		slog.Debug("debug mode: opening logs panel")
		go func() {
			// small delay to let the main window render first
			time.Sleep(200 * time.Millisecond)
			fyne.Do(func() {
				a.logsPanel.ShowWindow()
			})
		}()
	}

	a.window.ShowAndRun()
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

// onShowCommand shows the scrcpy command in a dialog with copy.
func (a *App) onShowCommand() {
	device := a.devicePanel.SelectedDevice()
	if device == "" {
		dialog.ShowInformation("No Device", "Select a device first.", a.window)
		return
	}

	a.optionsPanel.SyncToModel(&a.options)
	preview := a.runner.CommandPreview(device, a.options)

	cmdEntry := widget.NewMultiLineEntry()
	cmdEntry.SetText(preview)
	cmdEntry.Disable()
	cmdEntry.SetMinRowsVisible(3)

	copyBtn := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		a.window.Clipboard().SetContent(preview)
	})

	content := container.NewVBox(cmdEntry, copyBtn)

	dlg := dialog.NewCustom("Command Preview", "Close", content, a.window)
	dlg.Resize(fyne.NewSize(500, 200))
	dlg.Show()
}

// onLaunch validates options and starts scrcpy.
func (a *App) onLaunch() {
	device := a.devicePanel.SelectedDevice()
	if device == "" {
		a.logsPanel.Log("[WARN]No device selected")
		return
	}

	a.optionsPanel.SyncToModel(&a.options)

	if err := a.options.Validate(); err != nil {
		a.logsPanel.Log("[WARN]" + err.Error())
		return
	}

	preview := a.runner.CommandPreview(device, a.options)
	a.logsPanel.Log(">" + preview)

	err := a.runner.Launch(device, a.options, func(line string) {
		a.logsPanel.Log(line)
	}, "")
	if err != nil {
		a.logsPanel.Log("[ERROR]" + err.Error())
	}
}

// onStopAll terminates all scrcpy processes.
func (a *App) onStopAll() {
	a.runner.StopAll()
	a.logsPanel.Log("Stopped all processes")
}
