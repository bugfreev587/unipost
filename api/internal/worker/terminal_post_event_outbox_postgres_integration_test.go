//go:build integration

package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/testdbguard"
)

type postgresProbeDurableSink struct {
	pool      *pgxpool.Pool
	delivered []string
}

func (s *postgresProbeDurableSink) PublishDurable(ctx context.Context, eventID, _, _ string, _ []byte) error {
	if _, err := s.pool.Exec(ctx, `SELECT 1`); err != nil {
		return err
	}
	s.delivered = append(s.delivered, eventID)
	return nil
}

func openTerminalPostEventOutboxPool(t *testing.T, maxConns int32) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv("PUBLISHING_RESTRICTION_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Fatal("PUBLISHING_RESTRICTION_TEST_DATABASE_URL is required and must point to a temporary localhost PostgreSQL service")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, config, err := testdbguard.OpenValidated(
		"PUBLISHING_RESTRICTION_TEST_DATABASE_URL",
		databaseURL,
		func(validatedURL string) (*pgxpool.Pool, error) {
			return pgxpool.New(ctx, validatedURL)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("terminal_event_outbox_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	config.MaxConns = maxConns
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		admin.Close()
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		CREATE TABLE workspaces (id TEXT PRIMARY KEY);
		CREATE TABLE social_posts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			status TEXT NOT NULL,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb
		);
		CREATE TABLE terminal_post_first_publish_elections (
			workspace_id TEXT PRIMARY KEY REFERENCES workspaces(id) ON DELETE CASCADE,
			post_id TEXT NOT NULL,
			elected_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE terminal_post_event_outbox (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			sequence_number BIGSERIAL NOT NULL UNIQUE,
			post_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			parent_status TEXT NOT NULL,
			parent_version TEXT NOT NULL,
			event_type TEXT NOT NULL DEFAULT '',
			payload JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (post_id, parent_version)
		);
		CREATE TABLE terminal_post_event_deliveries (
			event_id TEXT NOT NULL REFERENCES terminal_post_event_outbox(id) ON DELETE CASCADE,
			sink TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			lease_expires_at TIMESTAMPTZ,
			last_error TEXT,
			enqueued_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (event_id, sink)
		);
	`); err != nil {
		pool.Close()
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = admin.Exec(cleanupCtx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		admin.Close()
	})
	return pool
}

func insertTerminalPostEventFixture(t *testing.T, pool *pgxpool.Pool, eventID, postID, version string, sinks ...string) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO terminal_post_event_outbox
			(id, post_id, workspace_id, parent_status, parent_version, event_type, payload)
		VALUES ($1, $2, 'ws_1', 'failed', $3, 'post.failed', '{"id":"post_1","status":"failed"}')
	`, eventID, postID, version); err != nil {
		t.Fatal(err)
	}
	for _, sink := range sinks {
		if _, err := pool.Exec(ctx, `INSERT INTO terminal_post_event_deliveries (event_id, sink) VALUES ($1, $2)`, eventID, sink); err != nil {
			t.Fatal(err)
		}
	}
}

func insertTerminalPostProjection(t *testing.T, pool *pgxpool.Pool, postID string) string {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO workspaces(id) VALUES ('ws_1') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts(id,workspace_id,status) VALUES ($1,'ws_1','failed')
	`, postID); err != nil {
		t.Fatal(err)
	}
	var version string
	if err := pool.QueryRow(ctx, `SELECT xmin::text FROM social_posts WHERE id=$1`, postID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

func advanceTerminalPostProjection(t *testing.T, pool *pgxpool.Pool, postID string) string {
	t.Helper()
	var version string
	if err := pool.QueryRow(context.Background(), `
		UPDATE social_posts SET status='failed' WHERE id=$1 RETURNING xmin::text
	`, postID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

func TestTerminalPostEventOutboxClaimUsesSkipLockedAndPerPostOrder(t *testing.T) {
	pool := openTerminalPostEventOutboxPool(t, 2)
	version1 := insertTerminalPostProjection(t, pool, "post_1")
	insertTerminalPostEventFixture(t, pool, "event_1", "post_1", version1, "webhook", "notification")
	queries := db.New(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := queries.WithTx(tx).ClaimPendingTerminalPostEventDeliveries(ctx, 10)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if len(claimed) != 2 || claimed[0].EventID != "event_1" || claimed[1].EventID != "event_1" {
		_ = tx.Rollback(ctx)
		t.Fatalf("tx1 claimed = %+v", claimed)
	}
	version2 := advanceTerminalPostProjection(t, pool, "post_1")
	insertTerminalPostEventFixture(t, pool, "event_2", "post_1", version2, "webhook")
	concurrent, err := queries.ClaimPendingTerminalPostEventDeliveries(ctx, 10)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if len(concurrent) != 0 {
		_ = tx.Rollback(ctx)
		t.Fatalf("tx2 bypassed locked/earlier event: %+v", concurrent)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, row := range claimed {
		affected, err := queries.MarkTerminalPostEventDeliveryEnqueued(ctx, db.MarkTerminalPostEventDeliveryEnqueuedParams{
			EventID: row.EventID, Sink: row.Sink, ExpectedAttempts: row.Attempts,
		})
		if err != nil || affected != 1 {
			t.Fatalf("mark claimed delivery affected=%d err=%v", affected, err)
		}
	}
	next, err := queries.ClaimPendingTerminalPostEventDeliveries(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(next) != 1 || next[0].EventID != "event_2" {
		t.Fatalf("next claim = %+v", next)
	}
	if _, err := pool.Exec(ctx, `UPDATE terminal_post_event_deliveries
		SET lease_expires_at=NOW()-INTERVAL '1 second'
		WHERE event_id='event_2' AND sink='webhook'`); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := queries.ClaimPendingTerminalPostEventDeliveries(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(reclaimed) != 1 || reclaimed[0].EventID != "event_2" || reclaimed[0].Attempts != 2 {
		t.Fatalf("expired lease reclaim = %+v", reclaimed)
	}
}

func TestTerminalPostEventOutboxClaimInvalidatesEventSupersededByRetry(t *testing.T) {
	pool := openTerminalPostEventOutboxPool(t, 2)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO workspaces(id) VALUES ('workspace_stale_terminal');
		INSERT INTO social_posts(id,workspace_id,status)
		VALUES ('post_stale_terminal','workspace_stale_terminal','failed');
	`); err != nil {
		t.Fatal(err)
	}
	var failedVersion string
	if err := pool.QueryRow(ctx, `
		SELECT xmin::text FROM social_posts WHERE id='post_stale_terminal'
	`).Scan(&failedVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO terminal_post_event_outbox
			(id,post_id,workspace_id,parent_status,parent_version,event_type,payload)
		VALUES
			('event_stale_terminal','post_stale_terminal','workspace_stale_terminal',
			 'failed',$1,'post.failed','{"id":"post_stale_terminal","status":"failed"}')
	`, failedVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO terminal_post_event_deliveries(event_id,sink)
		VALUES ('event_stale_terminal','webhook')
	`); err != nil {
		t.Fatal(err)
	}

	// A retry cycle moves the parent away and back before committing a newer
	// terminal generation. The earlier terminal event must not escape.
	if _, err := pool.Exec(ctx, `
		UPDATE social_posts SET status='publishing' WHERE id='post_stale_terminal'
	`); err != nil {
		t.Fatal(err)
	}
	var retryVersion string
	if err := pool.QueryRow(ctx, `
		SELECT xmin::text FROM social_posts WHERE id='post_stale_terminal'
	`).Scan(&retryVersion); err != nil {
		t.Fatal(err)
	}
	if retryVersion == failedVersion {
		t.Fatalf("retry did not advance parent version: before=%q after=%q", failedVersion, retryVersion)
	}
	var replacementVersion string
	if err := pool.QueryRow(ctx, `
		UPDATE social_posts SET status='failed'
		WHERE id='post_stale_terminal'
		RETURNING xmin::text
	`).Scan(&replacementVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO terminal_post_event_outbox
			(id,post_id,workspace_id,parent_status,parent_version,event_type,payload)
		VALUES
			('event_current_terminal','post_stale_terminal','workspace_stale_terminal',
			 'failed',$1,'post.failed','{"id":"post_stale_terminal","status":"failed"}')
	`, replacementVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO terminal_post_event_deliveries(event_id,sink)
		VALUES ('event_current_terminal','webhook')
	`); err != nil {
		t.Fatal(err)
	}

	claimed, err := db.New(pool).ClaimPendingTerminalPostEventDeliveries(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].EventID != "event_current_terminal" || claimed[0].ParentVersion != replacementVersion {
		t.Fatalf("claim returned stale event or lost current event: %+v", claimed)
	}
	var status string
	var lastError pgtype.Text
	if err := pool.QueryRow(ctx, `
		SELECT status,last_error
		FROM terminal_post_event_deliveries
		WHERE event_id='event_stale_terminal' AND sink='webhook'
	`).Scan(&status, &lastError); err != nil {
		t.Fatal(err)
	}
	if status != "enqueued" || !lastError.Valid || lastError.String != "invalidated: parent projection superseded" {
		t.Fatalf("stale delivery status=%q last_error=%+v", status, lastError)
	}
}

