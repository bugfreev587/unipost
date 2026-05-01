# UniPost — X (Twitter) Paid-Only + Per-Platform Daily Caps PRD
**Gate X behind paid plans and add per-account/per-day publish caps to protect customer accounts**
Version 1.0 | April 2026

---

## 1. Background

### 1.1 The product risks

UniPost currently treats every connected social account identically at runtime:

- the only per-account budget is `workspaces.per_account_monthly_limit` — one workspace-wide number, applied uniformly across all eight platforms, calendar-month window
  - source: [per_account.go](/Users/xiaoboyu/unipost/api/internal/quota/per_account.go), wired in [social_post_queue.go:393](/Users/xiaoboyu/unipost/api/internal/handler/social_post_queue.go) and [social_posts.go:541](/Users/xiaoboyu/unipost/api/internal/handler/social_posts.go)
- the per-plan rate limiters in [ratelimit/plans.go](/Users/xiaoboyu/unipost/api/internal/ratelimit/plans.go) are workspace-wide enqueue/burst controls; they explicitly do **not** track per-account or per-platform behavior (see file-level comment, line 8)
- every plan — including Free — advertises "All 8 platforms" on the pricing page ([pricing/page.tsx:22](/Users/xiaoboyu/unipost/dashboard/src/app/pricing/page.tsx))

This produces two issues we want to address now:

1. **Account-suspension risk.** A buggy script or an over-eager scheduler can push 200+ posts/day to a single Instagram, X, or Threads account. Each platform has its own informal "looks like a spam bot" threshold, and once a customer's account gets flagged we cannot un-flag it. Zernio (a direct competitor) ships explicit per-account daily caps for exactly this reason, and surfaces them as a **trust signal** to the customer ("we protect your accounts from being flagged").
2. **Free plan abuse on X.** X / Twitter API access carries the highest per-call cost in our adapter mix and the highest abuse signal — most of our raw API spend on free workspaces traces to X. Letting Free plans publish to X is no longer economic for us, and the Free tier's 100/month quota is small enough that gating X behind paid plans is a clean cut without disturbing the "permanent free trial" positioning.

### 1.2 Current codebase reality

Existing pieces we will reuse:

- per-account quota tracker pattern: [quota/per_account.go](/Users/xiaoboyu/unipost/api/internal/quota/per_account.go)
  - already supports `Allow(accountID) bool` semantics, snapshot-then-decrement, multi-account tracker per request
  - we'll add a sibling `PerPlatformDailyTracker` next to it rather than overload the monthly tracker
- plan id resolution: [quota/checker.go:85 `PlanIDFor`](/Users/xiaoboyu/unipost/api/internal/quota/checker.go)
  - returns the workspace's `plans.id` ("free" / "p10" / …); used today for billing and rate limits, ideal for the X gate as well
- plans table already has a precedent for plan-gated features: `white_label bool` column, added in [013_add_white_label_to_plans.sql](/Users/xiaoboyu/unipost/api/internal/db/migrations/013_add_white_label_to_plans.sql). We mirror that pattern.
- platform validation entry point: [internal/platform/validate.go](/Users/xiaoboyu/unipost/api/internal/platform/validate.go) and the per-result publish path in [handler/social_posts.go](/Users/xiaoboyu/unipost/api/internal/handler/social_posts.go) — both have natural hook points for the new checks.

### 1.3 What we do NOT have today

- No per-platform daily caps anywhere in the publish path.
- No mechanism for plan-gating which **platforms** a workspace can use (only feature flags like white_label).
- Pricing page, comparison rows, FAQ, and platform docs all assume Free = all 8 platforms.

---

## 2. Goals

1. **Move X out of Free.** Free plans cannot create new posts targeting X. Existing connected X accounts on Free workspaces remain connected (read-only) but publish attempts return a deterministic "upgrade required" error.
2. **Add per-account/per-day publish caps,** enforced server-side at publish time, scoped per `social_account_id` × per platform × per UTC calendar day:
   - X / Twitter: **20 / day**
   - Instagram: **100 / day**
   - Facebook: **100 / day**
   - Threads: **250 / day**
   - All other platforms (Bluesky, LinkedIn, TikTok, YouTube, Pinterest): **50 / day**
3. **Surface the changes** in pricing copy, plan comparison, FAQ, platform docs, and the in-app billing/limits UI — so customers and integrators understand both the gate and the protection.

---

