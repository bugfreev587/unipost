#!/usr/bin/env bash

set -euo pipefail

PASS=0
FAIL=0

TOKENGATE_BASE_URL="${TOKENGATE_REGRESSION_BASE_URL:-${TOKENGATE_BASE_URL:-https://gateway.mytokengate.com/v1}}"
TOKENGATE_API_KEY="${TOKENGATE_REGRESSION_API_KEY:-}"
TOKENGATE_EXPECTED_MODELS="${TOKENGATE_REGRESSION_EXPECTED_MODELS:-${TOKENGATE_EXPECTED_MODELS:-}}"
TOKENGATE_CHAT_MODEL="${TOKENGATE_REGRESSION_CHAT_MODEL:-${TOKENGATE_CHAT_MODEL:-}}"
AI_PROVIDER_MONITOR_CHAT="${AI_PROVIDER_MONITOR_CHAT:-true}"

trim() {
  local value="$*"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

normalize_base_url() {
  local value
  value="$(trim "$1")"
  printf '%s' "${value%/}"
}

pass() {
  echo "PASS $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "FAIL $1" >&2
  if [[ -n "${2:-}" ]]; then
    echo "  $2" >&2
  fi
  FAIL=$((FAIL + 1))
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "$1 is required"
    summarize
  fi
}

summarize() {
  echo
  echo "AI provider monitor summary"
  echo "  passed: $PASS"
  echo "  failed: $FAIL"
  if [[ "$FAIL" -gt 0 ]]; then
    exit 1
  fi
  exit 0
}

http_json() {
  local method="$1"
  local url="$2"
  local body="$3"
  local output_file="$4"
  local status

  if [[ -n "$body" ]]; then
    status="$(curl -sS -o "$output_file" -w '%{http_code}' \
      --max-time 30 \
      -X "$method" \
      -H "Authorization: Bearer ${TOKENGATE_API_KEY}" \
      -H "Content-Type: application/json" \
      --data-raw "$body" \
      "$url")" || status="000"
  else
    status="$(curl -sS -o "$output_file" -w '%{http_code}' \
      --max-time 30 \
      -X "$method" \
      -H "Authorization: Bearer ${TOKENGATE_API_KEY}" \
      -H "Content-Type: application/json" \
      "$url")" || status="000"
  fi

  printf '%s' "$status"
}

provider_excerpt() {
  local file="$1"
  if [[ ! -s "$file" ]]; then
    printf '<empty response>'
    return
  fi
  head -c 500 "$file" | tr '\n' ' '
}

check_models() {
  local base_url="$1"
  local body_file
  body_file="$(mktemp)"
  local status
  status="$(http_json GET "${base_url}/models" "" "$body_file")"

  if [[ "$status" != "200" ]]; then
    fail "TokenGate /models failed" "HTTP ${status}: $(provider_excerpt "$body_file")"
    rm -f "$body_file"
    return
  fi

  local models
  if ! models="$(jq -r '[((.data // [])[]?), ((.models // [])[]?)][] | (.id // .name // .model // empty)' "$body_file" 2>/dev/null)"; then
    fail "TokenGate /models returned invalid JSON" "$(provider_excerpt "$body_file")"
    rm -f "$body_file"
    return
  fi

  pass "TokenGate /models reachable"

  if [[ -n "$(trim "$TOKENGATE_EXPECTED_MODELS")" ]]; then
    local missing=()
    local raw_model
    IFS=',' read -r -a expected_models <<<"$TOKENGATE_EXPECTED_MODELS"
    for raw_model in "${expected_models[@]}"; do
      local model
      model="$(trim "$raw_model")"
      if [[ -z "$model" ]]; then
        continue
      fi
      if ! grep -Fxq "$model" <<<"$models"; then
        missing+=("$model")
      fi
    done
    if [[ "${#missing[@]}" -gt 0 ]]; then
      fail "Configured model unavailable: ${missing[*]}"
    else
      pass "Configured TokenGate models are listed"
    fi
  fi

  rm -f "$body_file"
}

chat_monitor_enabled() {
  case "$(printf '%s' "$AI_PROVIDER_MONITOR_CHAT" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

check_chat_completion() {
  local base_url="$1"
  local model
  model="$(trim "$TOKENGATE_CHAT_MODEL")"
  if [[ -z "$model" ]]; then
    fail "TOKENGATE_REGRESSION_CHAT_MODEL is required when AI_PROVIDER_MONITOR_CHAT=true"
    return
  fi

  local body
  body="$(jq -nc --arg model "$model" '{
    model: $model,
    messages: [
      {role: "system", content: "You are a synthetic health check. Reply briefly."},
      {role: "user", content: "Reply with unipost_ai_monitor_ok."}
    ],
    temperature: 0,
    max_tokens: 16
  }')"

  local body_file
  body_file="$(mktemp)"
  local status
  status="$(http_json POST "${base_url}/chat/completions" "$body" "$body_file")"

  if [[ "$status" != "200" ]]; then
    fail "TokenGate chat/completions failed" "HTTP ${status}: $(provider_excerpt "$body_file")"
    rm -f "$body_file"
    return
  fi

  local content
  content="$(jq -r '.choices[0].message.content // empty' "$body_file" 2>/dev/null || true)"
  if [[ -z "$(trim "$content")" ]]; then
    fail "TokenGate chat/completions returned no message content" "$(provider_excerpt "$body_file")"
    rm -f "$body_file"
    return
  fi

  pass "TokenGate chat/completions reachable"
  rm -f "$body_file"
}

require_cmd curl
require_cmd jq

TOKENGATE_BASE_URL="$(normalize_base_url "$TOKENGATE_BASE_URL")"
if [[ -z "$TOKENGATE_API_KEY" ]]; then
  fail "TOKENGATE_REGRESSION_API_KEY is required"
  summarize
fi

echo "UniPost AI provider monitor"
echo "  provider: tokengate"
echo "  base url: ${TOKENGATE_BASE_URL}"

check_models "$TOKENGATE_BASE_URL"
if chat_monitor_enabled; then
  check_chat_completion "$TOKENGATE_BASE_URL"
else
  echo "SKIP TokenGate chat/completions skipped (AI_PROVIDER_MONITOR_CHAT=${AI_PROVIDER_MONITOR_CHAT})"
fi

summarize
