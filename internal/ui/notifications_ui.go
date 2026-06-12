package ui

import (
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
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

func (c *NotificationCenter) push(n notification) notification {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextID++
	n.id = c.nextID
	c.history = append(c.history, n)
	if len(c.history) > maxNotificationHistory {
		c.history = c.history[len(c.history)-maxNotificationHistory:]
	}
	c.unread++
	return n
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
			break
		}
	}
	// keep the unread dot honest when dismissing without opening the tray
	if c.unread > len(c.history) {
		c.unread = len(c.history)
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
	notifDetailWidth   float32 = 380
	notifDetailMaxH    float32 = 400
	notifDetailPad     float32 = 18
)

func (a *App) showNotificationPopover(anchor fyne.CanvasObject) {
	a.notificationCenter.markRead()
	a.refreshBell()

	listBox := container.NewVBox()
	clearBtn := ttwidget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
	clearBtn.Importance = widget.LowImportance
	clearBtn.SetToolTip("Clear all")

	scroll := container.NewVScroll(listBox)

	titleLbl := widget.NewRichText(&widget.TextSegment{
		Text: "Notifications",
		Style: widget.RichTextStyle{
			SizeName:  theme.SizeNameSubHeadingText,
			TextStyle: fyne.TextStyle{Bold: true},
		},
	})
	headerRow := container.NewBorder(nil, nil, titleLbl, clearBtn)
	header := container.NewVBox(headerRow, widget.NewSeparator())

	var pop *popover
	size := fyne.NewSize(notifPopoverWidth, notifPopoverHeight)

	// size the popover to its content; the scroll takes over past the max.
	applySize := func() {
		maxPanelH := notifPopoverHeight
		// short windows get a smaller scrolling panel instead of one that
		// runs past the canvas
		if ch := a.window.Canvas().Size().Height - 2*notifPopoverGap; ch > 0 && ch < maxPanelH {
			maxPanelH = ch
		}
		// chrome = outer padding + header + border gap above the scroll
		chrome := header.MinSize().Height + theme.Padding() + 2*toastPadding
		scrollH := listBox.MinSize().Height
		if maxH := maxPanelH - chrome; scrollH > maxH {
			scrollH = maxH
		}
		scroll.SetMinSize(fyne.NewSize(notifPopoverWidth-2*toastPadding, scrollH))
		size = fyne.NewSize(notifPopoverWidth, scrollH+chrome)
		if pop != nil {
			pop.panelSize = size
			pop.Refresh()
		}
	}

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
				row := buildNotificationRow(n, func() {
					a.notificationCenter.removeByID(n.id)
					rebuild()
				})
				row.OnTap = func() {
					if pop != nil {
						pop.hideOverlay()
					}
					a.showNotificationDetailPopover(n)
				}
				rows = append(rows, row)
			}
		}
		listBox.Objects = rows
		listBox.Refresh()
		clearBtn.Hidden = len(history) == 0
		clearBtn.Refresh()
		applySize()
	}
	rebuild()

	body := container.NewBorder(header, nil, nil, nil, scroll)
	padded := container.New(&paddedLayout{pad: toastPadding}, body)

	pop = newPopover(padded, a.window.Canvas())

	clearBtn.OnTapped = func() {
		a.notificationCenter.clear()
		rebuild()
	}

	driver := fyne.CurrentApp().Driver()
	calcPos := func() fyne.Position {
		abs := driver.AbsolutePositionForObject(anchor)
		cs := a.window.Canvas().Size()
		x := abs.X + anchor.Size().Width - size.Width
		if maxX := cs.Width - size.Width - notifPopoverGap; x > maxX {
			x = maxX
		}
		if x < notifPopoverGap {
			x = notifPopoverGap
		}
		y := abs.Y + anchor.Size().Height + notifPopoverGap
		// no room below the anchor: flip above it, else pin to the bottom
		if y+size.Height+notifPopoverGap > cs.Height {
			if above := abs.Y - size.Height - notifPopoverGap; above >= notifPopoverGap {
				y = above
			} else if maxY := cs.Height - size.Height - notifPopoverGap; maxY >= notifPopoverGap {
				y = maxY
			} else {
				y = notifPopoverGap
			}
		}
		return fyne.NewPos(x, y)
	}
	pop.reposition = func(_ fyne.Size) fyne.Position { return calcPos() }

	cancel := a.addResizeListener(func(_ fyne.Size) { pop.Refresh() })
	pop.OnHide = cancel

	pop.ShowAt(calcPos(), size)
}

