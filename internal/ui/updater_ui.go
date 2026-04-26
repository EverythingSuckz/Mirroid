package ui

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/platform"
	"mirroid/internal/updater"
)

// getToolVersion runs a CLI tool with a version flag and returns a cleaned-up version string.
func getToolVersion(configPath, fallback, flag string) string {
	bin := configPath
	if bin == "" {
		bin = fallback
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, flag)
	platform.HideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "not found"
	}

	raw := strings.TrimSpace(string(out))

	// Take only the first line of output.
	if first, _, ok := strings.Cut(raw, "\n"); ok {
		raw = first
	}

	// adb: "Android Debug Bridge version 1.0.41" → "1.0.41"
	if strings.HasPrefix(raw, "Android Debug Bridge version") {
		raw = strings.TrimPrefix(raw, "Android Debug Bridge version ")
	}

	// scrcpy: "scrcpy 3.3.4 <https://github.com/Genymobile/scrcpy>" → "3.3.4"
	if strings.HasPrefix(raw, "scrcpy ") {
		raw = strings.TrimPrefix(raw, "scrcpy ")
		if idx := strings.IndexByte(raw, ' '); idx != -1 {
			raw = raw[:idx]
		}
	}

	return strings.TrimSpace(raw)
}

// checkForUpdates checks for available updates.
// If silent is true (startup auto-check), no UI is shown unless an update is available.
func (a *App) checkForUpdates(silent bool) {
	version := a.fyneApp.Metadata().Version
	if version == "" {
		version = "0.0.0"
	}

	u := updater.New(version)

	if !silent {
		// Show "Checking..." dialog for manual checks
		checking := dialog.NewCustomWithoutButtons("Checking for Updates",
			container.NewVBox(
				widget.NewLabel("Checking for updates..."),
				widget.NewProgressBarInfinite(),
			), a.window)
		checking.Resize(fyne.NewSize(300, 120))
		checking.Show()

		go func() {
			result, err := u.CheckForUpdate()
			// save timestamp only on successful check
			if err == nil {
				a.cfg.AppConf.LastUpdateCheck = time.Now().Unix()
				_ = a.cfg.SaveAppConfig()
			}
			// fetch clean changelog (no download table, no hashes)
			var changelog string
			if err == nil && result.Available {
				changelog = u.FetchChangelog(result.Release.TagName)
			}
			fyne.Do(func() {
				checking.Hide()
				if err != nil {
					slog.Error("update check failed", "error", err)
					dialog.ShowError(fmt.Errorf("Could not check for updates:\n%s", err), a.window)
					return
				}
				if result.Available {
					a.showUpdateDialog(u, result, changelog)
				} else {
					dialog.ShowInformation("Up to Date",
						fmt.Sprintf("You're running the latest version (v%s).", result.CurrentVersion),
						a.window)
				}
			})
		}()
		return
	}

	// silent mode: no UI unless update is available
	go func() {
		result, err := u.CheckForUpdate()
		if err != nil {
			slog.Warn("silent update check failed", "error", err)
			return
		}
		// save timestamp only on successful check
		a.cfg.AppConf.LastUpdateCheck = time.Now().Unix()
		_ = a.cfg.SaveAppConfig()
		if result.Available {
			changelog := u.FetchChangelog(result.Release.TagName)
			fyne.Do(func() {
				a.showUpdateDialog(u, result, changelog)
			})
		}
	}()
}

// showUpdateDialog shows the update-available dialog with changelog and install button.
func (a *App) showUpdateDialog(u *updater.Updater, result *updater.UpdateResult, changelog string) {
	versionLabel := widget.NewLabelWithStyle(
		fmt.Sprintf("v%s  →  v%s", result.CurrentVersion, result.LatestVersion),
		fyne.TextAlignCenter, fyne.TextStyle{Bold: true},
	)

	// prefer clean changelog from changelog.txt; fall back to release body
	body := changelog
	if body == "" {
		body = result.Release.Body
	}

	var changelogWidget fyne.CanvasObject
	if body != "" {
		rt := widget.NewRichTextFromMarkdown(body)
		rt.Wrapping = fyne.TextWrapWord
		changelogWidget = container.NewVScroll(rt)
		changelogWidget.(*container.Scroll).SetMinSize(fyne.NewSize(450, 200))
	} else {
		changelogWidget = widget.NewLabel("No changelog available.")
	}

	installType := updater.DetectInstallType()

	var dlg dialog.Dialog
	var actionBtn *widget.Button

	if installType == updater.InstallSystem {
		// .deb users: open browser to release page
		actionBtn = widget.NewButton("View on GitHub", func() {
			ghURL, err := url.Parse(result.Release.HTMLURL)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Invalid release URL: %s", err), a.window)
				return
			}
			_ = a.fyneApp.OpenURL(ghURL)
			dlg.Hide()
		})
	} else {
		actionBtn = widget.NewButton("Install Update", func() {
			dlg.Hide()
			a.performUpdate(u, result)
		})
	}
	actionBtn.Importance = widget.HighImportance

	laterBtn := widget.NewButton("Later", func() {
		dlg.Hide()
	})

	buttons := container.NewHBox(laterBtn, actionBtn)

	content := container.NewVBox(
		container.NewCenter(versionLabel),
		widget.NewSeparator(),
		changelogWidget,
		widget.NewSeparator(),
		container.NewCenter(buttons),
	)

	dlg = dialog.NewCustomWithoutButtons("Update Available", content, a.window)
	dlg.Resize(fyne.NewSize(500, 400))
	dlg.Show()
}

