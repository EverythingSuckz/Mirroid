#!/bin/bash
set -euo pipefail

# Build an AppImage for Mirroid
# Usage: build-appimage.sh <version> [binary-path]
#   version:     e.g. "v0.1.0"
#   binary-path: path to the mirroid binary (default: "Mirroid")

VERSION="${1:?Usage: build-appimage.sh <version> [binary-path]}"
BINARY="${2:-Mirroid}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Clean previous build
rm -rf AppDir

# Create AppDir structure
mkdir -p AppDir/usr/bin
mkdir -p AppDir/usr/share/applications
mkdir -p AppDir/usr/share/icons/hicolor/256x256/apps

# Copy binary
cp "$BINARY" AppDir/usr/bin/mirroid
chmod +x AppDir/usr/bin/mirroid

# Copy desktop file and icon
cp "$SCRIPT_DIR/mirroid.desktop" AppDir/usr/share/applications/mirroid.desktop
cp "$REPO_ROOT/assets/icon.png" AppDir/usr/share/icons/hicolor/256x256/apps/mirroid.png

# Also place icon at AppDir root (required by some AppImage tools)
cp "$REPO_ROOT/assets/icon.png" AppDir/mirroid.png

# Also place desktop file at AppDir root
cp "$SCRIPT_DIR/mirroid.desktop" AppDir/mirroid.desktop

# Copy bundled dependencies
BUNDLED_DIR="${3:-$REPO_ROOT/_bundled}"
if [ -d "$BUNDLED_DIR" ]; then
    mkdir -p AppDir/usr/lib/mirroid
    cp "$BUNDLED_DIR"/* AppDir/usr/lib/mirroid/
    chmod +x AppDir/usr/lib/mirroid/adb AppDir/usr/lib/mirroid/scrcpy 2>/dev/null || true
fi

# Download linuxdeploy if not present
if [ ! -f linuxdeploy-x86_64.AppImage ]; then
    echo "Downloading linuxdeploy..."
    wget -q "https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage"
    chmod +x linuxdeploy-x86_64.AppImage
fi

# Build the AppImage
export VERSION
./linuxdeploy-x86_64.AppImage \
    --appdir AppDir \
    --desktop-file "$SCRIPT_DIR/mirroid.desktop" \
    --icon-file "$REPO_ROOT/assets/icon.png" \
    --output appimage

# The output filename follows the pattern: Mirroid-VERSION-x86_64.AppImage
echo "Built: Mirroid-${VERSION}-x86_64.AppImage"
