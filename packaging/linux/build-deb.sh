#!/bin/bash
set -euo pipefail

# Build a .deb package for Mirroid
# Usage: build-deb.sh <version> [binary-path]
#   version:     e.g. "v0.1.0" (the "v" prefix is stripped automatically)
#   binary-path: path to the mirroid binary (default: "Mirroid")

VERSION="${1:?Usage: build-deb.sh <version> [binary-path]}"
BINARY="${2:-Mirroid}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Strip leading 'v' from version for Debian conventions
DEB_VERSION="${VERSION#v}"

DEB_ROOT="deb-root"
rm -rf "$DEB_ROOT"

# Create directory structure
mkdir -p "$DEB_ROOT/DEBIAN"
mkdir -p "$DEB_ROOT/usr/bin"
mkdir -p "$DEB_ROOT/usr/share/applications"
mkdir -p "$DEB_ROOT/usr/share/icons/hicolor/256x256/apps"

# Copy binary
cp "$BINARY" "$DEB_ROOT/usr/bin/mirroid"
chmod 755 "$DEB_ROOT/usr/bin/mirroid"

# Copy desktop entry
cp "$SCRIPT_DIR/mirroid.desktop" "$DEB_ROOT/usr/share/applications/mirroid.desktop"

# Copy icon
cp "$REPO_ROOT/assets/icon.png" "$DEB_ROOT/usr/share/icons/hicolor/256x256/apps/mirroid.png"

# Write control file
cat > "$DEB_ROOT/DEBIAN/control" <<EOF
Package: mirroid
Version: ${DEB_VERSION}
Architecture: amd64
Maintainer: EverythingSuckz <noreply@github.com>
Depends: libc6, libgl1
Recommends: adb, scrcpy
Section: utils
Priority: optional
Homepage: https://github.com/EverythingSuckz/Mirroid
Description: Desktop GUI for scrcpy
 A full-featured desktop GUI for scrcpy, built with Go and Fyne.
 Provides easy pairing, full scrcpy options, presets, and multi-device
 support for Android screen mirroring.
EOF

# Set correct permissions for control directory
chmod 755 "$DEB_ROOT/DEBIAN"

# Build the .deb
OUTPUT="mirroid_${DEB_VERSION}_amd64.deb"
dpkg-deb --build --root-owner-group "$DEB_ROOT" "$OUTPUT"

echo "Built: $OUTPUT"
