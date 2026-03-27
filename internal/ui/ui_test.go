package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"mirroid/internal/adb"
	"mirroid/internal/config"
	"mirroid/internal/model"
	"mirroid/internal/scrcpy"
)

func TestOptionsPanelBuild(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	app := &App{
		fyneApp: a,
		window:  a.NewWindow("Test"),
		options: model.DefaultOptions(),
	}
	app.logsPanel = NewLogsPanel()
	app.optionsPanel = NewOptionsPanel(app)

	obj := app.optionsPanel.Build()
	if obj == nil {
		t.Fatal("OptionsPanel.Build() returned nil")
	}
}

func TestOptionsPanelSyncRoundTrip(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	app := &App{
		fyneApp: a,
		window:  a.NewWindow("Test"),
		options: model.DefaultOptions(),
	}
	app.logsPanel = NewLogsPanel()
	app.optionsPanel = NewOptionsPanel(app)
	app.optionsPanel.Build()

	custom := model.ScrcpyOptions{
		Bitrate:       16,
		MaxSize:       1920,
		MaxFPS:        60,
		Codec:         "h265",
		AudioEnabled:  false,
		AudioSource:   "mic",
		Fullscreen:    true,
		Borderless:    true,
		AlwaysOnTop:   true,
		Rotation:      2,
		TurnScreenOff: true,
		RecordFile:    "test.mp4",
		WindowTitle:   "Test Title",
		ClipboardSync: false,
		StayAwake:     true,
		HIDKeyboard:   true,
		HIDMouse:      true,
		VideoSource:   "camera",
	}
	app.optionsPanel.SyncFromModel(custom)

	var result model.ScrcpyOptions
	app.optionsPanel.SyncToModel(&result)

	if result.Bitrate != 16 {
		t.Errorf("Bitrate: got %d, want 16", result.Bitrate)
	}
	if result.MaxSize != 1920 {
		t.Errorf("MaxSize: got %d, want 1920", result.MaxSize)
	}
	if result.MaxFPS != 60 {
		t.Errorf("MaxFPS: got %d, want 60", result.MaxFPS)
	}
	if result.Codec != "h265" {
		t.Errorf("Codec: got %s, want h265", result.Codec)
	}
	if result.AudioEnabled {
		t.Error("AudioEnabled should be false")
	}
	if result.AudioSource != "mic" {
		t.Errorf("AudioSource: got %s, want mic", result.AudioSource)
	}
	if !result.Fullscreen {
		t.Error("Fullscreen should be true")
	}
	if !result.Borderless {
		t.Error("Borderless should be true")
	}
	if !result.AlwaysOnTop {
		t.Error("AlwaysOnTop should be true")
	}
	if result.Rotation != 2 {
		t.Errorf("Rotation: got %d, want 2", result.Rotation)
	}
	if !result.TurnScreenOff {
		t.Error("TurnScreenOff should be true")
	}
	if result.RecordFile != "test.mp4" {
		t.Errorf("RecordFile: got %s, want test.mp4", result.RecordFile)
	}
	if result.WindowTitle != "Test Title" {
		t.Errorf("WindowTitle: got %s, want Test Title", result.WindowTitle)
	}
	if result.ClipboardSync {
		t.Error("ClipboardSync should be false")
	}
	if !result.StayAwake {
		t.Error("StayAwake should be true")
	}
	if !result.HIDKeyboard {
		t.Error("HIDKeyboard should be true")
	}
	if !result.HIDMouse {
		t.Error("HIDMouse should be true")
	}
	if result.VideoSource != "camera" {
		t.Errorf("VideoSource: got %s, want camera", result.VideoSource)
	}
}

func TestOptionsPanelDefaultValues(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	app := &App{
		fyneApp: a,
		window:  a.NewWindow("Test"),
		options: model.DefaultOptions(),
	}
	app.logsPanel = NewLogsPanel()
	app.optionsPanel = NewOptionsPanel(app)
	app.optionsPanel.Build()

	var result model.ScrcpyOptions
	app.optionsPanel.SyncToModel(&result)

	if result.Bitrate != 8 {
		t.Errorf("Default Bitrate: got %d, want 8", result.Bitrate)
	}
	if result.Codec != "h264" {
		t.Errorf("Default Codec: got %s, want h264", result.Codec)
	}
	if result.AudioEnabled {
		t.Error("Default AudioEnabled should be false")
	}
	if !result.ClipboardSync {
		t.Error("Default ClipboardSync should be true")
	}
	if result.VideoSource != "display" {
		t.Errorf("Default VideoSource: got %s, want display", result.VideoSource)
	}
}

