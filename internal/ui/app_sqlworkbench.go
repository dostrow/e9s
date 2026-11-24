package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/sqlworkbench"
	"github.com/dostrow/e9s/internal/ui/views"
)

type sqlTUIRunMode int

const (
	sqlTUIRunNone sqlTUIRunMode = iota
	sqlTUIRunCurrent
	sqlTUIRunAll
	sqlTUIDryRun
	sqlTUIRunInTransaction
)

// executorMode is the executor's run mode for a TUI run.
func (m sqlTUIRunMode) executorMode() sqlworkbench.RunMode {
	switch m {
	case sqlTUIDryRun:
		return sqlworkbench.RunDryRun
	case sqlTUIRunInTransaction:
		return sqlworkbench.RunInTransaction
	default:
		return sqlworkbench.RunAsWritten
	}
}

type sqlPasswordCache struct {
	mu     sync.RWMutex
	values map[string]string
}

func newSQLPasswordCache() *sqlPasswordCache {
	return &sqlPasswordCache{values: make(map[string]string)}
}

func (c *sqlPasswordCache) Get(name string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, found := c.values[name]
	return value, found
}

func (c *sqlPasswordCache) Set(name, password string) {
	c.mu.Lock()
	c.values[name] = password
	c.mu.Unlock()
}

type sqlExecutedMsg struct {
	tabID   string
	outcome sqlworkbench.RunOutcome
	err     error
}

// sqlProgressMsg reports the statement a run is executing. events delivers
// the run's later messages.
type sqlProgressMsg struct {
	tabID    string
	progress sqlworkbench.Progress
	events   <-chan tea.Msg
}

// waitForSQLEvent delivers the next message of a running SQL run.
func waitForSQLEvent(events <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-events }
}

type sqlReconnectedMsg struct {
	tabID   string
	profile string
	err     error
}

type sqlTransactionEndedMsg struct {
	tabID  string
	commit bool
	tag    string
	err    error
}

type sqlCatalogSchemasMsg struct {
	tabID   string
	schemas []string
	err     error
}

type sqlCatalogObjectsMsg struct {
	tabID   string
	schema  string
	kind    sqlworkbench.ObjectKind
	objects []sqlworkbench.DatabaseObject
	err     error
}

type sqlCatalogColumnsMsg struct {
	tabID   string
	object  sqlworkbench.DatabaseObject
	columns []sqlworkbench.ObjectColumn
	err     error
}

func (a App) openSQLConnections() (App, tea.Cmd) {
	a.mode = modeSQLWorkbench
	a.state = viewSQLConnections
	a.loading = false
	a.sqlProfiles = append(a.sqlProfiles[:0], a.cfg.SQL.Connections...)
	a.sqlConnectionsView = views.NewEC2ResourceList("SQL connections", []string{"NAME", "RESOURCE", "DATABASE", "USER", "AUTH"}).SetSize(a.width-3, a.height-6)
	rows := make([]views.EC2ResourceRow, 0, len(a.sqlProfiles))
	for _, profile := range a.sqlProfiles {
		resource := profile.Host
		if resource == "" {
			resource = profile.ResourceID
		}
		auth := profile.Auth
		if auth == "" {
			auth = "pgpass"
		}
		rows = append(rows, views.EC2ResourceRow{ID: profile.Name, Search: strings.Join([]string{profile.Name, resource, profile.Database, profile.User, auth}, " "), Cells: []string{
			profile.Name, valueOrDashTUI(resource), profile.Database, valueOrDashTUI(profile.User), auth,
		}})
	}
	a.sqlConnectionsView = a.sqlConnectionsView.SetRows(rows)
	return a, nil
}

