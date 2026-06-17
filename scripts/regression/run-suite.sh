#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUITE="${1:-}"
LOG_DIR="${LOG_DIR:-${ROOT_DIR}/artifacts/regression}"

if [[ -z "$SUITE" ]]; then
  echo "usage: $0 <smoke|sdk-js|sdk-python|sdk-go|sdk-java|ai-provider>" >&2
  exit 64
fi

UNIPOST_API_KEY="${UNIPOST_API_KEY:-}"
BASE_URL="${BASE_URL:-https://api.unipost.dev}"
TEST_ACCOUNT_ID="${TEST_ACCOUNT_ID:-}"
TEST_PUBLISH_NOW="${TEST_PUBLISH_NOW:-false}"
TOKENGATE_REGRESSION_API_KEY="${TOKENGATE_REGRESSION_API_KEY:-}"
TOKENGATE_REGRESSION_BASE_URL="${TOKENGATE_REGRESSION_BASE_URL:-}"
TOKENGATE_REGRESSION_EXPECTED_MODELS="${TOKENGATE_REGRESSION_EXPECTED_MODELS:-}"
TOKENGATE_REGRESSION_CHAT_MODEL="${TOKENGATE_REGRESSION_CHAT_MODEL:-}"
AI_PROVIDER_MONITOR_CHAT="${AI_PROVIDER_MONITOR_CHAT:-true}"

if [[ "$SUITE" != "ai-provider" && -z "$UNIPOST_API_KEY" ]]; then
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
    run_and_log env \
      UNIPOST_API_KEY="$UNIPOST_API_KEY" \
      BASE_URL="$BASE_URL" \
      TEST_ACCOUNT_ID="$TEST_ACCOUNT_ID" \
      TEST_PUBLISH_NOW="$TEST_PUBLISH_NOW" \
      LOG_DIR="$LOG_DIR" \
      bash "$ROOT_DIR/scripts/sdk-published-regression/run-suite.sh" sdk-js
    ;;
  sdk-python)
    run_and_log env \
      UNIPOST_API_KEY="$UNIPOST_API_KEY" \
      BASE_URL="$BASE_URL" \
      TEST_ACCOUNT_ID="$TEST_ACCOUNT_ID" \
      TEST_PUBLISH_NOW="$TEST_PUBLISH_NOW" \
      LOG_DIR="$LOG_DIR" \
      bash "$ROOT_DIR/scripts/sdk-published-regression/run-suite.sh" sdk-python
    ;;
  sdk-go)
    run_and_log env \
      UNIPOST_API_KEY="$UNIPOST_API_KEY" \
      BASE_URL="$BASE_URL" \
      TEST_ACCOUNT_ID="$TEST_ACCOUNT_ID" \
      TEST_PUBLISH_NOW="$TEST_PUBLISH_NOW" \
      LOG_DIR="$LOG_DIR" \
      bash "$ROOT_DIR/scripts/sdk-published-regression/run-suite.sh" sdk-go
    ;;
  sdk-java)
    run_and_log env \
      UNIPOST_API_KEY="$UNIPOST_API_KEY" \
      BASE_URL="$BASE_URL" \
      TEST_ACCOUNT_ID="$TEST_ACCOUNT_ID" \
      TEST_PUBLISH_NOW="$TEST_PUBLISH_NOW" \
      LOG_DIR="$LOG_DIR" \
      bash "$ROOT_DIR/scripts/sdk-published-regression/run-suite.sh" sdk-java
    ;;
  ai-provider)
    run_and_log env \
      TOKENGATE_REGRESSION_API_KEY="$TOKENGATE_REGRESSION_API_KEY" \
      TOKENGATE_REGRESSION_BASE_URL="$TOKENGATE_REGRESSION_BASE_URL" \
      TOKENGATE_REGRESSION_EXPECTED_MODELS="$TOKENGATE_REGRESSION_EXPECTED_MODELS" \
      TOKENGATE_REGRESSION_CHAT_MODEL="$TOKENGATE_REGRESSION_CHAT_MODEL" \
      AI_PROVIDER_MONITOR_CHAT="$AI_PROVIDER_MONITOR_CHAT" \
      bash "$ROOT_DIR/scripts/ai-provider-monitor.sh"
    ;;
  *)
    echo "unknown suite: $SUITE" >&2
    exit 64
    ;;
esac
