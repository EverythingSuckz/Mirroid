> [!WARNING]
> This project is under development and is not yet ready for use. Expect frequent breaking changes until the `1.0.0` release.

<p align="center">
  <img src="assets/icon.png" alt="Mirroid" width="128" />
</p>

<h1 align="center">Mirroid</h1>

<p align="center">
  A full-featured desktop GUI for <a href="https://github.com/Genymobile/scrcpy">scrcpy</a>, built with Go and <a href="https://fyne.io/">Fyne</a>.
</p>

---
<br>
<p align="center">
  <img src="assets/screenshot.png" alt="Mirroid Screenshot" width="80%" />
</p>

## Features

- **Easy Pairing** : pair devices via USB or wireless QR code with a single click
- **Full scrcpy Options** : bitrate, max size, FPS, codec (h264/h265/av1), audio, window flags, HID input, recording
- **Presets** : save and load option configurations as JSON
- **Multi-device** : launch scrcpy on multiple devices simultaneously
- **Cross-platform** : works on Windows, macOS, and Linux (only tested on Windows so far)
- **Single Executable** : no dependencies, just one binary to run

## Why make this?

Tired of typing out long scrcpy commands in the terminal. Wanted a simple way to manage multiple devices and configurations without memorizing flags.

## Requirements

| Dependency | Notes |
|---|---|
| **adb** | Android Debug Bridge, must be in `PATH` |
| **scrcpy** | Must be installed and in `PATH` |
| **Go 1.22+** | For building from source (Optional) |


## Installation

Currently, there are no pre-built binaries available. Stay tuned for future releases in the [Releases](https://github.com/EverythingSucks/Mirroid/releases) section.

## Building

#### Debug build:
```bash
go build -o mirroid.exe .
```

#### Release build with embedded assets:
```bash
fyne package -release
```

## License

MIT
