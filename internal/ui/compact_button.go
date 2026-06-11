package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	ttwidget "github.com/dweymouth/fyne-tooltip/widget"
)

const (
	compactButtonIconSize float32 = 20
	compactButtonPadding  float32 = 4
)

// compactButton is an icon-only tappable widget with much tighter padding
// than fyne's widget.Button (which adds 2 × InnerPadding around its icon -
// ~36 px tall at default sizes). Embeds fyne-tooltip's extension so
// SetToolTip works the same way as on ttwidget.Button.
type compactButton struct {
	widget.BaseWidget
	ttwidget.ToolTipWidgetExtend

	Icon     fyne.Resource
	OnTapped func()

	IconSize float32
	Padding  float32

	enabled bool
	hovered bool
}

func newCompactButton(icon fyne.Resource, onTapped func()) *compactButton {
	b := &compactButton{
		Icon:     icon,
		OnTapped: onTapped,
		IconSize: compactButtonIconSize,
		Padding:  compactButtonPadding,
		enabled:  true,
	}
	b.ExtendBaseWidget(b)
	return b
}

func (b *compactButton) ExtendBaseWidget(wid fyne.Widget) {
	b.ExtendToolTipWidget(wid)
	b.BaseWidget.ExtendBaseWidget(wid)
}

func (b *compactButton) Tapped(_ *fyne.PointEvent) {
	if !b.enabled || b.OnTapped == nil {
		return
	}
	b.OnTapped()
}

func (b *compactButton) Cursor() desktop.Cursor { return desktop.DefaultCursor }

func (b *compactButton) Enable() {
	if b.enabled {
		return
	}
	b.enabled = true
	b.Refresh()
}

func (b *compactButton) Disable() {
	if !b.enabled {
		return
	}
	b.enabled = false
	b.Refresh()
}

func (b *compactButton) Disabled() bool { return !b.enabled }

func (b *compactButton) MouseIn(e *desktop.MouseEvent) {
	b.ToolTipWidgetExtend.MouseIn(e)
	b.hovered = true
	b.Refresh()
}

func (b *compactButton) MouseOut() {
	b.ToolTipWidgetExtend.MouseOut()
	b.hovered = false
	b.Refresh()
}

func (b *compactButton) MouseMoved(e *desktop.MouseEvent) {
	b.ToolTipWidgetExtend.MouseMoved(e)
}

func (b *compactButton) themedIcon() fyne.Resource {
	if b.Icon == nil {
		return nil
	}
	res := theme.NewThemedResource(b.Icon)
	if !b.enabled {
		res.ColorName = theme.ColorNameDisabled
	}
	return res
}

func (b *compactButton) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 4

	iconImg := canvas.NewImageFromResource(b.themedIcon())
	iconImg.FillMode = canvas.ImageFillContain

	r := &compactButtonRenderer{btn: b, bg: bg, icon: iconImg}
	return r
}

type compactButtonRenderer struct {
	btn  *compactButton
	bg   *canvas.Rectangle
	icon *canvas.Image
}

func (r *compactButtonRenderer) MinSize() fyne.Size {
	side := r.btn.IconSize + r.btn.Padding*2
	return fyne.NewSize(side, side)
}

func (r *compactButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	iconSize := fyne.NewSquareSize(r.btn.IconSize)
	r.icon.Resize(iconSize)
	r.icon.Move(fyne.NewPos(
		(size.Width-r.btn.IconSize)/2,
		(size.Height-r.btn.IconSize)/2,
	))
}

func (r *compactButtonRenderer) Refresh() {
	th := r.btn.Theme()
	v := fyne.CurrentApp().Settings().ThemeVariant()

	if r.btn.enabled && r.btn.hovered {
		r.bg.FillColor = th.Color(theme.ColorNameHover, v)
	} else {
		r.bg.FillColor = color.Transparent
	}
	r.bg.Refresh()

	r.icon.Resource = r.btn.themedIcon()
	r.icon.Refresh()
}

func (r *compactButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.icon}
}

func (r *compactButtonRenderer) Destroy() {}