## 3. Non-goals

- We are **not** introducing per-platform monthly caps (the existing workspace-month quota stays as-is).
- We are **not** changing the per-account *monthly* limit (`per_account_monthly_limit`) — it stays as a separate, optional, workspace-configurable safety belt on top of the new daily caps.
- We are **not** revoking already-connected X accounts on Free workspaces. Disconnecting OAuth tokens is destructive and out of scope; we only block **new publishes**.
- We are **not** making the daily caps customer-configurable in v1. They are static per-platform constants. A per-workspace override (e.g. for Enterprise customers with verified high-volume use cases) is a follow-up.
- We are **not** building a "burndown" UI in the dashboard for daily caps in v1. Caps surface only as error responses + pricing/docs copy. UI surfacing is a fast-follow.

---

## 4. Deliverable 1 — X paid-only

### 4.1 Schema

New migration: `0XX_plans_allow_twitter.sql`

```sql
ALTER TABLE plans
  ADD COLUMN allow_twitter boolean NOT NULL DEFAULT true;

UPDATE plans SET allow_twitter = false WHERE id = 'free';
```

Mirroring the `white_label` precedent. Default true so unknown rows (manual inserts, future plans) stay permissive — only `free` is explicitly false.

Regenerate sqlc to surface `Plan.AllowTwitter bool`.

### 4.2 Enforcement points

A new helper in `internal/quota`:

```go
// PlanAllowsPlatform returns false only when the workspace's plan
// explicitly disallows the requested platform. Unknown plans / platforms
// fall through to true so we never accidentally block a customer.
func (c *Checker) PlanAllowsPlatform(ctx context.Context, workspaceID, platform string) bool
```

Hook order (earliest reject wins):

1. **Validate endpoint** ([validate.go](/Users/xiaoboyu/unipost/api/internal/platform/validate.go)): if any per-result targets `twitter` and the workspace's plan disallows it, return a `PLAN_PLATFORM_NOT_ALLOWED` error in the validate response (no DB write, no API call). Same shape as existing validator errors so MCP / dashboard pre-flight catches it.
2. **Publish path** ([social_posts.go](/Users/xiaoboyu/unipost/api/internal/handler/social_posts.go) + [social_post_queue.go](/Users/xiaoboyu/unipost/api/internal/handler/social_post_queue.go)): re-check at dispatch time. If the gate trips here (plan was downgraded between draft and publish, etc.), the per-result row gets `error_code = PLAN_PLATFORM_NOT_ALLOWED` and is **not** counted against the workspace monthly quota.
3. **OAuth connect flow** ([connect_*.go](/Users/xiaoboyu/unipost/api/internal/handler/)): for Free workspaces attempting to connect a *new* X account, return the same error before consuming the OAuth code. Already-connected accounts stay connected.

### 4.3 Error contract

```json
{
  "error": {
    "code": "PLAN_PLATFORM_NOT_ALLOWED",
    "message": "Publishing to X (Twitter) requires a paid plan. Upgrade at unipost.dev/pricing.",
    "platform": "twitter",
    "current_plan": "free",
    "upgrade_url": "https://app.unipost.dev/projects/{id}/billing"
  }
}
```

Status code: **402 Payment Required** for the publish/validate endpoints; **403** for the connect endpoint (consistent with existing connect-side errors).

### 4.4 UI copy

- Dashboard "Connect Account" picker greys out the X tile for Free workspaces and shows an "Upgrade to enable" pill linking to billing.
- The compose flow's platform selector hides X from the picker for Free workspaces (rather than listing it disabled — picker UX bug we don't want to ship).
- Existing connected X accounts remain visible in the accounts list with an "Inactive on Free plan" badge.

---

## 5. Deliverable 2 — Per-platform daily caps

### 5.1 The cap table

Static, code-defined, single source of truth: a new file `internal/quota/per_platform_daily.go`.

```go
// PerPlatformDailyCap is the maximum number of successful publishes
// per (social_account_id, platform) in a UTC calendar day. Values are
// conservative defaults sized to stay below each platform's "looks
// like a spam bot" automation threshold while remaining well above
// realistic human-scale posting volume. They are NOT customer-tunable
// in v1 — see PRD §3 for rationale.
var PerPlatformDailyCap = map[string]int{
    "twitter":   20,
    "instagram": 100,
    "facebook":  100,
    "threads":   250,
    "bluesky":   50,
    "linkedin":  50,
    "tiktok":    50,
    "youtube":   50,
    "pinterest": 50,
}

const PerPlatformDailyCapDefault = 50 // fallback for unknown platforms
```

