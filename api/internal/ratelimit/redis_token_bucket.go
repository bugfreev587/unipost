package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	redislib "github.com/redis/go-redis/v9"
)

// requestLimiter is the per-workspace request token bucket. The Lua
// script does the entire check-and-decrement atomically inside Redis
// so two API replicas cannot both consume the last token.
//
// Key: rl:req:ws:{workspace_id} (no per-route split — see PRD §9.2).
//
// Why a Lua script: pipelines aren't atomic across replicas, and
// WATCH/MULTI requires a retry loop on collision. Lua runs once,
// holds the key, returns the decision plus the suggested
// Retry-After seconds.
type requestLimiter struct {
	rdb     *redislib.Client
	breaker *Breaker

	// degradedMu guards the per-replica fallback buckets used when
	// the breaker is open. Sized lazily — most workspaces never see
	// a Redis outage during their session.
	degradedMu      sync.Mutex
	degradedBuckets map[string]*localTokenBucket
}

func newRequestLimiter(rdb *redislib.Client) *requestLimiter {
	return &requestLimiter{
		rdb:             rdb,
		breaker:         NewBreaker("request"),
		degradedBuckets: make(map[string]*localTokenBucket),
	}
}

// requestBucketScript is the atomic token-bucket update.
//
// KEYS[1] = bucket key
// ARGV[1] = capacity (max tokens)
// ARGV[2] = refill rate per second
// ARGV[3] = now (unix seconds, float)
// ARGV[4] = requested tokens (always 1 for request limiter)
//
// Returns: { allowed (0/1), retry_after_seconds, remaining (int),
//            limit (int), reset_unix_seconds (int) }
//
// remaining is floor(tokens) so a partially-refilled bucket reports
// the conservative number to the client. reset_unix_seconds is the
// unix timestamp at which the bucket would be back to full given
// the current refill rate; zero would mean "already full", which
// for a just-decremented bucket implies "1 token / rate" seconds.
//
// State stored in Redis: a hash with fields
//   tokens : current token count (float)
//   ts     : last refill timestamp (float seconds)
//
// Idle buckets expire on their own — TTL is set to ceil(capacity / rate)
// + 1s so a bucket that's full at full token capacity sticks around
// long enough to be useful but doesn't leak forever.
var requestBucketScript = redislib.NewScript(`
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local cost = tonumber(ARGV[4])

local data = redis.call('HMGET', key, 'tokens', 'ts')
local tokens = tonumber(data[1])
local ts = tonumber(data[2])

if tokens == nil then
    tokens = capacity
    ts = now
end

local elapsed = math.max(0, now - ts)
tokens = math.min(capacity, tokens + elapsed * rate)
ts = now

local allowed = 0
local retry_after = 0
if tokens >= cost then
    tokens = tokens - cost
    allowed = 1
else
    local needed = cost - tokens
    if rate > 0 then
        retry_after = math.ceil(needed / rate)
    else
        retry_after = 60
    end
end

redis.call('HSET', key, 'tokens', tokens, 'ts', ts)
local ttl = math.ceil(capacity / math.max(rate, 0.001)) + 1
redis.call('EXPIRE', key, ttl)

local time_to_full = 0
if tokens < capacity and rate > 0 then
    time_to_full = (capacity - tokens) / rate
end
local reset_unix = math.ceil(now + time_to_full)
local remaining = math.floor(tokens)

return { allowed, retry_after, remaining, math.floor(capacity), reset_unix }
`)

func (l *requestLimiter) Allow(ctx context.Context, scope RequestScope) (Decision, error) {
	limits := LimitsForPlan(scope.PlanID)
	capacity := float64(limits.RequestBurstTokens)
	if capacity < 1 {
		capacity = 1
	}
	rate := limits.RequestRatePerSec
	if rate <= 0 {
		rate = 0.5
	}

	if err := l.breaker.BeforeCall(); err != nil {
		return l.degradedAllow(scope.WorkspaceID, capacity, rate)
	}

	key := fmt.Sprintf("rl:req:ws:%s", scope.WorkspaceID)
	now := float64(time.Now().UnixMilli()) / 1000.0

	res, err := requestBucketScript.Run(ctx, l.rdb, []string{key},
		capacity, rate, now, 1,
	).Slice()
	l.breaker.AfterCall(err)
	if err != nil {
		// Breaker may have just tripped on this call; either way,
		// fall through to the degraded local bucket so the request
		// is still subject to *some* admission control.
		return l.degradedAllow(scope.WorkspaceID, capacity, rate)
	}

	state, perr := parseRequestBucketResult(res)
	if perr != nil {
		return Decision{}, perr
	}
	dec := Decision{
		Allowed:    state.allowed,
		RetryAfter: state.retryAfter,
		Limit:      state.limit,
		Remaining:  state.remaining,
		ResetUnix:  state.resetUnix,
	}
	if !dec.Allowed {
		dec.Reason = NormRequestLimited
	}
	return dec, nil
}

