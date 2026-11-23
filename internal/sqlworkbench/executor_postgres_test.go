package sqlworkbench

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/config"
)

// These tests need a PostgreSQL server the test may create tables in, given
// as E9S_TEST_POSTGRES_URL (postgres://user:password@host:port/db?sslmode=disable).
// They are skipped without one; CI provides a server.

func postgresTestProfile(t *testing.T) (config.SQLConnection, ExecutorOptions) {
	t.Helper()
	raw := os.Getenv("E9S_TEST_POSTGRES_URL")
	if raw == "" {
		t.Skip("set E9S_TEST_POSTGRES_URL to run PostgreSQL session tests")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse E9S_TEST_POSTGRES_URL: %v", err)
	}
	port := 5432
	if value := parsed.Port(); value != "" {
		port, _ = strconv.Atoi(value)
	}
	password, _ := parsed.User.Password()
	if password == "" {
		password = "unused"
	}
	sslMode := parsed.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "disable"
	}
	profile := config.SQLConnection{Name: "test", Host: parsed.Hostname(), Port: port, Database: strings.TrimPrefix(parsed.Path, "/"),
		User: parsed.User.Username(), Auth: "password", SSLMode: sslMode}
	options := ExecutorOptions{AllowWrites: true, Prompt: func(context.Context, config.SQLConnection) (string, error) { return password, nil }}
	return profile, options
}

func newPostgresExecutor(t *testing.T) (*Executor, config.SQLConnection) {
	t.Helper()
	profile, options := postgresTestProfile(t)
	executor := NewExecutor(options)
	t.Cleanup(executor.Close)
	return executor, profile
}

// testTable returns a table name unique to the test and drops it afterwards.
func testTable(t *testing.T, executor *Executor, profile config.SQLConnection) string {
	t.Helper()
	name := fmt.Sprintf("e9s_test_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		_, _ = executor.Run(context.Background(), RunRequest{Profile: profile, SessionID: "cleanup", AllowWrites: true,
			Source: "DROP TABLE IF EXISTS " + name + " CASCADE"})
	})
	return name
}

func run(t *testing.T, executor *Executor, profile config.SQLConnection, tab, source string, writes bool) RunOutcome {
	t.Helper()
	outcome, err := executor.Run(context.Background(), RunRequest{Profile: profile, SessionID: tab, Source: source, AllowWrites: writes})
	if err != nil {
		t.Fatalf("Run(%q): %v", source, err)
	}
	return outcome
}

func scalar(t *testing.T, outcome RunOutcome) string {
	t.Helper()
	if outcome.Failure != nil {
		t.Fatalf("statement failed: %s", outcome.Failure.Message)
	}
	last := outcome.Results[len(outcome.Results)-1]
	if len(last.Rows) != 1 || len(last.Rows[0]) != 1 {
		t.Fatalf("want one value, got %#v", last.Rows)
	}
	return last.Rows[0][0]
}

func TestRunKeepsEachTabsTransactionInItsOwnSession(t *testing.T) {
	executor, profile := newPostgresExecutor(t)
	table := testTable(t, executor, profile)
	run(t, executor, profile, "setup", "CREATE TABLE "+table+" (value int)", true)

	opened := run(t, executor, profile, "one", "BEGIN; INSERT INTO "+table+" VALUES (1)", true)
	if opened.Transaction != TxOpen || executor.SessionStatus("one").State != TxOpen {
		t.Fatalf("tab one should hold an open transaction: %#v", opened)
	}
	if got := scalar(t, run(t, executor, profile, "two", "SELECT count(*) FROM "+table, false)); got != "0" {
		t.Fatalf("tab two saw uncommitted rows from tab one: count = %s", got)
	}
	if state := executor.SessionStatus("two").State; state != TxIdle {
		t.Fatalf("tab two state = %v", state)
	}
	if ids := executor.OpenTransactions(); len(ids) != 1 || ids[0] != "one" {
		t.Fatalf("OpenTransactions() = %v", ids)
	}
	// The object browser has its own session, so it works mid-transaction.
	if _, err := executor.ListSchemas(context.Background(), profile); err != nil {
		t.Fatalf("catalog query during another tab's transaction: %v", err)
	}
	if tag, err := executor.EndTransaction("one", true); err != nil || tag != "COMMIT" {
		t.Fatalf("EndTransaction(commit) = %q, %v", tag, err)
	}
	if got := scalar(t, run(t, executor, profile, "two", "SELECT count(*) FROM "+table, false)); got != "1" {
		t.Fatalf("committed row not visible: count = %s", got)
	}
	if _, err := executor.EndTransaction("one", false); err != ErrNoTransaction {
		t.Fatalf("EndTransaction with nothing open = %v", err)
	}
}

