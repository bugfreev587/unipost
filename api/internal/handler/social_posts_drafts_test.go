package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/paidquota"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestRollbackDraftAndWriteFreePlanQuotaErrorReportsRollbackFailure(t *testing.T) {
	h := &SocialPostHandler{
		queries: db.New(&draftRollbackDB{execErr: errors.New("db unavailable")}),
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post_123/publish", nil)
	rr := httptest.NewRecorder()

	h.rollbackDraftAndWriteFreePlanQuotaError(rr, req, "post_123", quota.QuotaStatus{Usage: 100, Limit: 100}, 1)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rr.Code)
	}
	if strings.Contains(rr.Body.String(), "PLAN_POST_QUOTA_EXCEEDED") {
		t.Fatalf("should not return quota error when rollback failed: %s", rr.Body.String())
	}
}

func TestRollbackDraftAndWriteFreePlanQuotaErrorReturnsQuotaAfterRollback(t *testing.T) {
	h := &SocialPostHandler{
		queries: db.New(&draftRollbackDB{}),
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post_123/publish", nil)
	rr := httptest.NewRecorder()

	h.rollbackDraftAndWriteFreePlanQuotaError(rr, req, "post_123", quota.QuotaStatus{Usage: 100, Limit: 100}, 1)

	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402", rr.Code)
	}
	if got := rr.Header().Get("X-UniPost-Warning"); got != "over_limit" {
		t.Fatalf("X-UniPost-Warning = %q, want over_limit", got)
	}
	if !strings.Contains(rr.Body.String(), "PLAN_POST_QUOTA_EXCEEDED") {
		t.Fatalf("expected quota error body, got: %s", rr.Body.String())
	}
}

func TestFreePlanQuotaExceededMessageMentionsScheduledReservations(t *testing.T) {
	msg := freePlanQuotaExceededMessage(quota.QuotaStatus{
		Usage:    98,
		Reserved: 2,
		Limit:    100,
	}, 1)

	if !strings.Contains(msg, "98 of 100") {
		t.Fatalf("message should include published usage, got: %s", msg)
	}
	if !strings.Contains(msg, "2 scheduled posts") {
		t.Fatalf("message should explain scheduled reservations, got: %s", msg)
	}
	if !strings.Contains(msg, "needs 1 more") {
		t.Fatalf("message should include requested units, got: %s", msg)
	}
}

func TestPaidScheduleEditDeltasMovesCommittedUnitsAcrossMonths(t *testing.T) {
	oldTime := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	oldPosts := []platform.PlatformPostInput{
		{AccountID: "acct_1", Caption: "one"},
		{AccountID: "acct_2", Caption: "two"},
	}
	metadata, err := platform.EncodePostMetadata(oldPosts)
	if err != nil {
		t.Fatal(err)
	}
	existing := db.SocialPost{
		Status:      "scheduled",
		ScheduledAt: pgtype.Timestamptz{Time: oldTime, Valid: true},
		Metadata:    metadata,
	}
	accounts := map[string]platform.ValidateAccount{
		"acct_1": {Platform: "linkedin"},
		"acct_2": {Platform: "tiktok"},
	}

	deltas, err := paidScheduleEditDeltas(existing, oldPosts, newTime, accounts)
	if err != nil {
		t.Fatalf("deltas: %v", err)
	}
	want := []paidquota.PeriodDelta{
		{Period: "2026-07", ReleasedUnits: 2},
		{Period: "2026-08", RequestedUnits: 2},
	}
	if len(deltas) != len(want) {
		t.Fatalf("deltas = %#v, want %#v", deltas, want)
	}
	for i := range want {
		if deltas[i] != want[i] {
			t.Fatalf("delta[%d] = %#v, want %#v", i, deltas[i], want[i])
		}
	}
}

func TestPaidScheduleEditDeltasUsesNetDestinationChangeWithinMonth(t *testing.T) {
	scheduledAt := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	oldPosts := []platform.PlatformPostInput{{AccountID: "acct_1"}, {AccountID: "acct_2"}}
	newPosts := []platform.PlatformPostInput{{AccountID: "acct_1"}}
	metadata, err := platform.EncodePostMetadata(oldPosts)
	if err != nil {
		t.Fatal(err)
	}
	existing := db.SocialPost{
		Status:      "scheduled",
		ScheduledAt: pgtype.Timestamptz{Time: scheduledAt, Valid: true},
		Metadata:    metadata,
	}
	accounts := map[string]platform.ValidateAccount{
		"acct_1": {Platform: "linkedin"},
		"acct_2": {Platform: "tiktok"},
	}

	deltas, err := paidScheduleEditDeltas(existing, newPosts, scheduledAt, accounts)
	if err != nil {
		t.Fatalf("deltas: %v", err)
	}
	want := paidquota.PeriodDelta{Period: "2026-07", ReleasedUnits: 2, RequestedUnits: 1}
	if len(deltas) != 1 || deltas[0] != want {
		t.Fatalf("deltas = %#v, want %#v", deltas, []paidquota.PeriodDelta{want})
	}
}

