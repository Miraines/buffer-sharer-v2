#!/bin/bash

# Ad-hoc code signing script for Buffer Sharer
# This helps macOS remember permissions between rebuilds

APP_PATH="build/bin/Buffer Sharer.app"

if [ ! -d "$APP_PATH" ]; then
    echo "Error: App not found at $APP_PATH"
    echo "Run 'wails build' first"
    exit 1
fi

echo "Signing Buffer Sharer with ad-hoc signature..."

# Remove existing signature first
codesign --remove-signature "$APP_PATH" 2>/dev/null

# Sign with ad-hoc signature (no Apple Developer account needed)
# --deep signs all nested code (frameworks, helpers)
# --force replaces any existing signature
# -s - means ad-hoc (no identity)
codesign --deep --force -s - "$APP_PATH"

if [ $? -eq 0 ]; then
    echo "Successfully signed!"
    echo ""
    echo "Verifying signature..."
    codesign -dv "$APP_PATH"
    echo ""
    echo "IMPORTANT: After giving permissions in System Settings,"
    echo "you may need to RESTART the app for changes to take effect."
    echo ""
    echo "If permissions keep resetting, try:"
    echo "  1. Remove Buffer Sharer from Accessibility & Screen Recording lists"
    echo "  2. Run: sudo tccutil reset All app.buffer-sharer.desktop"
    echo "  3. Rebuild and re-sign the app"
    echo "  4. Add permissions again"
else
    echo "Signing failed!"
    exit 1
fi
