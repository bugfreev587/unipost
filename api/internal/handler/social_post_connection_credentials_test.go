package handler

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestPublishAndDisconnectLoadConnectionResolvedCredentials(t *testing.T) {
	queueSource, err := os.ReadFile("social_post_queue.go")
	if err != nil {
		t.Fatal(err)
	}
	queueBody := string(queueSource)
	loaderStart := strings.Index(queueBody, "func (h *SocialPostHandler) loadDBAccountsByIDs")
	loaderEnd := strings.Index(queueBody[loaderStart:], "\n}\n")
	if loaderStart < 0 || loaderEnd < 0 {
		t.Fatal("loadDBAccountsByIDs source not found")
	}
	loaderBody := queueBody[loaderStart : loaderStart+loaderEnd]
	if !strings.Contains(loaderBody, "GetResolvedSocialAccountByIDAndWorkspace") ||
		!strings.Contains(loaderBody, "AsSocialAccount()") {
		t.Fatal("publish account loading must project connection-owned credentials and state")
	}
	if strings.Contains(loaderBody, "GetSocialAccountByIDAndWorkspace(") {
		t.Fatal("publish account loading must not read stale binding-owned credentials")
	}

	accountsSource, err := os.ReadFile("social_accounts.go")
	if err != nil {
		t.Fatal(err)
	}
	disconnectBody := string(accountsSource)
	disconnectStart := strings.Index(disconnectBody, "func (h *SocialAccountHandler) Disconnect")
	disconnectEnd := strings.Index(disconnectBody, "func (h *SocialAccountHandler) revokeYouTubeConsent")
	if disconnectStart < 0 || disconnectEnd < 0 {
		t.Fatal("disconnect handler source not found")
	}
	disconnectBody = disconnectBody[disconnectStart:disconnectEnd]
	if !strings.Contains(disconnectBody, "GetResolvedSocialAccountByIDAndWorkspace") ||
		!strings.Contains(disconnectBody, "AsSocialAccount()") {
		t.Fatal("YouTube disconnect revoke must use connection-owned credentials")
	}
}

func TestPublishAccountLoaderUsesRotatedConnectionCredential(t *testing.T) {
	rawBinding := db.SocialAccount{
		ID: "binding-b", ProfileID: "profile-b", Platform: "twitter",
		AccessToken: "stale-binding-token", ExternalAccountID: "provider-a",
		Status: "disconnected", ConnectionType: "managed",
		ConnectionID:   pgtype.Text{String: "connection-a", Valid: true},
		BindingVersion: 3, BindingStatus: "active",
	}
	values := inboxTenantIsolationResolvedSocialAccountValues(rawBinding)
	resolvedOffset := len(inboxTenantIsolationSocialAccountValues(rawBinding))
	values[resolvedOffset] = "rotated-connection-token"
	values[resolvedOffset+9] = "active"
	values[resolvedOffset+10] = "managed"
	values[resolvedOffset+11] = "managed-a"

	handler := &SocialPostHandler{queries: db.New(&resolvedPublishAccountDB{values: values})}
	loaded := handler.loadDBAccountsByIDs(context.Background(), "workspace-a", []string{"binding-b"})
	account, ok := loaded["binding-b"]
	if !ok {
		t.Fatal("resolved sibling binding was not loaded")
	}
	if account.ID != "binding-b" || account.ConnectionID.String != "connection-a" {
		t.Fatalf("public binding projection changed: %+v", account)
	}
	if account.AccessToken != "rotated-connection-token" || account.Status != "active" {
		t.Fatalf("loader used stale binding authority: token=%q status=%q", account.AccessToken, account.Status)
	}
}

type resolvedPublishAccountDB struct{ values []any }

func (*resolvedPublishAccountDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (*resolvedPublishAccountDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, pgx.ErrNoRows
}

func (f *resolvedPublishAccountDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	if !strings.Contains(query, "-- name: GetResolvedSocialAccountByIDAndWorkspace") {
		return scanRow{err: pgx.ErrNoRows}
	}
	return scanRow{values: f.values}
}
