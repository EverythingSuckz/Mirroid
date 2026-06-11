package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/model"
)

const (
	addNewItem    = "+ New Preset"
	untitledLabel = "<Untitled>"
)

// PresetsPanel manages save/load of option presets.
type PresetsPanel struct {
	app          *App
	presetSelect *compactSelect
	saveBtn      *compactButton
	discardBtn   *compactButton
	deleteBtn    *compactButton
	presets      map[string]model.ScrcpyOptions
	devicePresets map[string]string          // serial -> preset name
	snapshot     model.ScrcpyOptions         // clean state to restore on discard
}

func NewPresetsPanel(app *App) *PresetsPanel {
	return &PresetsPanel{
		app:           app,
		presets:       make(map[string]model.ScrcpyOptions),
		devicePresets: make(map[string]string),
	}
}

func (pp *PresetsPanel) Build() fyne.CanvasObject {
	presets, err := pp.app.cfg.LoadPresets()
	if err != nil {
		pp.app.logsPanel.Log("[WARN]" + err.Error())
	} else {
		pp.presets = presets
	}

	dp, err := pp.app.cfg.LoadDevicePresets()
	if err != nil {
		pp.app.logsPanel.Log("[WARN]" + err.Error())
	} else {
		pp.devicePresets = dp
	}

	pp.snapshot = pp.app.options

	pp.presetSelect = newCompactSelect(pp.presetNames(), pp.onSelect)
	pp.presetSelect.PlaceHolder = "Presets..."

	pp.saveBtn = newCompactButton(theme.DocumentSaveIcon(), pp.onSave)
	pp.saveBtn.SetToolTip("Save current settings as preset")

	pp.discardBtn = newCompactButton(theme.CancelIcon(), pp.onDiscard)
	pp.discardBtn.SetToolTip("Discard Changes")
	pp.discardBtn.Disable()

	pp.deleteBtn = newCompactButton(theme.DeleteIcon(), pp.onDelete)
	pp.deleteBtn.SetToolTip("Delete selected preset")
	pp.deleteBtn.Disable()

	// Wire up dirty tracking from options panel.
	pp.app.optionsPanel.OnChanged = pp.markDirty

	// Wrap the dropdown in a Center container so HBox doesn't stretch it
	// to match the row's tallest child - the dropdown stays at its natural
	// (shorter) height while the buttons set the row height. Gaps add
	// breathing room: between the bordered dropdown and the borderless
	// buttons, and between the trash and the row's right edge.
	leftGap := canvas.NewImageFromResource(nil)
	leftGap.SetMinSize(fyne.NewSize(4, 1))
	rightGap := canvas.NewImageFromResource(nil)
	rightGap.SetMinSize(fyne.NewSize(4, 1))
	return container.NewHBox(container.NewCenter(pp.presetSelect), leftGap, pp.saveBtn, pp.discardBtn, pp.deleteBtn, rightGap)
}

func (pp *PresetsPanel) presetNames() []string {
	names := make([]string, 0, len(pp.presets)+1)
	names = append(names, addNewItem)
	for name := range pp.presets {
		names = append(names, name)
	}
	return names
}

func (pp *PresetsPanel) markDirty() {
	pp.saveBtn.Enable()
	pp.discardBtn.Enable()
}

func (pp *PresetsPanel) markClean() {
	pp.saveBtn.Disable()
	pp.discardBtn.Disable()
}

// markUnsaved enables save (to let the user name and store the current settings)
// but keeps discard disabled since there is no prior preset to revert to.
func (pp *PresetsPanel) markUnsaved() {
	pp.saveBtn.Enable()
	pp.discardBtn.Disable()
}

func (pp *PresetsPanel) updateDeleteState() {
	sel := pp.presetSelect.Selected
	if len(pp.presets) == 0 || sel == "" || sel == addNewItem || sel == untitledLabel {
		pp.deleteBtn.Disable()
	} else {
		pp.deleteBtn.Enable()
	}
}

// saveDeviceAssociation persists the current device -> preset mapping.
func (pp *PresetsPanel) saveDeviceAssociation(presetName string) {
	if pp.app.devicePanel == nil {
		return
	}
	serial := pp.app.devicePanel.SelectedDevice()
	if serial == "" {
		return
	}
	if presetName == "" {
		delete(pp.devicePresets, serial)
	} else {
		pp.devicePresets[serial] = presetName
	}
	_ = pp.app.cfg.SaveDevicePresets(pp.devicePresets)
}

// LoadPresetForDevice loads the last-used preset for a device, or resets to defaults.
func (pp *PresetsPanel) LoadPresetForDevice(serial string) {
	if pp.presetSelect == nil {
		return
	}

	presetName, ok := pp.devicePresets[serial]
	if ok && presetName != "" {
		// Check the preset still exists.
		if _, exists := pp.presets[presetName]; exists {
			pp.presetSelect.SetSelected(presetName) // triggers onSelect
			return
		}
		// Preset was deleted - clean up stale mapping.
		delete(pp.devicePresets, serial)
		_ = pp.app.cfg.SaveDevicePresets(pp.devicePresets)
	}

	// No saved preset for this device - reset to defaults.
	defaults := model.DefaultOptions()
	pp.app.optionsPanel.SyncFromModel(defaults)
	pp.app.options = defaults
	pp.snapshot = defaults
	pp.presetSelect.ClearSelected()
	pp.markUnsaved()
	pp.updateDeleteState()
}

