package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	redislib "github.com/redis/go-redis/v9"
)

// enqueueLimiter is the per-workspace enqueue throughput control. It
// tracks accepted post-count units across two sliding windows
// simultaneously (1-min and 5-min) so a workspace cannot evade the
// short-window cap by spreading admissions across the longer window
// or vice versa. Either window's breach denies admission.
//
// Implementation: a Redis ZSET keyed by workspace, scored by
// timestamp-millis, member is a unique request id. Each admit:
//   1. ZREMRANGEBYSCORE to drop entries older than the longest
//      window
//   2. count entries inside each window
//   3. if either count + units > cap, deny and return retry-after
//   4. otherwise ZADD `units` copies (one per unit, distinct members
//      so the sorted set tolerates concurrent admits)
//
// All four steps run in one Lua script so the check + add is atomic
// across replicas.
type enqueueLimiter struct {
	rdb     *redislib.Client
	breaker *Breaker

	// Degraded fallback: per-workspace sliding window kept locally.
	// Memory is bounded — entries are dropped on every Allow call.
	degradedMu      sync.Mutex
	degradedWindows map[string]*localSlidingWindow
}

func newEnqueueLimiter(rdb *redislib.Client) *enqueueLimiter {
	return &enqueueLimiter{
		rdb:             rdb,
		breaker:         NewBreaker("enqueue"),
		degradedWindows: make(map[string]*localSlidingWindow),
	}
}

// enqueueWindowScript is the atomic sliding-window add.
//
// KEYS[1] = zset key
// ARGV[1] = now (unix millis)
// ARGV[2] = window 1 millis (e.g. 60000 for 1 min)
// ARGV[3] = window 1 cap (e.g. 50 for free / 1 min)
// ARGV[4] = window 2 millis (e.g. 300000 for 5 min)
// ARGV[5] = window 2 cap
// ARGV[6] = units to admit
// ARGV[7] = request id seed (uniquifier — caller passes a random
//           string; the script appends -i for each unit so members
//           never collide)
//
// Returns: { allowed (0/1), retry_after_seconds (number), w1_count, w2_count }
//
// Retry-after is computed against whichever window denied: the
// oldest entry inside the breached window's score plus the window
// length minus now, in seconds (rounded up).
var enqueueWindowScript = redislib.NewScript(`
local key = KEYS[1]
local now = tonumber(ARGV[1])
local w1ms = tonumber(ARGV[2])
local w1cap = tonumber(ARGV[3])
local w2ms = tonumber(ARGV[4])
local w2cap = tonumber(ARGV[5])
local units = tonumber(ARGV[6])
local seed = ARGV[7]

-- Drop everything older than the longest window we care about.
local longest = math.max(w1ms, w2ms)
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - longest)

local w1_lo = now - w1ms
local w2_lo = now - w2ms
local w1_count = redis.call('ZCOUNT', key, w1_lo, now)
local w2_count = redis.call('ZCOUNT', key, w2_lo, now)

local breached_window = 0
if w1_count + units > w1cap then breached_window = 1 end
if w2_count + units > w2cap then breached_window = (breached_window == 0) and 2 or breached_window end

if breached_window > 0 then
    -- Retry-after = (oldest entry in breached window's score) + window - now, in seconds.
    local lo = (breached_window == 1) and w1_lo or w2_lo
    local oldest = redis.call('ZRANGEBYSCORE', key, lo, now, 'LIMIT', 0, 1)
    local retry_ms
    if oldest[1] then
        local oldest_score = redis.call('ZSCORE', key, oldest[1])
        local window_ms = (breached_window == 1) and w1ms or w2ms
        retry_ms = (tonumber(oldest_score) + window_ms) - now
        if retry_ms < 1000 then retry_ms = 1000 end
    else
        retry_ms = 1000
    end
    local retry_s = math.ceil(retry_ms / 1000)
    return { 0, retry_s, w1_count, w2_count }
end

for i = 1, units do
    redis.call('ZADD', key, now, seed .. '-' .. i)
end

local ttl = math.ceil(longest / 1000) + 1
redis.call('EXPIRE', key, ttl)

return { 1, 0, w1_count + units, w2_count + units }
`)

