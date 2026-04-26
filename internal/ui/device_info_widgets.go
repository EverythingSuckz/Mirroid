package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/adb"
	"mirroid/internal/icons"
	"mirroid/internal/model"
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

// statusColor maps a DeviceStatus to its pill background color.
func statusColor(s model.DeviceStatus) color.Color {
	switch s {
	case model.StatusConnected:
		return pillGreen
	case model.StatusMirroring:
		return pillBlue
	case model.StatusLaunching, model.StatusReconnecting:
		return pillTeal
	case model.StatusError:
		return pillRed
	default:
		return pillGray
	}
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

// connTypeLabel returns "Wi-Fi" or "USB" based on whether the serial looks
// like an IP:port wireless target. Used by both the device list and the side
// panel hero so the two surfaces stay consistent.
func connTypeLabel(serial string) string {
	if strings.Contains(serial, ":") {
		return "Wi-Fi"
	}
	return "USB"
}

func batteryStatusColor(status adb.BatteryStatus) color.Color {
	switch status {
	case adb.BatteryStatusCharging, adb.BatteryStatusFull:
		return pillGreen
	case adb.BatteryStatusDischarging:
		return pillRed
	default:
		return pillGray
	}
}

// buildHeroHeader returns the connected-device hero block plus refs to the
// dynamic name + address + icon + icon-bg widgets so they can be updated
// in place on refresh.
func buildHeroHeader() (fyne.CanvasObject, *styledText, *styledText, *canvas.Image, *avatarBg) {
	iconBg := newAvatarBg(cardRadius, pillGreen)
	heroIcon := canvas.NewImageFromResource(icons.NewTintedIcon(icons.SmartphoneIcon, color.White))
	heroIcon.SetMinSize(fyne.NewSize(heroIconLarge, heroIconLarge))
	heroIcon.FillMode = canvas.ImageFillContain
	iconBlock := container.NewStack(iconBg, container.NewCenter(heroIcon))
	iconWrapper := container.New(&fixedSizeLayout{width: heroBoxLarge, height: heroBoxLarge}, iconBlock)

	name := newStyledText("", widget.RichTextStyle{
		TextStyle: fyne.TextStyle{Bold: true},
		ColorName: theme.ColorNameForeground,
	})
	name.rt.Truncation = fyne.TextTruncateEllipsis

	address := newStyledText("", widget.RichTextStyle{
		Inline:    true,
		ColorName: theme.ColorNamePlaceHolder,
		SizeName:  theme.SizeNameCaptionText,
	})

	connectedBadge := buildStatusBadge("● Connected", pillGreen)
	// transparent spacer matches RichText's internal left padding (which uses
	// SizeNameInnerPadding) so the pill's bg edge aligns with the text rows above and below.
	badgeLeftPad := canvas.NewRectangle(color.Transparent)
	badgeLeftPad.SetMinSize(fyne.NewSize(theme.Size(theme.SizeNameInnerPadding), 0))
	badgeRow := container.NewHBox(badgeLeftPad, connectedBadge)

	rightSide := container.NewVBox(name.rt, badgeRow, address.rt)

	gap := canvas.NewRectangle(color.Transparent)
	gap.SetMinSize(fyne.NewSize(theme.Padding(), 0))
	iconWithGap := container.NewHBox(iconWrapper, gap)
	return container.NewBorder(nil, nil, iconWithGap, nil, rightSide), name, address, heroIcon, iconBg
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