// onSelect auto-loads a preset when selected from the dropdown.
func (pp *PresetsPanel) onSelect(name string) {
	if name == "" {
		return
	}

	// "+ New Preset" resets everything to defaults and parks the dropdown
	// on a placeholder-ish label so the user sees they're in a fresh
	// (un-saved) state instead of falling back to the "Presets..." prompt.
	if name == addNewItem {
		defaults := model.DefaultOptions()
		pp.app.optionsPanel.SyncFromModel(defaults)
		pp.app.options = defaults
		pp.snapshot = defaults
		// assign Selected directly so OnChanged doesn't re-fire (no real
		// preset is being chosen, just the visual placeholder).
		pp.presetSelect.Selected = untitledLabel
		pp.presetSelect.Refresh()
		pp.markUnsaved()
		pp.updateDeleteState()
		pp.saveDeviceAssociation("")
		return
	}

	opts, ok := pp.presets[name]
	if !ok {
		return
	}

	// Camera ids are device-specific. If the preset's id isn't in the
	// current device's list, drop it silently - scrcpy will pick its
	// default rather than failing at launch with "no such camera".
	if !pp.app.optionsPanel.IsCameraIDValid(opts.CameraID) {
		pp.app.logsPanel.Log("[WARN]preset's camera id '" + opts.CameraID + "' isn't on this device - using default")
		opts.CameraID = ""
	}

	pp.app.optionsPanel.SyncFromModel(opts)
	pp.app.options = opts
	pp.snapshot = opts
	pp.markClean()
	pp.updateDeleteState()
	pp.saveDeviceAssociation(name)
	pp.app.logsPanel.Log("[OK]Loaded: " + name)
}

// onSave opens a custom modal to name and save the current settings.
// Built manually instead of via dialog.NewCustomConfirm so we control the
// title size, the spacing between sections, and the action-button row.
func (pp *PresetsPanel) onSave() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Preset name")
	if sel := pp.presetSelect.Selected; sel != "" && sel != addNewItem && sel != untitledLabel {
		nameEntry.SetText(sel)
	}

	c := pp.app.window.Canvas()
	var modal *widget.PopUp

	title := newCompactText("Save Preset", theme.ColorNameForeground)
	title.FontSize = 18
	title.Bold = true

	nameLabel := newCompactText("Name", theme.ColorNameForeground)
	nameLabel.Bold = true

	submit := func() {
		name := strings.TrimSpace(nameEntry.Text)
		if name == "" {
			pp.app.logsPanel.Log("[WARN]Preset name cannot be empty")
			return
		}
		if name == addNewItem || name == untitledLabel {
			pp.app.logsPanel.Log("[WARN]Reserved name, choose another")
			return
		}

		var opts model.ScrcpyOptions
		pp.app.optionsPanel.SyncToModel(&opts)

		pp.presets[name] = opts
		if err := pp.app.cfg.SavePresets(pp.presets); err != nil {
			pp.app.logsPanel.Log("[ERROR]" + err.Error())
			return
		}

		pp.snapshot = opts
		pp.presetSelect.Options = pp.presetNames()
		pp.presetSelect.SetSelected(name)
		pp.presetSelect.Refresh()
		pp.markClean()
		pp.updateDeleteState()
		pp.saveDeviceAssociation(name)
		pp.app.logsPanel.Log("[OK]Saved: " + name)
		modal.Hide()
	}

	cancelBtn := widget.NewButton("Cancel", func() { modal.Hide() })
	saveBtn := widget.NewButton("Save", submit)
	saveBtn.Importance = widget.HighImportance
	nameEntry.OnSubmitted = func(string) { submit() }

	// Compact text-only buttons (no icons) for a smaller footprint than
	// the dialog-package confirm/cancel buttons.
	btnGap := canvas.NewImageFromResource(nil)
	btnGap.SetMinSize(fyne.NewSize(8, 1))
	buttons := container.New(layout.NewHBoxLayout(),
		layout.NewSpacer(), cancelBtn, btnGap, saveBtn,
	)

	// Tight spacers between sections - VBox's default theme.Padding gap
	// is too much, especially under "Save Preset".
	titleGap := canvas.NewImageFromResource(nil)
	titleGap.SetMinSize(fyne.NewSize(1, 4))
	bodyGap := canvas.NewImageFromResource(nil)
	bodyGap.SetMinSize(fyne.NewSize(1, 8))

	body := container.NewPadded(container.NewVBox(
		title,
		titleGap,
		nameLabel,
		nameEntry,
		bodyGap,
		buttons,
	))

	modal = widget.NewModalPopUp(body, c)
	modal.Resize(fyne.NewSize(380, 0))
	modal.Show()
	c.Focus(nameEntry)
}

// onDiscard restores options to the last loaded/saved state.
func (pp *PresetsPanel) onDiscard() {
	pp.app.optionsPanel.SyncFromModel(pp.snapshot)
	pp.app.options = pp.snapshot
	pp.markClean()
}

func (pp *PresetsPanel) onDelete() {
	name := pp.presetSelect.Selected
	if name == "" || name == addNewItem {
		return
	}

	delete(pp.presets, name)
	if err := pp.app.cfg.SavePresets(pp.presets); err != nil {
		pp.app.logsPanel.Log("[WARN]failed to save presets: " + err.Error())
	}

	// Remove any device associations pointing to the deleted preset.
	for serial, pName := range pp.devicePresets {
		if pName == name {
			delete(pp.devicePresets, serial)
		}
	}
	_ = pp.app.cfg.SaveDevicePresets(pp.devicePresets)

	pp.presetSelect.Options = pp.presetNames()
	pp.presetSelect.ClearSelected()
	pp.presetSelect.Refresh()

	// Reset to defaults since the active preset was deleted.
	pp.snapshot = model.DefaultOptions()
	pp.app.optionsPanel.SyncFromModel(pp.snapshot)
	pp.app.options = pp.snapshot
	pp.markUnsaved()
	pp.updateDeleteState()
	pp.app.logsPanel.Log("[OK]Deleted: " + name)
}
