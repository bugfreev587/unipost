package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestMediaIDsForRetentionFromPostMetadataDedupesAcrossPlatformPosts(t *testing.T) {
	meta, err := platform.EncodePostMetadata([]platform.PlatformPostInput{
		{AccountID: "sa_1", Caption: "one", MediaIDs: []string{"med_a", "med_b", "med_a"}},
		{AccountID: "sa_2", Caption: "two", MediaIDs: []string{"med_b", "med_c"}},
	})
	if err != nil {
		t.Fatalf("encode metadata: %v", err)
	}

	got := mediaIDsForRetention(db.SocialPost{
		ID:       "post_1",
		Caption:  pgtype.Text{String: "fallback", Valid: true},
		Metadata: meta,
	})

	want := []string{"med_a", "med_b", "med_c"}
	if len(got) != len(want) {
		t.Fatalf("ids = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %#v, want %#v", got, want)
		}
	}
}

func TestSyncPostMediaRetentionSchedulesCancelledPostMediaForCleanup(t *testing.T) {
	post := mediaRetentionPost(t, "cancelled")
	dbtx := &mediaRetentionTestDB{}
	handler := &SocialPostHandler{queries: db.New(dbtx)}

	before := time.Now().Add(47 * time.Hour)
	handler.syncPostMediaRetention(context.Background(), post, post.Status)
	after := time.Now().Add(49 * time.Hour)

	if len(dbtx.upserts) != 2 {
		t.Fatalf("upserts = %d, want 2", len(dbtx.upserts))
	}
	for _, upsert := range dbtx.upserts {
		if upsert.PostStatus != "cancelled" {
			t.Fatalf("post status = %q, want cancelled", upsert.PostStatus)
		}
		if !upsert.CleanupAfterAt.Valid {
			t.Fatal("cleanup_after_at should be set for cancelled posts")
		}
		if upsert.CleanupAfterAt.Time.Before(before) || upsert.CleanupAfterAt.Time.After(after) {
			t.Fatalf("cleanup_after_at = %s, want about 48h from now", upsert.CleanupAfterAt.Time)
		}
	}
}

func TestCancelSocialPostSyncsCancelledMediaRetention(t *testing.T) {
	post := mediaRetentionPost(t, "cancelled")
	dbtx := &mediaRetentionTestDB{cancelPost: post}
	handler := &SocialPostHandler{queries: db.New(dbtx)}
	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post_1", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), post.WorkspaceID))
	rr := httptest.NewRecorder()

	handler.cancelSocialPost(rr, req, post.WorkspaceID, post.ID)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body.String())
	}
	if len(dbtx.upserts) != 2 {
		t.Fatalf("cancel should sync media retention upserts = %d, want 2", len(dbtx.upserts))
	}
}

func TestSyncPostMediaRetentionUsesTeamTerminalWindows(t *testing.T) {
	tests := []struct {
		status string
		want   time.Duration
	}{
		{status: "published", want: 30 * 24 * time.Hour},
		{status: "failed", want: 60 * 24 * time.Hour},
		{status: "partial", want: 60 * 24 * time.Hour},
		{status: "cancelled", want: 60 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			post := mediaRetentionPost(t, tt.status)
			dbtx := &mediaRetentionTestDB{planID: "team"}
			queries := db.New(dbtx)
			handler := &SocialPostHandler{queries: queries, quota: quota.NewChecker(queries)}
			before := time.Now().Add(tt.want - time.Minute)

			handler.syncPostMediaRetention(context.Background(), post, tt.status)

			after := time.Now().Add(tt.want + time.Minute)
			if len(dbtx.upserts) != 2 {
				t.Fatalf("upserts=%d, want 2", len(dbtx.upserts))
			}
			for _, upsert := range dbtx.upserts {
				if !upsert.CleanupAfterAt.Valid || upsert.CleanupAfterAt.Time.Before(before) || upsert.CleanupAfterAt.Time.After(after) {
					t.Fatalf("cleanup_after_at=%#v, want about %s", upsert.CleanupAfterAt, tt.want)
				}
			}
		})
	}
}

