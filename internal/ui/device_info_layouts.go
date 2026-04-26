package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type themedRect struct {
	widget.BaseWidget
	colorName fyne.ThemeColorName
	radius    float32
}

func newThemedRect(colorName fyne.ThemeColorName, radius float32) *themedRect {
	r := &themedRect{colorName: colorName, radius: radius}
	r.ExtendBaseWidget(r)
	return r
}

func (r *themedRect) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(theme.Color(r.colorName))
	bg.CornerRadius = r.radius
	return &themedRectRenderer{rect: r, bg: bg}
}

type themedRectRenderer struct {
	rect *themedRect
	bg   *canvas.Rectangle
}

func (r *themedRectRenderer) Layout(size fyne.Size)        { r.bg.Resize(size) }
func (r *themedRectRenderer) MinSize() fyne.Size           { return fyne.NewSize(0, 0) }
func (r *themedRectRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.bg} }
func (r *themedRectRenderer) Destroy()                     {}

func (r *themedRectRenderer) Refresh() {
	r.bg.FillColor = theme.Color(r.rect.colorName)
	r.bg.CornerRadius = r.rect.radius
	r.bg.Refresh()
}

type progressBarLayout struct {
	pct float64
}

func (p *progressBarLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, progressBarHeight)
}

func (p *progressBarLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	y := (size.Height - progressBarHeight) / 2
	if y < 0 {
		y = 0
	}
	barWidth := size.Width - theme.Padding()*2
	if barWidth < 0 {
		barWidth = 0
	}
	objects[0].Resize(fyne.NewSize(barWidth, progressBarHeight))
	objects[0].Move(fyne.NewPos(0, y))

	pct := p.pct
	if pct < 0 {
		pct = 0
	} else if pct > 1 {
		pct = 1
	}
	fgWidth := barWidth * float32(pct)
	objects[1].Resize(fyne.NewSize(fgWidth, progressBarHeight))
	objects[1].Move(fyne.NewPos(0, y))
}

type badgeLayout struct {
	padX float32
	padY float32
}

func (l *badgeLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, 0)
	}
	contentMin := objects[1].MinSize()
	return fyne.NewSize(contentMin.Width+l.padX*2, contentMin.Height+l.padY*2)
}

func (l *badgeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	objects[0].Resize(size)
	objects[0].Move(fyne.NewPos(0, 0))
	contentMin := objects[1].MinSize()
	objects[1].Resize(contentMin)
	objects[1].Move(fyne.NewPos(
		(size.Width-contentMin.Width)/2,
		(size.Height-contentMin.Height)/2,
	))
}

// tightVLayout stacks children vertically with a fixed (small) gap.
// children get their MinSize.Height; layout width = max child width.
type tightVLayout struct {
	spacing float32
}

func (l *tightVLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for i, o := range objects {
		m := o.MinSize()
		if m.Width > w {
			w = m.Width
		}
		h += m.Height
		if i > 0 {
			h += l.spacing
		}
	}
	return fyne.NewSize(w, h)
}

func (l *tightVLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	heights := make([]float32, len(objects))
	var total float32
	for i, o := range objects {
		heights[i] = o.MinSize().Height
		total += heights[i]
		if i > 0 {
			total += l.spacing
		}
	}
	y := (size.Height - total) / 2
	if y < 0 {
		y = 0
	}
	for i, o := range objects {
		o.Resize(fyne.NewSize(size.Width, heights[i]))
		o.Move(fyne.NewPos(0, y))
		y += heights[i]
		if i < len(objects)-1 {
			y += l.spacing
		}
	}
}

type fixedSizeLayout struct {
	width  float32
	height float32
}

func (f *fixedSizeLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(f.width, f.height)
}

func (f *fixedSizeLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}
