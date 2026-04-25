package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/icons"
)

// styledText pairs a RichText widget with its first segment for in-place text updates.
type styledText struct {
	rt  *widget.RichText
	seg *widget.TextSegment
}

func newStyledText(text string, style widget.RichTextStyle) *styledText {
	seg := &widget.TextSegment{Text: text, Style: style}
	return &styledText{rt: widget.NewRichText(seg), seg: seg}
}

func (s *styledText) Set(text string) {
	if s.seg.Text == text {
		return
	}
	s.seg.Text = text
	s.rt.Refresh()
}

func styleHeading() widget.RichTextStyle {
	return widget.RichTextStyle{
		ColorName: theme.ColorNameForeground,
		SizeName:  theme.SizeNameHeadingText,
		TextStyle: fyne.TextStyle{Bold: true},
	}
}

func styleKey() widget.RichTextStyle {
	return widget.RichTextStyle{
		Inline:    true,
		ColorName: theme.ColorNameForeground,
	}
}

func styleValue() widget.RichTextStyle {
	return widget.RichTextStyle{
		Inline:    true,
		ColorName: theme.ColorNameForeground,
		TextStyle: fyne.TextStyle{Bold: true},
		Alignment: fyne.TextAlignTrailing,
	}
}

func styleStat() widget.RichTextStyle {
	return widget.RichTextStyle{
		Alignment: fyne.TextAlignCenter,
		ColorName: theme.ColorNameForeground,
		SizeName:  theme.SizeNameSubHeadingText,
		TextStyle: fyne.TextStyle{Bold: true},
	}
}

func styleStatLabel() widget.RichTextStyle {
	return widget.RichTextStyle{
		Alignment: fyne.TextAlignCenter,
		ColorName: theme.ColorNamePlaceHolder,
		SizeName:  theme.SizeNameCaptionText,
	}
}

// kvRow returns a card-style "key: value" row with the value as a styledText
// so the value can be updated in place.
func kvRow(key string) (*styledText, fyne.CanvasObject) {
	keyText := newStyledText(key, styleKey())
	val := newStyledText("", styleValue())
	return val, container.NewBorder(nil, nil, keyText.rt, nil, val.rt)
}

// statBlock returns a centered value-over-label stat with the value as a styledText.
func statBlock(label string) (*styledText, fyne.CanvasObject) {
	val := newStyledText("", styleStat())
	lbl := newStyledText(label, styleStatLabel())
	return val, container.NewVBox(val.rt, lbl.rt)
}

// statBlockTruncating is like statBlock but the value truncates with ellipsis
// on overflow so very long content (e.g. build IDs) doesn't drag the column.
func statBlockTruncating(label string) (*styledText, fyne.CanvasObject) {
	val := newStyledText("", styleStat())
	val.rt.Truncation = fyne.TextTruncateEllipsis
	lbl := newStyledText(label, styleStatLabel())
	return val, container.NewVBox(val.rt, lbl.rt)
}

func buildSectionLabel(icon fyne.Resource, title string) fyne.CanvasObject {
	img := canvas.NewImageFromResource(icons.NewThemedIcon(icon))
	img.SetMinSize(fyne.NewSize(sectionIconSize, sectionIconSize))
	img.FillMode = canvas.ImageFillContain
	label := widget.NewRichText(&widget.TextSegment{
		Text: strings.ToUpper(title),
		Style: widget.RichTextStyle{
			ColorName: theme.ColorNamePlaceHolder,
			SizeName:  theme.SizeNameCaptionText,
			TextStyle: fyne.TextStyle{Bold: true},
		},
	})
	return container.NewHBox(img, label)
}

func buildCard(content fyne.CanvasObject) fyne.CanvasObject {
	bg := newThemedRect(theme.ColorNameHeaderBackground, cardRadius)
	return container.NewStack(bg, container.NewPadded(content))
}

func buildThinBar(pct float64) (*fyne.Container, *progressBarLayout) {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	layout := &progressBarLayout{pct: pct}
	bg := newThemedRect(theme.ColorNameSeparator, progressBarHeight/2)
	fg := newThemedRect(theme.ColorNamePrimary, progressBarHeight/2)
	return container.New(layout, bg, fg), layout
}