func TestSyncPostMediaRetentionKeepsActiveTeamMedia(t *testing.T) {
	for _, status := range []string{"draft", "scheduled", "queued", "publishing", "processing"} {
		t.Run(status, func(t *testing.T) {
			post := mediaRetentionPost(t, status)
			dbtx := &mediaRetentionTestDB{planID: "team"}
			queries := db.New(dbtx)
			handler := &SocialPostHandler{queries: queries, quota: quota.NewChecker(queries)}

			handler.syncPostMediaRetention(context.Background(), post, status)

			if len(dbtx.upserts) != 2 {
				t.Fatalf("upserts=%d, want 2", len(dbtx.upserts))
			}
			for _, upsert := range dbtx.upserts {
				if upsert.CleanupAfterAt.Valid {
					t.Fatalf("active status %q scheduled cleanup at %s", status, upsert.CleanupAfterAt.Time)
				}
			}
		})
	}
}

func TestSyncPostMediaRetentionTransitionReplacesActiveDeadline(t *testing.T) {
	post := mediaRetentionPost(t, "scheduled")
	dbtx := &mediaRetentionTestDB{planID: "team"}
	queries := db.New(dbtx)
	handler := &SocialPostHandler{queries: queries, quota: quota.NewChecker(queries)}

	handler.syncPostMediaRetention(context.Background(), post, "scheduled")
	handler.syncPostMediaRetention(context.Background(), post, "published")

	if len(dbtx.upserts) != 4 {
		t.Fatalf("upserts=%d, want 4", len(dbtx.upserts))
	}
	for i := 0; i < 2; i++ {
		if dbtx.upserts[i].CleanupAfterAt.Valid {
			t.Fatalf("active upsert %d unexpectedly has cleanup deadline", i)
		}
	}
	for i := 2; i < 4; i++ {
		if !dbtx.upserts[i].CleanupAfterAt.Valid {
			t.Fatalf("terminal upsert %d missing cleanup deadline", i)
		}
	}
}

func TestSyncPostMediaRetentionForPublishingRestrictionUsesFailureTimePlusSixtyDays(t *testing.T) {
	post := mediaRetentionPost(t, "failed")
	dbtx := &mediaRetentionTestDB{planID: "free"}
	handler := &SocialPostHandler{queries: db.New(dbtx), quota: quota.NewChecker(db.New(dbtx))}
	failedAt := time.Date(2026, 7, 26, 18, 30, 0, 0, time.UTC)

	handler.syncPostMediaRetentionForPublishingRestrictionAt(context.Background(), post, failedAt)

	if len(dbtx.upserts) != 2 {
		t.Fatalf("upserts=%d, want 2", len(dbtx.upserts))
	}
	for _, upsert := range dbtx.upserts {
		if upsert.RetentionReason != "publishing_restriction" {
			t.Fatalf("retention_reason=%q", upsert.RetentionReason)
		}
		want := failedAt.Add(60 * 24 * time.Hour)
		if !upsert.CleanupAfterAt.Valid || !upsert.CleanupAfterAt.Time.Equal(want) {
			t.Fatalf("cleanup_after_at=%v, want %v", upsert.CleanupAfterAt, want)
		}
	}
}

func TestResultTransitionPreservesParentWidePublishingRestrictionDeadline(t *testing.T) {
	post := mediaRetentionPost(t, "partial")
	wantDeadline := time.Date(2026, 9, 24, 18, 30, 0, 0, time.UTC)
	dbtx := &mediaRetentionTestDB{
		planID:            "free",
		retentionDeadline: pgtype.Timestamptz{Time: wantDeadline, Valid: true},
	}
	handler := &SocialPostHandler{queries: db.New(dbtx), quota: quota.NewChecker(db.New(dbtx))}
	results := []db.SocialPostResult{
		{Status: "published"},
		{Status: "failed", ErrorCode: pgtype.Text{String: "plan_platform_publishing_restricted", Valid: true}},
	}

	handler.syncPostMediaRetentionAfterResultTransition(context.Background(), post, "partial", results)

	if len(dbtx.upserts) != 2 {
		t.Fatalf("upserts=%d, want 2", len(dbtx.upserts))
	}
	for _, upsert := range dbtx.upserts {
		if upsert.PostStatus != "partial" || upsert.RetentionReason != "publishing_restriction" {
			t.Fatalf("usage=%+v, want partial policy retention", upsert)
		}
		if !upsert.CleanupAfterAt.Valid || !upsert.CleanupAfterAt.Time.Equal(wantDeadline) {
			t.Fatalf("cleanup_after_at=%v, want preserved %v", upsert.CleanupAfterAt, wantDeadline)
		}
	}
}

