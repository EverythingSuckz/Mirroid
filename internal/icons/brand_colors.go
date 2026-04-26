package icons

import (
	"image/color"
	"strings"
)

// brandColors maps Simple Icons slugs to the brand's official hex color.
// values sourced from simple-icons/data/simple-icons.json.
var brandColors = map[string]color.NRGBA{
	"google":   {R: 0x42, G: 0x85, B: 0xF4, A: 0xff},
	"samsung":  {R: 0x14, G: 0x28, B: 0xA0, A: 0xff},
	"xiaomi":   {R: 0xFF, G: 0x69, B: 0x00, A: 0xff},
	"oneplus":  {R: 0xF5, G: 0x01, B: 0x0C, A: 0xff},
	"huawei":   {R: 0xFF, G: 0x00, B: 0x00, A: 0xff},
	"lg":       {R: 0xA5, G: 0x00, B: 0x34, A: 0xff},
	"motorola": {R: 0xE1, G: 0x14, B: 0x0A, A: 0xff},
	"sony":     {R: 0xFF, G: 0xFF, B: 0xFF, A: 0xff},
	"oppo":     {R: 0x2D, G: 0x68, B: 0x3D, A: 0xff},
	"vivo":     {R: 0x41, G: 0x5F, B: 0xFF, A: 0xff},
	"asus":     {R: 0x00, G: 0x00, B: 0x00, A: 0xff},
	"honor":    {R: 0x00, G: 0x00, B: 0x00, A: 0xff},
	"nokia":    {R: 0x00, G: 0x5A, B: 0xFF, A: 0xff},
	"nothing":  {R: 0x00, G: 0x00, B: 0x00, A: 0xff},
}

// BrandColor returns the official brand color for the given manufacturer, or
// (nil, false) if no color is bundled. Lookup matches BrandIcon's rules.
func BrandColor(manufacturer string) (color.Color, bool) {
	key := resolveBrandKey(manufacturer)
	if key == "" {
		return nil, false
	}
	if c, ok := brandColors[key]; ok {
		return c, true
	}
	if alt := firstTokenKey(key); alt != "" {
		if c, ok := brandColors[alt]; ok {
			return c, true
		}
	}
	return nil, false
}

// firstTokenKey returns the prefix of key up to the first space/comma/period,
// or "" when there is no separator.
func firstTokenKey(key string) string {
	if i := strings.IndexAny(key, " ,."); i > 0 {
		return key[:i]
	}
	return ""
}

// IsLightColor reports whether a color is bright enough that it would
// disappear against a white background (e.g., Sony's white).
func IsLightColor(c color.Color) bool {
	return perceivedBrightness(c) > 0.85
}

// IsDarkColor reports whether a color is dark enough that it would disappear
// against a dark-gray background (e.g., Honor's black).
func IsDarkColor(c color.Color) bool {
	return perceivedBrightness(c) < 0.05
}

// IsDarkBackground reports whether the given color is dark enough to warrant
// dark-theme treatment. Threshold (0.5) tuned for the default Fyne themes.
func IsDarkBackground(c color.Color) bool {
	return perceivedBrightness(c) < 0.5
}

// perceivedBrightness is a fast sRGB-weighted approximation, not WCAG
// relative luminance — it skips the gamma-to-linear step. Thresholds in
// IsLightColor / IsDarkColor / IsDarkBackground are tuned against this scale.
func perceivedBrightness(c color.Color) float64 {
	nr := color.NRGBAModel.Convert(c).(color.NRGBA)
	r := float64(nr.R) / 255.0
	g := float64(nr.G) / 255.0
	b := float64(nr.B) / 255.0
	return 0.2126*r + 0.7152*g + 0.0722*b
}
