#!/usr/bin/env bash
# Sprint 1 + Sprint 2 smoke test for the deployed UniPost API.
#
# Usage:
#   ./scripts/smoke-test.sh                  # default: validate-only / no platform posts
#   API_KEY=up_live_... ./scripts/smoke-test.sh
#   BASE_URL=http://localhost:8080 ./scripts/smoke-test.sh
#
# Environment:
#   API_KEY    — UniPost API key. Defaults to the smoke key baked in below.
#   BASE_URL   — API base URL. Defaults to https://api.unipost.dev.
#   ACCOUNT_ID — A real social account ID to use for media / draft tests.
#                If unset, the script auto-picks the first account from
#                /v1/accounts. Tests that need a specific account
#                will skip when the auto-pick fails.
#
# What this script does NOT do:
#   - Actually publish to real social platforms (use the dashboard or
#     a separate end-to-end test for that). Drafts are created but not
#     published; the publish path is exercised against the validate /
#     preflight API only.
#   - Test webhook delivery (needs an external receiver).
#   - Test the MCP tools (needs Claude Desktop).
#
# Exit code: 0 if every assertion passed, 1 otherwise.

set -uo pipefail

API_KEY="${API_KEY:-up_live_AaNS2SGj2S2HmF6kUkDANzXemqq45nqrJcG4uNSEnk9g}"
BASE_URL="${BASE_URL:-https://api.unipost.dev}"

# ── Output helpers ────────────────────────────────────────────────────

if [[ -t 1 ]]; then
  GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[0;33m'
  CYAN='\033[0;36m'; BOLD='\033[1m'; DIM='\033[2m'; NC='\033[0m'
else
  GREEN=''; RED=''; YELLOW=''; CYAN=''; BOLD=''; DIM=''; NC=''
fi

PASS=0
FAIL=0
SKIP=0

section() {
  echo
  echo -e "${BOLD}${CYAN}── $1 ──${NC}"
}

pass() {
  echo -e "  ${GREEN}✓${NC} $1"
  PASS=$((PASS + 1))
}

fail() {
  echo -e "  ${RED}✗${NC} $1"
  if [[ -n "${2:-}" ]]; then
    echo -e "    ${DIM}$2${NC}"
  fi
  FAIL=$((FAIL + 1))
}

skip() {
  echo -e "  ${YELLOW}↷${NC} $1 ${DIM}(skipped: ${2:-no reason})${NC}"
  SKIP=$((SKIP + 1))
}

# api: GET / POST / PATCH / DELETE wrappers that capture both body and
# HTTP status. Sets RESP_BODY and RESP_STATUS as side effects so each
# test can assert on whichever one it cares about without re-invoking.
api() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local raw
  if [[ -n "$body" ]]; then
    raw=$(curl -sS -o /tmp/unipost-smoke-body -w '%{http_code}' \
      -X "$method" \
      -H "Authorization: Bearer $API_KEY" \
      -H "Content-Type: application/json" \
      -d "$body" \
      "${BASE_URL}${path}" 2>/tmp/unipost-smoke-err) || raw="000"
  else
    raw=$(curl -sS -o /tmp/unipost-smoke-body -w '%{http_code}' \
      -X "$method" \
      -H "Authorization: Bearer $API_KEY" \
      "${BASE_URL}${path}" 2>/tmp/unipost-smoke-err) || raw="000"
  fi
  RESP_STATUS="$raw"
  RESP_BODY="$(cat /tmp/unipost-smoke-body 2>/dev/null || echo '')"
}

api_no_auth() {
  local raw
  raw=$(curl -sS -o /tmp/unipost-smoke-body -w '%{http_code}' \
    "${BASE_URL}$1" 2>/tmp/unipost-smoke-err) || raw="000"
  RESP_STATUS="$raw"
  RESP_BODY="$(cat /tmp/unipost-smoke-body 2>/dev/null || echo '')"
}

require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    echo -e "${RED}error:${NC} jq is required (brew install jq / apt install jq)"
    exit 1
  fi
}

assert_status() {
  local expected="$1"
  local label="$2"
  if [[ "$RESP_STATUS" == "$expected" ]]; then
    pass "$label (HTTP $RESP_STATUS)"
  else
    fail "$label" "expected HTTP $expected, got $RESP_STATUS — body: ${RESP_BODY:0:200}"
  fi
}

