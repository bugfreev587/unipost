# Instagram Inbox Window Hotfix Design

## Problem

Production Instagram inbox replies can fail with Meta error `10/2534022`:
`This message is sent outside of allowed window.`

For the affected `robynsocial` account, Meta's Conversations API shows a valid
inbound message less than 24 hours before the failed send, but the account's
`/{ig-user-id}/subscribed_apps` edge is empty. UniPost stores the account after
OAuth without subscribing it to Instagram messaging webhooks. The dashboard
also incorrectly treats the 24-hour reply window as Facebook-only.

## Considered approaches

1. Add only a dashboard 24-hour guard.
   This prevents some known-expired sends but leaves Instagram accounts
   unsubscribed and cannot repair the production root cause.
2. Add only the Instagram webhook subscription.
   This repairs future inbound delivery but still lets known-expired threads
   make avoidable Meta requests and surfaces an opaque platform error.
3. Repair the subscription and enforce the window at both product boundaries.
   This fixes new and existing accounts, prevents avoidable sends, and gives a
   clear recovery instruction when Meta still rejects a thread.

Approach 3 is selected.

## Design

### Shared Instagram webhook subscriber

Add a focused backend package that performs the idempotent Meta request:

`POST /{ig-user-id}/subscribed_apps`

The subscribed fields are `messages`, `messaging_postbacks`, and `comments`.
The package accepts an HTTP client so the exact request and error handling can
be tested without calling Meta.

### New connections

After an Instagram account is saved during the hosted Connect callback, call
the shared subscriber before marking the Connect session complete. If Meta
rejects the subscription, mark the social account `reconnect_required` and
return a connect error instead of presenting a silently broken active account.

### Existing connections

The inbox sync worker already scans every active Instagram account. On the
first scan after each process start, it attempts the same idempotent webhook
subscription. Successful account IDs are cached in memory for the lifetime of
the process. Failures remain uncached and retry on the next five-minute scan.
Polling continues even if subscription repair fails.

This repairs existing accounts without storing new secrets, adding a database
migration, or making a Meta subscription request on every poll.

### Reply eligibility and error handling

Both `ig_dm` and `fb_dm` use the standard 24-hour reply window. The dashboard
will disable the composer when the latest inbound item is older than 24 hours
and show platform-neutral recovery copy.

The backend will also reject a DM reply when the selected inbound item is
already older than 24 hours. Meta error subcode `2534022` will be mapped to a
clear message telling the operator that Meta considers the window closed and
the Instagram user must send a new message.

No `HUMAN_AGENT` tag is added. It requires separate Meta approval and is not a
safe general fallback for automated or unrelated content.

## Testing

- Unit-test the shared subscriber request, success response, and failure body.
- Unit-test Connect callback success and failure behavior with a fake
  subscriber.
- Unit-test the inbox worker's once-per-process repair and retry behavior.
- Unit-test the dashboard reply-window helper for Instagram, Facebook, and
  non-DM sources.
- Run the full Go suite, dashboard source test, dashboard production build,
  and dashboard Playwright regression suite.

## Deployment verification

After staging and production deploy:

1. Confirm the API and dashboard deployments are successful.
2. Query the affected Instagram account's `subscribed_apps` edge and confirm
   the UniPost app is present with messaging fields.
3. Open the real inbox route and confirm expired Instagram threads disable the
   composer with the recovery message.
4. Confirm Meta `2534022` is shown as the new actionable error rather than the
   raw platform response.
5. If no post-subscription inbound DM exists, the affected correspondent must
   send one new Instagram message before a successful real send can be safely
   accepted as evidence.

