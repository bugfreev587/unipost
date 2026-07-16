package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

type xInboxLockRow interface {
	Scan(...any) error
}

type xInboxLockSession interface {
	QueryRow(context.Context, string, ...any) xInboxLockRow
	Close(context.Context) error
}

type xInboxLockSessionFactory func(context.Context) (xInboxLockSession, error)

type ownedXInboxLock struct {
	cancel context.CancelFunc
}

type xInboxLockTimeouts struct {
	gate    time.Duration
	connect time.Duration
	query   time.Duration
	close   time.Duration
}

var defaultXInboxLockTimeouts = xInboxLockTimeouts{
	gate:    5 * time.Second,
	connect: 5 * time.Second,
	query:   5 * time.Second,
	close:   5 * time.Second,
}

type PostgresStreamLockManager struct {
	gate       chan struct{}
	factory    xInboxLockSessionFactory
	timeouts   xInboxLockTimeouts
	session    xInboxLockSession
	generation uint64
	owned      map[string]ownedXInboxLock
	closed     bool
}

func NewPostgresStreamLockManager(databaseURL string) *PostgresStreamLockManager {
	return newPostgresStreamLockManager(func(ctx context.Context) (xInboxLockSession, error) {
		conn, err := pgx.Connect(ctx, databaseURL)
		if err != nil {
			return nil, err
		}
		return &pgxInboxLockSession{conn: conn}, nil
	})
}

type pgxInboxLockSession struct {
	conn *pgx.Conn
}

func (s *pgxInboxLockSession) QueryRow(
	ctx context.Context,
	query string,
	args ...any,
) xInboxLockRow {
	return s.conn.QueryRow(ctx, query, args...)
}

func (s *pgxInboxLockSession) Close(ctx context.Context) error {
	return s.conn.Close(ctx)
}

func newPostgresStreamLockManager(factory xInboxLockSessionFactory) *PostgresStreamLockManager {
	return newPostgresStreamLockManagerWithTimeouts(factory, defaultXInboxLockTimeouts)
}

func newPostgresStreamLockManagerWithTimeouts(
	factory xInboxLockSessionFactory,
	timeouts xInboxLockTimeouts,
) *PostgresStreamLockManager {
	return &PostgresStreamLockManager{
		factory: factory,
		gate:    make(chan struct{}, 1),
		timeouts: xInboxLockTimeouts{
			gate:    positiveDuration(timeouts.gate, defaultXInboxLockTimeouts.gate),
			connect: positiveDuration(timeouts.connect, defaultXInboxLockTimeouts.connect),
			query:   positiveDuration(timeouts.query, defaultXInboxLockTimeouts.query),
			close:   positiveDuration(timeouts.close, defaultXInboxLockTimeouts.close),
		},
		owned: make(map[string]ownedXInboxLock),
	}
}

