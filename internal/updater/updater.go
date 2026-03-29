package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Release represents a parsed GitHub release.
type Release struct {
	TagName string
	Name    string
	Body    string // markdown changelog
	HTMLURL string // link to release page
	Assets  []Asset
}

// Asset represents a downloadable release artifact.
type Asset struct {
	Name               string
	BrowserDownloadURL string
	Size               int64
}

// UpdateResult is returned by CheckForUpdate.
type UpdateResult struct {
	Available      bool
	CurrentVersion string
	LatestVersion  string
	Release        Release
}

// ProgressFunc is called during download with bytes received and total size.
type ProgressFunc func(received, total int64)

// Updater checks for and applies updates from GitHub Releases.
type Updater struct {
	owner   string
	repo    string
	current string
	client  *http.Client // short timeout for API calls
	baseURL string       // GitHub API base URL (overridable for tests)
}

// New creates an Updater for the given current version.
// The version should be in "MAJOR.MINOR.PATCH" format (no leading "v").
func New(currentVersion string) *Updater {
	return &Updater{
		owner:   "EverythingSuckz",
		repo:    "Mirroid",
		current: strings.TrimPrefix(currentVersion, "v"),
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://api.github.com",
	}
}

// CheckForUpdate queries the GitHub Releases API for the latest release
// and compares its tag against the current version.
func (u *Updater) CheckForUpdate() (*UpdateResult, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", u.baseURL, u.owner, u.repo)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Mirroid-Updater")

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("updater: network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("updater: GitHub API returned %d", resp.StatusCode)
	}

	var gh struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
			Size               int64  `json:"size"`
		} `json:"assets"`
	}

	// Cap response body at 1MB to prevent unbounded reads
	limited := io.LimitReader(resp.Body, 1<<20)
	if err := json.NewDecoder(limited).Decode(&gh); err != nil {
		return nil, fmt.Errorf("updater: failed to parse response: %w", err)
	}

	latest := strings.TrimPrefix(gh.TagName, "v")

	assets := make([]Asset, len(gh.Assets))
	for i, a := range gh.Assets {
		assets[i] = Asset{
			Name:               a.Name,
			BrowserDownloadURL: a.BrowserDownloadURL,
			Size:               a.Size,
		}
	}

	return &UpdateResult{
		Available:      CompareSemver(latest, u.current) > 0,
		CurrentVersion: u.current,
		LatestVersion:  latest,
		Release: Release{
			TagName: gh.TagName,
			Name:    gh.Name,
			Body:    gh.Body,
			HTMLURL: gh.HTMLURL,
			Assets:  assets,
		},
	}, nil
}

// FetchChangelog downloads the changelog.txt asset from the given release tag.
// Returns the content as a string, or empty string on any failure.
func (u *Updater) FetchChangelog(tagName string) string {
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/changelog.txt", u.owner, u.repo, tagName)
	resp, err := u.client.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}

// Download fetches the asset at the given URL to a temporary file in destDir.
// destDir should be on the same filesystem as the target for atomic rename.
// progress is called periodically with bytes received and total size.
func (u *Updater) Download(assetURL, destDir string, progress ProgressFunc) (string, error) {
	// Use a separate client with no timeout for large downloads.
	// The API client has a 30s timeout which would kill multi-MB transfers.
	dlClient := &http.Client{}
	resp, err := dlClient.Get(assetURL)
	if err != nil {
		return "", fmt.Errorf("updater: download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("updater: download returned %d", resp.StatusCode)
	}

	total := resp.ContentLength

	// On Windows, temp files need .exe extension to be executable
	ext := ".tmp"
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	tmpFile, err := os.CreateTemp(destDir, "mirroid-update-*"+ext)
	if err != nil {
		return "", fmt.Errorf("updater: create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	var received int64
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := tmpFile.Write(buf[:n]); writeErr != nil {
				tmpFile.Close()
				os.Remove(tmpPath)
				return "", fmt.Errorf("updater: write: %w", writeErr)
			}
			received += int64(n)
			if progress != nil {
				progress(received, total)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			tmpFile.Close()
			os.Remove(tmpPath)
			return "", fmt.Errorf("updater: read: %w", readErr)
		}
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", err
	}

	return tmpPath, nil
}

// FindAsset finds an asset by exact name match.
func FindAsset(assets []Asset, name string) *Asset {
	for i := range assets {
		if assets[i].Name == name {
			return &assets[i]
		}
	}
	return nil
}

// FindAssetBySuffix finds the first asset whose name ends with the given suffix.
func FindAssetBySuffix(assets []Asset, suffix string) *Asset {
	for i := range assets {
		if strings.HasSuffix(assets[i].Name, suffix) {
			return &assets[i]
		}
	}
	return nil
}

// CompareSemver returns >0 if a > b, 0 if a == b, <0 if a < b.
// Both inputs must be "MAJOR.MINOR.PATCH" format (no leading "v").
func CompareSemver(a, b string) int {
	pa := ParseSemver(a)
	pb := ParseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] - pb[i]
		}
	}
	return 0
}

// ParseSemver splits "1.2.3" into [1, 2, 3]. Invalid input yields [0, 0, 0].
func ParseSemver(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	var result [3]int
	parts := strings.SplitN(v, ".", 3)
	for i, p := range parts {
		if i >= 3 {
			break
		}
		n, _ := strconv.Atoi(p)
		result[i] = n
	}
	return result
}

// moveFile tries os.Rename first; falls back to copy+delete for cross-device.
// Retries briefly to handle antivirus locks on newly downloaded executables.
func moveFile(src, dst string) error {
	// Retry rename — on Windows, antivirus may briefly lock new executables.
	var renameErr error
	for i := 0; i < 5; i++ {
		if renameErr = os.Rename(src, dst); renameErr == nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Cross-device fallback: copy bytes then delete source.
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		in.Close()
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		in.Close()
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		in.Close()
		return err
	}
	if err := out.Close(); err != nil {
		in.Close()
		return err
	}
	// Must close source before deleting — Windows locks open files.
	in.Close()
	return os.Remove(src)
}
