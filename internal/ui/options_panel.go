package ui

import (
	"image/color"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	ttwidget "github.com/dweymouth/fyne-tooltip/widget"

	"mirroid/internal/icons"
	"mirroid/internal/model"
	"mirroid/internal/scrcpy"
)

const cameraDefaultLabel = "Default"

// rotationLabels maps the model's 0-3 quarter-turn index to a human-readable
// degree string shown in the dropdown. Index = ScrcpyOptions.Rotation.
var rotationLabels = []string{"0°", "90°", "180°", "270°"}

func rotationLabelFor(r int) string {
	if r < 0 || r >= len(rotationLabels) {
		return rotationLabels[0]
	}
	return rotationLabels[r]
}

func rotationIndexFor(label string) int {
	for i, l := range rotationLabels {
		if l == label {
			return i
		}
	}
	return 0
}

// OptionsPanel manages all scrcpy option widgets.
type OptionsPanel struct {
	app *App

	bitrate   *widget.Entry
	maxSize   *widget.Entry
	maxFPS    *widget.Entry
	codec     *widget.Select
	videoSrc  *widget.Select
	cameraRow *fyne.Container

	cameraSelect      *widget.Select
	cameraRefreshBtn  *ttwidget.Button
	cameraRefreshSpin *rotatingIcon
	cameraMu          sync.Mutex
	cameraLabelToID   map[string]string
	cameraCache       map[string][]scrcpy.CameraInfo // per-serial cache, persisted to cameras.json

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

	OnChanged func() // called when any option widget changes
	syncing   bool   // suppresses OnChanged during SyncFromModel
}

func NewOptionsPanel(app *App) *OptionsPanel {
	return &OptionsPanel{app: app}
}

