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

//go:embed svg/*.svg svg/brands/*.svg
var svgFS embed.FS

var brandAliases = map[string]string{
	"google llc":                   "google",
	"samsung electronics":          "samsung",
	"xiaomi communications co ltd": "xiaomi",
	"redmi":                        "xiaomi",
	"poco":                         "xiaomi",
	"oneplus technology":           "oneplus",
	"motorola mobility":            "motorola",
	"hmd global":                   "nokia",
}

func BrandIcon(manufacturer string) fyne.Resource {
	key := resolveBrandKey(manufacturer)
	if key == "" {
		return nil
	}
	if b, err := svgFS.ReadFile("svg/brands/" + key + ".svg"); err == nil {
		return &fyne.StaticResource{StaticName: "brand-" + key + ".svg", StaticContent: b}
	}
	if alt := firstTokenKey(key); alt != "" {
		if b, err := svgFS.ReadFile("svg/brands/" + alt + ".svg"); err == nil {
			return &fyne.StaticResource{StaticName: "brand-" + alt + ".svg", StaticContent: b}
		}
	}
	return nil
}

// resolveBrandKey normalizes a manufacturer string to its brand-icon lookup
// key (lowercased, trimmed, alias-mapped). Returns "" for empty input.
func resolveBrandKey(manufacturer string) string {
	key := strings.ToLower(strings.TrimSpace(manufacturer))
	if key == "" {
		return ""
	}
	if alias, ok := brandAliases[key]; ok {
		return alias
	}
	return key
}

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

// Name varies by the active foreground so each themed variant has a unique
// key in fyne's name-based svg raster cache (avoids stale renders on theme switch).
func (r *themedStrokeResource) Name() string {
	return r.src.Name() + "@" + colorToHex(theme.Color(theme.ColorNameForeground))
}

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

var (
	tintedMu    sync.Mutex
	tintedCache = make(map[string]*fyne.StaticResource)
)

// NewTintedIcon substitutes `currentColor` once at construction. Use
// NewThemedIcon when the color should follow the active theme. Resources are
// cached per (source name, tint hex) so repeated calls in list bind callbacks
// don't re-allocate.
func NewTintedIcon(res fyne.Resource, c color.Color) fyne.Resource {
	hex := colorToHex(c)
	key := res.Name() + "@" + hex

	tintedMu.Lock()
	if cached, ok := tintedCache[key]; ok {
		tintedMu.Unlock()
		return cached
	}
	tintedMu.Unlock()

	content := strings.ReplaceAll(string(res.Content()), "currentColor", hex)
	out := &fyne.StaticResource{StaticName: key, StaticContent: []byte(content)}

	tintedMu.Lock()
	tintedCache[key] = out
	tintedMu.Unlock()
	return out
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
