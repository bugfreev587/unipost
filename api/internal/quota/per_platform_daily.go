package quota

import (
	"context"
	"errors"
	"sync"

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

// PerPlatformDailyTracker is a per-request, in-memory budget for
// per-(account, platform) dispatches in the current UTC day. Snapshots
// "remaining publishes today" once per unique social_account_id at the
// top of a publish request, then atomically decrements as each
// dispatch fires.
//
// Why per-request and not global: counting at the start of each
// request gives a tight enough bound for the safety-belt purpose
// (preventing one runaway script from posting 200 X tweets in a
// minute) without paying a DB round-trip per dispatch. The next
// request will re-fetch the count from social_post_results.published_at
// so the cap stays accurate at minute-scale granularity, which is
// well within the platform-flagging timescale we care about.
//
// Race scope: protects against parallel dispatch groups within ONE
// request hitting the same account at the same instant (threads run
// serial within a group; groups run parallel). It does NOT prevent
// two API replicas from concurrently dispatching to the same account
// — that's a soft-cap acceptance documented in the PRD §5.5. The cap
// is a runaway-script safety belt, not a billing-grade hard limit;
// the next request will see the updated DB count and clamp itself.
//
// All trackers carry a non-empty cap map (built per-request from
// PerPlatformDailyCap). A nil tracker short-circuits Allow() to true
// — used by tests and any future code path that wants to opt out.
type PerPlatformDailyTracker struct {
	mu        sync.Mutex
	remaining map[string]int // social_account_id → remaining for current UTC day
	cap       map[string]int // social_account_id → cap that applies (per platform)
}

// PerPlatformDailyTarget is one (account_id, platform) tuple the
// caller plans to dispatch to. The tracker uses the platform to look
// up the cap and the account_id to load the snapshot count.
type PerPlatformDailyTarget struct {
	AccountID string
	Platform  string
}

// NewPerPlatformDailyTracker builds a tracker for one publish request.
// Pass the deduped list of (account, platform) pairs the request will
// dispatch to. Counts are loaded once, up front, so the dispatch loop
// doesn't pay a DB round-trip per dispatch. Returns a tracker that's
// safe to share across goroutines.
//
// Best-effort on count load: a DB failure for one account degrades to
// "assume zero used today, allow this request" rather than 500ing the
// whole request. The legacy behavior — no daily cap enforcement —
// is the safer fallback if the partial index briefly stalls.
func NewPerPlatformDailyTracker(
	ctx context.Context,
	queries *db.Queries,
	targets []PerPlatformDailyTarget,
) *PerPlatformDailyTracker {
	t := &PerPlatformDailyTracker{
		remaining: make(map[string]int, len(targets)),
		cap:       make(map[string]int, len(targets)),
	}
	for _, tgt := range targets {
		if _, seen := t.cap[tgt.AccountID]; seen {
			continue
		}
		cap := CapForPlatform(tgt.Platform)
		t.cap[tgt.AccountID] = cap
		count, err := queries.CountPublishedTodayByAccount(ctx, tgt.AccountID)
		if err != nil {
			t.remaining[tgt.AccountID] = cap
			continue
		}
		left := cap - int(count)
		if left < 0 {
			left = 0
		}
		t.remaining[tgt.AccountID] = left
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
func (t *PerPlatformDailyTracker) Allow(accountID, platform string) bool {
	if t == nil {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	left, ok := t.remaining[accountID]
	if !ok {
		// Defensive: account wasn't in the snapshot. Snapshot upstream
		// should always cover every dispatch target — if we get here,
		// give the dispatch a fresh budget rather than crashing.
		cap := CapForPlatform(platform)
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
