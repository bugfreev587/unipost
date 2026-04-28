package ratelimit

import "context"

// NoopLimiter allows every admission attempt. Used when REDIS_URL is
// not configured so local dev does not require a Redis instance.
// Production startup logs a warning if NoopLimiter is selected; see
// main.go.
type NoopLimiter struct{}

func (NoopLimiter) AllowRequest(ctx context.Context, scope RequestScope) (Decision, error) {
	return Decision{Allowed: true}, nil
}

func (NoopLimiter) AllowEnqueue(ctx context.Context, scope EnqueueScope, units int) (Decision, error) {
	return Decision{Allowed: true}, nil
}

func (NoopLimiter) CheckQueueDepth(ctx context.Context, scope QueueScope, addedUnits int) (Decision, error) {
	return Decision{Allowed: true}, nil
}
