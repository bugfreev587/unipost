package auth

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/apikey"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestAuthenticateAPIKeyUsesCreatorsCurrentActiveRole(t *testing.T) {
	store := &apiKeyAuthTestDB{membershipRole: RoleEditor, membershipStatus: "active"}
	req := httptest.NewRequest("GET", "/v1/members", nil)
	rec := httptest.NewRecorder()

	ctx, ok := authenticateAPIKey(rec, req, db.New(store), apiKeyAuthTestToken)

	if !ok {
		t.Fatalf("authenticateAPIKey rejected active creator: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := GetRole(ctx); got != RoleEditor {
		t.Fatalf("role = %q, want %q", got, RoleEditor)
	}
}

func TestAuthenticateAPIKeyRejectsCreatorWithoutActiveMembership(t *testing.T) {
	tests := []struct {
		name             string
		membershipStatus string
		membershipErr    error
	}{
		{name: "membership removed", membershipErr: pgx.ErrNoRows},
		{name: "membership suspended", membershipStatus: "suspended"},
		{name: "membership lookup failed", membershipErr: errors.New("database unavailable")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &apiKeyAuthTestDB{
				membershipRole:   RoleEditor,
				membershipStatus: tt.membershipStatus,
				membershipErr:    tt.membershipErr,
			}
			req := httptest.NewRequest("GET", "/v1/members", nil)
			rec := httptest.NewRecorder()

			ctx, ok := authenticateAPIKey(rec, req, db.New(store), apiKeyAuthTestToken)

			if ok || ctx != nil {
				t.Fatalf("authenticateAPIKey authenticated inactive creator as role %q", GetRole(ctx))
			}
			if rec.Code != 401 {
				t.Fatalf("status = %d, want 401; body=%s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"code":"UNAUTHORIZED"`) {
				t.Fatalf("body = %s, want UNAUTHORIZED error", rec.Body.String())
			}
		})
	}
}

func TestAuthenticateAPIKeyKeepsLegacyCreatorlessKeyCompatibility(t *testing.T) {
	store := &apiKeyAuthTestDB{creatorUserID: ""}
	req := httptest.NewRequest("GET", "/v1/members", nil)
	rec := httptest.NewRecorder()

	ctx, ok := authenticateAPIKey(rec, req, db.New(store), apiKeyAuthTestToken)

	if !ok {
		t.Fatalf("authenticateAPIKey rejected legacy key: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := GetRole(ctx); got != RoleOwner {
		t.Fatalf("role = %q, want %q", got, RoleOwner)
	}
	if store.membershipQueries != 0 {
		t.Fatalf("membership queries = %d, want 0", store.membershipQueries)
	}
}

const apiKeyAuthTestToken = "up_test_11111111111111111111111111111111"

type apiKeyAuthTestDB struct {
	creatorUserID     string
	membershipRole    string
	membershipStatus  string
	membershipErr     error
	membershipQueries int
}

func (f *apiKeyAuthTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *apiKeyAuthTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, pgx.ErrNoRows
}

func (f *apiKeyAuthTestDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetAPIKeyByHash"):
		creatorUserID := f.creatorUserID
		if creatorUserID == "" && f.membershipRole != "" {
			creatorUserID = "user_editor"
		}
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return apiKeyAuthTestRow{values: []any{
			"key_1",
			"Editor key",
			"up_test_11111111",
			now,
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			apikey.Hash(apiKeyAuthTestToken),
			"test",
			"workspace_1",
			creatorUserID,
		}}
	case strings.Contains(query, "-- name: GetMembership"):
		f.membershipQueries++
		if f.membershipErr != nil {
			return apiKeyAuthTestRow{err: f.membershipErr}
		}
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		return apiKeyAuthTestRow{values: []any{
			"workspace_1",
			"user_editor",
			f.membershipRole,
			f.membershipStatus,
			pgtype.Text{},
			now,
			pgtype.Timestamptz{},
			now,
			now,
		}}
	default:
		return apiKeyAuthTestRow{err: pgx.ErrNoRows}
	}
}

type apiKeyAuthTestRow struct {
	values []any
	err    error
}

func (r apiKeyAuthTestRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("unexpected scan destination count")
	}
	for i := range dest {
		switch target := dest[i].(type) {
		case *string:
			*target = r.values[i].(string)
		case *pgtype.Text:
			*target = r.values[i].(pgtype.Text)
		case *pgtype.Timestamptz:
			*target = r.values[i].(pgtype.Timestamptz)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}
