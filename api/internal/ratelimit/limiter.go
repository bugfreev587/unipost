// Package ratelimit implements UniPost's runtime admission control:
// per-workspace request limiter, enqueue-throughput limiter, and
// queue-depth limiter. See docs/prd-rate-limit-and-queue-admission.md
// for the design rationale.
//
// The package exposes one Limiter interface that handlers call. The
// production wiring (NewRedisLimiter) composes a Redis-backed token
// bucket for request limiting, a Redis-backed sliding window for
// enqueue limiting, and a Postgres-backed depth check. Local dev
// without a Redis URL falls back to NoopLimiter so handlers don't
// need to special-case missing infra.
package ratelimit

import (
	"context"
	"time"
)

// Decision is the result of an admission check. RetryAfter is the
// hint the handler writes into the Retry-After response header — for
// allowed requests it is zero. Reason is for structured logging /
// metrics, never user-visible.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
	Reason     string
}

// RequestScope identifies one admission attempt against the per-workspace
// request limiter. Route is recorded as a metric label only; it is
// deliberately not part of the Redis key (per §9.2 of the PRD —
// splitting by route would let traffic spread across endpoints to
// bypass the cap).
type RequestScope struct {
	WorkspaceID    string
	PlanID         string
	ExternalUserID string
	Route          string
}

// EnqueueScope identifies one admission attempt against the enqueue
// throughput limiter. Units is the number of posts the request is
// trying to accept (1 for a single create / publish / retry; the
// post count for a batch).
type EnqueueScope struct {
	WorkspaceID    string
	PlanID         string
	ExternalUserID string
}

// QueueScope identifies one admission attempt against the queue depth
// limiter. AddedUnits is the number of new active delivery jobs the
// request will create if admitted.
type QueueScope struct {
	WorkspaceID    string
	PlanID         string
	ExternalUserID string
}

// Limiter is the admission-control surface handlers depend on. All
// three methods are safe to call when the underlying infrastructure
// is missing — implementations either fall back to a degraded mode
// (circuit breaker open) or return an Allowed decision (Noop).
type Limiter interface {
	// AllowRequest checks the per-workspace token bucket for a write
	// API hit. Cheap; called first in the handler flow.
	AllowRequest(ctx context.Context, scope RequestScope) (Decision, error)

	// AllowEnqueue checks the per-workspace sliding window for accepted
	// work volume. Units is the post count this request would admit.
	AllowEnqueue(ctx context.Context, scope EnqueueScope, units int) (Decision, error)

	// CheckQueueDepth verifies that admitting addedUnits new active
	// delivery jobs would not push the workspace past its depth cap.
	// v1 uses a best-effort Postgres count; see pg_depth.go for the
	// known TOCTOU caveat that Phase 1.5 will close.
	CheckQueueDepth(ctx context.Context, scope QueueScope, addedUnits int) (Decision, error)
}
