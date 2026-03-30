package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/adb"
	"mirroid/internal/config"
	"mirroid/internal/deps"
	"mirroid/internal/model"
	"mirroid/internal/scrcpy"
	"mirroid/internal/updater"
)

const (
	defaultWindowWidth  = 900
	defaultWindowHeight = 700
	emptyStateIconSize  = 96
	hintIconSize        = 64
	leftSplitOffset     = 0.35
	mainSplitOffset     = 0.5
)

type App struct {
	fyneApp   fyne.App
	window    fyne.Window
	cfg       *config.Config
	adbClient *adb.Client
	runner    *scrcpy.Runner
	options   model.ScrcpyOptions
	debug     bool

	// addresses explicitly disconnected by the user -- mDNS won't auto-reconnect these.
	ignoredAddrs sync.Map

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

	// theme
	themeManager     *ThemeManager
	themeSystemItem  *fyne.MenuItem
	themeDarkItem    *fyne.MenuItem
	themeLightItem   *fyne.MenuItem
}

func (a *App) isIgnored(key string) bool {
	_, ok := a.ignoredAddrs.Load(key)
	return ok
}

// ignoreDevice stores the device's serial, host, and hardware ID in ignoredAddrs.
func (a *App) ignoreDevice(serial, devID string) {
	a.ignoredAddrs.Store(serial, true)
	if host := parseHostFromAddr(serial); host != serial {
		a.ignoredAddrs.Store(host, true)
	}
	if devID != "" && devID != "-" {
		a.ignoredAddrs.Store("devid:"+devID, true)
	}
}

// disconnectAliases disconnects remaining ADB entries that share a hardware
// device ID with the given set, and stores their serials in ignoredAddrs.
func (a *App) disconnectAliases(devIDs map[string]bool) {
	if len(devIDs) == 0 {
		return
	}
	remaining, _ := a.adbClient.GetDevices()
	for _, d := range remaining {
		rid := a.adbClient.GetDeviceID(d.Serial)
		if rid != "" && devIDs[rid] {
			a.ignoredAddrs.Store(d.Serial, true)
			_ = a.adbClient.Disconnect(d.Serial)
		}
	}
}

func NewApp(debug bool) (*App, error) {
	fyneApp := app.NewWithID("com.mirroid.app")

	cfg, err := config.New()
	if err != nil {
		return nil, fmt.Errorf("initialize config: %w", err)
	}

	a := &App{
		fyneApp: fyneApp,
		window:  fyneApp.NewWindow("Mirroid"),
		cfg:     cfg,
		options: model.DefaultOptions(),
		debug:   debug,
	}

	a.themeManager = NewThemeManager(a.fyneApp, a.cfg, a.window)

	a.logsPanel = NewLogsPanel()
	a.logsPanel.SetApp(a)
	a.devicePanel = NewDevicePanel(a)
	a.optionsPanel = NewOptionsPanel(a)
	a.presetsPanel = NewPresetsPanel(a)
	a.deviceInfoPanel = NewDeviceInfoPanel(a)

	return a, nil
}

func (a *App) Run() {
	a.window.Resize(fyne.NewSize(defaultWindowWidth, defaultWindowHeight))

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

	var sb strings.Builder
	for i, d := range devices {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(a.runner.CommandPreview(d, a.options))
	}
	lines := sb.String()

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

