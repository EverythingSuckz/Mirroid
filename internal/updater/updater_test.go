package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int // >0, 0, <0
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"2.0.0", "1.9.9", 1},
		{"0.1.0", "0.0.9", 1},
		{"1.2.3", "1.2.3", 0},
		{"10.0.0", "9.9.9", 1},
	}
	for _, tt := range tests {
		got := CompareSemver(tt.a, tt.b)
		if (tt.want > 0 && got <= 0) || (tt.want == 0 && got != 0) || (tt.want < 0 && got >= 0) {
			t.Errorf("CompareSemver(%q, %q) = %d, want sign %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input string
		want  [3]int
	}{
		{"1.2.3", [3]int{1, 2, 3}},
		{"v1.2.3", [3]int{1, 2, 3}},
		{"0.0.1", [3]int{0, 0, 1}},
		{"10.20.30", [3]int{10, 20, 30}},
		{"invalid", [3]int{0, 0, 0}},
		{"1.2", [3]int{1, 2, 0}},
		{"", [3]int{0, 0, 0}},
	}
	for _, tt := range tests {
		got := ParseSemver(tt.input)
		if got != tt.want {
			t.Errorf("ParseSemver(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestFindAsset(t *testing.T) {
	assets := []Asset{
		{Name: "mirroid-windows-amd64.exe", BrowserDownloadURL: "https://example.com/win.exe", Size: 100},
		{Name: "mirroid-linux-amd64", BrowserDownloadURL: "https://example.com/linux", Size: 200},
		{Name: "Mirroid-v0.0.3-x86_64.AppImage", BrowserDownloadURL: "https://example.com/appimage", Size: 300},
	}

	// Exact match
	a := FindAsset(assets, "mirroid-linux-amd64")
	if a == nil || a.Name != "mirroid-linux-amd64" {
		t.Error("expected to find mirroid-linux-amd64")
	}

	// Not found
	a = FindAsset(assets, "nonexistent")
	if a != nil {
		t.Error("expected nil for nonexistent asset")
	}

	// Suffix match
	a = FindAssetBySuffix(assets, ".AppImage")
	if a == nil || a.Name != "Mirroid-v0.0.3-x86_64.AppImage" {
		t.Error("expected to find AppImage by suffix")
	}

	// Suffix not found
	a = FindAssetBySuffix(assets, ".dmg")
	if a != nil {
		t.Error("expected nil for nonexistent suffix")
	}
}

func TestCheckForUpdate(t *testing.T) {
	releaseResp := map[string]interface{}{
		"tag_name": "v0.0.3",
		"name":     "Release v0.0.3",
		"body":     "### Features\n- New feature",
		"html_url": "https://github.com/EverythingSuckz/Mirroid/releases/tag/v0.0.3",
		"assets": []map[string]interface{}{
			{
				"name":                 "mirroid-windows-amd64.exe",
				"browser_download_url": "https://github.com/download/win.exe",
				"size":                 25000000,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releaseResp)
	}))
	defer server.Close()

	// Test: current is older → update available
	u := &Updater{
		owner:   "EverythingSuckz",
		repo:    "Mirroid",
		current: "0.0.2",
		client:  server.Client(),
		baseURL: server.URL,
	}
	result, err := u.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Available {
		t.Error("expected update to be available")
	}
	if result.LatestVersion != "0.0.3" {
		t.Errorf("expected latest 0.0.3, got %s", result.LatestVersion)
	}
	if result.Release.Name != "Release v0.0.3" {
		t.Errorf("expected release name 'Release v0.0.3', got %s", result.Release.Name)
	}
	if len(result.Release.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(result.Release.Assets))
	}
	if result.Release.Assets[0].Name != "mirroid-windows-amd64.exe" {
		t.Errorf("expected asset name mirroid-windows-amd64.exe, got %s", result.Release.Assets[0].Name)
	}

	// Test: current matches latest → no update
	u.current = "0.0.3"
	result, err = u.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Available {
		t.Error("expected no update when versions match")
	}

	// Test: current is newer → no update
	u.current = "0.0.4"
	result, err = u.CheckForUpdate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Available {
		t.Error("expected no update when current is newer")
	}
}

func TestDetectInstallType(t *testing.T) {
	// We can only reliably test that DetectInstallType returns something valid
	// since we can't mock os.Executable in unit tests.
	it := DetectInstallType()
	if it != InstallPortable && it != InstallInstaller && it != InstallAppImage && it != InstallSystem {
		t.Errorf("unexpected install type: %d", it)
	}
}