Same package as the existing per-account *monthly* tracker so both the daily and monthly trackers can be assembled together at request time.

### 5.2 Counting source

We already record successful publishes in `social_post_results.published_at` (UTC `timestamptz`). New sqlc query:

```sql
-- name: CountPublishedTodayByAccountAndPlatform :one
SELECT count(*)
FROM social_post_results
WHERE social_account_id = $1
  AND platform = $2
  AND published_at >= date_trunc('day', now() AT TIME ZONE 'UTC')
  AND published_at <  date_trunc('day', now() AT TIME ZONE 'UTC') + interval '1 day';
```

Index: a partial index on `(social_account_id, platform, published_at)` where `published_at IS NOT NULL`. Only added if EXPLAIN on production-shaped data shows the lookup is hot — most accounts publish well below cap and the count is bounded.

### 5.3 Tracker

Add `PerPlatformDailyTracker` next to `PerAccountTracker` in `internal/quota/`. Same shape, snapshot-once-per-request semantics, lock-protected map. Key is `(accountID, platform)` since one social_account_id is bound to a single platform but explicit keying makes the lookups self-documenting and survives any future "multi-platform account" weirdness.

```go
type PerPlatformDailyTracker struct {
    mu        sync.Mutex
    remaining map[string]int // "<accountID>|<platform>" → remaining today
}

func NewPerPlatformDailyTracker(
    ctx context.Context,
    queries *db.Queries,
    targets []DispatchTarget, // {AccountID, Platform}
) *PerPlatformDailyTracker

func (t *PerPlatformDailyTracker) Allow(accountID, platform string) bool
```

Integrates at the same points as `PerAccountTracker`:

- `social_posts.go` `processBulkOne` / immediate publish path: build tracker once at request top, call `Allow` before each dispatch.
- `social_post_queue.go:443` worker dispatch path: the worker rebuilds a one-account tracker per claimed job (same pattern as `buildPerAccountTracker`).

Order of checks (cheapest first, soft → hard):
1. Plan platform gate (§4.2) — purely in-memory plan lookup.
2. Per-platform daily cap (this section) — one DB count per unique (account, platform).
3. Per-account monthly cap (existing) — one DB count per unique account.
4. Workspace monthly billing quota (existing).

### 5.4 Error contract

```json
{
  "error": {
    "code": "PER_PLATFORM_DAILY_CAP_EXCEEDED",
    "message": "This Instagram account has reached its daily publish limit (100/day). Resets at 00:00 UTC.",
    "platform": "instagram",
    "account_id": "acc_…",
    "cap": 100,
    "resets_at": "2026-05-01T00:00:00Z"
  }
}
```

Status code: **429 Too Many Requests** with `Retry-After` set to seconds until next UTC midnight. Same mechanism as the existing rate-limit headers in [response.go:158](/Users/xiaoboyu/unipost/api/internal/handler/response.go).

### 5.5 Soft-cap acceptance

Same race-window acceptance as `PerAccountTracker` (per_account.go header comment): two API replicas dispatching to the same account in the same instant can collectively over-fire by a small amount. This is a **safety belt, not a billing-grade hard limit**. The cap exists to keep customers below platform spam thresholds; a single-digit over-shoot is well within the safety margin we're choosing (X's documented limit is far higher than 20, etc.). Documented inline.

### 5.6 What counts toward the cap

- ✅ Successful publishes (`social_post_results.published_at IS NOT NULL`).
- ❌ Failed publishes — never counted, by design. A platform-side failure should not consume the customer's safety budget.
- ❌ Drafts, scheduled-but-not-yet-fired posts, validate-only calls.
- ❌ Retries of an already-counted result (the `published_at` timestamp is set once on the original success).

### 5.7 Reset semantics

UTC midnight, not workspace-local. Documented in the error message and FAQ. Local-tz semantics would create a confusing "the cap reset 4 hours ago in my local tz but the API still says I'm over" experience and isn't worth the per-workspace tz lookup at dispatch time.

---

## 6. Deliverable 3 — Docs + Pricing updates

### 6.1 Pricing page — `dashboard/src/app/pricing/page.tsx`

Required edits:

