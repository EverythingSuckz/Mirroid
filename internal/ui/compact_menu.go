package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	compactMenuItemPadX float32 = 8
	compactMenuItemPadY float32 = 4
)

// compactMenuItem is one tappable row inside a compactMenu. The label
// bolds when selected and the bg tints on hover - same visual language
// as compactSelect, so the dropdown and its expanded list line up.
type compactMenuItem struct {
	widget.BaseWidget

	Text     string
	Selected bool
	OnTapped func()

	hovered bool
}

func newCompactMenuItem(text string, selected bool, onTapped func()) *compactMenuItem {
	it := &compactMenuItem{Text: text, Selected: selected, OnTapped: onTapped}
	it.ExtendBaseWidget(it)
	return it
}

func (it *compactMenuItem) Tapped(_ *fyne.PointEvent) {
	if it.OnTapped != nil {
		it.OnTapped()
	}
}

func (it *compactMenuItem) MouseIn(_ *desktop.MouseEvent) {
	it.hovered = true
	it.Refresh()
}

func (it *compactMenuItem) MouseOut() {
	it.hovered = false
	it.Refresh()
}

func (it *compactMenuItem) MouseMoved(_ *desktop.MouseEvent) {}

func (it *compactMenuItem) Cursor() desktop.Cursor { return desktop.DefaultCursor }

func (it *compactMenuItem) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	label := newCompactText("", theme.ColorNameForeground)
	label.Truncate = true
	r := &compactMenuItemRenderer{item: it, bg: bg, label: label}
	r.applyTheme()
	return r
}

type compactMenuItemRenderer struct {
	item  *compactMenuItem
	bg    *canvas.Rectangle
	label *compactText
}

func (r *compactMenuItemRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	lm := r.label.MinSize()
	labelW := size.Width - compactMenuItemPadX*2
	if labelW < 0 {
		labelW = 0
	}
	r.label.Resize(fyne.NewSize(labelW, lm.Height))
	r.label.Move(fyne.NewPos(compactMenuItemPadX, (size.Height-lm.Height)/2))
}

func (r *compactMenuItemRenderer) MinSize() fyne.Size {
	lm := r.label.MinSize()
	return fyne.NewSize(lm.Width+compactMenuItemPadX*2, lm.Height+compactMenuItemPadY*2)
}

func (r *compactMenuItemRenderer) Refresh() {
	r.applyTheme()
	canvas.Refresh(r.item)
}

func (r *compactMenuItemRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.label}
}

func (r *compactMenuItemRenderer) Destroy() {}

func (r *compactMenuItemRenderer) applyTheme() {
	th := r.item.Theme()
	v := fyne.CurrentApp().Settings().ThemeVariant()

	if r.item.hovered {
		r.bg.FillColor = th.Color(theme.ColorNameHover, v)
	} else {
		r.bg.FillColor = color.Transparent
	}
	r.label.Text = r.item.Text
	r.label.Bold = r.item.Selected
	r.label.Refresh()
	r.bg.Refresh()
}

// compactMenu is the popup body - a vertical stack of items with no
// inter-item gap. widget.PopUp provides the surrounding rounded bg and
// shadow, so this widget just lays out the rows.
type compactMenu struct {
	widget.BaseWidget
	items    []*compactMenuItem
	MaxWidth float32 // 0 = unbounded; caps MinSize.Width so popup can't blow out wider than the trigger
}

func newCompactMenu(items []*compactMenuItem) *compactMenu {
	m := &compactMenu{items: items}
	m.ExtendBaseWidget(m)
	return m
}

func (m *compactMenu) CreateRenderer() fyne.WidgetRenderer {
	return &compactMenuRenderer{menu: m}
}

type compactMenuRenderer struct {
	menu *compactMenu
}

func (r *compactMenuRenderer) Layout(size fyne.Size) {
	var y float32
	for _, it := range r.menu.items {
		ih := it.MinSize().Height
		it.Resize(fyne.NewSize(size.Width, ih))
		it.Move(fyne.NewPos(0, y))
		y += ih
	}
}

func (r *compactMenuRenderer) MinSize() fyne.Size {
	var w, h float32
	for _, it := range r.menu.items {
		m := it.MinSize()
		if m.Width > w {
			w = m.Width
		}
		h += m.Height
	}
	if r.menu.MaxWidth > 0 && w > r.menu.MaxWidth {
		w = r.menu.MaxWidth
	}
	return fyne.NewSize(w, h)
}

func (r *compactMenuRenderer) Refresh() {
	for _, it := range r.menu.items {
		it.Refresh()
	}
	canvas.Refresh(r.menu)
}

func (r *compactMenuRenderer) Objects() []fyne.CanvasObject {
	objs := make([]fyne.CanvasObject, 0, len(r.menu.items))
	for _, it := range r.menu.items {
		objs = append(objs, it)
	}
	return objs
}

func (r *compactMenuRenderer) Destroy() {}
