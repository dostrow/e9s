package sqlworkbench

import (
	"context"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

type fakeDataAPI struct {
	requests []model.SQLQueryRequest
}

func (f *fakeDataAPI) ExecuteSQLData(_ context.Context, request model.SQLQueryRequest) (model.SQLQueryResult, error) {
	f.requests = append(f.requests, request)
	return model.SQLQueryResult{Columns: []string{"value"}, Rows: [][]string{{"1"}}}, nil
}

func TestExecutorDataAPIReadOnlyAndStatementSplitting(t *testing.T) {
	api := &fakeDataAPI{}
	executor := NewExecutor(ExecutorOptions{DataAPI: api})
	profile := config.SQLConnection{Name: "analytics", Database: "app", Auth: "data-api", ResourceARN: "cluster", SecretARN: "secret"}
	results, err := executor.Execute(context.Background(), profile, "select 1; select 2", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || len(api.requests) != 2 || api.requests[1].SQL != "select 2" {
		t.Fatalf("unexpected execution: results=%#v requests=%#v", results, api.requests)
	}
}

func TestExecutorRequiresBothWriteGuards(t *testing.T) {
	profile := config.SQLConnection{Name: "analytics", Database: "app", Auth: "data-api", ResourceARN: "cluster", SecretARN: "secret"}
	for _, test := range []struct {
		global, tab bool
	}{
		{false, false}, {true, false}, {false, true},
	} {
		executor := NewExecutor(ExecutorOptions{DataAPI: &fakeDataAPI{}, AllowWrites: test.global})
		_, err := executor.Execute(context.Background(), profile, "delete from records", test.tab)
		if err == nil || !strings.Contains(err.Error(), "write") {
			t.Fatalf("global=%v tab=%v error=%v", test.global, test.tab, err)
		}
	}
	api := &fakeDataAPI{}
	executor := NewExecutor(ExecutorOptions{DataAPI: api, AllowWrites: true})
	if _, err := executor.Execute(context.Background(), profile, "delete from records", true); err != nil {
		t.Fatal(err)
	}
}
