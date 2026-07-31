package quota

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// ErrPerPlatformDailyCapExceeded is the deterministic error the publish
// path records on a per-result row when the (social_account_id,
// platform) tuple would breach its daily safety cap. Customers see
// this string in the API response and the dashboard, so it spells out
// what tripped, why the cap exists, and when the count resets.
var ErrPerPlatformDailyCapExceeded = errors.New(
	"per_platform_daily_cap_exceeded: this account has hit its safe daily publish limit — caps reset at 00:00 UTC and exist to keep the connected account from being flagged for spam by the platform",
)

// PerPlatformDailyCap is the maximum number of successful publishes
// per social account in a UTC calendar day, keyed by platform name.
// Values are conservative defaults sized to stay below each platform's
// "looks like a spam bot" automation threshold while remaining well
// above realistic human-scale posting volume — the bar is "protect the
// customer's account from being flagged", not "match the platform's
// theoretical API ceiling".
//
// Mirrors Zernio's published trust-signal numbers 1:1 (X 20, IG 100,
// FB 100, Threads 250, all others 50) so a customer comparing the two
// products sees an apples-to-apples table.
//
// Not customer-tunable in v1 — see PRD §3 for the rationale and
// follow-up plan.
var PerPlatformDailyCap = map[string]int{
	"twitter":   20,
	"instagram": 100,
	"facebook":  100,
	"threads":   250,
	"bluesky":   50,
	"linkedin":  50,
	"tiktok":    50,
	"youtube":   50,
	"pinterest": 50,
}

// PerPlatformDailyCapDefault is the fallback applied when a platform
// is not present in the cap map. Any platform we don't recognize gets
// the conservative "Other platforms" tier from the public docs, so a
// new adapter wired up before this map is updated still has a safety
// belt rather than going uncapped.
const PerPlatformDailyCapDefault = 50

// CapForPlatform returns the daily cap for a given platform, falling
// back to PerPlatformDailyCapDefault for unknowns.
func CapForPlatform(platform string) int {
	if c, ok := PerPlatformDailyCap[platform]; ok {
		return c
	}
	return PerPlatformDailyCapDefault
}

// PerPlatformDailyTracker reserves a durable provider-account daily budget.
// Production calls are serialized by a conditional database upsert; the local
// maps remain only as a no-database fallback for focused unit tests.
//
// All trackers carry a non-empty cap map (built per-request from
// PerPlatformDailyCap). A nil tracker short-circuits Allow() to true
// — used by tests and any future code path that wants to opt out.
type PerPlatformDailyTracker struct {
	mu            sync.Mutex
	ctx           context.Context
	queries       dailyReservationStore
	workspaceID   string
	remaining     map[string]int // test/failover fallback budget
	cap           map[string]int // physical account ID → platform cap
	newlyReserved map[string]bool
	reserved      map[string]bool
}

type dailyReservationStore interface {
	ReservePhysicalDailyPublish(context.Context, db.ReservePhysicalDailyPublishParams) (int32, error)
	ReleasePhysicalDailyPublish(context.Context, db.ReleasePhysicalDailyPublishParams) (bool, error)
	FinalizePhysicalDailyPublish(context.Context, db.FinalizePhysicalDailyPublishParams) (bool, error)
}

// PerPlatformDailyTarget is one (account_id, platform) tuple the
// caller plans to dispatch to. The tracker uses the platform to look
// up the cap and the account_id to load the snapshot count.
type PerPlatformDailyTarget struct {
	AccountID string
	Platform  string
}

// NewPerPlatformDailyTracker builds a tracker for one publish request.
// Pass the deduped physical (account, platform) pairs the request will dispatch
// to. Reservation errors fail open to preserve the existing publishing path;
// reaching the cap (pgx.ErrNoRows) fails closed for that target.
func NewPerPlatformDailyTracker(
	ctx context.Context,
	queries dailyReservationStore,
	workspaceID string,
	targets []PerPlatformDailyTarget,
) *PerPlatformDailyTracker {
	t := &PerPlatformDailyTracker{
		ctx:           ctx,
		queries:       queries,
		workspaceID:   workspaceID,
		remaining:     make(map[string]int, len(targets)),
		cap:           make(map[string]int, len(targets)),
		newlyReserved: make(map[string]bool, len(targets)),
		reserved:      make(map[string]bool, len(targets)),
	}
	for _, tgt := range targets {
		if _, seen := t.cap[tgt.AccountID]; seen {
			continue
		}
		cap := CapForPlatform(tgt.Platform)
		t.cap[tgt.AccountID] = cap
		t.remaining[tgt.AccountID] = cap
	}
	return t
}