assert_jq() {
  local query="$1"
  local expected="$2"
  local label="$3"
  local got
  got=$(echo "$RESP_BODY" | jq -r "$query" 2>/dev/null || echo "<jq error>")
  if [[ "$got" == "$expected" ]]; then
    pass "$label ($query = $expected)"
  else
    fail "$label" "expected $query = $expected, got $got"
  fi
}

assert_jq_truthy() {
  local query="$1"
  local label="$2"
  local got
  got=$(echo "$RESP_BODY" | jq -r "$query" 2>/dev/null || echo "")
  if [[ -n "$got" && "$got" != "null" && "$got" != "false" ]]; then
    pass "$label ($query = $got)"
  else
    fail "$label" "expected $query to be truthy, got '$got'"
  fi
}

# ── Boot check ────────────────────────────────────────────────────────

require_jq

echo -e "${BOLD}UniPost smoke test${NC}"
echo -e "  base url: ${CYAN}${BASE_URL}${NC}"
echo -e "  api key:  ${CYAN}${API_KEY:0:16}...${NC}"
echo

# ── Sprint 1 / PR1 — Capabilities API (no auth) ───────────────────────

section "Sprint 1 PR1 — Capabilities API"

api_no_auth "/v1/platforms/capabilities"
assert_status "200" "GET /v1/platforms/capabilities (no auth)"
assert_jq '.data.schema_version' '1.1' 'Schema version bumped to 1.1 (Sprint 2)'
assert_jq '.data.platforms.twitter.text.max_length' '280' 'Twitter max_length=280'
assert_jq '.data.platforms.twitter.text.supports_threads' 'true' 'Twitter supports_threads=true'
assert_jq '.data.platforms.instagram.media.requires_media' 'true' 'Instagram requires_media=true'
assert_jq '.data.platforms.youtube.media.images.max_count' '0' 'YouTube image max_count=0'

# Real GET (not HEAD — chi only sets Cache-Control on GET).
CACHE_HEADER=$(curl -sS -D - -o /dev/null "${BASE_URL}/v1/platforms/capabilities" | tr -d '\r' | grep -i '^cache-control:' || echo '')
if [[ "$CACHE_HEADER" == *"max-age=3600"* ]]; then
  pass "Cache-Control: public, max-age=3600 set"
elif [[ -z "$CACHE_HEADER" ]]; then
  fail "Cache-Control header missing" "expected 'public, max-age=3600' on /v1/platforms/capabilities"
else
  fail "Cache-Control header" "got: $CACHE_HEADER"
fi

# Sanity: every Sprint 1 platform present.
EXPECTED_PLATFORMS=(twitter instagram tiktok youtube threads linkedin bluesky)
for p in "${EXPECTED_PLATFORMS[@]}"; do
  assert_jq ".data.platforms.${p}.display_name | length > 0" 'true' "Platform $p present"
done

# ── Sprint 1 / PR3 — Validate endpoint (pure preflight) ───────────────

section "Sprint 1 PR3 — Validate endpoint"

# Auto-pick representative accounts. We prefer a "text-only friendly"
# platform (twitter/linkedin/bluesky/threads) for the happy-path
# validate test so a missing-media-required platform like TikTok or
# Instagram doesn't dominate the auto-pick. ANY_ID is the catch-all
# for tests that don't care about media requirements.
api GET "/v1/accounts"
assert_status "200" "GET /v1/accounts"
TWITTER_ID=$(echo "$RESP_BODY" | jq -r '[.data[] | select(.platform == "twitter")][0].id // empty')
IG_ID=$(echo "$RESP_BODY" | jq -r '[.data[] | select(.platform == "instagram")][0].id // empty')
TEXT_ID=$(echo "$RESP_BODY" | jq -r '[.data[] | select(.platform == "twitter" or .platform == "linkedin" or .platform == "bluesky" or .platform == "threads")][0].id // empty')
ANY_ID=$(echo "$RESP_BODY" | jq -r '.data[0].id // empty')

if [[ -z "$ANY_ID" ]]; then
  echo -e "  ${YELLOW}note:${NC} no social accounts in this workspace — most tests below will skip"
fi

# Happy-path validate. Use a text-friendly platform so the test
# isn't tripped by a TikTok/IG missing-media error.
if [[ -n "$TEXT_ID" ]]; then
  api POST "/v1/posts/validate" "$(jq -nc --arg id "$TEXT_ID" \
    '{platform_posts: [{account_id: $id, caption: "smoke test"}]}')"
  assert_status "200" "POST /validate happy path (text-friendly account)"
  assert_jq '.data.valid' 'true' 'validate returns valid=true'