func TestReschedulePostPlansAndMutatesInsidePaidCoordinator(t *testing.T) {
	oldTime := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	posts := []platform.PlatformPostInput{
		{AccountID: "acct_1", Caption: "one"},
		{AccountID: "acct_2", Caption: "two"},
	}
	metadata, err := platform.EncodePostMetadata(posts)
	if err != nil {
		t.Fatal(err)
	}
	existing := db.SocialPost{
		ID:          "post_1",
		WorkspaceID: "ws_1",
		Status:      "scheduled",
		ScheduledAt: pgtype.Timestamptz{Time: oldTime, Valid: true},
		CreatedAt:   pgtype.Timestamptz{Time: oldTime.Add(-time.Hour), Valid: true},
		Metadata:    metadata,
		Source:      "api",
	}
	dbtx := &scheduledEditTestDB{post: existing}
	coordinator := &recordingPaidScheduleCoordinator{queries: db.New(dbtx)}
	h := &SocialPostHandler{
		queries:      db.New(dbtx),
		paidSchedule: coordinator,
	}
	req := httptest.NewRequest(
		http.MethodPatch,
		"/v1/posts/post_1",
		strings.NewReader(fmt.Sprintf(`{"scheduled_at":%q}`, newTime.Format(time.RFC3339))),
	)
	rr := httptest.NewRecorder()

	h.reschedulePost(rr, req, "ws_1", "post_1", []byte(fmt.Sprintf(`{"scheduled_at":%q}`, newTime.Format(time.RFC3339))))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	if coordinator.calls != 1 {
		t.Fatalf("coordinator calls = %d, want 1", coordinator.calls)
	}
	if len(coordinator.periods) != 2 || coordinator.periods[0] != "2026-07" || coordinator.periods[1] != "2026-08" {
		t.Fatalf("locked periods = %#v", coordinator.periods)
	}
	wantDeltas := []paidquota.PeriodDelta{
		{Period: "2026-07", ReleasedUnits: 2},
		{Period: "2026-08", RequestedUnits: 2},
	}
	if len(coordinator.deltas) != len(wantDeltas) {
		t.Fatalf("deltas = %#v", coordinator.deltas)
	}
	for i := range wantDeltas {
		if coordinator.deltas[i] != wantDeltas[i] {
			t.Fatalf("delta[%d] = %#v, want %#v", i, coordinator.deltas[i], wantDeltas[i])
		}
	}
	if dbtx.rescheduleCalls != 1 || !dbtx.post.ScheduledAt.Time.Equal(newTime) {
		t.Fatalf("reschedule calls=%d scheduled_at=%s", dbtx.rescheduleCalls, dbtx.post.ScheduledAt.Time)
	}
}

func TestUpdateScheduledContentPlansDestinationDeltaInsidePaidCoordinator(t *testing.T) {
	scheduledAt := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	oldPosts := []platform.PlatformPostInput{{AccountID: "acct_1", Caption: "one"}}
	newPosts := []platform.PlatformPostInput{
		{AccountID: "acct_1", Caption: "one"},
		{AccountID: "acct_2", Caption: "two"},
	}
	oldMetadata, err := platform.EncodePostMetadata(oldPosts)
	if err != nil {
		t.Fatal(err)
	}
	newMetadata, err := platform.EncodePostMetadata(newPosts)
	if err != nil {
		t.Fatal(err)
	}
	existing := db.SocialPost{
		ID:          "post_1",
		WorkspaceID: "ws_1",
		Status:      "scheduled",
		ScheduledAt: pgtype.Timestamptz{Time: scheduledAt, Valid: true},
		CreatedAt:   pgtype.Timestamptz{Time: scheduledAt.Add(-time.Hour), Valid: true},
		Metadata:    oldMetadata,
		Source:      "api",
	}
	dbtx := &scheduledEditTestDB{post: existing}
	coordinator := &recordingPaidScheduleCoordinator{queries: db.New(dbtx)}
	h := &SocialPostHandler{
		queries:      db.New(dbtx),
		paidSchedule: coordinator,
	}
	params := buildSocialPostContentUpdateParams(
		"post_1",
		"ws_1",
		newPosts,
		newMetadata,
		&scheduledAt,
		[]string{"prof_1"},
	)

	updated, err := h.updateEditablePostContent(
		context.Background(),
		"ws_1",
		existing,
		newPosts,
		scheduledAt,
		params,
	)
	if err != nil {
		t.Fatalf("update content: %v", err)
	}
	if updated.ID != "post_1" || dbtx.updateContentCalls != 1 {
		t.Fatalf("updated=%#v update calls=%d", updated, dbtx.updateContentCalls)
	}
	want := paidquota.PeriodDelta{
		Period:         "2026-07",
		ReleasedUnits:  1,
		RequestedUnits: 2,
	}
	if len(coordinator.deltas) != 1 || coordinator.deltas[0] != want {
		t.Fatalf("deltas = %#v, want %#v", coordinator.deltas, []paidquota.PeriodDelta{want})
	}
}