func (l *enqueueLimiter) Allow(ctx context.Context, scope EnqueueScope, units int) (Decision, error) {
	if units <= 0 {
		units = 1
	}
	limits := LimitsForPlan(scope.PlanID)

	if err := l.breaker.BeforeCall(); err != nil {
		return l.degradedAllow(scope.WorkspaceID, limits, units)
	}

	key := fmt.Sprintf("rl:enqueue:ws:%s", scope.WorkspaceID)
	now := time.Now().UnixMilli()
	seed := fmt.Sprintf("%d-%d", now, fastRand())

	res, err := enqueueWindowScript.Run(ctx, l.rdb, []string{key},
		now,
		int64(limits.Window1m()/time.Millisecond),
		limits.EnqueuePostsPerMin,
		int64(limits.Window5m()/time.Millisecond),
		limits.EnqueuePostsPer5Min,
		units,
		seed,
	).Slice()
	l.breaker.AfterCall(err)
	if err != nil {
		return l.degradedAllow(scope.WorkspaceID, limits, units)
	}

	if len(res) < 2 {
		return Decision{}, fmt.Errorf("ratelimit: enqueue lua returned %d values", len(res))
	}
	allowed, aerr := toInt64(res[0])
	if aerr != nil {
		return Decision{}, aerr
	}
	retry, rerr := toInt64(res[1])
	if rerr != nil {
		return Decision{}, rerr
	}
	if allowed == 1 {
		return Decision{Allowed: true}, nil
	}
	return Decision{
		Allowed:    false,
		RetryAfter: time.Duration(retry) * time.Second,
		Reason:     NormEnqueueLimited,
	}, nil
}

func (l *enqueueLimiter) degradedAllow(workspaceID string, limits PlanLimits, units int) (Decision, error) {
	l.degradedMu.Lock()
	w, ok := l.degradedWindows[workspaceID]
	if !ok {
		w = newLocalSlidingWindow(limits)
		l.degradedWindows[workspaceID] = w
	}
	l.degradedMu.Unlock()

	allowed, retry := w.allow(units, limits)
	if allowed {
		return Decision{Allowed: true}, nil
	}
	return Decision{
		Allowed:    false,
		RetryAfter: retry,
		Reason:     "enqueue_limited_degraded",
	}, nil
}

// localSlidingWindow is the in-process fallback used while the
// breaker is open. Holds raw timestamp entries; degraded mode is
// short-lived (30s open) so memory stays bounded.
type localSlidingWindow struct {
	mu      sync.Mutex
	entries []time.Time
}

func newLocalSlidingWindow(_ PlanLimits) *localSlidingWindow {
	return &localSlidingWindow{}
}

func (w *localSlidingWindow) allow(units int, limits PlanLimits) (bool, time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := time.Now()
	cutoff5m := now.Add(-limits.Window5m())

	// Drop expired.
	keep := w.entries[:0]
	for _, t := range w.entries {
		if t.After(cutoff5m) {
			keep = append(keep, t)
		}
	}
	w.entries = keep

	count1m := 0
	cutoff1m := now.Add(-limits.Window1m())
	for _, t := range w.entries {
		if t.After(cutoff1m) {
			count1m++
		}
	}
	count5m := len(w.entries)

	if count1m+units > limits.EnqueuePostsPerMin {
		return false, time.Until(w.entries[0].Add(limits.Window1m()))
	}
	if count5m+units > limits.EnqueuePostsPer5Min {
		return false, time.Until(w.entries[0].Add(limits.Window5m()))
	}
	for i := 0; i < units; i++ {
		w.entries = append(w.entries, now)
	}
	return true, 0
}

// fastRand returns a non-cryptographic random int — only used as a
// uniquifier for sorted-set members so concurrent admits from the
// same millisecond don't collide. Cheap math/rand is enough.
func fastRand() uint32 {
	return uint32(time.Now().UnixNano()) ^ uint32(time.Now().Unix())
}
