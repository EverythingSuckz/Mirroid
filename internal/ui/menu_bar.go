package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// slimMenuBarTheme delegates everything to the active theme but tightens
// padding so the menu-bar row is shorter than fyne's default button height.
type slimMenuBarTheme struct{}

var _ fyne.Theme = (*slimMenuBarTheme)(nil)

func (t *slimMenuBarTheme) base() fyne.Theme {
	return fyne.CurrentApp().Settings().Theme()
}

func (t *slimMenuBarTheme) Color(name fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return t.base().Color(name, v)
}

func (t *slimMenuBarTheme) Font(s fyne.TextStyle) fyne.Resource {
	return t.base().Font(s)
}

func (t *slimMenuBarTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return t.base().Icon(n)
}

func (t *slimMenuBarTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 2
	case theme.SizeNameInnerPadding:
		return 4
	}
	return t.base().Size(name)
}

// bellWithDotLayout sizes the first child (the bell button) to fill, and
// pins the second child (the unread dot) to the top-right corner with a
// small inset.
type bellWithDotLayout struct{ inset float32 }

func (l *bellWithDotLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	return objects[0].MinSize()
}

func (l *bellWithDotLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	objects[0].Resize(size)
	objects[0].Move(fyne.NewPos(0, 0))
	if len(objects) < 2 {
		return
	}
	dotSize := objects[1].MinSize()
	objects[1].Resize(dotSize)
	objects[1].Move(fyne.NewPos(size.Width-dotSize.Width-l.inset, l.inset))
}

// menuButton is a tappable text widget tuned for a menu bar:
//   - non-bold label (fyne's widget.Button hardcodes bold)
//   - subtle hover/selected background, transparent otherwise
//   - selected state stays "lit" while a popup menu is open
type menuButton struct {
	widget.BaseWidget

	label    string
	onTap    func()
	selected bool
	hovered  bool
}

func newMenuButton(label string, onTap func()) *menuButton {
	b := &menuButton{label: label, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func (b *menuButton) Tapped(_ *fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *menuButton) MouseIn(_ *desktop.MouseEvent) {
	b.hovered = true
	b.Refresh()
}

func (b *menuButton) MouseOut() {
	b.hovered = false
	b.Refresh()
}

func (b *menuButton) MouseMoved(_ *desktop.MouseEvent) {}

func (b *menuButton) setSelected(s bool) {
	if b.selected == s {
		return
	}
	b.selected = s
	b.Refresh()
}

func (b *menuButton) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 4
	text := canvas.NewText(b.label, color.Black)
	text.Alignment = fyne.TextAlignCenter
	r := &menuButtonRenderer{btn: b, bg: bg, text: text}
	r.applyTheme()
	return r
}

type menuButtonRenderer struct {
	btn  *menuButton
	bg   *canvas.Rectangle
	text *canvas.Text
}

func (r *menuButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))
	tm := r.text.MinSize()
	r.text.Resize(tm)
	r.text.Move(fyne.NewPos((size.Width-tm.Width)/2, (size.Height-tm.Height)/2))
}

func (r *menuButtonRenderer) MinSize() fyne.Size {
	th := r.btn.Theme()
	pad := th.Size(theme.SizeNamePadding)
	inner := th.Size(theme.SizeNameInnerPadding)
	tm := r.text.MinSize()
	return fyne.NewSize(tm.Width+inner*2+pad*2, tm.Height+inner*2)
}

func (r *menuButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.text}
}

func (r *menuButtonRenderer) Destroy() {}

func (r *menuButtonRenderer) Refresh() {
	r.applyTheme()
	r.bg.Refresh()
	r.text.Refresh()
}

func (r *menuButtonRenderer) applyTheme() {
	th := r.btn.Theme()
	v := fyne.CurrentApp().Settings().ThemeVariant()
	r.text.Color = th.Color(theme.ColorNameForeground, v)
	r.text.TextSize = th.Size(theme.SizeNameText)

	switch {
	case r.btn.selected:
		r.bg.FillColor = th.Color(theme.ColorNameButton, v)
	case r.btn.hovered:
		r.bg.FillColor = th.Color(theme.ColorNameHover, v)
	default:
		r.bg.FillColor = color.Transparent
	}
}

// menuBarButton creates a non-bold menu-bar button that opens its menu as
// a popup just below itself, and stays in a "selected" visual state while
// the popup is open.
func menuBarButton(c fyne.Canvas, label string, menu *fyne.Menu) *menuButton {
	var btn *menuButton
	btn = newMenuButton(label, func() {
		popup := widget.NewPopUpMenu(menu, c)
		prev := popup.OnDismiss
		popup.OnDismiss = func() {
			if prev != nil {
				prev()
			}
			btn.setSelected(false)
		}
		popup.ShowAtRelativePosition(fyne.NewPos(0, btn.Size().Height), btn)
		btn.setSelected(true)
	})
	return btn
}
