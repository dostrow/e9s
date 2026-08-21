//go:build gui

package gui

import (
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
