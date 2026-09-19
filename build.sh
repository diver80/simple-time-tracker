#!/usr/bin/env bash
set -euo pipefail

APP_NAME="yokto-time"
BIN_DIR="bin"

mkdir -p "${BIN_DIR}"
echo "Building ${APP_NAME}..."
go build -o "${BIN_DIR}/${APP_NAME}" ./cmd/yokto-time
echo "Successfully built ${BIN_DIR}/${APP_NAME}"
