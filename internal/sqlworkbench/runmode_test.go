package sqlworkbench

import (
	"context"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

func TestPlanTransactionReplacesTheScriptsOuterTransaction(t *testing.T) {
	source := "-- header\nBEGIN ISOLATION LEVEL REPEATABLE READ;\nSELECT 1;\nSAVEPOINT a;\nROLLBACK TO SAVEPOINT a;\nROLLBACK;"
	plan, err := planTransaction(SplitStatements(source), source)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan.begin, "REPEATABLE READ") || len(plan.body) != 3 || plan.body[0].SQL != "SELECT 1" {
		t.Fatalf("plan = %#v", plan)
	}
	if event := plan.event(RunDryRun); !strings.Contains(event, "BEGIN and ROLLBACK") {
		t.Fatalf("event = %q", event)
	}
}

func TestPlanTransactionRefusesControlThatWouldEndItEarly(t *testing.T) {
	for _, source := range []string{
		"BEGIN;\nUPDATE t SET x = 1;\nCOMMIT;\nSELECT 1;",
		"SELECT 1;\nCOMMIT AND CHAIN;\nSELECT 2;",
		"SELECT 1;\nBEGIN;\nSELECT 2;",
	} {
		_, err := planTransaction(SplitStatements(source), source)
		if err == nil || !strings.Contains(err.Error(), "line ") {
			t.Fatalf("%q: error = %v", source, err)
		}
	}
	if _, err := planTransaction(SplitStatements("BEGIN; COMMIT"), "BEGIN; COMMIT"); err == nil {
		t.Fatal("an empty transaction should be refused")
	}
	if !endsTransaction("COMMIT AND NO CHAIN") || endsTransaction("COMMIT AND CHAIN") || endsTransaction("ROLLBACK TO a") {
		t.Fatal("endsTransaction misread AND [NO] CHAIN or ROLLBACK TO")
	}
}

func TestFormatReportShowsEveryStatementFailureAndEvent(t *testing.T) {
	outcome := RunOutcome{Mode: RunDryRun, Statements: 3,
		Results: []model.SQLQueryResult{
			{SQL: "SELECT 'Study links to restore' AS section", Line: 4, Columns: []string{"section"}, Rows: [][]string{{"Study links to restore"}}, CommandTag: "SELECT 1"},
			{SQL: "UPDATE studies SET x = 1", Line: 6, CommandTag: "UPDATE 6", RowsAffected: 6},
		},
		Failure: &StatementFailure{Index: 2, SQL: "SELECT 1/0", Line: 9, Message: "ERROR: division by zero (SQLSTATE 22012)"},
		Events:  []string{"The transaction failed, so e9s rolled it back."},
	}
	report := FormatReport(outcome, ReportContext{Connection: "MDDS Prod", Database: "dicomweb", Tab: "restore"})
	for _, want := range []string{
		"-- Connection: MDDS Prod (database dicomweb)",
		"-- Mode: dry run",
		"-- Changes: line 6: UPDATE 6",
		"-- [1/3] line 4\nSELECT 'Study links to restore' AS section;\n section\n------------------------\n Study links to restore\n(1 row)",
		"-- [2/3] line 6\nUPDATE studies SET x = 1;\nUPDATE 6",
		"-- [3/3] line 9\nSELECT 1/0;\nERROR: division by zero",
		"-- e9s: The transaction failed",
	} {
		if !strings.Contains(report, want) {
			t.Fatalf("report lacks %q:\n%s", want, report)
		}
	}
}

func TestChangeSummaryCountsDataAPIRowsAffected(t *testing.T) {
	outcome := RunOutcome{Results: []model.SQLQueryResult{
		{CommandTag: "SELECT 2", Columns: []string{"a"}},
		{CommandTag: "3 rows affected", RowsAffected: 3, Line: 2},
		{CommandTag: "0 rows affected"},
		{CommandTag: "UPDATE 0", Line: 4},
	}}
	if got := outcome.ChangeSummary(); got != "line 2: 3 rows affected" {
		t.Fatalf("ChangeSummary() = %q", got)
	}
}

type fakeTransactionalDataAPI struct {
	fakeDataAPI
	begun, ended []string
	commits      []bool
}

func (f *fakeTransactionalDataAPI) BeginSQLDataTransaction(context.Context, model.SQLQueryRequest) (string, error) {
	id := "tx" + string(rune('1'+len(f.begun)))
	f.begun = append(f.begun, id)
	return id, nil
}

func (f *fakeTransactionalDataAPI) EndSQLDataTransaction(_ context.Context, request model.SQLQueryRequest, commit bool) (string, error) {
	f.ended = append(f.ended, request.TransactionID)
	f.commits = append(f.commits, commit)
	return "Transaction Ended", nil
}

