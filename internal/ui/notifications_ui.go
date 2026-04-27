package ui

import (
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	ttwidget "github.com/dweymouth/fyne-tooltip/widget"
)

const maxNotificationHistory = 50

type notification struct {
	id      uint64
	title   string
	message string
	variant ToastVariant
	when    time.Time
}

type NotificationCenter struct {
	mu      sync.Mutex
	history []notification
	unread  int
	nextID  uint64
}

func newNotificationCenter() *NotificationCenter {
	return &NotificationCenter{}
}

func (c *NotificationCenter) push(n notification) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	n.id = c.nextID
	c.history = append(c.history, n)
	if len(c.history) > maxNotificationHistory {
		c.history = c.history[len(c.history)-maxNotificationHistory:]
	}
	c.unread++
}

func (c *NotificationCenter) snapshot() []notification {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]notification, len(c.history))
	copy(out, c.history)
	return out
}

func (c *NotificationCenter) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = nil
	c.unread = 0
}

func (c *NotificationCenter) removeByID(id uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, n := range c.history {
		if n.id == id {
			c.history = append(c.history[:i], c.history[i+1:]...)
			return
		}
	}
}

func (c *NotificationCenter) markRead() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unread = 0
}

func (c *NotificationCenter) hasUnread() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.unread > 0
}

const (
	notifPopoverWidth  float32 = 340
	notifPopoverHeight float32 = 340
	notifPopoverGap    float32 = 6
)

func (a *App) showNotificationPopover(anchor fyne.CanvasObject) {
	a.notificationCenter.markRead()
	a.refreshBell()

	listBox := container.NewVBox()
	clearBtn := ttwidget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
	clearBtn.Importance = widget.LowImportance
	clearBtn.SetToolTip("Clear all")

	var rebuild func()
	rebuild = func() {
		history := a.notificationCenter.snapshot()
		var rows []fyne.CanvasObject
		if len(history) == 0 {
			empty := widget.NewLabelWithStyle("No notifications yet", fyne.TextAlignCenter, fyne.TextStyle{Italic: true})
			rows = []fyne.CanvasObject{container.NewPadded(empty)}
		} else {
			for i := len(history) - 1; i >= 0; i-- {
				if i < len(history)-1 {
					rows = append(rows, widget.NewSeparator())
				}
				n := history[i]
				rows = append(rows, buildNotificationRow(n, func() {
					a.notificationCenter.removeByID(n.id)
					rebuild()
				}))
			}
		}
		listBox.Objects = rows
		listBox.Refresh()
		clearBtn.Hidden = len(history) == 0
		clearBtn.Refresh()
	}
	rebuild()

	scroll := container.NewVScroll(listBox)
	scroll.SetMinSize(fyne.NewSize(notifPopoverWidth-2*toastPadding, notifPopoverHeight-56))

	titleLbl := widget.NewRichText(&widget.TextSegment{
		Text: "Notifications",
		Style: widget.RichTextStyle{
			SizeName:  theme.SizeNameSubHeadingText,
			TextStyle: fyne.TextStyle{Bold: true},
		},
	})
	header := container.NewBorder(nil, nil, titleLbl, clearBtn)
	body := container.NewBorder(header, nil, nil, nil, scroll)
	padded := container.New(&paddedLayout{pad: toastPadding}, body)

	pop := newPopover(padded, a.window.Canvas())

	clearBtn.OnTapped = func() {
		a.notificationCenter.clear()
		rebuild()
	}

	driver := fyne.CurrentApp().Driver()
	absPos := driver.AbsolutePositionForObject(anchor)
	size := fyne.NewSize(notifPopoverWidth, notifPopoverHeight)
	x := absPos.X + anchor.Size().Width - size.Width
	if x < notifPopoverGap {
		x = notifPopoverGap
	}
	y := absPos.Y + anchor.Size().Height + notifPopoverGap
	pop.ShowAt(fyne.NewPos(x, y), size)
}

func buildNotificationRow(n notification, onDismiss func()) fyne.CanvasObject {
	dot := canvas.NewRectangle(toastAccentColor(n.variant))
	dot.CornerRadius = toastDotSize / 2
	dotBox := container.New(&fixedSizeLayout{width: toastDotSize, height: toastDotSize}, dot)

	title := widget.NewLabelWithStyle(n.title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// canvas.Text gives us a real placeholder color in both themes —
	// widget.Label's LowImportance is too washed-out in light mode.
	timeText := canvas.NewText(formatRelativeTime(n.when), theme.Color(theme.ColorNamePlaceHolder))
	timeText.TextSize = theme.Size(theme.SizeNameCaptionText)

	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), onDismiss)
	closeBtn.Importance = widget.LowImportance

	right := container.NewHBox(container.NewCenter(timeText), closeBtn)
	header := container.NewBorder(nil, nil,
		container.NewHBox(container.NewCenter(dotBox), title),
		right,
	)

	msg := widget.NewLabel(n.message)
	msg.Wrapping = fyne.TextWrapWord

	return container.NewPadded(container.NewVBox(header, msg))
}

// formatRelativeTime is recomputed each time the popover is opened. While the
// popover stays open the text is static, but every fresh open reflects the
// real elapsed time (so a notification fired 3 minutes ago will read "3m ago"
// when you reopen the panel).
func formatRelativeTime(t time.Time) string {
	diff := time.Since(t)
	switch {
	case diff < time.Minute:
		return "just now"
	case diff < time.Hour:
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	case diff < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	default:
		return t.Format("15:04 Jan 2")
	}
}
