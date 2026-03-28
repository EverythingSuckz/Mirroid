package ui

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"image"
	"image/png"
	"math/big"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	qrcode "github.com/skip2/go-qrcode"

	"mirroid/internal/adb"
)

// PairingWindow manages the independent pairing window.
type PairingWindow struct {
	app    *App
	window fyne.Window

	statusLabel *widget.Label
	activity    *widget.Activity

	qrImage  *canvas.Image
	qrCancel context.CancelFunc

	codeCancel context.CancelFunc
}

// ShowPairingWindow creates and shows the independent pairing window.
// Returns the window reference so callers can track it if needed.
func ShowPairingWindow(a *App) fyne.Window {
	// clear the blocklist (user is intentionally re-pairing)
	for k := range a.ignoredAddrs {
		delete(a.ignoredAddrs, k)
	}

	pw := &PairingWindow{app: a}

	pw.window = a.fyneApp.NewWindow("Pair Device")

	pw.statusLabel = widget.NewLabel("")
	pw.statusLabel.TextStyle = fyne.TextStyle{Italic: true}

	pw.activity = widget.NewActivity()

	qrTab := pw.buildQRTab()
	codeTab := pw.buildCodeTab()

	tabs := container.NewAppTabs(
		container.NewTabItem("QR Code", qrTab),
		container.NewTabItem("Pairing Code", codeTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)
	tabs.OnSelected = func(ti *container.TabItem) {
		if ti.Text == "QR Code" {
			pw.stopCodeScan()
			pw.startQRSession()
		} else {
			pw.stopQRSession()
			pw.startCodeScan()
		}
	}

	statusBar := container.NewBorder(nil, nil, pw.activity, nil, pw.statusLabel)

	content := container.NewBorder(
		nil,
		container.NewVBox(widget.NewSeparator(), container.NewPadded(statusBar)),
		nil, nil,
		tabs,
	)

	pw.window.SetContent(content)
	pw.window.Resize(fyne.NewSize(650, 400))
	pw.window.SetOnClosed(func() {
		pw.stopQRSession()
		pw.stopCodeScan()
	})

	pw.window.Show()
	pw.startQRSession()
	return pw.window
}

func (pw *PairingWindow) buildQRTab() fyne.CanvasObject {
	pw.qrImage = canvas.NewImageFromImage(nil)
	pw.qrImage.FillMode = canvas.ImageFillContain
	pw.qrImage.SetMinSize(fyne.NewSize(200, 200))

	title := widget.NewLabelWithStyle("Scan to pair", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	step1 := widget.NewLabel("1. Open Developer Options")
	step2 := widget.NewLabel("2. Enable Wireless Debugging")
	step3 := widget.NewLabel("3. Tap \"Pair with QR code\"")
	step4 := widget.NewLabel("4. Point camera at this QR code")

	rightSide := container.NewVBox(title, step1, step2, step3, step4)

	innerRow := container.NewHBox(pw.qrImage, layout.NewSpacer(), rightSide)

	return container.NewCenter(innerRow)
}

func (pw *PairingWindow) buildCodeTab() fyne.CanvasObject {
	title := widget.NewLabelWithStyle("Pair with code", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	step1 := widget.NewLabel("1. Open Developer Options")
	step2 := widget.NewLabel("2. Enable Wireless Debugging")
	step3 := widget.NewLabel("3. Tap \"Pair with pairing code\"")
	step4 := widget.NewLabel("4. Wait for device detection")
	step5 := widget.NewLabel("5. Enter the code shown on phone")

	steps := container.NewVBox(title, step1, step2, step3, step4, step5)

	return container.NewCenter(steps)
}

func (pw *PairingWindow) startQRSession() {
	pw.stopQRSession()

	serviceName := fmt.Sprintf("mirroid-%s", randomDigits(6))
	password := randomDigits(8)
	qrContent := fmt.Sprintf("WIFI:T:ADB;S:%s;P:%s;;", serviceName, password)

	pw.app.logsPanel.Log("QR: " + qrContent)

	img, err := generateQRImage(qrContent)
	if err != nil {
		pw.setStatus("Failed to generate QR code")
		return
	}

	fyne.Do(func() {
		pw.qrImage.Image = img
		pw.qrImage.Refresh()
		pw.activity.Start()
		pw.activity.Show()
	})
	pw.setStatus("Waiting for phone to scan...")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	pw.qrCancel = cancel

	go func() {
		defer cancel()

		device, err := adb.WaitForNamedPairingDevice(ctx, serviceName, func(s string) {})
		if err != nil {
			if ctx.Err() == nil {
				pw.setStatus("Timed out. Switch tabs to retry.")
				fyne.Do(func() { pw.activity.Stop() })
			}
			return
		}

		pw.setStatus("Phone detected! Pairing...")
		pw.app.logsPanel.Log(fmt.Sprintf("QR: pairing with %s...", device.Addr))

		if err := pw.app.adbClient.Pair(device.Addr, password); err != nil {
			pw.setStatus("Pairing failed: " + err.Error())
			pw.app.logsPanel.Log("[ERROR]" + err.Error())
			fyne.Do(func() { pw.activity.Stop() })
			return
		}

		pw.app.logsPanel.Log("[OK]Paired!")
		pw.setStatus("Paired! Connecting...")

		doPostPairConnect(pw.app, device, pw.window)

		fyne.Do(func() {
			pw.activity.Stop()
			pw.window.Close()
		})
		pw.app.devicePanel.refreshDevices()
	}()
}

func (pw *PairingWindow) stopQRSession() {
	if pw.qrCancel != nil {
		pw.qrCancel()
		pw.qrCancel = nil
	}
	fyne.Do(func() { pw.activity.Stop() })
}

func (pw *PairingWindow) startCodeScan() {
	pw.stopCodeScan()

	fyne.Do(func() {
		pw.activity.Start()
		pw.activity.Show()
	})
	pw.setStatus("Scanning for devices...")
	pw.app.logsPanel.Log("Code: scanning for pairing devices...")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	pw.codeCancel = cancel

	go func() {
		defer cancel()

		device, err := adb.WaitForPairingDevice(ctx, func(s string) {
			pw.setStatus(s)
		})

		if err != nil {
			if ctx.Err() == nil {
				pw.setStatus("No device found. Switch tabs to retry.")
				pw.app.logsPanel.Log("[ERROR]Code scan: " + err.Error())
			}
			fyne.Do(func() { pw.activity.Stop() })
			return
		}

		pw.app.logsPanel.Log(fmt.Sprintf("Found %s at %s", device.Name, device.Addr))
		fyne.Do(func() {
			pw.activity.Stop()
			pw.showCodeEntryDialog(device)
		})
	}()
}

func (pw *PairingWindow) stopCodeScan() {
	if pw.codeCancel != nil {
		pw.codeCancel()
		pw.codeCancel = nil
	}
	fyne.Do(func() { pw.activity.Stop() })
}

func (pw *PairingWindow) showCodeEntryDialog(device *adb.MdnsDevice) {
	pw.setStatus(fmt.Sprintf("Found %s --enter the code", device.Name))

	codeEntry := widget.NewEntry()
	codeEntry.SetPlaceHolder("6-digit code")
	codeEntry.Validator = func(s string) error {
		if len(s) != 6 {
			return fmt.Errorf("6 digits required")
		}
		for _, c := range s {
			if c < '0' || c > '9' {
				return fmt.Errorf("digits only")
			}
		}
		return nil
	}

	dlg := dialog.NewForm(
		"Enter Pairing Code",
		"Pair", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Code from phone", codeEntry),
		},
		func(ok bool) {
			if !ok {
				pw.startCodeScan()
				return
			}
			fyne.Do(func() { pw.activity.Start() })
			pw.setStatus("Pairing...")
			go func() {
				pw.app.logsPanel.Log(fmt.Sprintf("Pairing with %s...", device.Addr))
				if err := pw.app.adbClient.Pair(device.Addr, codeEntry.Text); err != nil {
					pw.setStatus("Pairing failed: " + err.Error())
					pw.app.logsPanel.Log("[ERROR]" + err.Error())
					fyne.Do(func() { pw.activity.Stop() })
					return
				}
				pw.app.logsPanel.Log("[OK]Paired!")
				pw.setStatus("Paired! Connecting...")
				doPostPairConnect(pw.app, device, pw.window)
				fyne.Do(func() {
					pw.activity.Stop()
					pw.window.Close()
				})
				pw.app.devicePanel.refreshDevices()
			}()
		},
		pw.window,
	)
	dlg.Resize(fyne.NewSize(350, 160))
	dlg.Show()
}

func (pw *PairingWindow) setStatus(s string) {
	fyne.Do(func() { pw.statusLabel.SetText(s) })
}

func doPostPairConnect(a *App, device *adb.MdnsDevice, win fyne.Window) {
	var connected bool
	var dialogShown sync.Once

	for attempt := 1; attempt <= 30; attempt++ {
		ip := device.Addr
		if idx := strings.Index(ip, ":"); idx > 0 {
			ip = ip[:idx]
		}
		existingDevices, _ := a.adbClient.GetDevices()
		for _, d := range existingDevices {
			if strings.Contains(d.Serial, ip) {
				a.logsPanel.Log(fmt.Sprintf("[OK]Device already connected (%s)", d.Serial))
				return
			}
		}

		a.logsPanel.Log(fmt.Sprintf("Connect attempt %d (waiting 1s)...", attempt))
		time.Sleep(1 * time.Second)

		// after 3 failures, show the toggle dialog once
		if attempt == 3 {
			dialogShown.Do(func() {
				a.logsPanel.Log("[WARN]Connection taking long. Try toggling Wireless Debugging.")
				fyne.Do(func() {
					showToggleDebuggingDialog(win)
				})
			})
		}

		connectDevices, err := adb.DiscoverDevices(context.Background())
		if err != nil {
			continue
		}
		for _, cd := range connectDevices {
			if err := a.adbClient.Connect(cd.Addr); err == nil {
				// Verify it's actually reachable
				if a.adbClient.VerifyConnection(cd.Addr) {
					a.logsPanel.Log(fmt.Sprintf("[OK]Connected to %s!", cd.Addr))
					connected = true
					break
				}
				// False positive --disconnect stale entry
				a.adbClient.Disconnect(cd.Addr)
			}
		}
		if connected {
			break
		}
	}
	if !connected {
		a.logsPanel.Log("[WARN]gave up trying to connect after 30 attempts")
	}
}

func showToggleDebuggingDialog(win fyne.Window) {
	content := widget.NewLabel(
		"Connection is taking longer than expected.\n\n" +
			"On your phone:\n" +
			"1. Go to Developer Options\n" +
			"2. Turn OFF Wireless Debugging\n" +
			"3. Turn it back ON\n\n" +
			"The device should reconnect automatically.")

	dlg := dialog.NewCustom("Toggle Wireless Debugging", "OK", content, win)
	dlg.Resize(fyne.NewSize(400, 250))
	dlg.Show()
}

func randomDigits(n int) string {
	result := make([]byte, n)
	for i := range result {
		val, _ := rand.Int(rand.Reader, big.NewInt(10))
		result[i] = byte('0' + val.Int64())
	}
	return string(result)
}

func generateQRImage(text string) (image.Image, error) {
	qr, err := qrcode.New(text, qrcode.Medium)
	if err != nil {
		return nil, err
	}
	qr.DisableBorder = false
	pngBytes, err := qr.PNG(256)
	if err != nil {
		return nil, err
	}
	return png.Decode(bytes.NewReader(pngBytes))
}