func buildStatusBadge(text string, pillColor color.Color) fyne.CanvasObject {
	bg := canvas.NewRectangle(pillColor)
	bg.CornerRadius = badgeRadius
	label := canvas.NewText(text, color.White)
	label.TextSize = badgeTextSize
	badge := container.New(&badgeLayout{padX: statusBadgePadX, padY: badgePadY}, bg, label)
	return container.NewCenter(badge)
}

func buildInfoPill(icon fyne.Resource, text string, pillColor color.Color) fyne.CanvasObject {
	bg := canvas.NewRectangle(pillColor)
	bg.CornerRadius = pillRadius
	img := canvas.NewImageFromResource(icons.NewTintedIcon(icon, color.White))
	img.SetMinSize(fyne.NewSize(pillIconSize, pillIconSize))
	img.FillMode = canvas.ImageFillContain
	label := canvas.NewText(text, color.White)
	label.TextSize = pillTextSize
	row := container.NewHBox(img, label)
	return container.New(&badgeLayout{padX: pillPadX, padY: badgePadY}, bg, row)
}

func batteryStatusColor(status string) color.Color {
	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "charging"), strings.Contains(lower, "full"):
		return pillGreen
	case strings.Contains(lower, "discharging"):
		return pillRed
	default:
		return pillGray
	}
}

// buildHeroHeader returns the connected-device hero block plus refs to the
// dynamic name + address widgets so they can be updated in place on refresh.
func buildHeroHeader() (fyne.CanvasObject, *widget.Label, *styledText) {
	iconBg := canvas.NewRectangle(pillGreen)
	iconBg.CornerRadius = cardRadius
	phoneIcon := canvas.NewImageFromResource(icons.NewThemedIcon(icons.SmartphoneIcon))
	phoneIcon.SetMinSize(fyne.NewSize(heroIconLarge, heroIconLarge))
	phoneIcon.FillMode = canvas.ImageFillContain
	iconBlock := container.NewStack(iconBg, container.NewCenter(phoneIcon))
	iconWrapper := container.New(&fixedSizeLayout{width: heroBoxLarge, height: heroBoxLarge}, iconBlock)

	nameLabel := widget.NewLabel("")
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}
	nameLabel.Truncation = fyne.TextTruncateEllipsis

	address := newStyledText("", widget.RichTextStyle{
		Inline:    true,
		ColorName: theme.ColorNamePlaceHolder,
		SizeName:  theme.SizeNameCaptionText,
	})

	connectedBadge := buildStatusBadge("● Connected", pillGreen)

	addressRow := container.NewHBox(address.rt, connectedBadge)
	rightSide := container.NewVBox(nameLabel, addressRow)

	gap := canvas.NewRectangle(color.Transparent)
	gap.SetMinSize(fyne.NewSize(theme.Padding(), 0))
	iconWithGap := container.NewHBox(iconWrapper, gap)
	return container.NewBorder(nil, nil, iconWithGap, nil, rightSide), nameLabel, address
}

func buildSmallHero(name, badgeText string, accent color.Color) fyne.CanvasObject {
	iconBg := canvas.NewRectangle(accent)
	iconBg.CornerRadius = cardRadius
	phoneIcon := canvas.NewImageFromResource(icons.NewThemedIcon(icons.SmartphoneIcon))
	phoneIcon.SetMinSize(fyne.NewSize(heroIconSmall, heroIconSmall))
	phoneIcon.FillMode = canvas.ImageFillContain
	iconBlock := container.NewStack(iconBg, container.NewCenter(phoneIcon))
	iconWrapper := container.New(&fixedSizeLayout{width: heroBoxSmall, height: heroBoxSmall}, iconBlock)

	nameLabel := widget.NewLabel(name)
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}
	nameLabel.Truncation = fyne.TextTruncateEllipsis

	badge := buildStatusBadge(badgeText, accent)

	return container.NewBorder(nil, nil, iconWrapper, nil,
		container.NewVBox(nameLabel, badge),
	)
}