func TestRunRollsBackAFailedTransactionAndKeepsEarlierResults(t *testing.T) {
	executor, profile := newPostgresExecutor(t)
	table := testTable(t, executor, profile)
	source := "BEGIN;\nCREATE TABLE " + table + " (value int);\nSELECT 1 AS first;\nSELECT\n  1/0;\nSELECT 2;"
	outcome := run(t, executor, profile, "tab", source, true)
	if outcome.Failure == nil || outcome.Failure.Index != 3 || outcome.Failure.Code != "22012" {
		t.Fatalf("failure = %#v", outcome.Failure)
	}
	if outcome.Failure.Line != 4 {
		t.Fatalf("failure line = %d, want 4, where the failing statement starts", outcome.Failure.Line)
	}
	if len(outcome.Results) != 3 || outcome.Results[2].Rows[0][0] != "1" || outcome.Results[2].SQL != "SELECT 1 AS first" {
		t.Fatalf("results before the failure were not kept: %#v", outcome.Results)
	}
	if outcome.Transaction != TxIdle || len(outcome.Events) != 1 || !strings.Contains(outcome.Events[0], "rolled it back") {
		t.Fatalf("failed transaction not rolled back: %#v", outcome)
	}
	if got := scalar(t, run(t, executor, profile, "tab", "SELECT to_regclass('"+table+"') IS NULL", false)); got != "true" {
		t.Fatalf("table from the failed transaction exists: %s", got)
	}
}

func TestSyntaxErrorReportsTheLineOfItsPosition(t *testing.T) {
	executor, profile := newPostgresExecutor(t)
	outcome := run(t, executor, profile, "tab", "SELECT 1;\n\nSELECT\n  1,\n  FROM nowhere;", false)
	if outcome.Failure == nil || outcome.Failure.Code != "42601" || outcome.Failure.Line != 5 {
		t.Fatalf("failure = %#v, want a syntax error on line 5", outcome.Failure)
	}
}

func TestLockedTabIsReadOnlyOnTheServer(t *testing.T) {
	executor, profile := newPostgresExecutor(t)
	table := testTable(t, executor, profile)
	function := table + "_insert"
	run(t, executor, profile, "setup", "CREATE TABLE "+table+" (value int);\nCREATE FUNCTION "+function+
		"() RETURNS int LANGUAGE sql AS 'INSERT INTO "+table+" VALUES (1) RETURNING value'", true)

	locked := run(t, executor, profile, "tab", "SELECT "+function+"()", false)
	if locked.Failure == nil || locked.Failure.Code != "25006" {
		t.Fatalf("a locked tab ran a writing function: %#v", locked)
	}
	unlocked := run(t, executor, profile, "tab", "SELECT "+function+"()", true)
	if unlocked.Failure != nil {
		t.Fatalf("unlocked tab could not write: %s", unlocked.Failure.Message)
	}
}

func TestLockedTabCanRollBackButNotCommit(t *testing.T) {
	executor, profile := newPostgresExecutor(t)
	run(t, executor, profile, "tab", "BEGIN", true)
	if _, err := executor.Run(context.Background(), RunRequest{Profile: profile, SessionID: "tab", Source: "COMMIT"}); err == nil {
		t.Fatal("COMMIT ran in a locked tab")
	}
	if outcome := run(t, executor, profile, "tab", "ROLLBACK", false); outcome.Transaction != TxIdle {
		t.Fatalf("ROLLBACK in a locked tab left %v", outcome.Transaction)
	}
}

