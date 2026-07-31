package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/observabilityreads"
)

type fixedMetricsSelector bool

func (s fixedMetricsSelector) UseV2(context.Context) bool { return bool(s) }

type pointerMetricsSelector struct{ enabled bool }

func (s *pointerMetricsSelector) UseV2(context.Context) bool { return s != nil && s.enabled }

type failingMetricsPublicEvaluator struct{}

func (failingMetricsPublicEvaluator) Public(context.Context, string) (bool, error) {
	return false, errors.New("flag unavailable")
}

type fakeV2MetricsReader struct {
	overallCalls   int
	summaryCalls   int
	trendCalls     int
	statusCalls    int
	workspaceCalls int
	overall        observabilityreads.MetricsOverall
	summaries      []observabilityreads.MetricsSummaryRow
	trends         []observabilityreads.MetricsTrendRow
	statuses       []observabilityreads.MetricsStatusCodeRow
	workspaces     []observabilityreads.AdminMetricsWorkspaceRow
	freshness      observabilityreads.MetricsFreshness
	lastQuery      observabilityreads.MetricsQuery
}

func (f *fakeV2MetricsReader) Overall(_ context.Context, query observabilityreads.MetricsQuery) (observabilityreads.MetricsOverall, observabilityreads.MetricsFreshness, error) {
	f.overallCalls++
	f.lastQuery = query
	return f.overall, f.freshness, nil
}
func (f *fakeV2MetricsReader) Summary(_ context.Context, query observabilityreads.MetricsQuery) ([]observabilityreads.MetricsSummaryRow, observabilityreads.MetricsFreshness, error) {
	f.summaryCalls++
	f.lastQuery = query
	return f.summaries, f.freshness, nil
}
func (f *fakeV2MetricsReader) Trend(_ context.Context, query observabilityreads.MetricsQuery) ([]observabilityreads.MetricsTrendRow, observabilityreads.MetricsFreshness, error) {
	f.trendCalls++
	f.lastQuery = query
	return f.trends, f.freshness, nil
}
func (f *fakeV2MetricsReader) StatusCodes(_ context.Context, query observabilityreads.MetricsQuery) ([]observabilityreads.MetricsStatusCodeRow, observabilityreads.MetricsFreshness, error) {
	f.statusCalls++
	f.lastQuery = query
	return f.statuses, f.freshness, nil
}
func (f *fakeV2MetricsReader) Workspaces(_ context.Context, query observabilityreads.MetricsQuery) ([]observabilityreads.AdminMetricsWorkspaceRow, observabilityreads.MetricsFreshness, error) {
	f.workspaceCalls++
	f.lastQuery = query
	return f.workspaces, f.freshness, nil
}

type fakeAPIMetricsStore struct {
	overallCalls         int
	summaryCalls         int
	trendHourlyCalls     int
	trendDailyCalls      int
	statusCodeCalls      int
	adminOverallCalls    int
	adminSummaryCalls    int
	adminTrendCalls      int
	adminStatusCalls     int
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
	f.summaryCalls++
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
	f.adminSummaryCalls++
	return nil, nil
}

func (f *fakeAPIMetricsStore) GetAdminAPIMetricsTrendHourly(ctx context.Context, arg db.GetAdminAPIMetricsTrendHourlyParams) ([]db.GetAdminAPIMetricsTrendHourlyRow, error) {
	f.adminTrendCalls++
	return nil, nil
}

func (f *fakeAPIMetricsStore) GetAdminAPIMetricsTrendDaily(ctx context.Context, arg db.GetAdminAPIMetricsTrendDailyParams) ([]db.GetAdminAPIMetricsTrendDailyRow, error) {
	f.adminTrendCalls++
	return nil, nil
}

