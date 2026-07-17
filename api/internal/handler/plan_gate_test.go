package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestRequirePlanAuditLogAllowsTeam(t *testing.T) {
	store := &freePlanLimitsTestDB{planID: "team"}
	gate := RequirePlanAuditLog(quota.NewChecker(db.New(store)))
	calls := 0
	handler := gate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/v1/audit-log", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent || calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s, want 204/1", rec.Code, calls, rec.Body.String())
	}
}

func TestRequirePlanAuditLogBlocksNonTeamPlans(t *testing.T) {
	for _, planID := range []string{"free", "api", "basic", "growth"} {
		t.Run(planID, func(t *testing.T) {
			store := &freePlanLimitsTestDB{planID: planID}
			gate := RequirePlanAuditLog(quota.NewChecker(db.New(store)))
			calls := 0
			handler := gate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
			req := httptest.NewRequest(http.MethodGet, "/v1/audit-log", nil)
			req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusPaymentRequired {
				t.Fatalf("status=%d body=%s, want 402", rec.Code, rec.Body.String())
			}
			if calls != 0 {
				t.Fatalf("downstream calls=%d, want 0", calls)
			}
			if !strings.Contains(rec.Body.String(), `"code":"PLAN_FEATURE_NOT_AVAILABLE"`) {
				t.Fatalf("body=%s, want PLAN_FEATURE_NOT_AVAILABLE", rec.Body.String())
			}
		})
	}
}

func TestRequirePlanAuditLogRejectsMissingWorkspaceContext(t *testing.T) {
	gate := RequirePlanAuditLog(nil)
	calls := 0
	handler := gate(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/audit-log", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s, want 500", rec.Code, rec.Body.String())
	}
	if calls != 0 {
		t.Fatalf("downstream calls=%d, want 0", calls)
	}
}