- `FEATURES_FREE` (line 20): change `"All 8 platforms"` → `"7 platforms (X requires paid plan)"`. Keep the included flag true so the bullet still renders as a checkmark.
- `FEATURES_PAID` (line 28): keep `"All 8 platforms"` as-is.
- `COMPARE_ROWS` (line 36): the "All 8 platforms" row currently has `free: true, paid: true`. Split it: `free: "7 of 8"`, `paid: true`, and add a sub-line `"X requires paid plan"`.
- New compare row: `{ name: "Per-account daily safety caps", sub: "Protects accounts from spam flags — X 20/day, IG 100/day, FB 100/day, Threads 250/day, others 50/day", free: true, paid: true }`
- New FAQ entry:
  > **Q: Why are there per-account daily limits?**
  > A: To protect your customers' accounts from being flagged for spam by the platforms themselves. Each connected account has its own daily ceiling — X 20/day, Instagram 100/day, Facebook 100/day, Threads 250/day, and 50/day for Bluesky, LinkedIn, TikTok, YouTube, and Pinterest. Limits reset at 00:00 UTC and apply per connected account, so adding more accounts gives you more headroom. Failed posts never count toward the cap.
- New FAQ entry:
  > **Q: Why is X (Twitter) excluded from the Free plan?**
  > A: The X API has the highest per-call cost of any platform we support, and the Free plan's 100-post quota isn't large enough to absorb that cost without distorting our pricing for everyone else. Free workspaces can connect and read X accounts, but new X publishes require any paid plan ($10/mo and up).

### 6.2 Hero / "Permanent free trial" copy

Existing line in `pricing/page.tsx:55` ("Is there a free trial for paid plans?") references "100 posts/month with no credit card required." Add a parenthetical: "(X / Twitter publishing requires a paid plan.)"

Same surgical update in:
- `dashboard/src/data/competitors/zernio.ts:84`
- `dashboard/src/data/competitors/postforme.ts:88`
- `dashboard/src/data/competitors/ayrshare.ts:84`

### 6.3 Platform docs — `dashboard/src/app/docs/platforms/[platform]/_data.tsx`

For the `twitter` doc entry: add a `requirements` row `["Plan", "Paid plan ($10/mo or higher) — Free plans cannot publish to X"]`. The PlatformDoc type's `requirements` field already accepts arbitrary `[label, value]` pairs.

For every platform's `limitations` section: add a row `["Daily publish cap", "<N>/day per connected account (UTC reset)"]` with the corresponding cap value. Existing limitations table renders as a key/value grid; this slots in cleanly.

### 6.4 In-app billing & limits page

`dashboard/src/app/(dashboard)/projects/[id]/billing/page.tsx` and the API limits handler at `dashboard/src/app/(dashboard)/projects/[id]/posts/queue/page.tsx:196` already fetch `/v1/limits`. Extend that endpoint's response with a `per_platform_daily` block:

```json
{
  "per_platform_daily": {
    "twitter":   { "cap": 20,  "remaining_today": 14, "resets_at": "..." },
    "instagram": { "cap": 100, "remaining_today": 100, "resets_at": "..." },
    ...
  }
}
```

Render as a collapsed section under the existing rate-limit display. Counts are best-effort and refresh on mount; they don't have to be live.

### 6.5 Marketing landing pages

The `(marketing)` platform landing pages (per [project_platform_landing_pages](memory)) each list per-platform specs — those are publish-side capabilities (caption length etc.) and are NOT changing here. The only marketing edit is the X landing page hero, which gains a small "Paid plan required" badge near the CTA.

---

## 7. Implementation plan

Suggested PR sequence (each PR independently shippable / revertable):

1. **PR1 — schema + plan gate.**
   - Migration `0XX_plans_allow_twitter.sql`
   - sqlc regen, `Checker.PlanAllowsPlatform`
   - Wire into validate + publish + connect paths
   - Unit tests covering: free workspace blocked, paid workspace allowed, downgrade between draft and publish, validate-only call.
2. **PR2 — per-platform daily tracker.**
   - `internal/quota/per_platform_daily.go` with cap table + tracker
   - New sqlc query for daily count
   - Integrate at the same call sites as `PerAccountTracker`
   - Unit tests + race test mirroring `per_account_test.go`
   - Observability: increment a Prometheus counter `unipost_per_platform_daily_cap_hits_total{platform=...}` whenever a request is rejected.
