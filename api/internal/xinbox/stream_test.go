package xinbox

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

type scriptedStreamOpener struct {
	mu       sync.Mutex
	attempts int
	errs     []error
}

func (s *scriptedStreamOpener) ConsumeFilteredStream(context.Context, string, func(StreamEvent) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts++
	if len(s.errs) == 0 {
		return nil
	}
	err := s.errs[0]
	s.errs = s.errs[1:]
	return err
}

func TestStreamSupervisorUsesBoundedExponentialBackoff(t *testing.T) {
	opener := &scriptedStreamOpener{errs: []error{
		errors.New("disconnect 1"),
		errors.New("disconnect 2"),
		errors.New("disconnect 3"),
		nil,
	}}
	var delays []time.Duration
	supervisor := NewStreamSupervisor(opener, StreamSupervisorConfig{
		InitialBackoff: time.Second,
		MaxBackoff:     2 * time.Second,
		Sleep: func(_ context.Context, delay time.Duration) error {
			delays = append(delays, delay)
			return nil
		},
	})

	if err := supervisor.Run(context.Background(), "managed-app", "app-token", func(StreamEvent) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if want := []time.Duration{time.Second, 2 * time.Second, 2 * time.Second}; !reflect.DeepEqual(delays, want) {
		t.Fatalf("delays = %v, want %v", delays, want)
	}
}

func TestStreamSupervisorDoesNotOpenTwoConnectionsForSameApp(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	opener := streamOpenerFunc(func(ctx context.Context, _ string, _ func(StreamEvent) error) error {
		close(started)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-release:
			return nil
		}
	})
	supervisor := NewStreamSupervisor(opener, StreamSupervisorConfig{})

	errs := make(chan error, 1)
	go func() {
		errs <- supervisor.Run(context.Background(), "workspace:one", "app-token", func(StreamEvent) error { return nil })
	}()
	<-started

	err := supervisor.Run(context.Background(), "workspace:one", "app-token", func(StreamEvent) error { return nil })
	if !errors.Is(err, ErrStreamAlreadyRunning) {
		t.Fatalf("err = %v, want ErrStreamAlreadyRunning", err)
	}
	close(release)
	if err := <-errs; err != nil {
		t.Fatal(err)
	}
}

type streamOpenerFunc func(context.Context, string, func(StreamEvent) error) error

func (f streamOpenerFunc) ConsumeFilteredStream(ctx context.Context, token string, handler func(StreamEvent) error) error {
	return f(ctx, token, handler)
}