func (f *fakeAPIMetricsStore) GetAdminAPIMetricsStatusCodes(ctx context.Context, arg db.GetAdminAPIMetricsStatusCodesParams) ([]db.GetAdminAPIMetricsStatusCodesRow, error) {
	f.adminStatusCalls++
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

func TestAPIMetricsOverallRejectsRangeOverNinetyDaysBeforeV2Selection(t *testing.T) {
	legacy := &fakeAPIMetricsStore{}
	v2 := &fakeV2MetricsReader{}
	h := NewAPIMetricsHandler(legacy).WithObservabilityReads(fixedMetricsSelector(true), v2)
	req := httptest.NewRequest(http.MethodGet, "/v1/api-metrics/overall?from=2026-01-01T00:00:00Z&to=2026-04-02T00:00:01Z", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rr := httptest.NewRecorder()

	h.Overall(rr, req)

	if rr.Code != http.StatusBadRequest || legacy.overallCalls != 0 || v2.overallCalls != 0 {
		t.Fatalf("status/legacy/v2 = %d/%d/%d, want 400/0/0", rr.Code, legacy.overallCalls, v2.overallCalls)
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

func assertV2MetricsQuery(t *testing.T, q observabilityreads.MetricsQuery, workspaceID string, admin bool) {
	t.Helper()
	wantFrom := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)
	wantTo := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	if !q.From.Equal(wantFrom) || !q.To.Equal(wantTo) || q.WorkspaceID != workspaceID || q.ApplyMinCalls != admin || q.Method != "POST" || q.Path != "/v1/posts" || q.StatusClass != "4xx" || q.Sort != "p95_ms_desc" || q.Limit != 17 || q.MinCalls != 3 || q.Interval != "day" {
		t.Fatalf("v2 query=%#v", q)
	}
}

type metricsRouteContractCase struct {
	name        string
	path        string
	admin       bool
	invoke      func(*APIMetricsHandler, *AdminAPIMetricsHandler, *httptest.ResponseRecorder, *http.Request)
	v2Calls     func(*fakeV2MetricsReader) int
	legacyCalls func(*fakeAPIMetricsStore) int
	wantData    string
}

func metricsRouteContractCases() []metricsRouteContractCase {
	return []metricsRouteContractCase{
		{
			name: "workspace overall", path: "/v1/api-metrics/overall", wantData: `{"total_calls":9,"success_count":6,"client_error_count":1,"server_error_count":2,"rate_limited_count":1,"error_rate_pct":33.33,"server_failure_rate_pct":22.22,"reliability_pct":77.78,"p50_ms":10,"p95_ms":20,"p99_ms":30,"avg_ms":12}`,
			invoke: func(h *APIMetricsHandler, _ *AdminAPIMetricsHandler, w *httptest.ResponseRecorder, r *http.Request) {
				h.Overall(w, r)
			},
			v2Calls: func(f *fakeV2MetricsReader) int { return f.overallCalls }, legacyCalls: func(f *fakeAPIMetricsStore) int { return f.overallCalls },
		},
		{
			name: "workspace summary", path: "/v1/api-metrics/summary", wantData: `[{"path":"/v1/posts","method":"POST","total_calls":9,"success_count":6,"client_error_count":1,"server_error_count":2,"rate_limited_count":1,"error_rate_pct":33.33,"server_failure_rate_pct":22.22,"p50_ms":10,"p95_ms":20,"p99_ms":30,"avg_ms":12}]`,
			invoke: func(h *APIMetricsHandler, _ *AdminAPIMetricsHandler, w *httptest.ResponseRecorder, r *http.Request) {
				h.Summary(w, r)
			},
			v2Calls: func(f *fakeV2MetricsReader) int { return f.summaryCalls }, legacyCalls: func(f *fakeAPIMetricsStore) int { return f.summaryCalls },
		},
		{
			name: "workspace trend", path: "/v1/api-metrics/trend", wantData: `[{"bucket":"2026-07-29T12:00:00Z","total_calls":9,"success_count":6,"error_count":3,"client_error_count":1,"server_error_count":2,"rate_limited_count":1,"p50_ms":10,"p95_ms":20,"p99_ms":30,"avg_ms":12}]`,
			invoke: func(h *APIMetricsHandler, _ *AdminAPIMetricsHandler, w *httptest.ResponseRecorder, r *http.Request) {
				h.Trend(w, r)
			},
			v2Calls: func(f *fakeV2MetricsReader) int { return f.trendCalls }, legacyCalls: func(f *fakeAPIMetricsStore) int { return f.trendDailyCalls },
		},
		{
			name: "workspace status", path: "/v1/api-metrics/status-codes", wantData: `[{"status_code":429,"method":"POST","path":"/v1/posts","total_calls":1}]`,
			invoke: func(h *APIMetricsHandler, _ *AdminAPIMetricsHandler, w *httptest.ResponseRecorder, r *http.Request) {
				h.StatusCodes(w, r)
			},
			v2Calls: func(f *fakeV2MetricsReader) int { return f.statusCalls }, legacyCalls: func(f *fakeAPIMetricsStore) int { return f.statusCodeCalls },
		},
		{
			name: "admin overall", path: "/v1/admin/api-metrics/overall", admin: true, wantData: `{"total_calls":9,"success_count":6,"client_error_count":1,"server_error_count":2,"rate_limited_count":1,"error_rate_pct":33.33,"server_failure_rate_pct":22.22,"reliability_pct":77.78,"p50_ms":10,"p95_ms":20,"p99_ms":30,"avg_ms":12}`,
			invoke: func(_ *APIMetricsHandler, h *AdminAPIMetricsHandler, w *httptest.ResponseRecorder, r *http.Request) {
				h.Overall(w, r)
			},
			v2Calls: func(f *fakeV2MetricsReader) int { return f.overallCalls }, legacyCalls: func(f *fakeAPIMetricsStore) int { return f.adminOverallCalls },
		},
		{
			name: "admin summary", path: "/v1/admin/api-metrics/summary", admin: true, wantData: `[{"path":"/v1/posts","method":"POST","total_calls":9,"success_count":6,"client_error_count":1,"server_error_count":2,"rate_limited_count":1,"error_rate_pct":33.33,"server_failure_rate_pct":22.22,"p50_ms":10,"p95_ms":20,"p99_ms":30,"avg_ms":12}]`,
			invoke: func(_ *APIMetricsHandler, h *AdminAPIMetricsHandler, w *httptest.ResponseRecorder, r *http.Request) {
				h.Summary(w, r)
			},
			v2Calls: func(f *fakeV2MetricsReader) int { return f.summaryCalls }, legacyCalls: func(f *fakeAPIMetricsStore) int { return f.adminSummaryCalls },
		},
		{
			name: "admin trend", path: "/v1/admin/api-metrics/trend", admin: true, wantData: `[{"bucket":"2026-07-29T12:00:00Z","total_calls":9,"success_count":6,"error_count":3,"client_error_count":1,"server_error_count":2,"rate_limited_count":1,"p50_ms":10,"p95_ms":20,"p99_ms":30,"avg_ms":12}]`,
			invoke: func(_ *APIMetricsHandler, h *AdminAPIMetricsHandler, w *httptest.ResponseRecorder, r *http.Request) {
				h.Trend(w, r)
			},
			v2Calls: func(f *fakeV2MetricsReader) int { return f.trendCalls }, legacyCalls: func(f *fakeAPIMetricsStore) int { return f.adminTrendCalls },
		},
		{
			name: "admin status", path: "/v1/admin/api-metrics/status-codes", admin: true, wantData: `[{"status_code":429,"method":"POST","path":"/v1/posts","total_calls":1}]`,
			invoke: func(_ *APIMetricsHandler, h *AdminAPIMetricsHandler, w *httptest.ResponseRecorder, r *http.Request) {
				h.StatusCodes(w, r)
			},
			v2Calls: func(f *fakeV2MetricsReader) int { return f.statusCalls }, legacyCalls: func(f *fakeAPIMetricsStore) int { return f.adminStatusCalls },
		},
		{
			name: "admin workspaces", path: "/v1/admin/api-metrics/workspaces", admin: true, wantData: `[{"workspace_id":"ws_data","workspace_name":"Data Workspace","total_calls":9,"rate_limited_count":1,"error_rate_pct":33.33,"server_failure_rate_pct":22.22,"p95_ms":20,"p99_ms":30,"slowest_endpoint":"/v1/posts","slowest_endpoint_p95_ms":25}]`,
			invoke: func(_ *APIMetricsHandler, h *AdminAPIMetricsHandler, w *httptest.ResponseRecorder, r *http.Request) {
				h.Workspaces(w, r)
			},
			v2Calls: func(f *fakeV2MetricsReader) int { return f.workspaceCalls }, legacyCalls: func(f *fakeAPIMetricsStore) int { return f.adminWorkspacesCalls },
		},
	}
}

func newMetricsContractReader() *fakeV2MetricsReader {
	return &fakeV2MetricsReader{
		overall:    observabilityreads.MetricsOverall{TotalCalls: 9, SuccessCount: 6, ClientErrorCount: 1, ServerErrorCount: 2, RateLimitedCount: 1, ErrorRatePct: 33.33, ServerFailureRatePct: 22.22, ReliabilityPct: 77.78, P50MS: 10, P95MS: 20, P99MS: 30, AvgMS: 12},
		summaries:  []observabilityreads.MetricsSummaryRow{{Path: "/v1/posts", Method: "POST", TotalCalls: 9, SuccessCount: 6, ClientErrorCount: 1, ServerErrorCount: 2, RateLimitedCount: 1, ErrorRatePct: 33.33, ServerFailureRatePct: 22.22, P50MS: 10, P95MS: 20, P99MS: 30, AvgMS: 12}},
		trends:     []observabilityreads.MetricsTrendRow{{Bucket: time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC), TotalCalls: 9, SuccessCount: 6, ErrorCount: 3, ClientErrorCount: 1, ServerErrorCount: 2, RateLimitedCount: 1, P50MS: 10, P95MS: 20, P99MS: 30, AvgMS: 12}},
		statuses:   []observabilityreads.MetricsStatusCodeRow{{StatusCode: 429, Method: "POST", Path: "/v1/posts", TotalCalls: 1}},
		workspaces: []observabilityreads.AdminMetricsWorkspaceRow{{WorkspaceID: "ws_data", WorkspaceName: "Data Workspace", TotalCalls: 9, RateLimitedCount: 1, ErrorRatePct: 33.33, ServerFailureRatePct: 22.22, P95MS: 20, P99MS: 30, SlowestEndpoint: "/v1/posts", SlowestEndpointP95MS: 25}},
		freshness:  observabilityreads.MetricsFreshness{DataState: observabilityreads.MetricsDataDelayed, Approximate: true, MissingRollupHours: 2},
	}
}

func TestAPIMetricsV2RouteContracts(t *testing.T) {
	const query = "?from=2026-07-28T00:00:00Z&to=2026-07-29T00:00:00Z&workspace_id=ws_admin&method=POST&path=/v1/posts&status_class=4xx&sort=p95_ms_desc&limit=17&min_calls=3&interval=day"
	for _, test := range metricsRouteContractCases() {
		t.Run(test.name, func(t *testing.T) {
			legacy := &fakeAPIMetricsStore{}
			v2 := newMetricsContractReader()
			workspace := NewAPIMetricsHandler(legacy).WithObservabilityReads(fixedMetricsSelector(true), v2)
			admin := NewAdminAPIMetricsHandler(legacy).WithObservabilityReads(fixedMetricsSelector(true), v2)
			req := httptest.NewRequest(http.MethodGet, test.path+query, nil)
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_contract"))
			rr := httptest.NewRecorder()
			rr.Header().Set("X-Request-Id", "metrics-contract-request")

			test.invoke(workspace, admin, rr, req)

			if rr.Code != http.StatusOK || test.v2Calls(v2) != 1 || totalV2MetricCalls(v2) != 1 || totalLegacyMetricCalls(legacy) != 0 {
				t.Fatalf("status/v2 route/v2 total/legacy total = %d/%d/%d/%d, want 200/1/1/0: %s", rr.Code, test.v2Calls(v2), totalV2MetricCalls(v2), totalLegacyMetricCalls(legacy), rr.Body.String())
			}
			workspaceID := "ws_contract"
			if test.admin {
				workspaceID = "ws_admin"
			}
			assertV2MetricsQuery(t, v2.lastQuery, workspaceID, test.admin)
			var response struct {
				Data      json.RawMessage                     `json:"data"`
				Meta      *MetaResponse                       `json:"meta"`
				Freshness observabilityreads.MetricsFreshness `json:"freshness"`
				RequestID string                              `json:"request_id"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if response.Meta != nil || response.Freshness != v2.freshness || response.RequestID != "metrics-contract-request" {
				t.Fatalf("meta/freshness/request_id = %#v/%#v/%q, want nil/%#v/metrics-contract-request", response.Meta, response.Freshness, response.RequestID, v2.freshness)
			}
			assertJSONContract(t, response.Data, test.wantData)
		})
	}
}

func TestAPIMetricsV2OffFallsBackForEveryRoute(t *testing.T) {
	const query = "?from=2026-07-28T00:00:00Z&to=2026-07-29T00:00:00Z&workspace_id=ws_admin&method=POST&path=/v1/posts&status_class=4xx&sort=p95_ms_desc&limit=17&min_calls=3&interval=day"
	for _, test := range metricsRouteContractCases() {
		t.Run(test.name, func(t *testing.T) {
			legacy := &fakeAPIMetricsStore{}
			v2 := newMetricsContractReader()
			workspace := NewAPIMetricsHandler(legacy).WithObservabilityReads(fixedMetricsSelector(false), v2)
			admin := NewAdminAPIMetricsHandler(legacy).WithObservabilityReads(fixedMetricsSelector(false), v2)
			req := httptest.NewRequest(http.MethodGet, test.path+query, nil)
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_contract"))
			rr := httptest.NewRecorder()

			test.invoke(workspace, admin, rr, req)

			if rr.Code != http.StatusOK || test.legacyCalls(legacy) != 1 || totalLegacyMetricCalls(legacy) != 1 || totalV2MetricCalls(v2) != 0 {
				t.Fatalf("status/legacy route/legacy total/v2 total = %d/%d/%d/%d, want 200/1/1/0: %s", rr.Code, test.legacyCalls(legacy), totalLegacyMetricCalls(legacy), totalV2MetricCalls(v2), rr.Body.String())
			}
		})
	}
}

func TestSelectMetricsReaderFallbackMatrix(t *testing.T) {
	var typedNilSelector *pointerMetricsSelector
	var typedNilReader *fakeV2MetricsReader
	errorSelector := observabilityreads.NewReadSelector(failingMetricsPublicEvaluator{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, test := range []struct {
		name     string
		selector observabilityreads.ReadPathSelector
		reader   observabilityreads.MetricsReader
	}{
		{name: "off", selector: fixedMetricsSelector(false), reader: &fakeV2MetricsReader{}},
		{name: "nil selector", reader: &fakeV2MetricsReader{}},
		{name: "nil reader", selector: fixedMetricsSelector(true)},
		{name: "typed nil selector", selector: typedNilSelector, reader: &fakeV2MetricsReader{}},
		{name: "typed nil reader", selector: fixedMetricsSelector(true), reader: typedNilReader},
		{name: "evaluator error", selector: errorSelector, reader: &fakeV2MetricsReader{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := observabilityreads.SelectMetricsReader(context.Background(), test.selector, test.reader); got != nil {
				t.Fatalf("SelectMetricsReader = %#v, want nil legacy fallback", got)
			}
		})
	}
}

func totalV2MetricCalls(f *fakeV2MetricsReader) int {
	return f.overallCalls + f.summaryCalls + f.trendCalls + f.statusCalls + f.workspaceCalls
}

func totalLegacyMetricCalls(f *fakeAPIMetricsStore) int {
	return f.overallCalls + f.summaryCalls + f.trendHourlyCalls + f.trendDailyCalls + f.statusCodeCalls + f.adminOverallCalls + f.adminSummaryCalls + f.adminTrendCalls + f.adminStatusCalls + f.adminWorkspacesCalls
}

func assertJSONContract(t *testing.T, got json.RawMessage, want string) {
	t.Helper()
	var gotValue, wantValue any
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("unmarshal actual data: %v", err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("unmarshal expected data: %v", err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("data contract = %#v, want %#v", gotValue, wantValue)
	}
}
