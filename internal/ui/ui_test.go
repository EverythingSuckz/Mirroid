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

	// No devices - SelectedDevice should return empty
	if got := dp.SelectedDevice(); got != "" {
		t.Errorf("SelectedDevice with no devices: got %q, want empty", got)
	}

	// No devices - SelectedDevices should return empty
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

	// Inject test devices (both connected) and check two of them
	dp.mu.Lock()
	dp.devices = []adb.Device{
		{Serial: "192.168.1.5:5555", Model: "Pixel_7", Source: "wireless"},
		{Serial: "ABCD1234", Model: "Galaxy_S24", Source: "usb"},
	}
	dp.connectedSet["192.168.1.5:5555"] = true
	dp.connectedSet["ABCD1234"] = true
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

	// Test SelectedDisconnectedDevices: mark one device as disconnected
	dp.mu.Lock()
	delete(dp.connectedSet, "192.168.1.5:5555")
	dp.checkedSerials["192.168.1.5:5555"] = true
	dp.mu.Unlock()

	// SelectedDevices should no longer include the disconnected device
	got = dp.SelectedDevices()
	if len(got) != 0 {
		t.Errorf("SelectedDevices with disconnected: got %d serials, want 0", len(got))
	}

	// SelectedDisconnectedDevices should include it
	gotDisc := dp.SelectedDisconnectedDevices()
	if len(gotDisc) != 1 {
		t.Errorf("SelectedDisconnectedDevices: got %d serials, want 1", len(gotDisc))
	}
	if len(gotDisc) == 1 && gotDisc[0] != "192.168.1.5:5555" {
		t.Errorf("SelectedDisconnectedDevices: got %q, want 192.168.1.5:5555", gotDisc[0])
	}
}

func TestPairedSerialMatch(t *testing.T) {
	tests := []struct {
		serial, ip, guid string
		want             bool
	}{
		{"192.168.2.48:40001", "192.168.2.48", "adb-RZ8M81AB-aBcDeF", true},
		{"192.168.2.48:40001", "192.168.2.48", "", true},
		{"192.168.2.480:40001", "192.168.2.48", "", false}, // host must match exactly, not as a prefix
		{"192.168.2.99:40001", "192.168.2.48", "adb-RZ8M81AB-aBcDeF", false},
		{"adb-RZ8M81AB-aBcDeF._adb-tls-connect._tcp", "192.168.2.48", "adb-RZ8M81AB-aBcDeF", true},
		{"adb-RZ8M81AB-aBcDeF._adb-tls-connect._tcp", "192.168.2.48", "", false},
		{"adb-OTHER-zZzZzZ._adb-tls-connect._tcp", "192.168.2.48", "adb-RZ8M81AB-aBcDeF", false},
		// guid must match on the "." boundary, not as a bare string prefix
		{"adb-RZ8M81AB-aBcDeFGH._adb-tls-connect._tcp", "192.168.2.99", "adb-RZ8M81AB-aBcDeF", false},
		{"ABCD1234", "192.168.2.48", "adb-RZ8M81AB-aBcDeF", false}, // usb serial
		{"[fe80::1]:40001", "fe80::1", "", true},
	}
	for _, tt := range tests {
		if got := pairedSerialMatch(tt.serial, tt.ip, tt.guid); got != tt.want {
			t.Errorf("pairedSerialMatch(%q, %q, %q) = %v, want %v", tt.serial, tt.ip, tt.guid, got, tt.want)
		}
	}
}

func TestKnownHostLocked(t *testing.T) {
	dp := &DevicePanel{devices: []adb.Device{
		{Serial: "192.168.1.5:5555"},
		{Serial: "adb-RZ8M81AB-aBcDeF._adb-tls-connect._tcp", Host: "192.168.2.48"},
		{Serial: "ABCD1234"},
	}}

	if !dp.knownHostLocked("192.168.1.5") {
		t.Error("ip:port serial host should match")
	}
	if !dp.knownHostLocked("192.168.2.48") {
		t.Error("stored Host should match when serial is an instance name")
	}
	if dp.knownHostLocked("192.168.9.9") {
		t.Error("unknown host should not match")
	}
}

func TestBuildCameraLabelsDisambiguatesDuplicates(t *testing.T) {
	cams := []scrcpy.CameraInfo{
		{ID: "0", Facing: "back", Size: "4000x3000"},
		{ID: "2", Facing: "back", Size: "4000x3000"},
		{ID: "1", Facing: "front", Size: "3264x2448"},
	}

	labels, mapping := buildCameraLabels(cams)

	if len(labels) != 4 {
		t.Fatalf("labels: got %d, want 4 (default + 3 cameras)", len(labels))
	}
	if got := mapping["Back · 4000x3000 (0)"]; got != "0" {
		t.Errorf("first duplicate: got id %q, want 0", got)
	}
	if got := mapping["Back · 4000x3000 (2)"]; got != "2" {
		t.Errorf("second duplicate: got id %q, want 2", got)
	}
	if got := mapping["Front · 3264x2448"]; got != "1" {
		t.Errorf("unique label should stay unsuffixed: got id %q, want 1", got)
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
