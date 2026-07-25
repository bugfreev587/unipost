package handler

import (
	"context"
	"encoding/json"
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
		store.targetProfileID != "profile-b" || store.selectedExternalUserID != "managed-a" {
		t.Fatalf("BindExisting args = workspace=%q source=%q target=%q user=%q", store.workspaceID, store.sourceAccountID, store.targetProfileID, store.selectedExternalUserID)
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

type fakeSocialConnectionStore struct {
	bound db.SocialAccount
	err   error

	workspaceID            string
	sourceAccountID        string
	targetProfileID        string
	selectedExternalUserID string
}

func (*fakeSocialConnectionStore) SaveVerified(context.Context, socialconnections.SaveMode, socialconnections.CredentialInput) (db.SocialAccount, error) {
	return db.SocialAccount{}, nil
}

func (f *fakeSocialConnectionStore) BindExisting(_ context.Context, workspaceID, sourceAccountID, targetProfileID, selectedExternalUserID string) (db.SocialAccount, error) {
	f.workspaceID = workspaceID
	f.sourceAccountID = sourceAccountID
	f.targetProfileID = targetProfileID
	f.selectedExternalUserID = selectedExternalUserID
	return f.bound, f.err
}

func (*fakeSocialConnectionStore) Unbind(context.Context, string, string) error { return nil }

func (*fakeSocialConnectionStore) Disconnect(context.Context, string, string) ([]db.SocialAccount, error) {
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
