package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
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
	"github.com/xiaoboyu/unipost-api/internal/storage"
)

func TestMediaIDsForRetentionFromPostMetadataDedupesAcrossPlatformPosts(t *testing.T) {
	meta, err := platform.EncodePostMetadata([]platform.PlatformPostInput{
		{AccountID: "sa_1", Caption: "one", MediaIDs: []string{"med_c", "med_b", "med_c"}},
		{AccountID: "sa_2", Caption: "two", MediaIDs: []string{"med_b", "med_a"}},
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

func TestSyncPostMediaRetentionAppliesSamePolicyToPublishingPullObjects(t *testing.T) {
	post := mediaRetentionPost(t, "failed")
	dbtx := &mediaRetentionTestDB{}
	handler := &SocialPostHandler{queries: db.New(dbtx), storage: &storage.Client{}}

	before := time.Now().Add(47 * time.Hour)
	handler.syncPostMediaRetention(context.Background(), post, post.Status)
	after := time.Now().Add(49 * time.Hour)

	if len(dbtx.publishingUsageUpdates) != 1 {
		t.Fatalf("publishing pull usage updates = %d, want 1", len(dbtx.publishingUsageUpdates))
	}
	update := dbtx.publishingUsageUpdates[0]
	if update.PostID != post.ID || update.PostStatus != "failed" || update.RetentionReason != "plan_status" {
		t.Fatalf("publishing pull usage update = %#v", update)
	}
	if !update.CleanupAfterAt.Valid || update.CleanupAfterAt.Time.Before(before) || update.CleanupAfterAt.Time.After(after) {
		t.Fatalf("cleanup_after_at = %#v, want about 48h from now", update.CleanupAfterAt)
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

func TestStrictPublishingRestrictionRetentionUsesFailureTimePlusSixtyDays(t *testing.T) {
	post := mediaRetentionPost(t, "failed")
	dbtx := &mediaRetentionTestDB{planID: "free"}
	handler := &SocialPostHandler{queries: db.New(dbtx), quota: quota.NewChecker(db.New(dbtx))}
	failedAt := time.Date(2026, 7, 26, 18, 30, 0, 0, time.UTC)

	err := handler.syncPostMediaRetentionForPublishingRestrictionStatusAtStrict(
		context.Background(), post, post.Status, failedAt,
	)
	if err != nil {
		t.Fatalf("strict retention: %v", err)
	}
	if len(dbtx.upserts) != 2 {
		t.Fatalf("upserts=%d, want 2", len(dbtx.upserts))
	}
	for _, upsert := range dbtx.upserts {
		want := failedAt.Add(60 * 24 * time.Hour)
		if upsert.RetentionReason != "publishing_restriction" ||
			!upsert.CleanupAfterAt.Valid || !upsert.CleanupAfterAt.Time.Equal(want) {
			t.Fatalf("strict retention upsert=%+v, want publishing_restriction through %v", upsert, want)
		}
	}
}

func TestStrictPublishingRestrictionRetentionSortsUniqueMediaIDs(t *testing.T) {
	meta, err := platform.EncodePostMetadata([]platform.PlatformPostInput{
		{AccountID: "acct_1", Caption: "one", MediaIDs: []string{"media_b", "media_a"}},
		{AccountID: "acct_2", Caption: "two", MediaIDs: []string{"media_b"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	post := db.SocialPost{ID: "post_sorted", WorkspaceID: "ws_1", Status: "failed", Metadata: meta}
	dbtx := &mediaRetentionTestDB{}
	handler := &SocialPostHandler{queries: db.New(dbtx)}

	if err := handler.syncPostMediaRetentionForPublishingRestrictionStatusAtStrict(
		context.Background(), post, post.Status, time.Now(),
	); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(dbtx.upserts))
	for _, upsert := range dbtx.upserts {
		got = append(got, upsert.MediaID)
	}
	want := []string{"media_a", "media_b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("strict retention media order = %v, want %v", got, want)
	}
}

func TestStrictPublishingRestrictionRetentionFailsAtEveryLedgerBoundary(t *testing.T) {
	databaseUnavailable := errors.New("database unavailable")
	noMediaMetadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{
		AccountID: "acct_1",
		Caption:   "no media",
	}})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		mutate  func(*db.SocialPost, *mediaRetentionTestDB)
		wantErr error
	}{
		{
			name: "metadata decode",
			mutate: func(post *db.SocialPost, _ *mediaRetentionTestDB) {
				post.Metadata = []byte(`{"schema_version":2,"platform_posts":[`)
			},
			wantErr: errInvalidPostMediaMetadata,
		},
		{
			name: "delete stale usages",
			mutate: func(_ *db.SocialPost, dbtx *mediaRetentionTestDB) {
				dbtx.deleteExceptErr = databaseUnavailable
			},
			wantErr: databaseUnavailable,
		},
		{
			name: "delete all usages for no-media post",
			mutate: func(post *db.SocialPost, dbtx *mediaRetentionTestDB) {
				post.Metadata = noMediaMetadata
				dbtx.deleteAllErr = databaseUnavailable
			},
			wantErr: databaseUnavailable,
		},
		{
			name: "upsert database failure",
			mutate: func(_ *db.SocialPost, dbtx *mediaRetentionTestDB) {
				dbtx.upsertErr = databaseUnavailable
			},
			wantErr: databaseUnavailable,
		},
		{
			name: "upsert cleanup race",
			mutate: func(_ *db.SocialPost, dbtx *mediaRetentionTestDB) {
				dbtx.upsertErr = pgx.ErrNoRows
			},
			wantErr: pgx.ErrNoRows,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			post := mediaRetentionPost(t, "failed")
			dbtx := &mediaRetentionTestDB{}
			test.mutate(&post, dbtx)
			handler := &SocialPostHandler{queries: db.New(dbtx)}

			err := handler.syncPostMediaRetentionForPublishingRestrictionStatusAtStrict(
				context.Background(), post, post.Status, time.Now(),
			)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("strict retention error=%v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestOrdinaryMediaRetentionRemainsBestEffortAcrossLedgerFailures(t *testing.T) {
	databaseUnavailable := errors.New("database unavailable")
	dbtx := &mediaRetentionTestDB{
		deleteExceptErr: databaseUnavailable,
		upsertErr:       databaseUnavailable,
	}
	handler := &SocialPostHandler{queries: db.New(dbtx)}

	handler.syncPostMediaRetention(context.Background(), mediaRetentionPost(t, "published"), "published")

	if dbtx.upsertCalls != 2 {
		t.Fatalf("ordinary retention upsert attempts=%d, want both media attempted after best-effort failures", dbtx.upsertCalls)
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
		"LOWER(BTRIM(COALESCE((",
		"SELECT LOWER(BTRIM(plan_id))",
		"SET usage_version = parent.usage_version + 1",
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

func TestMediaLifecycleQueriesUseCanonicalParentLockOrder(t *testing.T) {
	postDeliverySource, err := os.ReadFile("../db/queries/post_delivery_jobs.sql")
	if err != nil {
		t.Fatal(err)
	}
	postDeliveryText := string(postDeliverySource)
	queryBetween := func(startMarker, endMarker string) string {
		t.Helper()
		start := strings.Index(postDeliveryText, startMarker)
		if start < 0 {
			t.Fatalf("query marker %q not found", startMarker)
		}
		end := strings.Index(postDeliveryText[start:], endMarker)
		if end < 0 {
			t.Fatalf("query boundary %q not found", endMarker)
		}
		return postDeliveryText[start : start+end]
	}

	retryQuery := queryBetween(
		"-- name: CreateRetryPostDeliveryJobWithMediaActivation",
		"-- name: GetPostDeliveryJobByIDAndWorkspace",
	)
	finalizeQuery := queryBetween(
		"-- name: FinalizeRestrictedPostDeliveryJob",
		"-- name: CancelPostDeliveryJob",
	)
	for name, query := range map[string]string{
		"retry activation":      retryQuery,
		"restriction finalizer": finalizeQuery,
	} {
		for _, fragment := range []string{
			"ordered_media AS MATERIALIZED",
			"ORDER BY parent.id",
			"FOR UPDATE OF parent",
		} {
			if !strings.Contains(query, fragment) {
				t.Fatalf("%s query missing canonical media lock fragment %q", name, fragment)
			}
		}
	}

	cleanupSource, err := os.ReadFile("../db/queries/media_post_usages.sql")
	if err != nil {
		t.Fatal(err)
	}
	cleanupText := string(cleanupSource)
	cleanupStart := strings.Index(cleanupText, "-- name: ClaimMediaDueForRetentionCleanup")
	if cleanupStart < 0 {
		t.Fatal("cleanup query marker not found")
	}
	cleanupQuery := cleanupText[cleanupStart:]
	eligibleStart := strings.Index(cleanupQuery, "eligible AS")
	if eligibleStart < 0 || !strings.Contains(cleanupQuery[eligibleStart:], "ORDER BY m.id") {
		t.Fatal("cleanup eligible rows must lock media parents in canonical id order")
	}
}

func TestMediaUsageUpsertCleanupWinnerIsExpectedNoop(t *testing.T) {
	if shouldLogMediaPostUsageUpsertError(pgx.ErrNoRows) {
		t.Fatal("cleanup-wins/non-uploaded media must not produce error noise")
	}
	if !shouldLogMediaPostUsageUpsertError(errors.New("database unavailable")) {
		t.Fatal("unexpected database failures must still be logged")
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
	cancelPost               db.SocialPost
	planID                   string
	retentionDeadline        pgtype.Timestamptz
	upserts                  []db.UpsertMediaPostUsageParams
	deleteAllErr             error
	deleteExceptErr          error
	upsertErr                error
	upsertCalls              int
	publishingUsageUpdates   []db.UpdatePublishingPullObjectUsagesForPostParams
	publishingUsageUpdateErr error
}

func (f *mediaRetentionTestDB) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(query, "-- name: UpdatePublishingPullObjectUsagesForPost"):
		f.publishingUsageUpdates = append(f.publishingUsageUpdates, db.UpdatePublishingPullObjectUsagesForPostParams{
			PostStatus:      args[0].(string),
			CleanupAfterAt:  args[1].(pgtype.Timestamptz),
			RetentionReason: args[2].(string),
			PostID:          args[3].(string),
		})
		return pgconn.CommandTag{}, f.publishingUsageUpdateErr
	case strings.Contains(query, "-- name: DeleteMediaPostUsagesForPostExcept"):
		return pgconn.CommandTag{}, f.deleteExceptErr
	case strings.Contains(query, "-- name: DeleteMediaPostUsagesForPost"):
		return pgconn.CommandTag{}, f.deleteAllErr
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
		f.upsertCalls++
		if f.upsertErr != nil {
			return scheduledIdempotencyRow{err: f.upsertErr}
		}
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