func TestSyncPostMediaRetentionMarksNonTerminalUsageActive(t *testing.T) {
	post := mediaRetentionPost(t, "publishing")
	dbtx := &mediaRetentionTestDB{}
	handler := &SocialPostHandler{queries: db.New(dbtx)}

	handler.syncPostMediaRetention(context.Background(), post, "publishing")

	for _, upsert := range dbtx.upserts {
		if upsert.RetentionReason != "active_post" || upsert.CleanupAfterAt.Valid {
			t.Fatalf("active usage=%+v", upsert)
		}
	}
}

func TestRetryJobCreationAtomicallyReactivatesMediaUsage(t *testing.T) {
	source, err := os.ReadFile("../db/queries/post_delivery_jobs.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "-- name: CreateRetryPostDeliveryJobWithMediaActivation")
	if start < 0 {
		t.Fatal("atomic retry/media query is missing")
	}
	query := text[start:]
	for _, fragment := range []string{
		"FOR SHARE OF restriction",
		"FOR UPDATE",
		"result.status = 'failed'",
		"state IN ('pending', 'running', 'retrying')",
		"platform_publishing_restrictions",
		"restricted_plan_ids",
		"SET usage_version = usage_version + 1",
		"INSERT INTO media_post_usages",
		"ON CONFLICT (media_id, post_id) DO UPDATE",
		"retention_reason = 'active_post'",
		"INSERT INTO post_delivery_jobs",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("atomic retry/media query missing %q", fragment)
		}
	}
}

func mediaRetentionPost(t *testing.T, status string) db.SocialPost {
	t.Helper()

	meta, err := platform.EncodePostMetadata([]platform.PlatformPostInput{
		{AccountID: "acct_1", Caption: "one", MediaIDs: []string{"media_1", "media_2"}},
	})
	if err != nil {
		t.Fatalf("encode metadata: %v", err)
	}

	return db.SocialPost{
		ID:          "post_1",
		WorkspaceID: "ws_1",
		Caption:     pgtype.Text{String: "one", Valid: true},
		Metadata:    meta,
		Status:      status,
		CreatedAt:   pgtype.Timestamptz{Time: time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC), Valid: true},
		Source:      "api",
		ProfileIds:  []string{"prof_1"},
	}
}

type mediaRetentionTestDB struct {
	cancelPost        db.SocialPost
	planID            string
	retentionDeadline pgtype.Timestamptz
	upserts           []db.UpsertMediaPostUsageParams
}

func (f *mediaRetentionTestDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(query, "-- name: DeleteMediaPostUsagesForPostExcept"):
		return pgconn.CommandTag{}, nil
	case strings.Contains(query, "-- name: DeleteMediaPostUsagesForPost"):
		return pgconn.CommandTag{}, nil
	default:
		return pgconn.CommandTag{}, errors.New("unexpected Exec: " + query)
	}
}

func (f *mediaRetentionTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (f *mediaRetentionTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		return subscriptionScanRow(f.planID)
	case strings.Contains(query, "-- name: CancelSocialPost"):
		return scheduledIdempotencySocialPostRow(f.cancelPost)
	case strings.Contains(query, "-- name: GetPostPublishingRestrictionMediaRetention"):
		return scheduledIdempotencyRow{values: []any{f.retentionDeadline}}
	case strings.Contains(query, "-- name: UpsertMediaPostUsage"):
		f.upserts = append(f.upserts, db.UpsertMediaPostUsageParams{
			MediaID:         args[0].(string),
			WorkspaceID:     args[1].(string),
			PostStatus:      args[2].(string),
			CleanupAfterAt:  args[3].(pgtype.Timestamptz),
			RetentionReason: args[4].(string),
			PostID:          args[5].(string),
		})
		return scheduledIdempotencyRow{values: []any{true}}
	default:
		return scheduledIdempotencyRow{err: errors.New("unexpected QueryRow: " + query)}
	}
}
