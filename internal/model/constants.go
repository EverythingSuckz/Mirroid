package model

// Device sources reported by adb.
const (
	SourceUSB      = "usb"
	SourceWireless = "wireless"
	SourceMDNS     = "mdns"
)

// Video codecs supported by scrcpy.
const (
	CodecH264 = "h264"
	CodecH265 = "h265"
	CodecAV1  = "av1"
)

// Video sources.
const (
	VideoSourceDisplay = "display"
	VideoSourceCamera  = "camera"
)

// Audio sources.
const (
	AudioSourceOutput   = "output"
	AudioSourceMic      = "mic"
	AudioSourcePlayback = "playback"
)

var (
	Codecs       = []string{CodecH264, CodecH265, CodecAV1}
	VideoSources = []string{VideoSourceDisplay, VideoSourceCamera}
	AudioSources = []string{AudioSourceOutput, AudioSourceMic, AudioSourcePlayback}
)
