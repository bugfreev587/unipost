# Email Preferences V1 Implementation Plan

## Scope

Implement the email notification preferences PRD through phases 1-4 and skip phase 5.

In scope:

- Registry-owned email policy metadata, footer policy, category mapping, and delivery class lookup.
- User email preference categories with backend API and settings UI.
- Send-time gating for Loops-owned service alerts, especially `post.failed` and `account.disconnected`.
- Server-provided footer variables for Loops transactional templates.
- Admin email visibility for preference category, footer policy, and decision.
- Dev, staging, and production verification after release.

Out of scope:

- Public one-click unsubscribe routes and signed unsubscribe tokens.
- Marketing/product update campaigns.
- Clerk-owned authentication email changes.

## Expected User Outcome

Users can manage optional UniPost email notification categories from `/settings/notifications`. Turning off publishing failure emails prevents future `post.failed` Loops emails. Turning off account connection emails prevents future `account.disconnected` Loops emails. Required transactional emails still send and explain why they cannot be disabled via footer variables.

## Implementation Steps

1. Add failing backend tests for registry metadata, footer variables, preference decisions, and disabled service-alert sends.
2. Add a user-scoped `email_preferences` table, sqlc queries, and a one-time backfill from existing email notification subscriptions.
3. Extend `emailregistry` with delivery classes, category metadata, footer policy, gating fields, and Loops event lookup helpers.
4. Add an `emailpolicy` package that prepares send decisions and server-rendered footer variables.
5. Wire `emailpolicy` into Loops lifecycle sends before calling Loops. Skipped sends should write an audit row with status `skipped` and reason `preference_disabled`.
6. Add `/v1/me/notifications/email-preferences` list/update endpoints.
7. Update `/settings/notifications` so email category toggles are separate from the Slack/Discord event matrix.
8. Add admin email response fields for category, footer policy, and preference decision.
9. Run local validation on the task branch, merge to local `dev`, rerun validation, push `origin/dev`, then verify dev, staging, and production environments.

## Validation

- `cd api && sqlc generate`
- `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/emailregistry ./internal/emailpolicy ./internal/loops ./internal/handler`
- `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`
- `cd dashboard && npm run build`
- `cd dashboard && npm run test:regression:dashboard` when Playwright browsers are installed

## Production Acceptance

- In production `/settings/notifications`, email preferences appear as category toggles and email is not a separate source of truth in the Slack/Discord matrix.
- In production admin email history, email rows expose enough policy metadata to explain whether an email was required, preference-gated, or skipped.
- In production, the application loads normally after the release and the critical notification settings flow remains healthy.
