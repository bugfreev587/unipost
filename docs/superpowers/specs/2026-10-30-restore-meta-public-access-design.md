# Restore Meta Public Access Design

## Goal

Restore public Facebook Pages, Instagram, and Threads account connection and publishing after the Railway credential incident, while keeping TikTok unavailable and excluding Facebook Messenger until Meta grants `pages_messaging` advanced access.

## Current state

- The Meta App is unpublished.
- Instagram and Threads already appear for regular users and their approved OAuth scopes remain wired.
- Facebook is hidden from regular users in the dashboard and rejected by API, callback, pending-connection, and Page-insights super-admin gates.
- Meta has approved or renewed the Facebook Page publishing, engagement, metadata, Page listing, and analytics scopes used by UniPost.
- Meta has not approved `pages_messaging`; requesting it during public OAuth can prevent regular users from completing Facebook authorization.
- The production platform-publishing-restriction table has no active restriction for Facebook, Instagram, or Threads. TikTok remains operationally separate and is outside this hotfix.

## Selected approach

Use a narrowly scoped hotfix based on the latest `origin/staging`:

1. Make Facebook Pages an always-visible connection option alongside Instagram and Threads.
2. Remove only the Facebook-specific super-admin gates from OAuth initiation, callback, pending Page selection/finalization, and Page insights.
3. Keep the shared super-admin checker for unrelated internal controls and free-plan account-sharing enforcement.
4. Stop requesting `pages_messaging` in both dashboard OAuth and hosted Connect flows, while retaining it in the canonical full-permission inventory for future restoration after approval.
5. Do not change Instagram, Threads, or TikTok behavior.
6. Validate locally, in staging, and in production. Publish the Meta App only after the production code can serve regular-user OAuth safely.

## Alternatives considered

### Wait for `pages_messaging` approval

This preserves Messenger in the first public release but prolongs the outage for approved connection and publishing features. Rejected because the user prioritized restoring account connection and posting now.

### Add all customers to `SUPER_ADMINS`

This avoids code changes but grants unrelated internal privileges and leaves Facebook coupled to an audit-only gate. Rejected as unsafe.

### Publish the Meta App without changing UniPost

Instagram and Threads could become reachable, but Facebook would remain hidden and blocked; Facebook OAuth would also request an unapproved permission. Rejected as incomplete and failure-prone.

## Code boundaries

- `dashboard/src/app/(dashboard)/projects/[id]/accounts/page.tsx`: present Facebook Pages to every authenticated user.
- `dashboard/src/lib/features-in-dev.ts` and `dashboard/src/components/dashboard/shell.tsx`: remove the retired Facebook audit-only feature marker/helper.
- `api/internal/handler/oauth.go`: allow regular users to start and complete Facebook OAuth.
- `api/internal/handler/oauth_facebook.go`: remove the callback-only super-admin helper.
- `api/cmd/api/main.go`: remove Facebook-only middleware from Page insights and pending-connection routes.
- `api/internal/platform/facebook.go`: separate approved public OAuth scopes from the future Messenger scope.
- `api/internal/connect/facebook.go`: omit the unapproved Messenger scope from hosted Connect.
- Focused tests will assert public visibility, absence of the Facebook gates, and absence of `pages_messaging` from generated OAuth URLs.

## Safety and rollback

- Existing disconnected accounts remain disconnected; users must complete a new OAuth grant.
- No old token is restored or reused.
- TikTok remains unchanged.
- If staging OAuth or publishing fails, stop before production and leave the Meta App unpublished.
- If production fails after release, unpublish the Meta App and revert the hotfix through the normal staging-to-main path.

## Acceptance criteria

- A non-super-admin user can see Facebook, Instagram, and Threads in Connections.
- A non-super-admin can start and complete Facebook OAuth and finalize Page selection.
- Facebook OAuth requests approved publishing/engagement/analytics scopes but not `pages_messaging`.
- Instagram and Threads OAuth start and callback behavior remains unchanged.
- A newly reconnected account can publish a test post on each of Facebook, Instagram, and Threads.
- TikTok remains unavailable under the existing incident policy.

