package ui

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"mirroid/internal/adb"
)

// after a failed auto-connect, leave the address alone for a while; doomed
// 1/s retries churn adb's transport table and can wedge the phone's adbd
const mdnsConnectBackoff = 60 * time.Second

// how long a transport must sit "offline" before the sweep drops it;
// in-flight handshakes also read "offline", so patience keeps them safe
const zombieGrace = 15 * time.Second

// parseHostFromAddr extracts the host/IP from an address that may include a port.
// Returns addr unchanged if no port is present.
func parseHostFromAddr(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// deviceFriendlyName returns a human-readable name, prefixing manufacturer
// when it isn't already part of the model string.
func deviceFriendlyName(d adb.Device) string {
	name := d.Model
	if name == "" {
		name = d.Serial
	}
	if d.Manufacturer != "" &&
		!strings.HasPrefix(strings.ToLower(name), strings.ToLower(d.Manufacturer)) {
		name = d.Manufacturer + " " + name
	}
	return name
}

// ReconnectDevice attempts to reconnect a disconnected device.
func (dp *DevicePanel) ReconnectDevice(serial string) {
	dp.mu.Lock()
	if dp.reconnectingSet[serial] {
		dp.mu.Unlock()
		return // already reconnecting
	}
	dp.reconnectingSet[serial] = true
	delete(dp.reconnectErrors, serial)
	dp.mu.Unlock()

	// clear ignoredAddrs so refreshDevices won't filter the device back out
	dp.app.ignoredAddrs.Delete(serial)
	if host := parseHostFromAddr(serial); host != serial {
		dp.app.ignoredAddrs.Delete(host)
	}

	dp.updateList()

	go func() {
		dp.app.logsPanel.Log("Reconnecting to " + serial + "...")
		err := dp.app.adbClient.Connect(serial)
		if errors.Is(err, adb.ErrAlreadyConnected) {
			// a live transport counts as success; a dead one is dropped
			// so the retry gets a fresh handshake
			if dp.app.adbClient.TransportState(serial) == "device" {
				err = nil
			} else {
				dp.app.adbClient.DropTransport(serial)
				err = dp.app.adbClient.Connect(serial)
				if errors.Is(err, adb.ErrAlreadyConnected) {
					// a racing connector (watcher, adb auto-connect)
					// recreated the transport; give its handshake a
					// moment before judging success or failure
					for i := 0; i < 6; i++ {
						if dp.app.adbClient.TransportState(serial) == "device" {
							err = nil
							break
						}
						time.Sleep(500 * time.Millisecond)
					}
				}
			}
		}
		if err == nil {
			// the user explicitly wants this device back; clear any alias
			// block left from a disconnect-then-failed-repair sequence
			if devID := dp.app.adbClient.GetDeviceID(serial); devID != "" {
				dp.app.ignoredAddrs.Delete("devid:" + devID)
			}
		}

		dp.mu.Lock()
		delete(dp.reconnectingSet, serial)
		if err != nil {
			dp.reconnectErrors[serial] = err.Error()
		}
		dp.mu.Unlock()

		if err != nil {
			dp.app.logsPanel.Log("[ERROR]Reconnect " + serial + ": " + err.Error())
			target := serial
			if dev, ok := dp.GetDevice(serial); ok && dev.Model != "" {
				target = deviceFriendlyName(dev) + " at " + serial
			}
			msg := "Couldn't reach " + target + ". The device may be offline, or its wireless debugging port may have changed."
			dp.app.Toast("Reconnect failed", msg, ToastError)
		} else {
			dp.app.logsPanel.Log("[OK]Reconnected " + serial)
		}

		dp.refreshDevices()

		// reload info panel if this device is selected
		dp.mu.Lock()
		selected := dp.lastSelected == serial
		dp.mu.Unlock()
		if selected && dp.app.deviceInfoPanel != nil {
			dp.app.deviceInfoPanel.LoadDeviceInfo(serial)
		}
	}()
}

// RemoveDevice permanently removes a device from the known list.
func (dp *DevicePanel) RemoveDevice(serial string) {
	dp.mu.Lock()
	for i, d := range dp.devices {
		if d.Serial == serial {
			dp.devices = append(dp.devices[:i], dp.devices[i+1:]...)
			break
		}
	}
	delete(dp.connectedSet, serial)
	delete(dp.checkedSerials, serial)
	if dp.lastSelected == serial {
		dp.lastSelected = ""
	}
	dp.mu.Unlock()

	dp.saveKnownDevices()
	dp.updateList()
}

// OnMdnsDevices handles devices discovered via mDNS. it only auto-connects
// hosts of devices seen before (i.e. already paired): hammering a not-yet-
// paired phone with doomed connects poisons adb's transport table and is
// what used to stall post-pair connects until a wireless debugging toggle.
func (dp *DevicePanel) OnMdnsDevices(mdnsDevices []adb.MdnsDevice) {
	if dp.app.pairingActive.Load() {
		return // the pairing flow owns connects while its window is open
	}
	for _, md := range mdnsDevices {
		ip := parseHostFromAddr(md.Addr)
		if dp.app.isIgnored(ip) {
			continue
		}

		dp.mu.Lock()
		known := dp.knownHostLocked(ip)
		if !known && md.Name != "" {
			// a device saved under its mdns instance serial matches the
			// announcement by name; connecting by ip repairs its entry
			for _, d := range dp.devices {
				if strings.HasPrefix(d.Serial, md.Name) {
					known = true
					break
				}
			}
		}
		skip := dp.connectedSet[md.Addr] || !known ||
			time.Now().Before(dp.connectBackoff[md.Addr])
		dp.mu.Unlock()
		if skip {
			continue
		}

		// `adb connect` won't re-handshake a stale offline transport
		if dp.app.adbClient.TransportState(md.Addr) == "offline" {
			dp.app.adbClient.DropTransport(md.Addr)
		}
		err := dp.app.adbClient.Connect(md.Addr)
		already := errors.Is(err, adb.ErrAlreadyConnected)
		if already && dp.app.adbClient.TransportState(md.Addr) == "device" {
			err = nil // live transport, connectedSet is just stale
		}
		if err != nil {
			slog.Debug("mdns auto-connect failed", "addr", md.Addr, "error", err)
			dp.setBackoff(md.Addr)
			continue
		}
		// verify this isn't an alias of a device the user disconnected
		if devID := dp.app.adbClient.GetDeviceID(md.Addr); devID != "" && dp.app.isIgnored("devid:"+devID) {
			_ = dp.app.adbClient.Disconnect(md.Addr)
			dp.setBackoff(md.Addr) // don't re-dial an ignored alias every tick
			continue
		}
		if !already {
			dp.app.logsPanel.Log(fmt.Sprintf("mDNS: Connected to %s", md.Addr))
			go dp.refreshDevices()
		}
	}
}

func (dp *DevicePanel) setBackoff(addr string) {
	dp.mu.Lock()
	dp.connectBackoff[addr] = time.Now().Add(mdnsConnectBackoff)
	dp.mu.Unlock()
}

// knownHostLocked reports whether any known device lives at this host. the
// stored Host covers devices whose serial was rewritten to an mdns instance
// name (which carries no ip).
func (dp *DevicePanel) knownHostLocked(ip string) bool {
	for _, d := range dp.devices {
		if d.Host == ip || parseHostFromAddr(d.Serial) == ip {
			return true
		}
	}
	return false
}

// sweepZombieTransports drops network transports stuck "offline" past the
// grace period. dead wireless transports pile up (one per rotated port) and
// make every `adb connect` to that address short-circuit to "already
// connected" without a handshake; instance-name zombies block adb's own
// auto-connect the same way. states comes from the refresh's own
// `adb devices` run, so the sweep costs no extra subprocess.
func (dp *DevicePanel) sweepZombieTransports(states map[string]string) {
	if states == nil || dp.app.pairingActive.Load() {
		return // no data, or the pairing flow heals its own device
	}
	now := time.Now()

	dp.mu.Lock()
	var drop []string
	for serial, state := range states {
		host := parseHostFromAddr(serial)
		sweepable := (host != serial && dp.knownHostLocked(host)) ||
			adb.IsInstanceSerial(serial)
		if state != "offline" || !sweepable {
			delete(dp.zombieSince, serial)
			continue
		}
		if first, seen := dp.zombieSince[serial]; !seen {
			dp.zombieSince[serial] = now
		} else if now.Sub(first) >= zombieGrace {
			drop = append(drop, serial)
			delete(dp.zombieSince, serial)
		}
	}
	for serial := range dp.zombieSince {
		if _, ok := states[serial]; !ok {
			delete(dp.zombieSince, serial)
		}
	}
	dp.mu.Unlock()

	if len(drop) == 0 {
		return
	}
	// drops poll adb for up to ~2s each; don't stall the refresh path
	go func() {
		for _, serial := range drop {
			dp.app.logsPanel.Log("Dropping stale transport " + serial)
			dp.app.adbClient.DropTransport(serial)
		}
	}()
}

// saveKnownDevices persists the known device list to config.
func (dp *DevicePanel) saveKnownDevices() {
	if dp.app.cfg == nil {
		return
	}
	dp.mu.Lock()
	devices := make([]adb.Device, len(dp.devices))
	copy(devices, dp.devices)
	dp.mu.Unlock()

	if err := dp.app.cfg.SaveKnownDevices(devices); err != nil {
		dp.app.logsPanel.Log("[WARN]Failed to save known devices: " + err.Error())
	}
}

// clearDisconnected removes all disconnected devices from the known list.
func (dp *DevicePanel) clearDisconnected() {
	dp.mu.Lock()
	var keep []adb.Device
	for _, d := range dp.devices {
		if dp.connectedSet[d.Serial] {
			keep = append(keep, d)
		} else {
			delete(dp.checkedSerials, d.Serial)
			if dp.lastSelected == d.Serial {
				dp.lastSelected = ""
			}
		}
	}
	removed := len(dp.devices) - len(keep)
	dp.devices = keep
	dp.mu.Unlock()

	if removed > 0 {
		dp.app.logsPanel.Log(fmt.Sprintf("[OK]Cleared %d disconnected device(s)", removed))
		dp.saveKnownDevices()
		dp.updateList()
	}
}
