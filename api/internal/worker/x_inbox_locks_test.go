package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeXInboxLockRow struct {
	value bool
	err   error
}

func (r fakeXInboxLockRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*(dest[0].(*bool)) = r.value
	return nil
}

type fakeXInboxLockSession struct {
	mu         sync.Mutex
	held       map[string]bool
	failUnlock bool
	closed     bool
	active     atomic.Int32
	maxActive  atomic.Int32
}

func (s *fakeXInboxLockSession) QueryRow(
	_ context.Context,
	query string,
	args ...any,
) xInboxLockRow {
	active := s.active.Add(1)
	for {
		previous := s.maxActive.Load()
		if active <= previous || s.maxActive.CompareAndSwap(previous, active) {
			break
		}
	}
	defer s.active.Add(-1)
	time.Sleep(time.Millisecond)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return fakeXInboxLockRow{err: errors.New("session closed")}
	}
	key := args[0].(string)
	if strings.Contains(query, "pg_try_advisory_lock") {
		if s.held[key] {
			return fakeXInboxLockRow{value: false}
		}
		s.held[key] = true
		return fakeXInboxLockRow{value: true}
	}
	if s.failUnlock {
		return fakeXInboxLockRow{err: errors.New("unlock connection failure")}
	}
	if !s.held[key] {
		return fakeXInboxLockRow{value: false}
	}
	delete(s.held, key)
	return fakeXInboxLockRow{value: true}
}

func (s *fakeXInboxLockSession) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func TestPostgresStreamLockManagerMultiplexesLocksOnOneSerializedSession(t *testing.T) {
	session := &fakeXInboxLockSession{held: make(map[string]bool)}
	var factoryCalls atomic.Int32
	manager := newPostgresStreamLockManager(func(context.Context) (xInboxLockSession, error) {
		factoryCalls.Add(1)
		return session, nil
	})

	const count = 10
	leases := make(chan XInboxLeaderLease, count)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			lease, acquired, err := manager.TryAcquire(
				context.Background(),
				fmt.Sprintf("x-inbox-stream:app-%d", index),
				func() {},
			)
			if err != nil || !acquired {
				t.Errorf("acquire %d: acquired=%v err=%v", index, acquired, err)
				return
			}
			leases <- lease
		}(i)
	}
	wg.Wait()
	close(leases)
	for lease := range leases {
		if err := lease.Release(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if factoryCalls.Load() != 1 {
		t.Fatalf("session factory calls = %d, want one process-level session", factoryCalls.Load())
	}
	if session.maxActive.Load() != 1 {
		t.Fatalf("max concurrent session operations = %d, want serialized", session.maxActive.Load())
	}
}

func TestPostgresStreamLockManagerSessionFailureCancelsAllOwnedStreams(t *testing.T) {
	session := &fakeXInboxLockSession{held: make(map[string]bool)}
	manager := newPostgresStreamLockManager(func(context.Context) (xInboxLockSession, error) {
		return session, nil
	})
	var cancelled atomic.Int32
	first, acquired, err := manager.TryAcquire(context.Background(), "stream:first", func() {
		cancelled.Add(1)
	})
	if err != nil || !acquired {
		t.Fatalf("first acquire: acquired=%v err=%v", acquired, err)
	}
	_, acquired, err = manager.TryAcquire(context.Background(), "stream:second", func() {
		cancelled.Add(1)
	})
	if err != nil || !acquired {
		t.Fatalf("second acquire: acquired=%v err=%v", acquired, err)
	}

	session.mu.Lock()
	session.failUnlock = true
	session.mu.Unlock()
	if err := first.Release(context.Background()); err == nil {
		t.Fatal("expected unlock failure")
	}
	if cancelled.Load() != 2 {
		t.Fatalf("cancelled streams = %d, want every owned stream", cancelled.Load())
	}
}

func TestPostgresStreamLockManagerReconnectsAfterInvalidSession(t *testing.T) {
	firstSession := &fakeXInboxLockSession{held: make(map[string]bool)}
	secondSession := &fakeXInboxLockSession{held: make(map[string]bool)}
	sessions := []xInboxLockSession{firstSession, secondSession}
	var factoryCalls atomic.Int32
	manager := newPostgresStreamLockManager(func(context.Context) (xInboxLockSession, error) {
		index := int(factoryCalls.Add(1)) - 1
		return sessions[index], nil
	})

	first, acquired, err := manager.TryAcquire(context.Background(), "stream:first", func() {})
	if err != nil || !acquired {
		t.Fatalf("first acquire: acquired=%v err=%v", acquired, err)
	}
	firstSession.mu.Lock()
	firstSession.failUnlock = true
	firstSession.mu.Unlock()
	if err := first.Release(context.Background()); err == nil {
		t.Fatal("expected first session failure")
	}

	second, acquired, err := manager.TryAcquire(context.Background(), "stream:second", func() {})
	if err != nil || !acquired {
		t.Fatalf("reconnect acquire: acquired=%v err=%v", acquired, err)
	}
	if factoryCalls.Load() != 2 {
		t.Fatalf("session factory calls = %d, want reconnect", factoryCalls.Load())
	}
	if err := second.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
}