func TestDataAPIDryRunUsesAndRollsBackATransaction(t *testing.T) {
	api := &fakeTransactionalDataAPI{}
	executor := NewExecutor(ExecutorOptions{DataAPI: api, AllowWrites: true})
	profile := config.SQLConnection{Name: "analytics", Database: "app", Auth: "data-api", ResourceARN: "cluster", SecretARN: "secret"}
	outcome, err := executor.Run(context.Background(), RunRequest{Profile: profile, SessionID: "tab", AllowWrites: true, Mode: RunDryRun,
		Source: "BEGIN; UPDATE t SET x = 1; ROLLBACK;"})
	if err != nil {
		t.Fatal(err)
	}
	if len(api.requests) != 1 || api.requests[0].TransactionID != "tx1" || api.requests[0].SQL != "UPDATE t SET x = 1" {
		t.Fatalf("requests = %#v", api.requests)
	}
	if len(api.ended) != 1 || api.commits[0] || outcome.Transaction != TxIdle {
		t.Fatalf("dry run was not rolled back: ended=%v commits=%v outcome=%#v", api.ended, api.commits, outcome)
	}
}

func TestDataAPIRunInTransactionStaysOpenUntilCommitted(t *testing.T) {
	api := &fakeTransactionalDataAPI{}
	executor := NewExecutor(ExecutorOptions{DataAPI: api, AllowWrites: true})
	profile := config.SQLConnection{Name: "analytics", Database: "app", Auth: "data-api", ResourceARN: "cluster", SecretARN: "secret"}
	request := RunRequest{Profile: profile, SessionID: "tab", AllowWrites: true, Mode: RunInTransaction, Source: "UPDATE t SET x = 1"}
	if outcome, err := executor.Run(context.Background(), request); err != nil || outcome.Transaction != TxOpen {
		t.Fatalf("outcome = %#v, err = %v", outcome, err)
	}
	if status := executor.SessionStatus("tab"); status.State != TxOpen || status.IdleTimeout != DataAPITransactionTimeout {
		t.Fatalf("status = %#v", status)
	}
	if ids := executor.OpenTransactions(); len(ids) != 1 {
		t.Fatalf("OpenTransactions() = %v", ids)
	}
	if _, err := executor.Run(context.Background(), request); err == nil {
		t.Fatal("a second transactional run started while one was open")
	}
	// Plain runs join the open transaction.
	request.Mode, request.Source = RunAsWritten, "SELECT 1"
	if _, err := executor.Run(context.Background(), request); err != nil || api.requests[len(api.requests)-1].TransactionID != "tx1" {
		t.Fatalf("plain run did not join the transaction: %v %#v", err, api.requests)
	}
	if _, err := executor.EndTransaction("tab", true, true); err != nil || !api.commits[0] {
		t.Fatalf("commit: %v %v", err, api.commits)
	}
	if executor.SessionStatus("tab").State != TxIdle {
		t.Fatal("transaction still shown as open after commit")
	}
}

func TestClosingTheExecutorRollsBackDataAPITransactions(t *testing.T) {
	api := &fakeTransactionalDataAPI{}
	executor := NewExecutor(ExecutorOptions{DataAPI: api, AllowWrites: true})
	profile := config.SQLConnection{Name: "analytics", Database: "app", Auth: "data-api", ResourceARN: "cluster", SecretARN: "secret"}
	if _, err := executor.Run(context.Background(), RunRequest{Profile: profile, SessionID: "tab", AllowWrites: true, Mode: RunInTransaction, Source: "UPDATE t SET x = 1"}); err != nil {
		t.Fatal(err)
	}
	executor.Close()
	if len(api.ended) != 1 || api.commits[0] {
		t.Fatalf("Close did not roll back: ended=%v commits=%v", api.ended, api.commits)
	}
}

func TestSummaryAndChangesForEdgeCases(t *testing.T) {
	early := RunOutcome{Statements: 3, Failure: &StatementFailure{Index: -1, Message: "start the transaction: boom"}}
	if got := early.Summary(); !strings.HasPrefix(got, "Failed before the first statement ran") || early.EntryLabel(0) != "ERROR before the first statement" {
		t.Fatalf("summary = %q, label = %q", got, early.EntryLabel(0))
	}
	created := RunOutcome{Results: []model.SQLQueryResult{
		{CommandTag: "SELECT 12", RowsAffected: 12, Line: 3},
		{CommandTag: "SELECT 4", RowsAffected: 4, Columns: []string{"n"}},
	}}
	if got := created.ChangeSummary(); got != "line 3: SELECT 12" {
		t.Fatalf("ChangeSummary() = %q, want only the CREATE TABLE AS", got)
	}
}
