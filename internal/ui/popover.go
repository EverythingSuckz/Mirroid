package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const popoverRadius = 8

// popover is a non-modal floating panel anchored at an absolute position on
// the canvas. It dismisses on a tap outside the panel area. Unlike
// widget.PopUp, the panel's background color and corner radius are owned by
// us so it can match surrounding UI surfaces instead of fyne's overlay color.
type popover struct {
	widget.BaseWidget

	canvas    fyne.Canvas
	content   fyne.CanvasObject
	panelPos  fyne.Position
	panelSize fyne.Size
}

func newPopover(content fyne.CanvasObject, c fyne.Canvas) *popover {
	p := &popover{canvas: c, content: content}
	p.ExtendBaseWidget(p)
	return p
}

// ShowAt registers the popover on the canvas's overlay stack so it floats
// above all other content.
func (p *popover) ShowAt(pos fyne.Position, size fyne.Size) {
	p.panelPos = pos
	p.panelSize = size
	p.canvas.Overlays().Add(p)
	p.Refresh()
}

func (p *popover) hideOverlay() {
	p.canvas.Overlays().Remove(p)
}

func (p *popover) Tapped(e *fyne.PointEvent) {
	inside := e.Position.X >= p.panelPos.X &&
		e.Position.Y >= p.panelPos.Y &&
		e.Position.X < p.panelPos.X+p.panelSize.Width &&
		e.Position.Y < p.panelPos.Y+p.panelSize.Height
	if !inside {
		p.hideOverlay()
	}
}

func (p *popover) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameMenuBackground))
	bg.CornerRadius = popoverRadius
	bg.StrokeColor = theme.Color(theme.ColorNameSeparator)
	bg.StrokeWidth = 1
	return &popoverRenderer{popover: p, bg: bg}
}

type popoverRenderer struct {
	popover *popover
	bg      *canvas.Rectangle
}

func (r *popoverRenderer) Layout(_ fyne.Size) {
	r.bg.Resize(r.popover.panelSize)
	r.bg.Move(r.popover.panelPos)
	r.popover.content.Resize(r.popover.panelSize)
	r.popover.content.Move(r.popover.panelPos)
}

func (r *popoverRenderer) MinSize() fyne.Size { return fyne.NewSize(0, 0) }

func (r *popoverRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.popover.content}
}

func (r *popoverRenderer) Destroy() {}

func (r *popoverRenderer) Refresh() {
	// Self-resize to match the canvas so Tapped sees clicks anywhere; same
	// pattern fyne's own widget.PopUp uses.
	if r.popover.canvas.Size() != r.popover.Size() {
		r.popover.Resize(r.popover.canvas.Size())
	} else {
		r.Layout(r.popover.Size())
	}
	r.bg.FillColor = theme.Color(theme.ColorNameMenuBackground)
	r.bg.StrokeColor = theme.Color(theme.ColorNameSeparator)
	r.bg.Refresh()
	r.popover.content.Refresh()
}
