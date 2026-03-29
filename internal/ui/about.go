package ui

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"mirroid/internal/updater"
)

// showAboutDialog shows a modal About dialog with app info.
func (a *App) showAboutDialog() {
	icon := canvas.NewImageFromResource(theme.ComputerIcon())
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(64, 64))

	version := a.fyneApp.Metadata().Version
	if version == "" {
		version = "dev"
	}

	title := canvas.NewText("Mirroid", theme.Color(theme.ColorNameForeground))
	title.TextSize = 22
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.Alignment = fyne.TextAlignCenter

	versionLabel := widget.NewLabelWithStyle(
		"Version "+version,
		fyne.TextAlignCenter, fyne.TextStyle{Italic: true},
	)

	description := widget.NewLabelWithStyle(
		"A desktop GUI for scrcpy",
		fyne.TextAlignCenter, fyne.TextStyle{},
	)

	ghURL, _ := url.Parse("https://github.com/EverythingSuckz/Mirroid")
	githubLink := widget.NewHyperlink("GitHub", ghURL)

	content := container.NewVBox(
		container.NewCenter(icon),
		container.NewCenter(title),
		container.NewCenter(versionLabel),
		container.NewCenter(description),
		widget.NewSeparator(),
		container.NewCenter(githubLink),
	)

	dlg := dialog.NewCustom("About Mirroid", "Close", content, a.window)
	dlg.Resize(fyne.NewSize(350, 320))
	dlg.Show()
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
			// Save timestamp only on successful check
			if err == nil {
				a.cfg.AppConf.LastUpdateCheck = time.Now().Unix()
				_ = a.cfg.SaveAppConfig()
			}
			// Fetch clean changelog (no download table, no hashes)
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

	// Silent mode: no UI unless update is available
	go func() {
		result, err := u.CheckForUpdate()
		if err != nil {
			slog.Debug("silent update check failed", "error", err)
			return
		}
		// Save timestamp only on successful check
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

	// Prefer clean changelog from changelog.txt; fall back to release body
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
			ghURL, _ := url.Parse(result.Release.HTMLURL)
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

	// Find the right asset
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

	// Progress UI
	progressBar := widget.NewProgressBar()
	statusLabel := widget.NewLabel("Downloading update...")

	progressContent := container.NewVBox(
		statusLabel,
		progressBar,
	)

	progressDlg := dialog.NewCustomWithoutButtons("Updating Mirroid", progressContent, a.window)
	progressDlg.Resize(fyne.NewSize(400, 130))
	progressDlg.Show()

	go func() {
		exe, err := os.Executable()
		if err != nil {
			fyne.Do(func() {
				progressDlg.Hide()
				dialog.ShowError(fmt.Errorf("Could not determine executable path: %s", err), a.window)
			})
			return
		}
		exe, _ = filepath.EvalSymlinks(exe)

		// Choose download directory:
		// - Installer installs (Program Files) → OS temp dir (no write access to install dir)
		// - Portable → exe directory (same filesystem for atomic rename)
		destDir := filepath.Dir(exe)
		if installType == updater.InstallInstaller {
			destDir = os.TempDir()
		}

		// Download with throttled progress updates (at most every 150ms)
		var lastProgressUpdate time.Time
		tmpPath, err := u.Download(asset.BrowserDownloadURL, destDir, func(received, total int64) {
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
				dialog.ShowError(fmt.Errorf("Download failed: %s", err), a.window)
			})
			return
		}

		fyne.Do(func() {
			statusLabel.SetText("Applying update...")
			progressBar.SetValue(1.0)
		})

		// Apply
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

		// For installer type, the update script was launched; exit so the installer can replace files
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
