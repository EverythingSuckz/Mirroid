// Package icons provides Lucide (https://lucide.dev) SVG icons as fyne.Resource constants.
// To add a new icon: drop the 24x24 SVG in the svg/ directory and add a var below.
// Use NewThemedIcon() instead of theme.NewThemedResource() — Fyne's built-in theming only
// handles fill-based SVGs, but Lucide icons are stroke-based (stroke="currentColor").
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

// themedStrokeResource wraps an SVG fyne.Resource and substitutes
// `currentColor` with the active theme foreground each time Fyne asks for the
// content. The substituted bytes are cached and only recomputed when the
// foreground color actually changes (i.e. on theme switch), avoiding a fresh
// SVG parse on every redraw.
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

// contentForColor returns the SVG bytes with `currentColor` substituted for
// the given color, served from cache when the color matches the previous
// call. Split out from Content() so tests can exercise it without booting a
// Fyne app.
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

// NewTintedIcon returns a resource with `currentColor` substituted for the
// given fixed color. Substitution happens once at construction; the returned
// resource is immutable and safe to share. Use NewThemedIcon when the color
// should follow the theme.
func NewTintedIcon(res fyne.Resource, c color.Color) fyne.Resource {
	content := strings.ReplaceAll(string(res.Content()), "currentColor", colorToHex(c))
	return &fyne.StaticResource{
		StaticName:    res.Name(),
		StaticContent: []byte(content),
	}
}

func colorToHex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
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
