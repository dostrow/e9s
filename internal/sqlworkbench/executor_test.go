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

func TestConnectionKeyIncludesTLSRootCertificate(t *testing.T) {
	base := ResolvedConnection{Host: "db", Port: 5432, Database: "app", User: "reader", SSLMode: "verify-full", AuthMode: "password"}
	withCertificate := base
	withCertificate.SSLRootCert = "/certificates/rds-ca.pem"
	if connectionKey(base) == connectionKey(withCertificate) {
		t.Fatal("connection key did not change with the TLS root certificate")
	}
}

func TestSQLValueStringFormatsPostgreSQLUUID(t *testing.T) {
	value := [16]byte{0xdf, 0x9c, 0xaa, 0x78, 0xd7, 0xdb, 0x47, 0x6a, 0x88, 0x67, 0x67, 0x1c, 0xd7, 0x75, 0x3c, 0x2d}
	if got, want := SQLValueString(value), "df9caa78-d7db-476a-8867-671cd7753c2d"; got != want {
		t.Fatalf("SQLValueString(UUID) = %q, want %q", got, want)
	}
}
