// Package postfailuredebug persists bounded publishing diagnostics outside
// customer publishing transactions. Enqueue is deliberately non-blocking and
// cannot report an error back into publishing control flow.
package postfailuredebug

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	MaxDetailBytes    = 64 * 1024
	defaultQueueSize  = 512
	defaultWriteLimit = 2 * time.Second
	defaultDrainLimit = 3 * time.Second
	defaultRedaction  = 1
)

type Detail struct {
	SocialPostResultID string
	WorkspaceID        string
	DebugText          string
	CapturedAt         time.Time
	RedactionVersion   int
}

type StoredDetail struct {
	Detail
	OriginalBytes int
	StoredBytes   int
	Truncated     bool
}

type Store interface {
	UpsertPostFailureDebug(context.Context, StoredDetail) error
}

type Config struct {
	QueueSize    int
	WriteTimeout time.Duration
	DrainTimeout time.Duration
}

type Stats struct {
	QueueDepth int
	Dropped    uint64
	Failures   uint64
	Writes     uint64
	Abandoned  uint64
}

type Writer struct {
	store        Store
	queue        chan StoredDetail
	writeTimeout time.Duration
	drainTimeout time.Duration
	done         chan struct{}
	stop         chan struct{}
	admissionMu  sync.RWMutex
	accepting    bool
	started      atomic.Bool
	dropped      atomic.Uint64
	failures     atomic.Uint64
	writes       atomic.Uint64
	abandoned    atomic.Uint64
}

func NewWriter(store Store, config Config) *Writer {
	queueSize := config.QueueSize
	if queueSize <= 0 {
		queueSize = defaultQueueSize
	}
	writeTimeout := config.WriteTimeout
	if writeTimeout <= 0 {
		writeTimeout = defaultWriteLimit
	}
	drainTimeout := config.DrainTimeout
	if drainTimeout <= 0 {
		drainTimeout = defaultDrainLimit
	}
	return &Writer{
		store:        store,
		queue:        make(chan StoredDetail, queueSize),
		writeTimeout: writeTimeout,
		drainTimeout: drainTimeout,
		done:         make(chan struct{}),
		stop:         make(chan struct{}),
		accepting:    true,
	}
}

// Enqueue accepts only complete, non-empty details and always returns
// immediately. Queue saturation is observable but never blocks publishing.
func (w *Writer) Enqueue(detail Detail) {
	if w == nil || w.store == nil || strings.TrimSpace(detail.SocialPostResultID) == "" ||
		strings.TrimSpace(detail.WorkspaceID) == "" || detail.DebugText == "" {
		return
	}
	stored := normalize(detail)
	w.admissionMu.RLock()
	defer w.admissionMu.RUnlock()
	if !w.accepting {
		w.abandoned.Add(1)
		return
	}
	select {
	case w.queue <- stored:
	default:
		// Do not call a remote logging handler from the publishing goroutine.
		// The worker reports this counter on its own ticker.
		w.dropped.Add(1)
	}
}

func (w *Writer) Start(ctx context.Context) {
	if w == nil || w.store == nil {
		return
	}
	if !w.started.CompareAndSwap(false, true) {
		return
	}
	defer close(w.done)
	slog.Info("post failure debug writer started", "queue_size", cap(w.queue))
	reportTicker := time.NewTicker(30 * time.Second)
	defer reportTicker.Stop()
	var reportedDropped uint64
	ctxDone := ctx.Done()
	for {
		select {
		case <-ctxDone:
			w.Stop()
			ctxDone = nil
		case <-w.stop:
			drained, abandoned := w.drainWithin(w.drainTimeout)
			slog.Info("post failure debug writer stopped",
				"drained", drained,
				"abandoned", abandoned,
				"dropped", w.dropped.Load(),
				"failures", w.failures.Load(),
			)
			return
		case detail := <-w.queue:
			w.write(ctx, detail, w.writeTimeout)
		case <-reportTicker.C:
			dropped := w.dropped.Load()
			if dropped > reportedDropped {
				slog.Warn("post_failure_debug_dropped",
					"queue_size", cap(w.queue),
					"new_drops", dropped-reportedDropped,
					"total_dropped", dropped,
				)
				reportedDropped = dropped
			}
		}
	}
}

