#!/usr/bin/env bash
set -euo pipefail

APP_NAME="time-tracker"
BIN_DIR="bin"

mkdir -p "${BIN_DIR}"
echo "Building ${APP_NAME}..."
go build -o "${BIN_DIR}/${APP_NAME}" ./cmd/time-tracker
echo "Successfully built ${BIN_DIR}/${APP_NAME}"

