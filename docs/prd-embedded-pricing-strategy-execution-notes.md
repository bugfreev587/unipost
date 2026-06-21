# Embedded Pricing Strategy Execution Notes

This note captures implementation findings for `docs/prd-embedded-pricing-strategy.md` after Phase 1.

## Phase 1 Status

Phase 1 positioning and competitor-data work is implemented locally.

- Zernio competitor data now reflects the public pay-per-connected-account model verified on `2026-06-21`.
- `/pricing` includes an embedded-app section and no longer references Zernio's old `$9/mo` add-ons.
- `/compare` and `/alternatives/zernio` can render account-based free tier and per-account price labels.
- Evidence for the Zernio pricing claim lives in `docs/competitive-evidence/zernio-pricing-2026-06-21.md`.

## Existing Plan-Gate Pattern

Backend plan gates currently use these conventions:

- Feature gates return `402 PLAN_FEATURE_NOT_AVAILABLE`.
- X/Twitter connection gating returns `402 PLAN_PLATFORM_NOT_ALLOWED`.
- Missing auth/workspace context returns `401 UNAUTHORIZED` or `500 INTERNAL_ERROR` depending on route context.
- Validation failures return `422 VALIDATION_ERROR`.
- Existing records are preserved on downgrade; handlers block new creation instead of deleting rows.
- `quota.Checker` centralizes plan lookup and uses fail-open behavior when subscription or plan lookup fails.

Relevant files:

- `api/internal/handler/plan_gate.go`
- `api/internal/ws/handler.go`
- `api/internal/handler/connect_sessions.go`
- `api/internal/handler/platform_credentials.go`
- `api/internal/quota/checker.go`

Recommendation for Phase 2:

- Use `402 PLAN_FEATURE_NOT_AVAILABLE` for plan-cap admission failures, including Free API-key cap, webhook cap, connected-account cap, and managed-user cap.
- Do not introduce a product-plan cap for Connect Session creation attempts. Keep abuse protection and runtime rate limits separate from packaging.
- Keep `409` reserved for state conflicts such as duplicate resources or account ownership conflicts if such cases arise separately.
- Add reusable limit helpers to `quota.Checker`, matching `MaxProfilesForPlan`, `MaxMembersForPlan`, and `WhiteLabelPlatformLimit`.

## New Backend Work Required

The following PRD items are not existing config changes. They require SQL, generated db methods, handler admission checks, response copy, and tests.

| Limit | Likely admission point | Missing implementation |
| --- | --- | --- |
| Free API keys | `APIKeyHandler.Create` | count active, non-revoked keys by workspace |
| Free webhook endpoints | `WebhookSubscriptionHandler.Create` | count active or total webhooks by workspace; product decision needed |
| Free connected accounts | connect completion and possibly connect-session creation | count active managed social accounts by workspace/profile; decide whether BYO accounts count |
| Free managed users | connect completion and possibly connect-session creation | cap at `3` distinct completed `external_user_id` values; do not count created sessions |
| Connect Session abuse/rate monitoring | `ConnectSessionHandler.Create` and existing rate-limit surfaces | no packaging cap; add monitoring only if needed for abuse or operational pressure |
| API/Growth embedded limits | connect/account/user paths | exact managed-user/account allowances and hard vs soft behavior are undecided |

Connect Session creation should remain uncapped by monthly count. It can still perform preflight checks when a request would clearly exceed an already-reached Free managed-user or managed-account cap. The callback/completion path must repeat the cap check so multiple pending sessions cannot bypass enforcement.

## SQL/DB Gaps

Existing useful queries:

- `ListAPIKeysByWorkspace`
- `ListWebhooksByWorkspace`
- `ListManagedUsersByProfile`
- `CountManagedUsersByProfile`
- `CreateConnectSession`
- `MarkConnectSessionCompleted`

Likely new queries:

- `CountActiveAPIKeysByWorkspace`
- `CountActiveWebhooksByWorkspace`
- `CountActiveManagedAccountsByWorkspace`
- `CountManagedUsersByWorkspace`
- Optional instrumentation query for Connect Session volume by workspace/period

If limits are scoped by profile instead of workspace, provide profile-scoped variants too. The pricing PRD reads as workspace-plan enforcement, so workspace-scoped counting is the safer default unless product chooses otherwise.

## Product Decisions Resolved Before Phase 2 Code

1. Free connected-account cap: `2` managed connected accounts.
2. Free managed-user cap: `3` distinct completed `external_user_id` values.
3. Free webhook cap basis: active webhooks only.
4. BYO/dashboard-connected accounts do not count toward the Free managed connected-account cap in Phase 2.
5. API and Growth get no hard managed-user, managed-account, or Connect Session creation cap in Phase 2.
6. Phase 2 Free-plan enforcement ships without a feature flag.

## Recommended Approval Set

If the team wants the lowest-risk default, approve this set:

| Decision | Recommendation | Rationale |
| --- | --- | --- |
| Free connected-account cap | `2` managed connected accounts | Matches the Zernio comparison mental model while keeping Free as evaluation-only. This should be framed as an evaluation boundary, not a pricing meter. |
| Free managed-user cap | `3` distinct completed `external_user_id` values | Counts real managed end users, not abandoned sessions. This keeps Free evaluation useful while nudging customer-facing embedded usage to paid plans. |
| Free Connect Sessions cap | No hard cap on create attempts | Developers may create many sessions during integration before a successful OAuth connection. Monitor abuse/rate-limit patterns instead of treating created sessions as a plan allowance. |
| Free API-key cap | `1` active, non-revoked API key | Simple evaluation boundary. Existing keys should stay valid; only new key creation is blocked above cap. |
| Free webhook cap | `1` active webhook endpoint | Lets developers test webhook delivery while keeping production fan-out on paid plans. Count active webhooks, not historical/revoked rows. |
| BYO/dashboard accounts in Free cap | Do not count BYO/dashboard-connected accounts in the managed connected-account cap for Phase 2 | The PRD's competitive story is about embedded customer-facing Connect. Keep dashboard evaluation friction low unless cost data shows BYO accounts are the problem. |
| API managed-user/account/session behavior | Soft warning and upgrade guidance, no hard cap in Phase 2; no hard cap on Connect Session create attempts | API is paid and should support light production. Hard API caps can be added after instrumentation shows cannibalization. |
| Growth managed-user/account/session behavior | High fair-use threshold, no hard cap in Phase 2; no hard cap on Connect Session create attempts | Growth is the embedded self-serve plan. Start with observability and Enterprise escalation before enforcing hard ceilings. |
| Feature flag | No flag for Phase 2 | Product decision: keep the implementation simple; validate in dev and rollback by reverting code if needed. |

This approval set lets Phase 2 ship only Free-plan enforcement. API/Growth differentiation remains copy, instrumentation, and upgrade guidance until real usage data supports hard caps.

## Suggested Phase 2 Implementation Shape

After decisions are made:

1. Add plan limit helpers to `quota.Checker`.
2. Add required count queries under `api/internal/db/queries`.
3. Regenerate sqlc outputs using the repo's existing generation flow.
4. Add admission checks to connect-session preflight where the cap is already known to be exhausted, and to connect completion where the successful managed account is persisted.
5. Preserve existing over-limit records and block only new creation.
6. Return `402 PLAN_FEATURE_NOT_AVAILABLE` with upgrade-oriented messages.
7. Add backend tests for each new admission check.
8. Surface limits in dashboard/API only after the backend behavior is settled.
