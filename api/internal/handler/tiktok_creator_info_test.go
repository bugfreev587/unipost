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

	"github.com/xiaoboyu/unipost-api/internal/auth"
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestTikTokCreatorInfoTreatsUndecryptableTokenAsReconnectRequired(t *testing.T) {
	dbtx := &tikTokCreatorInfoTestDB{account: db.SocialAccount{
		ID:                "acct_tiktok",
		ProfileID:         "profile_1",
		Platform:          "tiktok",
		AccessToken:       "not-an-encrypted-token",
		ExternalAccountID: "open_id",
		Status:            "reconnect_required",
		ConnectionType:    "byo",
	}}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatalf("new encryptor: %v", err)
	}
	handler := NewSocialAccountHandler(db.New(dbtx), encryptor, nil, nil)
	platform.Register(platform.NewTikTokAdapter())

	req := httptest.NewRequest(http.MethodGet, "/v1/social-accounts/acct_tiktok/tiktok/creator-info", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", "acct_tiktok")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	rr := httptest.NewRecorder()

	handler.TikTokCreatorInfo(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rr.Code, rr.Body.String())
	}
	var response struct {
		Error struct {
			Code           string `json:"code"`
			NormalizedCode string `json:"normalized_code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error.Code != "NEEDS_RECONNECT" || response.Error.NormalizedCode != "needs_reconnect" {
		t.Fatalf("error = %s/%s, want NEEDS_RECONNECT/needs_reconnect", response.Error.Code, response.Error.NormalizedCode)
	}
}

type tikTokCreatorInfoTestDB struct {
	account db.SocialAccount
}

func (f *tikTokCreatorInfoTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *tikTokCreatorInfoTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}

func (f *tikTokCreatorInfoTestDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	if strings.Contains(query, "-- name: GetSocialAccountByIDAndWorkspace") {
		return scanRow{values: socialAccountValues(f.account)}
	}
	return scanRow{err: errors.New("unexpected query")}
}
