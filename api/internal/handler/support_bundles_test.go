package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type fakeSupportBundleStore struct {
	createParams db.CreateSupportBundleParams
	createRow    db.SupportBundle
	createErr    error

	listParams db.ListAdminSupportBundlesParams
	listRows   []db.ListAdminSupportBundlesRow
	listErr    error

	getAdminID  string
	getAdminRow db.GetAdminSupportBundleRow
	getAdminErr error
}

func (f *fakeSupportBundleStore) CreateSupportBundle(ctx context.Context, arg db.CreateSupportBundleParams) (db.SupportBundle, error) {
	f.createParams = arg
	if f.createErr != nil {
		return db.SupportBundle{}, f.createErr
	}
	return f.createRow, nil
}

func (f *fakeSupportBundleStore) ListAdminSupportBundles(ctx context.Context, arg db.ListAdminSupportBundlesParams) ([]db.ListAdminSupportBundlesRow, error) {
	f.listParams = arg
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.listRows, nil
}

func (f *fakeSupportBundleStore) GetAdminSupportBundle(ctx context.Context, id string) (db.GetAdminSupportBundleRow, error) {
	f.getAdminID = id
	if f.getAdminErr != nil {
		return db.GetAdminSupportBundleRow{}, f.getAdminErr
	}
	return f.getAdminRow, nil
}

func TestSupportBundleCreateScopesToWorkspaceAndStoresRedactedReport(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeSupportBundleStore{
		createRow: db.SupportBundle{
			ID:               "sb_123",
			WorkspaceID:      "ws_1",
			RunID:            "doctor_123",
			SchemaVersion:    "doctor.v1",
			CliVersion:       "0.2.0",
			Summary:          "Need support",
			ReportMarkdown:   "# UniPost Debug Report\n",
			FindingCount:     2,
			RecentErrorCount: 1,
			CreatedAt:        pgTimestamptz(now),
		},
	}
	h := NewSupportBundleHandler(store)

	body := []byte(`{
		"schema_version":"doctor.v1",
		"run_id":"doctor_123",
		"cli_version":"0.2.0",
		"summary":"Need support",
		"finding_count":2,
		"recent_error_count":1,
		"report_markdown":"# UniPost Debug Report\nAPI key: [REDACTED]\n",
		"payload":{
			"authorization":"Bearer raw-token-value",
			"api_key":"up_live_rawpayloadkey",
			"nested":{"access_token":"oauth-token-value"},
			"message":"failed with up_live_rawmessagekey"
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/support-bundles", bytes.NewReader(body))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	req = req.WithContext(auth.SetAPIKeyID(req.Context(), "ak_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.createParams.WorkspaceID != "ws_1" {
		t.Fatalf("workspace = %q, want ws_1", store.createParams.WorkspaceID)
	}
	if !store.createParams.ActorApiKeyID.Valid || store.createParams.ActorApiKeyID.String != "ak_1" {
		t.Fatalf("actor api key not stored: %+v", store.createParams.ActorApiKeyID)
	}
	if store.createParams.ActorUserID.Valid {
		t.Fatalf("api-key upload should not invent actor user: %+v", store.createParams.ActorUserID)
	}
	if store.createParams.ReportMarkdown == "" || !bytes.Contains([]byte(store.createParams.ReportMarkdown), []byte("[REDACTED]")) {
		t.Fatalf("redacted markdown not stored: %q", store.createParams.ReportMarkdown)
	}
	if bytes.Contains([]byte(store.createParams.ReportMarkdown), []byte("up_live_")) {
		t.Fatalf("raw API key leaked into stored report: %q", store.createParams.ReportMarkdown)
	}
	if bytes.Contains(store.createParams.Payload, []byte("raw-token-value")) ||
		bytes.Contains(store.createParams.Payload, []byte("up_live_rawpayloadkey")) ||
		bytes.Contains(store.createParams.Payload, []byte("oauth-token-value")) ||
		bytes.Contains(store.createParams.Payload, []byte("up_live_rawmessagekey")) {
		t.Fatalf("raw secret leaked into stored payload: %s", string(store.createParams.Payload))
	}
	if !bytes.Contains(store.createParams.Payload, []byte("[REDACTED]")) {
		t.Fatalf("payload did not keep redaction marker: %s", string(store.createParams.Payload))
	}

	var resp struct {
		Data supportBundleResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Data.ID != "sb_123" || resp.Data.ReportMarkdown != "" {
		t.Fatalf("create response should include metadata but not report markdown: %+v", resp.Data)
	}
}

func TestSupportBundleCreateRejectsOversizedReports(t *testing.T) {
	h := NewSupportBundleHandler(&fakeSupportBundleStore{})
	oversized := bytes.Repeat([]byte("x"), maxSupportBundleReportBytes+1)
	payload, _ := json.Marshal(map[string]any{
		"schema_version":     "doctor.v1",
		"run_id":             "doctor_big",
		"summary":            "Too big",
		"report_markdown":    string(oversized),
		"finding_count":      1,
		"recent_error_count": 0,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/support-bundles", bytes.NewReader(payload))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rec := httptest.NewRecorder()

	h.Create(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminSupportBundlesListOmitsReportMarkdown(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeSupportBundleStore{
		listRows: []db.ListAdminSupportBundlesRow{{
			ID:               "sb_1",
			WorkspaceID:      "ws_1",
			WorkspaceName:    "Acme",
			OwnerEmail:       "owner@example.com",
			PlanID:           "api",
			RunID:            "doctor_1",
			SchemaVersion:    "doctor.v1",
			CliVersion:       "0.2.0",
			Summary:          "Auth failed",
			FindingCount:     1,
			RecentErrorCount: 2,
			CreatedAt:        pgTimestamptz(now),
		}},
	}
	h := NewSupportBundleHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/support-bundles?workspace_id=ws_1&q=auth&limit=25", nil)
	rec := httptest.NewRecorder()

	h.ListAdmin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.listParams.WorkspaceID != "ws_1" || store.listParams.Query != "auth" || store.listParams.Limit != 25 {
		t.Fatalf("unexpected list params: %+v", store.listParams)
	}
	var resp struct {
		Data []supportBundleResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("rows = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].ReportMarkdown != "" {
		t.Fatalf("list response must not include report markdown: %+v", resp.Data[0])
	}
}

func TestAdminSupportBundleGetIncludesReportMarkdown(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeSupportBundleStore{
		getAdminRow: db.GetAdminSupportBundleRow{
			ID:               "sb_1",
			WorkspaceID:      "ws_1",
			WorkspaceName:    "Acme",
			OwnerEmail:       "owner@example.com",
			PlanID:           "api",
			RunID:            "doctor_1",
			SchemaVersion:    "doctor.v1",
			CliVersion:       "0.2.0",
			Summary:          "Auth failed",
			ReportMarkdown:   "# Report\nsecret: [REDACTED]\n",
			FindingCount:     1,
			RecentErrorCount: 2,
			CreatedAt:        pgTimestamptz(now),
		},
	}
	h := NewSupportBundleHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/support-bundles/sb_1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "sb_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	h.GetAdmin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.getAdminID != "sb_1" {
		t.Fatalf("get id = %q, want sb_1", store.getAdminID)
	}
	var resp struct {
		Data supportBundleResponse `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Data.ReportMarkdown == "" {
		t.Fatalf("detail response must include report markdown: %+v", resp.Data)
	}
	if bytes.Contains([]byte(resp.Data.ReportMarkdown), []byte("up_live_")) {
		t.Fatalf("detail response leaked an API key: %q", resp.Data.ReportMarkdown)
	}
}

func pgTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