func TestLogsPanelLog(t *testing.T) {
	lp := NewLogsPanel()

	lp.Log("Hello")
	lp.Log("World")

	if len(lp.logLines) != 2 {
		t.Errorf("Expected 2 log lines, got %d", len(lp.logLines))
	}

	content := lp.GetContent()
	if !strings.Contains(content, "Hello") || !strings.Contains(content, "World") {
		t.Errorf("GetContent should contain both lines, got: %s", content)
	}
}

func TestLogsPanelClear(t *testing.T) {
	lp := NewLogsPanel()
	lp.Log("test")
	lp.Clear()

	if len(lp.logLines) != 0 {
		t.Errorf("Expected 0 log lines after clear, got %d", len(lp.logLines))
	}
	if lp.GetContent() != "" {
		t.Error("GetContent should be empty after clear")
	}
}

func TestDevicePanelBuild(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	app := &App{
		fyneApp: a,
		window:  a.NewWindow("Test"),
		options: model.DefaultOptions(),
		runner:  scrcpy.NewRunner(""),
	}
	app.logsPanel = NewLogsPanel()
	app.deviceInfoPanel = NewDeviceInfoPanel(app)

	dp := NewDevicePanel(app)
	if dp == nil {
		t.Fatal("NewDevicePanel returned nil")
	}
	if dp.app != app {
		t.Error("DevicePanel.app not set correctly")
	}

	obj := dp.Build()
	if obj == nil {
		t.Fatal("DevicePanel.Build() returned nil")
	}

	// No devices — SelectedDevice should return empty
	if got := dp.SelectedDevice(); got != "" {
		t.Errorf("SelectedDevice with no devices: got %q, want empty", got)
	}

	// No devices — SelectedDevices should return empty
	if got := dp.SelectedDevices(); len(got) != 0 {
		t.Errorf("SelectedDevices with no devices: got %v, want empty", got)
	}
}

func TestDevicePanelMultiSelect(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	app := &App{
		fyneApp: a,
		window:  a.NewWindow("Test"),
		options: model.DefaultOptions(),
		runner:  scrcpy.NewRunner(""),
	}
	app.logsPanel = NewLogsPanel()
	app.deviceInfoPanel = NewDeviceInfoPanel(app)

	dp := NewDevicePanel(app)
	dp.Build()

	// Inject test devices and check two of them
	dp.mu.Lock()
	dp.devices = []adb.Device{
		{Serial: "192.168.1.5:5555", Model: "Pixel_7", Source: "wireless"},
		{Serial: "ABCD1234", Model: "Galaxy_S24", Source: "usb"},
	}
	dp.checkedSerials["192.168.1.5:5555"] = true
	dp.checkedSerials["ABCD1234"] = true
	dp.mu.Unlock()

	got := dp.SelectedDevices()
	if len(got) != 2 {
		t.Errorf("SelectedDevices: got %d serials, want 2", len(got))
	}

	// Uncheck one
	dp.mu.Lock()
	delete(dp.checkedSerials, "ABCD1234")
	dp.mu.Unlock()

	got = dp.SelectedDevices()
	if len(got) != 1 {
		t.Errorf("SelectedDevices after uncheck: got %d serials, want 1", len(got))
	}
	if len(got) == 1 && got[0] != "192.168.1.5:5555" {
		t.Errorf("SelectedDevices: got %q, want 192.168.1.5:5555", got[0])
	}
}

func TestPresetsPanelBuild(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()

	cfg, err := config.New()
	if err != nil {
		t.Skipf("Could not create config: %v", err)
	}

	app := &App{
		fyneApp: a,
		window:  a.NewWindow("Test"),
		options: model.DefaultOptions(),
		cfg:     cfg,
	}
	app.logsPanel = NewLogsPanel()
	app.optionsPanel = NewOptionsPanel(app)
	app.optionsPanel.Build()
	app.presetsPanel = NewPresetsPanel(app)

	obj := app.presetsPanel.Build()
	if obj == nil {
		t.Fatal("PresetsPanel.Build() returned nil")
	}
}