func (op *OptionsPanel) Build() fyne.CanvasObject {
	op.syncing = true
	defer func() { op.syncing = false }()

	defaults := op.app.options

	op.bitrate = widget.NewEntry()
	op.bitrate.SetText(strconv.Itoa(defaults.Bitrate))
	op.bitrate.SetPlaceHolder("Mbps")
	op.bitrate.Validator = intValidator(0, 200)
	op.bitrate.OnChanged = op.notifyChanged

	op.maxSize = widget.NewEntry()
	op.maxSize.SetPlaceHolder("e.g. 1920")
	if defaults.MaxSize > 0 {
		op.maxSize.SetText(strconv.Itoa(defaults.MaxSize))
	}
	op.maxSize.OnChanged = op.notifyChanged

	op.maxFPS = widget.NewEntry()
	op.maxFPS.SetPlaceHolder("e.g. 60")
	if defaults.MaxFPS > 0 {
		op.maxFPS.SetText(strconv.Itoa(defaults.MaxFPS))
	}
	op.maxFPS.OnChanged = op.notifyChanged

	op.codec = widget.NewSelect(model.Codecs, op.notifyChanged)
	op.codec.SetSelected(defaults.Codec)

	op.cameraLabelToID = map[string]string{}
	if op.app != nil && op.app.cfg != nil {
		if cache, err := op.app.cfg.LoadCameraCache(); err != nil {
			slog.Warn("camera cache load failed", "error", err)
			op.cameraCache = make(map[string][]scrcpy.CameraInfo)
		} else {
			op.cameraCache = cache
		}
	} else {
		op.cameraCache = make(map[string][]scrcpy.CameraInfo)
	}
	op.cameraSelect = widget.NewSelect([]string{cameraDefaultLabel}, op.notifyChanged)
	op.cameraSelect.SetSelected(cameraDefaultLabel)

	op.cameraRefreshBtn = ttwidget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		op.refreshCameraList()
	})
	op.cameraRefreshBtn.Importance = widget.LowImportance
	op.cameraRefreshBtn.SetToolTip("Refresh camera list")

	op.cameraRefreshSpin = newRotatingIcon(theme.ViewRefreshIcon())
	op.cameraRefreshSpin.Hide()
	spacer := canvas.NewRectangle(color.Transparent)
	spacer.SetMinSize(op.cameraRefreshBtn.MinSize())
	spinSlot := container.NewStack(spacer, container.NewCenter(op.cameraRefreshSpin))
	cameraRefreshSlot := container.NewStack(op.cameraRefreshBtn, spinSlot)

	cameraField := container.NewBorder(nil, nil, nil, cameraRefreshSlot, op.cameraSelect)
	cameraBox := container.NewVBox(widget.NewLabel("Camera"), cameraField)
	// 2-column grid so the dropdown takes half the row width, matching
	// the Codec / Source row above it.
	op.cameraRow = container.NewGridWithColumns(2, cameraBox)

	op.videoSrc = widget.NewSelect(model.VideoSources, func(_ string) {
		op.refreshCameraVisibility()
		op.notify()
	})
	op.videoSrc.SetSelected(defaults.VideoSource)

	op.rotation = widget.NewSelect(rotationLabels, op.notifyChanged)
	op.rotation.SetSelected(rotationLabelFor(defaults.Rotation))

	videoTab := container.NewVBox(
		container.NewGridWithColumns(3,
			labeledField("Bitrate", op.bitrate),
			labeledField("Max Size", op.maxSize),
			labeledField("Max FPS", op.maxFPS),
		),
		container.NewGridWithColumns(3,
			labeledField("Codec", op.codec),
			labeledField("Source", op.videoSrc),
			labeledField("Rotation", op.rotation),
		),
		op.cameraRow,
	)
	op.refreshCameraVisibility()

	op.audioEnabled = widget.NewCheck("Enable Audio", op.notifyChangedBool)
	op.audioEnabled.SetChecked(defaults.AudioEnabled)

	op.audioSource = widget.NewSelect(model.AudioSources, op.notifyChanged)
	op.audioSource.SetSelected(defaults.AudioSource)

	audioTab := container.NewVBox(
		op.audioEnabled,
		labeledField("Source", op.audioSource),
	)

	op.fullscreen = widget.NewCheck("Fullscreen", op.notifyChangedBool)
	op.fullscreen.SetChecked(defaults.Fullscreen)

	op.borderless = widget.NewCheck("Borderless", op.notifyChangedBool)
	op.borderless.SetChecked(defaults.Borderless)

	op.alwaysOnTop = widget.NewCheck("Always on Top", op.notifyChangedBool)
	op.alwaysOnTop.SetChecked(defaults.AlwaysOnTop)

	op.windowTitle = widget.NewEntry()
	op.windowTitle.SetPlaceHolder("Custom title")
	op.windowTitle.SetText(defaults.WindowTitle)
	op.windowTitle.OnChanged = op.notifyChanged

	windowTab := container.NewVBox(
		container.NewGridWithColumns(3, op.fullscreen, op.borderless, op.alwaysOnTop),
		labeledField("Title", op.windowTitle),
	)

	op.turnScreenOff = widget.NewCheck("Screen Off", op.notifyChangedBool)
	op.turnScreenOff.SetChecked(defaults.TurnScreenOff)

	op.clipboardSync = widget.NewCheck("Clipboard", op.notifyChangedBool)
	op.clipboardSync.SetChecked(defaults.ClipboardSync)

	op.stayAwake = widget.NewCheck("Stay Awake", op.notifyChangedBool)
	op.stayAwake.SetChecked(defaults.StayAwake)

	op.hidKeyboard = widget.NewCheck("HID Keyboard", op.notifyChangedBool)
	op.hidKeyboard.SetChecked(defaults.HIDKeyboard)

	op.hidMouse = widget.NewCheck("HID Mouse", op.notifyChangedBool)
	op.hidMouse.SetChecked(defaults.HIDMouse)

	controlsTab := container.NewVBox(
		container.NewGridWithColumns(3, op.turnScreenOff, op.clipboardSync, op.stayAwake),
		container.NewGridWithColumns(3, op.hidKeyboard, op.hidMouse, widget.NewLabel("")),
	)

	recordTab := buildRecordEmptyState()

	// Each tab's content scrolls independently; the tab strip and the
	// section header (built one level up) stay pinned regardless of how
	// short the bottom panel gets.
	tabs := container.NewAppTabs(
		container.NewTabItem("Video", container.NewVScroll(videoTab)),
		container.NewTabItem("Audio", container.NewVScroll(audioTab)),
		container.NewTabItem("Window", container.NewVScroll(windowTab)),
		container.NewTabItem("Controls", container.NewVScroll(controlsTab)),
		container.NewTabItem("Record", container.NewVScroll(recordTab)),
	)

	return tabs
}

