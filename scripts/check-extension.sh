#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOG_DIR="${ROOT_DIR}/tmp/extension-check"
GRPC_ADDR="127.0.0.1:22345"
WEB_ADDR=":22346"
HEADLESS="${EXT_HEADLESS:-false}"

mkdir -p "${LOG_DIR}"

echo "[extension-check] starting extension server (headless: ${HEADLESS})"
(
	cd "${ROOT_DIR}"
	cmd=(./cmd.sh extension --grpc-addr "${GRPC_ADDR}" --web-addr "${WEB_ADDR}")
	if [ "${HEADLESS}" = "true" ]; then
		cmd+=(--headless)
	fi
	"${cmd[@]}" >"${LOG_DIR}/server.log" 2>&1 &
	echo $! > "${LOG_DIR}/server.pid"
)

PID=$(cat "${LOG_DIR}/server.pid")
trap 'kill ${PID} >/dev/null 2>&1 || true' EXIT

sleep 2

if [ "${HEADLESS}" = "true" ]; then
	echo "[extension-check] headless mode enabled; skipping web endpoint check"
else
	curl -k "https://127.0.0.1${WEB_ADDR}/" >/dev/null 2>&1 || {
		cat "${LOG_DIR}/server.log"
		echo "[extension-check] web endpoint is not reachable" >&2
		exit 1
	}
	echo "[extension-check] extension web endpoint reachable at https://127.0.0.1${WEB_ADDR}/"
fi
