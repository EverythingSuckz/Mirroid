package ui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/model"
)

// PresetsPanel manages save/load of option presets.
type PresetsPanel struct {
	app          *App
	presetName   *widget.Entry
	presetSelect *widget.Select
	presets      map[string]model.ScrcpyOptions
}

// NewPresetsPanel creates a new presets panel.
func NewPresetsPanel(app *App) *PresetsPanel {
	return &PresetsPanel{
		app:     app,
		presets: make(map[string]model.ScrcpyOptions),
	}
}

// Build creates the presets UI --compact single row.
func (pp *PresetsPanel) Build() fyne.CanvasObject {
	presets, err := pp.app.cfg.LoadPresets()
	if err != nil {
		pp.app.logsPanel.Log("[WARN]" + err.Error())
	} else {
		pp.presets = presets
	}

	pp.presetName = widget.NewEntry()
	pp.presetName.SetPlaceHolder("Name...")

	pp.presetSelect = widget.NewSelect(pp.presetNames(), nil)
	pp.presetSelect.PlaceHolder = "Select..."

	saveBtn := widget.NewButton("Save", pp.onSave)
	loadBtn := widget.NewButton("Load", pp.onLoad)
	deleteBtn := widget.NewButton("Del", pp.onDelete)
	deleteBtn.Importance = widget.DangerImportance

	return container.NewVBox(
		container.NewBorder(nil, nil, nil, saveBtn, pp.presetName),
		container.NewBorder(nil, nil, nil, container.NewHBox(loadBtn, deleteBtn), pp.presetSelect),
		layout.NewSpacer(),
	)
}

func (pp *PresetsPanel) presetNames() []string {
	names := make([]string, 0, len(pp.presets))
	for name := range pp.presets {
		names = append(names, name)
	}
	return names
}

func (pp *PresetsPanel) onSave() {
	name := strings.TrimSpace(pp.presetName.Text)
	if name == "" {
		pp.app.logsPanel.Log("[WARN]Preset name cannot be empty")
		return
	}

	var opts model.ScrcpyOptions
	pp.app.optionsPanel.SyncToModel(&opts)

	pp.presets[name] = opts
	if err := pp.app.cfg.SavePresets(pp.presets); err != nil {
		pp.app.logsPanel.Log("[ERROR]" + err.Error())
		return
	}

	pp.presetSelect.Options = pp.presetNames()
	pp.presetSelect.Refresh()
	pp.app.logsPanel.Log("[OK]Saved: " + name)
}

func (pp *PresetsPanel) onLoad() {
	name := pp.presetSelect.Selected
	if name == "" {
		return
	}

	opts, ok := pp.presets[name]
	if !ok {
		return
	}

	pp.app.optionsPanel.SyncFromModel(opts)
	pp.app.options = opts
	pp.app.logsPanel.Log("[OK]Loaded: " + name)
}

func (pp *PresetsPanel) onDelete() {
	name := pp.presetSelect.Selected
	if name == "" {
		return
	}

	delete(pp.presets, name)
	if err := pp.app.cfg.SavePresets(pp.presets); err != nil {
		pp.app.logsPanel.Log("[WARN]failed to save presets: " + err.Error())
	}

	pp.presetSelect.Options = pp.presetNames()
	pp.presetSelect.ClearSelected()
	pp.presetSelect.Refresh()
	pp.app.logsPanel.Log("[OK]Deleted: " + name)
}
