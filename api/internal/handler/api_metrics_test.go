package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type fakeAPIMetricsStore struct {
	overallCalls         int
	trendHourlyCalls     int
	trendDailyCalls      int
	statusCodeCalls      int
	adminOverallCalls    int
	adminWorkspacesCalls int

	overallRow         db.GetAPIMetricsOverallRow
	trendRows          []db.GetAPIMetricsTrendHourlyRow
	statusRows         []db.GetAPIMetricsStatusCodesRow
	adminOverallRow    db.GetAdminAPIMetricsOverallRow
	adminWorkspaceRows []db.GetAdminAPIMetricsWorkspacesRow
}

func (f *fakeAPIMetricsStore) GetAPIMetricsOverall(ctx context.Context, arg db.GetAPIMetricsOverallParams) (db.GetAPIMetricsOverallRow, error) {
	f.overallCalls++
	return f.overallRow, nil
}

func (f *fakeAPIMetricsStore) GetAPIMetricsSummary(ctx context.Context, arg db.GetAPIMetricsSummaryParams) ([]db.GetAPIMetricsSummaryRow, error) {
	return nil, nil
}

func (f *fakeAPIMetricsStore) GetAPIMetricsTrendHourly(ctx context.Context, arg db.GetAPIMetricsTrendHourlyParams) ([]db.GetAPIMetricsTrendHourlyRow, error) {
	f.trendHourlyCalls++
	return f.trendRows, nil
}

func (f *fakeAPIMetricsStore) GetAPIMetricsTrendDaily(ctx context.Context, arg db.GetAPIMetricsTrendDailyParams) ([]db.GetAPIMetricsTrendDailyRow, error) {
	f.trendDailyCalls++
	rows := make([]db.GetAPIMetricsTrendDailyRow, 0, len(f.trendRows))
	for _, row := range f.trendRows {
		rows = append(rows, db.GetAPIMetricsTrendDailyRow(row))
	}
	return rows, nil
}

func (f *fakeAPIMetricsStore) GetAPIMetricsStatusCodes(ctx context.Context, arg db.GetAPIMetricsStatusCodesParams) ([]db.GetAPIMetricsStatusCodesRow, error) {
	f.statusCodeCalls++
	return f.statusRows, nil
}

func (f *fakeAPIMetricsStore) GetAdminAPIMetricsOverall(ctx context.Context, arg db.GetAdminAPIMetricsOverallParams) (db.GetAdminAPIMetricsOverallRow, error) {
	f.adminOverallCalls++
	return f.adminOverallRow, nil
}

func (f *fakeAPIMetricsStore) GetAdminAPIMetricsSummary(ctx context.Context, arg db.GetAdminAPIMetricsSummaryParams) ([]db.GetAdminAPIMetricsSummaryRow, error) {
	return nil, nil
}

func (f *fakeAPIMetricsStore) GetAdminAPIMetricsTrendHourly(ctx context.Context, arg db.GetAdminAPIMetricsTrendHourlyParams) ([]db.GetAdminAPIMetricsTrendHourlyRow, error) {
	return nil, nil
}

func (f *fakeAPIMetricsStore) GetAdminAPIMetricsTrendDaily(ctx context.Context, arg db.GetAdminAPIMetricsTrendDailyParams) ([]db.GetAdminAPIMetricsTrendDailyRow, error) {
	return nil, nil
}

func (f *fakeAPIMetricsStore) GetAdminAPIMetricsStatusCodes(ctx context.Context, arg db.GetAdminAPIMetricsStatusCodesParams) ([]db.GetAdminAPIMetricsStatusCodesRow, error) {
	return nil, nil
}

func (f *fakeAPIMetricsStore) GetAdminAPIMetricsWorkspaces(ctx context.Context, arg db.GetAdminAPIMetricsWorkspacesParams) ([]db.GetAdminAPIMetricsWorkspacesRow, error) {
	f.adminWorkspacesCalls++
	return f.adminWorkspaceRows, nil
}

func TestAPIMetricsOverallRejectsInvalidTimeRange(t *testing.T) {
	store := &fakeAPIMetricsStore{}
	h := NewAPIMetricsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/api-metrics/overall?from=nope", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rr := httptest.NewRecorder()

	h.Overall(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	if store.overallCalls != 0 {
		t.Fatalf("overall query calls = %d, want 0 on invalid range", store.overallCalls)
	}
	var got ErrorResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal error response: %v", err)
	}
	if got.Error.Code != "VALIDATION_ERROR" {
		t.Fatalf("error code = %q, want VALIDATION_ERROR", got.Error.Code)
	}
}

