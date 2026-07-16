package db

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestExpandableTablesDoNotUseWildcardProjections(t *testing.T) {
	files, err := filepath.Glob("queries/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	targetTables := `(oauth_states|connect_sessions|social_accounts|platform_credentials)`
	wildcards := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bSELECT\s+(?:DISTINCT\s+)?(?:[a-z_][a-z0-9_]*\.)?\*\s+FROM\s+` + targetTables + `\b`),
		regexp.MustCompile(`(?is)\b(?:INSERT\s+INTO|UPDATE|DELETE\s+FROM)\s+` + targetTables + `\b[^;]*?\bRETURNING\s+\*`),
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, wildcard := range wildcards {
			if match := wildcard.Find(data); match != nil {
				t.Errorf("%s contains schema-expansion-unsafe projection %q", file, match)
			}
		}
	}
}
