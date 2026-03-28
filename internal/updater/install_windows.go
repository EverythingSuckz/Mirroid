//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mirroid/internal/platform"
)

// DetectInstallType determines if the app was installed via Inno Setup
// (in Program Files) or is running as a portable binary.
func DetectInstallType() InstallType {
	exe, err := os.Executable()
	if err != nil {
		return InstallPortable
	}
	lower := strings.ToLower(exe)
	if strings.Contains(lower, "program files") {
		return InstallInstaller
	}
	return InstallPortable
}

// AssetName returns the GitHub release asset filename for this install type.
func AssetName(installType InstallType) string {
	if installType == InstallInstaller {
		return "mirroid-windows-amd64-setup.exe"
	}
	return "mirroid-windows-amd64.exe"
}

// Apply replaces the current binary with the downloaded file.
func Apply(downloadedPath, currentExePath string, installType InstallType) error {
	if installType == InstallInstaller {
		// Write a helper batch script that uses "start /wait" (ShellExecute)
		// to trigger UAC elevation, waits for the installer to finish, then
		// launches the updated app. exec.Command uses CreateProcess which
		// cannot trigger UAC for admin-required executables.
		scriptPath := filepath.Join(os.TempDir(), "mirroid-update.bat")
		script := fmt.Sprintf(
			"@echo off\r\nstart \"\" /wait \"%s\" /SILENT /SUPPRESSMSGBOXES /NORESTART /CLOSEAPPLICATIONS\r\nif exist \"%s\" start \"\" \"%s\"\r\ndel \"%%~f0\"\r\n",
			downloadedPath, currentExePath, currentExePath)

		if err := os.WriteFile(scriptPath, []byte(script), 0o644); err != nil {
			return fmt.Errorf("updater: write update script: %w", err)
		}

		cmd := exec.Command("cmd.exe", "/c", scriptPath)
		platform.HideConsole(cmd)
		if err := cmd.Start(); err != nil {
			os.Remove(scriptPath)
			return fmt.Errorf("updater: launch update script: %w", err)
		}
		return nil // caller should os.Exit(0)
	}

	// Portable: rename-and-replace
	oldPath := currentExePath + ".old"
	os.Remove(oldPath) // remove any leftover .old

	if err := os.Rename(currentExePath, oldPath); err != nil {
		return fmt.Errorf("updater: rename current exe: %w", err)
	}

	if err := moveFile(downloadedPath, currentExePath); err != nil {
		// Attempt rollback
		os.Rename(oldPath, currentExePath)
		return fmt.Errorf("updater: move new binary: %w", err)
	}

	return nil
}

// Restart launches the new binary and exits the current process.
func Restart(exePath string) error {
	cmd := exec.Command(exePath)
	platform.SuppressConsole(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("updater: restart: %w", err)
	}
	os.Exit(0)
	return nil // unreachable
}

// Cleanup removes leftover .old files from a previous update.
// Retries because the previous process may still be releasing its file handle.
func Cleanup(exePath string) {
	oldPath := exePath + ".old"
	for i := 0; i < 10; i++ {
		if err := os.Remove(oldPath); err == nil || os.IsNotExist(err) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Clean up leftover update batch script
	scriptPath := filepath.Join(os.TempDir(), "mirroid-update.bat")
	os.Remove(scriptPath) // best effort
}
