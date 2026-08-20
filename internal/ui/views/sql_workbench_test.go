package views

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/sqlworkbench"
)

func TestSQLWorkbenchTabsKeepConnectionScopedQueriesAndResults(t *testing.T) {
	m := NewSQLWorkbench([]sqlworkbench.TabState{
		{ID: "one", ProfileName: "primary", Query: "select 1"},
		{ID: "two", ProfileName: "analytics", Query: "select 2"},
	}, "one").SetSize(120, 40)
	m.SetResults([]model.SQLQueryResult{{Columns: []string{"value"}, Rows: [][]string{{"1"}}}})
	m.Switch(1)
	if got := m.Query(); got != "select 2" {
		t.Fatalf("second tab query = %q", got)
	}
	if _, found := m.LastResult(); found {
		t.Fatal("result from first connection leaked into second tab")
	}
	m.Switch(-1)
	result, found := m.LastResult()
	if !found || result.Rows[0][0] != "1" {
		t.Fatal("first tab result was not retained")
	}
}

func TestSQLWorkbenchAlwaysRestoresWriteLock(t *testing.T) {
	m := NewSQLWorkbench([]sqlworkbench.TabState{{ID: "one", ProfileName: "primary", AllowWrites: true}}, "one")
	tab, found := m.ActiveTabValue()
	if !found || tab.AllowWrites {
		t.Fatal("restored SQL tab must start read-only")
	}
}

func TestSQLWorkbenchCursorOffsetSelectsCurrentStatement(t *testing.T) {
	m := NewSQLWorkbench([]sqlworkbench.TabState{{ID: "one", ProfileName: "primary", Query: "select 1;\nselect 2"}}, "one")
	statement, found := sqlworkbench.CurrentStatement(m.Query(), m.CursorByteOffset())
	if !found || !strings.Contains(statement.SQL, "select 2") {
		t.Fatalf("current statement = %#v, found=%t", statement, found)
	}
}
