package deps

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBinaryFileName(t *testing.T) {
	adbName := binaryFileName(BinADB)
	scrcpyName := binaryFileName(BinScrcpy)

	if runtime.GOOS == "windows" {
		if adbName != "adb.exe" {
			t.Errorf("expected adb.exe, got %s", adbName)
		}
		if scrcpyName != "scrcpy.exe" {
			t.Errorf("expected scrcpy.exe, got %s", scrcpyName)
		}
	} else {
		if adbName != "adb" {
			t.Errorf("expected adb, got %s", adbName)
		}
		if scrcpyName != "scrcpy" {
			t.Errorf("expected scrcpy, got %s", scrcpyName)
		}
	}
}

func TestDetect_AppDir(t *testing.T) {
	dir := t.TempDir()
	name := binaryFileName(BinADB)
	fakebin := filepath.Join(dir, name)
	if err := os.WriteFile(fakebin, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := Detect(BinADB, dir, "")
	if !result.Found {
		t.Fatal("expected to find adb in app dir")
	}
	if result.Source != "app_dir" {
		t.Errorf("expected source app_dir, got %s", result.Source)
	}
	if result.Path != fakebin {
		t.Errorf("expected path %s, got %s", fakebin, result.Path)
	}
}

func TestDetect_LibDir(t *testing.T) {
	// Simulate Linux .deb layout: /usr/bin/ (appDir) + /usr/lib/mirroid/ (deps)
	base := t.TempDir()
	binDir := filepath.Join(base, "usr", "bin")
	libDir := filepath.Join(base, "usr", "lib", "mirroid")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		t.Fatal(err)
	}

	name := binaryFileName(BinScrcpy)
	fakebin := filepath.Join(libDir, name)
	if err := os.WriteFile(fakebin, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := Detect(BinScrcpy, binDir, "")
	if !result.Found {
		t.Fatal("expected to find scrcpy in lib dir")
	}
	if result.Source != "lib_dir" {
		t.Errorf("expected source lib_dir, got %s", result.Source)
	}
}

func TestDetect_ConfigPath(t *testing.T) {
	dir := t.TempDir()
	name := binaryFileName(BinADB)
	fakebin := filepath.Join(dir, name)
	if err := os.WriteFile(fakebin, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Pass empty appDir so it doesn't find it there
	result := Detect(BinADB, "", fakebin)
	if !result.Found {
		t.Fatal("expected to find adb via config path")
	}
	if result.Source != "config" {
		t.Errorf("expected source config, got %s", result.Source)
	}
}

func TestDetect_PriorityOrder(t *testing.T) {
	// Both app dir and config path have the binary — app dir should win
	appDir := t.TempDir()
	configDir := t.TempDir()
	name := binaryFileName(BinADB)

	appBin := filepath.Join(appDir, name)
	configBin := filepath.Join(configDir, name)
	if err := os.WriteFile(appBin, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configBin, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	result := Detect(BinADB, appDir, configBin)
	if result.Source != "app_dir" {
		t.Errorf("expected app_dir to win over config, got %s", result.Source)
	}
}

func TestDetect_NotFound(t *testing.T) {
	// Use a binary name that definitely doesn't exist on PATH
	const fakeBin BinaryName = "nonexistent_mirroid_test_binary"
	result := Detect(fakeBin, t.TempDir(), "")
	if result.Found {
		t.Error("expected not found for non-existent binary")
	}
	if result.Binary != fakeBin {
		t.Errorf("expected binary %s, got %s", fakeBin, result.Binary)
	}
}

func TestDetectAll(t *testing.T) {
	dir := t.TempDir()
	adbName := binaryFileName(BinADB)
	scrcpyName := binaryFileName(BinScrcpy)

	if err := os.WriteFile(filepath.Join(dir, adbName), []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, scrcpyName), []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}

	adbR, scrcpyR := DetectAll(dir, "", "")
	if !adbR.Found {
		t.Error("expected adb found")
	}
	if !scrcpyR.Found {
		t.Error("expected scrcpy found")
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()

	// Non-existent
	if fileExists(filepath.Join(dir, "nope")) {
		t.Error("expected false for non-existent file")
	}

	// Directory (should return false)
	if fileExists(dir) {
		t.Error("expected false for directory")
	}

	// Regular file
	f := filepath.Join(dir, "exists")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !fileExists(f) {
		t.Error("expected true for existing file")
	}
}
