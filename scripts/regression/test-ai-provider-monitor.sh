#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MONITOR="${ROOT_DIR}/scripts/ai-provider-monitor.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

FAKE_BIN="${TMP_DIR}/bin"
mkdir -p "$FAKE_BIN"

cat >"${FAKE_BIN}/curl" <<'FAKE_CURL'
#!/usr/bin/env bash
set -euo pipefail

printf '%s\n' "$*" >>"${FAKE_CURL_LOG:?}"

out=""
url=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o)
      out="$2"
      shift 2
      ;;
    -w|-X|-H|-d|--data|--data-raw|--max-time)
      shift 2
      ;;
    -s|-S|-sS|--fail|--show-error|--silent)
      shift
      ;;
    *)
      url="$1"
      shift
      ;;
  esac
done

if [[ -z "$out" ]]; then
  echo "fake curl expected -o" >&2
  exit 2
fi

case "$url" in
  */models)
    if [[ "${FAKE_CURL_MODE:-ok}" == "missing-model" ]]; then
      printf '{"data":[{"id":"other-model"}]}' >"$out"
    else
      printf '{"data":[{"id":"gpt-monitor"},{"id":"claude-monitor"}]}' >"$out"
    fi
    printf '200'
    ;;
  */chat/completions)
    printf '{"choices":[{"message":{"content":"unipost_ai_monitor_ok"}}]}' >"$out"
    printf '200'
    ;;
  *)
    printf '{"error":"unexpected url"}' >"$out"
    printf '404'
    ;;
esac
FAKE_CURL
chmod +x "${FAKE_BIN}/curl"

run_monitor() {
  local output_file="$1"
  shift
  (
    export PATH="${FAKE_BIN}:$PATH"
    export FAKE_CURL_LOG="${TMP_DIR}/curl.log"
    : >"$FAKE_CURL_LOG"
    "$@"
  ) >"$output_file" 2>&1
}

assert_contains() {
  local file="$1"
  local expected="$2"
  if ! grep -Fq "$expected" "$file"; then
    echo "expected ${file} to contain: ${expected}" >&2
    echo "--- output ---" >&2
    cat "$file" >&2
    exit 1
  fi
}

success_output="${TMP_DIR}/success.out"
run_monitor "$success_output" env \
  TOKENGATE_REGRESSION_API_KEY="tg-test" \
  TOKENGATE_REGRESSION_CHAT_MODEL="gpt-monitor" \
  TOKENGATE_EXPECTED_MODELS="gpt-monitor,claude-monitor" \
  AI_PROVIDER_MONITOR_CHAT="true" \
  "$MONITOR"
assert_contains "$success_output" "TokenGate /models reachable"
assert_contains "$success_output" "TokenGate chat/completions reachable"
assert_contains "${TMP_DIR}/curl.log" "Authorization: Bearer tg-test"
assert_contains "${TMP_DIR}/curl.log" "https://gateway.mytokengate.com/v1/models"
assert_contains "${TMP_DIR}/curl.log" "https://gateway.mytokengate.com/v1/chat/completions"

missing_model_output="${TMP_DIR}/missing-model.out"
if run_monitor "$missing_model_output" env \
  FAKE_CURL_MODE="missing-model" \
  TOKENGATE_REGRESSION_API_KEY="tg-test" \
  TOKENGATE_REGRESSION_CHAT_MODEL="gpt-monitor" \
  TOKENGATE_EXPECTED_MODELS="gpt-monitor" \
  AI_PROVIDER_MONITOR_CHAT="false" \
  "$MONITOR"; then
  echo "expected monitor to fail when configured model is unavailable" >&2
  cat "$missing_model_output" >&2
  exit 1
fi
assert_contains "$missing_model_output" "Configured model unavailable: gpt-monitor"

wrapper_output="${TMP_DIR}/wrapper.out"
wrapper_log_dir="${TMP_DIR}/wrapper-logs"
mkdir -p "$wrapper_log_dir"
(
  export PATH="${FAKE_BIN}:$PATH"
  export FAKE_CURL_LOG="${TMP_DIR}/wrapper-curl.log"
  : >"$FAKE_CURL_LOG"
  env \
    LOG_DIR="$wrapper_log_dir" \
    TOKENGATE_REGRESSION_API_KEY="tg-test" \
    TOKENGATE_REGRESSION_CHAT_MODEL="gpt-monitor" \
    TOKENGATE_REGRESSION_EXPECTED_MODELS="gpt-monitor" \
    AI_PROVIDER_MONITOR_CHAT="true" \
    bash "$ROOT_DIR/scripts/regression/run-suite.sh" ai-provider
) >"$wrapper_output" 2>&1
assert_contains "$wrapper_log_dir/ai-provider.log" "TokenGate /models reachable"
assert_contains "$wrapper_log_dir/ai-provider.log" "TokenGate chat/completions reachable"

echo "ai provider monitor tests passed"
