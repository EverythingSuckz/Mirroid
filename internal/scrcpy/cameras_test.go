package scrcpy

import "testing"

func TestParseListCameras(t *testing.T) {
	out := []byte(`INFO: List of cameras:
    --camera-id=0  (back, 4032x3024)
        sensor orientation: 90°
    --camera-id=1  (front, 3264x2448)
    --camera-id=2  (external, 1920x1080)
`)
	cams := parseListCameras(out)
	if len(cams) != 3 {
		t.Fatalf("expected 3 cameras, got %d", len(cams))
	}
	want := []CameraInfo{
		{ID: "0", Facing: "back", Size: "4032x3024"},
		{ID: "1", Facing: "front", Size: "3264x2448"},
		{ID: "2", Facing: "external", Size: "1920x1080"},
	}
	for i, w := range want {
		if cams[i] != w {
			t.Errorf("cam %d: got %+v, want %+v", i, cams[i], w)
		}
	}
}

func TestParseListCameras_NoSize(t *testing.T) {
	out := []byte(`    --camera-id=0  (back)`)
	cams := parseListCameras(out)
	if len(cams) != 1 {
		t.Fatalf("expected 1 camera, got %d", len(cams))
	}
	if cams[0] != (CameraInfo{ID: "0", Facing: "back", Size: ""}) {
		t.Errorf("got %+v", cams[0])
	}
}
