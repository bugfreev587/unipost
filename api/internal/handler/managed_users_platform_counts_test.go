package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

// The App Users list under-reported connected platforms: the aggregation and
// the response mapping only knew twitter/linkedin/bluesky/youtube, so an App
// User owning a TikTok account reported account_count = 2 while the dashboard
// rendered a single platform icon. These tests pin the full supported Connect
// platform contract so the omission cannot come back silently.

// managedUserPlatformCountsFixture is one connected account per supported
// platform except twitter, which carries two so the test also proves the
// mapping does not collapse repeated platforms.
var managedUserPlatformCountsFixture = db.ListManagedUsersByProfileRow{
	ExternalUserID:    "sdk-inbox-x",
	ExternalUserEmail: "dev@example.com",
	AccountCount:      10,
	TwitterCount:      2,
	LinkedinCount:     1,
	BlueskyCount:      1,
	YoutubeCount:      1,
	TiktokCount:       1,
	InstagramCount:    1,
	ThreadsCount:      1,
	FacebookCount:     1,
	PinterestCount:    1,
	FirstConnectedAt:  pgtype.Timestamptz{Time: time.Unix(1_754_000_000, 0).UTC(), Valid: true},
}

// TestManagedUserPlatformsMatchesConnectSupportedSet keeps the aggregate
// contract aligned with the platforms Connect can actually onboard.
func TestManagedUserPlatformsMatchesConnectSupportedSet(t *testing.T) {
	// Same set as the connect_sessions platform CHECK constraint (migration 074).
	want := []string{
		"twitter", "linkedin", "bluesky", "youtube", "tiktok",
		"instagram", "threads", "facebook", "pinterest",
	}
	if !reflect.DeepEqual(managedUserPlatforms, want) {
		t.Fatalf("managedUserPlatforms = %v, want %v", managedUserPlatforms, want)
	}
}

// TestManagedUserPlatformCountsCoversEverySupportedPlatform proves each of the
// nine platforms maps to its own aggregate column rather than to a shared or
// missing one.
func TestManagedUserPlatformCountsCoversEverySupportedPlatform(t *testing.T) {
	counts := managedUserPlatformCounts(managedUserPlatformCountsFixture)

	want := map[string]int{
		"twitter":   2,
		"linkedin":  1,
		"bluesky":   1,
		"youtube":   1,
		"tiktok":    1,
		"instagram": 1,
		"threads":   1,
		"facebook":  1,
		"pinterest": 1,
	}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("platform counts = %v, want %v", counts, want)
	}
}

// TestManagedUserPlatformCountsSumMatchesAccountCount is the regression that
// would have caught the original defect: the per-platform breakdown must
// account for every row COUNT(*) reported.
func TestManagedUserPlatformCountsSumMatchesAccountCount(t *testing.T) {
	counts := managedUserPlatformCounts(managedUserPlatformCountsFixture)

	sum := 0
	for _, platform := range managedUserPlatforms {
		sum += counts[platform]
	}
	if want := int(managedUserPlatformCountsFixture.AccountCount); sum != want {
		t.Fatalf("platform count sum = %d, want account_count %d", sum, want)
	}
}

// TestManagedUserPlatformCountsIncludesZeroValues pins that unconnected
// platforms are present as explicit zeros, so a client can render from the
// complete key set instead of guessing which keys exist.
func TestManagedUserPlatformCountsIncludesZeroValues(t *testing.T) {
	counts := managedUserPlatformCounts(db.ListManagedUsersByProfileRow{
		ExternalUserID: "sdk-inbox-x",
		AccountCount:   1,
		TiktokCount:    1,
	})

	for _, platform := range managedUserPlatforms {
		count, ok := counts[platform]
		if !ok {
			t.Fatalf("platform_counts is missing key %q", platform)
		}
		if platform != "tiktok" && count != 0 {
			t.Fatalf("platform_counts[%q] = %d, want 0", platform, count)
		}
	}
	if counts["tiktok"] != 1 {
		t.Fatalf("platform_counts[tiktok] = %d, want 1", counts["tiktok"])
	}
}

