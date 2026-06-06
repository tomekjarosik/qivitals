#!/usr/bin/env bash
set -euo pipefail

REMOTE_USER="root"
REMOTE_HOST="prod-micro-tools"
REMOTE_DIR="/root"
SERVICE_NAME="qivitals-server"
CONTROL_PATH="/tmp/ssh-control-$(date +%s)"

# 1. Build
echo "Building..."
CGO_ENABLED=0 go build -o qivitals-server ./cmd/qivitals-server

# 2. Establish master SSH connection (prompts for Yubikey once)
echo "Establishing SSH connection..."
# Use -M (Master), -f (background), and -N (no command) to hold the connection open
ssh -M -f -N -o ControlPath="${CONTROL_PATH}" -o StrictHostKeyChecking=no "${REMOTE_USER}@${REMOTE_HOST}"

# 3. Copy to remote (uses cached connection)
echo "Copying to ${REMOTE_HOST}:${REMOTE_DIR}/..."
scp -o ControlPath="${CONTROL_PATH}" qivitals-server "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR}/"

# 4. Stop service
echo "Stopping ${SERVICE_NAME}..."
ssh -o ControlPath="${CONTROL_PATH}" "${REMOTE_USER}@${REMOTE_HOST}" "systemctl stop ${SERVICE_NAME}"

# 5. Install binary
echo "Installing binary..."
ssh -o ControlPath="${CONTROL_PATH}" "${REMOTE_USER}@${REMOTE_HOST}" "cp ${REMOTE_DIR}/${SERVICE_NAME} /usr/local/bin/${SERVICE_NAME} && chmod +x /usr/local/bin/${SERVICE_NAME}"

# 6. Start service
echo "Starting ${SERVICE_NAME}..."
ssh -o ControlPath="${CONTROL_PATH}" "${REMOTE_USER}@${REMOTE_HOST}" "systemctl start ${SERVICE_NAME}"

# 7. Clean up master connection
echo "Cleaning up..."
# Use -O exit to tell the background master process to terminate cleanly
ssh -O exit -o ControlPath="${CONTROL_PATH}" "${REMOTE_USER}@${REMOTE_HOST}" 2>/dev/null

echo "Done."