func TestTerminalPostEventOutboxClaimKeepsTerminalEventAfterSameStatusMetadataUpdate(t *testing.T) {
	pool := openTerminalPostEventOutboxPool(t, 2)
	ctx := context.Background()
	version := insertTerminalPostProjection(t, pool, "post_metadata_update")
	insertTerminalPostEventFixture(t, pool, "event_metadata_update", "post_metadata_update", version, "webhook")

	var updatedVersion string
	if err := pool.QueryRow(ctx, `
		UPDATE social_posts
		SET metadata=jsonb_build_object('error_summary','redacted provider detail')
		WHERE id='post_metadata_update'
		RETURNING xmin::text
	`).Scan(&updatedVersion); err != nil {
		t.Fatal(err)
	}
	if updatedVersion == version {
		t.Fatalf("metadata update did not advance xmin: before=%q after=%q", version, updatedVersion)
	}

	claimed, err := db.New(pool).ClaimPendingTerminalPostEventDeliveries(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].EventID != "event_metadata_update" {
		t.Fatalf("same-status metadata update invalidated terminal event: %+v", claimed)
	}
}

func TestTerminalPostEventOutboxReleasesSmallPoolBeforeSinkIO(t *testing.T) {
	pool := openTerminalPostEventOutboxPool(t, 1)
	version := insertTerminalPostProjection(t, pool, "post_small_pool")
	insertTerminalPostEventFixture(t, pool, "event_small_pool", "post_small_pool", version, "webhook")
	sink := &postgresProbeDurableSink{pool: pool}
	worker := NewTerminalPostEventOutboxWorker(db.New(pool), sink, sink, sink)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	worker.tick(ctx)
	if err := ctx.Err(); err != nil {
		t.Fatalf("sink DB re-entry exhausted MaxConns=1: %v", err)
	}
	if len(sink.delivered) != 1 || sink.delivered[0] != "event_small_pool" {
		t.Fatalf("delivered = %v", sink.delivered)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM terminal_post_event_deliveries WHERE event_id='event_small_pool' AND sink='webhook'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "enqueued" {
		t.Fatalf("delivery status = %q", status)
	}
}

func TestTerminalPostEventOutboxStaleClaimCannotOverwriteReclaimedDelivery(t *testing.T) {
	t.Run("stale retry cannot reopen successful reclaim", func(t *testing.T) {
		pool := openTerminalPostEventOutboxPool(t, 2)
		version1 := insertTerminalPostProjection(t, pool, "post_generation")
		insertTerminalPostEventFixture(t, pool, "event_generation", "post_generation", version1, "webhook")
		queries := db.New(pool)
		ctx := context.Background()

		claimA, err := queries.ClaimPendingTerminalPostEventDeliveries(ctx, 1)
		if err != nil || len(claimA) != 1 || claimA[0].Attempts != 1 {
			t.Fatalf("claim A = %+v, err=%v", claimA, err)
		}
		if _, err := pool.Exec(ctx, `UPDATE terminal_post_event_deliveries
			SET lease_expires_at=NOW()-INTERVAL '1 second'
			WHERE event_id='event_generation' AND sink='webhook'`); err != nil {
			t.Fatal(err)
		}
		claimB, err := queries.ClaimPendingTerminalPostEventDeliveries(ctx, 1)
		if err != nil || len(claimB) != 1 || claimB[0].Attempts != 2 {
			t.Fatalf("claim B = %+v, err=%v", claimB, err)
		}
		affected, err := queries.MarkTerminalPostEventDeliveryEnqueued(ctx, db.MarkTerminalPostEventDeliveryEnqueuedParams{
			EventID: claimB[0].EventID, Sink: claimB[0].Sink, ExpectedAttempts: claimB[0].Attempts,
		})
		if err != nil || affected != 1 {
			t.Fatalf("mark B success affected=%d err=%v", affected, err)
		}
		affected, err = queries.RetryTerminalPostEventDelivery(ctx, db.RetryTerminalPostEventDeliveryParams{
			EventID: claimA[0].EventID, Sink: claimA[0].Sink, ExpectedAttempts: claimA[0].Attempts,
			NextAttemptAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			LastError:     pgtype.Text{String: "stale A", Valid: true},
		})
		if err != nil || affected != 0 {
			t.Fatalf("stale retry affected=%d err=%v", affected, err)
		}
		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM terminal_post_event_deliveries
			WHERE event_id='event_generation' AND sink='webhook'`).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "enqueued" {
			t.Fatalf("stale retry reopened reclaimed success: status=%q", status)
		}
		version2 := advanceTerminalPostProjection(t, pool, "post_generation")
		insertTerminalPostEventFixture(t, pool, "event_next", "post_generation", version2, "webhook")
		next, err := queries.ClaimPendingTerminalPostEventDeliveries(ctx, 1)
		if err != nil || len(next) != 1 || next[0].EventID != "event_next" {
			t.Fatalf("next version remained blocked after true success: %+v, err=%v", next, err)
		}
	})

	t.Run("stale success cannot finish active reclaim", func(t *testing.T) {
		pool := openTerminalPostEventOutboxPool(t, 2)
		version1 := insertTerminalPostProjection(t, pool, "post_active")
		insertTerminalPostEventFixture(t, pool, "event_active", "post_active", version1, "webhook")
		queries := db.New(pool)
		ctx := context.Background()

		claimA, err := queries.ClaimPendingTerminalPostEventDeliveries(ctx, 1)
		if err != nil || len(claimA) != 1 {
			t.Fatalf("claim A = %+v, err=%v", claimA, err)
		}
		if _, err := pool.Exec(ctx, `UPDATE terminal_post_event_deliveries
			SET lease_expires_at=NOW()-INTERVAL '1 second'
			WHERE event_id='event_active' AND sink='webhook'`); err != nil {
			t.Fatal(err)
		}
		claimB, err := queries.ClaimPendingTerminalPostEventDeliveries(ctx, 1)
		if err != nil || len(claimB) != 1 || claimB[0].Attempts != 2 {
			t.Fatalf("claim B = %+v, err=%v", claimB, err)
		}
		affected, err := queries.MarkTerminalPostEventDeliveryEnqueued(ctx, db.MarkTerminalPostEventDeliveryEnqueuedParams{
			EventID: claimA[0].EventID, Sink: claimA[0].Sink, ExpectedAttempts: claimA[0].Attempts,
		})
		if err != nil || affected != 0 {
			t.Fatalf("stale success affected=%d err=%v", affected, err)
		}
		var status string
		var attempts int
		if err := pool.QueryRow(ctx, `SELECT status,attempts FROM terminal_post_event_deliveries
			WHERE event_id='event_active' AND sink='webhook'`).Scan(&status, &attempts); err != nil {
			t.Fatal(err)
		}
		if status != "processing" || attempts != 2 {
			t.Fatalf("stale success finished active reclaim: status=%q attempts=%d", status, attempts)
		}
		version2 := advanceTerminalPostProjection(t, pool, "post_active")
		insertTerminalPostEventFixture(t, pool, "event_active_next", "post_active", version2, "webhook")
		blocked, err := queries.ClaimPendingTerminalPostEventDeliveries(ctx, 1)
		if err != nil || len(blocked) != 0 {
			t.Fatalf("next version bypassed active reclaim: %+v, err=%v", blocked, err)
		}
	})
}

func TestConcurrentFirstPostElectionProducesOneLoopsLifecycleSend(t *testing.T) {
	pool := openTerminalPostEventOutboxPool(t, 4)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `
		INSERT INTO workspaces(id) VALUES ('workspace_first_election');
		INSERT INTO social_posts(id,workspace_id,status) VALUES
		 ('post_first_a','workspace_first_election','publishing'),
		 ('post_first_b','workspace_first_election','publishing');
	`); err != nil {
		t.Fatal(err)
	}
	txA, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer txA.Rollback(ctx)
	txB, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer txB.Rollback(ctx)
	if _, err := txA.Exec(ctx, `UPDATE social_posts SET status='published' WHERE id='post_first_a'`); err != nil {
		t.Fatal(err)
	}
	if _, err := txB.Exec(ctx, `UPDATE social_posts SET status='published' WHERE id='post_first_b'`); err != nil {
		t.Fatal(err)
	}
	var versionA, versionB string
	if err := txA.QueryRow(ctx, `SELECT xmin::text FROM social_posts WHERE id='post_first_a'`).Scan(&versionA); err != nil {
		t.Fatal(err)
	}
	if err := txB.QueryRow(ctx, `SELECT xmin::text FROM social_posts WHERE id='post_first_b'`).Scan(&versionB); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordPostStatusTransition(ctx, db.New(txA), "post_first_a", "workspace_first_election", "published", versionA, events.EventPostPublished, map[string]any{"id": "post_first_a"}); err != nil {
		t.Fatal(err)
	}
	type recordResult struct{ err error }
	recordedB := make(chan recordResult, 1)
	go func() {
		recordedB <- recordResult{err: db.RecordPostStatusTransition(ctx, db.New(txB), "post_first_b", "workspace_first_election", "published", versionB, events.EventPostPublished, map[string]any{"id": "post_first_b"})}
	}()
	committedA := false
	select {
	case result := <-recordedB:
		if result.err != nil {
			t.Fatal(result.err)
		}
	case <-time.After(100 * time.Millisecond):
		if err := txA.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		committedA = true
		if result := <-recordedB; result.err != nil {
			t.Fatal(result.err)
		}
	}
	if !committedA {
		if err := txA.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := txB.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	rows, err := pool.Query(ctx, `SELECT id,payload FROM terminal_post_event_outbox ORDER BY sequence_number`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	store := &fakeDurableLoopsStore{
		workspace: db.Workspace{ID: "workspace_first_election", UserID: "owner_first_election", Name: "Workspace"},
		owner:     db.User{ID: "owner_first_election", Email: "owner@example.com"},
	}
	sender := &deduplicatingLifecycleSender{}
	bus := &LoopsNotificationEmailBus{queries: store, syncer: sender, appBaseURL: "https://app.unipost.dev"}
	var snapshots []bool
	for rows.Next() {
		var eventID string
		var payload []byte
		if err := rows.Scan(&eventID, &payload); err != nil {
			t.Fatal(err)
		}
		var body map[string]any
		if err := json.Unmarshal(payload, &body); err != nil {
			t.Fatal(err)
		}
		snapshots = append(snapshots, body["first_post_published"].(bool))
		if err := bus.PublishDurable(ctx, eventID, "workspace_first_election", events.EventPostPublished, payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	trueCount := 0
	for _, snapshot := range snapshots {
		if snapshot {
			trueCount++
		}
	}
	if len(snapshots) != 2 || trueCount != 1 {
		t.Fatalf("first-post snapshots = %v, want exactly one winner", snapshots)
	}
	if sender.realSends != 1 || len(sender.attempts) != 1 || sender.attempts[0].IdempotencyKey != "first_post_published:workspace_first_election" {
		t.Fatalf("Loops sends=%d attempts=%+v", sender.realSends, sender.attempts)
	}
}

func TestWorkspaceFirstPostElectionRollbackAndHistoricalSafety(t *testing.T) {
	t.Run("rolled back winner releases election", func(t *testing.T) {
		pool := openTerminalPostEventOutboxPool(t, 4)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(ctx, `
			INSERT INTO workspaces(id) VALUES ('workspace_election_rollback');
			INSERT INTO social_posts(id,workspace_id,status) VALUES
			 ('post_rollback_a','workspace_election_rollback','publishing'),
			 ('post_rollback_b','workspace_election_rollback','publishing');
		`); err != nil {
			t.Fatal(err)
		}
		txA, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer txA.Rollback(ctx)
		txB, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer txB.Rollback(ctx)
		if _, err := txA.Exec(ctx, `UPDATE social_posts SET status='published' WHERE id='post_rollback_a'`); err != nil {
			t.Fatal(err)
		}
		if _, err := txB.Exec(ctx, `UPDATE social_posts SET status='published' WHERE id='post_rollback_b'`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.New(txA).ElectWorkspaceFirstPostPublished(ctx, db.ElectWorkspaceFirstPostPublishedParams{
			WorkspaceID: "workspace_election_rollback", PostID: "post_rollback_a",
		}); err != nil {
			t.Fatal(err)
		}
		type electionResult struct {
			workspaceID string
			err         error
		}
		resultB := make(chan electionResult, 1)
		go func() {
			workspaceID, err := db.New(txB).ElectWorkspaceFirstPostPublished(ctx, db.ElectWorkspaceFirstPostPublishedParams{
				WorkspaceID: "workspace_election_rollback", PostID: "post_rollback_b",
			})
			resultB <- electionResult{workspaceID: workspaceID, err: err}
		}()
		select {
		case early := <-resultB:
			t.Fatalf("second election did not serialize behind uncommitted winner: %+v", early)
		case <-time.After(100 * time.Millisecond):
		}
		if err := txA.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		winnerB := <-resultB
		if winnerB.err != nil || winnerB.workspaceID != "workspace_election_rollback" {
			t.Fatalf("replacement election = %+v", winnerB)
		}
		if err := txB.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var electedPost string
		if err := pool.QueryRow(ctx, `SELECT post_id FROM terminal_post_first_publish_elections
			WHERE workspace_id='workspace_election_rollback'`).Scan(&electedPost); err != nil {
			t.Fatal(err)
		}
		if electedPost != "post_rollback_b" {
			t.Fatalf("elected post = %q", electedPost)
		}
	})

	t.Run("historical workspace with multiple published posts is not elected", func(t *testing.T) {
		pool := openTerminalPostEventOutboxPool(t, 2)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `
			INSERT INTO workspaces(id) VALUES ('workspace_historical');
			INSERT INTO social_posts(id,workspace_id,status) VALUES
			 ('post_historical_1','workspace_historical','published'),
			 ('post_historical_2','workspace_historical','published');
		`); err != nil {
			t.Fatal(err)
		}
		_, err := db.New(pool).ElectWorkspaceFirstPostPublished(ctx, db.ElectWorkspaceFirstPostPublishedParams{
			WorkspaceID: "workspace_historical", PostID: "post_historical_2",
		})
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("historical election error = %v, want no winner", err)
		}
		var elections int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM terminal_post_first_publish_elections`).Scan(&elections); err != nil {
			t.Fatal(err)
		}
		if elections != 0 {
			t.Fatalf("historical workspace elections = %d", elections)
		}
	})
}