func TestSocialPostResponseExposesQuotaHoldMetadata(t *testing.T) {
	heldAt := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	originalScheduledAt := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	post := db.SocialPost{
		ID:                           "post_held",
		Status:                       "quota_hold",
		CreatedAt:                    pgtype.Timestamptz{Time: heldAt.Add(-time.Hour), Valid: true},
		ScheduledAt:                  pgtype.Timestamptz{Time: originalScheduledAt, Valid: true},
		QuotaHoldReason:              pgtype.Text{String: "plan_downgrade", Valid: true},
		QuotaHoldAt:                  pgtype.Timestamptz{Time: heldAt, Valid: true},
		QuotaHoldOriginalScheduledAt: pgtype.Timestamptz{Time: originalScheduledAt, Valid: true},
	}

	response := socialPostResponseFromRow(post)
	if response.QuotaHoldReason == nil || *response.QuotaHoldReason != "plan_downgrade" {
		t.Fatalf("quota hold reason = %#v", response.QuotaHoldReason)
	}
	if response.QuotaHoldAt == nil || !response.QuotaHoldAt.Equal(heldAt) {
		t.Fatalf("quota hold at = %#v", response.QuotaHoldAt)
	}
	if response.QuotaHoldOriginalScheduledAt == nil || !response.QuotaHoldOriginalScheduledAt.Equal(originalScheduledAt) {
		t.Fatalf("original scheduled at = %#v", response.QuotaHoldOriginalScheduledAt)
	}
}

func TestRollbackStatusForClaimedQuotaHold(t *testing.T) {
	if got := rollbackStatusForClaimedPost(db.SocialPost{
		Status:          "publishing",
		QuotaHoldReason: pgtype.Text{String: "plan_downgrade", Valid: true},
	}); got != "quota_hold" {
		t.Fatalf("rollback status = %q, want quota_hold", got)
	}
	if got := rollbackStatusForClaimedPost(db.SocialPost{Status: "publishing"}); got != "draft" {
		t.Fatalf("rollback status = %q, want draft", got)
	}
}

func TestMaybeReconcileQuotaHoldsInvokesCapacityRelease(t *testing.T) {
	reconciler := &recordingHoldReconciler{}
	h := (&SocialPostHandler{}).SetHoldReconciler(reconciler)

	h.maybeReconcileQuotaHolds(context.Background(), "ws_123", "capacity_released")

	if reconciler.calls != 1 || reconciler.workspaceID != "ws_123" || reconciler.reason != "capacity_released" {
		t.Fatalf("reconciler = %#v", reconciler)
	}
	if !reconciler.effectiveAt.IsZero() {
		t.Fatalf("effective at = %s, want zero for capacity release", reconciler.effectiveAt)
	}
}

type draftRollbackDB struct {
	execErr error
}

type scheduledEditTestDB struct {
	post               db.SocialPost
	rescheduleCalls    int
	updateContentCalls int
}

func (f *scheduledEditTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected exec")
}

func (f *scheduledEditTestDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	if strings.Contains(query, "-- name: ListSocialAccountsByWorkspace") {
		return &scheduledQuotaRows{values: [][]any{
			socialAccountValues(db.SocialAccount{
				ID:                "acct_1",
				ProfileID:         "prof_1",
				Platform:          "linkedin",
				AccessToken:       "token",
				ExternalAccountID: "linkedin-page",
				ConnectedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
				Status:            "connected",
			}),
			socialAccountValues(db.SocialAccount{
				ID:                "acct_2",
				ProfileID:         "prof_1",
				Platform:          "tiktok",
				AccessToken:       "token",
				ExternalAccountID: "tiktok-page",
				ConnectedAt:       pgtype.Timestamptz{Time: time.Now(), Valid: true},
				Status:            "connected",
			}),
		}}, nil
	}
	return nil, errors.New("unexpected query")
}

func (f *scheduledEditTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSocialPostByIDAndWorkspace"):
		return scheduledIdempotencySocialPostRow(f.post)
	case strings.Contains(query, "-- name: RescheduleSocialPost"):
		f.rescheduleCalls++
		f.post.ScheduledAt = args[2].(pgtype.Timestamptz)
		return scheduledIdempotencySocialPostRow(f.post)
	case strings.Contains(query, "-- name: UpdateDraftContent"):
		f.updateContentCalls++
		f.post.Caption = args[2].(pgtype.Text)
		f.post.MediaUrls = args[3].([]string)
		f.post.Metadata = args[4].([]byte)
		f.post.ScheduledAt = args[5].(pgtype.Timestamptz)
		f.post.ProfileIds = args[6].([]string)
		return scheduledIdempotencySocialPostRow(f.post)
	default:
		return scanRow{err: errors.New("unexpected query row")}
	}
}

func (d *draftRollbackDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, d.execErr
}

func (d *draftRollbackDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}

func (d *draftRollbackDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return draftRollbackRow{}
}

type draftRollbackRow struct{}

func (draftRollbackRow) Scan(...any) error {
	return errors.New("unexpected query row")
}
