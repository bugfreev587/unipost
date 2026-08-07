package db

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// ListManagedUsersByProfile aggregates one count per supported Connect
// platform. It originally covered only twitter/linkedin/bluesky/youtube, so an
// App User with a TikTok account got account_count = 2 from COUNT(*) while the
// per-platform breakdown described a single account. These tests pin both the
// query source and the generated row type to the full supported set.

// managedUsersSupportedPlatforms mirrors the connect_sessions platform CHECK
// constraint in migration 074.
var managedUsersSupportedPlatforms = []string{
	"twitter", "linkedin", "bluesky", "youtube", "tiktok",
	"instagram", "threads", "facebook", "pinterest",
}

func TestListManagedUsersByProfileCountsEverySupportedPlatform(t *testing.T) {
	source, err := os.ReadFile("queries/managed_users.sql")
	if err != nil {
		t.Fatalf("read queries/managed_users.sql: %v", err)
	}
	query := managedUsersNamedQuery(t, string(source), "ListManagedUsersByProfile")

	for _, platform := range managedUsersSupportedPlatforms {
		filter := "COUNT(*) FILTER (WHERE platform = '" + platform + "')"
		if !strings.Contains(query, filter) {
			t.Errorf("ListManagedUsersByProfile does not aggregate %q; expected %s", platform, filter)
		}
		alias := "AS " + platform + "_count"
		if !strings.Contains(query, alias) {
			t.Errorf("ListManagedUsersByProfile does not expose %q; expected alias %s", platform, alias)
		}
	}
}

// TestListManagedUsersByProfileRowExposesEveryPlatformCount guards the sqlc
// output: a regenerated row type that drops a count field would break the
// handler contract even when the SQL is correct.
func TestListManagedUsersByProfileRowExposesEveryPlatformCount(t *testing.T) {
	rowType := reflect.TypeOf(ListManagedUsersByProfileRow{})

	for _, platform := range managedUsersSupportedPlatforms {
		// twitter -> TwitterCount, tiktok -> TiktokCount (sqlc's Go casing).
		name := strings.ToUpper(platform[:1]) + platform[1:] + "Count"
		field, ok := rowType.FieldByName(name)
		if !ok {
			t.Errorf("ListManagedUsersByProfileRow has no %s field for platform %q", name, platform)
			continue
		}
		if field.Type.Kind() != reflect.Int32 {
			t.Errorf("%s is %s, want int32", name, field.Type)
		}
		if want := `json:"` + platform + `_count"`; !strings.Contains(string(field.Tag), want) {
			t.Errorf("%s tag = %q, want it to contain %s", name, field.Tag, want)
		}
	}
}

// managedUsersNamedQuery extracts one `-- name: X` block from a sqlc query file.
func managedUsersNamedQuery(t *testing.T, source, name string) string {
	t.Helper()

	marker := "-- name: " + name + " "
	start := strings.Index(source, marker)
	if start < 0 {
		t.Fatalf("query %s is missing from queries/managed_users.sql", name)
	}
	rest := source[start:]
	if next := strings.Index(rest[len(marker):], "-- name: "); next >= 0 {
		rest = rest[:len(marker)+next]
	}
	return rest
}
