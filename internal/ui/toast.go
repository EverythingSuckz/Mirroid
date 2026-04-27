package ui

import (
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type ToastVariant int

const (
	ToastInfo ToastVariant = iota
	ToastSuccess
	ToastWarning
	ToastError
)

const (
	toastWidth         float32 = 360
	toastMargin        float32 = 16
	toastGap           float32 = 8
	toastCornerRadius  float32 = 8
	toastDotSize       float32 = 10
	toastPadding       float32 = 12
	toastDrainHeight   float32 = 3
	toastSlideDuration         = 220 * time.Millisecond
	toastDefaultLife           = 5 * time.Second
	toastErrorLife             = 8 * time.Second
)

func toastAccentColor(v ToastVariant) color.Color {
	switch v {
	case ToastSuccess:
		return color.NRGBA{R: 0x4c, G: 0xaf, B: 0x50, A: 0xff}
	case ToastWarning:
		return color.NRGBA{R: 0xff, G: 0xa3, B: 0x1a, A: 0xff}
	case ToastError:
		return color.NRGBA{R: 0xef, G: 0x53, B: 0x50, A: 0xff}
	default:
		return color.NRGBA{R: 0x21, G: 0x96, B: 0xf3, A: 0xff}
	}
}

type paddedLayout struct{ pad float32 }

func (p *paddedLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	m := objects[0].MinSize()
	return fyne.NewSize(m.Width+2*p.pad, m.Height+2*p.pad)
}

func (p *paddedLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	objects[0].Resize(fyne.NewSize(size.Width-2*p.pad, size.Height-2*p.pad))
	objects[0].Move(fyne.NewPos(p.pad, p.pad))
}

// drainBarLayout insets the bar horizontally so its edges stay inside the
// card's rounded corners.
type drainBarLayout struct {
	progress float32
	inset    float32
}

func (d *drainBarLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(0, toastDrainHeight)
}

func (d *drainBarLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	available := size.Width - 2*d.inset
	if available < 0 {
		available = 0
	}
	width := available * d.progress
	if width < 0 {
		width = 0
	}
	objects[0].Resize(fyne.NewSize(width, toastDrainHeight))
	objects[0].Move(fyne.NewPos(d.inset, 0))
}

type toast struct {
	root      *fyne.Container
	height    float32
	manager   *ToastManager
	dismissed bool

	// Stop the previous animation before starting a new one so they don't
	// fight for control of t.root.Move per frame.
	posAnim *fyne.Animation

	drainAnim      *fyne.Animation
	drainContainer *fyne.Container
	drainLayout    *drainBarLayout
}

func (t *toast) startPosAnim(a *fyne.Animation) {
	if t.posAnim != nil {
		t.posAnim.Stop()
	}
	t.posAnim = a
	a.Start()
}

type ToastManager struct {
	canvas fyne.Canvas
	host   *fyne.Container
	mu     sync.Mutex
	toasts []*toast
}

func newToastManager(c fyne.Canvas) *ToastManager {
	return &ToastManager{
		canvas: c,
		host:   container.NewWithoutLayout(),
	}
}

func (m *ToastManager) Show(title, message string, variant ToastVariant) {
	t := m.buildToast(title, message, variant)

	life := toastDefaultLife
	if variant == ToastError || variant == ToastWarning {
		life = toastErrorLife
	}

	m.mu.Lock()
	m.toasts = append(m.toasts, t)
	hostW := m.canvas.Size().Width
	finalX := hostW - toastWidth - toastMargin
	y := toastMargin
	for i := 0; i < len(m.toasts)-1; i++ {
		y += m.toasts[i].height + toastGap
	}
	startPos := fyne.NewPos(hostW+8, y)
	endPos := fyne.NewPos(finalX, y)
	t.root.Resize(fyne.NewSize(toastWidth, t.height))
	t.root.Move(startPos)
	m.host.Add(t.root)
	// host.Add doesn't mark the canvas dirty; without this Refresh, t.root
	// never enters the canvas's render cache and animation Moves run
	// invisibly (their repaint can't find the canvas).
	m.canvas.Refresh(m.host)
	m.mu.Unlock()

	slide := canvas.NewPositionAnimation(startPos, endPos, toastSlideDuration, func(p fyne.Position) {
		t.root.Move(p)
	})
	slide.Curve = fyne.AnimationEaseOut
	t.startPosAnim(slide)

	drain := fyne.NewAnimation(life, func(done float32) {
		t.drainLayout.progress = 1 - done
		t.drainContainer.Refresh()
	})
	drain.Curve = fyne.AnimationLinear
	drain.Start()
	t.drainAnim = drain

	go func() {
		time.Sleep(life)
		fyne.Do(func() { t.dismiss() })
	}()
}

func (m *ToastManager) buildToast(title, message string, variant ToastVariant) *toast {
	bg := canvas.NewRectangle(theme.Color(theme.ColorNameOverlayBackground))
	bg.CornerRadius = toastCornerRadius
	bg.StrokeColor = theme.Color(theme.ColorNameSeparator)
	bg.StrokeWidth = 1

	dot := canvas.NewRectangle(toastAccentColor(variant))
	dot.CornerRadius = toastDotSize / 2
	dotBox := container.New(&fixedSizeLayout{width: toastDotSize, height: toastDotSize}, dot)

	titleLbl := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	t := &toast{}
	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() { t.dismiss() })
	closeBtn.Importance = widget.LowImportance

	header := container.NewBorder(nil, nil,
		container.NewHBox(container.NewCenter(dotBox), titleLbl),
		closeBtn,
	)

	var content fyne.CanvasObject
	if message != "" {
		body := widget.NewLabel(message)
		body.Wrapping = fyne.TextWrapWord
		content = container.NewVBox(header, body)
	} else {
		content = header
	}

	paddedContent := container.New(&paddedLayout{pad: toastPadding}, content)

	drainBar := canvas.NewRectangle(toastAccentColor(variant))
	drainLayoutInst := &drainBarLayout{progress: 1.0, inset: toastCornerRadius}
	drainContainer := container.New(drainLayoutInst, drainBar)

	inner := container.NewBorder(nil, drainContainer, nil, nil, paddedContent)
	root := container.NewStack(bg, inner)

	// Resize once so the wrapping label can compute its real height before
	// MinSize is read.
	root.Resize(fyne.NewSize(toastWidth, root.MinSize().Height))
	min := root.MinSize()

	t.root = root
	t.height = min.Height
	t.manager = m
	t.drainContainer = drainContainer
	t.drainLayout = drainLayoutInst
	return t
}