func valueOrDashTUI(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func (a App) openSelectedSQLConnection() (App, tea.Cmd) {
	profile, found := a.sqlProfileNamed(a.sqlConnectionsView.SelectedID())
	if !found {
		return a, nil
	}
	tab := sqlworkbench.TabState{ID: fmt.Sprintf("sql-%d", time.Now().UnixNano()), ProfileName: profile.Name, UpdatedAt: time.Now()}
	a.sqlWorkbenchView.AddTab(tab)
	a.state = viewSQLWorkbench
	a.saveSQLWorkbenchState()
	return a, nil
}

func (a App) showSQLWorkbench() (App, tea.Cmd) {
	if _, found := a.sqlWorkbenchView.ActiveTabValue(); !found {
		return a.openSQLConnections()
	}
	a.state = viewSQLWorkbench
	return a, nil
}

func (a App) toggleSQLCatalog() (App, tea.Cmd) {
	tab, found := a.sqlWorkbenchView.ActiveTabValue()
	if !found {
		return a, nil
	}
	_, needsSchemas := a.sqlWorkbenchView.ToggleCatalog()
	if needsSchemas {
		return a, a.loadSQLCatalogSchemas(tab)
	}
	return a, nil
}

func (a App) loadSQLCatalogSchemas(tab sqlworkbench.TabState) tea.Cmd {
	profile, found := a.sqlProfileNamed(tab.ProfileName)
	if !found || a.sqlExecutor == nil {
		return nil
	}
	executor, ctx, tabID := a.sqlExecutor, a.ctx, tab.ID
	return func() tea.Msg {
		requestCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		defer cancel()
		schemas, err := executor.ListSchemas(requestCtx, profile)
		return sqlCatalogSchemasMsg{tabID: tabID, schemas: schemas, err: err}
	}
}

func (a App) activateSQLCatalog() (App, tea.Cmd) {
	tab, found := a.sqlWorkbenchView.ActiveTabValue()
	if !found {
		return a, nil
	}
	request := a.sqlWorkbenchView.ActivateCatalog()
	profile, found := a.sqlProfileNamed(tab.ProfileName)
	if !found || a.sqlExecutor == nil {
		return a, nil
	}
	executor, ctx, tabID := a.sqlExecutor, a.ctx, tab.ID
	switch request.Kind {
	case views.SQLCatalogRequestObjects:
		return a, func() tea.Msg {
			requestCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()
			objects, err := executor.ListObjects(requestCtx, profile, request.Schema, request.ObjectKind)
			return sqlCatalogObjectsMsg{tabID: tabID, schema: request.Schema, kind: request.ObjectKind, objects: objects, err: err}
		}
	case views.SQLCatalogRequestColumns:
		return a, func() tea.Msg {
			requestCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()
			columns, err := executor.ListColumns(requestCtx, profile, request.Object)
			return sqlCatalogColumnsMsg{tabID: tabID, object: request.Object, columns: columns, err: err}
		}
	default:
		return a, nil
	}
}

func (a App) sqlProfileNamed(name string) (config.SQLConnection, bool) {
	for _, profile := range a.cfg.SQL.Connections {
		if profile.Name == name {
			return profile, true
		}
	}
	return config.SQLConnection{}, false
}

func (a App) runSQL(mode sqlTUIRunMode) (App, tea.Cmd) {
	tab, found := a.sqlWorkbenchView.ActiveTabValue()
	if !found || a.sqlExecutor == nil || a.sqlPending {
		return a, nil
	}
	profile, found := a.sqlProfileNamed(tab.ProfileName)
	if !found {
		a.err = fmt.Errorf("SQL connection profile %q is no longer configured", tab.ProfileName)
		return a, nil
	}
	if profile.Auth == "password" {
		if _, found := a.sqlPasswords.Get(profile.Name); !found {
			a.sqlPendingRun = mode
			a.sqlPendingProfile = profile.Name
			a.input = NewPasswordInput(InputSQLPassword, "Password for "+profile.Name)
			return a, nil
		}
	}
	source := a.sqlWorkbenchView.Query()
	if mode == sqlTUIRunCurrent {
		statement, found := sqlworkbench.CurrentStatement(source, a.sqlWorkbenchView.CursorByteOffset())
		if !found {
			a.err = fmt.Errorf("no SQL statement at the cursor")
			return a, nil
		}
		source = statement.SQL
	}
	a.sqlPending = true
	a.loading = true
	ctx, cancel := context.WithCancel(a.ctx)
	a.sqlRunCancel = cancel
	events := make(chan tea.Msg, 8)
	request := sqlworkbench.RunRequest{Profile: profile, SessionID: tab.ID, Source: source, AllowWrites: tab.AllowWrites, Mode: mode.executorMode(),
		Progress: func(progress sqlworkbench.Progress) {
			select {
			case events <- sqlProgressMsg{tabID: tab.ID, progress: progress, events: events}:
			default: // the display is behind; a later update will do
			}
		}}
	executor := a.sqlExecutor
	go func() {
		outcome, err := executor.Run(ctx, request)
		cancel()
		events <- sqlExecutedMsg{tabID: request.SessionID, outcome: outcome, err: err}
	}()
	return a, waitForSQLEvent(events)
}

// cancelSQLRun asks PostgreSQL to stop the running statement. The run then
// reports the cancellation and rolls back any transaction it was in.
func (a App) cancelSQLRun() (App, tea.Cmd) {
	if a.sqlRunCancel == nil {
		return a, nil
	}
	a.sqlRunCancel()
	a.flashMessage = "Cancelling the running SQL…"
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

// sqlTransactionOpen reports whether the active tab's session holds an open
// transaction.
func (a App) sqlTransactionOpen() (sqlworkbench.TabState, bool) {
	tab, found := a.sqlWorkbenchView.ActiveTabValue()
	if !found || a.sqlExecutor == nil {
		return tab, false
	}
	return tab, a.sqlExecutor.SessionStatus(tab.ID).State == sqlworkbench.TxOpen
}

// endSQLTransaction commits or rolls back the active tab's transaction.
// Commit asks first: its changes cannot be undone.
func (a App) endSQLTransaction(commit, confirmed bool) (App, tea.Cmd) {
	tab, open := a.sqlTransactionOpen()
	if a.sqlPending {
		return a, nil
	}
	if !open {
		a.err = sqlworkbench.ErrNoTransaction
		return a, nil
	}
	if commit && !confirmed {
		a.sqlConfirmTab = tab.ID
		a.confirm = NewConfirm(ConfirmSQLCommit, "Commit the open transaction in "+sqlTabName(tab)+"? Its changes become permanent.")
		return a, nil
	}
	a.sqlPending = true
	a.loading = true
	executor, tabID, allowWrites := a.sqlExecutor, tab.ID, tab.AllowWrites
	return a, func() tea.Msg {
		tag, err := executor.EndTransaction(tabID, commit, allowWrites)
		return sqlTransactionEndedMsg{tabID: tabID, commit: commit, tag: tag, err: err}
	}
}

func sqlTabName(tab sqlworkbench.TabState) string {
	if title := strings.TrimSpace(tab.Title); title != "" {
		return title
	}
	return tab.ProfileName
}

// reconnectSQL replaces the active tab's connection. Doing so discards an
// open transaction, so that needs confirming.
func (a App) reconnectSQL(discardTransaction bool) (App, tea.Cmd) {
	tab, found := a.sqlWorkbenchView.ActiveTabValue()
	if !found || a.sqlPending {
		return a, nil
	}
	if _, open := a.sqlTransactionOpen(); open && !discardTransaction {
		a.sqlConfirmTab = tab.ID
		a.confirm = NewConfirm(ConfirmSQLReconnect, "Reconnecting discards the open transaction in "+sqlTabName(tab)+": its changes are rolled back. Reconnect?")
		return a, nil
	}
	profile, found := a.sqlProfileNamed(tab.ProfileName)
	if !found {
		a.err = fmt.Errorf("SQL connection profile %q is no longer configured", tab.ProfileName)
		return a, nil
	}
	if profile.Auth == "password" {
		if _, found := a.sqlPasswords.Get(profile.Name); !found {
			a.sqlPendingRun = sqlTUIRunNone
			a.sqlPendingProfile = profile.Name
			a.input = NewPasswordInput(InputSQLPassword, "Password for "+profile.Name)
			return a, nil
		}
	}
	a.sqlPending = true
	a.loading = true
	executor, ctx, tabID := a.sqlExecutor, a.ctx, tab.ID
	return a, func() tea.Msg {
		err := executor.Reconnect(ctx, profile, tabID, discardTransaction)
		return sqlReconnectedMsg{tabID: tabID, profile: profile.Name, err: err}
	}
}

func (a App) toggleSQLWrites() (App, tea.Cmd) {
	tab, found := a.sqlWorkbenchView.ActiveTabValue()
	if !found {
		return a, nil
	}
	if tab.AllowWrites {
		a.sqlWorkbenchView.ToggleWrites()
		a.saveSQLWorkbenchState()
		return a, nil
	}
	if !a.cfg.SQL.AllowWrites {
		a.err = fmt.Errorf("SQL writes are disabled globally; set sql.allow_writes to true first")
		return a, nil
	}
	a.confirm = NewConfirm(ConfirmSQLWrites, "Break the glass and permit data-changing SQL in this tab? This authorization is not restored after restart.")
	return a, nil
}

// closeSQLTab closes the active tab and its connection. Closing discards an
// open transaction, so that needs confirming.
func (a App) closeSQLTab(discardTransaction bool) (App, tea.Cmd) {
	if a.sqlPending {
		return a, nil
	}
	tab, open := a.sqlTransactionOpen()
	if open && !discardTransaction {
		a.sqlConfirmTab = tab.ID
		a.confirm = NewConfirm(ConfirmSQLCloseTab, "Closing "+sqlTabName(tab)+" discards its open transaction: its changes are rolled back. Close it?")
		return a, nil
	}
	if a.sqlExecutor != nil {
		a.sqlExecutor.CloseSession(tab.ID)
	}
	a.sqlWorkbenchView.CloseActive()
	a.saveSQLWorkbenchState()
	if _, found := a.sqlWorkbenchView.ActiveTabValue(); !found {
		return a.openSQLConnections()
	}
	return a, nil
}

// confirmQuitWithOpenTransactions asks before quitting would discard open
// transactions. It reports whether it asked.
func (a *App) confirmQuitWithOpenTransactions() bool {
	if a.sqlExecutor == nil {
		return false
	}
	open := a.sqlExecutor.OpenTransactions()
	if len(open) == 0 {
		return false
	}
	a.confirm = NewConfirm(ConfirmSQLQuit, fmt.Sprintf("%d SQL tab(s) have an open transaction. Quitting rolls them back: their changes are not kept. Quit?", len(open)))
	return true
}

// promptSQLCommitAfterRun asks to commit a transactional run that left its
// transaction open, listing what it changed. Declining keeps it open.
func (a App) promptSQLCommitAfterRun(tabID string, outcome sqlworkbench.RunOutcome) App {
	tab, found := a.sqlWorkbenchView.ActiveTabValue()
	if !found || tab.ID != tabID || outcome.Mode != sqlworkbench.RunInTransaction || outcome.Transaction != sqlworkbench.TxOpen {
		return a
	}
	changes := outcome.ChangeSummary()
	if changes == "" {
		changes = "no rows were reported changed"
	}
	a.sqlConfirmTab = tabID
	a.confirm = NewConfirm(ConfirmSQLCommit, "The run succeeded ("+changes+"). Commit? No keeps the transaction open for review: commit with C or roll back with B.")
	return a
}

func (a App) sqlReportContext(tab sqlworkbench.TabState) sqlworkbench.ReportContext {
	context := sqlworkbench.ReportContext{Connection: tab.ProfileName, Tab: strings.TrimSpace(tab.Title)}
	if profile, found := a.sqlProfileNamed(tab.ProfileName); found {
		context.Database = profile.Database
	}
	return context
}

func (a App) promptSQLReportSave() (App, tea.Cmd) {
	tab, found := a.sqlWorkbenchView.ActiveTabValue()
	if _, ok := a.sqlWorkbenchView.Report(a.sqlReportContext(tab)); !found || !ok {
		a.err = fmt.Errorf("run SQL in this tab before saving a report")
		return a, nil
	}
	name := "sql-report-" + time.Now().Format("20060102-150405") + ".txt"
	a.input = NewInput(InputSQLReport, "Report output path", filepath.Join(a.cfg.SaveDir(), name))
	return a, nil
}

func (a App) saveSQLReport(path string) (App, tea.Cmd) {
	tab, _ := a.sqlWorkbenchView.ActiveTabValue()
	report, ok := a.sqlWorkbenchView.Report(a.sqlReportContext(tab))
	if !ok {
		return a, nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		a.err = fmt.Errorf("report output path cannot be empty")
		return a, nil
	}
	if err := os.WriteFile(path, []byte(report), 0o600); err != nil {
		a.err = err
		return a, nil
	}
	a.flashMessage = "Saved SQL report to " + path
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

func (a App) promptSQLExport() (App, tea.Cmd) {
	if _, found := a.sqlWorkbenchView.SelectedResult(); !found {
		a.err = fmt.Errorf("select a result set to export")
		return a, nil
	}
	directory := a.cfg.SaveDir()
	a.input = NewInput(InputSQLExport, "CSV output path", filepath.Join(directory, "query-results.csv"))
	return a, nil
}

func (a App) exportSQLResult(path string) (App, tea.Cmd) {
	result, found := a.sqlWorkbenchView.SelectedResult()
	if !found {
		return a, nil
	}
	path = strings.TrimSpace(path)
	if path == "" {
		a.err = fmt.Errorf("CSV output path cannot be empty")
		return a, nil
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err == nil {
		err = sqlworkbench.WriteCSV(file, result)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil {
		a.err = err
		return a, nil
	}
	a.flashMessage = "Exported SQL results to " + path
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

func (a *App) saveSQLWorkbenchState() {
	tabs := a.sqlWorkbenchView.Tabs()
	for index := range tabs {
		tabs[index].AllowWrites = false
	}
	state := sqlworkbench.WorkbenchState{Tabs: tabs, ActiveTabID: a.sqlWorkbenchView.ActiveID()}
	if err := sqlworkbench.SaveState(a.sqlStatePath, state); err != nil {
		a.err = fmt.Errorf("save SQL workbench state: %w", err)
	}
}

func sqlPasswordPrompt(cache *sqlPasswordCache) sqlworkbench.PasswordPrompt {
	return func(_ context.Context, profile config.SQLConnection) (string, error) {
		if password, found := cache.Get(profile.Name); found {
			return password, nil
		}
		return "", fmt.Errorf("password required for %s", profile.Name)
	}
}
