package views

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/sqlworkbench"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

// SQLWorkbenchModel is the terminal-native query editor and result browser.
// Tab metadata and query text are persisted by the parent application; query
// results intentionally live only for the current process.
type SQLWorkbenchModel struct {
	tabs         []sqlworkbench.TabState
	active       int
	editor       textarea.Model
	editing      bool
	runs         map[string]*sqlTabRun
	sessions     map[string]sqlworkbench.SessionStatus
	resultCursor map[string]int
	catalogs     map[string]*SQLCatalogModel
	status       string
	width        int
	height       int
}

func NewSQLWorkbench(tabs []sqlworkbench.TabState, activeID string) SQLWorkbenchModel {
	editor := textarea.New()
	editor.Placeholder = "SELECT …"
	editor.ShowLineNumbers = true
	editor.CharLimit = 0
	editor.Blur()
	m := SQLWorkbenchModel{
		tabs: append([]sqlworkbench.TabState(nil), tabs...), editor: editor,
		runs: make(map[string]*sqlTabRun), sessions: make(map[string]sqlworkbench.SessionStatus),
		resultCursor: make(map[string]int), catalogs: make(map[string]*SQLCatalogModel),
	}
	for index := range m.tabs {
		m.tabs[index].AllowWrites = false
		if m.tabs[index].ID == activeID {
			m.active = index
		}
		catalog := NewSQLCatalogModel()
		m.catalogs[m.tabs[index].ID] = &catalog
	}
	m.loadActiveQuery()
	return m
}

func (m SQLWorkbenchModel) SetSize(width, height int) SQLWorkbenchModel {
	m.width, m.height = width, height
	m.configureEditorWidth()
	editorHeight := max(5, min(14, (height-9)/2))
	m.editor.SetHeight(editorHeight)
	return m
}

func (m *SQLWorkbenchModel) configureEditorWidth() {
	width := m.width - 6
	if catalog := m.activeCatalog(); catalog != nil && catalog.Open {
		width -= min(52, max(30, m.width/3)) + 2
	}
	m.editor.SetWidth(max(20, width))
}

func (m SQLWorkbenchModel) Update(msg tea.Msg) (SQLWorkbenchModel, tea.Cmd) {
	if m.editing {
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		m.storeActiveQuery()
		return m, cmd
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		id := m.ActiveID()
		switch keyMsg.String() {
		case "j", "down":
			m.resultCursor[id]++
		case "k", "up":
			m.resultCursor[id] = max(0, m.resultCursor[id]-1)
		case "g":
			m.resultCursor[id] = 0
		case "G":
			if result, ok := m.SelectedResult(); ok {
				m.resultCursor[id] = max(0, len(result.Rows)-1)
			}
		case "<":
			m.MoveResult(-1)
		case ">":
			m.MoveResult(1)
		}
		if result, ok := m.SelectedResult(); ok {
			m.resultCursor[id] = min(m.resultCursor[id], max(0, len(result.Rows)-1))
		}
	}
	return m, nil
}

func (m *SQLWorkbenchModel) BeginEditing() tea.Cmd {
	if len(m.tabs) == 0 {
		return nil
	}
	m.editing = true
	return m.editor.Focus()
}

func (m *SQLWorkbenchModel) StopEditing() {
	m.storeActiveQuery()
	m.editing = false
	m.editor.Blur()
}

func (m SQLWorkbenchModel) Editing() bool { return m.editing }

func (m *SQLWorkbenchModel) AddTab(tab sqlworkbench.TabState) {
	m.storeActiveQuery()
	tab.AllowWrites = false
	m.tabs = append(m.tabs, tab)
	catalog := NewSQLCatalogModel()
	m.catalogs[tab.ID] = &catalog
	m.active = len(m.tabs) - 1
	m.loadActiveQuery()
	m.configureEditorWidth()
}

func (m *SQLWorkbenchModel) CloseActive() {
	if len(m.tabs) == 0 {
		return
	}
	id := m.ActiveID()
	delete(m.runs, id)
	delete(m.sessions, id)
	delete(m.resultCursor, id)
	delete(m.catalogs, id)
	m.tabs = append(m.tabs[:m.active], m.tabs[m.active+1:]...)
	if m.active >= len(m.tabs) {
		m.active = max(0, len(m.tabs)-1)
	}
	m.loadActiveQuery()
	m.configureEditorWidth()
}

