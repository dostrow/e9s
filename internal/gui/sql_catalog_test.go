//go:build gui

package gui

import (
	"testing"

	"github.com/dostrow/e9s/internal/sqlworkbench"
)

func TestFilterSQLObjectRowsKeepsHierarchy(t *testing.T) {
	table := sqlworkbench.DatabaseObject{OID: "42", Schema: "public", Name: "orders", Kind: sqlworkbench.ObjectTable}
	column := sqlworkbench.ObjectColumn{Name: "customer_id", DataType: "uuid", Nullable: false}
	rows := []sqlObjectBrowserRow{
		{key: sqlSchemaKey("public"), kind: sqlObjectRowSchema, schema: "public", label: "public"},
		{key: sqlCategoryKey("public", sqlworkbench.ObjectTable), kind: sqlObjectRowCategory, schema: "public", category: sqlworkbench.ObjectTable, label: "Tables"},
		{key: sqlObjectKey(table), kind: sqlObjectRowObject, schema: "public", category: sqlworkbench.ObjectTable, object: table, label: "orders"},
		{key: sqlColumnKey(table, column), kind: sqlObjectRowColumn, schema: "public", category: sqlworkbench.ObjectTable, object: table, column: column, label: column.DisplayName()},
	}
	filtered := filterSQLObjectRows(rows, "customer_id")
	if len(filtered) != 4 {
		t.Fatalf("filtered row count = %d, want schema, category, object, and column", len(filtered))
	}
}

func TestSQLBrowserRowNames(t *testing.T) {
	table := sqlworkbench.DatabaseObject{OID: "42", Schema: "Sales Data", Name: "Order", Kind: sqlworkbench.ObjectTable}
	column := sqlworkbench.ObjectColumn{Name: "Customer ID"}
	tests := []struct {
		name      string
		row       sqlObjectBrowserRow
		plain     string
		qualified string
	}{
		{name: "table", row: sqlObjectBrowserRow{kind: sqlObjectRowObject, object: table}, plain: `"Order"`, qualified: `"Sales Data"."Order"`},
		{name: "column", row: sqlObjectBrowserRow{kind: sqlObjectRowColumn, object: table, column: column}, plain: `"Customer ID"`, qualified: `"Sales Data"."Order"."Customer ID"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plain, qualified, ok := sqlBrowserRowNames(test.row)
			if !ok || plain != test.plain || qualified != test.qualified {
				t.Fatalf("sqlBrowserRowNames() = %q, %q, %v; want %q, %q, true", plain, qualified, ok, test.plain, test.qualified)
			}
		})
	}
}

func TestSQLTreeLabel(t *testing.T) {
	if got, want := sqlTreeLabel(1, true, "Tables"), "    ▾ Tables"; got != want {
		t.Fatalf("sqlTreeLabel() = %q, want %q", got, want)
	}
}