// showNotificationDetailPopover opens a centered popover with the full,
// selectable text. X closes; Dismiss also removes it from the tray history.
func (a *App) showNotificationDetailPopover(n notification) {
	var pop *popover

	dot := canvas.NewRectangle(toastAccentColor(n.variant))
	dot.CornerRadius = toastDotSize / 2
	dotBox := container.New(&fixedSizeLayout{width: toastDotSize, height: toastDotSize}, dot)

	title := widget.NewLabelWithStyle(n.title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText
	title.Wrapping = fyne.TextWrapWord
	title.Selectable = true

	timeText := canvas.NewText(formatRelativeTime(n.when), theme.Color(theme.ColorNamePlaceHolder))
	timeText.TextSize = theme.Size(theme.SizeNameCaptionText)

	closeX := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		if pop != nil {
			pop.hideOverlay()
		}
	})
	closeX.Importance = widget.LowImportance

	rightSide := container.NewHBox(container.NewCenter(timeText), closeX)
	header := container.NewBorder(nil, nil,
		container.NewCenter(dotBox),
		rightSide,
		title,
	)

	body := widget.NewLabel(n.message)
	body.Wrapping = fyne.TextWrapWord
	body.Selectable = true

	dismissBtn := widget.NewButton("Dismiss", func() {
		a.notificationCenter.removeByID(n.id)
		a.refreshBell()
		if pop != nil {
			pop.hideOverlay()
		}
	})
	dismissBtn.Importance = widget.HighImportance
	footer := container.NewHBox(layout.NewSpacer(), dismissBtn)

	topPart := container.NewVBox(header, widget.NewSeparator())
	content := container.NewBorder(topPart, footer, nil, nil, body)
	padded := container.New(&paddedLayout{pad: notifDetailPad}, content)

	// size to content: lay out at the fixed width so the wrapping label
	// reports its real height, then clamp + scroll past the max.
	maxH := notifDetailMaxH
	if ch := a.window.Canvas().Size().Height - 2*notifPopoverGap; ch > 0 && ch < maxH {
		maxH = ch
	}
	padded.Resize(fyne.NewSize(notifDetailWidth, padded.MinSize().Height))
	height := padded.MinSize().Height
	if height > maxH {
		height = maxH
		content = container.NewBorder(topPart, footer, nil, nil, container.NewVScroll(body))
		padded = container.New(&paddedLayout{pad: notifDetailPad}, content)
	}

	pop = newPopover(padded, a.window.Canvas())

	size := fyne.NewSize(notifDetailWidth, height)
	calcPos := func() fyne.Position {
		c := a.window.Canvas().Size()
		x := (c.Width - size.Width) / 2
		if x < notifPopoverGap {
			x = notifPopoverGap
		}
		y := (c.Height - size.Height) / 2
		if maxY := c.Height - size.Height - notifPopoverGap; y > maxY && maxY >= notifPopoverGap {
			y = maxY
		}
		if y < notifPopoverGap {
			y = notifPopoverGap
		}
		return fyne.NewPos(x, y)
	}
	pop.reposition = func(_ fyne.Size) fyne.Position { return calcPos() }

	cancel := a.addResizeListener(func(_ fyne.Size) { pop.Refresh() })
	pop.OnHide = cancel

	pop.ShowAt(calcPos(), size)
}

func buildNotificationRow(n notification, onDismiss func()) *hoverCard {
	dot := canvas.NewRectangle(toastAccentColor(n.variant))
	dot.CornerRadius = toastDotSize / 2
	dotBox := container.New(&fixedSizeLayout{width: toastDotSize, height: toastDotSize}, dot)

	title := widget.NewLabelWithStyle(n.title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.Truncation = fyne.TextTruncateEllipsis

	// canvas.Text gives us a real placeholder color in both themes -
	// widget.Label's LowImportance is too washed-out in light mode.
	timeText := canvas.NewText(formatRelativeTime(n.when), theme.Color(theme.ColorNamePlaceHolder))
	timeText.TextSize = theme.Size(theme.SizeNameCaptionText)

	closeBtn := ttwidget.NewButtonWithIcon("", theme.CancelIcon(), onDismiss)
	closeBtn.Importance = widget.LowImportance
	closeBtn.SetToolTip("Dismiss")

	// center slot so long titles truncate instead of pushing time/close off
	// the popover edge.
	right := container.NewHBox(container.NewCenter(timeText), closeBtn)
	header := container.NewBorder(nil, nil,
		container.NewCenter(dotBox),
		right,
		title,
	)

	// single line; the click-to-open detail view shows the full message.
	msg := widget.NewLabel(n.message)
	msg.Truncation = fyne.TextTruncateEllipsis

	return newHoverCard(container.NewPadded(container.NewVBox(header, indentToTitle(msg))))
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