func (m *SQLWorkbenchModel) CatalogOpen() bool {
	catalog := m.activeCatalog()
	return catalog != nil && catalog.Open
}

func (m *SQLWorkbenchModel) ToggleCatalog() (bool, bool) {
	catalog := m.activeCatalog()
	if catalog == nil {
		return false, false
	}
	opened, needsSchemas := catalog.Toggle()
	m.configureEditorWidth()
	return opened, needsSchemas
}

func (m *SQLWorkbenchModel) CatalogMove(delta int) {
	if catalog := m.activeCatalog(); catalog != nil {
		catalog.Move(delta)
	}
}

func (m *SQLWorkbenchModel) CatalogTop() {
	if catalog := m.activeCatalog(); catalog != nil {
		catalog.Top()
	}
}

func (m *SQLWorkbenchModel) CatalogBottom() {
	if catalog := m.activeCatalog(); catalog != nil {
		catalog.Bottom()
	}
}

func (m *SQLWorkbenchModel) ActivateCatalog() SQLCatalogRequest {
	if catalog := m.activeCatalog(); catalog != nil {
		return catalog.Activate()
	}
	return SQLCatalogRequest{}
}

func (m *SQLWorkbenchModel) InsertCatalogSelection(qualified bool) (string, bool) {
	catalog := m.activeCatalog()
	if catalog == nil {
		return "", false
	}
	text, ok := catalog.Insertion(qualified)
	if !ok {
		return "", false
	}
	m.editor.InsertString(text)
	m.storeActiveQuery()
	m.status = "Inserted " + text + " at the query cursor"
	return text, true
}

func (m *SQLWorkbenchModel) SetCatalogSchemasFor(id string, schemas []string, err error) {
	if catalog := m.catalogs[id]; catalog != nil {
		catalog.SetSchemas(schemas, err)
	}
}

func (m *SQLWorkbenchModel) SetCatalogObjectsFor(id, schema string, kind sqlworkbench.ObjectKind, objects []sqlworkbench.DatabaseObject, err error) {
	if catalog := m.catalogs[id]; catalog != nil {
		catalog.SetObjects(schema, kind, objects, err)
	}
}

func (m *SQLWorkbenchModel) SetCatalogColumnsFor(id string, object sqlworkbench.DatabaseObject, columns []sqlworkbench.ObjectColumn, err error) {
	if catalog := m.catalogs[id]; catalog != nil {
		catalog.SetColumns(object, columns, err)
	}
}

func (m *SQLWorkbenchModel) activeCatalog() *SQLCatalogModel {
	return m.catalogs[m.ActiveID()]
}

func (m *SQLWorkbenchModel) Switch(delta int) {
	if len(m.tabs) < 2 {
		return
	}
	m.storeActiveQuery()
	m.active = (m.active + delta + len(m.tabs)) % len(m.tabs)
	m.loadActiveQuery()
	m.configureEditorWidth()
}

func (m *SQLWorkbenchModel) Rename(title string) {
	if tab := m.ActiveTab(); tab != nil {
		tab.Title = strings.TrimSpace(title)
		tab.ManualTitle = tab.Title != ""
	}
}

func (m *SQLWorkbenchModel) ToggleWrites() bool {
	if tab := m.ActiveTab(); tab != nil {
		tab.AllowWrites = !tab.AllowWrites
		return tab.AllowWrites
	}
	return false
}

// SetResults shows results that did not come from a tab run, such as an
// object preview.
func (m *SQLWorkbenchModel) SetResults(results []model.SQLQueryResult) {
	m.SetRunFor(m.ActiveID(), sqlworkbench.RunOutcome{Statements: len(results), Results: results})
}

// SetRunFor shows a run's outcome in a tab. It opens on the failed
// statement when there is one, otherwise on the last result.
func (m *SQLWorkbenchModel) SetRunFor(id string, outcome sqlworkbench.RunOutcome) {
	if id == "" {
		return
	}
	run := &sqlTabRun{outcome: outcome}
	run.selected = max(0, run.entries()-1)
	m.runs[id] = run
	m.resultCursor[id] = 0
	if id == m.ActiveID() {
		m.status = run.summary()
	}
}

// SetSessionFor records a tab's session state for its header.
func (m *SQLWorkbenchModel) SetSessionFor(id string, status sqlworkbench.SessionStatus) {
	if id != "" {
		m.sessions[id] = status
	}
}

