package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
)

type fakeXCreditsSnapshotService struct {
	snapshot xcredits.Snapshot
	err      error
	setting  xcredits.InboundCapSetting
	update   xcredits.UpdateInboundCapRequest
}

func (f fakeXCreditsSnapshotService) Snapshot(context.Context, string, time.Time) (xcredits.Snapshot, error) {
	return f.snapshot, f.err
}

func (f *fakeXCreditsSnapshotService) UpdateInboundCap(_ context.Context, req xcredits.UpdateInboundCapRequest) (xcredits.InboundCapSetting, error) {
	f.update = req
	return f.setting, f.err
}

func xCreditsInt64(value int64) *int64 {
	return &value
}

func TestGetXCreditsReturnsMonthlyAllowance(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	h := (&BillingHandler{}).SetXCreditsService(&fakeXCreditsSnapshotService{
		snapshot: xcredits.Snapshot{
			PlanID:             "basic",
			PeriodStart:        start,
			PeriodEnd:          end,
			MonthlyAllowance:   xCreditsInt64(4000),
			MonthlyUsed:        215,
			MonthlyRemaining:   xCreditsInt64(3785),
			InboundDailyUsed:   25,
			InboundDailyLimit:  xCreditsInt64(400),
			InboundAccepted:    4,
			InboundSuppressed:  2,
			InboundResetAt:     time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC),
			InboundPercent:     6.25,
			PausePaidSources:   true,
			InboundPauseReason: xcredits.PauseReasonDailySafetyBuffer,
			CatalogVersion:     xcredits.CatalogVersion,
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
			Mode               string    `json:"mode"`
			MonthlyAllowance   *int64    `json:"monthly_allowance"`
			MonthlyUsed        int64     `json:"monthly_used"`
			MonthlyRemaining   *int64    `json:"monthly_remaining"`
			InboundDailyUsage  int64     `json:"inbound_daily_usage"`
			InboundDailyLimit  *int64    `json:"inbound_daily_limit"`
			InboundAccepted    int64     `json:"inbound_events_accepted"`
			InboundSuppressed  int64     `json:"inbound_events_suppressed"`
			InboundResetAt     time.Time `json:"inbound_daily_reset_at"`
			InboundPercent     float64   `json:"inbound_daily_percent"`
			PausePaidSources   bool      `json:"pause_paid_sources"`
			InboundPauseReason string    `json:"inbound_pause_reason"`
			ConnectionModeNote string    `json:"connection_mode_note"`
			CatalogVersion     string    `json:"catalog_version"`
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
	if body.Data.InboundAccepted != 4 || body.Data.InboundSuppressed != 2 ||
		!body.Data.InboundResetAt.Equal(time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)) ||
		body.Data.InboundPercent != 6.25 || !body.Data.PausePaidSources ||
		body.Data.InboundPauseReason != xcredits.PauseReasonDailySafetyBuffer {
		t.Fatalf("inbound metrics = %+v", body.Data)
	}
	if body.Data.CatalogVersion != xcredits.CatalogVersion || body.Data.ConnectionModeNote == "" {
		t.Fatalf("data = %+v", body.Data)
	}
	if body.RequestID != "req_x_credits" {
		t.Fatalf("request_id = %q", body.RequestID)
	}
}

func TestGetXCreditsFeatureOffReturnsUnavailableWithoutLoadingBalance(t *testing.T) {
	service := &fakeXCreditsSnapshotService{}
	h := (&BillingHandler{}).
		SetXCreditsService(service).
		SetFeatureFlags(platformFeatureFlags(false))
	req := httptest.NewRequest(http.MethodGet, "/v1/billing/x-credits", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.GetXCredits(rec, req)

	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "FEATURE_NOT_AVAILABLE") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetXCreditsEnterpriseUsesCustomNullLimits(t *testing.T) {
	h := (&BillingHandler{}).SetXCreditsService(&fakeXCreditsSnapshotService{
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

func TestPatchXInboundCapPassesAuthenticatedActorAndAcknowledgement(t *testing.T) {
	service := &fakeXCreditsSnapshotService{
		setting: xcredits.InboundCapSetting{
			InboundDailyLimit:    500,
			UpdatedBy:            "user_admin",
			AcknowledgedExposure: true,
			UpdatedAt:            time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC),
		},
	}
	h := (&BillingHandler{}).SetXCreditsService(service)
	req := httptest.NewRequest(http.MethodPatch, "/v1/billing/x-credits/inbound-cap",
		strings.NewReader(`{"inbound_daily_limit":500,"acknowledged_exposure":true}`))
	ctx := auth.SetWorkspaceID(req.Context(), "ws_1")
	ctx = context.WithValue(ctx, auth.UserIDKey, "user_admin")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	h.UpdateXInboundCap(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if service.update.WorkspaceID != "ws_1" || service.update.UpdatedBy != "user_admin" ||
		service.update.InboundDailyLimit != 500 || !service.update.AcknowledgedExposure {
		t.Fatalf("update = %+v", service.update)
	}
}

func TestPatchXInboundCapRejectsNegativeLimit(t *testing.T) {
	service := &fakeXCreditsSnapshotService{}
	h := (&BillingHandler{}).SetXCreditsService(service)
	req := httptest.NewRequest(http.MethodPatch, "/v1/billing/x-credits/inbound-cap",
		strings.NewReader(`{"inbound_daily_limit":-1}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.UpdateXInboundCap(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPatchXInboundCapRequiresExplicitLimit(t *testing.T) {
	service := &fakeXCreditsSnapshotService{}
	h := (&BillingHandler{}).SetXCreditsService(service)
	req := httptest.NewRequest(http.MethodPatch, "/v1/billing/x-credits/inbound-cap",
		strings.NewReader(`{"acknowledged_exposure":true}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.UpdateXInboundCap(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
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
