package db

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestXInboxDMLatchContract(t *testing.T) {
	migration, err := os.ReadFile("migrations/120_x_inbox_dm_forbidden_latch.sql")
	if err != nil {
		t.Fatal(err)
	}
	migrationText := string(migration)
	for _, required := range []string{
		"ALTER TABLE x_inbox_delivery_resources",
		"ADD COLUMN IF NOT EXISTS dm_subscription_forbidden_fingerprint TEXT",
	} {
		if !strings.Contains(migrationText, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"ADD COLUMN dm_subscription_forbidden_fingerprint TEXT NOT NULL",
		"ADD COLUMN dm_subscription_forbidden_fingerprint TEXT DEFAULT",
		"UPDATE x_inbox_delivery_resources",
		"DELETE FROM x_inbox_delivery_resources",
	} {
		if strings.Contains(migrationText, forbidden) {
			t.Fatalf("migration must add a nullable column without rewriting data; found %q", forbidden)
		}
	}

	generated, err := os.ReadFile("x_inbox.sql.go")
	if err != nil {
		t.Fatal(err)
	}
	generatedText := string(generated)
	const latchColumn = "dm_subscription_forbidden_fingerprint"

	getQuery := generatedSQLConstant(t, generatedText, "getXInboxDeliveryResource")
	assertSQLFragmentContains(t, "GetXInboxDeliveryResource SELECT list", sqlBetween(
		t,
		getQuery,
		"SELECT ",
		" FROM x_inbox_delivery_resources",
	), latchColumn)

	upsertQuery := generatedSQLConstant(t, generatedText, "upsertXInboxDeliveryResource")
	assertSQLFragmentContains(t, "UpsertXInboxDeliveryResource INSERT columns", sqlBetween(
		t,
		upsertQuery,
		"INSERT INTO x_inbox_delivery_resources (\n",
		"\n)\nVALUES",
	), latchColumn)
	assertSQLFragmentContains(t, "UpsertXInboxDeliveryResource conflict update", sqlBetween(
		t,
		upsertQuery,
		"ON CONFLICT (social_account_id) DO UPDATE\nSET ",
		"\nRETURNING ",
	), latchColumn+" = EXCLUDED."+latchColumn)
	assertSQLFragmentContains(t, "UpsertXInboxDeliveryResource RETURNING list", sqlAfter(
		t,
		upsertQuery,
		"\nRETURNING ",
	), latchColumn)

	updateQuery := generatedSQLConstant(t, generatedText, "updateXInboxDeliveryResource")
	assertSQLFragmentContains(t, "UpdateXInboxDeliveryResource SET list", sqlBetween(
		t,
		updateQuery,
		"UPDATE x_inbox_delivery_resources\nSET ",
		"\nWHERE social_account_id",
	), latchColumn+" =")
	assertSQLFragmentContains(t, "UpdateXInboxDeliveryResource RETURNING list", sqlAfter(
		t,
		updateQuery,
		"\nRETURNING ",
	), latchColumn)
}

func generatedSQLConstant(t *testing.T, source, name string) string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "x_inbox.sql.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range file.Decls {
		constants, ok := declaration.(*ast.GenDecl)
		if !ok || constants.Tok != token.CONST {
			continue
		}
		for _, specification := range constants.Specs {
			valueSpec, ok := specification.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for index, identifier := range valueSpec.Names {
				if identifier.Name != name || index >= len(valueSpec.Values) {
					continue
				}
				literal, ok := valueSpec.Values[index].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Fatalf("generated constant %s is not a string literal", name)
				}
				value, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatal(err)
				}
				return value
			}
		}
	}
	t.Fatalf("generated SQL constant %s not found", name)
	return ""
}

func sqlBetween(t *testing.T, query, start, end string) string {
	t.Helper()
	afterStart := sqlAfter(t, query, start)
	endIndex := strings.Index(afterStart, end)
	if endIndex < 0 {
		t.Fatalf("SQL fragment end %q not found after %q", end, start)
	}
	return afterStart[:endIndex]
}

func sqlAfter(t *testing.T, query, marker string) string {
	t.Helper()
	markerIndex := strings.Index(query, marker)
	if markerIndex < 0 {
		t.Fatalf("SQL fragment marker %q not found", marker)
	}
	return query[markerIndex+len(marker):]
}

func assertSQLFragmentContains(t *testing.T, fragmentName, fragment, required string) {
	t.Helper()
	if !strings.Contains(fragment, required) {
		t.Fatalf("%s missing %q; fragment: %q", fragmentName, required, fragment)
	}
}