// Session returns the last recorded session state of the active tab.
func (m SQLWorkbenchModel) Session() sqlworkbench.SessionStatus {
	return m.sessions[m.ActiveID()]
}

// MoveResult selects another entry of the active tab's run.
func (m *SQLWorkbenchModel) MoveResult(delta int) {
	run := m.runs[m.ActiveID()]
	if run == nil || run.entries() == 0 {
		return
	}
	next := min(max(0, run.selected+delta), run.entries()-1)
	if next != run.selected {
		run.selected = next
		m.resultCursor[m.ActiveID()] = 0
	}
}

func (m *SQLWorkbenchModel) SetStatus(status string) { m.status = status }

func (m SQLWorkbenchModel) Tabs() []sqlworkbench.TabState {
	tabs := append([]sqlworkbench.TabState(nil), m.tabs...)
	if len(tabs) > 0 && m.active < len(tabs) {
		tabs[m.active].Query = m.editor.Value()
	}
	return tabs
}

func (m SQLWorkbenchModel) ActiveID() string {
	if m.active < 0 || m.active >= len(m.tabs) {
		return ""
	}
	return m.tabs[m.active].ID
}

func (m *SQLWorkbenchModel) ActiveTab() *sqlworkbench.TabState {
	if m.active < 0 || m.active >= len(m.tabs) {
		return nil
	}
	return &m.tabs[m.active]
}

func (m SQLWorkbenchModel) ActiveTabValue() (sqlworkbench.TabState, bool) {
	if m.active < 0 || m.active >= len(m.tabs) {
		return sqlworkbench.TabState{}, false
	}
	tab := m.tabs[m.active]
	tab.Query = m.editor.Value()
	return tab, true
}

func (m SQLWorkbenchModel) Query() string { return m.editor.Value() }

// SetQuery replaces the active tab's query text.
func (m *SQLWorkbenchModel) SetQuery(query string) {
	m.editor.SetValue(query)
	m.storeActiveQuery()
}

func (m SQLWorkbenchModel) CursorByteOffset() int {
	value := m.editor.Value()
	lines := strings.Split(value, "\n")
	row := min(m.editor.Line(), max(0, len(lines)-1))
	offset := 0
	for index := 0; index < row; index++ {
		offset += len(lines[index]) + 1
	}
	lineRunes := []rune(lines[row])
	character := min(m.editor.LineInfo().CharOffset, len(lineRunes))
	offset += len(string(lineRunes[:character]))
	return min(offset, len(value))
}

// SelectedResult returns the result set selected in the active tab's run.
// It reports false when nothing ran or the failed statement is selected.
func (m SQLWorkbenchModel) SelectedResult() (model.SQLQueryResult, bool) {
	run := m.runs[m.ActiveID()]
	if run == nil || run.selected >= len(run.outcome.Results) {
		return model.SQLQueryResult{}, false
	}
	return run.outcome.Results[run.selected], true
}

func (m *SQLWorkbenchModel) storeActiveQuery() {
	if tab := m.ActiveTab(); tab != nil {
		tab.Query = m.editor.Value()
	}
}

func (m *SQLWorkbenchModel) loadActiveQuery() {
	query := ""
	if tab := m.ActiveTab(); tab != nil {
		query = tab.Query
	}
	m.editor.SetValue(query)
	if m.editing {
		m.editor.Focus()
	} else {
		m.editor.Blur()
	}
}