elif [[ -n "$ANY_ID" ]]; then
  # Fall back: use any account but allow missing-media to be a soft pass.
  api POST "/v1/posts/validate" "$(jq -nc --arg id "$ANY_ID" \
    '{platform_posts: [{account_id: $id, caption: "smoke test", media_urls: ["https://x/y.jpg"]}]}')"
  assert_status "200" "POST /validate happy path (with stub media)"
  assert_jq '.data.valid' 'true' 'validate returns valid=true'
else
  skip "validate happy path" "no account_id available"
fi

# Caption too long on Twitter — must surface exceeds_max_length.
if [[ -n "$TWITTER_ID" ]]; then
  LONG=$(printf 'a%.0s' $(seq 1 300))
  api POST "/v1/posts/validate" "$(jq -nc --arg id "$TWITTER_ID" --arg c "$LONG" \
    '{platform_posts: [{account_id: $id, caption: $c}]}')"
  assert_status "200" "POST /validate (long caption)"
  assert_jq '.data.valid' 'false' 'long caption invalid'
  assert_jq '.data.errors[0].code' 'exceeds_max_length' 'error code = exceeds_max_length'
else
  skip "long-caption test" "no twitter account"
fi

# Mutually exclusive shapes — both account_ids and platform_posts.
api POST "/v1/posts/validate" \
  '{"account_ids":["a"],"caption":"x","platform_posts":[{"account_id":"a","caption":"x"}]}'
assert_status "422" "POST /validate rejects mutually-exclusive shapes"

# Missing both shapes.
api POST "/v1/posts/validate" '{}'
assert_status "422" "POST /validate rejects empty body"

# ── Sprint 2 PR3 — thread_position validation ─────────────────────────

section "Sprint 2 PR3 — Thread validation"

if [[ -n "$TWITTER_ID" ]]; then
  # Happy path thread.
  api POST "/v1/posts/validate" "$(jq -nc --arg id "$TWITTER_ID" \
    '{platform_posts: [
      {account_id: $id, caption: "1/", thread_position: 1},
      {account_id: $id, caption: "2/", thread_position: 2},
      {account_id: $id, caption: "3/", thread_position: 3}
    ]}')"
  assert_jq '.data.valid' 'true' '3-tweet thread valid'

  # Missing position 1.
  api POST "/v1/posts/validate" "$(jq -nc --arg id "$TWITTER_ID" \
    '{platform_posts: [
      {account_id: $id, caption: "x", thread_position: 2},
      {account_id: $id, caption: "y", thread_position: 3}
    ]}')"
  assert_jq '.data.errors[0].code' 'thread_positions_not_contiguous' 'positions not contiguous'

  # Mixed thread + standalone.
  api POST "/v1/posts/validate" "$(jq -nc --arg id "$TWITTER_ID" \
    '{platform_posts: [
      {account_id: $id, caption: "thread 1", thread_position: 1},
      {account_id: $id, caption: "thread 2", thread_position: 2},
      {account_id: $id, caption: "standalone"}
    ]}')"
  assert_jq '.data.errors[0].code' 'thread_mixed_with_single' 'mixed thread + standalone'
else
  skip "thread validation tests" "no twitter account"
fi

if [[ -n "$IG_ID" ]]; then
  # Threads on a platform that doesn't support them.
  api POST "/v1/posts/validate" "$(jq -nc --arg id "$IG_ID" \
    '{platform_posts: [{account_id: $id, caption: "x", media_urls: ["https://x/y.jpg"], thread_position: 1}]}')"
  assert_jq '.data.errors | map(.code) | index("threads_unsupported") != null' 'true' 'threads_unsupported on instagram'
else
  skip "instagram thread test" "no instagram account"
fi

# ── Sprint 2 PR1+PR2 — Media library ──────────────────────────────────

section "Sprint 2 PR1+PR2 — Media library"

# Tiny 1x1 transparent PNG, base64 → bytes via openssl.
TINY_PNG_B64='iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgAAIAAAUAAen63NgAAAAASUVORK5CYII='
TINY_PNG_BYTES=/tmp/unipost-smoke-tiny.png
echo "$TINY_PNG_B64" | base64 -d > "$TINY_PNG_BYTES" 2>/dev/null || \
  echo "$TINY_PNG_B64" | base64 --decode > "$TINY_PNG_BYTES"
