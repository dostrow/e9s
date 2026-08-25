package views

import (
	"testing"

	"github.com/dostrow/e9s/internal/sqlworkbench"
)

func TestSQLCatalogLazilyExpandsColumnsAndBuildsInsertions(t *testing.T) {
	catalog := NewSQLCatalogModel()
	catalog.SetSchemas([]string{"Sales Data"}, nil)

	catalog.Activate()
	catalog.Move(1)
	request := catalog.Activate()
	if request.Kind != SQLCatalogRequestObjects || request.Schema != "Sales Data" || request.ObjectKind != sqlworkbench.ObjectTable {
		t.Fatalf("category request = %#v", request)
	}

	table := sqlworkbench.DatabaseObject{OID: "42", Schema: "Sales Data", Name: "Order", Kind: sqlworkbench.ObjectTable}
	catalog.SetObjects("Sales Data", sqlworkbench.ObjectTable, []sqlworkbench.DatabaseObject{table}, nil)
	catalog.Move(1)
	request = catalog.Activate()
	if request.Kind != SQLCatalogRequestColumns || request.Object != table {
		t.Fatalf("column request = %#v", request)
	}

	column := sqlworkbench.ObjectColumn{Name: "Customer ID", DataType: "uuid"}
	catalog.SetColumns(table, []sqlworkbench.ObjectColumn{column}, nil)
	catalog.Move(1)
	plain, ok := catalog.Insertion(false)
	if !ok || plain != `"Customer ID"` {
		t.Fatalf("plain insertion = %q, %v", plain, ok)
	}
	qualified, ok := catalog.Insertion(true)
	if !ok || qualified != `"Sales Data"."Order"."Customer ID"` {
		t.Fatalf("qualified insertion = %q, %v", qualified, ok)
	}
}

func TestSQLCatalogLoadingRowsAreNotExpandable(t *testing.T) {
	catalog := NewSQLCatalogModel()
	catalog.SetSchemas([]string{"public"}, nil)
	catalog.Activate()
	catalog.Move(1)
	request := catalog.Activate()
	if request.Kind != SQLCatalogRequestObjects {
		t.Fatalf("request kind = %v", request.Kind)
	}
	catalog.Move(1)
	if request := catalog.Activate(); request.Kind != SQLCatalogRequestNone {
		t.Fatalf("loading row activation returned %#v", request)
	}
}