3. **PR3 — error response shape + retry-after.**
   - Map both new error codes through the response helper
   - Include `Retry-After` header on 429 from the daily cap
   - Update API reference docs (`dashboard/src/app/docs/api/`).
4. **PR4 — pricing + FAQ + platform docs copy.**
   - All edits enumerated in §6
   - Visual regression check on pricing page
5. **PR5 — billing/limits UI surfacing.**
   - `/v1/limits` extension + dashboard render
   - Optional fast-follow if PR1–4 ship behind a feature flag.

PR1 + PR2 ship behind a single `enforce_platform_gates` env flag that defaults to **off in staging for one week**, then on. PR4 (copy) does NOT ship until the flag is on in production — we don't want to advertise a gate we aren't enforcing.

---

## 8. Migration / rollout plan

- **Day 0** — PR1 + PR2 land behind flag. Backfill check: for the prior 7 days, run a read-only report of how many (account, platform, day) tuples *would have* tripped each cap. Feeds the §10 review.
- **Day 7** — flip flag on in staging; smoke-test publish + validate flows for each platform.
- **Day 10** — flip flag on in production. Watch the new Prometheus counter, the publish error-rate dashboard, and `#oncall`.
- **Day 11** — ship PR4 (pricing + docs copy). Send a one-line in-app banner to Free workspaces with a connected X account: "Heads up — publishing to X now requires a paid plan. Upgrade or your X account will stay connected but read-only." Banner dismissible, no email blast for v1.
- **Day 21** — ship PR5 (limits UI).

If the backfill in Day 0 shows >2% of active accounts would trip the daily cap on any given day, hold rollout and revisit cap values before flipping the flag — that signals our cap numbers are too low for actual usage and need a calibration round before customers see 429s.

---

## 9. Observability

Counters / metrics to add:

- `unipost_per_platform_daily_cap_hits_total{platform=...}` — number of requests rejected by §5.
- `unipost_plan_platform_gate_hits_total{platform=...,plan=...}` — rejections by §4.
- `unipost_plan_platform_gate_oauth_blocked_total{platform=...,plan=...}` — connect-flow rejections.

Log fields on every cap hit: `workspace_id`, `social_account_id`, `platform`, `published_today`, `cap`. Goes to the existing structured-log pipeline; no new sink.

Dashboards: extend the existing rate-limit Grafana board, do not create a new one.

---

## 10. Open questions

1. **Cap calibration.** The 20/100/100/250/50 numbers track Zernio's published values 1:1. Before locking them in, run the §8 backfill; if any platform's 95th-percentile daily volume per-account is above 50% of the proposed cap, we should raise that cap (or the customer experience will be a lot of unexpected 429s).
2. **Enterprise overrides.** Out of scope per §3, but expected to come back. Where does the override live — `workspaces` row, `plans` row, or a new `workspace_platform_overrides` table? Decide before any Enterprise customer asks.
3. **Multi-account headroom.** A customer with 10 X accounts gets `10 × 20 = 200 posts/day` of X headroom. Is that the right framing for the FAQ, or do we want to also message a workspace-level X soft cap? Recommend: keep §6.1 framing as-is for v1, revisit if abuse appears.
4. **TikTok sandbox.** Per [project_tiktok_audit](memory), TikTok is still in sandbox pending the 2026-04-19 audit. The 50/day cap applies the moment TikTok goes live — no separate gating needed, since sandbox already throttles us harder than 50/day.
5. **X downgrade UX.** A workspace currently on a paid plan that downgrades to Free — do we show the X account as "inactive" (current spec) or hard-disconnect after 30 days? Current spec deliberately keeps it connected; revisit with billing.

---

## 11. Out of scope (explicit)

- Per-platform monthly caps
- Customer-configurable daily caps in v1
- Email notifications for cap hits (in-app error response only)
- Any change to TikTok / YouTube / Pinterest publishing flows beyond adding the daily cap
- Changes to the existing workspace `per_account_monthly_limit` field — it stays as the optional second belt
- Hard-disconnecting OAuth tokens for Free-plan X accounts

---

*Authoring notes: Mirrors the structure of [prd-rate-limit-and-queue-admission.md](/Users/xiaoboyu/unipost/docs/prd-rate-limit-and-queue-admission.md). Cap values match the Zernio public-facing trust signal for direct comparability; UniPost positioning in pricing copy treats the caps as a customer-protection feature rather than a usage restriction.*
