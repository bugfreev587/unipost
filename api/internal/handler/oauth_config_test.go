package handler

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
)

const testGetProfileQuery = `-- name: GetProfile :one
SELECT id, name, created_at, updated_at, branding_logo_url, branding_display_name, branding_primary_color, workspace_id FROM profiles WHERE id = $1
`

const testGetPlatformCredentialQuery = `-- name: GetPlatformCredential :one
SELECT id, platform, client_id, client_secret, created_at, workspace_id FROM platform_credentials
WHERE workspace_id = $1 AND platform = $2
`

func TestGetOAuthConfigForProfileUsesWorkspaceCredentials(t *testing.T) {
	t.Parallel()

	enc, err := crypto.NewAESEncryptor("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("NewAESEncryptor: %v", err)
	}
	encryptedSecret, err := enc.Encrypt("brand-secret")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	q := db.New(&fakeOAuthDB{
		profile: db.Profile{
			ID:                 "pr_123",
			Name:               "Default",
			CreatedAt:          pgtype.Timestamptz{Time: time.Unix(1700000000, 0), Valid: true},
			UpdatedAt:          pgtype.Timestamptz{Time: time.Unix(1700000000, 0), Valid: true},
			BrandingLogoUrl:    pgtype.Text{},
			BrandingDisplayName: pgtype.Text{},
			BrandingPrimaryColor: pgtype.Text{},
			WorkspaceID:        "ws_456",
		},
		credential: db.PlatformCredential{
			ID:           "pc_789",
			Platform:     "youtube",
			ClientID:     "brand-client-id",
			ClientSecret: encryptedSecret,
			CreatedAt:    pgtype.Timestamptz{Time: time.Unix(1700000000, 0), Valid: true},
			WorkspaceID:  "ws_456",
		},
	})
	h := &OAuthHandler{
		queries:         q,
		encryptor:       enc,
		baseRedirectURL: "https://api.unipost.dev",
	}
	req := httptest.NewRequest("GET", "/v1/profiles/pr_123/oauth/connect/youtube", nil)

	cfg := h.getOAuthConfigForProfile(req, "pr_123", "youtube", platform.NewYouTubeAdapter())
	if cfg.ClientID != "brand-client-id" {
		t.Fatalf("ClientID = %q, want brand-client-id", cfg.ClientID)
	}
	if cfg.ClientSecret != "brand-secret" {
		t.Fatalf("ClientSecret = %q, want brand-secret", cfg.ClientSecret)
	}
}

type fakeOAuthDB struct {
	profile    db.Profile
	credential db.PlatformCredential
}

func (f *fakeOAuthDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *fakeOAuthDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (f *fakeOAuthDB) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch query {
	case testGetProfileQuery:
		if len(args) == 1 && args[0] == f.profile.ID {
			return fakeRow{values: []any{
				f.profile.ID,
				f.profile.Name,
				f.profile.CreatedAt,
				f.profile.UpdatedAt,
				f.profile.BrandingLogoUrl,
				f.profile.BrandingDisplayName,
				f.profile.BrandingPrimaryColor,
				f.profile.WorkspaceID,
			}}
		}
	case testGetPlatformCredentialQuery:
		if len(args) == 2 && args[0] == f.credential.WorkspaceID && args[1] == f.credential.Platform {
			return fakeRow{values: []any{
				f.credential.ID,
				f.credential.Platform,
				f.credential.ClientID,
				f.credential.ClientSecret,
				f.credential.CreatedAt,
				f.credential.WorkspaceID,
			}}
		}
	}
	return fakeRow{err: pgx.ErrNoRows}
}

type fakeRow struct {
	values []any
	err    error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return pgx.ErrNoRows
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			*d = r.values[i].(string)
		case *pgtype.Timestamptz:
			*d = r.values[i].(pgtype.Timestamptz)
		case *pgtype.Text:
			*d = r.values[i].(pgtype.Text)
		default:
			return pgx.ErrNoRows
		}
	}
	return nil
}
