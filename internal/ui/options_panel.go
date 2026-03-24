package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/model"
)

// OptionsPanel manages all scrcpy option widgets.
type OptionsPanel struct {
	app *App

	bitrate  *widget.Entry
	maxSize  *widget.Entry
	maxFPS   *widget.Entry
	codec    *widget.Select
	videoSrc *widget.Select

	audioEnabled *widget.Check
	audioSource  *widget.Select

	fullscreen  *widget.Check
	borderless  *widget.Check
	alwaysOnTop *widget.Check
	windowTitle *widget.Entry

	rotation      *widget.Select
	turnScreenOff *widget.Check
	clipboardSync *widget.Check
	stayAwake     *widget.Check
	hidKeyboard   *widget.Check
	hidMouse      *widget.Check

	recordFile *widget.Entry
}

// NewOptionsPanel creates a new options panel.
func NewOptionsPanel(app *App) *OptionsPanel {
	return &OptionsPanel{app: app}
}

// Build creates the tabbed options UI.
func (op *OptionsPanel) Build() fyne.CanvasObject {
	defaults := op.app.options

	op.bitrate = widget.NewEntry()
	op.bitrate.SetText(strconv.Itoa(defaults.Bitrate))
	op.bitrate.SetPlaceHolder("Mbps")
	op.bitrate.Validator = intValidator(0, 200)

	op.maxSize = widget.NewEntry()
	op.maxSize.SetPlaceHolder("e.g. 1920")
	if defaults.MaxSize > 0 {
		op.maxSize.SetText(strconv.Itoa(defaults.MaxSize))
	}

	op.maxFPS = widget.NewEntry()
	op.maxFPS.SetPlaceHolder("e.g. 60")
	if defaults.MaxFPS > 0 {
		op.maxFPS.SetText(strconv.Itoa(defaults.MaxFPS))
	}

	op.codec = widget.NewSelect([]string{"h264", "h265", "av1"}, nil)
	op.codec.SetSelected(defaults.Codec)

	op.videoSrc = widget.NewSelect([]string{"display", "camera"}, nil)
	op.videoSrc.SetSelected(defaults.VideoSource)

	videoTab := container.NewVBox(
		container.NewGridWithColumns(3,
			labeledField("Bitrate", op.bitrate),
			labeledField("Max Size", op.maxSize),
			labeledField("Max FPS", op.maxFPS),
		),
		container.NewGridWithColumns(2,
			labeledField("Codec", op.codec),
			labeledField("Source", op.videoSrc),
		),
	)

	op.audioEnabled = widget.NewCheck("Enable Audio", nil)
	op.audioEnabled.SetChecked(defaults.AudioEnabled)

	op.audioSource = widget.NewSelect([]string{"output", "mic", "playback"}, nil)
	op.audioSource.SetSelected(defaults.AudioSource)

	audioTab := container.NewVBox(
		op.audioEnabled,
		labeledField("Source", op.audioSource),
	)

	op.fullscreen = widget.NewCheck("Fullscreen", nil)
	op.fullscreen.SetChecked(defaults.Fullscreen)

	op.borderless = widget.NewCheck("Borderless", nil)
	op.borderless.SetChecked(defaults.Borderless)

	op.alwaysOnTop = widget.NewCheck("Always on Top", nil)
	op.alwaysOnTop.SetChecked(defaults.AlwaysOnTop)

	op.windowTitle = widget.NewEntry()
	op.windowTitle.SetPlaceHolder("Custom title")
	op.windowTitle.SetText(defaults.WindowTitle)

	windowTab := container.NewVBox(
		container.NewGridWithColumns(3, op.fullscreen, op.borderless, op.alwaysOnTop),
		labeledField("Title", op.windowTitle),
	)

	op.rotation = widget.NewSelect([]string{"0", "1", "2", "3"}, nil)
	op.rotation.SetSelected(strconv.Itoa(defaults.Rotation))

	op.turnScreenOff = widget.NewCheck("Screen Off", nil)
	op.turnScreenOff.SetChecked(defaults.TurnScreenOff)

	op.clipboardSync = widget.NewCheck("Clipboard", nil)
	op.clipboardSync.SetChecked(defaults.ClipboardSync)

	op.stayAwake = widget.NewCheck("Stay Awake", nil)
	op.stayAwake.SetChecked(defaults.StayAwake)

	op.hidKeyboard = widget.NewCheck("HID Keyboard", nil)
	op.hidKeyboard.SetChecked(defaults.HIDKeyboard)

	op.hidMouse = widget.NewCheck("HID Mouse", nil)
	op.hidMouse.SetChecked(defaults.HIDMouse)

	controlsTab := container.NewVBox(
		container.NewGridWithColumns(3, op.turnScreenOff, op.clipboardSync, op.stayAwake),
		container.NewGridWithColumns(3, op.hidKeyboard, op.hidMouse, labeledField("Rotation", op.rotation)),
	)

	op.recordFile = widget.NewEntry()
	op.recordFile.SetPlaceHolder("Path to recording file")
	op.recordFile.SetText(defaults.RecordFile)

	recordTab := container.NewVBox(op.recordFile)

	tabs := container.NewAppTabs(
		container.NewTabItem("Video", videoTab),
		container.NewTabItem("Audio", audioTab),
		container.NewTabItem("Window", windowTab),
		container.NewTabItem("Controls", controlsTab),
		container.NewTabItem("Record", recordTab),
	)

	return tabs
}

