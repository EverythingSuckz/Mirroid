package ui

import (
	"context"
	"log/slog"
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
	ignoredAddrs map[string]bool

	// ui panels
	devicePanel             *DevicePanel
	optionsPanel            *OptionsPanel
	presetsPanel            *PresetsPanel
	logsPanel               *LogsPanel
	deviceInfoPanel         *DeviceInfoPanel
	autoOpenedPairingWindow fyne.Window // only set when auto-opened at first boot

	// layout states: empty (no devices) vs connected (has devices)
	emptyState     fyne.CanvasObject
	connectedState fyne.CanvasObject
	rootContainer  *fyne.Container
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

	// Wire reactive state updates: when a scrcpy process starts or exits,
	// refresh the device table's Status column and the info panel's buttons.
	a.runner.OnStateChange = func(serial string) {
		fyne.Do(func() {
			a.devicePanel.deviceList.Refresh()
		})
		a.deviceInfoPanel.RefreshActions()
	}

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
	optionsSection := widget.NewCard("Options", "", a.optionsPanel.Build())
	presetsSection := widget.NewCard("Presets", "", a.presetsPanel.Build())

	topArea := deviceSection
	bottomArea := container.NewVScroll(container.NewVBox(
		optionsSection,
		presetsSection,
	))

	leftSplit := container.NewVSplit(topArea, bottomArea)
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

	go func() {
		devices, err := a.adbClient.GetDevices()
		if err != nil || len(devices) == 0 {
			fyne.Do(func() {
				a.autoOpenedPairingWindow = ShowPairingWindow(a)
			})
		} else {
			a.devicePanel.refreshDevices()
		}
	}()

	if a.debug {
		slog.Debug("debug mode: opening logs panel")
		go func() {
			time.Sleep(200 * time.Millisecond)
			fyne.Do(func() {
				a.logsPanel.ShowWindow()
			})
		}()
	}

	a.window.ShowAndRun()
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

