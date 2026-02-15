#!/usr/bin/env bash
set -euo pipefail

PORT="${CDP_PORT:-9224}"
PROFILE_DIR="${CDP_PROFILE_DIR:-/home/ubuntu/.config/cli-365/profile-headless-2}"
LOG_FILE="${CDP_LOG_FILE:-/tmp/cli365-headless-${PORT}.log}"
URL="${CDP_URL:-https://login.microsoftonline.com/}"
CHROME_BIN="${CDP_CHROME_BIN:-/home/ubuntu/.cache/rod/browser/chromium-1321438/chrome}"

log() {
  echo "[headless-cdp] $*" >> "${LOG_FILE}"
}

start_chrome() {
  log "start $(date -Iseconds) port=${PORT} profile=${PROFILE_DIR} url=${URL}"
  "${CHROME_BIN}" \
    --headless=new \
    --remote-debugging-port="${PORT}" \
    --remote-debugging-address=127.0.0.1 \
    --user-data-dir="${PROFILE_DIR}" \
    --no-sandbox \
    --disable-gpu \
    --disable-dev-shm-usage \
    --no-first-run \
    --no-default-browser-check \
    --noerrdialogs \
    --ozone-platform=headless \
    --ozone-override-screen-size=800,600 \
    --use-angle=swiftshader-webgl \
    --enable-logging=stderr \
    --v=1 \
    about:blank >> "${LOG_FILE}" 2>&1
  code=$?
  log "exit $(date -Iseconds) code=${code}"
}

ensure_blank_tab() {
  if ! command -v curl >/dev/null 2>&1; then
    return
  fi
  for _ in {1..20}; do
    if curl -s "http://127.0.0.1:${PORT}/json/list" | grep -q "about:blank"; then
      return
    fi
    sleep 0.5
  done
  curl -s -X PUT "http://127.0.0.1:${PORT}/json/new?about:blank" >/dev/null 2>&1 || true
}

open_login_tab() {
  if ! command -v curl >/dev/null 2>&1; then
    return
  fi
  curl -s -X PUT "http://127.0.0.1:${PORT}/json/new?${URL}" >/dev/null 2>&1 || true
}

{
  start_chrome &
  chrome_pid=$!
  for _ in {1..40}; do
    if curl -s "http://127.0.0.1:${PORT}/json/version" >/dev/null 2>&1; then
      break
    fi
    sleep 0.25
  done
  ensure_blank_tab
  open_login_tab
  wait ${chrome_pid}
}