func (t *toast) dismiss() {
	t.manager.mu.Lock()
	if t.dismissed {
		t.manager.mu.Unlock()
		return
	}
	t.dismissed = true
	hostW := t.manager.canvas.Size().Width
	t.manager.mu.Unlock()

	if t.drainAnim != nil {
		t.drainAnim.Stop()
	}

	current := t.root.Position()
	endPos := fyne.NewPos(hostW+8, current.Y)
	slide := canvas.NewPositionAnimation(current, endPos, toastSlideDuration, func(p fyne.Position) {
		t.root.Move(p)
	})
	slide.Curve = fyne.AnimationEaseIn
	t.startPosAnim(slide)

	go func() {
		time.Sleep(toastSlideDuration)
		fyne.Do(func() { t.manager.removeToast(t) })
	}()
}

func (m *ToastManager) removeToast(target *toast) {
	m.mu.Lock()
	defer m.mu.Unlock()

	idx := -1
	for i, x := range m.toasts {
		if x == target {
			idx = i
			break
		}
	}
	if idx == -1 {
		return
	}
	m.toasts = append(m.toasts[:idx], m.toasts[idx+1:]...)
	m.host.Remove(target.root)

	hostW := m.canvas.Size().Width
	finalX := hostW - toastWidth - toastMargin
	y := toastMargin
	for _, tt := range m.toasts {
		// skip dismissing toasts so they keep their slide-out trajectory.
		if tt.dismissed {
			continue
		}
		current := tt.root.Position()
		newPos := fyne.NewPos(finalX, y)
		if current != newPos {
			tt := tt
			anim := canvas.NewPositionAnimation(current, newPos, toastSlideDuration, func(p fyne.Position) {
				tt.root.Move(p)
			})
			anim.Curve = fyne.AnimationEaseOut
			tt.startPosAnim(anim)
		}
		y += tt.height + toastGap
	}
}