TINY_PNG_SIZE=$(wc -c < "$TINY_PNG_BYTES" | tr -d ' ')

# Reject bad mime.
api POST "/v1/media" '{"filename":"x.exe","content_type":"application/x-executable","size_bytes":1024}'
assert_status "422" "POST /v1/media rejects bad mime"

# Reject oversized.
api POST "/v1/media" '{"filename":"big.jpg","content_type":"image/jpeg","size_bytes":104857600}'
assert_status "422" "POST /v1/media rejects > 25 MB"

# Happy-path create.
api POST "/v1/media" "$(jq -nc --arg sz "$TINY_PNG_SIZE" \
  '{filename: "smoke.png", content_type: "image/png", size_bytes: ($sz | tonumber)}')"
assert_status "201" "POST /v1/media create"
MEDIA_ID=$(echo "$RESP_BODY" | jq -r '.data.id // empty')
UPLOAD_URL=$(echo "$RESP_BODY" | jq -r '.data.upload_url // empty')

if [[ -z "$MEDIA_ID" || -z "$UPLOAD_URL" ]]; then
  fail "media create returned no id/upload_url" "$RESP_BODY"
else
  pass "media created: $MEDIA_ID"
  # PUT the bytes to R2.
  PUT_STATUS=$(curl -sS -o /tmp/unipost-smoke-r2 -w '%{http_code}' \
    -X PUT \
    -H "Content-Type: image/png" \
    --data-binary "@$TINY_PNG_BYTES" \
    "$UPLOAD_URL")
  if [[ "$PUT_STATUS" == "200" || "$PUT_STATUS" == "204" ]]; then
    pass "PUT to R2 succeeded ($PUT_STATUS)"
  else
    fail "PUT to R2" "got $PUT_STATUS, body: $(cat /tmp/unipost-smoke-r2 2>/dev/null | head -c 200)"
  fi

  # GET media — should hydrate from pending → uploaded.
  api GET "/v1/media/${MEDIA_ID}"
  assert_status "200" "GET /v1/media/{id}"
  assert_jq '.data.status' 'uploaded' 'media status hydrated to uploaded'
  assert_jq_truthy '.data.download_url' 'download URL present'

  # DELETE.
  api DELETE "/v1/media/${MEDIA_ID}"
  assert_status "200" "DELETE /v1/media/{id}"
fi

# media_id ownership check via validate.
if [[ -n "$ANY_ID" ]]; then
  api POST "/v1/posts/validate" "$(jq -nc --arg id "$ANY_ID" \
    '{platform_posts: [{account_id: $id, caption: "x", media_ids: ["med_does_not_exist"]}]}')"
  assert_jq '.data.errors | map(.code) | index("media_id_not_in_workspace") != null' 'true' 'unknown media_id rejected'
else
  skip "media_id validation" "no account_id"
fi

# ── Sprint 2 PR4 — Drafts API ─────────────────────────────────────────

section "Sprint 2 PR4 — Drafts API"

DRAFT_ID=""
if [[ -n "$ANY_ID" ]]; then
  api POST "/v1/posts" "$(jq -nc --arg id "$ANY_ID" \
    '{status: "draft", platform_posts: [{account_id: $id, caption: "draft me"}]}')"
  assert_status "201" "POST /v1/posts (draft)"
  DRAFT_ID=$(echo "$RESP_BODY" | jq -r '.data.id // empty')
  assert_jq '.data.status' 'draft' 'draft status correct'
  assert_jq_truthy '.data.validation' 'validation embedded in draft response'

  if [[ -n "$DRAFT_ID" ]]; then
    # PATCH the draft.
    api PATCH "/v1/posts/${DRAFT_ID}" "$(jq -nc --arg id "$ANY_ID" \
      '{platform_posts: [{account_id: $id, caption: "draft me v2"}]}')"
    assert_status "200" "PATCH draft"
    assert_jq '.data.caption' 'draft me v2' 'PATCH replaced caption'

    # PATCH a non-draft (we don't have one handy without publishing,
    # so test the inverse: PATCH a deleted/missing post).
    api PATCH "/v1/posts/post_does_not_exist_at_all" '{"platform_posts":[{"account_id":"x","caption":"y"}]}'
    if [[ "$RESP_STATUS" == "409" || "$RESP_STATUS" == "404" || "$RESP_STATUS" == "500" ]]; then
      pass "PATCH on missing post rejected (HTTP $RESP_STATUS)"
    else
      fail "PATCH on missing post should fail" "got HTTP $RESP_STATUS"
    fi

    # DELETE the draft.
    api DELETE "/v1/posts/${DRAFT_ID}"
    assert_status "200" "DELETE draft"
    DRAFT_ID="" # consumed
  fi
