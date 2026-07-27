# Publishing restriction email campaign runbook

## Unknown send outcomes

An unknown send outcome means UniPost attempted a provider send but could not determine the provider request outcome. The provider may already have accepted and delivered the email. The recipient is terminally failed with `retryable = false`; the worker must not retry it automatically.

This conservative behavior creates more failures that require manual review. It is intentional: an extra manual failure is safer than automatically sending a duplicate service-alert email.

## Owner snapshots across migration 126

Recipient owner/workspace pairs are immutable audience evidence. New application versions write both arrays explicitly. During a rolling deployment, migration 126 also supports an old binary that omits owner IDs on INSERT: for each represented workspace, it records the unique current active owner only when that owner's normalized email exactly matches the recipient snapshot. Missing or ambiguous evidence falls back to the canonical user and fails closed during later eligibility checks.

Rows created before migration 126 retain the conservative canonical-user fallback. Do not rewrite those historical pairs from current workspace membership: a current matching owner may represent a later email or ownership takeover rather than the owner at snapshot time. If the stored pair cannot pass current eligibility, leave it skipped. Recovery Preview may therefore report no eligible recipients for an ambiguous historical row; Preview does not issue a provider send.

## Investigate a terminal failure

Before deciding the disposition, inspect all of the following for the individual recipient:

1. The campaign recipient row, including status, `last_error`, `attempt_generation`, normalized email, and stable `idempotency_key`.
2. The linked `email_send_attempts` row and its attempt-specific audit key.
3. Provider request and delivery logs for the normalized recipient email and the same stable provider idempotency key.
4. Provider delivery, bounce, and suppression evidence for the recipient.

The stable provider idempotency key has the form:

```text
<cycle_id>:<campaign_type>:<canonical_user_id>
```

It remains unchanged across an operator-triggered retry. The local audit key changes with the attempt generation and attempt count, for example from `…:g1:a1` to `…:g2:a1`.

The stable provider key is defense in depth, not proof that an earlier request was rejected or that a retry cannot duplicate delivery. Provider retention windows and idempotency semantics may differ from UniPost's campaign lifetime.

## Retry decision

The retry-failed campaign action retries only recipients with definitive provider-failure or pre-send-failure evidence. It permanently excludes unknown or ambiguous provider outcomes, even when the same campaign also contains retry-safe failures.

There is currently no recipient-level override for a confirmed unknown outcome. Record provider evidence in the incident or operator log and leave the recipient terminally failed; a future recipient-level workflow must require explicit evidence and approval before it can send. If provider evidence is unavailable or ambiguous, escalate for manual review.