func positiveDuration(value, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

func (m *PostgresStreamLockManager) TryAcquire(
	ctx context.Context,
	lockKey string,
	cancel context.CancelFunc,
) (XInboxLeaderLease, bool, error) {
	releaseGate, err := m.acquireGate(ctx)
	if err != nil {
		return nil, false, err
	}
	defer releaseGate()
	if m.closed {
		return nil, false, errors.New("X inbox lock manager is closed")
	}
	if _, owned := m.owned[lockKey]; owned {
		return nil, false, nil
	}
	if m.session == nil {
		session, err := m.connect(ctx)
		if err != nil {
			return nil, false, m.invalidateLocked(err)
		}
		m.session = session
		m.generation++
	}
	acquired, err := m.queryBool(
		ctx,
		`SELECT pg_try_advisory_lock(hashtextextended($1, 0))`,
		lockKey,
	)
	if err != nil {
		return nil, false, m.invalidateLocked(err)
	}
	if !acquired {
		return nil, false, nil
	}
	m.owned[lockKey] = ownedXInboxLock{cancel: cancel}
	return &postgresStreamLockLease{
		manager:    m,
		lockKey:    lockKey,
		generation: m.generation,
	}, true, nil
}

func (m *PostgresStreamLockManager) Close() error {
	releaseGate, err := m.acquireGate(context.Background())
	if err != nil {
		return err
	}
	defer releaseGate()
	if m.closed {
		return nil
	}
	m.closed = true
	return m.invalidateLocked(nil)
}

func (m *PostgresStreamLockManager) release(
	ctx context.Context,
	lockKey string,
	generation uint64,
) error {
	releaseGate, err := m.acquireGate(ctx)
	if err != nil {
		return err
	}
	defer releaseGate()
	if generation != m.generation || m.session == nil {
		return nil
	}
	if _, owned := m.owned[lockKey]; !owned {
		return nil
	}
	released, err := m.queryBool(
		ctx,
		`SELECT pg_advisory_unlock(hashtextextended($1, 0))`,
		lockKey,
	)
	if err != nil {
		return m.invalidateLocked(err)
	}
	if !released {
		return m.invalidateLocked(errors.New("X inbox advisory lock was not held by dedicated session"))
	}
	delete(m.owned, lockKey)
	return nil
}

func (m *PostgresStreamLockManager) invalidateLocked(cause error) error {
	for _, lock := range m.owned {
		if lock.cancel != nil {
			lock.cancel()
		}
	}
	clear(m.owned)
	session := m.session
	m.session = nil
	m.generation++
	if session != nil {
		cause = errors.Join(cause, m.closeSession(session))
	}
	return cause
}

func (m *PostgresStreamLockManager) acquireGate(ctx context.Context) (func(), error) {
	waitCtx, cancel := context.WithTimeout(ctx, m.timeouts.gate)
	select {
	case m.gate <- struct{}{}:
		cancel()
		return func() { <-m.gate }, nil
	case <-waitCtx.Done():
		err := waitCtx.Err()
		cancel()
		return nil, fmt.Errorf("wait for X inbox lock session: %w", err)
	}
}

type xInboxSessionResult struct {
	session xInboxLockSession
	err     error
}

func (m *PostgresStreamLockManager) connect(ctx context.Context) (xInboxLockSession, error) {
	connectCtx, cancel := context.WithTimeout(ctx, m.timeouts.connect)
	defer cancel()
	result := make(chan xInboxSessionResult, 1)
	go func() {
		session, err := m.factory(connectCtx)
		result <- xInboxSessionResult{session: session, err: err}
	}()
	select {
	case connected := <-result:
		if connected.err != nil {
			return nil, fmt.Errorf("connect dedicated X inbox lock session: %w", connected.err)
		}
		return connected.session, nil
	case <-connectCtx.Done():
		go func() {
			connected := <-result
			if connected.session != nil {
				_ = m.closeSession(connected.session)
			}
		}()
		return nil, fmt.Errorf("connect dedicated X inbox lock session: %w", connectCtx.Err())
	}
}

type xInboxBoolQueryResult struct {
	value bool
	err   error
}

func (m *PostgresStreamLockManager) queryBool(
	ctx context.Context,
	query string,
	args ...any,
) (bool, error) {
	queryCtx, cancel := context.WithTimeout(ctx, m.timeouts.query)
	defer cancel()
	session := m.session
	result := make(chan xInboxBoolQueryResult, 1)
	go func() {
		var value bool
		err := session.QueryRow(queryCtx, query, args...).Scan(&value)
		result <- xInboxBoolQueryResult{value: value, err: err}
	}()
	select {
	case queried := <-result:
		if queried.err != nil {
			return false, fmt.Errorf("query dedicated X inbox lock session: %w", queried.err)
		}
		return queried.value, nil
	case <-queryCtx.Done():
		return false, fmt.Errorf("query dedicated X inbox lock session: %w", queryCtx.Err())
	}
}

func (m *PostgresStreamLockManager) closeSession(session xInboxLockSession) error {
	closeCtx, cancel := context.WithTimeout(context.Background(), m.timeouts.close)
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- session.Close(closeCtx)
	}()
	select {
	case err := <-result:
		if err != nil {
			return fmt.Errorf("close dedicated X inbox lock session: %w", err)
		}
		return nil
	case <-closeCtx.Done():
		return fmt.Errorf("close dedicated X inbox lock session: %w", closeCtx.Err())
	}
}

type postgresStreamLockLease struct {
	manager    *PostgresStreamLockManager
	lockKey    string
	generation uint64
	once       sync.Once
	err        error
}

func (l *postgresStreamLockLease) Release(ctx context.Context) error {
	l.once.Do(func() {
		l.err = l.manager.release(ctx, l.lockKey, l.generation)
	})
	return l.err
}
