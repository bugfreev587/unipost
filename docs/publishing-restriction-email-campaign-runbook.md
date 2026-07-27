# Publishing restriction email campaign runbook

## Unknown send outcomes

An unknown send outcome means UniPost made a provider request but did not receive a response. The provider may already have accepted and delivered the email. The recipient is terminally failed with `retryable = false`; the worker must not retry it automatically.

This conservative behavior creates more failures that require manual review. It is intentional: an extra manual failure is safer than automatically sending a duplicate service-alert email.

## Investigate a terminal failure

Before retrying, inspect all of the following for the individual recipient:

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

Use the retry-failed campaign action only after provider evidence confirms the unknown attempt was not delivered or accepted for delivery. Record the supporting provider evidence in the incident or operator log before retrying.

Never bulk retry unknown-outcome recipients without recipient-by-recipient provider confirmation. If provider evidence is unavailable or ambiguous, leave the recipient terminally failed and escalate for manual review.
