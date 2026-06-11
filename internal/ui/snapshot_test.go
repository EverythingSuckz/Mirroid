package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// snapshot tests render real UI states to PNG for visual review. they skip
// unless MIRROID_SNAPSHOT_DIR is set so `go test ./...` stays assertion-only.

func snapshotDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("MIRROID_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("MIRROID_SNAPSHOT_DIR not set")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create snapshot dir: %v", err)
	}
	return dir
}

func saveSnapshot(t *testing.T, c fyne.Canvas, name string) {
	t.Helper()
	path := filepath.Join(snapshotDir(t), name+".png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, c.Capture()); err != nil {
		t.Fatalf("encode %s: %v", path, err)
	}
	t.Logf("wrote %s", path)
}

func newSnapshotApp(t *testing.T, variant fyne.ThemeVariant) (*App, fyne.Window) {
	t.Helper()
	a := test.NewApp()
	t.Cleanup(a.Quit)
	a.Settings().SetTheme(&variantTheme{variant: variant})

	w := a.NewWindow("Snapshot")
	w.SetContent(container.NewStack(widget.NewLabel("")))
	w.Resize(fyne.NewSize(900, 600))

	app := &App{fyneApp: a, window: w}
	app.notificationCenter = newNotificationCenter()
	return app, w
}

func reconnectFailedNotification(when time.Time) notification {
	return notification{
		id:    1,
		title: "Reconnect failed",
		message: "Couldn't reach Xiaomi Redmi Note 8 at 192.168.2.48:37997. " +
			"The device may be offline, or its wireless debugging port may have changed.",
		variant: ToastError,
		when:    when,
	}
}

func TestSnapshotNotificationDetail(t *testing.T) {
	snapshotDir(t)
	app, w := newSnapshotApp(t, theme.VariantDark)

	app.showNotificationDetailPopover(reconnectFailedNotification(time.Now().Add(-3 * time.Minute)))
	saveSnapshot(t, w.Canvas(), "notification_detail_dark")
}

func TestSnapshotNotificationDetailLight(t *testing.T) {
	snapshotDir(t)
	app, w := newSnapshotApp(t, theme.VariantLight)

	app.showNotificationDetailPopover(reconnectFailedNotification(time.Now().Add(-3 * time.Minute)))
	saveSnapshot(t, w.Canvas(), "notification_detail_light")
}

func TestSnapshotNotificationDetailSelection(t *testing.T) {
	snapshotDir(t)
	app, w := newSnapshotApp(t, theme.VariantDark)

	app.showNotificationDetailPopover(reconnectFailedNotification(time.Now().Add(-3 * time.Minute)))
	// drag across the second body line; coordinates assume the 900x600 canvas
	// and this exact message (manual review harness, not an assertion).
	test.Drag(w.Canvas(), fyne.NewPos(300, 305), 200, 0)
	saveSnapshot(t, w.Canvas(), "notification_detail_selection_dark")
}

func TestSnapshotNotificationDetailLong(t *testing.T) {
	snapshotDir(t)
	app, w := newSnapshotApp(t, theme.VariantDark)

	app.showNotificationDetailPopover(notification{
		id:    1,
		title: "Mirror error · Redmi Note 8",
		message: strings.Repeat(
			"Capture/encoding error: java.lang.IllegalStateException: Surface was abandoned before frame delivery. ", 10),
		variant: ToastError,
		when:    time.Now().Add(-1 * time.Minute),
	})
	saveSnapshot(t, w.Canvas(), "notification_detail_long_dark")
}

func TestSnapshotNotificationPanel(t *testing.T) {
	snapshotDir(t)
	app, w := newSnapshotApp(t, theme.VariantDark)

	now := time.Now()
	app.notificationCenter.push(notification{
		title: "Device connected", message: "Xiaomi Redmi Note 8",
		variant: ToastSuccess, when: now.Add(-2 * time.Hour),
	})
	app.notificationCenter.push(notification{
		title: "Device disconnected", message: "Xiaomi Redmi Note 8",
		variant: ToastWarning, when: now.Add(-50 * time.Minute),
	})
	app.notificationCenter.push(reconnectFailedNotification(now.Add(-30 * time.Second)))

	anchor := widget.NewButtonWithIcon("", theme.InfoIcon(), nil)
	w.SetContent(container.NewBorder(
		container.NewBorder(nil, nil, nil, anchor),
		nil, nil, nil, widget.NewLabel(""),
	))
	app.showNotificationPopover(anchor)
	saveSnapshot(t, w.Canvas(), "notification_panel_dark")
}