// degradedAllow runs the local in-memory token bucket used while the
// circuit is open. Sized at capacity / replicaCount; replicaCount is
// approximated as 1 for v1 — accurate replica count requires a
// service-discovery hook we don't have yet, and over-counting is
// safer than under-counting (a single replica might have to absorb
// the whole workspace burst when Redis is down).
func (l *requestLimiter) degradedAllow(workspaceID string, capacity, rate float64) (Decision, error) {
	l.degradedMu.Lock()
	bucket, ok := l.degradedBuckets[workspaceID]
	if !ok {
		bucket = newLocalTokenBucket(capacity, rate)
		l.degradedBuckets[workspaceID] = bucket
	}
	l.degradedMu.Unlock()

	allowed, retry, remaining, reset := bucket.allow()
	dec := Decision{
		Allowed:    allowed,
		RetryAfter: retry,
		Limit:      int(capacity),
		Remaining:  remaining,
		ResetUnix:  reset,
	}
	if !allowed {
		dec.Reason = "request_limited_degraded"
	}
	return dec, nil
}

// localTokenBucket is the in-process fallback used when the circuit
// breaker is open. Independent state per workspace, refills at rate.
type localTokenBucket struct {
	mu       sync.Mutex
	capacity float64
	rate     float64
	tokens   float64
	last     time.Time
}

func newLocalTokenBucket(capacity, rate float64) *localTokenBucket {
	return &localTokenBucket{
		capacity: capacity,
		rate:     rate,
		tokens:   capacity,
		last:     time.Now(),
	}
}

// allow returns (allowed, retry-after, remaining-tokens-floor,
// reset-unix-when-full). The remaining/reset values are populated
// for both allow and deny so the degraded fallback still drives the
// X-UniPost-RateLimit-* headers — a Redis outage shouldn't make the
// headers go silent.
func (b *localTokenBucket) allow() (bool, time.Duration, int, int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(b.last).Seconds()
	b.tokens = math.Min(b.capacity, b.tokens+elapsed*b.rate)
	b.last = now

	allowed := b.tokens >= 1
	var retry time.Duration
	if allowed {
		b.tokens -= 1
	} else {
		needed := 1 - b.tokens
		retry = time.Duration(math.Ceil(needed/b.rate)) * time.Second
		if retry < time.Second {
			retry = time.Second
		}
	}

	timeToFull := 0.0
	if b.tokens < b.capacity && b.rate > 0 {
		timeToFull = (b.capacity - b.tokens) / b.rate
	}
	return allowed, retry, int(math.Floor(b.tokens)), now.Unix() + int64(math.Ceil(timeToFull))
}

// requestBucketState holds the parsed Lua return values for the
// request limiter. Kept as a struct so the Allow path can copy the
// state directly into Decision instead of juggling 5 return values.
type requestBucketState struct {
	allowed    bool
	retryAfter time.Duration
	remaining  int
	limit      int
	resetUnix  int64
}

// parseRequestBucketResult unpacks the {allowed, retry, remaining,
// limit, reset_unix} tuple Lua returns. go-redis decodes Lua
// integers as int64 inside an []interface{}.
func parseRequestBucketResult(res []interface{}) (requestBucketState, error) {
	if len(res) != 5 {
		return requestBucketState{}, fmt.Errorf("ratelimit: token bucket lua returned %d values, expected 5", len(res))
	}
	a, err := toInt64(res[0])
	if err != nil {
		return requestBucketState{}, err
	}
	retry, err := toInt64(res[1])
	if err != nil {
		return requestBucketState{}, err
	}
	remaining, err := toInt64(res[2])
	if err != nil {
		return requestBucketState{}, err
	}
	limit, err := toInt64(res[3])
	if err != nil {
		return requestBucketState{}, err
	}
	reset, err := toInt64(res[4])
	if err != nil {
		return requestBucketState{}, err
	}
	if remaining < 0 {
		remaining = 0
	}
	return requestBucketState{
		allowed:    a == 1,
		retryAfter: time.Duration(retry) * time.Second,
		remaining:  int(remaining),
		limit:      int(limit),
		resetUnix:  reset,
	}, nil
}

func toInt64(v interface{}) (int64, error) {
	switch x := v.(type) {
	case int64:
		return x, nil
	case int:
		return int64(x), nil
	case string:
		n, err := strconv.ParseInt(x, 10, 64)
		if err == nil {
			return n, nil
		}
		f, err := strconv.ParseFloat(x, 64)
		if err == nil {
			return int64(math.Ceil(f)), nil
		}
		return 0, fmt.Errorf("ratelimit: cannot parse %q as int", x)
	case float64:
		return int64(math.Ceil(x)), nil
	}
	return 0, errors.New("ratelimit: unexpected lua return type")
}
