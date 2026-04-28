package ratelimit

import (
	"context"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// depthLimiter is the queue-depth safety belt. It prevents one
// workspace from exceeding its plan's active-job cap by counting
// rows in post_delivery_jobs.{pending,running,retrying} before each
// admission and rejecting if count + addedUnits exceeds the cap.
//
// v1 implementation note (Phase 1.5 hardens this): the count and
// the subsequent insert in the handler are NOT in the same
// transaction, so two API replicas can race past the cap by up to
// `replica_count * units_per_request` rows. This is acceptable for
// v1 because:
//   - the request limiter and enqueue limiter (both atomic in
//     Redis) are the primary protections against bursts
//   - the depth check is the safety belt for the slower failure
//     mode where workers are backed up; tiny TOCTOU slack does
//     not meaningfully change the protection
//   - pulling the entire publish-path insert into one transaction
//     for the sake of the lock would be a non-trivial refactor
//     that we'd rather not stack on top of the rate-limit PRD
//
// Phase 1.5 adds pg_try_advisory_xact_lock around a tx-bound
// count + insert sequence, closing the slack. See PRD §6.3.
type depthLimiter struct {
	queries *db.Queries
}

func newDepthLimiter(queries *db.Queries) *depthLimiter {
	return &depthLimiter{queries: queries}
}

func (l *depthLimiter) Check(ctx context.Context, scope QueueScope, addedUnits int) (Decision, error) {
	limits := LimitsForPlan(scope.PlanID)
	cap := limits.WorkspaceQueueDepthCap
	if cap <= 0 {
		// Treat zero/negative cap as "no limit configured" so a
		// misconfigured plan does not silently lock out a workspace.
		return Decision{Allowed: true}, nil
	}

	count, err := l.queries.CountActiveDeliveryJobsByWorkspace(ctx, scope.WorkspaceID)
	if err != nil {
		// Counting failed — conservative choice is to allow rather
		// than break publish on a transient DB hiccup. The request
		// limiter and enqueue limiter still apply, so we are not
		// completely unprotected.
		return Decision{Allowed: true, Reason: "depth_count_failed"}, err
	}

	depth := int(count)
	dec := Decision{
		QueueDepth: depth,
		QueueCap:   cap,
	}
	if depth+addedUnits > cap {
		dec.Allowed = false
		dec.RetryAfter = 30 * time.Second
		dec.Reason = NormQueueDepthExceeded
		return dec, nil
	}
	dec.Allowed = true
	return dec, nil
}
