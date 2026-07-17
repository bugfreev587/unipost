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

func TestAPIKeyCreate_FreePlanRejectsSecondActiveKey(t *testing.T) {
	store := &freePlanLimitsTestDB{planID: "free", activeAPIKeys: 1}
	h := NewAPIKeyHandler(db.New(store))
	req := httptest.NewRequest(http.MethodPost, "/v1/api-keys", strings.NewReader(`{
		"name": "second key",
		"environment": "production"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.createAPIKeyCalls != 0 {
		t.Fatalf("CreateAPIKey calls = %d, want 0", store.createAPIKeyCalls)
	}
}

func TestAPIKeyCreate_PaidPlanAllowsMultipleKeys(t *testing.T) {
	store := &freePlanLimitsTestDB{planID: "basic", activeAPIKeys: 1}
	h := NewAPIKeyHandler(db.New(store))
	req := httptest.NewRequest(http.MethodPost, "/v1/api-keys", strings.NewReader(`{
		"name": "second key",
		"environment": "production"
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "user_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.createAPIKeyCalls != 1 {
		t.Fatalf("CreateAPIKey calls = %d, want 1", store.createAPIKeyCalls)
	}
}

func TestWebhookCreate_FreePlanRejectsSecondActiveWebhook(t *testing.T) {
	store := &freePlanLimitsTestDB{planID: "free", activeWebhooks: 1}
	h := NewWebhookSubscriptionHandler(db.New(store))
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks", strings.NewReader(`{
		"name": "second webhook",
		"url": "https://example.com/hooks/two",
		"events": ["post.published"]
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.createWebhookCalls != 0 {
		t.Fatalf("CreateWebhook calls = %d, want 0", store.createWebhookCalls)
	}
}

func TestWebhookCreate_FreePlanAllowsInactiveWebhookWhenActiveCapReached(t *testing.T) {
	store := &freePlanLimitsTestDB{planID: "free", activeWebhooks: 1}
	h := NewWebhookSubscriptionHandler(db.New(store))
	req := httptest.NewRequest(http.MethodPost, "/v1/webhooks", strings.NewReader(`{
		"name": "inactive webhook",
		"url": "https://example.com/hooks/inactive",
		"events": ["post.published"],
		"active": false
	}`))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.createWebhookCalls != 1 {
		t.Fatalf("CreateWebhook calls = %d, want 1", store.createWebhookCalls)
	}
}

func TestApiLimits_FreePlanReturnsPackagingCaps(t *testing.T) {
	store := &freePlanLimitsTestDB{
		planID:                "free",
		activeAPIKeys:         1,
		activeWebhooks:        1,
		activeManagedAccounts: 2,
		managedUsers:          3,
		activeDeliveryJobs:    4,
		currentProfiles:       1,
		currentMembers:        1,
		freePlanMaxProfiles:   1,
		freePlanMaxMembers:    1,
		whiteLabelAllowed:     false,
		allowInbox:            false,
		allowAnalytics:        false,
		allowTwitter:          false,
	}
	queries := db.New(store)
	h := NewApiLimitsHandler(queries, quota.NewChecker(queries))
	req := httptest.NewRequest(http.MethodGet, "/v1/limits", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	data := decodeLimitsData(t, rec.Body.Bytes())
	assertJSONNumber(t, data, "max_api_keys", 1)
	assertJSONNumber(t, data, "current_api_keys", 1)
	assertJSONNumber(t, data, "max_webhooks", 1)
	assertJSONNumber(t, data, "current_webhooks", 1)
	assertJSONNumber(t, data, "max_managed_accounts", 2)
	assertJSONNumber(t, data, "current_managed_accounts", 2)
	assertJSONNumber(t, data, "max_managed_users", 3)
	assertJSONNumber(t, data, "current_managed_users", 3)
}

func TestApiLimits_TeamPlanReturnsPublishedEntitlementBundle(t *testing.T) {
	store := &freePlanLimitsTestDB{
		planID:                "team",
		activeAPIKeys:         4,
		activeWebhooks:        2,
		activeManagedAccounts: 18,
		managedUsers:          11,
		activeDeliveryJobs:    8,
		currentProfiles:       26,
		currentMembers:        4,
		whiteLabelAllowed:     true,
		allowInbox:            true,
		allowAnalytics:        true,
		allowTwitter:          true,
	}
	queries := db.New(store)
	h := NewApiLimitsHandler(queries, quota.NewChecker(queries))
	req := httptest.NewRequest(http.MethodGet, "/v1/limits", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	data := decodeLimitsData(t, rec.Body.Bytes())
	if got := data["plan_id"]; got != "team" {
		t.Fatalf("plan_id = %#v, want team", got)
	}
	assertJSONBool(t, data, "plan_allows_twitter", true)
	assertJSONBool(t, data, "plan_allows_inbox", true)
	assertJSONBool(t, data, "plan_allows_analytics", true)
	assertJSONBool(t, data, "plan_allows_audit_log", true)
	assertJSONBool(t, data, "plan_allows_white_label", true)
	assertJSONBool(t, data, "plan_allows_hosted_connect_branding", true)
	assertJSONBool(t, data, "plan_allows_hide_powered_by", true)
	assertJSONNumber(t, data, "white_label_platform_limit", -1)
	assertJSONNumber(t, data, "max_profiles", -1)
	assertJSONNumber(t, data, "current_profiles", 26)
	assertJSONNumber(t, data, "max_members", -1)
	assertJSONNumber(t, data, "current_members", 4)
	assertJSONNumber(t, data, "max_api_keys", -1)
	assertJSONNumber(t, data, "current_api_keys", 4)
	assertJSONNumber(t, data, "max_webhooks", -1)
	assertJSONNumber(t, data, "current_webhooks", 2)
	assertJSONNumber(t, data, "max_managed_accounts", -1)
	assertJSONNumber(t, data, "current_managed_accounts", 18)
	assertJSONNumber(t, data, "max_managed_users", -1)
	assertJSONNumber(t, data, "current_managed_users", 11)
}

func assertJSONBool(t *testing.T, data map[string]any, key string, want bool) {
	t.Helper()
	got, ok := data[key].(bool)
	if !ok {
		t.Fatalf("%s = %#v, want JSON boolean %v", key, data[key], want)
	}
	if got != want {
		t.Fatalf("%s = %v, want %v", key, got, want)
	}
}

type freePlanLimitsTestDB struct {
	planID                string
	activeAPIKeys         int32
	activeWebhooks        int32
	activeManagedAccounts int32
	managedUsers          int32
	activeDeliveryJobs    int64
	currentProfiles       int32
	currentMembers        int32
	freePlanMaxProfiles   int32
	freePlanMaxMembers    int32
	whiteLabelAllowed     bool
	allowTwitter          bool
	allowInbox            bool
	allowAnalytics        bool
	createAPIKeyCalls     int
	createWebhookCalls    int
}

func (f *freePlanLimitsTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *freePlanLimitsTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, pgx.ErrNoRows
}

func (f *freePlanLimitsTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetSubscriptionByWorkspace"):
		return subscriptionScanRow(f.planID)
	case strings.Contains(query, "-- name: GetPlan"):
		return planScanRow(f)
	case strings.Contains(query, "-- name: GetWorkspace"):
		return workspaceScanRow()
	case strings.Contains(query, "-- name: CountActiveDeliveryJobsByWorkspace"):
		return scanRow{values: []any{f.activeDeliveryJobs}}
	case strings.Contains(query, "-- name: CountProfilesByWorkspace"):
		return scanRow{values: []any{f.currentProfiles}}
	case strings.Contains(query, "-- name: CountActiveMembersByWorkspace"):
		return scanRow{values: []any{f.currentMembers}}
	case strings.Contains(query, "-- name: CountActiveAPIKeysByWorkspace"):
		return scanRow{values: []any{f.activeAPIKeys}}
	case strings.Contains(query, "-- name: CreateAPIKey"):
		f.createAPIKeyCalls++
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		name, _ := args[2].(string)
		prefix, _ := args[3].(string)
		keyHash, _ := args[4].(string)
		env, _ := args[5].(string)
		expiresAt, _ := args[6].(pgtype.Timestamptz)
		createdBy, _ := args[7].(string)
		return scanRow{values: []any{
			"ak_1",
			name,
			prefix,
			now,
			pgtype.Timestamptz{},
			expiresAt,
			pgtype.Timestamptz{},
			keyHash,
			env,
			"ws_1",
			createdBy,
		}}
	case strings.Contains(query, "-- name: CountActiveWebhooksByWorkspace"):
		return scanRow{values: []any{f.activeWebhooks}}
	case strings.Contains(query, "-- name: CountActiveManagedAccountsByWorkspace"):
		return scanRow{values: []any{f.activeManagedAccounts}}
	case strings.Contains(query, "-- name: CountManagedUsersByWorkspace"):
		return scanRow{values: []any{f.managedUsers}}
	case strings.Contains(query, "-- name: CreateWebhook"):
		f.createWebhookCalls++
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		name, _ := args[1].(string)
		urlValue, _ := args[2].(string)
		secret, _ := args[3].(string)
		events, _ := args[4].([]string)
		active, _ := args[5].(bool)
		return scanRow{values: []any{
			"wh_1",
			urlValue,
			secret,
			events,
			active,
			now,
			"ws_1",
			name,
		}}
	default:
		return scanRow{err: pgx.ErrNoRows}
	}
}

func decodeLimitsData(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var response struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, string(body))
	}
	if response.Data == nil {
		t.Fatalf("response data is nil: %s", string(body))
	}
	return response.Data
}

func assertJSONNumber(t *testing.T, data map[string]any, key string, want float64) {
	t.Helper()
	got, ok := data[key].(float64)
	if !ok {
		t.Fatalf("%s = %#v, want JSON number %v", key, data[key], want)
	}
	if got != want {
		t.Fatalf("%s = %v, want %v", key, got, want)
	}
}

func subscriptionScanRow(planID string) scanRow {
	if planID == "" {
		planID = "free"
	}
	return scanRow{values: []any{
		"sub_1",
		planID,
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
}

func planScanRow(f *freePlanLimitsTestDB) scanRow {
	planID := f.planID
	if planID == "" {
		planID = "free"
	}
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	maxProfiles := pgtype.Int4{}
	if f.freePlanMaxProfiles > 0 {
		maxProfiles = pgtype.Int4{Int32: f.freePlanMaxProfiles, Valid: true}
	}
	maxMembers := pgtype.Int4{}
	if f.freePlanMaxMembers > 0 {
		maxMembers = pgtype.Int4{Int32: f.freePlanMaxMembers, Valid: true}
	}
	return scanRow{values: []any{
		planID,
		planID,
		int32(0),
		int32(100),
		pgtype.Text{},
		now,
		f.whiteLabelAllowed,
		f.allowTwitter,
		f.allowInbox,
		f.allowAnalytics,
		maxProfiles,
		maxMembers,
	}}
}

func workspaceScanRow() scanRow {
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	return scanRow{values: []any{
		"ws_1",
		"user_1",
		"Workspace",
		pgtype.Int4{},
		now,
		now,
		[]string{"dashboard", "api"},
		pgtype.Text{},
	}}
}
