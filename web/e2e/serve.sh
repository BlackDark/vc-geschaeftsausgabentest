#!/bin/sh
# Starts a fresh server for end-to-end tests with an empty data directory.
# SPESEN_BIN may point to a prebuilt binary; otherwise `go run` is used.
set -eu
port="$1"
data="$(mktemp -d)"
export SPESEN_DATA_DIR="$data" SPESEN_LISTEN="127.0.0.1:$port" SPESEN_BASE_URL="http://127.0.0.1:$port" SPESEN_SETUP_TOKEN="e2e-setup-token"
if [ -n "${SPESEN_BIN:-}" ]; then
  exec "$SPESEN_BIN"
fi
cd "$(dirname "$0")/../.."
exec go run ./cmd/spesen
