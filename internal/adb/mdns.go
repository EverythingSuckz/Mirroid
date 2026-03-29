package adb

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
)

const (
	pairingService = "_adb-tls-pairing._tcp"
	connectService = "_adb-tls-connect._tcp"
	mdnsDomain     = "local."
)

// MdnsDevice represents a device discovered via mDNS.
type MdnsDevice struct {
	Name string
	Addr string // ip:port
	Port int
}

// mdnsAddr extracts an IP:port string from a zeroconf entry,
// preferring IPv4 but falling back to IPv6.
func mdnsAddr(entry *zeroconf.ServiceEntry) (string, bool) {
	if len(entry.AddrIPv4) > 0 {
		return fmt.Sprintf("%s:%d", entry.AddrIPv4[0].String(), entry.Port), true
	}
	if len(entry.AddrIPv6) > 0 {
		return fmt.Sprintf("[%s]:%d", entry.AddrIPv6[0].String(), entry.Port), true
	}
	return "", false
}

// DiscoverDevices browses for already-paired wireless debugging devices
// via the _adb-tls-connect._tcp mDNS service.
func DiscoverDevices(ctx context.Context) ([]MdnsDevice, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("mdns: resolver init: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	var devices []MdnsDevice

	done := make(chan struct{})
	go func() {
		defer close(done)
		for entry := range entries {
			addr, ok := mdnsAddr(entry)
			if !ok {
				continue
			}
			devices = append(devices, MdnsDevice{
				Name: entry.Instance,
				Addr: addr,
				Port: entry.Port,
			})
		}
	}()

	scanCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	if err := resolver.Browse(scanCtx, connectService, mdnsDomain, entries); err != nil {
		close(entries) // unblock goroutine so it doesn't leak
		return nil, fmt.Errorf("mdns: browse: %w", err)
	}

	<-scanCtx.Done()
	<-done
	return devices, nil
}

// WatchDevices continuously watches for mDNS devices and sends them on a channel.
// it re-scans every interval until ctx is cancelled.
func WatchDevices(ctx context.Context, interval time.Duration, onFound func([]MdnsDevice)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			devices, err := DiscoverDevices(ctx)
			if err != nil {
				log.Printf("mdns watch: %v", err)
				continue
			}
			if len(devices) > 0 {
				onFound(devices)
			}
		}
	}
}

// DiscoverPairingDevices scans for devices that are in pairing mode
// (advertising _adb-tls-pairing._tcp). This is what happens when the user
// taps "Pair device with pairing code" or "Pair device with QR code" on Android.
func DiscoverPairingDevices(ctx context.Context, timeout time.Duration) ([]MdnsDevice, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("mdns: resolver init: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	var devices []MdnsDevice

	done := make(chan struct{})
	go func() {
		defer close(done)
		for entry := range entries {
			addr, ok := mdnsAddr(entry)
			if !ok {
				continue
			}
			devices = append(devices, MdnsDevice{
				Name: entry.Instance,
				Addr: addr,
				Port: entry.Port,
			})
		}
	}()

	scanCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := resolver.Browse(scanCtx, pairingService, mdnsDomain, entries); err != nil {
		close(entries) // unblock goroutine so it doesn't leak
		return nil, fmt.Errorf("mdns: browse pairing: %w", err)
	}

	<-scanCtx.Done()
	<-done
	return devices, nil
}

// WaitForPairingDevice blocks until a device in pairing mode is discovered,
// or until the context expires. Returns the first device found.
func WaitForPairingDevice(ctx context.Context, onStatus func(string)) (*MdnsDevice, error) {
	onStatus("Scanning for devices in pairing mode...")

	// poll for pairing devices every second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			devices, err := DiscoverPairingDevices(ctx, 1*time.Second)
			if err != nil {
				continue
			}
			if len(devices) > 0 {
				return &devices[0], nil
			}
		}
	}
}

// WaitForNamedPairingDevice browses for a pairing device whose mDNS instance
// name starts with the given serviceName. This is used for QR pairing: the phone
// echoes back the service name from the QR code as its mDNS instance name.
func WaitForNamedPairingDevice(ctx context.Context, serviceName string, onStatus func(string)) (*MdnsDevice, error) {
	onStatus("Waiting for phone to scan QR code...")

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			devices, err := DiscoverPairingDevices(ctx, 2*time.Second)
			if err != nil {
				continue
			}
			for _, d := range devices {
				if strings.HasPrefix(d.Name, serviceName) {
					return &d, nil
				}
			}
		}
	}
}