// SyncToModel reads all widget values into the given ScrcpyOptions.
func (op *OptionsPanel) SyncToModel(opts *model.ScrcpyOptions) {
	opts.Bitrate = parseIntOr(op.bitrate.Text, 8)
	opts.MaxSize = parseIntOr(op.maxSize.Text, 0)
	opts.MaxFPS = parseIntOr(op.maxFPS.Text, 0)
	opts.Codec = op.codec.Selected
	opts.AudioEnabled = op.audioEnabled.Checked
	opts.AudioSource = op.audioSource.Selected
	opts.Fullscreen = op.fullscreen.Checked
	opts.Borderless = op.borderless.Checked
	opts.AlwaysOnTop = op.alwaysOnTop.Checked
	opts.Rotation = parseIntOr(op.rotation.Selected, 0)
	opts.TurnScreenOff = op.turnScreenOff.Checked
	opts.RecordFile = op.recordFile.Text
	opts.WindowTitle = op.windowTitle.Text
	opts.ClipboardSync = op.clipboardSync.Checked
	opts.StayAwake = op.stayAwake.Checked
	opts.HIDKeyboard = op.hidKeyboard.Checked
	opts.HIDMouse = op.hidMouse.Checked
	opts.VideoSource = op.videoSrc.Selected
}

// SyncFromModel sets all widgets from a ScrcpyOptions.
func (op *OptionsPanel) SyncFromModel(opts model.ScrcpyOptions) {
	op.bitrate.SetText(strconv.Itoa(opts.Bitrate))
	if opts.MaxSize > 0 {
		op.maxSize.SetText(strconv.Itoa(opts.MaxSize))
	} else {
		op.maxSize.SetText("")
	}
	if opts.MaxFPS > 0 {
		op.maxFPS.SetText(strconv.Itoa(opts.MaxFPS))
	} else {
		op.maxFPS.SetText("")
	}
	op.codec.SetSelected(opts.Codec)
	op.audioEnabled.SetChecked(opts.AudioEnabled)
	op.audioSource.SetSelected(opts.AudioSource)
	op.fullscreen.SetChecked(opts.Fullscreen)
	op.borderless.SetChecked(opts.Borderless)
	op.alwaysOnTop.SetChecked(opts.AlwaysOnTop)
	op.rotation.SetSelected(strconv.Itoa(opts.Rotation))
	op.turnScreenOff.SetChecked(opts.TurnScreenOff)
	op.recordFile.SetText(opts.RecordFile)
	op.windowTitle.SetText(opts.WindowTitle)
	op.clipboardSync.SetChecked(opts.ClipboardSync)
	op.stayAwake.SetChecked(opts.StayAwake)
	op.hidKeyboard.SetChecked(opts.HIDKeyboard)
	op.hidMouse.SetChecked(opts.HIDMouse)
	op.videoSrc.SetSelected(opts.VideoSource)
}

func labeledField(label string, obj fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(widget.NewLabel(label), obj)
}

func parseIntOr(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}

func intValidator(min, max int) fyne.StringValidator {
	return func(s string) error {
		if s == "" {
			return nil
		}
		v, err := strconv.Atoi(s)
		if err != nil {
			return err
		}
		if v < min || v > max {
			return strconv.ErrRange
		}
		return nil
	}
}