func (m SQLWorkbenchModel) View() string {
	if len(m.tabs) == 0 {
		return theme.HelpStyle.Render("  No SQL tabs are open. Return to Connections and press Enter to create one.")
	}
	var b strings.Builder
	for index, tab := range m.tabs {
		title := strings.TrimSpace(tab.Title)
		if title == "" {
			title = tab.ProfileName
		}
		label := fmt.Sprintf(" %d:%s ", index+1, truncateSQLCell(title, 22))
		if index == m.active {
			label = theme.SelectedRowStyle.Render(label)
		} else {
			label = theme.HelpStyle.Render(label)
		}
		b.WriteString(label)
	}
	b.WriteString("\n")
	tab, _ := m.ActiveTabValue()
	lock := theme.ColorGreen
	lockText := "read-only"
	if tab.AllowWrites {
		lock, lockText = theme.ColorRed, "WRITES ENABLED"
	}
	b.WriteString(theme.TitleStyle.Render(fmt.Sprintf("  %s / %s", tab.ProfileName, lockText)))
	if m.editing {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.ColorYellow).Render("  EDITING — esc returns to commands"))
	}
	if transaction := m.Session().Describe(time.Now()); transaction != "" {
		b.WriteString("  " + lipgloss.NewStyle().Bold(true).Foreground(theme.ColorRed).Render("● "+transaction) +
			theme.HelpStyle.Render("  [C] commit  [B] roll back"))
	}
	b.WriteString("\n")
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lock).Padding(0, 1)
	editorWidth := max(10, m.width-4)
	editorPanel := border.Width(editorWidth).Render(m.editor.View())
	if catalog := m.activeCatalog(); catalog != nil && catalog.Open {
		catalogWidth := min(52, max(30, m.width/3))
		editorWidth = max(10, m.width-catalogWidth-6)
		editorPanel = lipgloss.JoinHorizontal(lipgloss.Top, catalog.View(catalogWidth, m.editor.Height()+2),
			border.Width(editorWidth).Render(m.editor.View()))
	}
	b.WriteString(editorPanel)
	b.WriteString("\n")
	run := m.runs[m.ActiveID()]
	if run == nil || run.entries() == 0 {
		b.WriteString(theme.HelpStyle.Render("  No result loaded for this tab."))
		if m.status != "" {
			b.WriteString("  " + m.status)
		}
		return b.String()
	}
	b.WriteString(theme.HelpStyle.Render("  "+truncateSQLCell(run.outcome.EntryLabel(run.selected), max(20, m.width-24))) +
		theme.HelpStyle.Render("  [</>] results") + "\n")
	visible := max(3, m.height-m.editor.Height()-13-len(run.outcome.Events))
	if result, ok := m.SelectedResult(); ok {
		b.WriteString(renderSQLResultTable(result, m.resultCursor[m.ActiveID()], visible))
	} else if failure := run.outcome.Failure; failure != nil {
		b.WriteString(renderSQLFailure(*failure, m.width))
	}
	for _, event := range run.outcome.Events {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.ColorYellow).Render("  "+event) + "\n")
	}
	if m.status != "" {
		b.WriteString(theme.HelpStyle.Render("  " + m.status))
	}
	return b.String()
}

func renderSQLResultTable(result model.SQLQueryResult, cursor, visible int) string {
	if len(result.Columns) == 0 {
		return theme.HelpStyle.Render("  "+sqlworkbench.ResultTag(result)+" — no result set") + "\n"
	}
	columns := make([]components.Column, len(result.Columns))
	for index, title := range result.Columns {
		columns[index] = components.Column{Title: truncateSQLCell(title, 24)}
	}
	table := components.NewTable(columns)
	for _, row := range result.Rows {
		cells := make([]components.Cell, len(row))
		for index, value := range row {
			cells[index] = components.Plain(truncateSQLCell(strings.NewReplacer("\n", " ↵ ", "\r", "", "\t", "    ").Replace(value), 32))
		}
		table.AddRow(cells...)
	}
	return table.Render(cursor, "", visible)
}

func renderSQLFailure(failure sqlworkbench.StatementFailure, width int) string {
	var b strings.Builder
	heading := fmt.Sprintf("Statement %d failed", failure.Index+1)
	if failure.Line > 0 {
		heading += fmt.Sprintf(" at line %d", failure.Line)
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.ColorRed).Render("  "+heading) + "\n")
	for _, line := range strings.Split(failure.Message, "\n") {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.ColorRed).Render("  "+truncateSQLCell(line, max(20, width-4))) + "\n")
	}
	if failure.SQL != "" {
		lines := strings.Split(failure.SQL, "\n")
		for index, line := range lines {
			if index == 6 {
				b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("    … %d more line(s)", len(lines)-index)) + "\n")
				break
			}
			b.WriteString(theme.HelpStyle.Render("    "+truncateSQLCell(line, max(20, width-6))) + "\n")
		}
	}
	return b.String()
}

func truncateSQLCell(value string, width int) string {
	if utf8.RuneCountInString(value) <= width {
		return value
	}
	runes := []rune(value)
	return string(runes[:max(1, width-1)]) + "…"
}

// sqlTabRun is a tab's latest run and the entry selected in it.
type sqlTabRun struct {
	outcome  sqlworkbench.RunOutcome
	selected int
}

func (r *sqlTabRun) entries() int { return r.outcome.EntryCount() }

func (r *sqlTabRun) summary() string { return r.outcome.Summary() }
