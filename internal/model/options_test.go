package model

import (
	"testing"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts.Bitrate != 8 {
		t.Errorf("expected bitrate 8, got %d", opts.Bitrate)
	}
	if opts.Codec != "h264" {
		t.Errorf("expected codec h264, got %s", opts.Codec)
	}
	if opts.AudioEnabled {
		t.Error("expected audio disabled by default")
	}
	if !opts.ClipboardSync {
		t.Error("expected clipboard sync enabled by default")
	}
	if opts.VideoSource != "display" {
		t.Errorf("expected video source display, got %s", opts.VideoSource)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*ScrcpyOptions)
		wantErr bool
	}{
		{"valid defaults", func(o *ScrcpyOptions) {}, false},
		{"bitrate negative", func(o *ScrcpyOptions) { o.Bitrate = -1 }, true},
		{"bitrate too high", func(o *ScrcpyOptions) { o.Bitrate = 300 }, true},
		{"max fps negative", func(o *ScrcpyOptions) { o.MaxFPS = -5 }, true},
		{"max fps too high", func(o *ScrcpyOptions) { o.MaxFPS = 500 }, true},
		{"invalid codec", func(o *ScrcpyOptions) { o.Codec = "vp9" }, true},
		{"invalid audio source", func(o *ScrcpyOptions) { o.AudioSource = "bluetooth" }, true},
		{"rotation too high", func(o *ScrcpyOptions) { o.Rotation = 5 }, true},
		{"invalid video source", func(o *ScrcpyOptions) { o.VideoSource = "hdmi" }, true},
		{"valid h265", func(o *ScrcpyOptions) { o.Codec = "h265" }, false},
		{"valid av1", func(o *ScrcpyOptions) { o.Codec = "av1" }, false},
		{"valid max values", func(o *ScrcpyOptions) { o.Bitrate = 200; o.MaxFPS = 240; o.Rotation = 3 }, false},
		{"camera with front facing", func(o *ScrcpyOptions) { o.VideoSource = "camera"; o.CameraFacing = "front" }, false},
		{"camera with invalid facing", func(o *ScrcpyOptions) { o.VideoSource = "camera"; o.CameraFacing = "selfie" }, true},
		{"camera with empty facing", func(o *ScrcpyOptions) { o.VideoSource = "camera"; o.CameraFacing = "" }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultOptions()
			tt.modify(&opts)
			err := opts.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		name     string
		serial   string
		modify   func(*ScrcpyOptions)
		contains []string
		excludes []string
	}{
		{
			name:     "defaults",
			serial:   "abc123",
			modify:   func(o *ScrcpyOptions) {},
			contains: []string{"scrcpy", "-s", "abc123", "-b", "8000000", "--video-codec", "h264", "--no-audio"},
			excludes: []string{"-f", "--window-borderless"},
		},
		{
			name:   "fullscreen + borderless",
			serial: "dev1",
			modify: func(o *ScrcpyOptions) {
				o.Fullscreen = true
				o.Borderless = true
			},
			contains: []string{"-f", "--window-borderless"},
		},
		{
			name:   "audio disabled",
			serial: "dev1",
			modify: func(o *ScrcpyOptions) {
				o.AudioEnabled = true
			},
			excludes: []string{"--no-audio"},
		},
		{
			name:   "hid keyboard and mouse",
			serial: "dev1",
			modify: func(o *ScrcpyOptions) {
				o.HIDKeyboard = true
				o.HIDMouse = true
			},
			contains: []string{"-K", "-M"},
		},
		{
			name:   "camera source defaults to back facing",
			serial: "dev1",
			modify: func(o *ScrcpyOptions) {
				o.VideoSource = "camera"
			},
			contains: []string{"--video-source", "camera", "--camera-facing", "back"},
		},
		{
			name:   "camera front facing",
			serial: "dev1",
			modify: func(o *ScrcpyOptions) {
				o.VideoSource = "camera"
				o.CameraFacing = "front"
			},
			contains: []string{"--video-source", "camera", "--camera-facing", "front"},
		},
		{
			name:   "record file",
			serial: "dev1",
			modify: func(o *ScrcpyOptions) {
				o.RecordFile = "video.mp4"
			},
			contains: []string{"-r", "video.mp4"},
		},
		{
			name:   "clipboard sync disabled",
			serial: "dev1",
			modify: func(o *ScrcpyOptions) {
				o.ClipboardSync = false
			},
			contains: []string{"--no-clipboard-autosync"},
		},
		{
			name:     "no serial",
			serial:   "",
			modify:   func(o *ScrcpyOptions) {},
			excludes: []string{"-s"},
		},
		{
			name:   "rotation",
			serial: "dev1",
			modify: func(o *ScrcpyOptions) {
				o.Rotation = 2
			},
			contains: []string{"--orientation", "2"},
		},
		{
			name:   "window title",
			serial: "dev1",
			modify: func(o *ScrcpyOptions) {
				o.WindowTitle = "My Phone"
			},
			contains: []string{"--window-title", "My Phone"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultOptions()
			tt.modify(&opts)
			cmd := opts.BuildCommand("scrcpy", tt.serial)
			cmdStr := ""
			for _, c := range cmd {
				cmdStr += c + " "
			}

			for _, want := range tt.contains {
				found := false
				for _, c := range cmd {
					if c == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected %q in command: %s", want, cmdStr)
				}
			}

			for _, exclude := range tt.excludes {
				for _, c := range cmd {
					if c == exclude {
						t.Errorf("did not expect %q in command: %s", exclude, cmdStr)
					}
				}
			}
		})
	}
}

func TestJSONRoundTrip(t *testing.T) {
	original := DefaultOptions()
	original.Bitrate = 16
	original.Codec = "h265"
	original.Fullscreen = true
	original.HIDKeyboard = true
	original.WindowTitle = "Test Window"

	data, err := original.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON() error: %v", err)
	}

	restored, err := FromJSON(data)
	if err != nil {
		t.Fatalf("FromJSON() error: %v", err)
	}

	if restored.Bitrate != 16 {
		t.Errorf("bitrate: got %d, want 16", restored.Bitrate)
	}
	if restored.Codec != "h265" {
		t.Errorf("codec: got %s, want h265", restored.Codec)
	}
	if !restored.Fullscreen {
		t.Error("fullscreen should be true")
	}
	if !restored.HIDKeyboard {
		t.Error("HID keyboard should be true")
	}
	if restored.WindowTitle != "Test Window" {
		t.Errorf("window title: got %s, want Test Window", restored.WindowTitle)
	}
	// check defaults are preserved for fields not in JSON
	if restored.AudioEnabled {
		t.Error("audio should be disabled (default)")
	}
}
