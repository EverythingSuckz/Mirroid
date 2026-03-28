//go:build !windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// DetectInstallType determines the install type on Linux/macOS.
func DetectInstallType() InstallType {
	if os.Getenv("APPIMAGE") != "" {
		return InstallAppImage
	}

	exe, err := os.Executable()
	if err != nil {
		return InstallPortable
	}
	exe, _ = filepath.EvalSymlinks(exe)

	if runtime.GOOS == "linux" && strings.HasPrefix(exe, "/usr/bin") {
		return InstallSystem
	}

	return InstallPortable
}

// AssetName returns the GitHub release asset filename for this install type.
func AssetName(installType InstallType) string {
	switch installType {
	case InstallAppImage:
		return "" // use FindAssetBySuffix(".AppImage")
	case InstallSystem:
		return "" // .deb users get browser redirect
	default:
		if runtime.GOOS == "darwin" {
			return "mirroid-macos-arm64"
		}
		return "mirroid-linux-amd64"
	}
}

// Apply replaces the current binary with the downloaded file.
func Apply(downloadedPath, currentExePath string, installType InstallType) error {
	if installType == InstallSystem {
		return fmt.Errorf("system package installs cannot be updated in-app; use your package manager")
	}

	targetPath := currentExePath
	if installType == InstallAppImage {
		targetPath = os.Getenv("APPIMAGE")
		if targetPath == "" {
			return fmt.Errorf("updater: $APPIMAGE not set")
		}
	}

	// Preserve existing file permissions
	info, err := os.Stat(targetPath)
	if err != nil {
		return fmt.Errorf("updater: stat current binary: %w", err)
	}

	if err := os.Chmod(downloadedPath, info.Mode()); err != nil {
		return fmt.Errorf("updater: chmod: %w", err)
	}

	// Atomic replace on Unix
	if err := os.Rename(downloadedPath, targetPath); err != nil {
		// Fall back to moveFile for cross-device
		if err2 := moveFile(downloadedPath, targetPath); err2 != nil {
			return fmt.Errorf("updater: replace binary: %w", err2)
		}
	}

	return nil
}

// Restart launches the new binary and exits the current process.
func Restart(exePath string) error {
	cmd := exec.Command(exePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("updater: restart: %w", err)
	}
	os.Exit(0)
	return nil // unreachable
}

// Cleanup is a no-op on Linux/macOS.
func Cleanup(_ string) {}
