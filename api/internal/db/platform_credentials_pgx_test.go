package db

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCreatePlatformCredentialExecutesAgainstPostgres(t *testing.T) {
	databaseURL := os.Getenv("PLATFORM_CREDENTIAL_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PLATFORM_CREDENTIAL_TEST_DATABASE_URL is not configured")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	userID := "platform_credential_test_user_" + suffix
	workspaceID := "platform_credential_test_workspace_" + suffix
	if _, err := tx.Exec(ctx,
		"INSERT INTO users (id, email, name) VALUES ($1, $2, $3)",
		userID, "codex-platform-credential-"+suffix+"@example.com", "Platform Credential Test",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO workspaces (id, user_id, name) VALUES ($1, $2, $3)",
		workspaceID, userID, "Platform Credential Test",
	); err != nil {
		t.Fatal(err)
	}

	credential, err := New(tx).CreatePlatformCredential(ctx, CreatePlatformCredentialParams{
		WorkspaceID:            workspaceID,
		Platform:               "bluesky",
		ClientID:               "client-id",
		ClientSecret:           "encrypted-client-secret",
		AppBearerToken:         pgtype.Text{},
		ConsumerSecret:         pgtype.Text{},
		WebhookRouteKey:        "",
		AppBearerTokenSupplied: false,
		ConsumerSecretSupplied: false,
	})
	if err != nil {
		t.Fatalf("CreatePlatformCredential: %v", err)
	}
	if credential.WorkspaceID != workspaceID || credential.Platform != "bluesky" {
		t.Fatalf("credential = %+v", credential)
	}

	twitterCredential, err := New(tx).CreatePlatformCredential(ctx, CreatePlatformCredentialParams{
		WorkspaceID:            workspaceID,
		Platform:               "twitter",
		ClientID:               "twitter-client-id",
		ClientSecret:           "encrypted-twitter-client-secret",
		AppBearerToken:         pgtype.Text{String: "encrypted-app-bearer", Valid: true},
		ConsumerSecret:         pgtype.Text{String: "encrypted-consumer-secret", Valid: true},
		WebhookRouteKey:        "opaque-webhook-route",
		AppBearerTokenSupplied: true,
		ConsumerSecretSupplied: true,
	})
	if err != nil {
		t.Fatalf("CreatePlatformCredential twitter: %v", err)
	}
	if twitterCredential.WebhookRouteKey.String != "opaque-webhook-route" {
		t.Fatalf("twitter webhook route = %+v", twitterCredential.WebhookRouteKey)
	}
}
