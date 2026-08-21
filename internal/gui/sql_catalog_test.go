//go:build gui

package gui

import (
	"testing"

	"github.com/dostrow/e9s/internal/sqlworkbench"
)

func TestFilterSQLObjectRowsKeepsHierarchy(t *testing.T) {
	table := sqlworkbench.DatabaseObject{OID: "42", Schema: "public", Name: "orders", Kind: sqlworkbench.ObjectTable}
	rows := []sqlObjectBrowserRow{
		{key: sqlSchemaKey("public"), kind: sqlObjectRowSchema, schema: "public", label: "public"},
		{key: sqlCategoryKey("public", sqlworkbench.ObjectTable), kind: sqlObjectRowCategory, schema: "public", category: sqlworkbench.ObjectTable, label: "Tables"},
		{key: sqlObjectKey(table), kind: sqlObjectRowObject, schema: "public", category: sqlworkbench.ObjectTable, object: table, label: "orders"},
	}
	filtered := filterSQLObjectRows(rows, "orders")
	if len(filtered) != 3 {
		t.Fatalf("filtered row count = %d, want schema, category, and object", len(filtered))
	}
}

func TestSQLTreeLabel(t *testing.T) {
	if got, want := sqlTreeLabel(1, true, "Tables"), "    ▾ Tables"; got != want {
		t.Fatalf("sqlTreeLabel() = %q, want %q", got, want)
	}
}
