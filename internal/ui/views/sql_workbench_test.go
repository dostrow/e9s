package views

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
	if _, found := m.SelectedResult(); found {
		t.Fatal("result from first connection leaked into second tab")
	}
	m.Switch(-1)
	result, found := m.SelectedResult()
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

func TestSQLWorkbenchShowsEveryResultSetOfARun(t *testing.T) {
	m := NewSQLWorkbench([]sqlworkbench.TabState{{ID: "one", ProfileName: "primary"}}, "one").SetSize(120, 40)
	m.SetRunFor("one", sqlworkbench.RunOutcome{Statements: 3, Results: []model.SQLQueryResult{
		{SQL: "SELECT 'first'", Columns: []string{"label"}, Rows: [][]string{{"first"}}, CommandTag: "SELECT 1"},
		{SQL: "UPDATE t SET x = 1", CommandTag: "UPDATE 4"},
		{SQL: "SELECT 'last'", Columns: []string{"label"}, Rows: [][]string{{"last"}}, CommandTag: "SELECT 1"},
	}})
	if result, _ := m.SelectedResult(); result.Rows[0][0] != "last" {
		t.Fatalf("a run should open on its last result, got %#v", result)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})
	if result, _ := m.SelectedResult(); result.CommandTag != "UPDATE 4" || !strings.Contains(m.View(), "no result set") {
		t.Fatalf("< should select the UPDATE, got %#v", result)
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("<")})
	if result, _ := m.SelectedResult(); result.Rows[0][0] != "first" || !strings.Contains(m.View(), "1/3  SELECT 1  SELECT 'first'") {
		t.Fatalf("< should stop at the first result, got %#v\n%s", result, m.View())
	}
}

func TestSQLWorkbenchOpensAFailedRunOnTheFailure(t *testing.T) {
	m := NewSQLWorkbench([]sqlworkbench.TabState{{ID: "one", ProfileName: "primary"}}, "one").SetSize(120, 40)
	m.SetRunFor("one", sqlworkbench.RunOutcome{Statements: 4,
		Results: []model.SQLQueryResult{{SQL: "BEGIN", CommandTag: "BEGIN"}, {SQL: "SELECT 1", Columns: []string{"n"}, Rows: [][]string{{"1"}}}},
		Failure: &sqlworkbench.StatementFailure{Index: 2, SQL: "SELECT 1/0", Message: "ERROR: division by zero (SQLSTATE 22012)", Line: 7},
		Events:  []string{"The transaction failed, so e9s rolled it back."}})
	if _, found := m.SelectedResult(); found {
		t.Fatal("a failed run should open on the failed statement")
	}
	view := m.View()
	for _, want := range []string{"Statement 3 failed at line 7", "division by zero", "SELECT 1/0", "rolled it back", "Failed at statement 3 of 4"} {
		if !strings.Contains(view, want) {
			t.Fatalf("view lacks %q:\n%s", want, view)
		}
	}
	m.MoveResult(-1)
	if result, found := m.SelectedResult(); !found || result.Rows[0][0] != "1" {
		t.Fatalf("results before the failure are not reachable: %#v", result)
	}
}

func TestSQLWorkbenchShowsAnOpenTransaction(t *testing.T) {
	m := NewSQLWorkbench([]sqlworkbench.TabState{{ID: "one", ProfileName: "primary"}, {ID: "two", ProfileName: "primary"}}, "one").SetSize(140, 40)
	m.SetSessionFor("one", sqlworkbench.SessionStatus{Connected: true, State: sqlworkbench.TxOpen, IdleTimeout: 5 * time.Minute})
	if view := m.View(); !strings.Contains(view, "TRANSACTION OPEN") || !strings.Contains(view, "[C] commit") {
		t.Fatalf("open transaction not shown:\n%s", view)
	}
	m.Switch(1)
	if strings.Contains(m.View(), "TRANSACTION OPEN") {
		t.Fatal("the transaction indicator leaked into another tab")
	}
}
