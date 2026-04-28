#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUITE="${1:-}"
LOG_DIR="${LOG_DIR:-${ROOT_DIR}/artifacts/sdk-source-validation}"
UNIPOST_DEV_ROOT="${UNIPOST_DEV_ROOT:-/Users/xiaoboyu/unipost-dev}"

if [[ -z "$SUITE" ]]; then
  echo "usage: $0 <sdk-js|sdk-python|sdk-go>" >&2
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
  sdk-js)
    run_and_log bash -lc '
      cd scripts/sdk-validation/js
      npm ci
      UNIPOST_JS_SDK_IMPORT="$1/dist/index.mjs" UNIPOST_API_KEY="$2" BASE_URL="$3" TEST_ACCOUNT_ID="$4" TEST_PUBLISH_NOW="$5" node unipost-sdk-test.mjs
    ' _ "${UNIPOST_DEV_ROOT}/sdk-js" "$UNIPOST_API_KEY" "$BASE_URL" "$TEST_ACCOUNT_ID" "$TEST_PUBLISH_NOW"
    ;;
  sdk-python)
    run_and_log bash -lc '
      cd scripts/sdk-validation/python
      python3 -m pip install --disable-pip-version-check -r requirements.txt
      UNIPOST_PYTHON_SDK_PATH="$1" UNIPOST_API_KEY="$2" BASE_URL="$3" TEST_ACCOUNT_ID="$4" TEST_PUBLISH_NOW="$5" python3 unipost_sdk_test.py
    ' _ "${UNIPOST_DEV_ROOT}/sdk-python" "$UNIPOST_API_KEY" "$BASE_URL" "$TEST_ACCOUNT_ID" "$TEST_PUBLISH_NOW"
    ;;
  sdk-go)
    run_and_log bash -lc '
      TMP_DIR="${RUNNER_TEMP:-/tmp}/unipost-go-source-validation"
      rm -rf "$TMP_DIR"
      mkdir -p "$TMP_DIR"
      cp scripts/sdk-validation/go/main.go "$TMP_DIR/main.go"
      cat >"$TMP_DIR/go.mod" <<EOF
module unipost-sdk-source-validation

go 1.21

require github.com/unipost-dev/sdk-go v0.2.4

replace github.com/unipost-dev/sdk-go => $1/sdk-go
EOF
      cd "$TMP_DIR"
      GOCACHE="${RUNNER_TEMP:-/tmp}/unipost-go-cache" UNIPOST_API_KEY="$2" BASE_URL="$3" TEST_ACCOUNT_ID="$4" TEST_PUBLISH_NOW="$5" go run main.go
    ' _ "$UNIPOST_DEV_ROOT" "$UNIPOST_API_KEY" "$BASE_URL" "$TEST_ACCOUNT_ID" "$TEST_PUBLISH_NOW"
    ;;
  *)
    echo "unknown suite: $SUITE" >&2
    exit 64
    ;;
esac