// buildRecordEmptyState renders the centered "coming soon" placeholder
// shown on the Record tab. Recording isn't wired up yet - the empty state
// is intentional and signals that to the user.
func buildRecordEmptyState() fyne.CanvasObject {
	const iconSize float32 = 64

	cone := canvas.NewImageFromResource(icons.NewThemedIcon(icons.TrafficConeIcon))
	cone.FillMode = canvas.ImageFillContain
	cone.SetMinSize(fyne.NewSize(iconSize, iconSize))

	heading := newCompactText("Recording is coming soon", theme.ColorNameForeground)
	heading.FontSize = 16
	heading.Bold = true

	subtitle := newCompactText("This area is under construction.", theme.ColorNamePlaceHolder)

	stack := container.NewVBox(
		container.NewCenter(cone),
		container.NewCenter(heading),
		container.NewCenter(subtitle),
	)
	return container.NewCenter(stack)
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
	opts.Rotation = rotationIndexFor(op.rotation.Selected)
	opts.TurnScreenOff = op.turnScreenOff.Checked
	opts.RecordFile = "" // record UI is not implemented yet
	opts.WindowTitle = op.windowTitle.Text
	opts.ClipboardSync = op.clipboardSync.Checked
	opts.StayAwake = op.stayAwake.Checked
	opts.HIDKeyboard = op.hidKeyboard.Checked
	opts.HIDMouse = op.hidMouse.Checked
	opts.VideoSource = op.videoSrc.Selected

	op.cameraMu.Lock()
	opts.CameraID = op.cameraLabelToID[op.cameraSelect.Selected]
	op.cameraMu.Unlock()
}

func (op *OptionsPanel) refreshCameraVisibility() {
	if op.cameraRow == nil {
		return
	}
	if op.videoSrc.Selected == model.VideoSourceCamera {
		op.cameraRow.Show()
	} else {
		op.cameraRow.Hide()
	}
}

// refreshCameraList runs `scrcpy --list-cameras` for the currently selected
// device and updates the dropdown options. The cached list is shown
// immediately so the dropdown isn't empty while scrcpy spins up; the
// async fetch then refreshes the cache on success or logs a [WARN] in
// the chat on failure (keeping any cached list visible).
func (op *OptionsPanel) refreshCameraList() {
	if op.app == nil || op.app.runner == nil {
		return
	}
	serial := op.app.devicePanel.SelectedDevice()
	if serial == "" {
		op.applyCameraOptions(nil)
		return
	}

	// instant populate from cache
	op.cameraMu.Lock()
	cached := op.cameraCache[serial]
	op.cameraMu.Unlock()
	if len(cached) > 0 {
		op.applyCameraOptions(cached)
	}

	op.cameraRefreshBtn.Hide()
	op.cameraRefreshSpin.Show()
	op.cameraRefreshSpin.Start()

	go func() {
		cams, err := op.app.runner.ListCameras(serial)
		if err != nil {
			slog.Warn("scrcpy --list-cameras failed", "serial", serial, "error", err)
			if op.app.logsPanel != nil {
				op.app.logsPanel.Log("[WARN]list cameras for " + serial + " failed: " + err.Error())
			}
		}
		fyne.Do(func() {
			op.cameraRefreshSpin.Stop()
			op.cameraRefreshSpin.Hide()
			op.cameraRefreshBtn.Show()
			if err != nil {
				// keep showing whatever was cached; don't blank the dropdown
				return
			}
			op.cameraMu.Lock()
			op.cameraCache[serial] = cams
			snapshot := make(map[string][]scrcpy.CameraInfo, len(op.cameraCache))
			for k, v := range op.cameraCache {
				snapshot[k] = v
			}
			op.cameraMu.Unlock()

			op.applyCameraOptions(cams)

			if op.app.cfg != nil {
				if saveErr := op.app.cfg.SaveCameraCache(snapshot); saveErr != nil {
					slog.Warn("camera cache save failed", "error", saveErr)
				}
			}
		})
	}()
}

