package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestAdminObjectStoragePeriodWindows(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 34, 56, 0, time.UTC)
	for _, tt := range []struct {
		key      string
		wantFrom time.Time
		wantTo   time.Time
	}{
		{"yesterday", time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)},
		{"last_7_days", now.AddDate(0, 0, -7), now},
		{"last_month", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)},
		{"this_week", time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC), now},
		{"this_month", time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), now},
		{"this_year", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), now},
	} {
		t.Run(tt.key, func(t *testing.T) {
			got, err := parseAdminObjectStoragePeriod(tt.key, now)
			if err != nil {
				t.Fatalf("parse period: %v", err)
			}
			if got.From != tt.wantFrom || got.To != tt.wantTo {
				t.Fatalf("period %s = [%s, %s), want [%s, %s)", tt.key, got.From, got.To, tt.wantFrom, tt.wantTo)
			}
		})
	}
}

func TestAdminObjectStorageRejectsInvalidPeriod(t *testing.T) {
	h := NewAdminObjectStorageHandler(&fakeAdminObjectStorageStore{}, "unipost-media")
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/object-storage?period=last_30_days", nil)
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rr.Code, rr.Body.String())
	}
	var got ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if got.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("error code = %q, want VALIDATION_ERROR", got.Error.Code)
	}
}

