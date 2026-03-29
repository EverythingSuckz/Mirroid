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
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	qrcode "github.com/skip2/go-qrcode"

	"mirroid/internal/adb"
)

func doPostPairConnect(a *App, device *adb.MdnsDevice, win fyne.Window) {
	var connected bool
	var dialogShown sync.Once

	for attempt := 1; attempt <= 30; attempt++ {
		ip := parseHostFromAddr(device.Addr)
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
