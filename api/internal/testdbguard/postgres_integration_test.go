//go:build integration

package testdbguard

import (
	"errors"
	"strings"
	"testing"
)

const testDatabaseEnv = "TEST_DATABASE_URL"

func TestOpenValidatedRejectsRemoteFallbackBeforeOpen(t *testing.T) {
	t.Parallel()

	openCalls := 0
	_, _, err := OpenValidated(
		testDatabaseEnv,
		"postgresql://postgres:test@localhost:1,db.example.com:5432/unipost?sslmode=disable",
		func(string) (struct{}, error) {
			openCalls++
			return struct{}{}, errors.New("must not open a database with a remote fallback")
		},
	)
	if err == nil || !strings.Contains(err.Error(), testDatabaseEnv) || !strings.Contains(err.Error(), "db.example.com") {
		t.Fatalf("guard error = %v, want environment name and rejected fallback host", err)
	}
	if openCalls != 0 {
		t.Fatalf("database open callback calls = %d, want 0", openCalls)
	}
}

func TestOpenValidatedRejectsRemotePrimaryBeforeOpen(t *testing.T) {
	t.Parallel()

	openCalls := 0
	_, _, err := OpenValidated(
		testDatabaseEnv,
		"postgresql://postgres:test@db.example.com:5432/unipost?sslmode=require",
		func(string) (struct{}, error) {
			openCalls++
			return struct{}{}, errors.New("must not open a remote database")
		},
	)
	if err == nil || !strings.Contains(err.Error(), testDatabaseEnv) || !strings.Contains(err.Error(), "db.example.com") {
		t.Fatalf("guard error = %v, want environment name and rejected primary host", err)
	}
	if openCalls != 0 {
		t.Fatalf("database open callback calls = %d, want 0", openCalls)
	}
}

func TestOpenValidatedAllowsAllLoopbackMultiHost(t *testing.T) {
	t.Parallel()

	const databaseURL = "postgresql://postgres:test@localhost:5432,127.0.0.1:5433/unipost?sslmode=disable"
	openCalls := 0
	gotURL, config, err := OpenValidated(
		testDatabaseEnv,
		databaseURL,
		func(validatedURL string) (string, error) {
			openCalls++
			return validatedURL, nil
		},
	)
	if err != nil {
		t.Fatalf("validate all-loopback URL: %v", err)
	}
	if gotURL != databaseURL {
		t.Fatalf("opened URL = %q, want %q", gotURL, databaseURL)
	}
	if openCalls != 1 {
		t.Fatalf("database open callback calls = %d, want 1", openCalls)
	}
	if config == nil || len(config.ConnConfig.Fallbacks) != 1 {
		t.Fatalf("parsed fallback count = %d, want 1", len(config.ConnConfig.Fallbacks))
	}
}