func TestAdminObjectStorageReturnsSummary(t *testing.T) {
	started := time.Date(2026, 7, 7, 8, 0, 0, 0, time.UTC)
	finished := started.Add(12 * time.Second)
	next := started.Add(24 * time.Hour)
	activeStarted := started.Add(2 * time.Hour)
	deadline := time.Date(2026, 7, 7, 19, 20, 0, 0, time.UTC)
	store := &fakeAdminObjectStorageStore{
		current: db.GetAdminObjectStorageCurrentRow{
			TrackedObjects:        12,
			PendingObjects:        2,
			UploadedObjects:       10,
			ConfirmedTrackedBytes: 4096,
		},
		additions: db.GetAdminObjectStoragePeriodAdditionsRow{
			AddedObjects:        5,
			AddedConfirmedBytes: 2048,
		},
		backlog: db.GetAdminObjectStorageDueBacklogRow{
			DueObjects: 3,
			DueBytes:   1024,
		},
		referencedObjects: 8,
		nextDeadline:      pgtype.Timestamptz{Time: deadline, Valid: true},
		periodRuns: db.GetAdminObjectStoragePeriodCleanupRunsRow{
			DeletedObjects:    4,
			DeletedBytes:      8192,
			CleanupRuns:       2,
			FailedObjectCount: 1,
			FailedRunCount:    1,
		},
		runningSummary: db.GetAdminObjectStorageRunningSummaryRow{
			ActiveRunStartedAt: pgtype.Timestamptz{Time: activeStarted, Valid: true},
			StaleRunningRuns:   2,
		},
		recentRuns: []db.MediaCleanupRun{{
			ID:             "run_1",
			Status:         "completed",
			StartedAt:      pgtype.Timestamptz{Time: started, Valid: true},
			FinishedAt:     pgtype.Timestamptz{Time: finished, Valid: true},
			NextRunAt:      pgtype.Timestamptz{Time: next, Valid: true},
			DeletedObjects: 4,
			DeletedBytes:   8192,
		}},
		contentTypes: []db.GetAdminObjectStorageContentTypesRow{{
			ContentType:           "video/mp4",
			TrackedObjects:        7,
			ConfirmedTrackedBytes: 3000,
		}},
		statusBreakdown: []db.GetAdminObjectStorageStatusBreakdownRow{{
			Status:                "uploaded",
			TrackedObjects:        10,
			ConfirmedTrackedBytes: 4096,
		}},
		dailyActivity: []db.ListAdminObjectStorageDailyActivityRow{
			{Day: pgtype.Date{Time: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), Valid: true}, ConfirmedBytes: 1024},
			{Day: pgtype.Date{Time: time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC), Valid: true}, DeletedBytes: 2048},
			{Day: pgtype.Date{Time: time.Date(2026, 7, 4, 0, 0, 0, 0, time.UTC), Valid: true}, ConfirmedBytes: 512, DeletedBytes: 256},
		},
	}
	h := NewAdminObjectStorageHandler(store, "unipost-media")
	h.now = func() time.Time { return time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC) }
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/object-storage?period=this_month", nil)
	rr := httptest.NewRecorder()

	h.Get(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Data adminObjectStorageResponse `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Data.Current.ConfirmedTrackedBytes != 4096 || got.Data.Current.ReferencedObjects != 8 {
		t.Fatalf("current = %#v, want confirmed bytes and referenced objects", got.Data.Current)
	}
	if len(got.Data.Buckets) != 1 || got.Data.Buckets[0].BucketName != "unipost-media" || got.Data.Buckets[0].ConfirmedTrackedBytes != 4096 {
		t.Fatalf("buckets = %#v, want configured bucket with confirmed bytes", got.Data.Buckets)
	}
	if got.Data.Worker.EstimatedNextRunAt == nil || *got.Data.Worker.EstimatedNextRunAt != next.Format(time.RFC3339) {
		t.Fatalf("worker = %#v, want estimated next run", got.Data.Worker)
	}
	if got.Data.Worker.ActiveRunStartedAt == nil || *got.Data.Worker.ActiveRunStartedAt != activeStarted.Format(time.RFC3339) || got.Data.Worker.StaleRunningRuns != 2 {
		t.Fatalf("worker = %#v, want running summary", got.Data.Worker)
	}
	if got.Data.PeriodMetrics.FailedObjectCount != 1 || got.Data.PeriodMetrics.FailedRunCount != 1 {
		t.Fatalf("period metrics = %#v, want separate failure counts", got.Data.PeriodMetrics)
	}
	if len(got.Data.DailyActivity) != 4 {
		t.Fatalf("daily activity length = %d, want 4", len(got.Data.DailyActivity))
	}
	if day := got.Data.DailyActivity[1]; day.Date != "2026-07-02" || day.ConfirmedBytes != 0 || day.DeletedBytes != 0 {
		t.Fatalf("daily activity[1] = %#v, want zero-filled July 2", day)
	}
	if day := got.Data.DailyActivity[3]; day.Date != "2026-07-04" || day.ConfirmedBytes != 512 || day.DeletedBytes != 256 {
		t.Fatalf("daily activity[3] = %#v, want paired July 4 values", day)
	}
}

func TestAdminObjectStorageRouteIsRegistered(t *testing.T) {
	source, err := os.ReadFile("../../cmd/api/main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	body := string(source)
	if !strings.Contains(body, `adminObjectStorageHandler := handler.NewAdminObjectStorageHandler`) {
		t.Fatalf("admin object storage handler is not constructed")
	}
	if !strings.Contains(body, `r.Get("/v1/admin/object-storage", adminObjectStorageHandler.Get)`) {
		t.Fatalf("admin object storage route is not registered")
	}
}

func TestAdminObjectStorageDailyActivityFillsHalfOpenPeriod(t *testing.T) {
	from := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 7, 0, 0, 0, 0, time.UTC)
	rows := []db.ListAdminObjectStorageDailyActivityRow{{
		Day:            pgtype.Date{Time: from, Valid: true},
		ConfirmedBytes: 1024,
	}}

	got := adminObjectStorageDailyActivity(rows, from, to)

	if len(got) != 1 {
		t.Fatalf("daily activity length = %d, want 1", len(got))
	}
	if got[0].Date != "2026-07-06" || got[0].ConfirmedBytes != 1024 || got[0].DeletedBytes != 0 {
		t.Fatalf("daily activity = %#v, want only July 6 confirmation", got)
	}
}

type fakeAdminObjectStorageStore struct {
	current           db.GetAdminObjectStorageCurrentRow
	additions         db.GetAdminObjectStoragePeriodAdditionsRow
	backlog           db.GetAdminObjectStorageDueBacklogRow
	referencedObjects int64
	nextDeadline      pgtype.Timestamptz
	periodRuns        db.GetAdminObjectStoragePeriodCleanupRunsRow
	runningSummary    db.GetAdminObjectStorageRunningSummaryRow
	recentRuns        []db.MediaCleanupRun
	contentTypes      []db.GetAdminObjectStorageContentTypesRow
	statusBreakdown   []db.GetAdminObjectStorageStatusBreakdownRow
	dailyActivity     []db.ListAdminObjectStorageDailyActivityRow
}

func (f *fakeAdminObjectStorageStore) GetAdminObjectStorageCurrent(context.Context) (db.GetAdminObjectStorageCurrentRow, error) {
	return f.current, nil
}

func (f *fakeAdminObjectStorageStore) GetAdminObjectStoragePeriodAdditions(context.Context, db.GetAdminObjectStoragePeriodAdditionsParams) (db.GetAdminObjectStoragePeriodAdditionsRow, error) {
	return f.additions, nil
}

func (f *fakeAdminObjectStorageStore) GetAdminObjectStorageDueBacklog(context.Context) (db.GetAdminObjectStorageDueBacklogRow, error) {
	return f.backlog, nil
}

func (f *fakeAdminObjectStorageStore) GetAdminObjectStorageReferencedObjects(context.Context) (int64, error) {
	return f.referencedObjects, nil
}

func (f *fakeAdminObjectStorageStore) GetAdminObjectStorageNextCleanupDeadline(context.Context) (pgtype.Timestamptz, error) {
	return f.nextDeadline, nil
}

func (f *fakeAdminObjectStorageStore) GetAdminObjectStoragePeriodCleanupRuns(context.Context, db.GetAdminObjectStoragePeriodCleanupRunsParams) (db.GetAdminObjectStoragePeriodCleanupRunsRow, error) {
	return f.periodRuns, nil
}

func (f *fakeAdminObjectStorageStore) GetAdminObjectStorageRunningSummary(context.Context, db.GetAdminObjectStorageRunningSummaryParams) (db.GetAdminObjectStorageRunningSummaryRow, error) {
	return f.runningSummary, nil
}

func (f *fakeAdminObjectStorageStore) ListAdminObjectStorageRecentRuns(context.Context, int32) ([]db.MediaCleanupRun, error) {
	return f.recentRuns, nil
}

func (f *fakeAdminObjectStorageStore) GetAdminObjectStorageContentTypes(context.Context, int32) ([]db.GetAdminObjectStorageContentTypesRow, error) {
	return f.contentTypes, nil
}

func (f *fakeAdminObjectStorageStore) GetAdminObjectStorageStatusBreakdown(context.Context, int32) ([]db.GetAdminObjectStorageStatusBreakdownRow, error) {
	return f.statusBreakdown, nil
}

func (f *fakeAdminObjectStorageStore) ListAdminObjectStorageDailyActivity(context.Context, db.ListAdminObjectStorageDailyActivityParams) ([]db.ListAdminObjectStorageDailyActivityRow, error) {
	return f.dailyActivity, nil
}
