#!/usr/bin/env bash
set -euo pipefail

# install-systemd.sh — Install file-uploader as a systemd service.
# Must be run as root.

if [[ "$(id -u)" -ne 0 ]]; then
    echo "ERROR: This script must be run as root" >&2
    exit 1
fi

INSTALL_DIR="/opt/file-uploader"
SERVICE_USER="file-uploader"
SERVICE_FILE="file-uploader.service"

# 1. Create system user (no login shell, no home directory)
if ! id -u "$SERVICE_USER" &>/dev/null; then
    echo "==> Creating system user '$SERVICE_USER'"
    useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER"
fi

# 2. Create directory structure
echo "==> Creating directory structure under $INSTALL_DIR"
mkdir -p "$INSTALL_DIR/data/players-db/work"
mkdir -p "$INSTALL_DIR/data/data-processing/upload"
mkdir -p "$INSTALL_DIR/data/data-processing/processing"
mkdir -p "$INSTALL_DIR/data/data-processing/uploading"
mkdir -p "$INSTALL_DIR/data/data-processing/archive"

# 3. Copy binary and config
echo "==> Copying binary and config"
if [[ -f "file-uploader" ]]; then
    cp file-uploader "$INSTALL_DIR/file-uploader"
    chmod +x "$INSTALL_DIR/file-uploader"
else
    echo "ERROR: file-uploader binary not found in current directory" >&2
    exit 1
fi

if [[ -f "file-uploader.toml" ]]; then
    # Only copy config if it doesn't already exist (don't overwrite user config)
    if [[ ! -f "$INSTALL_DIR/file-uploader.toml" ]]; then
        cp file-uploader.toml "$INSTALL_DIR/file-uploader.toml"
    else
        echo "    Config already exists at $INSTALL_DIR/file-uploader.toml — skipping"
    fi
fi

# 4. Set ownership
echo "==> Setting ownership to $SERVICE_USER"
chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"

# 5. Install systemd unit file
echo "==> Installing systemd unit file"
cp "$SERVICE_FILE" "/etc/systemd/system/$SERVICE_FILE"

# 6. Reload systemd daemon
echo "==> Reloading systemd daemon"
systemctl daemon-reload

echo ""
echo "Installation complete. To start the service:"
echo "  systemctl start file-uploader"
echo "  systemctl enable file-uploader   # auto-start on boot"
echo ""
echo "To check status:"
echo "  systemctl status file-uploader"
echo "  journalctl -u file-uploader -f"
