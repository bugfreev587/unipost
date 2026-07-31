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

type platformFeatureFlags bool

func (f platformFeatureFlags) ForWorkspace(context.Context, string, string) (bool, error) {
	return bool(f), nil
}

func TestXAccountCapabilityFlagOffDoesNotRequestDMReconnect(t *testing.T) {
	store := &xCapabilityTestDB{
		planID:  "basic",
		appMode: "unipost_managed_app",
		scopes:  []string{"tweet.read", "tweet.write", "users.read", "offline.access"},
	}
	rec := invokeXAccountCapabilitiesWithFlags(t, store, platformFeatureFlags(false))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			XInbox struct {
				CommentsEnabled   bool     `json:"comments_enabled"`
				DMsEnabled        bool     `json:"dms_enabled"`
				MissingScopes     []string `json:"missing_scopes"`
				ReconnectRequired bool     `json:"reconnect_required"`
			} `json:"x_inbox"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Data.XInbox.CommentsEnabled || body.Data.XInbox.DMsEnabled ||
		body.Data.XInbox.ReconnectRequired || len(body.Data.XInbox.MissingScopes) != 0 {
		t.Fatalf("x_inbox = %+v, want comments with hidden DMs and no reconnect", body.Data.XInbox)
	}
}

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

func TestXAccountCapabilityNormalizesNullAppModeToLegacyReconnect(t *testing.T) {
	store := &xCapabilityTestDB{
		planID: "basic",
		scopes: []string{"tweet.read", "tweet.write", "users.read", "dm.read", "dm.write"},
	}
	rec := invokeXAccountCapabilities(t, store)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s, want 200", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			XInbox struct {
				CommentsEnabled   bool   `json:"comments_enabled"`
				DMsEnabled        bool   `json:"dms_enabled"`
				ReconnectRequired bool   `json:"reconnect_required"`
				AppMode           string `json:"app_mode"`
			} `json:"x_inbox"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.XInbox.AppMode != "legacy_unknown" ||
		body.Data.XInbox.CommentsEnabled ||
		body.Data.XInbox.DMsEnabled ||
		!body.Data.XInbox.ReconnectRequired {
		t.Fatalf("x_inbox = %+v, want normalized legacy reconnect state", body.Data.XInbox)
	}
}

func invokeXAccountCapabilities(t *testing.T, store *xCapabilityTestDB) *httptest.ResponseRecorder {
	return invokeXAccountCapabilitiesWithFlags(t, store, nil)
}

func invokeXAccountCapabilitiesWithFlags(
	t *testing.T,
	store *xCapabilityTestDB,
	flags interface {
		ForWorkspace(context.Context, string, string) (bool, error)
	},
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/sa_1/capabilities", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "sa_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	NewPlatformHandler(db.New(store)).SetFeatureFlags(flags).GetAccountCapabilities(rec, req)
	return rec
}

type xCapabilityTestDB struct {
	planID           string
	appMode          string
	scopes           []string
	externalUser     string
	refreshToken     string
	allowedWorkspace string
	lastQueryArgs    []any
}

func (f *xCapabilityTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *xCapabilityTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, pgx.ErrNoRows
}

func (f *xCapabilityTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSocialAccountByIDAndWorkspace"):
		f.lastQueryArgs = append([]any(nil), args...)
		if f.allowedWorkspace != "" && (len(args) < 2 || args[1] != f.allowedWorkspace) {
			return scanRow{err: pgx.ErrNoRows}
		}
		return xCapabilityAccountRow{scopes: f.scopes, appMode: f.appMode, externalUser: f.externalUser, refreshToken: f.refreshToken}
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
	scopes       []string
	appMode      string
	externalUser string
	refreshToken string
}

func (r xCapabilityAccountRow) Scan(dest ...any) error {
	values := []any{
		"sa_1", "pr_1", "twitter", "encrypted-access", pgtype.Text{String: r.refreshToken, Valid: r.refreshToken != ""},
		pgtype.Timestamptz{}, "x-user-1", pgtype.Text{String: "UniPost", Valid: true},
		pgtype.Text{}, pgtype.Timestamptz{Time: time.Now(), Valid: true},
		pgtype.Timestamptz{}, []byte(`{}`), r.scopes, "active", "byo",
		pgtype.Text{}, pgtype.Text{String: r.externalUser, Valid: r.externalUser != ""}, pgtype.Text{}, pgtype.Timestamptz{},
	}
	values = append(values,
		pgtype.Text{String: r.appMode, Valid: r.appMode != ""},
		pgtype.Text{},
		int64(1),
		"active",
	)
	return scanRow{values: values}.Scan(dest...)
}

func TestXAccountReadCapabilitiesUseCreditsFlagAndManagedUserSelector(t *testing.T) {
	store := &xCapabilityTestDB{
		planID: "basic", appMode: "unipost_managed_app",
		scopes:       []string{"tweet.read", "users.read", "offline.access"},
		externalUser: "managed_1", refreshToken: "encrypted-refresh",
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/sa_1/capabilities?external_user_id=managed_1", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "sa_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	NewPlatformHandler(db.New(store)).SetFeatureFlags(platformFeatureFlags(false)).GetAccountCapabilities(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			SchemaVersion string `json:"schema_version"`
			Reads         struct {
				Profile struct {
					Authorized bool `json:"authorized"`
					Credits    struct {
						AccountingEnabled bool   `json:"accounting_enabled"`
						BypassReason      string `json:"bypass_reason"`
						Catalog           int64  `json:"catalog_credits_per_resource"`
						Effective         int64  `json:"effective_credits_per_resource"`
					} `json:"credits"`
				} `json:"profile_read"`
				Posts struct {
					Min int `json:"min_page_size"`
					Max int `json:"max_page_size"`
				} `json:"own_post_history_read"`
			} `json:"x_account_reads"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.SchemaVersion != "1.8" || !body.Data.Reads.Profile.Authorized ||
		body.Data.Reads.Profile.Credits.AccountingEnabled ||
		body.Data.Reads.Profile.Credits.BypassReason != "feature_disabled" ||
		body.Data.Reads.Profile.Credits.Catalog != 10 || body.Data.Reads.Profile.Credits.Effective != 0 ||
		body.Data.Reads.Posts.Min != 5 || body.Data.Reads.Posts.Max != 100 {
		t.Fatalf("data=%+v", body.Data)
	}
}

func TestXAccountReadCapabilitiesRejectMismatchedManagedUser(t *testing.T) {
	store := &xCapabilityTestDB{planID: "basic", appMode: "unipost_managed_app", externalUser: "managed_1"}
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts/sa_1/capabilities?external_user_id=managed_2", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "sa_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()
	NewPlatformHandler(db.New(store)).SetFeatureFlags(platformFeatureFlags(false)).GetAccountCapabilities(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "ACCOUNT_ACCESS_DENIED") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
