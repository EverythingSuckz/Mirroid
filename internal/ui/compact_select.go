package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// compactSelect is a drop-in alternative to widget.Select with significantly
// less internal vertical padding. fyne's Select hardcodes a label inset of
// SizeNamePadding plus a 1× InnerPadding height bump that keeps the box
// taller than the underlying text. compactSelect renders the label as a
// plain canvas.Text inside a rounded rectangle and exposes the same
// surface (Options / Selected / PlaceHolder / OnChanged / SetSelected /
// ClearSelected) so callers don't need to change.
type compactSelect struct {
	widget.BaseWidget

	Options     []string
	Selected    string
	PlaceHolder string
	OnChanged   func(string)

	hovered bool
}

const (
	compactSelectIconSize      float32 = 16
	compactSelectPadX          float32 = 8
	compactSelectPadY          float32 = 4
	compactSelectGap           float32 = 4
	compactSelectMaxLabelWidth float32 = 180
)

func newCompactSelect(options []string, onChanged func(string)) *compactSelect {
	s := &compactSelect{Options: options, OnChanged: onChanged}
	s.ExtendBaseWidget(s)
	return s
}

func (s *compactSelect) SetSelected(value string) {
	if s.Selected == value {
		return
	}
	s.Selected = value
	s.Refresh()
	if s.OnChanged != nil {
		s.OnChanged(value)
	}
}

// ClearSelected resets the selection without firing OnChanged, mirroring
// widget.Select.ClearSelected.
func (s *compactSelect) ClearSelected() {
	s.Selected = ""
	s.Refresh()
}

func (s *compactSelect) Tapped(_ *fyne.PointEvent) {
	d := fyne.CurrentApp().Driver()
	c := d.CanvasForObject(s)
	if c == nil || len(s.Options) == 0 {
		return
	}
	// Capture the absolute position now - fyne caches canvas lookups in an
	// expiringCache, so calling ShowAtRelativePosition (which re-resolves
	// the parent canvas internally) can occasionally race the cache out
	// and log "Could not locate parent object…". Using ShowAtPosition with
	// a pre-computed absolute position avoids the second lookup entirely.
	abs := d.AbsolutePositionForObject(s)

	var popup *widget.PopUp
	items := make([]*compactMenuItem, 0, len(s.Options))
	for _, opt := range s.Options {
		opt := opt
		items = append(items, newCompactMenuItem(opt, opt == s.Selected, func() {
			if popup != nil {
				popup.Hide()
			}
			s.SetSelected(opt)
		}))
	}
	menu := newCompactMenu(items)
	// cap menu width to the trigger so widget.PopUp can't expand it to
	// fit a long option's natural width; items truncate inside their rows.
	menu.MaxWidth = s.Size().Width
	popup = widget.NewPopUp(menu, c)
	popup.Resize(fyne.NewSize(s.Size().Width, menu.MinSize().Height))
	popup.ShowAtPosition(fyne.NewPos(abs.X, abs.Y+s.Size().Height))
}

func (s *compactSelect) MouseIn(_ *desktop.MouseEvent) {
	s.hovered = true
	s.Refresh()
}

func (s *compactSelect) MouseOut() {
	s.hovered = false
	s.Refresh()
}

func (s *compactSelect) MouseMoved(_ *desktop.MouseEvent) {}

func (s *compactSelect) Cursor() desktop.Cursor { return desktop.DefaultCursor }

func (s *compactSelect) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = 4
	bg.StrokeWidth = 1

	label := newCompactText("", theme.ColorNameForeground)
	label.Truncate = true

	arrow := canvas.NewImageFromResource(theme.NewThemedResource(theme.MenuDropDownIcon()))
	arrow.FillMode = canvas.ImageFillContain

	r := &compactSelectRenderer{sel: s, bg: bg, label: label, arrow: arrow}
	r.applyTheme()
	return r
}

type compactSelectRenderer struct {
	sel   *compactSelect
	bg    *canvas.Rectangle
	label *compactText
	arrow *canvas.Image
}

func (r *compactSelectRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	arrowSize := fyne.NewSquareSize(compactSelectIconSize)
	r.arrow.Resize(arrowSize)
	r.arrow.Move(fyne.NewPos(
		size.Width-compactSelectIconSize-compactSelectPadX,
		(size.Height-compactSelectIconSize)/2,
	))

	labelMin := r.label.MinSize()
	labelW := size.Width - compactSelectIconSize - compactSelectPadX*2 - compactSelectGap
	if labelW < 0 {
		labelW = 0
	}
	r.label.Resize(fyne.NewSize(labelW, labelMin.Height))
	r.label.Move(fyne.NewPos(compactSelectPadX, (size.Height-labelMin.Height)/2))
}

func (r *compactSelectRenderer) MinSize() fyne.Size {
	th := r.sel.Theme()
	fontSize := th.Size(theme.SizeNameText)

	// pick the longest option (and placeholder) so the box is wide enough
	// to show any value without truncation.
	candidates := make([]string, 0, len(r.sel.Options)+1)
	if r.sel.PlaceHolder != "" {
		candidates = append(candidates, r.sel.PlaceHolder)
	}
	candidates = append(candidates, r.sel.Options...)

	var maxW float32
	for _, s := range candidates {
		w := fyne.MeasureText(s, fontSize, fyne.TextStyle{}).Width
		if w > maxW {
			maxW = w
		}
	}
	// cap the dropdown width so a single long preset name can't blow out
	// the row - the label widget truncates to fit (see compactText.Truncate).
	if maxW > compactSelectMaxLabelWidth {
		maxW = compactSelectMaxLabelWidth
	}

	// label MinSize already trims fyne's default line-metric height down
	// to fontSize + 2 (see compactText). Just use it.
	labelMin := r.label.MinSize()
	height := labelMin.Height + compactSelectPadY*2
	if compactSelectIconSize+compactSelectPadY*2 > height {
		height = compactSelectIconSize + compactSelectPadY*2
	}
	return fyne.NewSize(
		maxW+compactSelectIconSize+compactSelectPadX*2+compactSelectGap,
		height,
	)
}

func (r *compactSelectRenderer) Refresh() {
	r.applyTheme()
	canvas.Refresh(r.sel)
}

func (r *compactSelectRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.label, r.arrow}
}

func (r *compactSelectRenderer) Destroy() {}

func (r *compactSelectRenderer) applyTheme() {
	th := r.sel.Theme()
	v := fyne.CurrentApp().Settings().ThemeVariant()

	if r.sel.hovered {
		r.bg.FillColor = th.Color(theme.ColorNameHover, v)
	} else {
		// MenuBackground matches the popup's surface tone, so the
		// dropdown and its expanded list visually line up.
		r.bg.FillColor = th.Color(theme.ColorNameMenuBackground, v)
	}
	r.bg.StrokeColor = th.Color(theme.ColorNameInputBorder, v)

	text := r.sel.Selected
	if text == "" {
		text = r.sel.PlaceHolder
		r.label.ColorName = theme.ColorNamePlaceHolder
	} else {
		r.label.ColorName = theme.ColorNameForeground
	}
	r.label.Text = text

	r.bg.Refresh()
	r.label.Refresh()
	r.arrow.Refresh()
}
