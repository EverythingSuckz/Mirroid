package scrcpy

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"mirroid/internal/platform"
)

// CameraInfo describes a single camera reported by `scrcpy --list-cameras`.
// Tagged for JSON because the list is cached per-device on disk.
type CameraInfo struct {
	ID     string `json:"id"`
	Facing string `json:"facing"`
	Size   string `json:"size"`
}

// listCamerasRe matches lines like:
//
//	--camera-id=0  (back, 4032x3024)
//	--camera-id=2  (external, 1920x1080)
var listCamerasRe = regexp.MustCompile(`--camera-id=(\S+)\s*\(([^,)]+)(?:,\s*([^)]+))?\)`)

// ListCameras runs `scrcpy --list-cameras -s <serial>` and returns the
// detected cameras for the given device. The 5s context guards against
// scrcpy hanging if adb is unresponsive.
func (r *Runner) ListCameras(serial string) ([]CameraInfo, error) {
	if r.scrcpyPath == "" {
		return nil, fmt.Errorf("scrcpy path not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, r.scrcpyPath, "-s", serial, "--list-cameras")
	platform.HideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("scrcpy --list-cameras: %w", err)
	}

	cams := parseListCameras(out)
	if len(cams) == 0 && err != nil {
		return nil, fmt.Errorf("scrcpy --list-cameras: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return cams, nil
}

func parseListCameras(out []byte) []CameraInfo {
	matches := listCamerasRe.FindAllStringSubmatch(string(out), -1)
	cams := make([]CameraInfo, 0, len(matches))
	for _, m := range matches {
		cams = append(cams, CameraInfo{
			ID:     m[1],
			Facing: strings.TrimSpace(m[2]),
			Size:   strings.TrimSpace(m[3]),
		})
	}
	return cams
}
