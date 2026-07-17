package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestFeatureFlagsCompatReturnsEmptyFlagsAndPlanGates(t *testing.T) {
	t.Setenv("UNIPOST_ENV", "production")

	h := NewMeHandler(db.New(meFeaturesCompatDB{}), nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/me/features", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_1"))
	rec := httptest.NewRecorder()

	h.FeatureFlagsCompat(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Data struct {
			Environment string          `json:"environment"`
			Provider    string          `json:"provider"`
			Flags       map[string]bool `json:"flags"`
			PlanGates   map[string]bool `json:"plan_gates"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Data.Environment != "production" {
		t.Fatalf("environment = %q, want production", body.Data.Environment)
	}
	if body.Data.Provider != "removed" {
		t.Fatalf("provider = %q, want removed", body.Data.Provider)
	}
	if body.Data.Flags == nil {
		t.Fatalf("flags map should be present")
	}
	if len(body.Data.Flags) != 0 {
		t.Fatalf("flags = %#v, want empty map", body.Data.Flags)
	}
	if got := body.Data.PlanGates["inbox"]; got {
		t.Fatalf("plan_gates.inbox = true, want false without workspace")
	}
}

func TestPlanGatesExposeTeamOnlyAuditLog(t *testing.T) {
	for _, tt := range []struct {
		planID       string
		wantAuditLog bool
	}{
		{planID: "team", wantAuditLog: true},
		{planID: "growth", wantAuditLog: false},
	} {
		t.Run(tt.planID, func(t *testing.T) {
			store := meFeaturesCompatDB{planID: tt.planID}
			queries := db.New(store)
			h := NewMeHandler(queries, nil, nil).SetQuotaChecker(quota.NewChecker(queries))
			req := httptest.NewRequest(http.MethodGet, "/v1/me/plan-gates", nil)
			req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_1"))
			rec := httptest.NewRecorder()

			h.PlanGates(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			var body struct {
				Data struct {
					PlanGates map[string]bool `json:"plan_gates"`
				} `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if got := body.Data.PlanGates["audit_log"]; got != tt.wantAuditLog {
				t.Fatalf("plan_gates.audit_log = %v, want %v; body=%s", got, tt.wantAuditLog, rec.Body.String())
			}
		})
	}
}

type meFeaturesCompatDB struct {
	planID string
}

func (meFeaturesCompatDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (meFeaturesCompatDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return meFeaturesCompatRows{}, nil
}

func (f meFeaturesCompatDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	if f.planID == "" {
		return meFeaturesCompatRow{err: pgx.ErrNoRows}
	}
	switch {
	case strings.Contains(query, "-- name: GetActiveMembership"):
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return scanRow{values: []any{
			"ws_1",
			"user_1",
			auth.RoleOwner,
			"active",
			pgtype.Text{},
			now,
			now,
			now,
			now,
		}}
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		return subscriptionScanRow(f.planID)
	case strings.Contains(query, "-- name: GetPlan"):
		return planScanRow(&freePlanLimitsTestDB{
			planID:            f.planID,
			allowInbox:        f.planID != "free" && f.planID != "api",
			allowAnalytics:    f.planID != "free",
			allowTwitter:      f.planID != "free",
			whiteLabelAllowed: f.planID != "free" && f.planID != "api",
		})
	default:
		return meFeaturesCompatRow{err: pgx.ErrNoRows}
	}
}

type meFeaturesCompatRow struct {
	err error
}

func (r meFeaturesCompatRow) Scan(...interface{}) error {
	return r.err
}

type meFeaturesCompatRows struct{}

func (meFeaturesCompatRows) Close()                                       {}
func (meFeaturesCompatRows) Err() error                                   { return nil }
func (meFeaturesCompatRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (meFeaturesCompatRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (meFeaturesCompatRows) Next() bool                                   { return false }
func (meFeaturesCompatRows) Scan(...interface{}) error                    { return nil }
func (meFeaturesCompatRows) Values() ([]interface{}, error)               { return nil, nil }
func (meFeaturesCompatRows) RawValues() [][]byte                          { return nil }
func (meFeaturesCompatRows) Conn() *pgx.Conn                              { return nil }
