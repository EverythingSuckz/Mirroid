package ui

import (
	"image/png"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/platform"
)

const (
	screenshotWindowWidth  = 500
	screenshotWindowHeight = 700
	screenshotMinWidth     = 400
	screenshotMinHeight    = 600
)

// takeScreenshot captures the screen, pulls to temp, and shows preview window.
func (dip *DeviceInfoPanel) takeScreenshot(serial string) {
	dip.app.logsPanel.Log("Taking screenshot...")

	tmp, err := os.CreateTemp("", "mirroid_screenshot-*.png")
	if err != nil {
		dip.app.logsPanel.Log("[ERROR]Failed to create temp file: " + err.Error())
		return
	}
	tmpFile := tmp.Name()
	tmp.Close()

	if err := dip.app.adbClient.TakeScreenshot(serial, tmpFile); err != nil {
		dip.app.logsPanel.Log("[ERROR]Screenshot failed: " + err.Error())
		os.Remove(tmpFile)
		return
	}

	dip.app.logsPanel.Log("[OK]Screenshot captured")

	f, err := os.Open(tmpFile)
	if err != nil {
		dip.app.logsPanel.Log("[ERROR]Failed to open screenshot: " + err.Error())
		os.Remove(tmpFile)
		return
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		dip.app.logsPanel.Log("[ERROR]Failed to decode screenshot: " + err.Error())
		os.Remove(tmpFile)
		return
	}

	fyne.Do(func() {
		screenshotWin := dip.app.fyneApp.NewWindow("Screenshot")
		screenshotWin.Resize(fyne.NewSize(screenshotWindowWidth, screenshotWindowHeight))
		screenshotWin.SetOnClosed(func() {
			os.Remove(tmpFile)
		})

		imgWidget := canvas.NewImageFromImage(img)
		imgWidget.FillMode = canvas.ImageFillContain
		imgWidget.SetMinSize(fyne.NewSize(screenshotMinWidth, screenshotMinHeight))

		copyBtn := widget.NewButtonWithIcon("Copy to Clipboard", theme.ContentCopyIcon(), func() {
			go func() {
				if err := platform.CopyImageToClipboard(tmpFile); err != nil {
					dip.app.logsPanel.Log("[ERROR]Copy failed: " + err.Error())
				} else {
					dip.app.logsPanel.Log("[OK]Screenshot copied to clipboard")
				}
			}()
		})

		saveBtn := widget.NewButtonWithIcon("Save As...", theme.DocumentSaveIcon(), func() {
			fd := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
				if err != nil || writer == nil {
					return
				}
				defer writer.Close()

				data, readErr := os.ReadFile(tmpFile)
				if readErr != nil {
					dip.app.logsPanel.Log("[ERROR]Read failed: " + readErr.Error())
					return
				}
				if _, writeErr := writer.Write(data); writeErr != nil {
					dip.app.logsPanel.Log("[ERROR]Save failed: " + writeErr.Error())
					return
				}
				dip.app.logsPanel.Log("[OK]Screenshot saved to " + writer.URI().Path())
			}, screenshotWin)
			fd.SetFilter(storage.NewExtensionFileFilter([]string{".png"}))
			fd.SetFileName("screenshot.png")
			fd.Show()
		})
		saveBtn.Importance = widget.HighImportance

		toolbar := container.NewHBox(layout.NewSpacer(), copyBtn, saveBtn, layout.NewSpacer())

		screenshotWin.SetContent(container.NewBorder(
			nil,
			container.NewPadded(toolbar),
			nil, nil,
			container.NewPadded(imgWidget),
		))
		screenshotWin.Show()
	})
}
