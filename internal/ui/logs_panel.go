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
	logContent *readOnlyEntry
}

type readOnlyEntry struct {
	widget.Entry
}

func newReadOnlyEntry() *readOnlyEntry {
	e := &readOnlyEntry{}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrap(fyne.TextTruncateClip)
	e.ExtendBaseWidget(e)
	return e
}

func (e *readOnlyEntry) TypedRune(_ rune) {}

func (e *readOnlyEntry) TypedKey(ev *fyne.KeyEvent) {
	switch ev.Name {
	case fyne.KeyBackspace, fyne.KeyDelete, fyne.KeyReturn, fyne.KeyEnter:
		return
	}
	e.Entry.TypedKey(ev)
}

func (e *readOnlyEntry) TypedShortcut(s fyne.Shortcut) {
	switch s.(type) {
	case *fyne.ShortcutPaste, *fyne.ShortcutCut:
		return
	}
	e.Entry.TypedShortcut(s)
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
	lp.logWin.Resize(fyne.NewSize(1000, 450))

	lp.logContent = newReadOnlyEntry()
	lp.logContent.TextStyle = fyne.TextStyle{Monospace: true}
	lp.logContent.SetPlaceHolder("No logs yet...")
	lp.refreshLogContent()

	copyBtn := widget.NewButtonWithIcon("Copy All", theme.ContentCopyIcon(), func() {
		lp.logWin.Clipboard().SetContent(lp.GetContent())
	})

	clearBtn := widget.NewButtonWithIcon("Clear", theme.DeleteIcon(), func() {
		lp.Clear()
	})

	toolbar := container.NewHBox(copyBtn, clearBtn)

	lp.logWin.SetContent(container.NewBorder(toolbar, nil, nil, nil, lp.logContent))
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

// refreshLogContent updates the Entry text and scrolls to the bottom.
func (lp *LogsPanel) refreshLogContent() {
	if lp.logContent == nil {
		return
	}

	lp.mu.Lock()
	lines := make([]string, len(lp.logLines))
	copy(lines, lp.logLines)
	lp.mu.Unlock()

	text := strings.Join(lines, "\n")
	lp.logContent.SetText(text)

	// Move cursor to end so the view auto-scrolls to show newest logs
	if len(lines) > 0 {
		lastRow := len(lines) - 1
		lastCol := len(lines[lastRow])
		lp.logContent.CursorRow = lastRow
		lp.logContent.CursorColumn = lastCol
	}
}
