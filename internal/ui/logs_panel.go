package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// LogsPanel collects log lines and can display them in a live-updating window
// with white monospace text.
type LogsPanel struct {
	app      *App
	logLines []string
	maxLines int
	mu       sync.Mutex

	// live log window references
	logWin     fyne.Window
	logContent *widget.RichText
}

// NewLogsPanel creates a new logs panel.
func NewLogsPanel() *LogsPanel {
	return &LogsPanel{
		maxLines: 2000,
	}
}

// SetApp sets the app reference.
func (lp *LogsPanel) SetApp(a *App) {
	lp.app = a
}

// Log appends a timestamped line and updates the live log window if open.
func (lp *LogsPanel) Log(text string) {
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s", ts, text)

	lp.mu.Lock()
	lp.logLines = append(lp.logLines, line)
	if len(lp.logLines) > lp.maxLines {
		// sorry old logs, it's time to go. survival of the newest
		lp.logLines = lp.logLines[len(lp.logLines)-lp.maxLines:]
	}
	lp.mu.Unlock()

	lp.refreshLogWindow()
}

// GetContent returns all log lines joined by newlines.
func (lp *LogsPanel) GetContent() string {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	return strings.Join(lp.logLines, "\n")
}

// Clear empties all log lines.
func (lp *LogsPanel) Clear() {
	lp.mu.Lock()
	lp.logLines = nil
	lp.mu.Unlock()
	lp.refreshLogWindow()
}

// ShowWindow opens (or focuses) the logs window with white monospace text.
func (lp *LogsPanel) ShowWindow() {
	if lp.logWin != nil {
		lp.logWin.RequestFocus()
		return
	}

	lp.logWin = lp.app.fyneApp.NewWindow("Logs")
	lp.logWin.Resize(fyne.NewSize(700, 450))

	lp.logContent = widget.NewRichText()
	lp.refreshLogContent()

	scrollContainer := container.NewVScroll(lp.logContent)

	copyBtn := widget.NewButtonWithIcon("Copy All", theme.ContentCopyIcon(), func() {
		lp.logWin.Clipboard().SetContent(lp.GetContent())
	})

	clearBtn := widget.NewButtonWithIcon("Clear", theme.DeleteIcon(), func() {
		lp.Clear()
	})

	toolbar := container.NewHBox(copyBtn, clearBtn)

	lp.logWin.SetContent(container.NewBorder(toolbar, nil, nil, nil, scrollContainer))
	lp.logWin.SetOnClosed(func() {
		lp.logWin = nil
		lp.logContent = nil
	})
	lp.logWin.Show()
}

// refreshLogWindow updates the live log window if it's open.
func (lp *LogsPanel) refreshLogWindow() {
	if lp.logContent == nil {
		return
	}
	fyne.Do(func() {
		lp.refreshLogContent()
	})
}

// refreshLogContent rebuilds the RichText segments with white monospace style.
func (lp *LogsPanel) refreshLogContent() {
	if lp.logContent == nil {
		return
	}

	lp.mu.Lock()
	lines := make([]string, len(lp.logLines))
	copy(lines, lp.logLines)
	lp.mu.Unlock()

	if len(lines) == 0 {
		lp.logContent.Segments = []widget.RichTextSegment{
			&widget.TextSegment{
				Text: "No logs yet...",
				Style: widget.RichTextStyle{
					ColorName: theme.ColorNameDisabled,
					TextStyle: fyne.TextStyle{Monospace: true},
				},
			},
		}
	} else {
		text := strings.Join(lines, "\n")
		lp.logContent.Segments = []widget.RichTextSegment{
			&widget.TextSegment{
				Text: text,
				Style: widget.RichTextStyle{
					ColorName: "",
					Inline:    false,
					TextStyle: fyne.TextStyle{Monospace: true},
				},
			},
		}
	}
	lp.logContent.Refresh()
}
