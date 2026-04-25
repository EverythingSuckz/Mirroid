package ui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/adb"
	"mirroid/internal/icons"
)

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

func buildCardKV(key, value string) fyne.CanvasObject {
	keyWidget := widget.NewRichText(&widget.TextSegment{
		Text: key,
		Style: widget.RichTextStyle{
			Inline:    true,
			ColorName: theme.ColorNameForeground,
		},
	})
	valueWidget := widget.NewRichText(&widget.TextSegment{
		Text: value,
		Style: widget.RichTextStyle{
			Inline:    true,
			ColorName: theme.ColorNameForeground,
			TextStyle: fyne.TextStyle{Bold: true},
			Alignment: fyne.TextAlignTrailing,
		},
	})
	return container.NewBorder(nil, nil, keyWidget, nil, valueWidget)
}

func buildThinBar(pct float64) fyne.CanvasObject {
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	bg := newThemedRect(theme.ColorNameSeparator, progressBarHeight/2)
	fg := newThemedRect(theme.ColorNamePrimary, progressBarHeight/2)
	return container.New(&progressBarLayout{pct: pct}, bg, fg)
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

func buildStatBlock(value, label string) fyne.CanvasObject {
	valueText := widget.NewRichText(&widget.TextSegment{
		Text: value,
		Style: widget.RichTextStyle{
			Alignment: fyne.TextAlignCenter,
			ColorName: theme.ColorNameForeground,
			SizeName:  theme.SizeNameSubHeadingText,
			TextStyle: fyne.TextStyle{Bold: true},
		},
	})
	labelText := widget.NewRichText(&widget.TextSegment{
		Text: label,
		Style: widget.RichTextStyle{
			Alignment: fyne.TextAlignCenter,
			ColorName: theme.ColorNamePlaceHolder,
			SizeName:  theme.SizeNameCaptionText,
		},
	})
	return container.NewVBox(valueText, labelText)
}

func buildHeroHeader(info adb.DeviceInfo) fyne.CanvasObject {
	iconBg := canvas.NewRectangle(pillGreen)
	iconBg.CornerRadius = cardRadius
	phoneIcon := canvas.NewImageFromResource(icons.NewThemedIcon(icons.SmartphoneIcon))
	phoneIcon.SetMinSize(fyne.NewSize(heroIconLarge, heroIconLarge))
	phoneIcon.FillMode = canvas.ImageFillContain
	iconBlock := container.NewStack(iconBg, container.NewCenter(phoneIcon))
	iconWrapper := container.New(&fixedSizeLayout{width: heroBoxLarge, height: heroBoxLarge}, iconBlock)

	name := fmt.Sprintf("%s %s", info.Manufacturer, info.Model)
	nameLabel := widget.NewLabel(name)
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}
	nameLabel.Truncation = fyne.TextTruncateEllipsis

	addressText := widget.NewRichText(&widget.TextSegment{
		Text: info.Serial,
		Style: widget.RichTextStyle{
			Inline:    true,
			ColorName: theme.ColorNamePlaceHolder,
			SizeName:  theme.SizeNameCaptionText,
		},
	})

	connectedBadge := buildStatusBadge("● Connected", pillGreen)

	addressRow := container.NewHBox(addressText, connectedBadge)
	rightSide := container.NewVBox(nameLabel, addressRow)

	gap := canvas.NewRectangle(color.Transparent)
	gap.SetMinSize(fyne.NewSize(theme.Padding(), 0))
	iconWithGap := container.NewHBox(iconWrapper, gap)
	return container.NewBorder(nil, nil, iconWithGap, nil, rightSide)
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
