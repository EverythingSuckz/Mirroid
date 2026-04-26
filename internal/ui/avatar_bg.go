package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/icons"
)

var (
	avatarBgLight = color.NRGBA{R: 0xee, G: 0xee, B: 0xee, A: 0xff}
	avatarBgDark  = color.NRGBA{R: 0x2a, G: 0x2a, B: 0x2a, A: 0xff}
)

// avatarBg is a rounded rectangle whose fill is resolved at Refresh time from
// (current theme variant) + (optional brand color). It auto-refreshes on
// theme change because Fyne calls Refresh on every widget when settings change.
//
//   - brand == nil: render fallback color as-is.
//   - brand is very light (Sony white): always dark-gray bg.
//   - brand is very dark (Honor black): always light-gray bg.
//   - otherwise: light-gray bg on light theme, dark-gray on dark theme.
type avatarBg struct {
	widget.BaseWidget
	radius   float32
	brand    color.Color // nil ⇒ use fallback
	fallback color.Color
}

func newAvatarBg(radius float32, fallback color.Color) *avatarBg {
	a := &avatarBg{radius: radius, fallback: fallback}
	a.ExtendBaseWidget(a)
	return a
}

func (a *avatarBg) SetBrand(brand color.Color) {
	a.brand = brand
	a.Refresh()
}

func (a *avatarBg) resolve() color.Color {
	if a.brand == nil {
		return a.fallback
	}
	if icons.IsLightColor(a.brand) {
		return avatarBgDark
	}
	if icons.IsDarkColor(a.brand) {
		return avatarBgLight
	}
	if icons.IsDarkBackground(theme.Color(theme.ColorNameBackground)) {
		return avatarBgDark
	}
	return avatarBgLight
}

func (a *avatarBg) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(a.resolve())
	rect.CornerRadius = a.radius
	return &avatarBgRenderer{w: a, rect: rect}
}

type avatarBgRenderer struct {
	w    *avatarBg
	rect *canvas.Rectangle
}

func (r *avatarBgRenderer) Layout(size fyne.Size) { r.rect.Resize(size) }
func (r *avatarBgRenderer) MinSize() fyne.Size    { return fyne.NewSize(0, 0) }
func (r *avatarBgRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.rect}
}
func (r *avatarBgRenderer) Destroy() {}

func (r *avatarBgRenderer) Refresh() {
	r.rect.FillColor = r.w.resolve()
	r.rect.CornerRadius = r.w.radius
	r.rect.Refresh()
}
