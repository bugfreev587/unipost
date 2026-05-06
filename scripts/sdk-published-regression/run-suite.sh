#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUITE="${1:-}"
LOG_DIR="${LOG_DIR:-${ROOT_DIR}/artifacts/sdk-published-regression}"

if [[ -z "$SUITE" ]]; then
  echo "usage: $0 <sdk-js|sdk-python|sdk-go|sdk-java>" >&2
  exit 64
fi

UNIPOST_API_KEY="${UNIPOST_API_KEY:-}"
BASE_URL="${BASE_URL:-https://api.unipost.dev}"
TEST_ACCOUNT_ID="${TEST_ACCOUNT_ID:-}"
TEST_PUBLISH_NOW="${TEST_PUBLISH_NOW:-false}"
JS_SDK_SPEC="${JS_SDK_SPEC:-@unipost/sdk@latest}"
PYTHON_SDK_SPEC="${PYTHON_SDK_SPEC:-unipost}"
GO_SDK_SPEC="${GO_SDK_SPEC:-github.com/unipost-dev/sdk-go@latest}"
JAVA_SDK_VERSION="${JAVA_SDK_VERSION:-0.2.5}"

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
      npm install --silent --no-save --package-lock=false "$1"
      UNIPOST_JS_SDK_IMPORT="@unipost/sdk" UNIPOST_API_KEY="$2" BASE_URL="$3" TEST_ACCOUNT_ID="$4" TEST_PUBLISH_NOW="$5" node unipost-sdk-test.mjs
    ' _ "$JS_SDK_SPEC" "$UNIPOST_API_KEY" "$BASE_URL" "$TEST_ACCOUNT_ID" "$TEST_PUBLISH_NOW"
    ;;
  sdk-python)
    run_and_log bash -lc '
      cd scripts/sdk-validation/python
      python3 -m pip install --disable-pip-version-check -r requirements.txt
      python3 -m pip install --disable-pip-version-check "$1"
      UNIPOST_PYTHON_SDK_PATH="" UNIPOST_API_KEY="$2" BASE_URL="$3" TEST_ACCOUNT_ID="$4" TEST_PUBLISH_NOW="$5" python3 unipost_sdk_test.py
    ' _ "$PYTHON_SDK_SPEC" "$UNIPOST_API_KEY" "$BASE_URL" "$TEST_ACCOUNT_ID" "$TEST_PUBLISH_NOW"
    ;;
  sdk-go)
    run_and_log bash -lc '
      TMP_DIR="${RUNNER_TEMP:-/tmp}/unipost-go-published-regression"
      rm -rf "$TMP_DIR"
      mkdir -p "$TMP_DIR"
      cp scripts/sdk-validation/go/main.go "$TMP_DIR/main.go"
      cat >"$TMP_DIR/go.mod" <<EOF
module unipost-sdk-published-regression

go 1.21
EOF
      cd "$TMP_DIR"
      go get "$1"
      GOCACHE="${RUNNER_TEMP:-/tmp}/unipost-go-cache" UNIPOST_API_KEY="$2" BASE_URL="$3" TEST_ACCOUNT_ID="$4" TEST_PUBLISH_NOW="$5" go run main.go
    ' _ "$GO_SDK_SPEC" "$UNIPOST_API_KEY" "$BASE_URL" "$TEST_ACCOUNT_ID" "$TEST_PUBLISH_NOW"
    ;;
  sdk-java)
    run_and_log bash -lc '
      cd scripts/sdk-validation/java
      UNIPOST_API_KEY="$2" BASE_URL="$3" TEST_ACCOUNT_ID="$4" TEST_PUBLISH_NOW="$5" ./gradlew run -PunipostJavaSdkVersion="$1"
    ' _ "$JAVA_SDK_VERSION" "$UNIPOST_API_KEY" "$BASE_URL" "$TEST_ACCOUNT_ID" "$TEST_PUBLISH_NOW"
    ;;
  *)
    echo "unknown suite: $SUITE" >&2
    exit 64
    ;;
esac
