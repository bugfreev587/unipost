#!/usr/bin/env bash
set -euo pipefail

require_env() {
  local name="$1"
  if [ -z "${!name:-}" ]; then
    echo "$name is required"
    exit 1
  fi
}

wait_for_pr_merged() {
  local pr_number="$1"
  local label="$2"

  for _ in $(seq 1 90); do
    state="$(gh pr view "$pr_number" --json state --jq .state)"
    if [ "$state" = "MERGED" ]; then
      echo "$label PR #$pr_number merged"
      return 0
    fi
    if [ "$state" = "CLOSED" ]; then
      echo "$label PR #$pr_number closed without merge"
      exit 1
    fi
    sleep 20
  done

  echo "$label PR #$pr_number did not merge before timeout"
  exit 1
}

wait_for_pr_checks() {
  local pr_number="$1"
  local label="$2"
  local output

  for _ in $(seq 1 60); do
    if output="$(gh pr checks "$pr_number" 2>&1)"; then
      if [ -n "$output" ]; then
        return 0
      fi
    elif ! printf "%s" "$output" | grep -qi "no checks"; then
      echo "$output"
      exit 1
    fi
    sleep 10
  done

  echo "No checks reported for $label PR #$pr_number before timeout"
  exit 1
}

open_and_merge_pr() {
  local base="$1"
  local head="$2"
  local title="$3"
  local body="$4"
  local label="$5"

  git fetch origin "$base"
  pr_url="$(gh pr create --base "$base" --head "$head" --title "$title" --body "$body")"
  pr_number="${pr_url##*/}"
  wait_for_pr_checks "$pr_number" "$label"
  gh pr checks "$pr_number" --watch
  gh pr merge "$pr_number" --squash
  wait_for_pr_merged "$pr_number" "$label"
}

verify_url() {
  local url="$1"
  local label="$2"

  for _ in $(seq 1 60); do
    status="$(curl -sS -o /dev/null -w "%{http_code}" "$url" || true)"
    if [ "$status" = "200" ]; then
      echo "$label verified at $url"
      return 0
    fi
    sleep 15
  done

  echo "$label did not return HTTP 200 at $url"
  exit 1
}

require_env CHANGELOG_CANDIDATE_ID
require_env CHANGELOG_RELEASE_GITHUB_TOKEN
require_env RELEASE_BRANCH

body="Automated changelog release for candidate \`$CHANGELOG_CANDIDATE_ID\`.

- Source hash: \`${CHANGELOG_SOURCE_HASH:-unknown}\`
- Requested by: \`${REQUESTED_BY:-unknown}\`
- Action request: \`${GITHUB_RUN_ID:-unknown}\`"

open_and_merge_pr "dev" "$RELEASE_BRANCH" "Add changelog release $CHANGELOG_CANDIDATE_ID" "$body" "dev"
verify_url "https://dev.unipost.dev/changelog" "Development changelog"

open_and_merge_pr "staging" "dev" "Promote changelog release $CHANGELOG_CANDIDATE_ID to staging" "$body" "staging"
verify_url "https://staging.unipost.dev/changelog" "Staging changelog"

open_and_merge_pr "main" "staging" "Promote changelog release $CHANGELOG_CANDIDATE_ID to production" "$body" "production"
verify_url "https://unipost.dev/changelog" "Production changelog"
