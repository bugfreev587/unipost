#!/usr/bin/env bash

set -euo pipefail

ALERT_WEBHOOK_URL="${ALERT_WEBHOOK_URL:-}"

if [[ -z "$ALERT_WEBHOOK_URL" ]]; then
  echo "ALERT_WEBHOOK_URL is not set; skipping alert"
  exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required to send alerts" >&2
  exit 1
fi

RUN_URL="${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-unknown}/actions/runs/${GITHUB_RUN_ID:-unknown}"
BASE_URL="${BASE_URL:-https://api.unipost.dev}"
TRIGGER="${GITHUB_EVENT_NAME:-manual}"
BRANCH="${GITHUB_REF_NAME:-unknown}"
REPOSITORY="${GITHUB_REPOSITORY:-unknown}"
FAILED_SUITES="${FAILED_SUITES:-unknown}"

MESSAGE=$(
  cat <<EOF
UniPost regression monitor failed.
Repository: ${REPOSITORY}
Branch: ${BRANCH}
Trigger: ${TRIGGER}
Base URL: ${BASE_URL}
Failed suites: ${FAILED_SUITES}
Run: ${RUN_URL}
EOF
)

if [[ "$ALERT_WEBHOOK_URL" == *"discord.com/api/webhooks/"* || "$ALERT_WEBHOOK_URL" == *"discordapp.com/api/webhooks/"* ]]; then
  PAYLOAD="$(jq -nc --arg content "$MESSAGE" '{content: $content}')"
else
  PAYLOAD="$(jq -nc --arg text "$MESSAGE" '{text: $text}')"
fi

curl -fsS -X POST \
  -H "Content-Type: application/json" \
  -d "$PAYLOAD" \
  "$ALERT_WEBHOOK_URL" >/dev/null
