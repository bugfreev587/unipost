package handler

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/xiaoboyu/unipost-api/internal/featureflags"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestCreateScheduledPostReturnsQuotaExceededWhenFreePlanCapIncludesReservations(t *testing.T) {
	featureflags.SetProvider(featureflags.EnvProvider{})
	t.Cleanup(func() { featureflags.SetProvider(featureflags.EnvProvider{}) })
	t.Setenv("FEATURE_BILLING_FREE_PLAN_HARD_POST_QUOTA", "true")

	dbtx := &scheduledQuotaHTTPTestDB{}
	handler := NewSocialPostHandler(db.New(dbtx), nil, quota.NewChecker(db.New(dbtx)), nil, nil, nil, nil)
	body := strings.NewReader(`{
		"scheduled_at": "2026-06-25T13:00:00Z",
		"platform_posts": [
			{
				"account_id": "acct_linkedin",
				"caption": "Codex regression coverage for free quota hard cap."
			}
		]
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/posts", body)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rr := httptest.NewRecorder()

	handler.Create(rr, req)

	if rr.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402; body: %s", rr.Code, rr.Body.String())
	}
	if dbtx.createSocialPostCalls != 0 {
		t.Fatalf("CreateSocialPost calls = %d, want 0", dbtx.createSocialPostCalls)
	}
	if got := rr.Header().Get("X-UniPost-Usage"); got != "122/100" {
		t.Fatalf("X-UniPost-Usage = %q, want 122/100", got)
	}
	if got := rr.Header().Get("X-UniPost-Warning"); got != "over_limit" {
		t.Fatalf("X-UniPost-Warning = %q, want over_limit", got)
	}

	var envelope struct {
		Error struct {
			Code           string `json:"code"`
			NormalizedCode string `json:"normalized_code"`
			Message        string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Error.Code != "PLAN_POST_QUOTA_EXCEEDED" {
		t.Fatalf("error.code = %q", envelope.Error.Code)
	}
	if envelope.Error.NormalizedCode != "plan_post_quota_exceeded" {
		t.Fatalf("error.normalized_code = %q", envelope.Error.NormalizedCode)
	}
	for _, want := range []string{
		"122 of 100 posts",
		"1 scheduled post reserved",
		"needs 1 more",
	} {
		if !strings.Contains(envelope.Error.Message, want) {
			t.Fatalf("error.message = %q, want substring %q", envelope.Error.Message, want)
		}
	}
}

type scheduledQuotaHTTPTestDB struct {
	createSocialPostCalls int
}

func (f *scheduledQuotaHTTPTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *scheduledQuotaHTTPTestDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "-- name: ListSocialAccountsByWorkspace"):
		return &scheduledQuotaRows{values: [][]any{socialAccountValues(db.SocialAccount{
			ID:                "acct_linkedin",
			ProfileID:         "prof_1",
			Platform:          "linkedin",
			AccessToken:       "token",
			ExternalAccountID: "linkedin-page",
			AccountName:       pgtype.Text{String: "LinkedIn Page", Valid: true},
			ConnectedAt:       pgtype.Timestamptz{Time: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC), Valid: true},
			Status:            "connected",
		})}}, nil
	default:
		return nil, errors.New("unexpected Query: " + query)
	}
}

func (f *scheduledQuotaHTTPTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		return scanRow{values: []any{
			"sub_1",
			"free",
			pgtype.Text{},
			pgtype.Text{},
			"active",
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			pgtype.Bool{},
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			false,
			"ws_1",
		}}
	case strings.Contains(query, "-- name: GetPlan"):
		return scanRow{values: []any{
			"free",
			"Free",
			int32(0),
			int32(100),
			pgtype.Text{},
			pgtype.Timestamptz{},
			false,
			false,
			false,
			false,
			pgtype.Int4{},
			pgtype.Int4{},
		}}
	case strings.Contains(query, "-- name: GetUsage"):
		return scanRow{values: []any{
			"usage_1",
			args[1].(string),
			int32(122),
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			"ws_1",
		}}
	case strings.Contains(query, "-- name: GetScheduledSocialPostByIdempotencyKey"):
		return scanRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "FROM social_posts sp") && strings.Contains(query, "sp.status = 'scheduled'"):
		return scanRow{values: []any{int32(1)}}
	case strings.Contains(query, "-- name: CreateSocialPost"):
		f.createSocialPostCalls++
		return scanRow{err: errors.New("CreateSocialPost should not be called when free quota is exceeded")}
	default:
		return scanRow{err: errors.New("unexpected QueryRow: " + query)}
	}
}

type scheduledQuotaRows struct {
	values [][]any
	index  int
}

func (r *scheduledQuotaRows) Close()                                       {}
func (r *scheduledQuotaRows) Err() error                                   { return nil }
func (r *scheduledQuotaRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (r *scheduledQuotaRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *scheduledQuotaRows) Next() bool {
	if r.index >= len(r.values) {
		return false
	}
	r.index++
	return true
}
func (r *scheduledQuotaRows) Scan(dest ...any) error {
	if r.index == 0 || r.index > len(r.values) {
		return errors.New("Scan called without current row")
	}
	return scanRow{values: r.values[r.index-1]}.Scan(dest...)
}
func (r *scheduledQuotaRows) Values() ([]any, error) { return r.values[r.index-1], nil }
func (r *scheduledQuotaRows) RawValues() [][]byte    { return nil }
func (r *scheduledQuotaRows) Conn() *pgx.Conn        { return nil }

func socialAccountValues(a db.SocialAccount) []any {
	return []any{
		a.ID,
		a.ProfileID,
		a.Platform,
		a.AccessToken,
		a.RefreshToken,
		a.TokenExpiresAt,
		a.ExternalAccountID,
		a.AccountName,
		a.AccountAvatarUrl,
		a.ConnectedAt,
		a.DisconnectedAt,
		a.Metadata,
		a.Scope,
		a.Status,
		a.ConnectionType,
		a.ConnectSessionID,
		a.ExternalUserID,
		a.ExternalUserEmail,
		a.LastRefreshedAt,
	}
}
