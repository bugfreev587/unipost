#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUITE="${1:-}"
LOG_DIR="${LOG_DIR:-${ROOT_DIR}/artifacts/regression}"

if [[ -z "$SUITE" ]]; then
  echo "usage: $0 <smoke|sdk-js|sdk-python|sdk-go>" >&2
  exit 64
fi

UNIPOST_API_KEY="${UNIPOST_API_KEY:-}"
BASE_URL="${BASE_URL:-https://api.unipost.dev}"
TEST_ACCOUNT_ID="${TEST_ACCOUNT_ID:-}"
TEST_PUBLISH_NOW="${TEST_PUBLISH_NOW:-false}"

if [[ -z "$UNIPOST_API_KEY" ]]; then
  echo "UNIPOST_API_KEY is required" >&2
  exit 64
fi

mkdir -p "$LOG_DIR"
LOG_FILE="${LOG_DIR}/${SUITE}.log"

run_and_log() {
  echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] starting ${SUITE}" | tee "$LOG_FILE"
  (
    set -euo pipefail
    cd "$ROOT_DIR"
    "$@"
  ) 2>&1 | tee -a "$LOG_FILE"
}

case "$SUITE" in
  smoke)
    run_and_log env \
      API_KEY="$UNIPOST_API_KEY" \
      BASE_URL="$BASE_URL" \
      ACCOUNT_ID="$TEST_ACCOUNT_ID" \
      bash "$ROOT_DIR/scripts/smoke-test.sh"
    ;;
  sdk-js)
    run_and_log bash -lc '
      cd scripts/sdk-validation/js
      npm ci
      UNIPOST_API_KEY="$1" BASE_URL="$2" TEST_ACCOUNT_ID="$3" TEST_PUBLISH_NOW="$4" node unipost-sdk-test.mjs
    ' _ "$UNIPOST_API_KEY" "$BASE_URL" "$TEST_ACCOUNT_ID" "$TEST_PUBLISH_NOW"
    ;;
  sdk-python)
    run_and_log bash -lc '
      cd scripts/sdk-validation/python
      python3 -m pip install --disable-pip-version-check -r requirements.txt
      UNIPOST_API_KEY="$1" BASE_URL="$2" TEST_ACCOUNT_ID="$3" TEST_PUBLISH_NOW="$4" python3 unipost_sdk_test.py
    ' _ "$UNIPOST_API_KEY" "$BASE_URL" "$TEST_ACCOUNT_ID" "$TEST_PUBLISH_NOW"
    ;;
  sdk-go)
    run_and_log bash -lc '
      cd scripts/sdk-validation/go
      GOCACHE="${RUNNER_TEMP:-/tmp}/unipost-go-cache" UNIPOST_API_KEY="$1" BASE_URL="$2" TEST_ACCOUNT_ID="$3" TEST_PUBLISH_NOW="$4" go run main.go
    ' _ "$UNIPOST_API_KEY" "$BASE_URL" "$TEST_ACCOUNT_ID" "$TEST_PUBLISH_NOW"
    ;;
  *)
    echo "unknown suite: $SUITE" >&2
    exit 64
    ;;
esac
