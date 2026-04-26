package icons

import (
	"image/color"
	"math"
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

func BrandColor(manufacturer string) (color.Color, bool) {
	key := strings.ToLower(strings.TrimSpace(manufacturer))
	if key == "" {
		return nil, false
	}
	if alias, ok := brandAliases[key]; ok {
		key = alias
	}
	if c, ok := brandColors[key]; ok {
		return c, true
	}
	if i := strings.IndexAny(key, " ,."); i > 0 {
		if c, ok := brandColors[key[:i]]; ok {
			return c, true
		}
	}
	return nil, false
}

// reports whether two colors differ enough in luminance to be
// visually distinguishable when one is rendered on top of the other.
// Threshold is 0.35
func HasContrast(fg, bg color.Color) bool {
	return math.Abs(relativeLuminance(fg)-relativeLuminance(bg)) > 0.35
}

// reports whether a color's relative luminance is high enough that it would disappear against a white background
func IsLightColor(c color.Color) bool {
	return relativeLuminance(c) > 0.85
}

// reports whether a color's relative luminance is low enough that it would disappear against a dark-gray background
func IsDarkColor(c color.Color) bool {
	return relativeLuminance(c) < 0.05
}

// reports whether the given background color is dark enough to warrant the dark-theme treatment
func IsDarkTheme(bg color.Color) bool {
	return relativeLuminance(bg) < 0.5
}

func relativeLuminance(c color.Color) float64 {
	nr := color.NRGBAModel.Convert(c).(color.NRGBA)
	r := float64(nr.R) / 255.0
	g := float64(nr.G) / 255.0
	b := float64(nr.B) / 255.0
	return 0.2126*r + 0.7152*g + 0.0722*b
}
