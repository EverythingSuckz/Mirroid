package model

import (
	"encoding/json"
	"errors"
	"strconv"
)

// ScrcpyOptions holds all configurable scrcpy options.
// this is the single source of truth which is used by the UI, presets, and command builder.
type ScrcpyOptions struct {
	Bitrate       int    `json:"bitrate"`  // Mbps
	MaxSize       int    `json:"max_size"` // pixels (0 = unlimited)
	MaxFPS        int    `json:"max_fps"`  // 0 = unlimited
	Codec         string `json:"codec"`    // h264, h265, av1
	AudioEnabled  bool   `json:"audio_enabled"`
	AudioSource   string `json:"audio_source"` // output, mic, playback
	Fullscreen    bool   `json:"fullscreen"`
	Borderless    bool   `json:"borderless"`
	AlwaysOnTop   bool   `json:"always_on_top"`
	Rotation      int    `json:"rotation"` // 0-3
	TurnScreenOff bool   `json:"turn_screen_off"`
	RecordFile    string `json:"record_file"`
	WindowTitle   string `json:"window_title"`
	ClipboardSync bool   `json:"clipboard_sync"`
	StayAwake     bool   `json:"stay_awake"`
	HIDKeyboard   bool   `json:"hid_keyboard"`
	HIDMouse      bool   `json:"hid_mouse"`
	VideoSource   string `json:"video_source"`  // display, camera
	CameraFacing  string `json:"camera_facing"` // back, front, external (only used when VideoSource is camera)
}

// DefaultOptions returns a ScrcpyOptions with sensible defaults.
func DefaultOptions() ScrcpyOptions {
	return ScrcpyOptions{
		Bitrate:       8,
		Codec:         CodecH264,
		AudioEnabled:  false,
		AudioSource:   AudioSourceOutput,
		ClipboardSync: true,
		VideoSource:   VideoSourceDisplay,
		CameraFacing:  CameraFacingBack,
	}
}

// Validate checks all fields for correctness and returns a descriptive error.
func (o *ScrcpyOptions) Validate() error {
	// 200 Mbps ought to be enough for anybody
	if o.Bitrate < 0 || o.Bitrate > 200 {
		return errors.New("bitrate must be between 0 and 200 Mbps")
	}
	if o.MaxSize < 0 {
		return errors.New("max size must be non-negative")
	}
	if o.MaxFPS < 0 || o.MaxFPS > 240 {
		return errors.New("max FPS must be between 0 and 240")
	}
	validCodecs := map[string]bool{CodecH264: true, CodecH265: true, CodecAV1: true}
	if !validCodecs[o.Codec] {
		return errors.New("codec must be one of: h264, h265, av1")
	}
	validAudioSources := map[string]bool{AudioSourceOutput: true, AudioSourceMic: true, AudioSourcePlayback: true}
	if !validAudioSources[o.AudioSource] {
		return errors.New("audio source must be one of: output, mic, playback")
	}
	if o.Rotation < 0 || o.Rotation > 3 {
		return errors.New("rotation must be between 0 and 3")
	}
	validVideoSources := map[string]bool{VideoSourceDisplay: true, VideoSourceCamera: true}
	if !validVideoSources[o.VideoSource] {
		return errors.New("video source must be one of: display, camera")
	}
	if o.VideoSource == VideoSourceCamera {
		validFacings := map[string]bool{CameraFacingBack: true, CameraFacingFront: true, CameraFacingExternal: true}
		if !validFacings[o.CameraFacing] {
			return errors.New("camera facing must be one of: back, front, external")
		}
	}
	return nil
}

// ToJSON serializes the options to a JSON byte slice.
func (o *ScrcpyOptions) ToJSON() ([]byte, error) {
	return json.MarshalIndent(o, "", "    ")
}

// FromJSON deserializes options from a JSON byte slice.
func FromJSON(data []byte) (ScrcpyOptions, error) {
	opts := DefaultOptions()
	if err := json.Unmarshal(data, &opts); err != nil {
		return ScrcpyOptions{}, err
	}
	return opts, nil
}

// BuildCommand builds the scrcpy command-line arguments for a given device serial.
func (o *ScrcpyOptions) BuildCommand(scrcpyPath, deviceSerial string) []string {
	if scrcpyPath == "" {
		scrcpyPath = "scrcpy"
	}

	cmd := []string{scrcpyPath}
	if deviceSerial != "" {
		cmd = append(cmd, "-s", deviceSerial)
	}

	if o.Bitrate > 0 {
		cmd = append(cmd, "-b", strconv.Itoa(o.Bitrate*1_000_000))
	}
	if o.MaxSize > 0 {
		cmd = append(cmd, "--max-size", strconv.Itoa(o.MaxSize))
	}
	if o.MaxFPS > 0 {
		cmd = append(cmd, "--max-fps", strconv.Itoa(o.MaxFPS))
	}
	if o.Codec != "" {
		cmd = append(cmd, "--video-codec", o.Codec)
	}
	if !o.AudioEnabled {
		cmd = append(cmd, "--no-audio")
	}
	if o.AudioSource != "" && o.AudioSource != AudioSourceOutput {
		cmd = append(cmd, "--audio-source", o.AudioSource)
	}
	if o.Fullscreen {
		cmd = append(cmd, "-f")
	}
	if o.Borderless {
		cmd = append(cmd, "--window-borderless")
	}
	if o.AlwaysOnTop {
		cmd = append(cmd, "--always-on-top")
	}
	if o.Rotation > 0 {
		cmd = append(cmd, "--orientation", strconv.Itoa(o.Rotation))
	}
	if o.TurnScreenOff {
		cmd = append(cmd, "-S")
	}
	if o.RecordFile != "" {
		cmd = append(cmd, "-r", o.RecordFile)
	}
	if o.WindowTitle != "" {
		cmd = append(cmd, "--window-title", o.WindowTitle)
	}
	if !o.ClipboardSync {
		cmd = append(cmd, "--no-clipboard-autosync")
	}
	if o.StayAwake {
		cmd = append(cmd, "-w")
	}
	if o.HIDKeyboard {
		cmd = append(cmd, "-K")
	}
	if o.HIDMouse {
		cmd = append(cmd, "-M")
	}
	if o.VideoSource == VideoSourceCamera {
		cmd = append(cmd, "--video-source", VideoSourceCamera)
		if o.CameraFacing != "" {
			cmd = append(cmd, "--camera-facing", o.CameraFacing)
		}
	}

	return cmd
}
