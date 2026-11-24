//go:build gui

package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/sqlworkbench"
)

func TestSQLProfilePresentationDefaults(t *testing.T) {
	profile := config.SQLConnection{Name: "prod", Host: "db.example", Database: "app"}
	if got := sqlProfileResource(profile); got != "db.example:5432" {
		t.Fatalf("resource = %q", got)
	}
	if got := sqlAuthMode(profile); got != "pgpass" {
		t.Fatalf("auth = %q", got)
	}
}

func TestSQLTabExportRequiresResultSchema(t *testing.T) {
	tests := []struct {
		name    string
		results []model.SQLQueryResult
		want    bool
	}{
		{name: "none"},
		{name: "command only", results: []model.SQLQueryResult{{CommandTag: "UPDATE 1"}}},
		{name: "empty select", results: []model.SQLQueryResult{{Columns: []string{"id"}}}, want: true},
		{name: "selected statement wins", results: []model.SQLQueryResult{{Columns: []string{"id"}}, {CommandTag: "UPDATE 1"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tab := &sqlWorkbenchTab{run: sqlworkbench.RunOutcome{Results: test.results}, selectedResult: len(test.results) - 1}
			if got := sqlTabHasExportableResult(tab); got != test.want {
				t.Fatalf("sqlTabHasExportableResult() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSQLResultCellReturnsOriginalValue(t *testing.T) {
	result := model.SQLQueryResult{
		Columns: []string{"id", "payload"},
		Rows:    [][]string{{"7", "first line\nsecond\tline"}},
	}
	column, value, ok := sqlResultCell(result, 0, 1)
	if !ok {
		t.Fatal("sqlResultCell rejected a valid coordinate")
	}
	if column != "payload" {
		t.Fatalf("column = %q, want payload", column)
	}
	if value != "first line\nsecond\tline" {
		t.Fatalf("value = %q, want the unmodified cell contents", value)
	}
}

func TestSQLResultCellRejectsInvalidCoordinates(t *testing.T) {
	result := model.SQLQueryResult{Columns: []string{"id"}, Rows: [][]string{{"7"}}}
	for _, coordinate := range [][2]int{{-1, 0}, {0, -1}, {1, 0}, {0, 1}} {
		if _, _, ok := sqlResultCell(result, coordinate[0], coordinate[1]); ok {
			t.Fatalf("sqlResultCell accepted coordinate row=%d field=%d", coordinate[0], coordinate[1])
		}
	}
}

func TestFormatSQLResultRowsUsesTSVAndOriginalValues(t *testing.T) {
	result := model.SQLQueryResult{
		Columns: []string{"id", "payload"},
		Rows: [][]string{
			{"1", "plain"},
			{"2", "first line\nsecond\tline"},
			{"3", "ignored"},
		},
	}
	got, rows, err := formatSQLResultRows(result, []int{1, 0}, true)
	if err != nil {
		t.Fatalf("formatSQLResultRows: %v", err)
	}
	if rows != 2 {
		t.Fatalf("rows = %d, want 2", rows)
	}
	for _, want := range []string{"id\tpayload", "2\t\"first line\nsecond\tline\"", "1\tplain"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatted rows missing %q:\n%s", want, got)
		}
	}
}

func TestFormatSQLResultRowsSkipsInvalidPositions(t *testing.T) {
	result := model.SQLQueryResult{Columns: []string{"id"}, Rows: [][]string{{"7"}}}
	got, rows, err := formatSQLResultRows(result, []int{-1, 1, 0}, false)
	if err != nil {
		t.Fatalf("formatSQLResultRows: %v", err)
	}
	if rows != 1 || got != "7" {
		t.Fatalf("formatSQLResultRows() = %q, %d rows; want 7, 1 row", got, rows)
	}
}

func TestSQLFailureTextNamesTheStatementAndLine(t *testing.T) {
	text := sqlFailureText(sqlworkbench.StatementFailure{Index: 6, Line: 112, Message: "ERROR: boom (SQLSTATE XX000)\nHINT: try again",
		SQL: strings.TrimSuffix(strings.Repeat("SELECT 1\n", 20), "\n")})
	for _, want := range []string{"Statement 7 failed at line 112", "HINT: try again", "… 8 more line(s)"} {
		if !strings.Contains(text, want) {
			t.Fatalf("failure text lacks %q:\n%s", want, text)
		}
	}
}

func TestSQLTabSelectedResultExcludesTheFailure(t *testing.T) {
	tab := &sqlWorkbenchTab{run: sqlworkbench.RunOutcome{
		Results: []model.SQLQueryResult{{Columns: []string{"id"}}},
		Failure: &sqlworkbench.StatementFailure{Index: 1, Message: "ERROR"},
	}, selectedResult: 1}
	if _, ok := sqlTabSelectedResult(tab); ok {
		t.Fatal("the failed statement has no result set")
	}
	tab.selectedResult = 0
	if _, ok := sqlTabSelectedResult(tab); !ok {
		t.Fatal("the earlier result should be selectable")
	}
}

func TestChooseSQLBrowserTabPrefersTheActiveThenTheMostRecentTab(t *testing.T) {
	now := time.Now()
	tabs := []sqlBrowserCandidate{
		{profile: "prod", lastUsed: now.Add(-3 * time.Minute)},
		{profile: "staging", lastUsed: now},
		{profile: "prod", lastUsed: now.Add(-time.Minute)},
		{profile: "prod", lastUsed: now.Add(-2 * time.Minute)},
	}
	for _, test := range []struct {
		name    string
		active  int
		profile string
		want    int
	}{
		{"the active tab when it is on the connection", 3, "prod", 3},
		{"the connection's most recently used tab otherwise", 1, "prod", 2},
		{"the most recent tab when no tab is active", -1, "prod", 2},
		{"the only tab on the connection", 0, "staging", 1},
		{"none when the connection has no tab", 0, "analytics", -1},
	} {
		if got := chooseSQLBrowserTab(tabs, test.active, test.profile); got != test.want {
			t.Errorf("%s: got %d, want %d", test.name, got, test.want)
		}
	}
	if got := chooseSQLBrowserTab(nil, 0, "prod"); got != -1 {
		t.Errorf("no tabs: got %d", got)
	}
}

func TestContextMenuKeys(t *testing.T) {
	if !isContextMenuKey(gdk.KEY_Menu, 0) || !isContextMenuKey(gdk.KEY_F10, gdk.ShiftMask) {
		t.Fatal("Menu and Shift+F10 should open a context menu")
	}
	if isContextMenuKey(gdk.KEY_F10, 0) || isContextMenuKey(gdk.KEY_Return, gdk.ShiftMask) {
		t.Fatal("F10 alone or Shift+Return should not open a context menu")
	}
}
