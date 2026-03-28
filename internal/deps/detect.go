package deps

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// BinaryName represents a required external dependency.
type BinaryName string

const (
	BinADB    BinaryName = "adb"
	BinScrcpy BinaryName = "scrcpy"
)

// DetectResult holds the detection outcome for a single binary.
type DetectResult struct {
	Binary BinaryName
	Found  bool
	Path   string // absolute path to resolved binary
	Source string // "app_dir" | "lib_dir" | "config" | "path"
}

// DetectAll checks both adb and scrcpy, returning results for each.
func DetectAll(appDir, configADB, configScrcpy string) (adbResult, scrcpyResult DetectResult) {
	adbResult = Detect(BinADB, appDir, configADB)
	scrcpyResult = Detect(BinScrcpy, appDir, configScrcpy)
	return
}

// Detect checks a single binary by searching these locations in order:
//  1. App directory (flat alongside exe) — covers Windows installer, portable, Linux tarball, macOS .app
//  2. Lib directory (appDir/../lib/mirroid/) — covers Linux .deb and AppImage
//  3. Config path — if absolute and file exists (returning users with custom paths)
//  4. System PATH — exec.LookPath (users who installed separately)
func Detect(bin BinaryName, appDir, configPath string) DetectResult {
	name := binaryFileName(bin)

	// 1. App directory
	if appDir != "" {
		p := filepath.Join(appDir, name)
		if fileExists(p) {
			return DetectResult{Binary: bin, Found: true, Path: p, Source: "app_dir"}
		}
	}

	// 2. Lib directory (for Linux .deb / AppImage: /usr/bin/../lib/mirroid/)
	if appDir != "" {
		p := filepath.Join(appDir, "..", "lib", "mirroid", name)
		if abs, err := filepath.Abs(p); err == nil && fileExists(abs) {
			return DetectResult{Binary: bin, Found: true, Path: abs, Source: "lib_dir"}
		}
	}

	// 3. Config path (if it's an absolute path and exists)
	if configPath != "" && filepath.IsAbs(configPath) && fileExists(configPath) {
		return DetectResult{Binary: bin, Found: true, Path: configPath, Source: "config"}
	}

	// 4. System PATH
	if p, err := exec.LookPath(string(bin)); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return DetectResult{Binary: bin, Found: true, Path: abs, Source: "path"}
		}
	}

	return DetectResult{Binary: bin, Found: false}
}

// binaryFileName returns the platform-specific filename for a binary.
func binaryFileName(bin BinaryName) string {
	name := string(bin)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// fileExists checks if a path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
