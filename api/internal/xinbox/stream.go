package xinbox

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrStreamAlreadyRunning = errors.New("X filtered stream already running for app identity")

type StreamOpener interface {
	ConsumeFilteredStream(context.Context, string, func(StreamEvent) error) error
}

type StreamSupervisorConfig struct {
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Sleep          func(context.Context, time.Duration) error
}

type StreamSupervisor struct {
	opener         StreamOpener
	initialBackoff time.Duration
	maxBackoff     time.Duration
	sleep          func(context.Context, time.Duration) error

	mu     sync.Mutex
	active map[string]struct{}
}

func NewStreamSupervisor(opener StreamOpener, config StreamSupervisorConfig) *StreamSupervisor {
	initialBackoff := config.InitialBackoff
	if initialBackoff <= 0 {
		initialBackoff = time.Second
	}
	maxBackoff := config.MaxBackoff
	if maxBackoff < initialBackoff {
		maxBackoff = 30 * time.Second
	}
	sleep := config.Sleep
	if sleep == nil {
		sleep = sleepContext
	}
	return &StreamSupervisor{
		opener:         opener,
		initialBackoff: initialBackoff,
		maxBackoff:     maxBackoff,
		sleep:          sleep,
		active:         make(map[string]struct{}),
	}
}

func (s *StreamSupervisor) Run(
	ctx context.Context,
	appIdentity string,
	bearerToken string,
	handler func(StreamEvent) error,
) error {
	s.mu.Lock()
	if _, exists := s.active[appIdentity]; exists {
		s.mu.Unlock()
		return ErrStreamAlreadyRunning
	}
	s.active[appIdentity] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.active, appIdentity)
		s.mu.Unlock()
	}()

	backoff := s.initialBackoff
	for {
		err := s.opener.ConsumeFilteredStream(ctx, bearerToken, handler)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.sleep(ctx, backoff); err != nil {
			return err
		}
		backoff *= 2
		if backoff > s.maxBackoff {
			backoff = s.maxBackoff
		}
	}
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
