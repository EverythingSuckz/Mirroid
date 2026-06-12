package ui

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/adb"
)

const scanHintMsg = "Still scanning... check the phone is in pairing mode and on the same Wi-Fi network."

// PairingWindow manages the independent pairing window.
type PairingWindow struct {
	app    *App
	window fyne.Window

	// cancelled when the window closes
	winCtx    context.Context
	winCancel context.CancelFunc

	// supersedes the previous session's claim on status bar + spinner
	sessionCancel context.CancelFunc

	statusLabel *widget.Label
	activity    *widget.Activity

	qrImage *canvas.Image
}

// ShowPairingWindow creates and shows the independent pairing window.
// Returns the window reference so callers can track it if needed.
func ShowPairingWindow(a *App) fyne.Window {
	// singleton: a second window would also race pairingActive on close
	if a.pairingWin != nil {
		a.pairingWin.RequestFocus()
		return a.pairingWin
	}
	a.pairingActive.Store(true)

	pw := &PairingWindow{app: a}
	pw.winCtx, pw.winCancel = context.WithCancel(context.Background())

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
			pw.startQRSession()
		} else {
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
		a.pairingWin = nil
		a.pairingActive.Store(false)
		pw.winCancel()
	})
	a.pairingWin = pw.window

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
	session := pw.newSession()

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

	go func() {
		// scan for as long as this session is current; no give-up timeout
		hint := time.AfterFunc(scanHintAfter, func() {
			pw.setStatusIf(session, scanHintMsg)
		})
		device, err := adb.WaitForNamedPairingDevice(session, serviceName, func(string) {})
		hint.Stop()
		if err != nil {
			return // session superseded or window closed
		}

		pw.setStatusIf(session, "Phone detected! Pairing...")
		pw.app.logsPanel.Log(fmt.Sprintf("QR: pairing with %s...", device.Addr))

		guid, err := pw.app.adbClient.Pair(device.Addr, password)
		if err != nil {
			pw.app.logsPanel.Log("[ERROR]" + err.Error())
			// the phone has to scan again anyway, so a fresh QR is the retry
			pw.ifSession(session, func() {
				pw.startQRSession()
				pw.setStatus("Pairing failed - scan the new QR code to retry.")
			})
			return
		}

		pw.finishPairing(session, device, guid)
	}()
}

// the watch survives tab switches (window-scoped); ui writes are session-gated
func (pw *PairingWindow) finishPairing(session context.Context, device *adb.MdnsDevice, guid string) {
	pw.app.logsPanel.Log("[OK]Paired!")
	// pairing is an explicit "i want this device": unblock it, and only it
	ip := parseHostFromAddr(device.Addr)
	pw.app.ignoredAddrs.Range(func(key, _ any) bool {
		if k, ok := key.(string); ok && parseHostFromAddr(k) == ip {
			pw.app.ignoredAddrs.Delete(key)
		}
		return true
	})
	pw.setStatusIf(session, "Paired! Connecting...")

	serial := doPostPairConnect(pw.winCtx, pw.app, device, guid, func(s string) {
		pw.setStatusIf(session, s)
	})
	if serial == "" {
		return // window closed
	}
	// the alias block is keyed by hardware id, knowable only now
	if devID := pw.app.adbClient.GetDeviceID(serial); devID != "" {
		pw.app.ignoredAddrs.Delete("devid:" + devID)
	}

	// close unconditionally: the device is connected, the window's job is
	// done. gating this on the session left the window open (and the mdns
	// watcher suppressed) forever after a mid-connect tab switch.
	fyne.Do(func() {
		pw.activity.Stop()
		pw.window.Close()
	})
	pw.app.devicePanel.refreshDevices()
}

func (pw *PairingWindow) startCodeScan() {
	session := pw.newSession()

	fyne.Do(func() {
		pw.activity.Start()
		pw.activity.Show()
	})
	pw.setStatus("Scanning for devices...")
	pw.app.logsPanel.Log("Code: scanning for pairing devices...")

	go func() {
		// scan for as long as this session is current; no give-up timeout
		hint := time.AfterFunc(scanHintAfter, func() {
			pw.setStatusIf(session, scanHintMsg)
		})
		device, err := adb.WaitForPairingDevice(session, func(string) {})
		hint.Stop()
		if err != nil {
			return // session superseded or window closed
		}

		pw.app.logsPanel.Log(fmt.Sprintf("Found %s at %s", device.Name, device.Addr))
		pw.ifSession(session, func() {
			pw.activity.Stop()
			pw.showCodeEntryDialog(session, device)
		})
	}()
}

func (pw *PairingWindow) showCodeEntryDialog(session context.Context, device *adb.MdnsDevice) {
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
				guid, err := pw.app.adbClient.Pair(device.Addr, codeEntry.Text)
				if err != nil {
					pw.app.logsPanel.Log("[ERROR]" + err.Error())
					// the phone is usually still in pairing mode, so the
					// rescan reopens the entry dialog
					pw.ifSession(session, func() {
						pw.startCodeScan()
						pw.setStatus("Pairing failed. Check the code, and that the phone is still in pairing mode.")
					})
					return
				}
				pw.finishPairing(session, device, guid)
			}()
		},
		pw.window,
	)
	codeEntry.OnSubmitted = func(string) { dlg.Submit() }
	dlg.Resize(fyne.NewSize(350, 160))
	dlg.Show()
	pw.window.Canvas().Focus(codeEntry)
}

// cancelled when a newer session starts or the window closes, so stale
// goroutines can't stomp the active session's ui. main thread only.
func (pw *PairingWindow) newSession() context.Context {
	if pw.sessionCancel != nil {
		pw.sessionCancel()
	}
	ctx, cancel := context.WithCancel(pw.winCtx)
	pw.sessionCancel = cancel
	return ctx
}

func (pw *PairingWindow) ifSession(session context.Context, f func()) {
	fyne.Do(func() {
		if session.Err() == nil {
			f()
		}
	})
}

func (pw *PairingWindow) setStatusIf(session context.Context, s string) {
	pw.ifSession(session, func() { pw.statusLabel.SetText(s) })
}

func (pw *PairingWindow) setStatus(s string) {
	fyne.Do(func() { pw.statusLabel.SetText(s) })
}
