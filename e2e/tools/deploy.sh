#!/usr/bin/env bash
set -euo pipefail

REMOTE_USER="root"
REMOTE_HOST="prod-micro-tools"
REMOTE_DIR="/root"
SERVICE_NAME="qivitals-server"

echo "Building..."
CGO_ENABLED=0 go build -o qivitals-server ./cmd/qivitals-server

echo "Copying to ${REMOTE_HOST}:${REMOTE_DIR}/"
scp qivitals-server "${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_DIR}/"

echo "Stopping ${SERVICE_NAME}..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "systemctl stop ${SERVICE_NAME}"

echo "Installing binary..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "cp ${REMOTE_DIR}/${SERVICE_NAME} /usr/local/bin/${SERVICE_NAME} && chmod +x /usr/local/bin/${SERVICE_NAME}"

echo "Starting ${SERVICE_NAME}..."
ssh "${REMOTE_USER}@${REMOTE_HOST}" "systemctl start ${SERVICE_NAME}"

echo "Done."