func TestSnapshotToasts(t *testing.T) {
	snapshotDir(t)
	app, w := newSnapshotApp(t, theme.VariantDark)
	app.toastManager = newToastManager(w.Canvas())
	w.SetContent(container.NewStack(widget.NewLabel(""), app.toastManager.host))

	app.toastManager.Show("Device connected", "Xiaomi Redmi Note 8", ToastSuccess, nil)
	app.toastManager.Show("Reconnect failed · Some Device With A Really Long Name", "192.168.2.48:37997", ToastError, nil)
	app.toastManager.Show("Mirror error · Redmi Note 8",
		strings.Repeat("Capture/encoding error: java.lang.IllegalStateException: Surface was abandoned. ", 8),
		ToastError, nil)
	// snap past the slide-in animation so positions are deterministic.
	app.toastManager.reflow()
	saveSnapshot(t, w.Canvas(), "toasts_dark")
}

func TestSnapshotToastTapOpensDetail(t *testing.T) {
	snapshotDir(t)
	app, w := newSnapshotApp(t, theme.VariantDark)
	app.toastManager = newToastManager(w.Canvas())
	w.SetContent(container.NewStack(widget.NewLabel(""), app.toastManager.host))

	n := app.notificationCenter.push(reconnectFailedNotification(time.Now()))
	app.toastManager.Show(n.title, n.message, n.variant, func() {
		app.showNotificationDetailPopover(n)
	})
	app.toastManager.reflow()
	// hover then tap the toast body (right edge stack, first toast).
	test.MoveMouse(w.Canvas(), fyne.NewPos(650, 90))
	saveSnapshot(t, w.Canvas(), "toast_hover_dark")
	test.TapCanvas(w.Canvas(), fyne.NewPos(650, 90))
	saveSnapshot(t, w.Canvas(), "toast_tap_detail_dark")
}

func TestSnapshotNotificationPanelHover(t *testing.T) {
	snapshotDir(t)
	app, w := newSnapshotApp(t, theme.VariantDark)

	now := time.Now()
	app.notificationCenter.push(notification{
		title: "Device connected", message: "Xiaomi Redmi Note 8",
		variant: ToastSuccess, when: now.Add(-2 * time.Hour),
	})
	app.notificationCenter.push(reconnectFailedNotification(now.Add(-30 * time.Second)))

	anchor := widget.NewButtonWithIcon("", theme.InfoIcon(), nil)
	w.SetContent(container.NewBorder(
		container.NewBorder(nil, nil, nil, anchor),
		nil, nil, nil, widget.NewLabel(""),
	))
	app.showNotificationPopover(anchor)
	// hover the first row; coordinates assume the 900x600 canvas.
	test.MoveMouse(w.Canvas(), fyne.NewPos(720, 130))
	saveSnapshot(t, w.Canvas(), "notification_panel_hover_dark")
}

func TestSnapshotShortWindow(t *testing.T) {
	snapshotDir(t)
	app, w := newSnapshotApp(t, theme.VariantDark)
	w.Resize(fyne.NewSize(900, 240))

	now := time.Now()
	app.notificationCenter.push(notification{
		title: "Device connected", message: "Xiaomi Redmi Note 8",
		variant: ToastSuccess, when: now.Add(-2 * time.Hour),
	})
	app.notificationCenter.push(notification{
		title: "Device disconnected", message: "Xiaomi Redmi Note 8",
		variant: ToastWarning, when: now.Add(-50 * time.Minute),
	})
	app.notificationCenter.push(reconnectFailedNotification(now.Add(-30 * time.Second)))

	anchor := widget.NewButtonWithIcon("", theme.InfoIcon(), nil)
	w.SetContent(container.NewBorder(
		container.NewBorder(nil, nil, nil, anchor),
		nil, nil, nil, widget.NewLabel(""),
	))
	app.showNotificationPopover(anchor)
	saveSnapshot(t, w.Canvas(), "notification_panel_short_dark")
}

func TestSnapshotShortWindowDetail(t *testing.T) {
	snapshotDir(t)
	app, w := newSnapshotApp(t, theme.VariantDark)
	w.Resize(fyne.NewSize(900, 240))

	app.showNotificationDetailPopover(notification{
		id:    1,
		title: "Mirror error · Redmi Note 8",
		message: strings.Repeat(
			"Capture/encoding error: java.lang.IllegalStateException: Surface was abandoned. ", 6),
		variant: ToastError,
		when:    time.Now(),
	})
	saveSnapshot(t, w.Canvas(), "notification_detail_short_dark")
}

func TestSnapshotNotificationPanelEmpty(t *testing.T) {
	snapshotDir(t)
	app, w := newSnapshotApp(t, theme.VariantDark)

	anchor := widget.NewButtonWithIcon("", theme.InfoIcon(), nil)
	w.SetContent(container.NewBorder(
		container.NewBorder(nil, nil, nil, anchor),
		nil, nil, nil, widget.NewLabel(""),
	))
	app.showNotificationPopover(anchor)
	saveSnapshot(t, w.Canvas(), "notification_panel_empty_dark")
}
