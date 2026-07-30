package requesteventpartitions

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type stubInspector struct {
	inspection Inspection
	err        error
	minimum    int
}

func (s *stubInspector) Inspect(_ context.Context, _ time.Time, minimum int) (Inspection, error) {
	s.minimum = minimum
	return s.inspection, s.err
}

func TestStatusHandlerReturnsInspectionAndWorkerStats(t *testing.T) {
	inspector := &stubInspector{inspection: Inspection{
		InspectedAt:       time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC),
		CoverageDays:      60,
		EventDefaultRows:  0,
		DetailDefaultRows: 0,
		Ready:             true,
		Reasons:           []string{},
	}}
	worker := &Worker{stats: WorkerStats{FailureCount: 0}}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/admin/observability/request-event-partitions", nil)

	StatusHandler(inspector, worker).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if inspector.minimum != MinimumCoverageDays {
		t.Fatalf("minimum coverage = %d, want %d", inspector.minimum, MinimumCoverageDays)
	}
	var response struct {
		Data struct {
			Inspection Inspection  `json:"inspection"`
			Worker     WorkerStats `json:"worker"`
		} `json:"data"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if !response.Data.Inspection.Ready || response.Data.Inspection.CoverageDays != 60 {
		t.Fatalf("inspection = %#v", response.Data.Inspection)
	}
	if response.Data.Worker.FailureCount != 0 {
		t.Fatalf("worker = %#v", response.Data.Worker)
	}
}

func TestStatusHandlerReturnsServiceUnavailableWhenInspectionFails(t *testing.T) {
	inspector := &stubInspector{err: errors.New("database timeout")}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/admin/observability/request-event-partitions", nil)

	StatusHandler(inspector, &Worker{}).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", recorder.Code)
	}
	if recorder.Body.String() != "request-event partition inspection is unavailable\n" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}
