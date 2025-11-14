#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
cd "$REPO_ROOT"
GO_VERSION=$(go version)
echo "[check-build] go version: $GO_VERSION"
echo "[check-build] running go test ./..."
go test ./...
