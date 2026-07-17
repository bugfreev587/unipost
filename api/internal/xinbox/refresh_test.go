package xinbox

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

	"github.com/xiaoboyu/unipost-api/internal/connect"
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestXTokenRefreshResolverUsesPersistedAppModeRegardlessOfConnectionType(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	workspaceSecret, err := encryptor.Encrypt("workspace-secret")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name           string
		appMode        AppMode
		connectionType string
		wantClientID   string
		wantSecret     string
		wantCredReads  int
	}{
		{"managed app on BYO row", AppModeUniPostManaged, "byo", "global-client", "global-secret", 0},
		{"managed app on managed row", AppModeUniPostManaged, "managed", "global-client", "global-secret", 0},
		{"workspace app on BYO row", AppModeWorkspace, "byo", "workspace-client", "workspace-secret", 1},
		{"workspace app on managed row", AppModeWorkspace, "managed", "workspace-client", "workspace-secret", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &refreshResolverTestDB{
				workspaceID:     "ws_1",
				credential:      true,
				clientID:        "workspace-client",
				encryptedSecret: workspaceSecret,
			}
			resolver := NewTokenRefreshResolver(db.New(store), encryptor, "global-client", "global-secret", "https://api.example.test")
			connector, err := resolver.Resolve(context.Background(), db.SocialAccount{
				ProfileID:      "profile_1",
				Platform:       "twitter",
				ConnectionType: tt.connectionType,
				XAppMode:       pgtype.Text{String: string(tt.appMode), Valid: true},
			})
			if err != nil {
				t.Fatalf("Resolve: %v", err)
			}
			gotClientID, gotSecret := refreshClientCredentials(t, connector)
			if gotClientID != tt.wantClientID {
				t.Fatalf("refresh client_id = %q, want %q", gotClientID, tt.wantClientID)
			}
			if gotSecret != tt.wantSecret {
				t.Fatalf("refresh client_secret = %q, want %q", gotSecret, tt.wantSecret)
			}
			if store.credentialReads != tt.wantCredReads {
				t.Fatalf("credential reads = %d, want %d", store.credentialReads, tt.wantCredReads)
			}
		})
	}
}

func TestXTokenRefreshResolverWorkspaceModeNeverFallsBackToGlobal(t *testing.T) {
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name            string
		credential      bool
		encryptedSecret string
	}{
		{"credential removed", false, ""},
		{"credential cannot decrypt", true, "not-ciphertext"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &refreshResolverTestDB{
				workspaceID:     "ws_1",
				credential:      tt.credential,
				clientID:        "workspace-client",
				encryptedSecret: tt.encryptedSecret,
			}
			resolver := NewTokenRefreshResolver(db.New(store), encryptor, "global-client", "global-secret", "")
			if _, err := resolver.Resolve(context.Background(), db.SocialAccount{
				ProfileID: "profile_1",
				Platform:  "twitter",
				XAppMode:  pgtype.Text{String: string(AppModeWorkspace), Valid: true},
			}); err == nil {
				t.Fatal("Resolve error = nil, want fail-closed workspace credential error")
			}
		})
	}
}

func TestXTokenRefreshResolverRejectsUnknownAppIdentity(t *testing.T) {
	resolver := NewTokenRefreshResolver(nil, nil, "global-client", "global-secret", "")
	for _, raw := range []string{"", "garbage", string(AppModeLegacyUnknown)} {
		t.Run(raw, func(t *testing.T) {
			if _, err := resolver.Resolve(context.Background(), db.SocialAccount{
				Platform: "twitter",
				XAppMode: pgtype.Text{String: raw, Valid: raw != ""},
			}); err == nil {
				t.Fatalf("Resolve app_mode=%q error = nil, want validation/reconnect error", raw)
			}
		})
	}
}

func refreshClientCredentials(t *testing.T, connector connect.Connector) (string, string) {
	t.Helper()
	var gotClientID string
	var gotSecret string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		gotClientID = r.Form.Get("client_id")
		_, gotSecret, _ = r.BasicAuth()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access",
			"refresh_token": "refresh",
			"expires_in":    3600,
		})
	}))
	defer server.Close()
	twitter, ok := connector.(*connect.TwitterConnector)
	if !ok {
		t.Fatalf("connector = %T, want *connect.TwitterConnector", connector)
	}
	twitter.TokenEndpoint = server.URL
	if _, err := connector.Refresh(context.Background(), "old-refresh"); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return gotClientID, gotSecret
}

type refreshResolverTestDB struct {
	workspaceID     string
	credential      bool
	clientID        string
	encryptedSecret string
	credentialReads int
}

func (f *refreshResolverTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *refreshResolverTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, pgx.ErrNoRows
}

func (f *refreshResolverTestDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "-- name: GetProfile"):
		return refreshProfileRow{workspaceID: f.workspaceID}
	case strings.Contains(query, "-- name: GetPlatformCredential"):
		f.credentialReads++
		if !f.credential {
			return refreshErrorRow{err: pgx.ErrNoRows}
		}
		return refreshCredentialRow{
			workspaceID:     f.workspaceID,
			clientID:        f.clientID,
			encryptedSecret: f.encryptedSecret,
		}
	default:
		return refreshErrorRow{err: pgx.ErrNoRows}
	}
}

type refreshProfileRow struct{ workspaceID string }

func (r refreshProfileRow) Scan(dest ...any) error {
	*(dest[0].(*string)) = "profile_1"
	*(dest[1].(*string)) = "Profile"
	*(dest[2].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	*(dest[3].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	for _, i := range []int{4, 5, 6, 9} {
		*(dest[i].(*pgtype.Text)) = pgtype.Text{}
	}
	*(dest[7].(*string)) = r.workspaceID
	*(dest[8].(*bool)) = false
	return nil
}

type refreshCredentialRow struct {
	workspaceID     string
	clientID        string
	encryptedSecret string
}

func (r refreshCredentialRow) Scan(dest ...any) error {
	*(dest[0].(*string)) = "pc_1"
	*(dest[1].(*string)) = "twitter"
	*(dest[2].(*string)) = r.clientID
	*(dest[3].(*string)) = r.encryptedSecret
	*(dest[4].(*pgtype.Timestamptz)) = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	*(dest[5].(*string)) = r.workspaceID
	*(dest[6].(*pgtype.Text)) = pgtype.Text{}
	*(dest[7].(*pgtype.Text)) = pgtype.Text{}
	return nil
}

type refreshErrorRow struct{ err error }

func (r refreshErrorRow) Scan(...any) error { return r.err }