// Allow atomically checks-and-decrements the daily budget for one
// dispatch. Returns true if the dispatch can proceed (and consumes
// one slot), false if the account has already hit its daily cap.
//
// Nil receivers are safe and always return true — used by tests
// and the optional fallback path. Unknown account_ids (defensive
// corner case) get a fresh budget sized for the named platform.
func (t *PerPlatformDailyTracker) Allow(accountID, platform, operationKey string) bool {
	if t == nil {
		return true
	}
	cap := CapForPlatform(platform)
	if configuredCap, ok := t.cap[accountID]; ok {
		cap = configuredCap
	}
	if t.queries != nil && t.workspaceID != "" {
		result, err := t.queries.ReservePhysicalDailyPublish(t.ctx, db.ReservePhysicalDailyPublishParams{
			WorkspaceID:       t.workspaceID,
			PhysicalAccountID: accountID,
			Platform:          platform,
			OperationKey:      operationKey,
			DailyCap:          int32(cap),
		})
		if err != nil {
			return true
		}
		if result == 2 {
			t.mu.Lock()
			t.newlyReserved[operationKey] = true
			t.reserved[operationKey] = true
			t.mu.Unlock()
		} else if result == 1 {
			t.mu.Lock()
			t.reserved[operationKey] = true
			t.mu.Unlock()
		}
		return result > 0
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	left, ok := t.remaining[accountID]
	if !ok {
		// Defensive: account wasn't in the snapshot. Snapshot upstream
		// should always cover every dispatch target — if we get here,
		// give the dispatch a fresh budget rather than crashing.
		t.cap[accountID] = cap
		left = cap
	}
	if left <= 0 {
		t.remaining[accountID] = 0
		return false
	}
	t.remaining[accountID] = left - 1
	return true
}

// Release removes only a slot created by this tracker invocation. A retry that
// reused an earlier outcome-unknown reservation must not release that older
// safety record when its new attempt fails definitively.
func (t *PerPlatformDailyTracker) Release(operationKey string) error {
	if t == nil || t.queries == nil || t.workspaceID == "" {
		return nil
	}
	t.mu.Lock()
	created := t.newlyReserved[operationKey]
	t.mu.Unlock()
	if !created {
		return nil
	}

	// Settlement must survive request cancellation: once a pre-provider or
	// definite provider failure is known, leaving the durable reservation behind
	// would falsely consume the account's budget until the UTC day rolls over.
	// Retry a transient database failure within a short detached window and keep
	// local ownership unless the operation is confirmed released/already absent.
	base := t.ctx
	if base == nil {
		base = context.Background()
	}
	settlementCtx, cancel := context.WithTimeout(context.WithoutCancel(base), 2*time.Second)
	defer cancel()
	var releaseErr error
	for attempt := 0; attempt < 3; attempt++ {
		_, releaseErr = t.queries.ReleasePhysicalDailyPublish(settlementCtx, db.ReleasePhysicalDailyPublishParams{
			WorkspaceID: t.workspaceID, OperationKey: operationKey,
		})
		if releaseErr == nil {
			t.mu.Lock()
			delete(t.newlyReserved, operationKey)
			delete(t.reserved, operationKey)
			t.mu.Unlock()
			return nil
		}
		if settlementCtx.Err() != nil {
			break
		}
		if attempt < 2 {
			backoff := time.Duration(1<<attempt) * 50 * time.Millisecond
			timer := time.NewTimer(backoff)
			select {
			case <-timer.C:
			case <-settlementCtx.Done():
				timer.Stop()
				return releaseErr
			}
		}
	}
	return releaseErr
}

func (t *PerPlatformDailyTracker) Finalize(operationKey string) {
	if t == nil || t.queries == nil || t.workspaceID == "" {
		return
	}
	t.mu.Lock()
	delete(t.newlyReserved, operationKey)
	delete(t.reserved, operationKey)
	t.mu.Unlock()
	_, _ = t.queries.FinalizePhysicalDailyPublish(t.ctx, db.FinalizePhysicalDailyPublishParams{
		WorkspaceID: t.workspaceID, OperationKey: operationKey,
	})
}

func (t *PerPlatformDailyTracker) Reserved(operationKey string) bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.reserved[operationKey]
}
