# PR #270 Review Hardening Design

## Goal

Harden the stacked PR #270 changes against migration drift, terminal email-audit regression, duplicated TikTok publication after a persisted publish token, and untested PostgreSQL locking behavior.

## Migration compatibility

Migration 124 is treated as immutable deployed history. Its Up statements remain byte-for-byte unchanged so a persistent Preview that already recorded version 124 and a fresh environment continue through the same historical sequence. Its Down section will document that the `media_post_usages.retention_reason` data rewrite is intentionally irreversible.

A new migration 125 will correct every existing publishing-restriction email recipient whose status is `failed` to `retryable = FALSE`. This is deliberately unconditional with respect to the current `retryable` value, so databases that already ran migration 124 and fresh databases converge. The migration's Down path will not reactivate failed recipients because the pre-125 retryability state cannot be reconstructed safely.

An upgrade integration test will execute the real migrations 122, 124, and 125 over a minimal compatible PostgreSQL baseline, seed a failed recipient between 122 and 124, and prove the row becomes non-retryable only after 125. It will also prove the candidate claim path does not return that row.

## Email audit terminal state

`CreateEmailSendAttemptAudit` will keep its existing upsert behavior for `pending` and `failed` rows: retry metadata is refreshed, the attempt count advances, and the row returns to `pending`. When the provider/idempotency key already identifies a `sent` row, every terminal field and snapshot remains unchanged, including `status`, `sent_at`, payload snapshots, attempt count, and provider-related metadata. The query still returns the existing row so current callers retain their API contract.

A PostgreSQL behavior test will create a sent row, replay the same provider key with conflicting payload, and compare the persisted terminal record before and after the replay. A companion failed-row case will prove retry behavior remains available.

## Persisted TikTok publish-token resume

The H1 test will drive the real `SocialPostHandler.ProcessPostDeliveryJob` method. A database fake supplies an active TikTok delivery job, the post/result/account state, and a result with a persisted publish token. A restriction evaluator returns restricted if invoked. A registered TikTok spy adapter observes publish options and simulates the resume/poll boundary.

The test asserts that the restriction evaluator and restricted-finalization writes are not invoked, the adapter receives `OptResumePublishToken`, and no TikTok init/upload operation occurs. The job may proceed only through the existing resume path.

## PostgreSQL transaction integration

The repository currently has environment-gated PostgreSQL tests that skip when a URL is absent, while the main CI job provides no PostgreSQL service. The new required integration suite will use a dedicated GitHub Actions job with an isolated PostgreSQL service and a build tag. The tagged tests fail—not skip—when their explicit test database URL is absent.

Each test creates and drops a unique database or schema owned by that CI service, never using UniPost dev, staging, production, or the shared Railway URL. One test holds the recipient lock in transaction 1 and proves transaction 2's `FOR UPDATE SKIP LOCKED` claim skips it. Another makes a `sending` recipient stale, invokes the real store claim operation, verifies terminal `failed/retryable=FALSE`, and proves a later claim cannot retrieve it.

## Validation and delivery

Focused RED/GREEN commands precede full API and Dashboard checks. The final branch is pushed only after required local validation succeeds. A Draft stacked pull request targets `codex/staging-tiktok-free-publishing-restriction`; it is not merged or deployed, and its description states that its commit must first be applied to PR #270's head branch for renewed review.