func TestAPIMetricsOverallReturnsV1Taxonomy(t *testing.T) {
	store := &fakeAPIMetricsStore{overallRow: db.GetAPIMetricsOverallRow{
		TotalCalls:           100,
		SuccessCount:         90,
		ClientErrorCount:     7,
		ServerErrorCount:     3,
		RateLimitedCount:     2,
		ErrorRatePct:         10,
		ServerFailureRatePct: 3,
		P50Ms:                100,
		P95Ms:                400,
		P99Ms:                900,
		AvgMs:                150,
	}}
	h := NewAPIMetricsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/api-metrics/overall", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rr := httptest.NewRecorder()

	h.Overall(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	var got struct {
		Data map[string]float64 `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	for field, want := range map[string]float64{
		"total_calls":             100,
		"success_count":           90,
		"client_error_count":      7,
		"server_error_count":      3,
		"rate_limited_count":      2,
		"error_rate_pct":          10,
		"server_failure_rate_pct": 3,
		"reliability_pct":         97,
		"p50_ms":                  100,
		"p95_ms":                  400,
		"p99_ms":                  900,
		"avg_ms":                  150,
	} {
		if got.Data[field] != want {
			t.Fatalf("%s = %v, want %v in %#v", field, got.Data[field], want, got.Data)
		}
	}
}

func TestAPIMetricsTrendSelectsDailyInterval(t *testing.T) {
	store := &fakeAPIMetricsStore{trendRows: []db.GetAPIMetricsTrendHourlyRow{{
		Bucket:       pgtype.Timestamptz{Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Valid: true},
		TotalCalls:   4,
		SuccessCount: 3,
		ErrorCount:   1,
		P95Ms:        250,
	}}}
	h := NewAPIMetricsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/api-metrics/trend?interval=day", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rr := httptest.NewRecorder()

	h.Trend(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if store.trendDailyCalls != 1 || store.trendHourlyCalls != 0 {
		t.Fatalf("daily/hourly calls = %d/%d, want 1/0", store.trendDailyCalls, store.trendHourlyCalls)
	}
}

func TestAPIMetricsStatusCodesReturnsDistribution(t *testing.T) {
	store := &fakeAPIMetricsStore{statusRows: []db.GetAPIMetricsStatusCodesRow{{
		StatusCode: 429,
		TotalCalls: 3,
		Method:     "POST",
		Path:       "/v1/posts",
	}}}
	h := NewAPIMetricsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/api-metrics/status-codes", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rr := httptest.NewRecorder()

	h.StatusCodes(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if store.statusCodeCalls != 1 {
		t.Fatalf("status code query calls = %d, want 1", store.statusCodeCalls)
	}
	var got struct {
		Data []db.GetAPIMetricsStatusCodesRow `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got.Data) != 1 || got.Data[0].StatusCode != 429 || got.Data[0].TotalCalls != 3 {
		t.Fatalf("status rows = %#v, want one 429 row", got.Data)
	}
}

func TestAdminAPIMetricsOverallReturnsGlobalMetrics(t *testing.T) {
	store := &fakeAPIMetricsStore{adminOverallRow: db.GetAdminAPIMetricsOverallRow{
		TotalCalls:           250,
		SuccessCount:         240,
		ClientErrorCount:     8,
		ServerErrorCount:     2,
		RateLimitedCount:     4,
		ErrorRatePct:         4,
		ServerFailureRatePct: 0.8,
		P50Ms:                80,
		P95Ms:                360,
		P99Ms:                940,
		AvgMs:                120,
	}}
	h := NewAdminAPIMetricsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/api-metrics/overall", nil)
	rr := httptest.NewRecorder()

	h.Overall(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if store.adminOverallCalls != 1 {
		t.Fatalf("admin overall calls = %d, want 1", store.adminOverallCalls)
	}
	var got struct {
		Data map[string]float64 `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Data["total_calls"] != 250 || got.Data["rate_limited_count"] != 4 || got.Data["reliability_pct"] != 99.2 {
		t.Fatalf("admin overall data = %#v, want total/rate-limited/reliability", got.Data)
	}
}

func TestAdminAPIMetricsWorkspacesReturnsImpactRows(t *testing.T) {
	store := &fakeAPIMetricsStore{adminWorkspaceRows: []db.GetAdminAPIMetricsWorkspacesRow{{
		WorkspaceID:          "ws_1",
		WorkspaceName:        "Default",
		TotalCalls:           42,
		RateLimitedCount:     5,
		ErrorRatePct:         14.2,
		ServerFailureRatePct: 2.4,
		P95Ms:                700,
		P99Ms:                1200,
		SlowestEndpoint:      "/v1/posts/:id/publish",
		SlowestEndpointP95Ms: 900,
	}}}
	h := NewAdminAPIMetricsHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/api-metrics/workspaces", nil)
	rr := httptest.NewRecorder()

	h.Workspaces(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rr.Code, rr.Body.String())
	}
	if store.adminWorkspacesCalls != 1 {
		t.Fatalf("admin workspaces calls = %d, want 1", store.adminWorkspacesCalls)
	}
	var got struct {
		Data []db.GetAdminAPIMetricsWorkspacesRow `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got.Data) != 1 || got.Data[0].WorkspaceID != "ws_1" || got.Data[0].SlowestEndpointP95Ms != 900 {
		t.Fatalf("workspace rows = %#v, want impact row", got.Data)
	}
}
