package ui

import (
	"context"
	"image/color"
	"log/slog"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/theme"

	"mirroid/internal/config"
	"mirroid/internal/platform"
)

// variantTheme wraps the default fyne theme but forces a specific variant
// in Color(). replaces the deprecated theme.DarkTheme() / theme.LightTheme().
type variantTheme struct {
	variant fyne.ThemeVariant
}

var _ fyne.Theme = (*variantTheme)(nil)

func (t *variantTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return theme.DefaultTheme().Color(name, t.variant)
}

func (t *variantTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (t *variantTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (t *variantTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}

// ThemeManager handles theme switching, config persistence, and OS theme polling.
type ThemeManager struct {
	app    fyne.App
	window fyne.Window
	cfg    *config.Config
	mode   config.ThemeMode

	cancelWatch context.CancelFunc
	mu          sync.Mutex
}

// normalizeMode maps unrecognised or empty values to ThemeModeSystem.
func normalizeMode(m config.ThemeMode) config.ThemeMode {
	switch m {
	case config.ThemeModeSystem, config.ThemeModeDark, config.ThemeModeLight:
		return m
	default:
		return config.ThemeModeSystem
	}
}

// NewThemeManager reads the saved preference, applies it, and starts the
// OS watcher if in system mode.
func NewThemeManager(app fyne.App, cfg *config.Config, window fyne.Window) *ThemeManager {
	mode := normalizeMode(cfg.AppConf.ThemeMode)

	tm := &ThemeManager{
		app:    app,
		window: window,
		cfg:    cfg,
		mode:   mode,
	}

	tm.applyTheme()

	if mode == config.ThemeModeSystem {
		tm.startSystemWatcher()
	}

	return tm
}

// SetMode switches theme, persists the preference, and manages the OS watcher.
func (tm *ThemeManager) SetMode(mode config.ThemeMode) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.stopWatcherLocked()

	mode = normalizeMode(mode)
	tm.mode = mode
	tm.cfg.AppConf.ThemeMode = mode
	if err := tm.cfg.SaveAppConfig(); err != nil {
		slog.Warn("could not save theme preference", "error", err)
	}

	tm.applyTheme()

	if mode == config.ThemeModeSystem {
		tm.startSystemWatcher()
	}
}

// Mode returns the current theme mode.
func (tm *ThemeManager) Mode() config.ThemeMode {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	return normalizeMode(tm.mode)
}

// RefreshTitleBar re-applies the title bar attribute once the native
// window handle is available (call after Show).
func (tm *ThemeManager) RefreshTitleBar() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	dark := tm.mode == config.ThemeModeDark ||
		(tm.mode != config.ThemeModeLight && platform.SystemThemeIsDark())
	tm.updateTitleBar(dark)
}

// Stop cancels the OS watcher goroutine.
func (tm *ThemeManager) Stop() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.stopWatcherLocked()
}

// applyTheme sets the fyne theme and updates the native title bar.
// must be called while tm.mu is held or during initialisation.
func (tm *ThemeManager) applyTheme() {
	var dark bool
	switch tm.mode {
	case config.ThemeModeDark:
		dark = true
	case config.ThemeModeLight:
		dark = false
	default:
		dark = platform.SystemThemeIsDark()
	}

	if dark {
		tm.app.Settings().SetTheme(&variantTheme{variant: theme.VariantDark})
	} else {
		tm.app.Settings().SetTheme(&variantTheme{variant: theme.VariantLight})
	}

	tm.updateTitleBar(dark)
}

// updateTitleBar calls the platform-specific title bar API.
func (tm *ThemeManager) updateTitleBar(dark bool) {
	if tm.window == nil {
		return
	}
	if nw, ok := tm.window.(driver.NativeWindow); ok {
		nw.RunNative(func(ctx any) {
			if winCtx, ok := ctx.(driver.WindowsWindowContext); ok {
				platform.SetTitleBarDarkMode(winCtx.HWND, dark)
			}
		})
	}
}

// startSystemWatcher polls the OS theme every 5s and re-applies on change.
func (tm *ThemeManager) startSystemWatcher() {
	ctx, cancel := context.WithCancel(context.Background())
	tm.cancelWatch = cancel

	go func() {
		lastDark := platform.SystemThemeIsDark()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				dark := platform.SystemThemeIsDark()
				if dark != lastDark {
					lastDark = dark
					fyne.Do(func() {
						tm.mu.Lock()
						defer tm.mu.Unlock()
						if tm.mode == config.ThemeModeSystem {
							tm.applyTheme()
						}
					})
				}
			}
		}
	}()
}

func (tm *ThemeManager) stopWatcherLocked() {
	if tm.cancelWatch != nil {
		tm.cancelWatch()
		tm.cancelWatch = nil
	}
}
