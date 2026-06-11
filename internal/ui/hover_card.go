package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// hoverCard wraps content and paints theme.ColorNameHover behind it while
// the pointer is over the wrapper.
//
// Caveat: fyne hover events do not bubble - a Hoverable child (e.g. a
// button inside the card) takes over the pointer, briefly removing the
// card's hover bg until the pointer leaves the child. Visible flicker is
// minor and acceptable for short-lived popovers.
type hoverCard struct {
	widget.BaseWidget

	content fyne.CanvasObject
	hovered bool
	OnTap   func()  // optional: makes the card tappable
	radius  float32 // corner radius of the hover backdrop
}

func newHoverCard(content fyne.CanvasObject) *hoverCard {
	c := &hoverCard{content: content}
	c.ExtendBaseWidget(c)
	return c
}

func (c *hoverCard) MouseIn(_ *desktop.MouseEvent) {
	c.hovered = true
	c.Refresh()
}

func (c *hoverCard) MouseOut() {
	c.hovered = false
	c.Refresh()
}

func (c *hoverCard) MouseMoved(_ *desktop.MouseEvent) {}

func (c *hoverCard) Tapped(_ *fyne.PointEvent) {
	if c.OnTap != nil {
		c.OnTap()
	}
}

func (c *hoverCard) Cursor() desktop.Cursor {
	if c.OnTap != nil {
		return desktop.PointerCursor
	}
	return desktop.DefaultCursor
}

func (c *hoverCard) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	return &hoverCardRenderer{card: c, bg: bg}
}

type hoverCardRenderer struct {
	card *hoverCard
	bg   *canvas.Rectangle
}

func (r *hoverCardRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))
	r.card.content.Resize(size)
	r.card.content.Move(fyne.NewPos(0, 0))
}

func (r *hoverCardRenderer) MinSize() fyne.Size {
	return r.card.content.MinSize()
}

func (r *hoverCardRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.card.content}
}

func (r *hoverCardRenderer) Destroy() {}

func (r *hoverCardRenderer) Refresh() {
	if r.card.hovered {
		r.bg.FillColor = theme.Color(theme.ColorNameHover)
	} else {
		r.bg.FillColor = color.Transparent
	}
	r.bg.CornerRadius = r.card.radius
	r.bg.Refresh()
	r.card.content.Refresh()
}
