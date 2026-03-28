#!/bin/bash
set -euo pipefail

# Build a DMG installer for Mirroid
# Usage: build-dmg.sh <version> [app-bundle-path]
#   version:         e.g. "v0.1.0"
#   app-bundle-path: path to Mirroid.app (default: "Mirroid.app")

VERSION="${1:?Usage: build-dmg.sh <version> [app-bundle-path]}"
APP_BUNDLE="${2:-Mirroid.app}"

if [ ! -d "$APP_BUNDLE" ]; then
    echo "Error: App bundle not found at '$APP_BUNDLE'"
    exit 1
fi

OUTPUT="mirroid-macos-arm64.dmg"

# Remove previous DMG if it exists (create-dmg fails if output already exists)
rm -f "$OUTPUT"

create-dmg \
    --volname "Mirroid ${VERSION}" \
    --window-pos 200 120 \
    --window-size 600 400 \
    --icon-size 100 \
    --icon "Mirroid.app" 150 185 \
    --app-drop-link 450 185 \
    --hide-extension "Mirroid.app" \
    --no-internet-enable \
    "$OUTPUT" \
    "$APP_BUNDLE"

echo "Built: $OUTPUT"
