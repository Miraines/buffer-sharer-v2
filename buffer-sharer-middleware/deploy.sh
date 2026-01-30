#!/bin/bash
set -e

# Buffer Sharer Middleware — build, deploy & restart
# Usage: ./deploy.sh

SSH_KEY="$HOME/.ssh/ssh-key-1762630178961"
SSH_USER="maust"
SSH_HOST="158.160.173.98"

REMOTE_DIR="/home/maust"
BINARY_NAME="buffer-sharer-middleware"
PORT=9000
MAIN_GO="main.go"

remote() {
    ssh -i "$SSH_KEY" -l "$SSH_USER" "$SSH_HOST" "$@"
}

echo "╔════════════════════════════════════════╗"
echo "║  Buffer Sharer — Deploy Script         ║"
echo "╚════════════════════════════════════════╝"
echo ""

# --- 1. Bump version ---
CURRENT_VERSION=$(grep -o 'Middleware v[0-9]*\.[0-9]*' "$MAIN_GO" | grep -o '[0-9]*\.[0-9]*')
MAJOR=$(echo "$CURRENT_VERSION" | cut -d. -f1)
MINOR=$(echo "$CURRENT_VERSION" | cut -d. -f2)
NEW_MINOR=$((MINOR + 1))
NEW_VERSION="${MAJOR}.${NEW_MINOR}"

sed -i '' "s/Middleware v${CURRENT_VERSION}/Middleware v${NEW_VERSION}/" "$MAIN_GO"
echo ">> Version: v${CURRENT_VERSION} -> v${NEW_VERSION}"

# --- 2. Build for Linux ---
echo ">> Building for linux/amd64..."
GOOS=linux GOARCH=amd64 go build -o "$BINARY_NAME" .
echo ">> Build OK ($(du -h "$BINARY_NAME" | cut -f1))"

# --- 3. Stop old process on VM ---
echo ">> Stopping old process on VM..."
remote "pkill -f $BINARY_NAME" || true
sleep 1

# --- 4. Upload new binary ---
echo ">> Uploading binary to VM..."
scp -i "$SSH_KEY" "$BINARY_NAME" "${SSH_USER}@${SSH_HOST}:${REMOTE_DIR}/${BINARY_NAME}"

# --- 5. Start on VM ---
echo ">> Starting on VM (port $PORT)..."
ssh -i "$SSH_KEY" -l "$SSH_USER" "$SSH_HOST" -f "chmod +x ${REMOTE_DIR}/${BINARY_NAME} && ${REMOTE_DIR}/${BINARY_NAME} -port ${PORT} > ${REMOTE_DIR}/${BINARY_NAME}.log 2>&1"
sleep 1

# --- 6. Verify ---
echo ">> Checking process..."
remote "pgrep -f $BINARY_NAME" && echo ">> OK, running!" || echo ">> FAILED to start"

# --- Cleanup local binary ---
rm -f "$BINARY_NAME"

echo ""
echo "╔════════════════════════════════════════╗"
echo "║  Deployed v${NEW_VERSION} to ${SSH_HOST}        ║"
echo "║  Port: ${PORT}                              ║"
echo "║                                        ║"
echo "║  Logs: ssh ... tail -f ${BINARY_NAME}.log ║"
echo "╚════════════════════════════════════════╝"
