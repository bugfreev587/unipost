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
//
// Limit / Remaining / ResetUnix / QueueDepth / QueueCap are
// optional state-snapshot fields populated by Redis-backed limiters
// (request token bucket and Postgres depth checker today). They
// drive the X-UniPost-RateLimit-* response headers — when zero,
// the handler omits the header rather than emitting a misleading
// "0 remaining" value. NoopLimiter leaves them all zero so degraded
// installs simply skip the headers.
type Decision struct {
	Allowed    bool
	RetryAfter time.Duration
	Reason     string

	// Limit is the burst capacity / window cap exposed to the
	// client (e.g. 20 for p10 request bucket).
	Limit int

	// Remaining is how many units the workspace can still consume
	// before being denied. For token bucket this is current tokens
	// floor()'d to int.
	Remaining int

	// ResetUnix is the unix-seconds timestamp at which the bucket
	// (or window) returns to full / empty respectively. Zero means
	// "no useful reset hint" and the header is omitted.
	ResetUnix int64

	// QueueDepth / QueueCap describe the workspace's active
	// delivery-job count and its plan cap. Populated by the depth
	// limiter; both zero ⇒ skip the X-UniPost-QueueDepth header.
	QueueDepth int
	QueueCap   int
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
