package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
	cancelPost db.SocialPost
	planID     string
	upserts    []db.UpsertMediaPostUsageParams
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
	case strings.Contains(query, "-- name: UpsertMediaPostUsage"):
		f.upserts = append(f.upserts, db.UpsertMediaPostUsageParams{
			MediaID:        args[0].(string),
			WorkspaceID:    args[1].(string),
			PostStatus:     args[2].(string),
			CleanupAfterAt: args[3].(pgtype.Timestamptz),
			PostID:         args[4].(string),
		})
		return scheduledIdempotencyRow{values: []any{true}}
	default:
		return scheduledIdempotencyRow{err: errors.New("unexpected QueryRow: " + query)}
	}
}
