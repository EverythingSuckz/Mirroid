package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

const (
	rotatingIconSrcSize  = 64
	rotatingIconDuration = 900 * time.Millisecond
)

type rotatingIcon struct {
	widget.BaseWidget
	src fyne.Resource

	mu        sync.Mutex
	base      *image.RGBA
	baseColor [4]uint32
	rotated   *image.RGBA
	img       *canvas.Image
	angle     float32
	anim      *fyne.Animation
}

func newRotatingIcon(src fyne.Resource) *rotatingIcon {
	r := &rotatingIcon{
		src:     src,
		rotated: image.NewRGBA(image.Rect(0, 0, rotatingIconSrcSize, rotatingIconSrcSize)),
	}
	r.ExtendBaseWidget(r)
	r.img = canvas.NewImageFromImage(r.rotated)
	r.img.FillMode = canvas.ImageFillContain
	s := theme.IconInlineSize()
	r.img.SetMinSize(fyne.NewSize(s, s))
	r.render()
	return r
}

func (r *rotatingIcon) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.img)
}

func (r *rotatingIcon) Start() {
	r.mu.Lock()
	if r.anim != nil {
		r.mu.Unlock()
		return
	}
	r.anim = fyne.NewAnimation(rotatingIconDuration, func(progress float32) {
		r.mu.Lock()
		r.angle = progress * 360
		r.mu.Unlock()
		r.render()
	})
	r.anim.RepeatCount = fyne.AnimationRepeatForever
	a := r.anim
	r.mu.Unlock()
	a.Start()
}

func (r *rotatingIcon) Stop() {
	r.mu.Lock()
	a := r.anim
	r.anim = nil
	r.angle = 0
	r.mu.Unlock()
	if a != nil {
		a.Stop()
	}
	r.render()
}

func (r *rotatingIcon) render() {
	fg := theme.Color(theme.ColorNameForeground)

	r.mu.Lock()
	key := colorRGBAKey(fg)
	if r.base == nil || r.baseColor != key {
		r.base = rasterizeSVG(r.src, fg, rotatingIconSrcSize)
		r.baseColor = key
	}
	angle := r.angle
	r.mu.Unlock()

	for i := range r.rotated.Pix {
		r.rotated.Pix[i] = 0
	}

	cx := float64(rotatingIconSrcSize) / 2
	cy := float64(rotatingIconSrcSize) / 2
	rad := float64(angle) * math.Pi / 180
	cos, sin := math.Cos(rad), math.Sin(rad)
	m := f64.Aff3{
		cos, -sin, cx - cos*cx + sin*cy,
		sin, cos, cy - sin*cx - cos*cy,
	}
	xdraw.BiLinear.Transform(r.rotated, m, r.base, r.base.Bounds(), xdraw.Over, nil)

	canvas.Refresh(r.img)
}

func rasterizeSVG(res fyne.Resource, fg color.Color, size int) *image.RGBA {
	content := strings.ReplaceAll(string(res.Content()), "currentColor", rgbaToHex(fg))
	icon, err := oksvg.ReadIconStream(bytes.NewReader([]byte(content)))
	if err != nil {
		return image.NewRGBA(image.Rect(0, 0, size, size))
	}
	icon.SetTarget(0, 0, float64(size), float64(size))
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	scanner := rasterx.NewScannerGV(size, size, img, img.Bounds())
	raster := rasterx.NewDasher(size, size, scanner)
	icon.Draw(raster, 1.0)
	return img
}

func rgbaToHex(c color.Color) string {
	nr := color.NRGBAModel.Convert(c).(color.NRGBA)
	return fmt.Sprintf("#%02x%02x%02x", nr.R, nr.G, nr.B)
}

func colorRGBAKey(c color.Color) [4]uint32 {
	r, g, b, a := c.RGBA()
	return [4]uint32{r, g, b, a}
}