else
  skip "drafts API tests" "no account_id"
fi

# ── Sprint 2 PR5 — Hosted preview ─────────────────────────────────────

section "Sprint 2 PR5 — Hosted preview"

if [[ -n "$ANY_ID" ]]; then
  # Mint a fresh draft just for the preview test.
  api POST "/v1/posts" "$(jq -nc --arg id "$ANY_ID" \
    '{status: "draft", platform_posts: [{account_id: $id, caption: "preview me"}]}')"
  PREVIEW_DRAFT=$(echo "$RESP_BODY" | jq -r '.data.id // empty')

  if [[ -n "$PREVIEW_DRAFT" ]]; then
    api POST "/v1/posts/${PREVIEW_DRAFT}/preview-link" ""
    assert_status "200" "POST /preview-link on a draft"
    PREVIEW_URL=$(echo "$RESP_BODY" | jq -r '.data.url // empty')
    PREVIEW_TOKEN=$(echo "$RESP_BODY" | jq -r '.data.token // empty')

    if [[ -n "$PREVIEW_URL" && "$PREVIEW_URL" == *"/preview/"* && "$PREVIEW_URL" == *"?token="* ]]; then
      pass "preview URL shape: app.unipost.dev/preview/<id>?token=..."
    else
      fail "preview URL shape" "got: $PREVIEW_URL"
    fi

    # Public draft endpoint with the token.
    api_no_auth "/v1/public/drafts/${PREVIEW_DRAFT}?token=${PREVIEW_TOKEN}"
    assert_status "200" "GET /v1/public/drafts/{id} with valid token"
    assert_jq '.data.post_id' "$PREVIEW_DRAFT" 'public endpoint returns the right post'

    # Tampered token.
    api_no_auth "/v1/public/drafts/${PREVIEW_DRAFT}?token=tampered.signature"
    assert_status "401" "Public endpoint rejects tampered token"

    # Token for a different post.
    api_no_auth "/v1/public/drafts/post_other_id?token=${PREVIEW_TOKEN}"
    assert_status "401" "Public endpoint rejects token/post mismatch"

    # Cleanup.
    api DELETE "/v1/posts/${PREVIEW_DRAFT}"
  else
    skip "preview link tests" "could not create preview draft"
  fi
else
  skip "preview tests" "no account_id"
fi

# ── Sprint 2 PR7 — list_posts filters + cursor ────────────────────────

section "Sprint 2 PR7 — list_posts filters + cursor"

api GET "/v1/posts?limit=2"
assert_status "200" "GET /v1/posts?limit=2"
assert_jq_truthy '.data' 'data array present'

NEXT_CURSOR=$(echo "$RESP_BODY" | jq -r '.meta.next_cursor // .next_cursor // empty')
if [[ -n "$NEXT_CURSOR" ]]; then
  pass "next_cursor returned for paginated list"
  api GET "/v1/posts?limit=2&cursor=${NEXT_CURSOR}"
  assert_status "200" "GET /v1/posts?cursor=..."
else
  echo -e "    ${DIM}note: workspace has ≤2 posts, no next_cursor expected${NC}"
fi

# Status filter.
api GET "/v1/posts?status=draft&limit=5"
assert_status "200" "GET /v1/posts?status=draft"

# Date range filter (last hour).
NOW=$(date -u +%Y-%m-%dT%H:%M:%SZ)
HOUR_AGO=$(date -u -v-1H +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)
api GET "/v1/posts?from=${HOUR_AGO}&to=${NOW}"
assert_status "200" "GET /v1/posts?from=&to="

# ── Sprint 2 PR7 — Account health ─────────────────────────────────────

section "Sprint 2 PR7 — Account health"