// TestManagedUsersListResponseCarriesEveryPlatformCount runs the real List
// handler so the assertion covers the serialized JSON contract the dashboard
// consumes, not just the internal mapping helper.
func TestManagedUsersListResponseCarriesEveryPlatformCount(t *testing.T) {
	// The reproduced staging case: one App User, one TikTok plus one X account.
	store := &managedUsersListTestDB{
		rows: [][]any{{
			"sdk-inbox-x",
			"dev@example.com",
			int32(2), // account_count
			int32(1), // twitter
			int32(0), // linkedin
			int32(0), // bluesky
			int32(0), // youtube
			int32(1), // tiktok
			int32(0), // instagram
			int32(0), // threads
			int32(0), // facebook
			int32(0), // pinterest
			int32(0), // reconnect
			int32(0), // disconnected
			pgtype.Timestamptz{Time: time.Unix(1_754_000_000, 0).UTC(), Valid: true},
			pgtype.Timestamptz{},
		}},
		total: 1,
	}
	h := NewManagedUsersHandler(db.New(store))

	req := httptest.NewRequest(http.MethodGet, "/v1/profiles/pr_1/users", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_1"))
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("profileID", "pr_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}

	var body struct {
		Data []struct {
			ExternalUserID string         `json:"external_user_id"`
			AccountCount   int            `json:"account_count"`
			PlatformCounts map[string]int `json:"platform_counts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v (body %s)", err, rec.Body.String())
	}
	if len(body.Data) != 1 {
		t.Fatalf("data length = %d, want one App User row", len(body.Data))
	}

	entry := body.Data[0]
	if entry.ExternalUserID != "sdk-inbox-x" {
		t.Fatalf("external_user_id = %q, want sdk-inbox-x", entry.ExternalUserID)
	}
	if len(entry.PlatformCounts) != len(managedUserPlatforms) {
		t.Fatalf("platform_counts has %d keys, want %d: %v",
			len(entry.PlatformCounts), len(managedUserPlatforms), entry.PlatformCounts)
	}

	sum := 0
	for _, platform := range managedUserPlatforms {
		count, ok := entry.PlatformCounts[platform]
		if !ok {
			t.Fatalf("platform_counts is missing key %q: %v", platform, entry.PlatformCounts)
		}
		sum += count
	}
	if sum != entry.AccountCount {
		t.Fatalf("platform_counts sum = %d, want account_count %d", sum, entry.AccountCount)
	}
	// The defect surfaced exactly here: TikTok stayed at 0 while account_count was 2.
	if entry.PlatformCounts["tiktok"] != 1 {
		t.Fatalf("platform_counts[tiktok] = %d, want 1", entry.PlatformCounts["tiktok"])
	}
	if entry.PlatformCounts["twitter"] != 1 {
		t.Fatalf("platform_counts[twitter] = %d, want 1", entry.PlatformCounts["twitter"])
	}
}

// managedUsersListTestDB serves the two queries GET /v1/profiles/{id}/users
// issues: the profile ownership lookup and the managed-user aggregation.
type managedUsersListTestDB struct {
	rows  [][]any
	total int32
}

func (*managedUsersListTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *managedUsersListTestDB) Query(_ context.Context, query string, _ ...interface{}) (pgx.Rows, error) {
	if !containsQueryName(query, "ListManagedUsersByProfile") {
		return nil, fmt.Errorf("unexpected Query: %s", query)
	}
	return &managedUsersListRows{values: f.rows}, nil
}

func (f *managedUsersListTestDB) QueryRow(_ context.Context, query string, _ ...interface{}) pgx.Row {
	switch {
	case containsQueryName(query, "GetProfile"):
		now := pgtype.Timestamptz{Time: time.Unix(1_754_000_000, 0).UTC(), Valid: true}
		return scanRow{values: []any{
			"pr_1", "TailTales", now, now,
			pgtype.Text{}, pgtype.Text{}, pgtype.Text{},
			"ws_1", false, pgtype.Text{},
		}}
	case containsQueryName(query, "CountManagedUsersByProfile"):
		return scanRow{values: []any{f.total}}
	default:
		return scanRow{err: fmt.Errorf("unexpected QueryRow: %s", query)}
	}
}

func containsQueryName(query, name string) bool {
	return strings.Contains(query, "-- name: "+name)
}

type managedUsersListRows struct {
	values [][]any
	index  int
}

func (*managedUsersListRows) Close()                                       {}
func (*managedUsersListRows) Err() error                                   { return nil }
func (*managedUsersListRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (*managedUsersListRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (*managedUsersListRows) Conn() *pgx.Conn                              { return nil }
func (*managedUsersListRows) RawValues() [][]byte                          { return nil }

func (r *managedUsersListRows) Next() bool {
	if r.index >= len(r.values) {
		return false
	}
	r.index++
	return true
}

func (r *managedUsersListRows) Values() ([]interface{}, error) {
	return r.values[r.index-1], nil
}

func (r *managedUsersListRows) Scan(dest ...interface{}) error {
	if r.index == 0 || r.index > len(r.values) {
		return errors.New("Scan called without current row")
	}
	values := r.values[r.index-1]
	if len(dest) != len(values) {
		return fmt.Errorf("scan destination count %d != values count %d", len(dest), len(values))
	}
	for i := range dest {
		target := reflect.ValueOf(dest[i])
		if target.Kind() != reflect.Pointer || target.IsNil() {
			return fmt.Errorf("scan destination %d is not a pointer", i)
		}
		value := reflect.ValueOf(values[i])
		if !value.IsValid() || !value.Type().AssignableTo(target.Elem().Type()) {
			return fmt.Errorf("scan value %d type mismatch: %T -> %s", i, values[i], target.Elem().Type())
		}
		target.Elem().Set(value)
	}
	return nil
}
