package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// compactText is a tighter alternative to widget.Label / canvas.Text.
// It reports MinSize as (measuredWidth, fontSize + 2) instead of the
// font's full line-metric height (which includes ascent + descent + leading
// space the visible glyphs typically don't fill). The actual canvas.Text
// still renders at its natural size - surplus extends harmlessly past the
// reported bounds since fyne containers don't clip by default.
//
// Color and size auto-update when the active theme changes.
type compactText struct {
	widget.BaseWidget

	Text      string
	ColorName fyne.ThemeColorName
	FontSize  float32 // 0 = theme.SizeNameText
	Bold      bool
	Truncate  bool // when true, Layout shortens the displayed text with an ellipsis if it overflows the assigned width
}

func newCompactText(text string, colorName fyne.ThemeColorName) *compactText {
	t := &compactText{Text: text, ColorName: colorName}
	if t.ColorName == "" {
		t.ColorName = theme.ColorNameForeground
	}
	t.ExtendBaseWidget(t)
	return t
}

func (t *compactText) SetText(s string) {
	if t.Text == s {
		return
	}
	t.Text = s
	t.Refresh()
}

func (t *compactText) fontSize() float32 {
	if t.FontSize > 0 {
		return t.FontSize
	}
	return t.Theme().Size(theme.SizeNameText)
}

func (t *compactText) CreateRenderer() fyne.WidgetRenderer {
	text := canvas.NewText("", nil)
	r := &compactTextRenderer{ct: t, text: text}
	r.applyTheme()
	return r
}

type compactTextRenderer struct {
	ct   *compactText
	text *canvas.Text
}

func (r *compactTextRenderer) MinSize() fyne.Size {
	fontSize := r.ct.fontSize()
	measured := fyne.MeasureText(r.ct.Text, fontSize, fyne.TextStyle{Bold: r.ct.Bold})
	return fyne.NewSize(measured.Width, fontSize+2)
}

func (r *compactTextRenderer) Layout(size fyne.Size) {
	display := r.ct.Text
	style := fyne.TextStyle{Bold: r.ct.Bold}
	if r.ct.Truncate && size.Width > 0 {
		full := fyne.MeasureText(r.ct.Text, r.ct.fontSize(), style).Width
		if full > size.Width {
			display = truncateWithEllipsis(r.ct.Text, r.ct.fontSize(), style, size.Width)
		}
	}
	if r.text.Text != display {
		r.text.Text = display
		r.text.Refresh()
	}

	natural := r.text.MinSize()
	r.text.Resize(natural)
	// vertically center the text's natural bounding box inside our reported
	// (smaller) widget bounds; the surplus ascent/descent space spills
	// invisibly above and below.
	r.text.Move(fyne.NewPos(0, (size.Height-natural.Height)/2))
}

// truncateWithEllipsis returns the longest prefix of s (in runes) that -
// followed by "…" - still fits within maxW. Empty if even the ellipsis
// alone overflows.
func truncateWithEllipsis(s string, fontSize float32, style fyne.TextStyle, maxW float32) string {
	const ellipsis = "…"
	if s == "" {
		return s
	}
	if fyne.MeasureText(ellipsis, fontSize, style).Width > maxW {
		return ""
	}
	runes := []rune(s)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		w := fyne.MeasureText(string(runes[:mid])+ellipsis, fontSize, style).Width
		if w <= maxW {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(runes[:lo]) + ellipsis
}

func (r *compactTextRenderer) Refresh() {
	r.applyTheme()
	canvas.Refresh(r.ct)
}

func (r *compactTextRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.text}
}

func (r *compactTextRenderer) Destroy() {}

func (r *compactTextRenderer) applyTheme() {
	th := r.ct.Theme()
	v := fyne.CurrentApp().Settings().ThemeVariant()

	r.text.Text = r.ct.Text
	r.text.Color = th.Color(r.ct.ColorName, v)
	r.text.TextSize = r.ct.fontSize()
	r.text.TextStyle = fyne.TextStyle{Bold: r.ct.Bold}
	r.text.Refresh()
}
