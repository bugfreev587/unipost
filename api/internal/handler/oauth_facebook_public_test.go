package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestPendingFacebookConnectionIsWorkspaceScopedAndFinalizesForOwner(t *testing.T) {
	newHandler := func(t *testing.T, store *pendingFacebookTestDB) *OAuthHandler {
		t.Helper()
		encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
		if err != nil {
			t.Fatalf("encryptor: %v", err)
		}
		return NewOAuthHandler(db.New(store), encryptor, nil)
	}

	t.Run("owner can read pending Page choices without token disclosure", func(t *testing.T) {
		store := newPendingFacebookTestDB(t)
		h := newHandler(t, store)
		req := pendingFacebookRequest(http.MethodGet, "/v1/pending-connections/pending_1", "ws_owner", "pending_1", "")
		rec := httptest.NewRecorder()

		h.PendingConnectionGet(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), "encrypted-page-token") || strings.Contains(rec.Body.String(), "encrypted-user-token") {
			t.Fatalf("response disclosed encrypted token material: %s", rec.Body.String())
		}
	})

	t.Run("different workspace cannot read or finalize", func(t *testing.T) {
		store := newPendingFacebookTestDB(t)
		h := newHandler(t, store)

		getReq := pendingFacebookRequest(http.MethodGet, "/v1/pending-connections/pending_1", "ws_other", "pending_1", "")
		getRec := httptest.NewRecorder()
		h.PendingConnectionGet(getRec, getReq)
		if getRec.Code != http.StatusNotFound {
			t.Fatalf("cross-workspace read status = %d, body = %s", getRec.Code, getRec.Body.String())
		}

		postReq := pendingFacebookRequest(http.MethodPost, "/v1/pending-connections/pending_1/finalize", "ws_other", "pending_1", `{"page_ids":["page_1"]}`)
		postRec := httptest.NewRecorder()
		h.PendingConnectionFinalize(postRec, postReq)
		if postRec.Code != http.StatusNotFound {
			t.Fatalf("cross-workspace finalize status = %d, body = %s", postRec.Code, postRec.Body.String())
		}
		if store.writeCalls != 0 {
			t.Fatalf("cross-workspace finalize performed %d writes", store.writeCalls)
		}
	})

	t.Run("unknown Page is rejected before writes", func(t *testing.T) {
		store := newPendingFacebookTestDB(t)
		h := newHandler(t, store)
		req := pendingFacebookRequest(http.MethodPost, "/v1/pending-connections/pending_1/finalize", "ws_owner", "pending_1", `{"page_ids":["page_not_authorized"]}`)
		rec := httptest.NewRecorder()

		h.PendingConnectionFinalize(rec, req)

		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "UNKNOWN_PAGE") {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if store.writeCalls != 0 {
			t.Fatalf("unknown Page finalize performed %d writes", store.writeCalls)
		}
	})

	t.Run("owner can finalize an authorized Page", func(t *testing.T) {
		store := newPendingFacebookTestDB(t)
		h := newHandler(t, store)
		req := pendingFacebookRequest(http.MethodPost, "/v1/pending-connections/pending_1/finalize", "ws_owner", "pending_1", `{"page_ids":["page_1"]}`)
		rec := httptest.NewRecorder()

		h.PendingConnectionFinalize(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
		}
		if store.writeCalls != 3 {
			t.Fatalf("write calls = %d, want meta token + social account + pending delete", store.writeCalls)
		}
	})
}

func pendingFacebookRequest(method, path, workspaceID, pendingID, body string) *http.Request {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), workspaceID))
	return withOAuthPKCEChiParams(req, map[string]string{"id": pendingID})
}

type pendingFacebookTestDB struct {
	pending    db.PendingConnection
	writeCalls int
}

func newPendingFacebookTestDB(t *testing.T) *pendingFacebookTestDB {
	t.Helper()
	pages, err := json.Marshal(pendingFacebookConnection{Pages: []pendingFacebookPage{{
		ID:                       "page_1",
		Name:                     "Owner Page",
		Tasks:                    []string{"CREATE_CONTENT"},
		PageAccessTokenEncrypted: "encrypted-page-token",
	}}})
	if err != nil {
		t.Fatalf("marshal pages: %v", err)
	}
	now := time.Now()
	return &pendingFacebookTestDB{pending: db.PendingConnection{
		ID:                 "pending_1",
		WorkspaceID:        "ws_owner",
		ProfileID:          "profile_1",
		Platform:           "facebook",
		MetaUserID:         "meta_user_1",
		UserTokenEncrypted: "encrypted-user-token",
		UserTokenExpiresAt: pgtype.Timestamptz{Time: now.Add(30 * 24 * time.Hour), Valid: true},
		PagesJson:          pages,
		ExpiresAt:          pgtype.Timestamptz{Time: now.Add(10 * time.Minute), Valid: true},
		CreatedAt:          pgtype.Timestamptz{Time: now, Valid: true},
	}}
}

func (f *pendingFacebookTestDB) Exec(_ context.Context, query string, _ ...interface{}) (pgconn.CommandTag, error) {
	if strings.Contains(query, "-- name: DeletePendingConnection") {
		f.writeCalls++
		return pgconn.CommandTag{}, nil
	}
	return pgconn.CommandTag{}, fmt.Errorf("unexpected Exec: %s", query)
}

func (f *pendingFacebookTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, fmt.Errorf("unexpected Query")
}

func (f *pendingFacebookTestDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	switch {
	case strings.Contains(query, "-- name: GetPendingConnection"):
		if len(args) != 2 || args[0] != f.pending.ID || args[1] != f.pending.WorkspaceID {
			return scanRow{err: pgx.ErrNoRows}
		}
		p := f.pending
		return scanRow{values: []any{p.ID, p.WorkspaceID, p.ProfileID, p.Platform, p.MetaUserID, p.UserTokenEncrypted, p.UserTokenExpiresAt, p.PagesJson, p.ExpiresAt, p.CreatedAt}}
	case strings.Contains(query, "-- name: UpsertMetaUserToken"):
		f.writeCalls++
		return scanRow{values: []any{f.pending.WorkspaceID, f.pending.MetaUserID, f.pending.UserTokenEncrypted, f.pending.UserTokenExpiresAt, now, now}}
	case strings.Contains(query, "-- name: FindSocialAccountByExternalID"):
		return scanRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "-- name: CreateSocialAccount"):
		f.writeCalls++
		return scanRow{values: []any{
			"account_1", f.pending.ProfileID, "facebook", "encrypted-page-token", pgtype.Text{}, pgtype.Timestamptz{},
			"page_1", pgtype.Text{String: "Owner Page", Valid: true}, pgtype.Text{}, now, pgtype.Timestamptz{}, []byte("{}"),
			[]string{"pages_manage_posts"}, "active", "byo", pgtype.Text{}, pgtype.Text{}, pgtype.Text{}, now,
		}}
	default:
		return scanRow{err: fmt.Errorf("unexpected QueryRow: %s", query)}
	}
}