func TestClosingASessionDiscardsItsTransaction(t *testing.T) {
	executor, profile := newPostgresExecutor(t)
	table := testTable(t, executor, profile)
	run(t, executor, profile, "setup", "CREATE TABLE "+table+" (value int)", true)
	run(t, executor, profile, "tab", "BEGIN; INSERT INTO "+table+" VALUES (1)", true)
	if err := executor.Reconnect(context.Background(), profile, "tab", false); err != ErrTransactionOpen {
		t.Fatalf("Reconnect with an open transaction = %v", err)
	}
	executor.CloseSession("tab")
	if state := executor.SessionStatus("tab"); state.Connected || state.State != TxIdle {
		t.Fatalf("closed session status = %#v", state)
	}
	if got := scalar(t, run(t, executor, profile, "other", "SELECT count(*) FROM "+table, false)); got != "0" {
		t.Fatalf("closing the tab kept its uncommitted row: count = %s", got)
	}
}

func backendPID(t *testing.T, executor *Executor, profile config.SQLConnection, tab string) string {
	t.Helper()
	return scalar(t, run(t, executor, profile, tab, "SELECT pg_backend_pid()", false))
}

func TestLostConnectionRetriesOnlyIdleReadOnlyRuns(t *testing.T) {
	executor, profile := newPostgresExecutor(t)
	pid := backendPID(t, executor, profile, "tab")
	run(t, executor, profile, "killer", "SELECT pg_terminate_backend("+pid+")", false)
	retried := run(t, executor, profile, "tab", "SELECT 42", false)
	if got := scalar(t, retried); got != "42" || len(retried.Events) == 0 || !strings.Contains(retried.Events[0], "reconnected") {
		t.Fatalf("idle read-only run was not retried: %#v", retried)
	}

	run(t, executor, profile, "tab", "BEGIN", true)
	pid = backendPID(t, executor, profile, "tab")
	run(t, executor, profile, "killer", "SELECT pg_terminate_backend("+pid+")", false)
	lost := run(t, executor, profile, "tab", "SELECT 42", true)
	if lost.Failure == nil || !lost.ConnectionLost || lost.Transaction != TxIdle {
		t.Fatalf("run in a lost transaction = %#v", lost)
	}
	if len(lost.Events) != 1 || !strings.Contains(lost.Events[0], "rolled it back") {
		t.Fatalf("lost transaction was not reported: %#v", lost.Events)
	}
}

func TestIdleTransactionIsEndedByTheServer(t *testing.T) {
	executor, profile := newPostgresExecutor(t)
	profile.IdleInTransactionSecs = 1
	run(t, executor, profile, "tab", "BEGIN", true)
	if status := executor.SessionStatus("tab"); status.IdleTimeout != time.Second || status.Since.IsZero() {
		t.Fatalf("status = %#v", status)
	}
	time.Sleep(2500 * time.Millisecond)
	outcome := run(t, executor, profile, "tab", "SELECT 1", false)
	if !outcome.ConnectionLost || outcome.Transaction != TxIdle {
		t.Fatalf("server did not end the idle transaction: %#v", outcome)
	}
}

func runMode(t *testing.T, executor *Executor, profile config.SQLConnection, tab, source string, mode RunMode) RunOutcome {
	t.Helper()
	outcome, err := executor.Run(context.Background(), RunRequest{Profile: profile, SessionID: tab, Source: source, AllowWrites: true, Mode: mode})
	if err != nil {
		t.Fatalf("Run(%v, %q): %v", mode, source, err)
	}
	return outcome
}

