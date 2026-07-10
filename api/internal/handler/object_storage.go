package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type AdminObjectStorageHandler struct {
	store      adminObjectStorageStore
	bucketName string
	now        func() time.Time
}

const adminObjectStorageWorkerName = "media_cleanup"
const adminObjectStorageStaleRunningAfter = 48 * time.Hour

type adminObjectStorageStore interface {
	GetAdminObjectStorageCurrent(context.Context) (db.GetAdminObjectStorageCurrentRow, error)
	GetAdminObjectStoragePeriodAdditions(context.Context, db.GetAdminObjectStoragePeriodAdditionsParams) (db.GetAdminObjectStoragePeriodAdditionsRow, error)
	GetAdminObjectStorageDueBacklog(context.Context) (db.GetAdminObjectStorageDueBacklogRow, error)
	GetAdminObjectStorageReferencedObjects(context.Context) (int64, error)
	GetAdminObjectStorageNextCleanupDeadline(context.Context) (pgtype.Timestamptz, error)
	GetAdminObjectStoragePeriodCleanupRuns(context.Context, db.GetAdminObjectStoragePeriodCleanupRunsParams) (db.GetAdminObjectStoragePeriodCleanupRunsRow, error)
	ListAdminObjectStorageDailyActivity(context.Context, db.ListAdminObjectStorageDailyActivityParams) ([]db.ListAdminObjectStorageDailyActivityRow, error)
	GetAdminObjectStorageRunningSummary(context.Context, db.GetAdminObjectStorageRunningSummaryParams) (db.GetAdminObjectStorageRunningSummaryRow, error)
	ListAdminObjectStorageRecentRuns(context.Context, int32) ([]db.MediaCleanupRun, error)
	GetAdminObjectStorageContentTypes(context.Context, int32) ([]db.GetAdminObjectStorageContentTypesRow, error)
	GetAdminObjectStorageStatusBreakdown(context.Context, int32) ([]db.GetAdminObjectStorageStatusBreakdownRow, error)
}

