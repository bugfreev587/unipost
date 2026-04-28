package ratelimit

import (
	"errors"
	"log/slog"
	"sync"
	"time"
)

// circuitState is the small state machine wrapping every Redis call
// the limiter makes. Closed = pass calls through. Open = short-circuit
// to a degraded local fallback. Half-open = let exactly one canary
// through; success closes the circuit, failure re-opens it.
//
// Why a custom breaker rather than a library: the call surface is
// tiny (Redis Eval / Get) and the failure semantics are domain
// specific (we want to fail open into a local in-memory bucket, not
// just return an error). Pulling in github.com/sony/gobreaker for a
// 60-line file would be more code, not less.
type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

// Breaker thresholds. Defaults match §12.1 of the PRD: 5 failures
// inside a 10s window trip the circuit, the open state lasts 30s,
// then a single canary call probes recovery.
const (
	defaultFailureThreshold = 5
	defaultFailureWindow    = 10 * time.Second
	defaultOpenDuration     = 30 * time.Second
)

// Breaker is the small failure-counter state machine. Methods are
// goroutine-safe; the limiter wraps every Redis call in BeforeCall /
// AfterCall.
type Breaker struct {
	mu sync.Mutex

	state         circuitState
	openedAt      time.Time
	canaryInFlight bool

	// Rolling failure window — list of timestamps of recent failures.
	// Cleared on every state transition; capped at failureThreshold
	// entries so memory is bounded.
	failures []time.Time

	failureThreshold int
	failureWindow    time.Duration
	openDuration     time.Duration

	// label is used for log lines so a transition tells you which
	// limiter tripped (request bucket vs enqueue window).
	label string
}

// NewBreaker returns a Breaker with the PRD defaults. Pass a
// non-empty label so logs identify which Redis-backed limiter is
// transitioning.
func NewBreaker(label string) *Breaker {
	return &Breaker{
		state:            circuitClosed,
		failureThreshold: defaultFailureThreshold,
		failureWindow:    defaultFailureWindow,
		openDuration:     defaultOpenDuration,
		label:            label,
	}
}

// ErrCircuitOpen is returned by BeforeCall when the breaker is open
// and a canary attempt is not authorized for this caller.
var ErrCircuitOpen = errors.New("ratelimit: circuit open, redis unavailable")

// BeforeCall returns nil when the caller may proceed to invoke
// Redis, or ErrCircuitOpen when the breaker has decided the call
// should short-circuit to the degraded fallback. Closed → always
// allow. Open → allow nothing until the open window elapses, then
// promote one caller to half-open canary. Half-open → block all
// callers except the in-flight canary.
func (b *Breaker) BeforeCall() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	switch b.state {
	case circuitClosed:
		return nil
	case circuitOpen:
		if now.Sub(b.openedAt) < b.openDuration {
			return ErrCircuitOpen
		}
		// Probationary half-open — exactly one canary at a time.
		b.state = circuitHalfOpen
		b.canaryInFlight = true
		slog.Info("ratelimit: circuit half-open, sending canary", "label", b.label)
		return nil
	case circuitHalfOpen:
		if b.canaryInFlight {
			return ErrCircuitOpen
		}
		b.canaryInFlight = true
		return nil
	default:
		return ErrCircuitOpen
	}
}

// AfterCall feeds the call result back into the breaker. err == nil
// is treated as success; any non-nil err counts as a failure and
// contributes to the rolling failure window.
func (b *Breaker) AfterCall(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err == nil {
		b.recordSuccessLocked()
		return
	}
	b.recordFailureLocked()
}

func (b *Breaker) recordSuccessLocked() {
	switch b.state {
	case circuitClosed:
		// Forget any old failures so a long-lived process doesn't
		// accumulate stale entries.
		if len(b.failures) > 0 {
			b.failures = b.failures[:0]
		}
	case circuitHalfOpen:
		b.state = circuitClosed
		b.canaryInFlight = false
		b.failures = b.failures[:0]
		slog.Info("ratelimit: circuit closed (canary succeeded)", "label", b.label)
	case circuitOpen:
		// Should not normally happen — BeforeCall blocks Open. If it
		// does, treat as if we were half-open.
		b.state = circuitClosed
		b.canaryInFlight = false
		b.failures = b.failures[:0]
	}
}

func (b *Breaker) recordFailureLocked() {
	now := time.Now()
	cutoff := now.Add(-b.failureWindow)

	// Drop entries outside the rolling window.
	keep := b.failures[:0]
	for _, t := range b.failures {
		if t.After(cutoff) {
			keep = append(keep, t)
		}
	}
	b.failures = append(keep, now)

	switch b.state {
	case circuitClosed:
		if len(b.failures) >= b.failureThreshold {
			b.state = circuitOpen
			b.openedAt = now
			slog.Warn("ratelimit: circuit open (failure threshold reached)",
				"label", b.label,
				"failures", len(b.failures),
				"window", b.failureWindow,
			)
		}
	case circuitHalfOpen:
		b.state = circuitOpen
		b.openedAt = now
		b.canaryInFlight = false
		b.failures = b.failures[:0]
		slog.Warn("ratelimit: circuit re-opened (canary failed)", "label", b.label)
	}
}

// State is exposed for metrics; do not use it for branching.
func (b *Breaker) State() circuitState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}
