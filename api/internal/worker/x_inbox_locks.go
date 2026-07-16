package worker

import (
	"context"
	"errors"
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

type PostgresStreamLockManager struct {
	mu         sync.Mutex
	factory    xInboxLockSessionFactory
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
	return &PostgresStreamLockManager{
		factory: factory,
		owned:   make(map[string]ownedXInboxLock),
	}
}

func (m *PostgresStreamLockManager) TryAcquire(
	ctx context.Context,
	lockKey string,
	cancel context.CancelFunc,
) (XInboxLeaderLease, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, false, errors.New("X inbox lock manager is closed")
	}
	if _, owned := m.owned[lockKey]; owned {
		return nil, false, nil
	}
	if m.session == nil {
		session, err := m.factory(ctx)
		if err != nil {
			return nil, false, err
		}
		m.session = session
		m.generation++
	}
	var acquired bool
	if err := m.session.QueryRow(
		ctx,
		`SELECT pg_try_advisory_lock(hashtextextended($1, 0))`,
		lockKey,
	).Scan(&acquired); err != nil {
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
	m.mu.Lock()
	defer m.mu.Unlock()
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
	m.mu.Lock()
	defer m.mu.Unlock()
	if generation != m.generation || m.session == nil {
		return nil
	}
	if _, owned := m.owned[lockKey]; !owned {
		return nil
	}
	var released bool
	if err := m.session.QueryRow(
		ctx,
		`SELECT pg_advisory_unlock(hashtextextended($1, 0))`,
		lockKey,
	).Scan(&released); err != nil {
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
	if m.session != nil {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		closeErr := m.session.Close(closeCtx)
		cancel()
		cause = errors.Join(cause, closeErr)
		m.session = nil
		m.generation++
	}
	return cause
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
