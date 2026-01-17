#!/bin/bash

# Build, sign and deploy script for Buffer Sharer
# Builds the app, signs it, and installs to /Applications

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
APP_NAME="Buffer Sharer"
APP_PATH="$PROJECT_DIR/build/bin/$APP_NAME.app"
DEST_PATH="/Applications/$APP_NAME.app"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}  Buffer Sharer - Build & Deploy Script${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Step 1: Build
echo -e "${YELLOW}[1/4]${NC} Building application..."
cd "$PROJECT_DIR"
wails build

if [ ! -d "$APP_PATH" ]; then
    echo -e "${RED}Error: Build failed - app not found at $APP_PATH${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Build complete${NC}"
echo ""

# Step 2: Sign
echo -e "${YELLOW}[2/4]${NC} Signing application..."

# Remove existing signature first
codesign --remove-signature "$APP_PATH" 2>/dev/null || true

# Sign with ad-hoc signature
codesign --deep --force -s - "$APP_PATH"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Signed successfully${NC}"
else
    echo -e "${RED}Error: Signing failed${NC}"
    exit 1
fi
echo ""

# Step 3: Remove old app from /Applications
echo -e "${YELLOW}[3/4]${NC} Removing old version from /Applications..."

if [ -d "$DEST_PATH" ]; then
    # Check if app is running and kill it
    pkill -f "$APP_NAME" 2>/dev/null || true
    sleep 1

    rm -rf "$DEST_PATH"
    echo -e "${GREEN}✓ Old version removed${NC}"
else
    echo -e "${BLUE}  No existing installation found${NC}"
fi
echo ""

# Step 4: Copy to /Applications
echo -e "${YELLOW}[4/4]${NC} Installing to /Applications..."

cp -R "$APP_PATH" "$DEST_PATH"

if [ -d "$DEST_PATH" ]; then
    echo -e "${GREEN}✓ Installed successfully${NC}"
else
    echo -e "${RED}Error: Installation failed${NC}"
    exit 1
fi

echo ""
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}  ✓ All done!${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo -e "  App installed at: ${BLUE}$DEST_PATH${NC}"
echo ""

# Verify signature
echo -e "  Signature info:"
codesign -dv "$DEST_PATH" 2>&1 | grep -E "^(Identifier|Format|CodeDirectory)" | sed 's/^/    /'
echo ""

# Ask to launch
read -p "  Launch app now? [Y/n] " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]] || [[ -z $REPLY ]]; then
    echo -e "  ${BLUE}Launching...${NC}"
    open "$DEST_PATH"
fi

echo ""
echo -e "${YELLOW}Note:${NC} If permissions reset, try:"
echo "  1. Remove from Accessibility & Screen Recording lists"
echo "  2. Run: sudo tccutil reset All app.buffer-sharer.desktop"
echo "  3. Re-run this script and grant permissions again"
echo ""
