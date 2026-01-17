#!/bin/bash
set -e

# Buffer Sharer Middleware Deployment Script
# Usage: ./deploy.sh [port]

PORT=${1:-8080}
BINARY_NAME="buffer-sharer-middleware"
INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/${BINARY_NAME}.service"

echo "╔════════════════════════════════════════╗"
echo "║  Buffer Sharer Middleware Installer    ║"
echo "╚════════════════════════════════════════╝"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root: sudo ./deploy.sh"
    exit 1
fi

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        BINARY_SUFFIX="linux-amd64"
        ;;
    aarch64|arm64)
        BINARY_SUFFIX="linux-arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

# Check if binary exists, otherwise build
if [ -f "${BINARY_NAME}-${BINARY_SUFFIX}" ]; then
    BINARY="${BINARY_NAME}-${BINARY_SUFFIX}"
elif [ -f "${BINARY_NAME}" ]; then
    BINARY="${BINARY_NAME}"
else
    echo "Building from source..."
    if ! command -v go &> /dev/null; then
        echo "Go is not installed. Installing..."
        apt-get update && apt-get install -y golang-go
    fi
    GOOS=linux GOARCH=$(echo $BINARY_SUFFIX | cut -d'-' -f2) go build -o ${BINARY_NAME} .
    BINARY="${BINARY_NAME}"
fi

echo "Installing binary..."
cp "$BINARY" "${INSTALL_DIR}/${BINARY_NAME}"
chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

echo "Creating systemd service..."
cat > "$SERVICE_FILE" << EOF
[Unit]
Description=Buffer Sharer Middleware Server
After=network.target

[Service]
Type=simple
User=nobody
Group=nogroup
ExecStart=${INSTALL_DIR}/${BINARY_NAME} -port ${PORT}
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
PrivateDevices=yes

[Install]
WantedBy=multi-user.target
EOF

echo "Configuring systemd..."
systemctl daemon-reload
systemctl enable ${BINARY_NAME}
systemctl restart ${BINARY_NAME}

# Configure firewall if ufw is installed
if command -v ufw &> /dev/null; then
    echo "Configuring firewall..."
    ufw allow ${PORT}/tcp
fi

echo ""
echo "╔════════════════════════════════════════╗"
echo "║        Installation Complete!          ║"
echo "╠════════════════════════════════════════╣"
echo "║  Middleware running on port ${PORT}         ║"
echo "╠════════════════════════════════════════╣"
echo "║  Commands:                             ║"
echo "║  systemctl status ${BINARY_NAME}  ║"
echo "║  journalctl -u ${BINARY_NAME} -f  ║"
echo "╚════════════════════════════════════════╝"
echo ""

# Show status
systemctl status ${BINARY_NAME} --no-pager || true
