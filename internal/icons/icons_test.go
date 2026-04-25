package icons

import (
	"image/color"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
)

func TestAllIconsValid(t *testing.T) {
	icons := []struct {
		name string
		res  fyne.Resource
	}{
		{"SmartphoneIcon", SmartphoneIcon},
		{"MonitorIcon", MonitorIcon},
		{"BatteryMediumIcon", BatteryMediumIcon},
		{"HardDriveIcon", HardDriveIcon},
		{"CPUIcon", CPUIcon},
		{"WifiIcon", WifiIcon},
		{"SettingsIcon", SettingsIcon},
		{"ThermometerIcon", ThermometerIcon},
		{"ClockIcon", ClockIcon},
		{"PackageIcon", PackageIcon},
		{"MemoryStickIcon", MemoryStickIcon},
		{"ZapIcon", ZapIcon},
		{"HeartIcon", HeartIcon},
	}

	for _, tt := range icons {
		t.Run(tt.name, func(t *testing.T) {
			if tt.res == nil {
				t.Fatal("resource is nil")
			}
			if len(tt.res.Content()) == 0 {
				t.Fatal("resource content is empty")
			}
			if !strings.HasSuffix(tt.res.Name(), ".svg") {
				t.Fatalf("expected .svg suffix, got %q", tt.res.Name())
			}
		})
	}
}

func TestColorToHex(t *testing.T) {
	tests := []struct {
		name string
		in   color.Color
		want string
	}{
		{"black", color.Black, "#000000"},
		{"white", color.White, "#ffffff"},
		{"red", color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}, "#ff0000"},
		{"green", color.NRGBA{R: 0x00, G: 0xff, B: 0x00, A: 0xff}, "#00ff00"},
		{"mid-gray", color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff}, "#808080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := colorToHex(tt.in); got != tt.want {
				t.Errorf("colorToHex(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// stubResource is a test double for fyne.Resource that lets us inject SVG
// content without touching disk or the embed FS.
type stubResource struct {
	name    string
	content []byte
}

func (s *stubResource) Name() string    { return s.name }
func (s *stubResource) Content() []byte { return s.content }

func TestThemedStrokeResourceCachesByColor(t *testing.T) {
	src := &stubResource{
		name:    "x.svg",
		content: []byte(`<svg stroke="currentColor"/>`),
	}
	r := &themedStrokeResource{src: src}

	red := color.NRGBA{R: 0xff, A: 0xff}
	out1 := r.contentForColor(red)
	out2 := r.contentForColor(red)
	// Same color → same backing slice (cache hit).
	if &out1[0] != &out2[0] {
		t.Errorf("expected cached slice to be reused for identical color; got fresh allocation")
	}
	if !strings.Contains(string(out1), "#ff0000") {
		t.Errorf("expected substituted content to contain #ff0000, got %q", out1)
	}

	blue := color.NRGBA{B: 0xff, A: 0xff}
	out3 := r.contentForColor(blue)
	if &out1[0] == &out3[0] {
		t.Errorf("expected new slice when color changed")
	}
	if !strings.Contains(string(out3), "#0000ff") {
		t.Errorf("expected substituted content to contain #0000ff, got %q", out3)
	}
}
