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
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/sqlworkbench"
	"github.com/dostrow/e9s/internal/ui/views"
)

type sqlTUIRunMode int

const (
	sqlTUIRunNone sqlTUIRunMode = iota
	sqlTUIRunCurrent
	sqlTUIRunAll
)

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
	results []model.SQLQueryResult
	err     error
}

type sqlReconnectedMsg struct {
	profile string
	err     error
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
	tabID := tab.ID
	executor, ctx := a.sqlExecutor, a.ctx
	return a, func() tea.Msg {
		results, err := executor.Execute(ctx, profile, source, tab.AllowWrites)
		return sqlExecutedMsg{tabID: tabID, results: results, err: err}
	}
}

func (a App) reconnectSQL() (App, tea.Cmd) {
	tab, found := a.sqlWorkbenchView.ActiveTabValue()
	if !found || a.sqlPending {
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
	executor, ctx := a.sqlExecutor, a.ctx
	return a, func() tea.Msg {
		err := executor.Reconnect(ctx, profile)
		return sqlReconnectedMsg{profile: profile.Name, err: err}
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

func (a App) closeSQLTab() (App, tea.Cmd) {
	if a.sqlPending {
		return a, nil
	}
	a.sqlWorkbenchView.CloseActive()
	a.saveSQLWorkbenchState()
	if _, found := a.sqlWorkbenchView.ActiveTabValue(); !found {
		return a.openSQLConnections()
	}
	return a, nil
}

func (a App) promptSQLExport() (App, tea.Cmd) {
	if _, found := a.sqlWorkbenchView.LastResult(); !found {
		a.err = fmt.Errorf("no SQL result is available to export")
		return a, nil
	}
	directory := a.cfg.SaveDir()
	a.input = NewInput(InputSQLExport, "CSV output path", filepath.Join(directory, "query-results.csv"))
	return a, nil
}

func (a App) exportSQLResult(path string) (App, tea.Cmd) {
	result, found := a.sqlWorkbenchView.LastResult()
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
