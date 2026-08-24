//go:build gui

package gui

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
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
		{name: "latest statement wins", results: []model.SQLQueryResult{{Columns: []string{"id"}}, {CommandTag: "UPDATE 1"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tab := &sqlWorkbenchTab{results: test.results}
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
