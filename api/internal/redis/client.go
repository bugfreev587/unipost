// Package redis owns the shared *redis.Client used by other internal
// packages (rate limiting today, idempotency cache and lookup caches
// later). Connection lifecycle lives here so future Redis consumers
// reuse one client and one set of pool tuning knobs.
//
// New is intentionally lenient about REDIS_URL being unset — local
// dev should keep working without Redis. Callers (main.go, the
// ratelimit package) check for a nil client and fall back to
// disabled-mode limiters in that case.
package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	redislib "github.com/redis/go-redis/v9"
)

// Client is an alias so callers can depend on this package without
// also importing go-redis directly.
type Client = redislib.Client

// New parses url and returns a connected client. A nil client is
// returned without error when url is empty so local dev (and tests)
// can proceed without Redis. A non-empty url that fails to ping
// returns an error so production startup can fail fast.
func New(ctx context.Context, url string) (*Client, error) {
	if url == "" {
		slog.Info("redis: REDIS_URL not set; redis-backed features will run in disabled mode")
		return nil, nil
	}

	opts, err := redislib.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis: parse REDIS_URL: %w", err)
	}

	// Conservative timeouts so a Redis stall doesn't fan out into
	// publish-path latency. The circuit breaker in
	// internal/ratelimit picks up repeated timeouts and trips into
	// degraded mode.
	if opts.DialTimeout == 0 {
		opts.DialTimeout = 2 * time.Second
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = 200 * time.Millisecond
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = 200 * time.Millisecond
	}

	c := redislib.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := c.Ping(pingCtx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("redis: ping %s: %w", opts.Addr, err)
	}

	slog.Info("redis: connected", "addr", opts.Addr, "db", opts.DB)
	return c, nil
}
