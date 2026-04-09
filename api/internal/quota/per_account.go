package quota

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// ErrPerAccountQuotaExceeded is the deterministic error the publish
// path records on a per-result row when the project's
// per_account_monthly_limit caps further dispatches for that account
// in the current calendar month. Customers see this string in the
// API response and the dashboard, so it's deliberately verbose about
// the cause and the remedy.
var ErrPerAccountQuotaExceeded = errors.New(
	"per_account_monthly_quota_exceeded: this social account has hit the project's per-account monthly publish limit; raise the limit on the project or wait for the next calendar month",
)

// PerAccountTracker is a per-request, in-memory budget for per-account
// dispatches. It snapshots "remaining publishes for this month" once
// per unique social_account_id at the top of a publish request, then
// atomically decrements as each dispatch fires.
//
// Why per-request and not global: the project-level Checker already
// serves as the project-wide budget. This tracker is the per-account
// belt that prevents one runaway end-user account from eating the
// project budget, and it's bounded to the request that triggered it —
// once the request returns, the next publish request re-fetches the
// count from social_post_results.published_at.
//
// Race scope: protects against parallel dispatch groups within ONE
// request hitting the same account_id at the same instant (threads
// run serial within a group, but groups run parallel). It does NOT
// protect against two different API replicas concurrently dispatching
// to the same account — that's a soft-cap acceptance documented in
// Sprint 5 PR2 PRD. The cap is a runaway-script safety belt, not a
// billing-grade hard limit; the next request will see the updated
// DB count and clamp itself.
//
// limit==nil on the tracker means "the project has no per-account
// cap configured" — every Allow returns true and the publish path
// bypasses enforcement entirely. This keeps the legacy zero-config
// behavior intact for projects that haven't set the field.
type PerAccountTracker struct {
	mu        sync.Mutex
	limit     *int           // nil = unlimited (no cap configured)
	remaining map[string]int // social_account_id → remaining for current month
}

// NewPerAccountTracker builds a tracker for one publish request. Pass
// project.PerAccountMonthlyLimit (the pgtype.Int4 from the loaded
// project row) and the deduped list of account ids the request is
// going to dispatch to. Counts are loaded once, up front, so the
// dispatch loop doesn't pay a DB round-trip per dispatch. Returns a
// tracker that's safe to share across goroutines.
//
// If limit is NULL/invalid, returns an unlimited tracker that
// short-circuits Allow() — no DB queries are made.
func NewPerAccountTracker(
	ctx context.Context,
	queries *db.Queries,
	limit pgtype.Int4,
	accountIDs []string,
) *PerAccountTracker {
	if !limit.Valid {
		return &PerAccountTracker{}
	}
	cap := int(limit.Int32)
	t := &PerAccountTracker{
		limit:     &cap,
		remaining: make(map[string]int, len(accountIDs)),
	}
	for _, id := range accountIDs {
		// Best-effort: a count failure shouldn't block publishing —
		// we degrade to "unknown current usage, allow this request"
		// rather than 500ing. The next request will see the right
		// number once the DB recovers. The publish path's other
		// safety nets (validator, adapter errors) still apply.
		count, err := queries.CountPublishedThisMonthByAccount(ctx, id)
		if err != nil {
			t.remaining[id] = cap // assume zero used on lookup failure
			continue
		}
		left := cap - int(count)
		if left < 0 {
			left = 0
		}
		t.remaining[id] = left
	}
	return t
}

// Allow atomically checks-and-decrements the per-account budget.
// Returns true if the dispatch can proceed (and consumes one slot),
// false if the account has exhausted its monthly budget.
//
// Unlimited trackers (limit==nil) always return true without
// touching the map.
func (t *PerAccountTracker) Allow(accountID string) bool {
	if t.limit == nil {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	left, ok := t.remaining[accountID]
	if !ok {
		// An account that wasn't in the snapshot (e.g. a thread that
		// references an account the dedup list missed) gets a fresh
		// budget of `limit` for this request. Defensive — the dedup
		// upstream should always include every account.
		left = *t.limit
	}
	if left <= 0 {
		t.remaining[accountID] = 0
		return false
	}
	t.remaining[accountID] = left - 1
	return true
}
