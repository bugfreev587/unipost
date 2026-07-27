//go:build integration

package db

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestPublishingRestrictionRetentionDeadlineSurvivesMediaHardDelete(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE TABLE social_posts (
			id TEXT PRIMARY KEY,
			metadata JSONB NOT NULL
		);
		CREATE TABLE media (
			id TEXT PRIMARY KEY
		);
		CREATE TABLE media_post_usages (
			id TEXT PRIMARY KEY,
			media_id TEXT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
			post_id TEXT NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
			cleanup_after_at TIMESTAMPTZ,
			retention_reason TEXT NOT NULL
		);
		CREATE TABLE post_failures (
			id TEXT PRIMARY KEY,
			post_id TEXT NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
			error_code TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	failedAt := time.Date(2026, 7, 27, 10, 30, 0, 0, time.UTC)
	wantDeadline := failedAt.Add(60 * 24 * time.Hour)
	_, err = pool.Exec(ctx, `
		INSERT INTO social_posts (id, metadata) VALUES (
			'post_retained',
			'{"platform_posts":[{"media_ids":["media_retained"]}]}'::JSONB
		);
		INSERT INTO media (id) VALUES ('media_retained');
	`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO media_post_usages (
			id, media_id, post_id, cleanup_after_at, retention_reason
		) VALUES (
			'usage_retained', 'media_retained', 'post_retained', $1, 'publishing_restriction'
		);
	`, wantDeadline)
	if err != nil {
		t.Fatal(err)
	}
	queries := New(pool)
	before, err := queries.GetPostPublishingRestrictionMediaRetention(ctx, "post_retained")
	if err != nil {
		t.Fatal(err)
	}
	assertRestrictionRetentionDeadline(t, before, wantDeadline)

	if err := queries.HardDeleteMedia(ctx, "media_retained"); err != nil {
		t.Fatal(err)
	}
	var usageCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM media_post_usages`).Scan(&usageCount); err != nil {
		t.Fatal(err)
	}
	if usageCount != 0 {
		t.Fatalf("media usage rows after hard delete = %d, want 0", usageCount)
	}

	after, err := queries.GetPostPublishingRestrictionMediaRetention(ctx, "post_retained")
	if err != nil {
		t.Fatal(err)
	}
	assertRestrictionRetentionDeadline(t, after, wantDeadline)

	// A previously failed R2 deletion can complete after a newer retention
	// cycle was already preserved. The late cleanup must not regress history.
	olderDeadline := wantDeadline.Add(-24 * time.Hour)
	_, err = pool.Exec(ctx, `
		WITH inserted_media AS (
			INSERT INTO media (id) VALUES ('media_older')
			RETURNING id
		)
		INSERT INTO media_post_usages (
			id, media_id, post_id, cleanup_after_at, retention_reason
		)
		SELECT 'usage_older', id, 'post_retained', $1, 'publishing_restriction'
		FROM inserted_media;
	`, olderDeadline)
	if err != nil {
		t.Fatal(err)
	}
	if err := queries.HardDeleteMedia(ctx, "media_older"); err != nil {
		t.Fatal(err)
	}
	afterLateCleanup, err := queries.GetPostPublishingRestrictionMediaRetention(ctx, "post_retained")
	if err != nil {
		t.Fatal(err)
	}
	assertRestrictionRetentionDeadline(t, afterLateCleanup, wantDeadline)
}

func TestPublishingRestrictionRetentionDeadlineNotProjectedForTextOnlyPost(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE TABLE social_posts (
			id TEXT PRIMARY KEY,
			metadata JSONB NOT NULL
		);
		CREATE TABLE media_post_usages (
			post_id TEXT NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
			cleanup_after_at TIMESTAMPTZ,
			retention_reason TEXT NOT NULL
		);
		CREATE TABLE post_failures (
			id TEXT PRIMARY KEY,
			post_id TEXT NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
			error_code TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		);
		INSERT INTO social_posts (id, metadata) VALUES (
			'post_text',
			'{"platform_posts":[{"media_ids":[]}]}'::JSONB
		);
		INSERT INTO post_failures (id, post_id, error_code, created_at) VALUES (
			'failure_text', 'post_text', 'plan_platform_publishing_restricted', NOW()
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	retainedUntil, err := New(pool).GetPostPublishingRestrictionMediaRetention(ctx, "post_text")
	if err != nil {
		t.Fatal(err)
	}
	if retainedUntil.Valid {
		t.Fatalf("text-only post retention deadline = %s, want NULL", retainedUntil.Time)
	}
}

func TestPublishingRestrictionRetentionRepairsUnsafeMetadataBeforeHardDelete(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE TABLE social_posts (
			id TEXT PRIMARY KEY,
			metadata JSONB
		);
		CREATE TABLE media (
			id TEXT PRIMARY KEY
		);
		CREATE TABLE media_post_usages (
			id TEXT PRIMARY KEY,
			media_id TEXT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
			post_id TEXT NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
			cleanup_after_at TIMESTAMPTZ,
			retention_reason TEXT NOT NULL
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Date(2026, 9, 25, 10, 30, 0, 0, time.UTC)
	tests := []struct {
		name             string
		metadata         any
		wantUnrelatedKey bool
	}{
		{name: "SQL NULL", metadata: nil},
		{name: "top-level array", metadata: `["legacy"]`},
		{name: "top-level boolean", metadata: `true`},
		{name: "deadline object", metadata: `{"keep":"preserved","publishing_restriction_media_retained_until":{"bad":true}}`, wantUnrelatedKey: true},
		{name: "deadline array", metadata: `{"keep":"preserved","publishing_restriction_media_retained_until":[]}`, wantUnrelatedKey: true},
		{name: "deadline boolean", metadata: `{"keep":"preserved","publishing_restriction_media_retained_until":false}`, wantUnrelatedKey: true},
		{name: "deadline invalid string", metadata: `{"keep":"preserved","publishing_restriction_media_retained_until":"not-a-timestamp"}`, wantUnrelatedKey: true},
		{name: "deadline JSON null", metadata: `{"keep":"preserved","publishing_restriction_media_retained_until":null}`, wantUnrelatedKey: true},
		{name: "deadline parseable JSON number", metadata: `{"keep":"preserved","publishing_restriction_media_retained_until":20990101}`, wantUnrelatedKey: true},
		{name: "deadline positive infinity", metadata: `{"keep":"preserved","publishing_restriction_media_retained_until":"infinity"}`, wantUnrelatedKey: true},
		{name: "deadline negative infinity", metadata: `{"keep":"preserved","publishing_restriction_media_retained_until":"-infinity"}`, wantUnrelatedKey: true},
	}
	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			postID := fmt.Sprintf("post_unsafe_%d", i)
			mediaID := fmt.Sprintf("media_unsafe_%d", i)
			usageID := fmt.Sprintf("usage_unsafe_%d", i)
			if _, err := pool.Exec(ctx, `
				INSERT INTO social_posts (id, metadata) VALUES ($1, $2::jsonb)
			`, postID, test.metadata); err != nil {
				t.Fatal(err)
			}
			queries := New(pool)
			historicalOnly, err := queries.GetPostPublishingRestrictionMediaRetention(ctx, postID)
			if err != nil {
				t.Fatalf("get unsafe historical retention deadline: %v", err)
			}
			if historicalOnly.Valid {
				t.Fatalf("unsafe historical retention deadline = %s, want NULL", historicalOnly.Time)
			}
			if _, err := pool.Exec(ctx, `
				WITH inserted_media AS (
					INSERT INTO media (id) VALUES ($1)
					RETURNING id
				)
				INSERT INTO media_post_usages (
					id, media_id, post_id, cleanup_after_at, retention_reason
				)
				SELECT $2, id, $3, $4, 'publishing_restriction'
				FROM inserted_media
			`, mediaID, usageID, postID, deadline); err != nil {
				t.Fatal(err)
			}

			before, err := queries.GetPostPublishingRestrictionMediaRetention(ctx, postID)
			if err != nil {
				t.Fatalf("get current retention deadline: %v", err)
			}
			assertRestrictionRetentionDeadline(t, before, deadline)

			if err := queries.HardDeleteMedia(ctx, mediaID); err != nil {
				t.Fatalf("hard delete media with unsafe metadata: %v", err)
			}
			var mediaCount int
			if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM media WHERE id=$1`, mediaID).Scan(&mediaCount); err != nil {
				t.Fatal(err)
			}
			if mediaCount != 0 {
				t.Fatalf("media rows after hard delete = %d, want 0", mediaCount)
			}
			after, err := queries.GetPostPublishingRestrictionMediaRetention(ctx, postID)
			if err != nil {
				t.Fatalf("get preserved retention deadline: %v", err)
			}
			assertRestrictionRetentionDeadline(t, after, deadline)

			var rawMetadata []byte
			if err := pool.QueryRow(ctx, `SELECT metadata FROM social_posts WHERE id=$1`, postID).Scan(&rawMetadata); err != nil {
				t.Fatal(err)
			}
			var metadata map[string]any
			if err := json.Unmarshal(rawMetadata, &metadata); err != nil {
				t.Fatalf("metadata is not an object after repair: %s: %v", rawMetadata, err)
			}
			if test.wantUnrelatedKey && metadata["keep"] != "preserved" {
				t.Fatalf("unrelated metadata key = %#v, want preserved", metadata["keep"])
			}
		})
	}
}

func assertRestrictionRetentionDeadline(t *testing.T, got pgtype.Timestamptz, want time.Time) {
	t.Helper()
	if !got.Valid {
		t.Fatal("retention deadline is NULL")
	}
	if !got.Time.Equal(want) {
		t.Fatalf("retention deadline = %s, want %s", got.Time, want)
	}
}