if [[ -n "$ANY_ID" ]]; then
  api GET "/v1/accounts/${ANY_ID}/health"
  assert_status "200" "GET /v1/accounts/{id}/health"
  assert_jq_truthy '.data.status' 'health.status present'
  STATUS=$(echo "$RESP_BODY" | jq -r '.data.status')
  if [[ "$STATUS" == "ok" || "$STATUS" == "degraded" || "$STATUS" == "disconnected" ]]; then
    pass "health.status is one of {ok, degraded, disconnected}: $STATUS"
  else
    fail "health.status enum" "got $STATUS"
  fi

  # 404 for an account in another workspace.
  api GET "/v1/accounts/account_does_not_exist/health"
  assert_status "404" "GET /health for unknown account → 404"
else
  skip "account health" "no account_id"
fi

# ── Sprint 1 PR1 — Per-account capabilities ───────────────────────────

if [[ -n "$ANY_ID" ]]; then
  section "Sprint 1 PR1 — Per-account capabilities"
  api GET "/v1/accounts/${ANY_ID}/capabilities"
  assert_status "200" "GET /v1/accounts/{id}/capabilities"
  assert_jq '.data.schema_version' '1.1' 'schema 1.1'
  assert_jq_truthy '.data.platform' 'platform present'
fi

# ── Sprint 1 PR8 — Webhook secret server-generated ────────────────────

section "Sprint 1 PR8 — Webhook secret server-generated"

# Reject client-provided secret.
api POST "/v1/webhooks" '{"url":"https://webhook.site/_smoke","events":["post.published"],"secret":"my-secret"}'
assert_status "422" "POST /v1/webhooks rejects client-provided secret"

# Create webhook with no secret — server generates one.
api POST "/v1/webhooks" '{"url":"https://webhook.site/_smoke","events":["post.published"]}'
if [[ "$RESP_STATUS" == "201" ]]; then
  pass "POST /v1/webhooks creates webhook"
  WEBHOOK_ID=$(echo "$RESP_BODY" | jq -r '.data.id // empty')
  WEBHOOK_SECRET=$(echo "$RESP_BODY" | jq -r '.data.secret // empty')
  WEBHOOK_PREVIEW=$(echo "$RESP_BODY" | jq -r '.data.secret_preview // empty')

  if [[ "$WEBHOOK_SECRET" =~ ^whsec_[a-f0-9]{32}$ ]]; then
    pass "Server generated whsec_+32 hex char secret"
  else
    fail "secret format" "got '$WEBHOOK_SECRET'"
  fi

  if [[ "$WEBHOOK_PREVIEW" == "whsec_"* && "$WEBHOOK_PREVIEW" == *"…" ]]; then
    pass "secret_preview returned (whsec_xx…)"
  else
    fail "secret_preview format" "got '$WEBHOOK_PREVIEW'"
  fi

  # GET should NOT leak plaintext.
  api GET "/v1/webhooks/${WEBHOOK_ID}"
  PLAIN_LEAK=$(echo "$RESP_BODY" | jq -r '.data.secret // empty')
  if [[ -z "$PLAIN_LEAK" ]]; then
    pass "GET /v1/webhooks/{id} hides plaintext secret"
  else
    fail "plaintext leak on GET" "got: $PLAIN_LEAK"
  fi

  # Rotate.
  api POST "/v1/webhooks/${WEBHOOK_ID}/rotate" ""
  assert_status "200" "POST /v1/webhooks/{id}/rotate"
  NEW_SECRET=$(echo "$RESP_BODY" | jq -r '.data.secret // empty')
  if [[ -n "$NEW_SECRET" && "$NEW_SECRET" != "$WEBHOOK_SECRET" ]]; then
    pass "rotated secret differs from original"
  else
    fail "rotate" "new secret empty or unchanged"
  fi

  # Cleanup.
  api DELETE "/v1/webhooks/${WEBHOOK_ID}"
  assert_status "200" "DELETE /v1/webhooks/{id}"
else
  fail "POST /v1/webhooks" "HTTP $RESP_STATUS — body: ${RESP_BODY:0:200}"
fi

# ── Summary ────────────────────────────────────────────────────────────

echo
echo -e "${BOLD}── Summary ──${NC}"
echo -e "  ${GREEN}passed: ${PASS}${NC}"
echo -e "  ${RED}failed: ${FAIL}${NC}"
echo -e "  ${YELLOW}skipped: ${SKIP}${NC}"
echo

if [[ "$FAIL" -gt 0 ]]; then
  echo -e "${RED}${BOLD}✗ smoke test FAILED${NC}"
  exit 1
fi
echo -e "${GREEN}${BOLD}✓ smoke test PASSED${NC}"
exit 0