func NewAdminObjectStorageHandler(store adminObjectStorageStore, bucketName string) *AdminObjectStorageHandler {
	if bucketName == "" {
		bucketName = "not configured"
	}
	return &AdminObjectStorageHandler{
		store:      store,
		bucketName: bucketName,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

type adminObjectStoragePeriod struct {
	Key  string    `json:"key"`
	From time.Time `json:"-"`
	To   time.Time `json:"-"`
}

type adminObjectStoragePeriodResponse struct {
	Key  string `json:"key"`
	From string `json:"from"`
	To   string `json:"to"`
}

type adminObjectStorageResponse struct {
	Period          adminObjectStoragePeriodResponse            `json:"period"`
	Current         adminObjectStorageCurrentResponse           `json:"current"`
	Worker          adminObjectStorageWorkerResponse            `json:"worker"`
	PeriodMetrics   adminObjectStoragePeriodMetricsResponse     `json:"period_metrics"`
	Backlog         adminObjectStorageBacklogResponse           `json:"backlog"`
	Buckets         []adminObjectStorageBucketResponse          `json:"buckets"`
	ContentTypes    []adminObjectStorageContentTypeResponse     `json:"content_types"`
	StatusBreakdown []adminObjectStorageStatusBreakdownResponse `json:"status_breakdown"`
	RecentRuns      []adminObjectStorageRunResponse             `json:"recent_runs"`
	DailyActivity   []adminObjectStorageDailyActivityResponse   `json:"daily_activity"`
}

type adminObjectStorageCurrentResponse struct {
	TrackedObjects        int64 `json:"tracked_objects"`
	ConfirmedTrackedBytes int64 `json:"confirmed_tracked_bytes"`
	PendingObjects        int64 `json:"pending_objects"`
	UploadedObjects       int64 `json:"uploaded_objects"`
	ReferencedObjects     int64 `json:"referenced_objects"`
}

type adminObjectStorageWorkerResponse struct {
	LastRunStartedAt   *string `json:"last_run_started_at"`
	LastRunFinishedAt  *string `json:"last_run_finished_at"`
	LastRunStatus      *string `json:"last_run_status"`
	EstimatedNextRunAt *string `json:"estimated_next_run_at"`
	LastDeletedObjects int32   `json:"last_deleted_objects"`
	LastDeletedBytes   int64   `json:"last_deleted_bytes"`
	LastFailedObjects  int32   `json:"last_failed_objects"`
	ActiveRunStartedAt *string `json:"active_run_started_at"`
	StaleRunningRuns   int64   `json:"stale_running_runs"`
}

type adminObjectStoragePeriodMetricsResponse struct {
	AddedObjects        int64 `json:"added_objects"`
	AddedConfirmedBytes int64 `json:"added_confirmed_bytes"`
	DeletedObjects      int64 `json:"deleted_objects"`
	DeletedBytes        int64 `json:"deleted_bytes"`
	CleanupRuns         int64 `json:"cleanup_runs"`
	FailedObjectCount   int64 `json:"failed_object_count"`
	FailedRunCount      int64 `json:"failed_run_count"`
}

type adminObjectStorageBacklogResponse struct {
	DueObjects            int64   `json:"due_objects"`
	DueBytes              int64   `json:"due_bytes"`
	NextCleanupDeadlineAt *string `json:"next_cleanup_deadline_at"`
}

type adminObjectStorageBucketResponse struct {
	BucketName            string `json:"bucket_name"`
	TrackedObjects        int64  `json:"tracked_objects"`
	ConfirmedTrackedBytes int64  `json:"confirmed_tracked_bytes"`
	PendingObjects        int64  `json:"pending_objects"`
	UploadedObjects       int64  `json:"uploaded_objects"`
	ReferencedObjects     int64  `json:"referenced_objects"`
	DueObjects            int64  `json:"due_objects"`
	DueBytes              int64  `json:"due_bytes"`
}

type adminObjectStorageContentTypeResponse struct {
	ContentType           string `json:"content_type"`
	TrackedObjects        int64  `json:"tracked_objects"`
	ConfirmedTrackedBytes int64  `json:"confirmed_tracked_bytes"`
}

type adminObjectStorageStatusBreakdownResponse struct {
	Status                string `json:"status"`
	TrackedObjects        int64  `json:"tracked_objects"`
	ConfirmedTrackedBytes int64  `json:"confirmed_tracked_bytes"`
}

type adminObjectStorageRunResponse struct {
	ID             string  `json:"id"`
	Status         string  `json:"status"`
	StartedAt      *string `json:"started_at"`
	FinishedAt     *string `json:"finished_at"`
	DeletedObjects int32   `json:"deleted_objects"`
	DeletedBytes   int64   `json:"deleted_bytes"`
	FailedObjects  int32   `json:"failed_objects"`
	ErrorSummary   string  `json:"error_summary"`
}

type adminObjectStorageDailyActivityResponse struct {
	Date           string `json:"date"`
	ConfirmedBytes int64  `json:"confirmed_bytes"`
	DeletedBytes   int64  `json:"deleted_bytes"`
}

func (h *AdminObjectStorageHandler) Get(w http.ResponseWriter, r *http.Request) {
	period, err := parseAdminObjectStoragePeriod(r.URL.Query().Get("period"), h.now())
	if err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	from := pgtype.Timestamptz{Time: period.From, Valid: true}
	to := pgtype.Timestamptz{Time: period.To, Valid: true}

	current, err := h.store.GetAdminObjectStorageCurrent(r.Context())
	if err != nil {
		h.writeQueryError(w, "current", err)
		return
	}
	additions, err := h.store.GetAdminObjectStoragePeriodAdditions(r.Context(), db.GetAdminObjectStoragePeriodAdditionsParams{
		CreatedAt:   from,
		CreatedAt_2: to,
	})
	if err != nil {
		h.writeQueryError(w, "period additions", err)
		return
	}
	backlog, err := h.store.GetAdminObjectStorageDueBacklog(r.Context())
	if err != nil {
		h.writeQueryError(w, "due backlog", err)
		return
	}
	referenced, err := h.store.GetAdminObjectStorageReferencedObjects(r.Context())
	if err != nil {
		h.writeQueryError(w, "referenced objects", err)
		return
	}
	nextDeadline, err := h.store.GetAdminObjectStorageNextCleanupDeadline(r.Context())
	if err != nil {
		h.writeQueryError(w, "next cleanup deadline", err)
		return
	}
	periodRuns, err := h.store.GetAdminObjectStoragePeriodCleanupRuns(r.Context(), db.GetAdminObjectStoragePeriodCleanupRunsParams{
		FinishedAt:   from,
		FinishedAt_2: to,
	})
	if err != nil {
		h.writeQueryError(w, "period cleanup runs", err)
		return
	}
	dailyActivity, err := h.store.ListAdminObjectStorageDailyActivity(r.Context(), db.ListAdminObjectStorageDailyActivityParams{
		PeriodFrom: from,
		PeriodTo:   to,
	})
	if err != nil {
		h.writeQueryError(w, "daily activity", err)
		return
	}
	runningSummary, err := h.store.GetAdminObjectStorageRunningSummary(r.Context(), db.GetAdminObjectStorageRunningSummaryParams{
		StartedAt:  pgtype.Timestamptz{Time: h.now().UTC().Add(-adminObjectStorageStaleRunningAfter), Valid: true},
		WorkerName: adminObjectStorageWorkerName,
	})
	if err != nil {
		h.writeQueryError(w, "running cleanup runs", err)
		return
	}
	recentRuns, err := h.store.ListAdminObjectStorageRecentRuns(r.Context(), 10)
	if err != nil {
		h.writeQueryError(w, "recent cleanup runs", err)
		return
	}
	contentTypes, err := h.store.GetAdminObjectStorageContentTypes(r.Context(), 12)
	if err != nil {
		h.writeQueryError(w, "content types", err)
		return
	}
	statusBreakdown, err := h.store.GetAdminObjectStorageStatusBreakdown(r.Context(), 8)
	if err != nil {
		h.writeQueryError(w, "status breakdown", err)
		return
	}

	currentResponse := adminObjectStorageCurrentResponse{
		TrackedObjects:        current.TrackedObjects,
		ConfirmedTrackedBytes: current.ConfirmedTrackedBytes,
		PendingObjects:        current.PendingObjects,
		UploadedObjects:       current.UploadedObjects,
		ReferencedObjects:     referenced,
	}
	backlogResponse := adminObjectStorageBacklogResponse{
		DueObjects:            backlog.DueObjects,
		DueBytes:              backlog.DueBytes,
		NextCleanupDeadlineAt: timeString(nextDeadline),
	}
	writeSuccess(w, adminObjectStorageResponse{
		Period: adminObjectStoragePeriodResponse{
			Key:  period.Key,
			From: period.From.Format(time.RFC3339),
			To:   period.To.Format(time.RFC3339),
		},
		Current: currentResponse,
		Worker:  adminObjectStorageWorkerFromRuns(recentRuns, runningSummary),
		PeriodMetrics: adminObjectStoragePeriodMetricsResponse{
			AddedObjects:        additions.AddedObjects,
			AddedConfirmedBytes: additions.AddedConfirmedBytes,
			DeletedObjects:      periodRuns.DeletedObjects,
			DeletedBytes:        periodRuns.DeletedBytes,
			CleanupRuns:         periodRuns.CleanupRuns,
			FailedObjectCount:   periodRuns.FailedObjectCount,
			FailedRunCount:      periodRuns.FailedRunCount,
		},
		Backlog: backlogResponse,
		Buckets: []adminObjectStorageBucketResponse{{
			BucketName:            h.bucketName,
			TrackedObjects:        currentResponse.TrackedObjects,
			ConfirmedTrackedBytes: currentResponse.ConfirmedTrackedBytes,
			PendingObjects:        currentResponse.PendingObjects,
			UploadedObjects:       currentResponse.UploadedObjects,
			ReferencedObjects:     currentResponse.ReferencedObjects,
			DueObjects:            backlogResponse.DueObjects,
			DueBytes:              backlogResponse.DueBytes,
		}},
		ContentTypes:    adminObjectStorageContentTypes(contentTypes),
		StatusBreakdown: adminObjectStorageStatusBreakdown(statusBreakdown),
		RecentRuns:      adminObjectStorageRuns(recentRuns),
		DailyActivity:   adminObjectStorageDailyActivity(dailyActivity, period.From, period.To),
	})
}

func (h *AdminObjectStorageHandler) writeQueryError(w http.ResponseWriter, label string, err error) {
	slog.Error("admin object storage: query failed", "query", label, "error", err)
	writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load object storage metrics")
}

func parseAdminObjectStoragePeriod(key string, now time.Time) (adminObjectStoragePeriod, error) {
	now = now.UTC()
	if key == "" {
		key = "last_7_days"
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	switch key {
	case "yesterday":
		return adminObjectStoragePeriod{Key: key, From: today.AddDate(0, 0, -1), To: today}, nil
	case "last_7_days":
		return adminObjectStoragePeriod{Key: key, From: now.AddDate(0, 0, -7), To: now}, nil
	case "last_month":
		thisMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		return adminObjectStoragePeriod{Key: key, From: thisMonth.AddDate(0, -1, 0), To: thisMonth}, nil
	case "this_week":
		daysSinceMonday := (int(now.Weekday()) + 6) % 7
		return adminObjectStoragePeriod{Key: key, From: today.AddDate(0, 0, -daysSinceMonday), To: now}, nil
	case "this_month":
		return adminObjectStoragePeriod{Key: key, From: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), To: now}, nil
	case "this_year":
		return adminObjectStoragePeriod{Key: key, From: time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC), To: now}, nil
	default:
		return adminObjectStoragePeriod{}, fmt.Errorf("invalid period")
	}
}

func adminObjectStorageWorkerFromRuns(runs []db.MediaCleanupRun, running db.GetAdminObjectStorageRunningSummaryRow) adminObjectStorageWorkerResponse {
	if len(runs) == 0 {
		return adminObjectStorageWorkerResponse{
			ActiveRunStartedAt: timeString(running.ActiveRunStartedAt),
			StaleRunningRuns:   running.StaleRunningRuns,
		}
	}
	latest := runs[0]
	status := latest.Status
	return adminObjectStorageWorkerResponse{
		LastRunStartedAt:   timeString(latest.StartedAt),
		LastRunFinishedAt:  timeString(latest.FinishedAt),
		LastRunStatus:      &status,
		EstimatedNextRunAt: timeString(latest.NextRunAt),
		LastDeletedObjects: latest.DeletedObjects,
		LastDeletedBytes:   latest.DeletedBytes,
		LastFailedObjects:  latest.FailedObjects,
		ActiveRunStartedAt: timeString(running.ActiveRunStartedAt),
		StaleRunningRuns:   running.StaleRunningRuns,
	}
}

func adminObjectStorageContentTypes(rows []db.GetAdminObjectStorageContentTypesRow) []adminObjectStorageContentTypeResponse {
	out := make([]adminObjectStorageContentTypeResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminObjectStorageContentTypeResponse(row))
	}
	return out
}

