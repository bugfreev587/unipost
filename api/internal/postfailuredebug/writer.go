// Package postfailuredebug persists bounded publishing diagnostics outside
// customer publishing transactions. Enqueue is deliberately non-blocking and
// cannot report an error back into publishing control flow.
package postfailuredebug

import (
	"context"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	MaxDetailBytes    = 64 * 1024
	defaultQueueSize  = 512
	defaultWriteLimit = 2 * time.Second
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
}

type Stats struct {
	QueueDepth int
	Dropped    uint64
	Failures   uint64
	Writes     uint64
}

type Writer struct {
	store        Store
	queue        chan StoredDetail
	writeTimeout time.Duration
	dropped      atomic.Uint64
	failures     atomic.Uint64
	writes       atomic.Uint64
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
	return &Writer{
		store:        store,
		queue:        make(chan StoredDetail, queueSize),
		writeTimeout: writeTimeout,
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
	slog.Info("post failure debug writer started", "queue_size", cap(w.queue))
	reportTicker := time.NewTicker(30 * time.Second)
	defer reportTicker.Stop()
	var reportedDropped uint64
	for {
		select {
		case <-ctx.Done():
			drained := w.drain()
			slog.Info("post failure debug writer stopped", "drained", drained)
			return
		case detail := <-w.queue:
			w.write(detail)
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

func (w *Writer) Stats() Stats {
	if w == nil {
		return Stats{}
	}
	return Stats{
		QueueDepth: len(w.queue),
		Dropped:    w.dropped.Load(),
		Failures:   w.failures.Load(),
		Writes:     w.writes.Load(),
	}
}

func (w *Writer) drain() int {
	drained := 0
	for {
		select {
		case detail := <-w.queue:
			w.write(detail)
			drained++
		default:
			return drained
		}
	}
}

func (w *Writer) write(detail StoredDetail) {
	ctx, cancel := context.WithTimeout(context.Background(), w.writeTimeout)
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
	text := strings.ToValidUTF8(detail.DebugText, "�")
	text = strings.ReplaceAll(text, "\x00", "")
	truncated := text != detail.DebugText
	if len(text) > MaxDetailBytes {
		const marker = "\n# diagnostic truncated at 65536 bytes"
		prefix := text[:MaxDetailBytes-len(marker)]
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
