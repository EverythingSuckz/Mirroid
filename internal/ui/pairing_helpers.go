package ui

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math/big"
	"net"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"mirroid/internal/adb"
)

const (
	postPairHintAfter   = 15 * time.Second
	postPairToggleAfter = 30 * time.Second
	postPairWarnAfter   = 60 * time.Second
	scanHintAfter       = 60 * time.Second
)

// the sole connector while the pairing window is open (the mdns watcher
// stands down). watches `adb devices` until the paired device shows up,
// healing dead transports that would block fresh handshakes.
func doPostPairConnect(ctx context.Context, a *App, device *adb.MdnsDevice, guid string, setStatus func(string)) bool {
	ip := parseHostFromAddr(device.Addr)

	// one snapshot serves both the success check and the zombie heal:
	// an "offline" transport makes every `adb connect` short-circuit to
	// "already connected" without re-handshaking the new pairing key
	checkAndHeal := func() bool {
		states := a.adbClient.DeviceStates()
		for serial, state := range states {
			if state == "device" && pairedSerialMatch(serial, ip, guid) {
				a.logsPanel.Log(fmt.Sprintf("[OK]Device connected (%s)", serial))
				return true
			}
		}
		for serial, state := range states {
			// instance-name transports are adb's own auto-connect attempts
			// and read "offline" mid-handshake; dropping them here would
			// abort them every iteration. the sweep reaps real instance
			// zombies later, and our ip:port connect isn't blocked by them.
			if state == "offline" && pairedSerialMatch(serial, ip, guid) &&
				!adb.IsInstanceSerial(serial) {
				a.adbClient.DropTransport(serial)
			}
		}
		return false
	}

	start := time.Now()
	hinted, toggleHinted, warned := false, false, false
	refused := 0

	for {
		if checkAndHeal() {
			return true
		}

		switch elapsed := time.Since(start); {
		case !hinted && elapsed >= postPairHintAfter:
			hinted = true
			setStatus("Still connecting...")
		case !toggleHinted && elapsed >= postPairToggleAfter:
			toggleHinted = true
			setStatus("No connection yet. Toggle Wireless Debugging off and on to reconnect.")
		case !warned && elapsed >= postPairWarnAfter:
			warned = true
			a.logsPanel.Log("[WARN]" + ip + " paired but hasn't connected yet; still watching")
		}

		if found, err := adb.DiscoverDevices(ctx); err == nil {
			for _, cd := range found {
				if parseHostFromAddr(cd.Addr) != ip {
					continue
				}
				err := a.adbClient.Connect(cd.Addr)
				if err == nil {
					refused = 0
					if checkAndHeal() {
						return true
					}
				} else if !errors.Is(err, adb.ErrAlreadyConnected) {
					// still advertising on mdns but refusing tcp/tls:
					// the phone's adbd is wedged, only a toggle revives it
					refused++
					if refused == 3 {
						a.logsPanel.Log("[WARN]" + cd.Addr + " refused the connection: " + err.Error())
						hinted, toggleHinted = true, true
						setStatus("The phone isn't accepting connections. Toggle Wireless Debugging off and on.")
					}
				}
				break
			}
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(1 * time.Second):
		}
	}
}

// matches ip:port serials by host, mdns instance serials by guid prefix
func pairedSerialMatch(serial, ip, guid string) bool {
	if guid != "" && strings.HasPrefix(serial, guid) {
		return true
	}
	host, _, err := net.SplitHostPort(serial)
	return err == nil && ip != "" && host == ip
}

func randomDigits(n int) string {
	result := make([]byte, n)
	for i := range result {
		val, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			panic("crypto/rand failed: " + err.Error())
		}
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
