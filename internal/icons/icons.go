// Package icons provides Lucide (https://lucide.dev) SVG icons as fyne.Resource constants.
// Use NewThemedIcon for stroke-based Lucide icons; theme.NewThemedResource doesn't
// resolve `currentColor` via oksvg so it renders black on dark themes.
package icons

import (
	"embed"
	"fmt"
	"image/color"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

//go:embed svg/*.svg
var svgFS embed.FS

func mustLoad(name string) *fyne.StaticResource {
	b, err := svgFS.ReadFile("svg/" + name)
	if err != nil {
		panic(fmt.Errorf("icons: missing embedded svg/%s: %w", name, err))
	}
	return &fyne.StaticResource{StaticName: name, StaticContent: b}
}

type themedStrokeResource struct {
	src fyne.Resource

	mu       sync.Mutex
	cachedFG [4]uint32
	hasCache bool
	cached   []byte
}

func (r *themedStrokeResource) Name() string { return r.src.Name() }

func (r *themedStrokeResource) Content() []byte {
	return r.contentForColor(theme.Color(theme.ColorNameForeground))
}

func (r *themedStrokeResource) contentForColor(c color.Color) []byte {
	key := colorKey(c)

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.hasCache && r.cachedFG == key {
		return r.cached
	}
	r.cached = []byte(strings.ReplaceAll(string(r.src.Content()), "currentColor", colorToHex(c)))
	r.cachedFG = key
	r.hasCache = true
	return r.cached
}

func NewThemedIcon(res fyne.Resource) fyne.Resource {
	return &themedStrokeResource{src: res}
}

// NewTintedIcon substitutes `currentColor` once at construction; use NewThemedIcon when the color should follow the theme.
// the tint hex is embedded in the resource name so each tinted variant has a
// unique key in fyne's name-based svg cache.
func NewTintedIcon(res fyne.Resource, c color.Color) fyne.Resource {
	hex := colorToHex(c)
	content := strings.ReplaceAll(string(res.Content()), "currentColor", hex)
	return &fyne.StaticResource{
		StaticName:    res.Name() + "@" + hex,
		StaticContent: []byte(content),
	}
}

func colorToHex(c color.Color) string {
	nr := color.NRGBAModel.Convert(c).(color.NRGBA)
	return fmt.Sprintf("#%02x%02x%02x", nr.R, nr.G, nr.B)
}

func colorKey(c color.Color) [4]uint32 {
	r, g, b, a := c.RGBA()
	return [4]uint32{r, g, b, a}
}

var (
	SmartphoneIcon    = mustLoad("smartphone.svg")
	MonitorIcon       = mustLoad("monitor.svg")
	BatteryMediumIcon = mustLoad("battery-medium.svg")
	HardDriveIcon     = mustLoad("hard-drive.svg")
	CPUIcon           = mustLoad("cpu.svg")
	WifiIcon          = mustLoad("wifi.svg")
	SettingsIcon      = mustLoad("settings-2.svg")
	ThermometerIcon   = mustLoad("thermometer.svg")
	ClockIcon         = mustLoad("clock.svg")
	PackageIcon       = mustLoad("package.svg")
	MemoryStickIcon   = mustLoad("memory-stick.svg")
	ZapIcon           = mustLoad("zap.svg")
	HeartIcon         = mustLoad("heart.svg")
)