// performUpdate downloads and applies the update with a progress dialog.
func (a *App) performUpdate(u *updater.Updater, result *updater.UpdateResult) {
	installType := updater.DetectInstallType()

	assetName := updater.AssetName(installType)
	var asset *updater.Asset
	if assetName != "" {
		asset = updater.FindAsset(result.Release.Assets, assetName)
	} else if installType == updater.InstallAppImage {
		asset = updater.FindAssetBySuffix(result.Release.Assets, ".AppImage")
	}

	if asset == nil {
		dialog.ShowError(fmt.Errorf("Could not find a compatible update for your platform."), a.window)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())

	progressBar := widget.NewProgressBar()
	statusLabel := widget.NewLabel("Downloading update...")
	cancelBtn := widget.NewButton("Cancel", func() {
		cancel()
	})

	progressContent := container.NewVBox(
		statusLabel,
		progressBar,
		container.NewCenter(cancelBtn),
	)

	progressDlg := dialog.NewCustomWithoutButtons("Updating Mirroid", progressContent, a.window)
	progressDlg.Resize(fyne.NewSize(400, 150))
	progressDlg.Show()

	go func() {
		defer cancel()
		exe, err := os.Executable()
		if err != nil {
			fyne.Do(func() {
				progressDlg.Hide()
				dialog.ShowError(fmt.Errorf("Could not determine executable path: %s", err), a.window)
			})
			return
		}
		exe, _ = filepath.EvalSymlinks(exe)

		// for AppImage, use $APPIMAGE instead of the mounted FUSE binary
		if installType == updater.InstallAppImage {
			if appImage := os.Getenv("APPIMAGE"); appImage != "" {
				exe = appImage
			}
		}

		// choose download directory:
		// - Installer installs (Program Files) → OS temp dir (no write access to install dir)
		// - Portable / AppImage → exe directory (same filesystem for atomic rename)
		destDir := filepath.Dir(exe)
		if installType == updater.InstallInstaller {
			destDir = os.TempDir()
		}

		// download with throttled progress updates (at most every 150ms)
		var lastProgressUpdate time.Time
		tmpPath, err := u.Download(ctx, asset.BrowserDownloadURL, destDir, func(received, total int64) {
			if total <= 0 {
				return
			}
			now := time.Now()
			if received < total && now.Sub(lastProgressUpdate) < 150*time.Millisecond {
				return
			}
			lastProgressUpdate = now
			fyne.Do(func() {
				progressBar.SetValue(float64(received) / float64(total))
				statusLabel.SetText(fmt.Sprintf("Downloading... %.1f / %.1f MB",
					float64(received)/1024/1024, float64(total)/1024/1024))
			})
		})
		if err != nil {
			fyne.Do(func() {
				progressDlg.Hide()
				if ctx.Err() == nil {
					dialog.ShowError(fmt.Errorf("Download failed: %s", err), a.window)
				}
			})
			return
		}

		fyne.Do(func() {
			statusLabel.SetText("Applying update...")
			progressBar.SetValue(1.0)
			cancelBtn.Hide()
		})

		if err := updater.Apply(tmpPath, exe, installType); err != nil {
			os.Remove(tmpPath)
			fyne.Do(func() {
				progressDlg.Hide()
				dialog.ShowError(fmt.Errorf("Update failed: %s", err), a.window)
			})
			return
		}

		fyne.Do(func() {
			statusLabel.SetText("Restarting...")
		})

		// for installer type, the update script was launched; exit so the installer can replace files
		if installType == updater.InstallInstaller {
			os.Exit(0)
		}

		// Restart
		if err := updater.Restart(exe); err != nil {
			fyne.Do(func() {
				progressDlg.Hide()
				dialog.ShowError(fmt.Errorf("Restart failed: %s\nPlease restart manually.", err), a.window)
			})
		}
	}()
}

// shouldAutoCheck returns true if enough time has passed since the last update check.
func (a *App) shouldAutoCheck() bool {
	last := a.cfg.AppConf.LastUpdateCheck
	if last == 0 {
		return true
	}
	const cooldown = 12 * 60 * 60 // 12 hours in seconds
	return time.Now().Unix()-last > cooldown
}