// Stop atomically closes admission before asking the worker to drain. Any
// producer that races with shutdown is counted as abandoned instead of being
// accepted into a queue that no longer has a consumer.
func (w *Writer) Stop() {
	if w == nil {
		return
	}
	w.admissionMu.Lock()
	defer w.admissionMu.Unlock()
	if !w.accepting {
		return
	}
	w.accepting = false
	close(w.stop)
}

func (w *Writer) Stats() Stats {
	if w == nil {
		return Stats{}
	}
	return Stats{
		QueueDepth: len(w.queue),
		Dropped:    w.dropped.Load(),
		Failures:   w.failures.Load(),
		Writes:     w.writes.Load(),
		Abandoned:  w.abandoned.Load(),
	}
}

func (w *Writer) Wait(ctx context.Context) error {
	if w == nil || !w.started.Load() {
		return nil
	}
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *Writer) drainWithin(limit time.Duration) (int, uint64) {
	deadline := time.Now().Add(limit)
	drained := 0
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			abandoned := uint64(len(w.queue))
			w.abandoned.Add(abandoned)
			return drained, abandoned
		}
		select {
		case detail := <-w.queue:
			writeLimit := w.writeTimeout
			if remaining < writeLimit {
				writeLimit = remaining
			}
			w.write(context.Background(), detail, writeLimit)
			drained++
		default:
			return drained, 0
		}
	}
}

func (w *Writer) write(parent context.Context, detail StoredDetail, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	if err := w.store.UpsertPostFailureDebug(ctx, detail); err != nil {
		failures := w.failures.Add(1)
		slog.Warn("post_failure_debug_write_failed",
			"workspace_id", detail.WorkspaceID,
			"social_post_result_id", detail.SocialPostResultID,
			"error", err,
			"total_failures", failures,
		)
		return
	}
	w.writes.Add(1)
}

func normalize(detail Detail) StoredDetail {
	originalBytes := len(detail.DebugText)
	const marker = "\n# diagnostic truncated at 65536 bytes"
	raw := detail.DebugText
	truncated := false
	if len(raw) > MaxDetailBytes-len(marker) {
		raw = raw[:MaxDetailBytes-len(marker)]
		truncated = true
	}
	text := strings.ToValidUTF8(raw, "�")
	text = strings.ReplaceAll(text, "\x00", "")
	truncated = truncated || text != detail.DebugText
	if truncated || len(text) > MaxDetailBytes {
		prefix := text
		if len(prefix) > MaxDetailBytes-len(marker) {
			prefix = prefix[:MaxDetailBytes-len(marker)]
		}
		for !utf8.ValidString(prefix) {
			prefix = prefix[:len(prefix)-1]
		}
		text = prefix + marker
		truncated = true
	}
	if detail.CapturedAt.IsZero() {
		detail.CapturedAt = time.Now().UTC()
	}
	if detail.RedactionVersion <= 0 {
		detail.RedactionVersion = defaultRedaction
	}
	detail.DebugText = text
	return StoredDetail{
		Detail:        detail,
		OriginalBytes: originalBytes,
		StoredBytes:   len(text),
		Truncated:     truncated,
	}
}

type postgresExecer interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type PostgresStore struct {
	db postgresExecer
}

func NewPostgresStore(db postgresExecer) *PostgresStore {
	return &PostgresStore{db: db}
}

const upsertPostFailureDebugSQL = `
INSERT INTO post_failure_debug_details (
  social_post_result_id,
  workspace_id,
  debug_text,
  original_bytes,
  stored_bytes,
  truncated,
  redaction_version,
  captured_at,
  updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
ON CONFLICT (social_post_result_id) DO UPDATE SET
  workspace_id = EXCLUDED.workspace_id,
  debug_text = EXCLUDED.debug_text,
  original_bytes = EXCLUDED.original_bytes,
  stored_bytes = EXCLUDED.stored_bytes,
  truncated = EXCLUDED.truncated,
  redaction_version = EXCLUDED.redaction_version,
  captured_at = EXCLUDED.captured_at,
  updated_at = NOW()`

func (s *PostgresStore) UpsertPostFailureDebug(ctx context.Context, detail StoredDetail) error {
	if s == nil || s.db == nil {
		return context.Canceled
	}
	_, err := s.db.Exec(ctx, upsertPostFailureDebugSQL,
		detail.SocialPostResultID,
		detail.WorkspaceID,
		detail.DebugText,
		detail.OriginalBytes,
		detail.StoredBytes,
		detail.Truncated,
		detail.RedactionVersion,
		detail.CapturedAt,
	)
	return err
}
