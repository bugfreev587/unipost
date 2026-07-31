//go:build integration

package db

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const publishingPullObjectRaceKey = "pull/test/race-object.mp4"

func TestPublishingPullObjectClaimCoordinatesWithReservations(t *testing.T) {
	t.Run("reservation owns object lock before cleanup", func(t *testing.T) {
		pool := openPublishingRestrictionIntegrationPool(t)
		setupPublishingPullObjectConcurrencySchema(t, pool)
		seedExpiredPublishingPullObject(t, pool)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		reservationConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire reservation connection: %v", err)
		}
		defer reservationConn.Release()
		cleanupConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire cleanup connection: %v", err)
		}
		defer cleanupConn.Release()

		reservationTx, err := reservationConn.Begin(ctx)
		if err != nil {
			t.Fatalf("begin reservation transaction: %v", err)
		}
		defer reservationTx.Rollback(context.Background())
		if _, err := reservationTx.Exec(ctx, `
			UPDATE publishing_pull_objects
			SET updated_at = NOW()
			WHERE object_key = $1
		`, publishingPullObjectRaceKey); err != nil {
			t.Fatalf("lock publishing object for reservation: %v", err)
		}

		if _, err := reservationTx.Exec(ctx, `
			INSERT INTO publishing_pull_object_usages (
				object_key, workspace_id, post_id, post_status,
				cleanup_after_at, retention_reason
			) VALUES ($1, 'workspace-1', 'post-active', 'publishing', NULL, 'active_post')
		`, publishingPullObjectRaceKey); err != nil {
			t.Fatalf("insert active usage while reservation owns object lock: %v", err)
		}
		rows, err := New(cleanupConn).ClaimPublishingPullObjectsDue(ctx, 10)
		if err != nil {
			t.Fatalf("claim while reservation owns object lock: %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("cleanup claimed %d objects while an active reservation owned the lock, want 0", len(rows))
		}
		if err := reservationTx.Commit(ctx); err != nil {
			t.Fatalf("commit winning reservation: %v", err)
		}
		assertPublishingPullObjectState(t, pool, "active", 1)
	})

	t.Run("cleanup owns object lock before reservation", func(t *testing.T) {
		pool := openPublishingRestrictionIntegrationPool(t)
		setupPublishingPullObjectConcurrencySchema(t, pool)
		seedExpiredPublishingPullObject(t, pool)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cleanupConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire cleanup connection: %v", err)
		}
		defer cleanupConn.Release()
		reservationConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire reservation connection: %v", err)
		}
		defer reservationConn.Release()

		cleanupTx, err := cleanupConn.Begin(ctx)
		if err != nil {
			t.Fatalf("begin cleanup transaction: %v", err)
		}
		defer cleanupTx.Rollback(context.Background())
		cleanupQueries := New(cleanupTx)
		candidates, err := cleanupQueries.LockPublishingPullObjectCandidates(ctx, 10)
		if err != nil {
			t.Fatalf("lock publishing object candidates for cleanup: %v", err)
		}
		if len(candidates) != 1 || candidates[0].ObjectKey != publishingPullObjectRaceKey {
			t.Fatalf("locked cleanup candidates = %+v, want only %q", candidates, publishingPullObjectRaceKey)
		}

		reservationPID := publishingPullObjectBackendPID(t, ctx, reservationConn)
		reservationResultCh := make(chan error, 1)
		go func() {
			_, reserveErr := New(reservationConn).ReservePublishingPullObjectUsage(ctx, ReservePublishingPullObjectUsageParams{
				ObjectKey:   publishingPullObjectRaceKey,
				ContentType: "video/mp4",
				SizeBytes:   128,
				WorkspaceID: "workspace-1",
				PostID:      "post-active",
			})
			reservationResultCh <- reserveErr
		}()
		waitForPublishingPullObjectLock(t, ctx, pool, reservationPID)

		hasActiveUsage, err := cleanupQueries.PublishingPullObjectHasActiveUsage(ctx, publishingPullObjectRaceKey)
		if err != nil {
			t.Fatalf("recheck usages while cleanup owns object lock: %v", err)
		}
		if hasActiveUsage {
			t.Fatal("seeded cleanup candidate unexpectedly has an active usage")
		}
		if _, err := cleanupQueries.MarkPublishingPullObjectDeleting(ctx, publishingPullObjectRaceKey); err != nil {
			t.Fatalf("mark publishing object deleting: %v", err)
		}
		if err := cleanupTx.Commit(ctx); err != nil {
			t.Fatalf("commit winning cleanup: %v", err)
		}

		if err := <-reservationResultCh; !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("reservation after cleanup claim error = %v, want pgx.ErrNoRows", err)
		}
		assertPublishingPullObjectState(t, pool, "deleting", 0)
	})

	t.Run("expired pending upload self heals after abandonment write failure", func(t *testing.T) {
		pool := openPublishingRestrictionIntegrationPool(t)
		setupPublishingPullObjectConcurrencySchema(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		queries := New(pool)

		usageID, err := queries.ReservePublishingPullObjectUsage(ctx, ReservePublishingPullObjectUsageParams{
			ObjectKey:   publishingPullObjectRaceKey,
			ContentType: "video/mp4",
			SizeBytes:   128,
			WorkspaceID: "workspace-1",
			PostID:      "post-active",
		})
		if err != nil {
			t.Fatalf("reserve pending upload usage: %v", err)
		}
		rows, err := queries.ClaimPublishingPullObjectsDue(ctx, 10)
		if err != nil {
			t.Fatalf("claim before pending lease expiry: %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("cleanup claimed %d objects before pending lease expiry, want 0", len(rows))
		}

		if _, err := pool.Exec(ctx, `
			UPDATE publishing_pull_object_usages
			SET upload_lease_expires_at = NOW() - INTERVAL '1 second'
			WHERE id = $1
		`, usageID); err != nil {
			t.Fatalf("advance pending upload lease to expired: %v", err)
		}
		rows, err = queries.ClaimPublishingPullObjectsDue(ctx, 10)
		if err != nil {
			t.Fatalf("claim after pending lease expiry: %v", err)
		}
		if len(rows) != 1 || rows[0].ObjectKey != publishingPullObjectRaceKey {
			t.Fatalf("claimed rows after pending lease expiry = %+v, want %q", rows, publishingPullObjectRaceKey)
		}
		assertPublishingPullObjectState(t, pool, "deleting", 0)
	})

	t.Run("post-wide retention sync preserves pending upload lease", func(t *testing.T) {
		pool := openPublishingRestrictionIntegrationPool(t)
		setupPublishingPullObjectConcurrencySchema(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		queries := New(pool)

		usageID, err := queries.ReservePublishingPullObjectUsage(ctx, ReservePublishingPullObjectUsageParams{
			ObjectKey:   publishingPullObjectRaceKey,
			ContentType: "video/mp4",
			SizeBytes:   128,
			WorkspaceID: "workspace-1",
			PostID:      "post-active",
		})
		if err != nil {
			t.Fatalf("reserve pending upload usage: %v", err)
		}
		cleanupAfter := time.Now().Add(time.Hour).UTC()
		if err := queries.UpdatePublishingPullObjectUsagesForPost(ctx, UpdatePublishingPullObjectUsagesForPostParams{
			PostStatus:      "failed",
			CleanupAfterAt:  pgtype.Timestamptz{Time: cleanupAfter, Valid: true},
			RetentionReason: "failed_post",
			PostID:          "post-active",
		}); err != nil {
			t.Fatalf("sync post retention while upload is pending: %v", err)
		}

		var uploadState string
		var leaseValid bool
		if err := pool.QueryRow(ctx, `
			SELECT upload_state, upload_lease_expires_at > NOW()
			FROM publishing_pull_object_usages
			WHERE id = $1
		`, usageID).Scan(&uploadState, &leaseValid); err != nil {
			t.Fatalf("read pending upload state after post sync: %v", err)
		}
		if uploadState != "pending" || !leaseValid {
			t.Fatalf("upload state/lease after post sync = %q/%t, want pending/valid", uploadState, leaseValid)
		}
		if _, err := queries.ActivatePublishingPullObjectUsage(ctx, usageID); err != nil {
			t.Fatalf("activate exact pending usage after post sync: %v", err)
		}

		var postStatus, retentionReason string
		var cleanupPreserved, leaseCleared bool
		if err := pool.QueryRow(ctx, `
			SELECT post_status,
			       retention_reason,
			       cleanup_after_at = $2,
			       upload_state = 'active' AND upload_lease_expires_at IS NULL
			FROM publishing_pull_object_usages
			WHERE id = $1
		`, usageID, cleanupAfter).Scan(&postStatus, &retentionReason, &cleanupPreserved, &leaseCleared); err != nil {
			t.Fatalf("read activated usage after post sync: %v", err)
		}
		if postStatus != "failed" || retentionReason != "failed_post" || !cleanupPreserved || !leaseCleared {
			t.Fatalf("activated usage fields = status %q reason %q cleanup-preserved %t lease-cleared %t", postStatus, retentionReason, cleanupPreserved, leaseCleared)
		}
	})

	t.Run("activation object lock prevents concurrent cleanup claim", func(t *testing.T) {
		pool := openPublishingRestrictionIntegrationPool(t)
		setupPublishingPullObjectConcurrencySchema(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		usageID, err := New(pool).ReservePublishingPullObjectUsage(ctx, ReservePublishingPullObjectUsageParams{
			ObjectKey:   publishingPullObjectRaceKey,
			ContentType: "video/mp4",
			SizeBytes:   128,
			WorkspaceID: "workspace-1",
			PostID:      "post-active",
		})
		if err != nil {
			t.Fatalf("reserve pending upload usage: %v", err)
		}
		activationConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire activation connection: %v", err)
		}
		defer activationConn.Release()
		activationTx, err := activationConn.Begin(ctx)
		if err != nil {
			t.Fatalf("begin activation transaction: %v", err)
		}
		defer activationTx.Rollback(context.Background())
		activationQueries := New(activationTx)
		if _, err := activationQueries.LockPublishingPullObjectForUsageActivation(ctx, usageID); err != nil {
			t.Fatalf("lock publishing object for activation: %v", err)
		}
		if _, err := activationQueries.MarkPublishingPullObjectUsageActive(ctx, usageID); err != nil {
			t.Fatalf("mark usage active while object lock is held: %v", err)
		}

		rows, err := New(pool).ClaimPublishingPullObjectsDue(ctx, 10)
		if err != nil {
			t.Fatalf("claim while activation transaction owns object lock: %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("cleanup claimed %d objects while activation owned the object lock, want 0", len(rows))
		}
		if err := activationTx.Commit(ctx); err != nil {
			t.Fatalf("commit activation transaction: %v", err)
		}
		assertPublishingPullObjectState(t, pool, "active", 1)
	})

	t.Run("cleanup object lock makes later activation fail closed", func(t *testing.T) {
		pool := openPublishingRestrictionIntegrationPool(t)
		setupPublishingPullObjectConcurrencySchema(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		queries := New(pool)

		usageID, err := queries.ReservePublishingPullObjectUsage(ctx, ReservePublishingPullObjectUsageParams{
			ObjectKey:   publishingPullObjectRaceKey,
			ContentType: "video/mp4",
			SizeBytes:   128,
			WorkspaceID: "workspace-1",
			PostID:      "post-active",
		})
		if err != nil {
			t.Fatalf("reserve pending upload usage: %v", err)
		}
		cleanupConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire cleanup connection: %v", err)
		}
		defer cleanupConn.Release()
		cleanupTx, err := cleanupConn.Begin(ctx)
		if err != nil {
			t.Fatalf("begin cleanup transaction: %v", err)
		}
		defer cleanupTx.Rollback(context.Background())
		if _, err := cleanupTx.Exec(ctx, `
			SELECT object_key
			FROM publishing_pull_objects
			WHERE object_key = $1
			FOR UPDATE
		`, publishingPullObjectRaceKey); err != nil {
			t.Fatalf("lock publishing object for cleanup: %v", err)
		}

		activationConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("acquire later activation connection: %v", err)
		}
		defer activationConn.Release()
		activationPID := publishingPullObjectBackendPID(t, ctx, activationConn)
		activationResultCh := make(chan error, 1)
		go func() {
			_, activateErr := New(activationConn).ActivatePublishingPullObjectUsage(ctx, usageID)
			activationResultCh <- activateErr
		}()
		waitForPublishingPullObjectLock(t, ctx, pool, activationPID)

		if _, err := cleanupTx.Exec(ctx, `
			UPDATE publishing_pull_objects
			SET cleanup_state = 'deleting', updated_at = NOW()
			WHERE object_key = $1
		`, publishingPullObjectRaceKey); err != nil {
			t.Fatalf("mark publishing object deleting while cleanup owns lock: %v", err)
		}
		if err := cleanupTx.Commit(ctx); err != nil {
			t.Fatalf("commit winning cleanup: %v", err)
		}
		if err := <-activationResultCh; !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("activation after cleanup claim error = %v, want pgx.ErrNoRows", err)
		}
		assertPublishingPullObjectState(t, pool, "deleting", 1)
	})
}

func setupPublishingPullObjectConcurrencySchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		CREATE TABLE workspaces (id TEXT PRIMARY KEY);
		CREATE TABLE social_posts (id TEXT PRIMARY KEY);
		INSERT INTO workspaces (id) VALUES ('workspace-1');
		INSERT INTO social_posts (id) VALUES ('post-expired'), ('post-active');
	`); err != nil {
		t.Fatalf("create publishing pull object integration prerequisites: %v", err)
	}
	applyMigrationUpForIntegration(t, pool, "136_publishing_pull_object_lifecycle.sql")
}

func seedExpiredPublishingPullObject(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO publishing_pull_objects (
			object_key, content_type, size_bytes, cleanup_state
		) VALUES ($1, 'video/mp4', 128, 'active')
	`, publishingPullObjectRaceKey); err != nil {
		t.Fatalf("seed expired publishing pull object row: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO publishing_pull_object_usages (
			object_key, workspace_id, post_id, post_status,
			cleanup_after_at, retention_reason
		) VALUES ($1, 'workspace-1', 'post-expired', 'failed', NOW() - INTERVAL '1 hour', 'upload_failed')
	`, publishingPullObjectRaceKey); err != nil {
		t.Fatalf("seed expired publishing pull object usage: %v", err)
	}
}

func publishingPullObjectBackendPID(t *testing.T, ctx context.Context, conn *pgxpool.Conn) int32 {
	t.Helper()
	var pid int32
	if err := conn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatalf("read PostgreSQL backend PID: %v", err)
	}
	return pid
}

func waitForPublishingPullObjectLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, pid int32) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waitEventType string
		if err := pool.QueryRow(ctx, `
			SELECT COALESCE(wait_event_type, '')
			FROM pg_stat_activity
			WHERE pid = $1
		`, pid).Scan(&waitEventType); err != nil {
			t.Fatalf("read PostgreSQL wait state for PID %d: %v", pid, err)
		}
		if waitEventType == "Lock" {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("backend PID %d did not wait for the publishing object lock: %v", pid, ctx.Err())
		case <-ticker.C:
		}
	}
}

func assertPublishingPullObjectState(t *testing.T, pool *pgxpool.Pool, wantState string, wantActiveUsages int) {
	t.Helper()
	ctx := context.Background()
	var state string
	if err := pool.QueryRow(ctx, `
		SELECT cleanup_state
		FROM publishing_pull_objects
		WHERE object_key = $1
	`, publishingPullObjectRaceKey).Scan(&state); err != nil {
		t.Fatalf("read publishing object state: %v", err)
	}
	if state != wantState {
		t.Fatalf("publishing object state = %q, want %q", state, wantState)
	}
	var activeUsages int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM publishing_pull_object_usages
		WHERE object_key = $1
		  AND (
		    (upload_state = 'pending' AND upload_lease_expires_at > NOW())
		    OR (
		      upload_state = 'active'
		      AND (cleanup_after_at IS NULL OR cleanup_after_at > NOW())
		    )
		  )
	`, publishingPullObjectRaceKey).Scan(&activeUsages); err != nil {
		t.Fatalf("count active publishing object usages: %v", err)
	}
	if activeUsages != wantActiveUsages {
		t.Fatalf("active publishing object usages = %d, want %d", activeUsages, wantActiveUsages)
	}
}
