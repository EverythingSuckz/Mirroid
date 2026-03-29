package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	ttwidget "github.com/dweymouth/fyne-tooltip/widget"

	"mirroid/internal/model"
)

const addNewItem = "+ New Preset"

// PresetsPanel manages save/load of option presets.
type PresetsPanel struct {
	app          *App
	presetSelect *widget.Select
	saveBtn      *ttwidget.Button
	discardBtn   *ttwidget.Button
	deleteBtn    *ttwidget.Button
	presets      map[string]model.ScrcpyOptions
	devicePresets map[string]string          // serial -> preset name
	snapshot     model.ScrcpyOptions         // clean state to restore on discard
}

// NewPresetsPanel creates a new presets panel.
func NewPresetsPanel(app *App) *PresetsPanel {
	return &PresetsPanel{
		app:           app,
		presets:       make(map[string]model.ScrcpyOptions),
		devicePresets: make(map[string]string),
	}
}

// Build creates the compact preset controls (dropdown + save/discard/delete icons).
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

	pp.presetSelect = widget.NewSelect(pp.presetNames(), pp.onSelect)
	pp.presetSelect.PlaceHolder = "Presets..."

	pp.saveBtn = ttwidget.NewButtonWithIcon("", theme.DocumentSaveIcon(), pp.onSave)
	pp.saveBtn.Importance = widget.LowImportance
	pp.saveBtn.SetToolTip("Save current settings as preset")

	pp.discardBtn = ttwidget.NewButtonWithIcon("", theme.CancelIcon(), pp.onDiscard)
	pp.discardBtn.Importance = widget.LowImportance
	pp.discardBtn.SetToolTip("Discard Changes")
	pp.discardBtn.Disable()

	pp.deleteBtn = ttwidget.NewButtonWithIcon("", theme.DeleteIcon(), pp.onDelete)
	pp.deleteBtn.Importance = widget.LowImportance
	pp.deleteBtn.SetToolTip("Delete selected preset")
	pp.deleteBtn.Disable()

	// Wire up dirty tracking from options panel.
	pp.app.optionsPanel.OnChanged = pp.markDirty

	return container.NewHBox(pp.presetSelect, pp.saveBtn, pp.discardBtn, pp.deleteBtn)
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
	if len(pp.presets) == 0 || pp.presetSelect.Selected == "" || pp.presetSelect.Selected == addNewItem {
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
		// Preset was deleted — clean up stale mapping.
		delete(pp.devicePresets, serial)
		_ = pp.app.cfg.SaveDevicePresets(pp.devicePresets)
	}

	// No saved preset for this device — reset to defaults.
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

	// "+ New Preset" resets everything to defaults.
	if name == addNewItem {
		defaults := model.DefaultOptions()
		pp.app.optionsPanel.SyncFromModel(defaults)
		pp.app.options = defaults
		pp.snapshot = defaults
		pp.presetSelect.ClearSelected()
		pp.markUnsaved()
		pp.updateDeleteState()
		pp.saveDeviceAssociation("")
		return
	}

	opts, ok := pp.presets[name]
	if !ok {
		return
	}

	pp.app.optionsPanel.SyncFromModel(opts)
	pp.app.options = opts
	pp.snapshot = opts
	pp.markClean()
	pp.updateDeleteState()
	pp.saveDeviceAssociation(name)
	pp.app.logsPanel.Log("[OK]Loaded: " + name)
}

// onSave opens a dialog to name and save the current settings as a preset.
func (pp *PresetsPanel) onSave() {
	nameEntry := widget.NewEntry()
	nameEntry.SetPlaceHolder("Preset name")

	// Pre-fill with the currently selected preset name for easy overwrite.
	if sel := pp.presetSelect.Selected; sel != "" && sel != addNewItem {
		nameEntry.SetText(sel)
	}

	dlg := dialog.NewForm(
		"Save Preset",
		"Save", "Cancel",
		[]*widget.FormItem{
			widget.NewFormItem("Name", nameEntry),
		},
		func(ok bool) {
			if !ok {
				return
			}
			name := strings.TrimSpace(nameEntry.Text)
			if name == "" {
				pp.app.logsPanel.Log("[WARN]Preset name cannot be empty")
				return
			}
			if name == addNewItem {
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
		},
		pp.app.window,
	)
	dlg.Resize(fyne.NewSize(300, 150))
	dlg.Show()
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
