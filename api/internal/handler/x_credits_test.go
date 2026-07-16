package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
)

type fakeXCreditsSnapshotService struct {
	snapshot xcredits.Snapshot
	err      error
}

func (f fakeXCreditsSnapshotService) Snapshot(context.Context, string, time.Time) (xcredits.Snapshot, error) {
	return f.snapshot, f.err
}

func xCreditsInt64(value int64) *int64 {
	return &value
}

func TestGetXCreditsReturnsMonthlyAllowance(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	h := (&BillingHandler{}).SetXCreditsService(fakeXCreditsSnapshotService{
		snapshot: xcredits.Snapshot{
			PlanID:            "basic",
			PeriodStart:       start,
			PeriodEnd:         end,
			MonthlyAllowance:  xCreditsInt64(4000),
			MonthlyUsed:       215,
			MonthlyRemaining:  xCreditsInt64(3785),
			InboundDailyUsed:  25,
			InboundDailyLimit: xCreditsInt64(400),
			CatalogVersion:    xcredits.CatalogVersion,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/billing/x-credits", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()
	rec.Header().Set("X-Request-Id", "req_x_credits")

	h.GetXCredits(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Mode               string `json:"mode"`
			MonthlyAllowance   *int64 `json:"monthly_allowance"`
			MonthlyUsed        int64  `json:"monthly_used"`
			MonthlyRemaining   *int64 `json:"monthly_remaining"`
			InboundDailyUsage  int64  `json:"inbound_daily_usage"`
			InboundDailyLimit  *int64 `json:"inbound_daily_limit"`
			ConnectionModeNote string `json:"connection_mode_note"`
			CatalogVersion     string `json:"catalog_version"`
		} `json:"data"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Mode != "monthly_allowance" || body.Data.MonthlyAllowance == nil || *body.Data.MonthlyAllowance != 4000 {
		t.Fatalf("data = %+v", body.Data)
	}
	if body.Data.MonthlyUsed != 215 || body.Data.MonthlyRemaining == nil || *body.Data.MonthlyRemaining != 3785 {
		t.Fatalf("data = %+v", body.Data)
	}
	if body.Data.InboundDailyUsage != 25 || body.Data.InboundDailyLimit == nil || *body.Data.InboundDailyLimit != 400 {
		t.Fatalf("data = %+v", body.Data)
	}
	if body.Data.CatalogVersion != xcredits.CatalogVersion || body.Data.ConnectionModeNote == "" {
		t.Fatalf("data = %+v", body.Data)
	}
	if body.RequestID != "req_x_credits" {
		t.Fatalf("request_id = %q", body.RequestID)
	}
}

func TestGetXCreditsEnterpriseUsesCustomNullLimits(t *testing.T) {
	h := (&BillingHandler{}).SetXCreditsService(fakeXCreditsSnapshotService{
		snapshot: xcredits.Snapshot{
			PlanID:         "enterprise",
			PeriodStart:    time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			PeriodEnd:      time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			MonthlyUsed:    500,
			CatalogVersion: xcredits.CatalogVersion,
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/x-credits", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_enterprise"))
	rec := httptest.NewRecorder()

	h.GetXCredits(rec, req)

	var body struct {
		Data struct {
			MonthlyAllowance  *int64 `json:"monthly_allowance"`
			MonthlyRemaining  *int64 `json:"monthly_remaining"`
			InboundDailyLimit *int64 `json:"inbound_daily_limit"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.MonthlyAllowance != nil || body.Data.MonthlyRemaining != nil || body.Data.InboundDailyLimit != nil {
		t.Fatalf("enterprise limits must be null: %+v", body.Data)
	}
}

func TestListPlansEnterpriseSerialization(t *testing.T) {
	enterprise := planResponseFromDB(db.Plan{
		ID:         "enterprise",
		Name:       "Enterprise",
		PriceCents: 0,
		PostLimit:  -1,
	})
	if enterprise.PriceCents != nil || enterprise.PricingModel != "custom" {
		t.Fatalf("enterprise = %+v", enterprise)
	}

	basic := planResponseFromDB(db.Plan{
		ID:         "basic",
		Name:       "Basic",
		PriceCents: 1900,
		PostLimit:  2500,
	})
	if basic.PriceCents == nil || *basic.PriceCents != 1900 || basic.PricingModel != "fixed" {
		t.Fatalf("basic = %+v", basic)
	}
}