func adminObjectStorageStatusBreakdown(rows []db.GetAdminObjectStorageStatusBreakdownRow) []adminObjectStorageStatusBreakdownResponse {
	out := make([]adminObjectStorageStatusBreakdownResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, adminObjectStorageStatusBreakdownResponse(row))
	}
	return out
}

func adminObjectStorageRuns(rows []db.MediaCleanupRun) []adminObjectStorageRunResponse {
	out := make([]adminObjectStorageRunResponse, 0, len(rows))
	for _, row := range rows {
		errorSummary := ""
		if row.ErrorSummary.Valid {
			errorSummary = row.ErrorSummary.String
		}
		out = append(out, adminObjectStorageRunResponse{
			ID:             row.ID,
			Status:         row.Status,
			StartedAt:      timeString(row.StartedAt),
			FinishedAt:     timeString(row.FinishedAt),
			DeletedObjects: row.DeletedObjects,
			DeletedBytes:   row.DeletedBytes,
			FailedObjects:  row.FailedObjects,
			ErrorSummary:   errorSummary,
		})
	}
	return out
}

func adminObjectStorageDailyActivity(rows []db.ListAdminObjectStorageDailyActivityRow, from, to time.Time) []adminObjectStorageDailyActivityResponse {
	totals := make(map[string]adminObjectStorageDailyActivityResponse, len(rows))
	for _, row := range rows {
		if !row.Day.Valid {
			continue
		}
		key := row.Day.Time.UTC().Format("2006-01-02")
		totals[key] = adminObjectStorageDailyActivityResponse{
			Date:           key,
			ConfirmedBytes: row.ConfirmedBytes,
			DeletedBytes:   row.DeletedBytes,
		}
	}

	start := time.Date(from.UTC().Year(), from.UTC().Month(), from.UTC().Day(), 0, 0, 0, 0, time.UTC)
	inclusiveEnd := to.UTC().Add(-time.Nanosecond)
	end := time.Date(inclusiveEnd.Year(), inclusiveEnd.Month(), inclusiveEnd.Day(), 0, 0, 0, 0, time.UTC)
	out := make([]adminObjectStorageDailyActivityResponse, 0, int(end.Sub(start).Hours()/24)+1)
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		key := day.Format("2006-01-02")
		if row, ok := totals[key]; ok {
			out = append(out, row)
			continue
		}
		out = append(out, adminObjectStorageDailyActivityResponse{Date: key})
	}
	return out
}

func timeString(ts pgtype.Timestamptz) *string {
	if !ts.Valid {
		return nil
	}
	formatted := ts.Time.UTC().Format(time.RFC3339)
	return &formatted
}
