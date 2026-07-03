package handler

import (
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestYouTubeAnalyticsHasRequiredScope(t *testing.T) {
	if !youtubeAnalyticsHasRequiredScope([]string{
		"https://www.googleapis.com/auth/youtube.readonly",
		"https://www.googleapis.com/auth/yt-analytics.readonly",
	}) {
		t.Fatal("expected yt-analytics.readonly scope to pass")
	}
	if youtubeAnalyticsHasRequiredScope([]string{"https://www.googleapis.com/auth/youtube.readonly"}) {
		t.Fatal("expected missing analytics scope to fail")
	}
}

func TestParseYouTubeAnalyticsRangeDefaultsToLast28CompleteDays(t *testing.T) {
	now := time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)

	got, err := parseYouTubeAnalyticsRange(url.Values{}, now)
	if err != nil {
		t.Fatalf("parseYouTubeAnalyticsRange: %v", err)
	}

	if got.StartDate != "2026-06-05" || got.EndDate != "2026-07-02" {
		t.Fatalf("range = %#v, want 2026-06-05..2026-07-02", got)
	}
}

func TestParseYouTubeAnalyticsRangeAcceptsAliases(t *testing.T) {
	values := url.Values{}
	values.Set("from", "2026-07-01")
	values.Set("to", "2026-07-28")

	got, err := parseYouTubeAnalyticsRange(values, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("parseYouTubeAnalyticsRange: %v", err)
	}

	if got.StartDate != "2026-07-01" || got.EndDate != "2026-07-28" {
		t.Fatalf("range = %#v, want explicit from/to dates", got)
	}
}

func TestParseYouTubeAnalyticsRangeRejectsInvalidDates(t *testing.T) {
	values := url.Values{}
	values.Set("start_date", "2026-07-28")
	values.Set("end_date", "2026-07-01")

	_, err := parseYouTubeAnalyticsRange(values, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if err == nil {
		t.Fatal("expected invalid range error")
	}
}

func TestYouTubeAnalyticsErrorResponse(t *testing.T) {
	status, code, message, ok := youtubeAnalyticsErrorResponse(
		errors.Join(errors.New("youtube analytics rejected credentials"), platform.ErrNeedsReconnect),
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

	if _, _, _, ok := youtubeAnalyticsErrorResponse(errors.New("quota exceeded")); ok {
		t.Fatal("quota errors should fall through to upstream response")
	}
}

func TestYouTubeAnalyticsRoutesAreRegistered(t *testing.T) {
	source, err := os.ReadFile("../../cmd/api/main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	body := string(source)

	for _, want := range []string{
		`r.Get("/v1/accounts/{id}/youtube/analytics/summary", socialAccountHandler.YouTubeAnalyticsSummary)`,
		`r.Get("/v1/accounts/{id}/youtube/analytics/trend", socialAccountHandler.YouTubeAnalyticsTrend)`,
		`r.Get("/v1/accounts/{id}/youtube/analytics/videos", socialAccountHandler.YouTubeAnalyticsVideos)`,
		`r.Get("/v1/profiles/{profileID}/accounts/{accountID}/youtube/analytics/summary", socialAccountHandler.YouTubeAnalyticsSummary)`,
		`r.Get("/v1/profiles/{profileID}/accounts/{accountID}/youtube/analytics/trend", socialAccountHandler.YouTubeAnalyticsTrend)`,
		`r.Get("/v1/profiles/{profileID}/accounts/{accountID}/youtube/analytics/videos", socialAccountHandler.YouTubeAnalyticsVideos)`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("YouTube analytics route missing %q", want)
		}
	}
}
