package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/socialconnections"
)

func TestSocialAccountBindingCreatesTargetProfileBindingWithoutExposingConnectionID(t *testing.T) {
	store := &fakeSocialConnectionStore{bound: db.SocialAccount{
		ID: "account-b", ProfileID: "profile-b", Platform: "twitter",
		ExternalAccountID: "provider-a", Status: "active", ConnectionType: "managed",
		ConnectionID: pgtype.Text{String: "connection-secret", Valid: true}, BindingStatus: "active",
	}}
	handler := NewSocialAccountHandler(db.New(&bindingHandlerDB{}), nil, nil, nil).SetConnectionStore(store)

	req := httptest.NewRequest(http.MethodPost, "/v1/accounts/account-a/bindings", strings.NewReader(`{
		"profile_id":"profile-b",
		"external_user_id":"managed-a"
	}`))
	ctx := auth.SetWorkspaceID(req.Context(), "workspace-a")
	route := chi.NewRouteContext()
	route.URLParams.Add("id", "account-a")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, route)
	req = req.WithContext(ctx)
	recorder := httptest.NewRecorder()

	handler.Bind(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if store.workspaceID != "workspace-a" || store.sourceAccountID != "account-a" ||
		store.targetProfileID != "profile-b" || store.externalUserID != "managed-a" {
		t.Fatalf("BindExisting args = workspace=%q source=%q target=%q external_user=%q", store.workspaceID, store.sourceAccountID, store.targetProfileID, store.externalUserID)
	}
	var envelope struct {
		Data struct {
			ID               string   `json:"id"`
			SharedConnection bool     `json:"shared_connection"`
			BoundProfileIDs  []string `json:"bound_profile_ids"`
			LeakedConnection string   `json:"connection_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.ID != "account-b" || !envelope.Data.SharedConnection {
		t.Fatalf("binding response = %+v", envelope.Data)
	}
	if len(envelope.Data.BoundProfileIDs) != 2 || envelope.Data.BoundProfileIDs[0] != "profile-a" || envelope.Data.BoundProfileIDs[1] != "profile-b" {
		t.Fatalf("bound_profile_ids = %v", envelope.Data.BoundProfileIDs)
	}
	if envelope.Data.LeakedConnection != "" || strings.Contains(recorder.Body.String(), "connection-secret") {
		t.Fatalf("response leaked connection ID: %s", recorder.Body.String())
	}
}

func TestSocialAccountBindingDisconnectedConnectionRequiresReconnect(t *testing.T) {
	store := &fakeSocialConnectionStore{err: socialconnections.ErrReconnectRequired}
	handler := NewSocialAccountHandler(db.New(&bindingHandlerDB{}), nil, nil, nil).SetConnectionStore(store)
	req := httptest.NewRequest(http.MethodPost, "/v1/accounts/account-a/bindings", strings.NewReader(`{"profile_id":"profile-b"}`))
	ctx := auth.SetWorkspaceID(req.Context(), "workspace-a")
	route := chi.NewRouteContext()
	route.URLParams.Add("id", "account-a")
	req = req.WithContext(context.WithValue(ctx, chi.RouteCtxKey, route))
	recorder := httptest.NewRecorder()

	handler.Bind(recorder, req)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "ACCOUNT_RECONNECT_REQUIRED") {
		t.Fatalf("response missing reconnect code: %s", recorder.Body.String())
	}
}

func TestSocialAccountUnbindLeavesPhysicalConnectionActive(t *testing.T) {
	store := &fakeSocialConnectionStore{}
	handler := NewSocialAccountHandler(db.New(&bindingHandlerDB{}), nil, nil, nil).SetConnectionStore(store)
	req := httptest.NewRequest(http.MethodDelete, "/v1/accounts/account-a/binding", nil)
	ctx := auth.SetWorkspaceID(req.Context(), "workspace-a")
	route := chi.NewRouteContext()
	route.URLParams.Add("id", "account-a")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, route)
	req = req.WithContext(ctx)
	recorder := httptest.NewRecorder()

	handler.Unbind(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if store.unbindWorkspaceID != "workspace-a" || store.unbindAccountID != "account-a" {
		t.Fatalf("Unbind args = workspace=%q account=%q", store.unbindWorkspaceID, store.unbindAccountID)
	}
	if store.disconnectCalls != 0 {
		t.Fatalf("unbind called physical disconnect %d times", store.disconnectCalls)
	}
}

func TestSocialAccountListReturnsScopedBindingMetadataWithoutConnectionID(t *testing.T) {
	account := db.SocialAccount{
		ID: "account-a", ProfileID: "profile-a", Platform: "twitter",
		AccessToken: "encrypted", ExternalAccountID: "provider-a", Status: "active",
		ConnectionType: "byo", ConnectionID: pgtype.Text{String: "connection-secret", Valid: true},
		BindingVersion: 1, BindingStatus: "active",
	}
	store := &bindingListDB{account: account}
	handler := NewSocialAccountHandler(db.New(store), nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/accounts", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "workspace-a"))
	recorder := httptest.NewRecorder()

	handler.List(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data []struct {
			SharedConnection  bool     `json:"shared_connection"`
			BoundProfileIDs   []string `json:"bound_profile_ids"`
			SiblingAccountIDs []string `json:"sibling_account_ids"`
			LeakedConnection  string   `json:"connection_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data) != 1 || !envelope.Data[0].SharedConnection {
		t.Fatalf("list binding metadata = %+v", envelope.Data)
	}
	if got := envelope.Data[0].BoundProfileIDs; len(got) != 2 || got[0] != "profile-a" || got[1] != "profile-b" {
		t.Fatalf("bound_profile_ids = %v", got)
	}
	if got := envelope.Data[0].SiblingAccountIDs; len(got) != 2 || got[0] != "account-a" || got[1] != "account-b" {
		t.Fatalf("sibling_account_ids = %v", got)
	}
	if envelope.Data[0].LeakedConnection != "" || strings.Contains(recorder.Body.String(), "connection-secret") {
		t.Fatalf("list response leaked connection ID: %s", recorder.Body.String())
	}
	if store.resolvedCalls != 1 {
		t.Fatalf("resolved connection reads = %d, want 1", store.resolvedCalls)
	}
}

type fakeSocialConnectionStore struct {
	bound db.SocialAccount
	err   error

	workspaceID       string
	sourceAccountID   string
	targetProfileID   string
	externalUserID    string
	unbindWorkspaceID string
	unbindAccountID   string
	disconnectCalls   int
}

func (*fakeSocialConnectionStore) SaveVerified(context.Context, socialconnections.SaveMode, socialconnections.CredentialInput) (db.SocialAccount, error) {
	return db.SocialAccount{}, nil
}

func (f *fakeSocialConnectionStore) BindExisting(_ context.Context, workspaceID, sourceAccountID, targetProfileID, externalUserID string) (db.SocialAccount, error) {
	f.workspaceID = workspaceID
	f.sourceAccountID = sourceAccountID
	f.targetProfileID = targetProfileID
	f.externalUserID = externalUserID
	return f.bound, f.err
}

func (f *fakeSocialConnectionStore) Unbind(_ context.Context, workspaceID, accountID string) error {
	f.unbindWorkspaceID = workspaceID
	f.unbindAccountID = accountID
	return f.err
}

func (f *fakeSocialConnectionStore) Disconnect(context.Context, string, string) ([]db.SocialAccount, error) {
	f.disconnectCalls++
	return nil, nil
}

type bindingHandlerDB struct{}

func (*bindingHandlerDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (*bindingHandlerDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	if strings.Contains(query, "-- name: ListBoundProfileIDsForAccount") {
		return &scheduledQuotaRows{values: [][]any{{"profile-a"}, {"profile-b"}}}, nil
	}
	return nil, pgx.ErrNoRows
}

func (*bindingHandlerDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return scanRow{err: pgx.ErrNoRows}
}

type bindingListDB struct {
	account       db.SocialAccount
	resolvedCalls int
}

func (*bindingListDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *bindingListDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	switch {
	case strings.Contains(query, "-- name: ListSocialAccountsByWorkspace"):
		return &scheduledQuotaRows{values: [][]any{socialAccountValues(f.account)}}, nil
	case strings.Contains(query, "-- name: ListBoundProfileIDsForAccount"):
		return &scheduledQuotaRows{values: [][]any{{"profile-a"}, {"profile-b"}}}, nil
	case strings.Contains(query, "-- name: ListBoundAccountIDsForAccount"):
		return &scheduledQuotaRows{values: [][]any{{"account-a"}, {"account-b"}}}, nil
	default:
		return nil, errors.New("unexpected Query call")
	}
}

func (f *bindingListDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	if strings.Contains(query, "-- name: GetResolvedSocialAccountByIDAndWorkspace") {
		f.resolvedCalls++
		return scanRow{values: inboxTenantIsolationResolvedSocialAccountValues(f.account)}
	}
	if strings.Contains(query, "-- name: GetProfile") {
		return scanRow{values: []any{
			"profile-a", "Development", pgtype.Timestamptz{}, pgtype.Timestamptz{},
			pgtype.Text{}, pgtype.Text{}, pgtype.Text{}, "workspace-a", false, pgtype.Text{},
		}}
	}
	return scanRow{err: errors.New("unexpected QueryRow call")}
}
