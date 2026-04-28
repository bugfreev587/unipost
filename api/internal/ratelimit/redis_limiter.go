package ratelimit

import (
	"context"

	redislib "github.com/redis/go-redis/v9"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// RedisLimiter composes the three v1 admission controls behind one
// Limiter interface. Construct via NewRedisLimiter from main.go.
type RedisLimiter struct {
	requests *requestLimiter
	enqueue  *enqueueLimiter
	depth    *depthLimiter
}

// NewRedisLimiter wires up the production limiter. rdb may be nil —
// the caller (main.go) is expected to substitute NoopLimiter in that
// case rather than passing nil here.
func NewRedisLimiter(rdb *redislib.Client, queries *db.Queries) *RedisLimiter {
	return &RedisLimiter{
		requests: newRequestLimiter(rdb),
		enqueue:  newEnqueueLimiter(rdb),
		depth:    newDepthLimiter(queries),
	}
}

func (l *RedisLimiter) AllowRequest(ctx context.Context, scope RequestScope) (Decision, error) {
	return l.requests.Allow(ctx, scope)
}

func (l *RedisLimiter) AllowEnqueue(ctx context.Context, scope EnqueueScope, units int) (Decision, error) {
	return l.enqueue.Allow(ctx, scope, units)
}

func (l *RedisLimiter) CheckQueueDepth(ctx context.Context, scope QueueScope, addedUnits int) (Decision, error) {
	return l.depth.Check(ctx, scope, addedUnits)
}
