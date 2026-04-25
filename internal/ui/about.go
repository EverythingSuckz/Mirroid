package ui

import (
	"fmt"
	"net/url"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/platform"
)

// showAboutDialog shows a modal popup with app info.
func (a *App) showAboutDialog() {
	appIcon := a.fyneApp.Icon()
	icon := canvas.NewImageFromResource(appIcon)
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(72, 72))

	version := a.fyneApp.Metadata().Version
	if version == "" {
		version = "dev"
	}

	title := canvas.NewText("Mirroid", theme.Color(theme.ColorNameForeground))
	title.TextSize = 24
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	versionLabel := widget.NewLabelWithStyle(
		"Version "+version,
		fyne.TextAlignCenter, fyne.TextStyle{Italic: true},
	)

	description := widget.NewLabelWithStyle(
		"A desktop GUI for scrcpy",
		fyne.TextAlignCenter, fyne.TextStyle{},
	)

	// Clickable dependency versions linking to their release pages
	adbVer := getToolVersion(a.cfg.AppConf.ADBPath, "adb", "version")
	scrcpyVer := getToolVersion(a.cfg.AppConf.ScrcpyPath, "scrcpy", "--version")

	adbReleasesURL, _ := url.Parse("https://developer.android.com/tools/releases/platform-tools")
	scrcpyReleasesURL, _ := url.Parse("https://github.com/Genymobile/scrcpy/releases")

	// Single RichText widget avoids the per-widget internal padding that
	// Label/Hyperlink add, giving tight line spacing between rows.
	inlineBold := widget.RichTextStyle{Inline: true, TextStyle: fyne.TextStyle{Bold: true}}
	inlinePlain := widget.RichTextStyle{Inline: true}
	infoText := widget.NewRichText(
		&widget.TextSegment{Text: "ADB:   ", Style: inlineBold},
		&widget.HyperlinkSegment{Text: adbVer, URL: adbReleasesURL},
		&widget.TextSegment{Text: "\nscrcpy:   ", Style: inlineBold},
		&widget.HyperlinkSegment{Text: scrcpyVer, URL: scrcpyReleasesURL},
		&widget.TextSegment{Text: "\nOS:   ", Style: inlineBold},
		&widget.TextSegment{Text: fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH), Style: inlinePlain},
	)

	// GitHub button opens the repo
	ghURL, _ := url.Parse("https://github.com/EverythingSuckz/Mirroid")
	ghBtn := widget.NewButton("GitHub", func() {
		_ = a.fyneApp.OpenURL(ghURL)
	})

	configBtn := widget.NewButton("Open Config Folder", func() {
		if err := platform.OpenFolder(a.cfg.Dir()); err != nil {
			a.logsPanel.Log("[ERROR]Open config: " + err.Error())
		}
	})

	var popup *widget.PopUp

	// Header row: title left, close X right.
	headerLabel := widget.NewLabelWithStyle("About Mirroid", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		popup.Hide()
	})
	header := container.NewBorder(nil, nil, nil, closeBtn, headerLabel)

	// Small spacer between header and separator for breathing room.
	headerSpacer := canvas.NewRectangle(nil)
	headerSpacer.SetMinSize(fyne.NewSize(0, theme.Padding()))

	body := container.NewVBox(
		header,
		headerSpacer,
		widget.NewSeparator(),
		container.NewCenter(icon),
		title,
		versionLabel,
		description,
		widget.NewSeparator(),
		container.NewCenter(infoText),
		widget.NewSeparator(),
		container.NewCenter(container.NewHBox(ghBtn, configBtn)),
	)

	popup = widget.NewModalPopUp(container.NewPadded(body), a.window.Canvas())
	popup.Resize(fyne.NewSize(350, popup.MinSize().Height))
	popup.Show()
}
