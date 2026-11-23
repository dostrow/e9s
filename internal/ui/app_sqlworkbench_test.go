package ui

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/sqlworkbench"
	"github.com/dostrow/e9s/internal/ui/views"
)

// sqlTestApp is a terminal app with one SQL tab, writes enabled, on the
// server in E9S_TEST_POSTGRES_URL. The test is skipped without one.
func sqlTestApp(t *testing.T) App {
	t.Helper()
	raw := os.Getenv("E9S_TEST_POSTGRES_URL")
	if raw == "" {
		t.Skip("set E9S_TEST_POSTGRES_URL to run PostgreSQL session tests")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(parsed.Port())
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
	passwords := newSQLPasswordCache()
	passwords.Set(profile.Name, password)
	executor := sqlworkbench.NewExecutor(sqlworkbench.ExecutorOptions{AllowWrites: true, Prompt: sqlPasswordPrompt(passwords)})
	t.Cleanup(executor.Close)
	workbench := views.NewSQLWorkbench([]sqlworkbench.TabState{{ID: "one", ProfileName: profile.Name}}, "one").SetSize(120, 40)
	workbench.ToggleWrites()
	return App{ctx: context.Background(), kb: NewKeyBindings(), cfg: &config.Config{SQL: config.SQLConfig{AllowWrites: true, Connections: []config.SQLConnection{profile}}},
		mode: modeSQLWorkbench, state: viewSQLWorkbench, width: 120, height: 40, sqlExecutor: executor, sqlPasswords: passwords,
		sqlWorkbenchView: workbench, sqlStatePath: filepath.Join(t.TempDir(), "sql.json")}
}

// finish runs a command's message back through the app, as Bubble Tea would.
func finish(t *testing.T, a App, cmd tea.Cmd) App {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command")
	}
	model, _ := a.Update(cmd())
	return model.(App)
}

func runTabSQL(t *testing.T, a App, source string) App {
	t.Helper()
	a.sqlWorkbenchView.SetQuery(source)
	next, cmd := a.runSQL(sqlTUIRunAll)
	return finish(t, next, cmd)
}

func TestSQLTabAsksBeforeDiscardingAnOpenTransaction(t *testing.T) {
	a := sqlTestApp(t)
	a = runTabSQL(t, a, "BEGIN; SELECT 1")
	if a.err != nil || a.sqlWorkbenchView.Session().State != sqlworkbench.TxOpen {
		t.Fatalf("transaction not tracked: err=%v session=%#v", a.err, a.sqlWorkbenchView.Session())
	}

	closing, _ := a.closeSQLTab(false)
	if !closing.confirm.Active || closing.confirm.Action != ConfirmSQLCloseTab {
		t.Fatal("closing a tab with an open transaction did not ask first")
	}
	if _, found := closing.sqlWorkbenchView.ActiveTabValue(); !found {
		t.Fatal("the tab closed before the confirmation")
	}
	reconnecting, _ := a.reconnectSQL(false)
	if !reconnecting.confirm.Active || reconnecting.confirm.Action != ConfirmSQLReconnect {
		t.Fatal("reconnecting with an open transaction did not ask first")
	}
	quitting := a
	if !quitting.confirmQuitWithOpenTransactions() || quitting.confirm.Action != ConfirmSQLQuit {
		t.Fatal("quitting with an open transaction did not ask first")
	}
	committing, cmd := a.endSQLTransaction(true, false)
	if cmd != nil || committing.confirm.Action != ConfirmSQLCommit {
		t.Fatal("commit ran without confirmation")
	}

	rolledBack, cmd := a.endSQLTransaction(false, false)
	a = finish(t, rolledBack, cmd)
	if a.err != nil || a.sqlWorkbenchView.Session().State != sqlworkbench.TxIdle || !strings.Contains(a.flashMessage, "ROLLBACK") {
		t.Fatalf("rollback: err=%v session=%#v flash=%q", a.err, a.sqlWorkbenchView.Session(), a.flashMessage)
	}
	if a.confirmQuitWithOpenTransactions() {
		t.Fatal("quit asked with no open transaction")
	}
	closed, _ := a.closeSQLTab(false)
	if closed.confirm.Active {
		t.Fatal("closing an idle tab asked for confirmation")
	}
}

func TestSQLTabRunKeepsResultsBeforeAFailure(t *testing.T) {
	a := sqlTestApp(t)
	a = runTabSQL(t, a, "SELECT 'kept' AS value;\nSELECT 1/0;")
	if a.err != nil {
		t.Fatalf("a statement failure is part of the run, not an app error: %v", a.err)
	}
	view := a.sqlWorkbenchView.View()
	if !strings.Contains(view, "Statement 2 failed at line 2") || !strings.Contains(view, "division by zero") {
		t.Fatalf("failure not shown:\n%s", view)
	}
	a.sqlWorkbenchView.MoveResult(-1)
	if result, found := a.sqlWorkbenchView.SelectedResult(); !found || result.Rows[0][0] != "kept" {
		t.Fatalf("result before the failure: %#v", result)
	}
}
