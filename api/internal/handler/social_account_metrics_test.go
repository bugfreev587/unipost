package handler

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestAccountMetricsPlatformErrorResponseNeedsReconnectSentinel(t *testing.T) {
	status, code, message, ok := accountMetricsPlatformErrorResponse(
		"youtube",
		fmt.Errorf("youtube metrics credentials rejected: %w", platform.ErrNeedsReconnect),
	)

	if !ok {
		t.Fatal("expected reconnect response")
	}
	if status != http.StatusConflict || code != "NEEDS_RECONNECT" {
		t.Fatalf("response = %d/%s, want 409/NEEDS_RECONNECT", status, code)
	}
	if message != "Reconnect YouTube to enable analytics." {
		t.Fatalf("message = %q", message)
	}
}

func TestAccountMetricsPlatformErrorResponseNoYouTubeChannelNeedsReconnect(t *testing.T) {
	status, code, _, ok := accountMetricsPlatformErrorResponse("youtube", platform.ErrYouTubeNoChannel)

	if !ok {
		t.Fatal("expected reconnect response")
	}
	if status != http.StatusConflict || code != "NEEDS_RECONNECT" {
		t.Fatalf("response = %d/%s, want 409/NEEDS_RECONNECT", status, code)
	}
}

func TestAccountMetricsPlatformErrorResponseKeepsLegacyReconnectDetection(t *testing.T) {
	tests := []struct {
		platformName string
		err          error
		wantMessage  string
	}{
		{
			platformName: "tiktok",
			err:          errors.New("missing scope: user.info.stats"),
			wantMessage:  "Reconnect TikTok to enable analytics.",
		},
		{
			platformName: "instagram",
			err:          errors.New("OAuthException: missing permission"),
			wantMessage:  "Reconnect Instagram to enable analytics.",
		},
		{
			platformName: "threads",
			err:          errors.New("session has expired"),
			wantMessage:  "Reconnect Threads to enable analytics.",
		},
	}

	for _, tc := range tests {
		status, code, message, ok := accountMetricsPlatformErrorResponse(tc.platformName, tc.err)
		if !ok {
			t.Fatalf("%s: expected reconnect response", tc.platformName)
		}
		if status != http.StatusConflict || code != "NEEDS_RECONNECT" {
			t.Fatalf("%s: response = %d/%s, want 409/NEEDS_RECONNECT", tc.platformName, status, code)
		}
		if message != tc.wantMessage {
			t.Fatalf("%s: message = %q, want %q", tc.platformName, message, tc.wantMessage)
		}
	}
}

func TestAccountMetricsPlatformErrorResponseIgnoresUpstreamErrors(t *testing.T) {
	_, _, _, ok := accountMetricsPlatformErrorResponse("youtube", errors.New("quota exceeded"))

	if ok {
		t.Fatal("expected ordinary upstream error to fall through")
	}
}

func TestAccountMetricsRefreshTokenForUpdateKeepsExistingWhenProviderDoesNotRotate(t *testing.T) {
	existing := pgtype.Text{String: "encrypted-old-refresh", Valid: true}

	got := accountMetricsRefreshTokenForUpdate(existing, "")
	if got != existing {
		t.Fatalf("refresh token = %#v, want existing %#v", got, existing)
	}

	got = accountMetricsRefreshTokenForUpdate(existing, "encrypted-new-refresh")
	want := pgtype.Text{String: "encrypted-new-refresh", Valid: true}
	if got != want {
		t.Fatalf("refresh token = %#v, want %#v", got, want)
	}
}
