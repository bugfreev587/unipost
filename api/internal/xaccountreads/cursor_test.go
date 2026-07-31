package xaccountreads

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestCursorRoundTripEncryptsUpstreamTokenAndBindsScope(t *testing.T) {
	codec, err := NewCursorCodec([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	scope := CursorScope{
		WorkspaceID: "ws_1", AccountID: "sa_1", ExternalUserID: "managed_1",
		StartTime: "2025-07-29T00:00:00Z", EndTime: "2026-07-29T00:00:00Z",
		ExcludeReposts: true, ExcludeRepliesToOthers: true,
	}
	encoded, expiresAt, err := codec.Encode(scope, "secret-upstream-token", now)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(encoded, "secret-upstream-token") || expiresAt.Sub(now) != 7*24*time.Hour {
		t.Fatalf("cursor=%q expires=%s", encoded, expiresAt)
	}
	token, gotExpiry, err := codec.Decode(encoded, scope, now.Add(time.Hour))
	if err != nil || token != "secret-upstream-token" || !gotExpiry.Equal(expiresAt) {
		t.Fatalf("token=%q expiry=%s err=%v", token, gotExpiry, err)
	}
}

func TestCursorRejectsTamperingExpiryAndScopeMismatch(t *testing.T) {
	codec, _ := NewCursorCodec([]byte("0123456789abcdef0123456789abcdef"))
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	scope := CursorScope{WorkspaceID: "ws_1", AccountID: "sa_1", ExternalUserID: "managed_1"}
	encoded, _, _ := codec.Encode(scope, "page-2", now)
	tamperedBytes, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	tamperedBytes[len(tamperedBytes)-1] ^= 0x01
	tampered := base64.RawURLEncoding.EncodeToString(tamperedBytes)

	cases := []struct {
		name   string
		cursor string
		scope  CursorScope
		now    time.Time
	}{
		{name: "tampered", cursor: tampered, scope: scope, now: now},
		{name: "expired", cursor: encoded, scope: scope, now: now.Add(7*24*time.Hour + time.Second)},
		{name: "workspace", cursor: encoded, scope: CursorScope{WorkspaceID: "ws_2", AccountID: "sa_1", ExternalUserID: "managed_1"}, now: now},
		{name: "account", cursor: encoded, scope: CursorScope{WorkspaceID: "ws_1", AccountID: "sa_2", ExternalUserID: "managed_1"}, now: now},
		{name: "managed user", cursor: encoded, scope: CursorScope{WorkspaceID: "ws_1", AccountID: "sa_1", ExternalUserID: "managed_2"}, now: now},
		{name: "filter", cursor: encoded, scope: CursorScope{WorkspaceID: "ws_1", AccountID: "sa_1", ExternalUserID: "managed_1", ExcludeReposts: true}, now: now},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := codec.Decode(tc.cursor, tc.scope, tc.now); err == nil {
				t.Fatal("expected invalid cursor")
			}
		})
	}
}

func TestCursorRefreshPreservesUpstreamToken(t *testing.T) {
	codec, _ := NewCursorCodec([]byte("0123456789abcdef0123456789abcdef"))
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	scope := CursorScope{WorkspaceID: "ws_1", AccountID: "sa_1", ExternalUserID: "managed_1"}
	old, _, _ := codec.Encode(scope, "page-2", now)
	refreshed, expiresAt, err := codec.Refresh(old, scope, now.Add(6*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	token, _, err := codec.Decode(refreshed, scope, now.Add(6*24*time.Hour))
	if err != nil || token != "page-2" || expiresAt.Sub(now.Add(6*24*time.Hour)) != 7*24*time.Hour {
		t.Fatalf("token=%q expires=%s err=%v", token, expiresAt, err)
	}
}

func TestCursorKeyRotationDecodesPreviousKeyAndIssuesCurrentKey(t *testing.T) {
	oldSecret := []byte("old-0123456789abcdef0123456789abcdef")
	newSecret := []byte("new-0123456789abcdef0123456789abcdef")
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	scope := CursorScope{WorkspaceID: "ws_1", AccountID: "sa_1", ExternalUserID: "managed_1"}
	oldCodec, err := NewCursorCodec(oldSecret)
	if err != nil {
		t.Fatal(err)
	}
	oldCursor, _, err := oldCodec.Encode(scope, "old-page", now)
	if err != nil {
		t.Fatal(err)
	}

	rotated, err := NewCursorCodec(newSecret, oldSecret)
	if err != nil {
		t.Fatal(err)
	}
	if token, _, err := rotated.Decode(oldCursor, scope, now.Add(time.Hour)); err != nil || token != "old-page" {
		t.Fatalf("token=%q err=%v", token, err)
	}
	newCursor, _, err := rotated.Encode(scope, "new-page", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := oldCodec.Decode(newCursor, scope, now); err == nil {
		t.Fatal("old key unexpectedly decoded a cursor issued by the rotated current key")
	}
}
