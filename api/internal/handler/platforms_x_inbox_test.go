package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestXAccountCapabilityKeepsPublishingActiveWhenDMScopesNeedReconnect(t *testing.T) {
	store := &xCapabilityTestDB{
		planID:  "basic",
		appMode: "unipost_managed_app",
		scopes:  []string{"tweet.read", "tweet.write", "users.read", "offline.access"},
	}
	rec := invokeXAccountCapabilities(t, store)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Capability struct {
				FirstComment struct {
					Supported bool `json:"supported"`
				} `json:"first_comment"`
			} `json:"capability"`
			XInbox struct {
				CommentsEnabled   bool     `json:"comments_enabled"`
				DMsEnabled        bool     `json:"dms_enabled"`
				MissingScopes     []string `json:"missing_scopes"`
				ReconnectRequired bool     `json:"reconnect_required"`
				DeliveryStatus    string   `json:"delivery_status"`
				AppMode           string   `json:"app_mode"`
			} `json:"x_inbox"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Data.Capability.FirstComment.Supported {
		t.Fatal("existing X publish/reply capability was disabled by missing DM scopes")
	}
	if !body.Data.XInbox.CommentsEnabled || body.Data.XInbox.DMsEnabled {
		t.Fatalf("x_inbox = %+v", body.Data.XInbox)
	}
	if want := []string{"dm.read", "dm.write"}; !reflect.DeepEqual(body.Data.XInbox.MissingScopes, want) {
		t.Fatalf("missing scopes = %v, want %v", body.Data.XInbox.MissingScopes, want)
	}
	if !body.Data.XInbox.ReconnectRequired {
		t.Fatal("reconnect_required = false, want true")
	}
	if body.Data.XInbox.DeliveryStatus != "pending" || body.Data.XInbox.AppMode != "unipost_managed_app" {
		t.Fatalf("x_inbox = %+v", body.Data.XInbox)
	}
}

func TestXAccountCapabilityAPIPlanDoesNotPromptReconnect(t *testing.T) {
	store := &xCapabilityTestDB{
		planID:  "api",
		appMode: "unipost_managed_app",
		scopes:  []string{"tweet.read", "tweet.write", "users.read"},
	}
	rec := invokeXAccountCapabilities(t, store)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			XInbox struct {
				CommentsEnabled   bool   `json:"comments_enabled"`
				DMsEnabled        bool   `json:"dms_enabled"`
				ReconnectRequired bool   `json:"reconnect_required"`
				DeliveryStatus    string `json:"delivery_status"`
			} `json:"x_inbox"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.XInbox.CommentsEnabled || body.Data.XInbox.DMsEnabled || body.Data.XInbox.ReconnectRequired {
		t.Fatalf("x_inbox = %+v, want disabled with no prompt", body.Data.XInbox)
	}
	if body.Data.XInbox.DeliveryStatus != "paused_plan" {
		t.Fatalf("delivery_status = %q, want paused_plan", body.Data.XInbox.DeliveryStatus)
	}
}

func TestXAccountCapabilityRejectsInvalidPersistedAppMode(t *testing.T) {
	store := &xCapabilityTestDB{
		planID:  "basic",
		appMode: "garbage",
		scopes:  []string{"tweet.read", "tweet.write", "users.read", "dm.read", "dm.write"},
	}
	rec := invokeXAccountCapabilities(t, store)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body = %s, want 500", rec.Code, rec.Body.String())
	}
}

func invokeXAccountCapabilities(t *testing.T, store *xCapabilityTestDB) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/sa_1/capabilities", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "sa_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	NewPlatformHandler(db.New(store)).GetAccountCapabilities(rec, req)
	return rec
}

type xCapabilityTestDB struct {
	planID  string
	appMode string
	scopes  []string
}

func (f *xCapabilityTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *xCapabilityTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, pgx.ErrNoRows
}

func (f *xCapabilityTestDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSocialAccountByIDAndWorkspace"):
		return xCapabilityAccountRow{scopes: f.scopes, appMode: f.appMode}
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return scanRow{values: []any{
			"sub_1", f.planID, pgtype.Text{}, pgtype.Text{}, "active",
			now, now, pgtype.Bool{}, now, now, false, "ws_1",
		}}
	case strings.Contains(query, "-- name: GetPlan"):
		allowInbox := f.planID != "free" && f.planID != "api"
		return scanRow{values: []any{
			f.planID, f.planID, int32(1000), int32(1000), pgtype.Text{},
			pgtype.Timestamptz{Time: time.Now(), Valid: true}, true, true,
			allowInbox, true, pgtype.Int4{}, pgtype.Int4{},
		}}
	case strings.Contains(query, "-- name: GetXInboxDeliveryResource"),
		strings.Contains(query, "-- name: GetPlatformCredential"):
		return scanRow{err: pgx.ErrNoRows}
	default:
		return scanRow{err: fmt.Errorf("unexpected query: %s", query)}
	}
}

type xCapabilityAccountRow struct {
	scopes  []string
	appMode string
}

func (r xCapabilityAccountRow) Scan(dest ...any) error {
	values := []any{
		"sa_1", "pr_1", "twitter", "encrypted-access", pgtype.Text{},
		pgtype.Timestamptz{}, "x-user-1", pgtype.Text{String: "UniPost", Valid: true},
		pgtype.Text{}, pgtype.Timestamptz{Time: time.Now(), Valid: true},
		pgtype.Timestamptz{}, []byte(`{}`), r.scopes, "active", "byo",
		pgtype.Text{}, pgtype.Text{}, pgtype.Text{}, pgtype.Timestamptz{},
	}
	if len(dest) == len(values)+1 {
		values = append(values, pgtype.Text{String: r.appMode, Valid: r.appMode != ""})
	}
	return scanRow{values: values}.Scan(dest...)
}