// applyCameraOptions rebuilds the dropdown options from the detected
// camera list, preserving the current selection by camera id when possible.
func (op *OptionsPanel) applyCameraOptions(cams []scrcpy.CameraInfo) {
	prevID := ""
	op.cameraMu.Lock()
	if op.cameraSelect != nil {
		prevID = op.cameraLabelToID[op.cameraSelect.Selected]
	}
	op.cameraMu.Unlock()

	labels, mapping := buildCameraLabels(cams)

	op.cameraMu.Lock()
	op.cameraLabelToID = mapping
	op.cameraMu.Unlock()

	op.cameraSelect.Options = labels

	pick := cameraDefaultLabel
	for label, id := range mapping {
		if id == prevID && id != "" {
			pick = label
			break
		}
	}
	op.cameraSelect.SetSelected(pick)
	op.cameraSelect.Refresh()
}

// IsCameraIDValid reports whether the given camera id is part of the
// current device's loaded list (cache or fresh). Empty id is always valid
// (means "scrcpy default"); unknown serials short-circuit to true so
// presets applied before the list loads aren't punished.
func (op *OptionsPanel) IsCameraIDValid(id string) bool {
	if id == "" {
		return true
	}
	op.cameraMu.Lock()
	defer op.cameraMu.Unlock()
	if len(op.cameraLabelToID) == 0 {
		return true
	}
	for _, cid := range op.cameraLabelToID {
		if cid == id {
			return true
		}
	}
	return false
}

// buildCameraLabels returns the dropdown labels and label to id mapping.
// Labels double as map keys, so duplicates (two cameras with the same
// facing and size) get the camera id appended to stay selectable.
func buildCameraLabels(cams []scrcpy.CameraInfo) ([]string, map[string]string) {
	counts := map[string]int{}
	for _, c := range cams {
		counts[cameraLabel(c)]++
	}

	labels := []string{cameraDefaultLabel}
	mapping := map[string]string{cameraDefaultLabel: ""}
	for _, c := range cams {
		label := cameraLabel(c)
		if counts[label] > 1 {
			label += " (" + c.ID + ")"
		}
		labels = append(labels, label)
		mapping[label] = c.ID
	}
	return labels, mapping
}

func cameraLabel(c scrcpy.CameraInfo) string {
	parts := []string{}
	if c.Facing != "" {
		parts = append(parts, capitalize(c.Facing))
	}
	if c.Size != "" {
		parts = append(parts, c.Size)
	}
	label := strings.Join(parts, " · ")
	if label == "" {
		label = "Camera " + c.ID
	}
	return label
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// onDeviceChanged is called by DevicePanel when the selected device changes.
// We re-query the camera list so the dropdown reflects what the new device
// actually exposes.
func (op *OptionsPanel) onDeviceChanged(_ string) {
	if op.cameraSelect == nil {
		return
	}
	op.refreshCameraList()
}

// SyncFromModel sets all widgets from a ScrcpyOptions.
func (op *OptionsPanel) SyncFromModel(opts model.ScrcpyOptions) {
	op.syncing = true
	defer func() { op.syncing = false }()

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
	op.rotation.SetSelected(rotationLabelFor(opts.Rotation))
	op.turnScreenOff.SetChecked(opts.TurnScreenOff)
	op.windowTitle.SetText(opts.WindowTitle)
	op.clipboardSync.SetChecked(opts.ClipboardSync)
	op.stayAwake.SetChecked(opts.StayAwake)
	op.hidKeyboard.SetChecked(opts.HIDKeyboard)
	op.hidMouse.SetChecked(opts.HIDMouse)
	op.videoSrc.SetSelected(opts.VideoSource)

	op.cameraMu.Lock()
	pick := cameraDefaultLabel
	for label, id := range op.cameraLabelToID {
		if id == opts.CameraID && id != "" {
			pick = label
			break
		}
	}
	op.cameraMu.Unlock()
	op.cameraSelect.SetSelected(pick)
	op.refreshCameraVisibility()
}

func (op *OptionsPanel) notify() {
	if !op.syncing && op.OnChanged != nil {
		op.OnChanged()
	}
}

func (op *OptionsPanel) notifyChanged(_ string)   { op.notify() }
func (op *OptionsPanel) notifyChangedBool(_ bool) { op.notify() }

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