// A dry run of a script written for psql, with its own BEGIN, temporary
// tables and final ROLLBACK, shows every result and keeps nothing.
func TestDryRunOfAVerificationScriptKeepsNothing(t *testing.T) {
	executor, profile := newPostgresExecutor(t)
	table := testTable(t, executor, profile)
	run(t, executor, profile, "setup", "CREATE TABLE "+table+" (value int); INSERT INTO "+table+" VALUES (1), (2)", true)
	script := `BEGIN;
CREATE TEMP TABLE candidates ON COMMIT DROP AS SELECT value FROM ` + table + `;
SELECT 'Rows to change' AS section;
SELECT value FROM candidates ORDER BY value;
UPDATE ` + table + ` SET value = value * 10 WHERE value IN (SELECT value FROM candidates);
SELECT 'Rows after the change' AS section;
SELECT value FROM ` + table + ` ORDER BY value;
ROLLBACK;`
	outcome := runMode(t, executor, profile, "tab", script, RunDryRun)
	if outcome.Failure != nil {
		t.Fatalf("dry run failed: %s", outcome.Failure.Message)
	}
	if len(outcome.Results) != 6 || outcome.Results[5].Rows[1][0] != "20" || outcome.Results[3].CommandTag != "UPDATE 2" {
		t.Fatalf("results = %#v", outcome.Results)
	}
	if outcome.Results[0].Line != 2 || outcome.Transaction != TxIdle || executor.SessionStatus("tab").State != TxIdle {
		t.Fatalf("line or state wrong: line=%d tx=%v", outcome.Results[0].Line, outcome.Transaction)
	}
	if got := scalar(t, run(t, executor, profile, "tab", "SELECT string_agg(value::text, ',' ORDER BY value) FROM "+table, false)); got != "1,2" {
		t.Fatalf("the dry run kept its changes: %s", got)
	}
	// The temporary table went with the transaction, so the same script can
	// run again at once.
	if again := runMode(t, executor, profile, "tab", script, RunDryRun); again.Failure != nil {
		t.Fatalf("second dry run failed: %s", again.Failure.Message)
	}
	report := FormatReport(outcome, ReportContext{Connection: profile.Name})
	if !strings.Contains(report, "UPDATE 2") || !strings.Contains(report, "Dry run: e9s rolled the transaction back") {
		t.Fatalf("report:\n%s", report)
	}
}

func TestRunInTransactionLeavesItOpenForReview(t *testing.T) {
	executor, profile := newPostgresExecutor(t)
	table := testTable(t, executor, profile)
	run(t, executor, profile, "setup", "CREATE TABLE "+table+" (value int)", true)
	outcome := runMode(t, executor, profile, "tab", "INSERT INTO "+table+" VALUES (1);\nINSERT INTO "+table+" VALUES (2), (3);", RunInTransaction)
	if outcome.Transaction != TxOpen || outcome.ChangeSummary() != "line 1: INSERT 0 1; line 2: INSERT 0 2" {
		t.Fatalf("outcome = %#v, changes = %q", outcome, outcome.ChangeSummary())
	}
	if got := scalar(t, run(t, executor, profile, "other", "SELECT count(*) FROM "+table, false)); got != "0" {
		t.Fatalf("uncommitted rows visible elsewhere: %s", got)
	}
	if _, err := executor.Run(context.Background(), RunRequest{Profile: profile, SessionID: "tab", Source: "SELECT 1", AllowWrites: true, Mode: RunDryRun}); err == nil {
		t.Fatal("a dry run started inside an open transaction")
	}
	if _, err := executor.EndTransaction("tab", true); err != nil {
		t.Fatal(err)
	}
	if got := scalar(t, run(t, executor, profile, "other", "SELECT count(*) FROM "+table, false)); got != "3" {
		t.Fatalf("committed rows: %s", got)
	}
}

func TestRunInTransactionRollsBackWhenAStatementFails(t *testing.T) {
	executor, profile := newPostgresExecutor(t)
	table := testTable(t, executor, profile)
	run(t, executor, profile, "setup", "CREATE TABLE "+table+" (value int)", true)
	outcome := runMode(t, executor, profile, "tab", "INSERT INTO "+table+" VALUES (1);\nSELECT 1/0;", RunInTransaction)
	if outcome.Failure == nil || outcome.Transaction != TxIdle {
		t.Fatalf("outcome = %#v", outcome)
	}
	if got := scalar(t, run(t, executor, profile, "other", "SELECT count(*) FROM "+table, false)); got != "0" {
		t.Fatalf("a failed transactional run kept rows: %s", got)
	}
}
