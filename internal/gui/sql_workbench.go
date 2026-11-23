//go:build gui

package gui

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/sqlworkbench"
)

type sqlWorkbenchTab struct {
	state                 sqlworkbench.TabState
	editor                *sourceEditor
	page                  gtk.Widgetter
	tabLabel              *gtk.Label
	resultTable           *stringTable
	resultScroll          *gtk.ScrolledWindow
	resultLabel           *gtk.Label
	resultChooser         *gtk.DropDown
	resultChoices         *gtk.StringList
	choosingResult        bool
	failureLabel          *gtk.Label
	eventsLabel           *gtk.Label
	noticesLabel          *gtk.Label
	run                   sqlworkbench.RunOutcome
	selectedResult        int
	transactionBar        *gtk.Box
	transactionLabel      *gtk.Label
	commitButton          *gtk.Button
	rollbackButton        *gtk.Button
	transactionTimer      glib.SourceHandle
	reportBuffer          *gtk.TextBuffer
	contextNotebook       *gtk.Notebook
	structureBuffer       *gtk.TextBuffer
	definitionBuffer      *gtk.TextBuffer
	definitionEditor      *sourceEditor
	objectToolbar         *gtk.Box
	previewButton         *gtk.Button
	generateButton        *gtk.Button
	insertNameButton      *gtk.Button
	insertQualifiedButton *gtk.Button
	copyObjectButton      *gtk.Button
	definitionButton      *gtk.Button
	refreshObjectButton   *gtk.Button
	explorer              sqlObjectExplorer
}

type sqlObjectRowKind int

const (
	sqlObjectRowMessage sqlObjectRowKind = iota
	sqlObjectRowSchema
	sqlObjectRowCategory
	sqlObjectRowObject
	sqlObjectRowColumn
)

type sqlObjectBrowserRow struct {
	key      string
	kind     sqlObjectRowKind
	schema   string
	category sqlworkbench.ObjectKind
	object   sqlworkbench.DatabaseObject
	column   sqlworkbench.ObjectColumn
	label    string
}

type sqlObjectExplorer struct {
	schemas           []string
	objects           map[string][]sqlworkbench.DatabaseObject
	columns           map[string][]sqlworkbench.ObjectColumn
	loaded            map[string]bool
	loading           map[string]bool
	columnsLoaded     map[string]bool
	columnsLoading    map[string]bool
	expanded          map[string]bool
	rows              []sqlObjectBrowserRow
	selected          *sqlworkbench.DatabaseObject
	loadingSchemas    bool
	requestGeneration uint64
	detailGeneration  uint64
	restoreScroll     bool
	detailPending     bool
	detailCancel      context.CancelFunc
	catalogContext    context.Context
	catalogCancel     context.CancelFunc
}

func (w *mainWindow) buildSQLWorkbenchPane() *gtk.Box {
	newTab := gtk.NewButtonWithLabel("New tab")
	newTab.SetTooltipText("Open a query tab for the selected connection")
	newTab.ConnectClicked(w.openSelectedSQLProfile)
	manage := gtk.NewButtonWithLabel("Manage connections…")
	manage.ConnectClicked(w.manageSQLConnections)
	rename := gtk.NewButtonWithLabel("Rename tab…")
	rename.ConnectClicked(w.promptRenameSQLTab)
	closeTab := gtk.NewButtonWithLabel("Close tab")
	closeTab.ConnectClicked(w.closeActiveSQLTab)
	w.sqlReconnectButton = gtk.NewButtonWithLabel("Reconnect")
	w.sqlReconnectButton.ConnectClicked(w.reconnectActiveSQLTab)
	w.sqlRunSelectionButton = gtk.NewButtonWithLabel("Run selection")
	w.sqlRunSelectionButton.SetTooltipText("Run the selected SQL text (F5 when text is selected)")
	w.sqlRunSelectionButton.ConnectClicked(func() { w.runActiveSQL(sqlRunSelection) })
	w.sqlRunCurrentButton = gtk.NewButtonWithLabel("Run current")
	w.sqlRunCurrentButton.SetTooltipText("Run only the statement containing the cursor (F5 when no text is selected)")
	w.sqlRunCurrentButton.ConnectClicked(func() { w.runActiveSQL(sqlRunCurrent) })
	w.sqlRunAllButton = gtk.NewButtonWithLabel("Run all")
	w.sqlRunAllButton.SetTooltipText("Run every statement in this query tab, in order")
	w.sqlRunAllButton.ConnectClicked(func() { w.runActiveSQL(sqlRunAll) })
	w.sqlDryRunButton = gtk.NewButtonWithLabel("Dry run")
	w.sqlDryRunButton.SetTooltipText("Run every statement inside a transaction that is always rolled back, so nothing is kept")
	w.sqlDryRunButton.ConnectClicked(func() { w.runActiveSQL(sqlRunDry) })
	w.sqlRunInTransactionButton = gtk.NewButtonWithLabel("Run in transaction…")
	w.sqlRunInTransactionButton.SetTooltipText("Run every statement inside a transaction, review the results, then commit or roll back")
	w.sqlRunInTransactionButton.ConnectClicked(func() { w.runActiveSQL(sqlRunTransaction) })
	w.sqlCancelButton = gtk.NewButtonWithLabel("Cancel")
	w.sqlCancelButton.AddCSSClass("destructive-action")
	w.sqlCancelButton.SetTooltipText("Ask PostgreSQL to stop the running statement; an open transaction is rolled back")
	w.sqlCancelButton.SetVisible(false)
	w.sqlCancelButton.ConnectClicked(func() {
		if w.sqlActionCancel != nil {
			w.sqlActionCancel()
			w.setStatus("Cancelling the running SQL…", false)
		}
	})
	w.sqlWritesButton = gtk.NewToggleButtonWithLabel("Writes locked")
	w.sqlWritesButton.AddCSSClass("destructive-action")
	w.sqlWritesButton.ConnectToggled(w.toggleSQLWrites)
	w.sqlExportButton = gtk.NewButtonWithLabel("Export CSV…")
	w.sqlExportButton.ConnectClicked(w.exportActiveSQLResult)

	toolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	toolbar.AddCSSClass("log-toolbar")
	toolbar.Append(newTab)
	toolbar.Append(manage)
	toolbar.Append(rename)
	toolbar.Append(closeTab)
	toolbar.Append(w.sqlReconnectButton)
	toolbar.Append(w.sqlRunSelectionButton)
	toolbar.Append(w.sqlRunCurrentButton)
	toolbar.Append(w.sqlRunAllButton)
	toolbar.Append(w.sqlDryRunButton)
	toolbar.Append(w.sqlRunInTransactionButton)
	toolbar.Append(w.sqlCancelButton)
	toolbar.Append(w.sqlWritesButton)
	toolbar.Append(w.sqlExportButton)

	w.sqlEmptyLabel = gtk.NewLabel("Select a saved connection and open a query tab.")
	w.sqlEmptyLabel.SetWrap(true)
	w.sqlEmptyLabel.SetXAlign(0)
	w.sqlEmptyLabel.SetYAlign(0)
	w.sqlEmptyLabel.SetMarginTop(16)
	w.sqlEmptyLabel.SetMarginStart(16)
	w.sqlNotebook = gtk.NewNotebook()
	w.sqlNotebook.AddCSSClass("e9s-terminal-notebook")
	w.sqlNotebook.AddCSSClass("e9s-sql-notebook")
	w.sqlNotebook.SetScrollable(true)
	w.sqlNotebook.SetHExpand(true)
	w.sqlNotebook.SetVExpand(true)
	w.sqlNotebook.ConnectSwitchPage(func(_ gtk.Widgetter, _ uint) {
		if tab := w.activeSQLTab(); tab != nil {
			w.sqlState.ActiveTabID = tab.state.ID
			if w.currentPage == pageSQLObjects {
				w.showSQLObjectBrowser(tab)
			}
		}
		w.updateSQLControls()
		w.scheduleSQLStateSave()
	})

	w.sqlPaneStack = gtk.NewStack()
	w.sqlPaneStack.SetHExpand(true)
	w.sqlPaneStack.SetVExpand(true)
	w.sqlPaneStack.AddNamed(w.sqlEmptyLabel, "empty")
	w.sqlPaneStack.AddNamed(w.sqlNotebook, "tabs")
	w.sqlPaneStack.SetVisibleChildName("empty")

	pane := gtk.NewBox(gtk.OrientationVertical, 0)
	pane.AddCSSClass("e9s-sql-pane")
	pane.Append(toolbar)
	pane.Append(w.sqlPaneStack)
	w.sqlPane = pane
	w.restoreSQLTabs()
	w.updateSQLControls()
	return pane
}

func (w *mainWindow) openSQLWorkbenchModule() {
	w.resetWorkspaceForBrowserChange()
	w.currentPage = pageSQLConnections
	w.backButton.SetSensitive(false)
	w.search.SetSensitive(true)
	w.search.SetText("")
	w.search.SetPlaceholderText("Filter SQL connections…")
	w.resourceStack.SetVisibleChildName(pageSQLConnections)
	w.detailStack.SetVisibleChildName("sql-workbench")
	w.setBreadcrumb("SQL Workbench / Connections")
	w.refreshSQLProfiles(false)
	w.revealModuleForPage(w.currentPage)
	w.updateActionSensitivity()
}

func (w *mainWindow) refreshSQLProfiles(foreground bool) {
	if w.options.Config == nil {
		w.sqlProfiles = nil
	} else {
		w.sqlProfiles = append(w.sqlProfiles[:0], w.options.Config.SQL.Connections...)
	}
	w.applySQLProfileFilter()
	if foreground {
		w.setStatus(fmt.Sprintf("Loaded %d SQL connection profiles", len(w.sqlProfiles)), false)
	}
}

func (w *mainWindow) applySQLProfileFilter() {
	term := strings.ToLower(strings.TrimSpace(w.search.Text()))
	w.filteredSQLProfiles = w.filteredSQLProfiles[:0]
	rows := make([]string, 0, len(w.sqlProfiles))
	for _, profile := range w.sqlProfiles {
		resource := sqlProfileResource(profile)
		searchable := strings.ToLower(strings.Join([]string{profile.Name, resource, profile.Database, profile.User, profile.Auth}, " "))
		if term != "" && !strings.Contains(searchable, term) {
			continue
		}
		w.filteredSQLProfiles = append(w.filteredSQLProfiles, profile)
		rows = append(rows, strings.Join([]string{profile.Name, resource, profile.Database, valueOrDash(profile.User), sqlAuthMode(profile)}, "\t"))
	}
	w.sqlProfileTable.replace(rows)
}

func sqlProfileResource(profile config.SQLConnection) string {
	if profile.Host != "" {
		port := profile.Port
		if port == 0 {
			port = 5432
		}
		return profile.Host + ":" + strconv.Itoa(port)
	}
	return firstValue(profile.ResourceID, profile.ResourceARN, "—")
}

func sqlAuthMode(profile config.SQLConnection) string {
	if mode := strings.TrimSpace(profile.Auth); mode != "" {
		return mode
	}
	return "pgpass"
}

func (w *mainWindow) selectSQLProfileRow() {
	position := w.sqlProfileTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredSQLProfiles) {
		w.selectedSQLProfile = ""
		return
	}
	profile := w.filteredSQLProfiles[position]
	w.selectedSQLProfile = profile.Name
	w.setBreadcrumb("SQL Workbench / Connections / " + profile.Name)
	w.setStatus("Selected SQL connection "+profile.Name+"; double-click to open a query tab", false)
}

func (w *mainWindow) openSQLProfileAt(position uint) {
	if int(position) >= len(w.filteredSQLProfiles) {
		return
	}
	w.selectedSQLProfile = w.filteredSQLProfiles[position].Name
	w.openSQLTab(w.filteredSQLProfiles[position], sqlworkbench.TabState{})
}

func (w *mainWindow) openSelectedSQLProfile() {
	var profile config.SQLConnection
	var found bool
	if w.currentPage == pageSQLObjects {
		if tab := w.activeSQLTab(); tab != nil {
			profile, found = w.sqlProfileNamed(tab.state.ProfileName)
		}
	} else {
		profile, found = w.sqlProfileNamed(w.selectedSQLProfile)
	}
	if !found {
		w.setStatus("Select a SQL connection or activate an existing query tab first", true)
		return
	}
	w.openSQLTab(profile, sqlworkbench.TabState{})
}

func (w *mainWindow) restoreSQLTabs() {
	for _, state := range w.sqlState.Tabs {
		state.AllowWrites = false
		profile, found := w.sqlProfileNamedInConfig(state.ProfileName)
		if !found {
			profile = config.SQLConnection{Name: state.ProfileName, Database: "unavailable"}
		}
		w.openSQLTab(profile, state)
	}
	for index, tab := range w.sqlTabs {
		if tab.state.ID == w.sqlState.ActiveTabID {
			w.sqlNotebook.SetCurrentPage(index)
			break
		}
	}
}

func (w *mainWindow) openSQLTab(profile config.SQLConnection, state sqlworkbench.TabState) {
	if state.ID == "" {
		state.ID = fmt.Sprintf("sql-%d", time.Now().UnixNano())
		state.ProfileName = profile.Name
		state.UpdatedAt = time.Now()
	}
	editor := newSourceEditor(sourceDocument{Path: "query.sql", Language: "sql"})
	editor.SetText(state.Query)
	editor.ApplyPalette(w.currentSemanticPalette(w.window.StyleContext()))
	editorScroll := gtk.NewScrolledWindow()
	editorScroll.SetHExpand(true)
	editorScroll.SetVExpand(true)
	editorScroll.SetChild(editor.Widget())

	resultTable := newMultiStringTable(nil)
	resultTable.view.SetTooltipText("Select one or more rows to copy; double-click a cell to inspect its complete value")
	resultScroll := gtk.NewScrolledWindow()
	resultScroll.SetHExpand(true)
	resultScroll.SetVExpand(true)
	resultScroll.SetChild(resultTable.view)
	// GtkColumnView cannot configure a valid horizontal adjustment while it
	// has no columns. Keep the empty result table out of allocation until a
	// query supplies a schema; otherwise GTK 4.20 emits a critical on every
	// layout pass.
	resultScroll.SetVisible(false)
	resultLabel := gtk.NewLabel("No query has been run in this tab.")
	resultLabel.SetXAlign(0)
	resultLabel.SetHExpand(true)
	resultLabel.AddCSSClass("muted")
	// One entry per statement of the last run, so a script's earlier result
	// sets stay reachable after later statements run.
	resultChoices := gtk.NewStringList(nil)
	resultChooser := gtk.NewDropDown(resultChoices, nil)
	resultChooser.SetTooltipText("Every statement of the last run, in order")
	resultChooser.SetVisible(false)
	resultHeader := gtk.NewBox(gtk.OrientationHorizontal, 8)
	resultHeader.Append(resultChooser)
	resultHeader.Append(resultLabel)
	failureLabel := gtk.NewLabel("")
	failureLabel.SetXAlign(0)
	failureLabel.SetYAlign(0)
	failureLabel.SetWrap(true)
	failureLabel.SetSelectable(true)
	failureLabel.AddCSSClass("error")
	failureLabel.SetVisible(false)
	eventsLabel := gtk.NewLabel("")
	eventsLabel.SetXAlign(0)
	eventsLabel.SetWrap(true)
	eventsLabel.AddCSSClass("semantic-warning")
	eventsLabel.SetVisible(false)
	noticesLabel := gtk.NewLabel("")
	noticesLabel.SetXAlign(0)
	noticesLabel.SetWrap(true)
	noticesLabel.SetSelectable(true)
	noticesLabel.AddCSSClass("muted")
	noticesLabel.SetVisible(false)
	resultBox := gtk.NewBox(gtk.OrientationVertical, 6)
	resultBox.SetMarginTop(6)
	resultBox.Append(resultHeader)
	resultBox.Append(eventsLabel)
	resultBox.Append(failureLabel)
	resultBox.Append(noticesLabel)
	resultBox.Append(resultScroll)

	transactionLabel := gtk.NewLabel("")
	transactionLabel.SetXAlign(0)
	transactionLabel.SetHExpand(true)
	transactionLabel.AddCSSClass("semantic-error")
	commitButton := gtk.NewButtonWithLabel("Commit…")
	commitButton.AddCSSClass("destructive-action")
	commitButton.SetTooltipText("Make the open transaction's changes permanent")
	rollbackButton := gtk.NewButtonWithLabel("Roll back")
	rollbackButton.SetTooltipText("Discard the open transaction's changes and release its locks")
	transactionBar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	transactionBar.AddCSSClass("log-toolbar")
	transactionBar.Append(transactionLabel)
	transactionBar.Append(rollbackButton)
	transactionBar.Append(commitButton)
	transactionBar.SetVisible(false)

	structureBuffer := gtk.NewTextBuffer(nil)
	structureBuffer.SetText("Select a table, view, sequence, or function in the Browser Pane.")
	structureView := gtk.NewTextViewWithBuffer(structureBuffer)
	structureView.SetEditable(false)
	structureView.SetCursorVisible(true)
	structureView.SetMonospace(true)
	structureView.SetWrapMode(gtk.WrapWordChar)
	structureView.AddCSSClass("inspector")
	structureScroll := gtk.NewScrolledWindow()
	structureScroll.SetHExpand(true)
	structureScroll.SetVExpand(true)
	structureScroll.SetChild(structureView)

	definitionEditor := newSourceEditor(sourceDocument{Path: "definition.sql", Language: "sql"})
	definitionEditor.SetText("Select a database object to inspect its SQL definition.")
	definitionEditor.ApplyPalette(w.currentSemanticPalette(w.window.StyleContext()))
	if definitionView, ok := definitionEditor.Widget().(*gtk.TextView); ok {
		definitionView.SetEditable(false)
		definitionView.SetCursorVisible(true)
	}
	definitionScroll := gtk.NewScrolledWindow()
	definitionScroll.SetHExpand(true)
	definitionScroll.SetVExpand(true)
	definitionScroll.SetChild(definitionEditor.Widget())

	contextNotebook := gtk.NewNotebook()
	// Reuse the normalized SQL notebook surface so themes cannot draw their
	// own segmented border between this contextual tab strip and its content.
	contextNotebook.AddCSSClass("e9s-sql-notebook")
	contextNotebook.AddCSSClass("e9s-sql-context-notebook")
	contextNotebook.SetHExpand(true)
	contextNotebook.SetVExpand(true)
	contextNotebook.AppendPage(resultBox, gtk.NewLabel("Results"))
	contextNotebook.AppendPage(structureScroll, gtk.NewLabel("Structure"))
	contextNotebook.AppendPage(definitionScroll, gtk.NewLabel("Definition"))
	reportBuffer := gtk.NewTextBuffer(nil)
	reportBuffer.SetText("Run SQL in this tab to see a report of every statement.")
	reportView := gtk.NewTextViewWithBuffer(reportBuffer)
	reportView.SetEditable(false)
	reportView.SetCursorVisible(true)
	reportView.SetMonospace(true)
	reportView.AddCSSClass("inspector")
	reportScroll := gtk.NewScrolledWindow()
	reportScroll.SetHExpand(true)
	reportScroll.SetVExpand(true)
	reportScroll.SetChild(reportView)
	saveReportButton := gtk.NewButtonWithLabel("Save report…")
	copyReportButton := gtk.NewButtonWithLabel("Copy report")
	reportToolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	reportToolbar.AddCSSClass("log-toolbar")
	reportToolbar.Append(saveReportButton)
	reportToolbar.Append(copyReportButton)
	reportBox := gtk.NewBox(gtk.OrientationVertical, 0)
	reportBox.Append(reportToolbar)
	reportBox.Append(reportScroll)
	contextNotebook.AppendPage(reportBox, gtk.NewLabel("Report"))

	previewButton := gtk.NewButtonWithLabel("Preview 100 rows")
	generateButton := gtk.NewButtonWithLabel("Generate SELECT")
	insertNameButton := gtk.NewButtonWithLabel("Insert name")
	insertNameButton.SetTooltipText("Insert the selected object or column name at the query cursor")
	insertQualifiedButton := gtk.NewButtonWithLabel("Insert qualified")
	insertQualifiedButton.SetTooltipText("Insert the selected schema-qualified object or column name at the query cursor")
	copyObjectButton := gtk.NewButtonWithLabel("Copy qualified name")
	definitionButton := gtk.NewButtonWithLabel("Open definition")
	refreshObjectButton := gtk.NewButtonWithLabel("Refresh metadata")
	objectToolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	objectToolbar.AddCSSClass("log-toolbar")
	objectToolbar.Append(previewButton)
	objectToolbar.Append(generateButton)
	objectToolbar.Append(insertNameButton)
	objectToolbar.Append(insertQualifiedButton)
	objectToolbar.Append(copyObjectButton)
	objectToolbar.Append(definitionButton)
	objectToolbar.Append(refreshObjectButton)
	objectToolbar.SetVisible(false)
	lowerBox := gtk.NewBox(gtk.OrientationVertical, 0)
	lowerBox.Append(transactionBar)
	lowerBox.Append(objectToolbar)
	lowerBox.Append(contextNotebook)

	split := gtk.NewPaned(gtk.OrientationVertical)
	split.AddCSSClass("pane-split")
	split.SetStartChild(editorScroll)
	split.SetEndChild(lowerBox)
	split.SetPosition(320)
	split.SetResizeStartChild(true)
	split.SetResizeEndChild(true)
	tab := &sqlWorkbenchTab{
		state: state, editor: editor, page: split, tabLabel: gtk.NewLabel(""),
		resultTable: resultTable, resultScroll: resultScroll, resultLabel: resultLabel,
		resultChooser: resultChooser, resultChoices: resultChoices, failureLabel: failureLabel, eventsLabel: eventsLabel, noticesLabel: noticesLabel,
		transactionBar: transactionBar, transactionLabel: transactionLabel, commitButton: commitButton, rollbackButton: rollbackButton,
		reportBuffer:    reportBuffer,
		contextNotebook: contextNotebook, structureBuffer: structureBuffer, definitionBuffer: definitionEditor.Buffer(), definitionEditor: definitionEditor,
		objectToolbar: objectToolbar, previewButton: previewButton, generateButton: generateButton,
		insertNameButton: insertNameButton, insertQualifiedButton: insertQualifiedButton,
		copyObjectButton: copyObjectButton, definitionButton: definitionButton, refreshObjectButton: refreshObjectButton,
		explorer: newSQLObjectExplorer(state.Explorer),
	}
	resultTable.cellActivated = func(position uint, field int) {
		w.showSQLResultCell(tab, int(position), field)
	}
	w.installSQLResultContextMenu(tab)
	resultChooser.NotifyProperty("selected", func() {
		if tab.choosingResult {
			return
		}
		tab.selectedResult = int(resultChooser.Selected())
		w.renderSelectedSQLResult(tab)
	})
	commitButton.ConnectClicked(func() { w.confirmCommitSQLTransaction(tab) })
	saveReportButton.ConnectClicked(func() { w.saveSQLReport(tab) })
	copyReportButton.ConnectClicked(func() {
		if report, ok := w.sqlReport(tab); ok {
			w.window.Clipboard().SetText(report)
			w.setStatus("Copied the SQL run report", false)
		}
	})
	rollbackButton.ConnectClicked(func() { w.endSQLTransaction(tab, false) })
	previewButton.ConnectClicked(func() { w.previewSQLObject(tab) })
	generateButton.ConnectClicked(func() { w.generateSQLSelect(tab) })
	insertNameButton.ConnectClicked(func() { w.insertSQLBrowserSelection(tab, false) })
	insertQualifiedButton.ConnectClicked(func() { w.insertSQLBrowserSelection(tab, true) })
	copyObjectButton.ConnectClicked(func() { w.copySQLObjectName(tab) })
	definitionButton.ConnectClicked(func() { tab.contextNotebook.SetCurrentPage(2) })
	refreshObjectButton.ConnectClicked(func() { w.refreshSQLObjectDetail(tab) })
	tab.tabLabel.SetEllipsize(pango.EllipsizeMiddle)
	tab.tabLabel.SetMaxWidthChars(28)
	editor.ConnectChanged(func() {
		tab.state.Query = editor.Text()
		tab.state.UpdatedAt = time.Now()
		w.scheduleSQLStateSave()
	})
	w.sqlTabs = append(w.sqlTabs, tab)
	page := w.sqlNotebook.AppendPage(tab.page, w.sqlTabLabel(tab))
	w.sqlPaneStack.SetVisibleChildName("tabs")
	w.updateSQLTabTitle(tab)
	w.sqlNotebook.SetCurrentPage(page)
	w.sqlState.ActiveTabID = tab.state.ID
	w.scheduleSQLStateSave()
	w.updateSQLControls()
	if moduleForPage(w.currentPage) == moduleSQLWorkbench {
		w.showSQLObjectBrowser(tab)
	}
	glib.IdleAdd(func() {
		if view, ok := editor.Widget().(*gtk.TextView); ok {
			view.GrabFocus()
		}
	})
}

func (w *mainWindow) sqlTabLabel(tab *sqlWorkbenchTab) *gtk.Box {
	box := gtk.NewBox(gtk.OrientationHorizontal, 4)
	box.Append(tab.tabLabel)
	closeButton := gtk.NewButtonFromIconName("window-close-symbolic")
	closeButton.AddCSSClass("flat")
	closeButton.AddCSSClass("terminal-tab-close")
	closeButton.SetTooltipText("Close this query tab")
	closeButton.ConnectClicked(func() { w.closeSQLTab(tab) })
	box.Append(closeButton)
	return box
}

func (w *mainWindow) activeSQLTab() *sqlWorkbenchTab {
	if w.sqlNotebook == nil {
		return nil
	}
	page := w.sqlNotebook.CurrentPage()
	if page < 0 || page >= len(w.sqlTabs) {
		return nil
	}
	return w.sqlTabs[page]
}

func (w *mainWindow) closeActiveSQLTab() { w.closeSQLTab(w.activeSQLTab()) }

// closeSQLTab closes a tab and its connection, asking first when that would
// discard an open transaction.
func (w *mainWindow) closeSQLTab(tab *sqlWorkbenchTab) {
	if tab == nil || w.sqlActionPending {
		return
	}
	if w.sqlTransactionOpen(tab) {
		w.confirmDiscardSQLTransaction(tab, "Close "+w.sqlTabName(tab)+"?", "Close tab", func() { w.closeSQLTabNow(tab) })
		return
	}
	w.closeSQLTabNow(tab)
}

func (w *mainWindow) closeSQLTabNow(tab *sqlWorkbenchTab) {
	index := -1
	for candidate, existing := range w.sqlTabs {
		if existing == tab {
			index = candidate
			break
		}
	}
	if index < 0 {
		return
	}
	if tab.explorer.detailCancel != nil {
		tab.explorer.detailCancel()
	}
	if tab.explorer.catalogCancel != nil {
		tab.explorer.catalogCancel()
	}
	if w.sqlExecutor != nil {
		w.sqlExecutor.CloseSession(tab.state.ID)
	}
	w.sqlNotebook.RemovePage(index)
	w.sqlTabs = append(w.sqlTabs[:index], w.sqlTabs[index+1:]...)
	if len(w.sqlTabs) == 0 {
		w.sqlPaneStack.SetVisibleChildName("empty")
	}
	if active := w.activeSQLTab(); active != nil {
		w.sqlState.ActiveTabID = active.state.ID
	} else {
		w.sqlState.ActiveTabID = ""
	}
	if w.currentPage == pageSQLObjects {
		if active := w.activeSQLTab(); active != nil {
			w.showSQLObjectBrowser(active)
		} else {
			w.openSQLWorkbenchModule()
		}
	}
	w.scheduleSQLStateSave()
	w.updateSQLControls()
}

func (w *mainWindow) updateSQLTabTitle(tab *sqlWorkbenchTab) {
	if tab == nil {
		return
	}
	title := strings.TrimSpace(tab.state.Title)
	if title == "" {
		title = tab.state.ProfileName
	}
	if title == "" {
		title = "SQL query"
	}
	tab.tabLabel.SetLabel(title)
	width := utf8.RuneCountInString(title)
	if width < 8 {
		width = 8
	}
	if width > 28 {
		width = 28
	}
	tab.tabLabel.SetWidthChars(width)
	tab.tabLabel.SetTooltipText(title + "\nConnection: " + tab.state.ProfileName)
}

func (w *mainWindow) promptRenameSQLTab() {
	tab := w.activeSQLTab()
	if tab == nil {
		return
	}
	dialog := gtk.NewDialogWithFlags("Rename SQL tab", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	entry := gtk.NewEntry()
	entry.SetText(tab.state.Title)
	entry.SetPlaceholderText(tab.state.ProfileName)
	entry.SetActivatesDefault(true)
	content.Append(gtk.NewLabel("Tab title (blank follows the connection name)"))
	content.Append(entry)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Rename", int(gtk.ResponseOK))
	dialog.SetDefaultResponse(int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response == int(gtk.ResponseOK) {
			tab.state.Title = strings.TrimSpace(entry.Text())
			tab.state.ManualTitle = tab.state.Title != ""
			tab.state.UpdatedAt = time.Now()
			w.updateSQLTabTitle(tab)
			w.scheduleSQLStateSave()
		}
		dialog.Destroy()
	})
	dialog.Present()
}

type sqlRunMode int

const (
	sqlRunSelection sqlRunMode = iota
	sqlRunCurrent
	sqlRunAll
	sqlRunDry
	sqlRunTransaction
)

func (m sqlRunMode) executorMode() sqlworkbench.RunMode {
	switch m {
	case sqlRunDry:
		return sqlworkbench.RunDryRun
	case sqlRunTransaction:
		return sqlworkbench.RunInTransaction
	default:
		return sqlworkbench.RunAsWritten
	}
}

func (w *mainWindow) runSelectedOrCurrentSQL() {
	tab := w.activeSQLTab()
	if tab == nil {
		return
	}
	_, _, selected := tab.editor.Buffer().SelectionBounds()
	if selected {
		w.runActiveSQL(sqlRunSelection)
		return
	}
	w.runActiveSQL(sqlRunCurrent)
}

func (w *mainWindow) runActiveSQL(mode sqlRunMode) {
	tab := w.activeSQLTab()
	if tab == nil || w.sqlActionPending || w.sqlExecutor == nil {
		return
	}
	profile, found := w.sqlProfileNamed(tab.state.ProfileName)
	if !found {
		w.setStatus("SQL connection profile "+tab.state.ProfileName+" is no longer configured", true)
		return
	}
	source := tab.editor.Text()
	switch mode {
	case sqlRunSelection:
		start, end, selected := tab.editor.Buffer().SelectionBounds()
		if !selected {
			w.setStatus("Select SQL text before using Run selection", true)
			return
		}
		source = tab.editor.Buffer().Text(start, end, true)
	case sqlRunCurrent:
		start, _ := tab.editor.Buffer().Bounds()
		cursor := tab.editor.Buffer().IterAtMark(tab.editor.Buffer().GetInsert())
		prefix := tab.editor.Buffer().Text(start, cursor, true)
		statement, found := sqlworkbench.CurrentStatement(source, len(prefix))
		if !found {
			w.setStatus("No SQL statement at the cursor", true)
			return
		}
		source = statement.SQL
	}
	tab.state.Query = tab.editor.Text()
	tab.state.UpdatedAt = time.Now()
	w.saveSQLStateNow()
	request := sqlworkbench.RunRequest{Profile: profile, SessionID: tab.state.ID, Source: source, AllowWrites: tab.state.AllowWrites, Mode: mode.executorMode(),
		Progress: w.sqlProgressReporter()}
	label := "Running SQL on " + profile.Name + "…"
	if request.Mode != sqlworkbench.RunAsWritten {
		label = "Running SQL on " + profile.Name + " (" + request.Mode.String() + ")…"
	}
	w.startSQLAction(tab, label, func(ctx context.Context) func() {
		outcome, err := w.sqlExecutor.Run(ctx, request)
		return func() {
			if err != nil {
				w.setStatus("SQL query failed: "+err.Error(), true)
				return
			}
			tab.run = outcome
			w.renderSQLResults(tab)
			w.setStatus(outcome.Summary(), outcome.Failure != nil)
			if outcome.Mode == sqlworkbench.RunInTransaction && outcome.Transaction == sqlworkbench.TxOpen {
				w.updateSQLTransactionBar(tab)
				w.reviewSQLTransaction(tab)
			}
		}
	})
}

// reviewSQLTransaction offers to end a transactional run's open
// transaction, listing what it changed. Keeping it open leaves the
// transaction bar's Commit and Roll back.
func (w *mainWindow) reviewSQLTransaction(tab *sqlWorkbenchTab) {
	changes := tab.run.ChangeSummary()
	if changes == "" {
		changes = "No statement reported changed rows."
	} else {
		changes = "Changes: " + changes
	}
	const keepOpen = 1
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageQuestion, gtk.ButtonsNone)
	dialog.SetTitle("Review transaction")
	dialog.SetMarkup("The run in <b>" + glib.MarkupEscapeText(w.sqlTabName(tab)) + "</b> succeeded. Commit its transaction?")
	dialog.SetObjectProperty("secondary-text", changes+"\n\nKeep it open to review the results first; it holds its locks until it ends.")
	dialog.AddButton("Roll back", int(gtk.ResponseReject))
	dialog.AddButton("Keep open", keepOpen)
	dialog.AddButton("Commit", int(gtk.ResponseAccept))
	dialog.SetDefaultResponse(keepOpen)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		switch response {
		case int(gtk.ResponseAccept):
			w.endSQLTransaction(tab, true)
		case int(gtk.ResponseReject):
			w.endSQLTransaction(tab, false)
		}
	})
	dialog.Present()
}

// sqlProgressReporter shows which statement a run is executing, at most a
// few times a second so a long script cannot flood the main loop.
func (w *mainWindow) sqlProgressReporter() func(sqlworkbench.Progress) {
	var last time.Time
	return func(progress sqlworkbench.Progress) {
		if now := time.Now(); now.Sub(last) >= 200*time.Millisecond || progress.Index == 0 {
			last = now
			text := fmt.Sprintf("Running statement %d of %d (line %d): %s", progress.Index+1, progress.Total, progress.Line, progress.Statement)
			glib.IdleAdd(func() {
				if w.sqlActionPending {
					w.setStatus(text, false)
				}
			})
		}
	}
}

// startSQLAction runs work for a tab off the UI thread, then applies its
// result. SQL actions have a context of their own: a page change or refresh
// elsewhere in the window must not cancel a script partway through.
func (w *mainWindow) startSQLAction(tab *sqlWorkbenchTab, label string, work func(context.Context) func()) {
	w.sqlActionPending = true
	ctx, cancel := context.WithCancel(w.ctx)
	w.sqlActionCancel = cancel
	w.updateSQLControls()
	w.spinner.Start()
	w.setStatus(label, false)
	go func() {
		apply := work(ctx)
		glib.IdleAdd(func() {
			cancel()
			w.sqlActionCancel = nil
			w.sqlActionPending = false
			w.spinner.Stop()
			if w.sqlTabIsOpen(tab) {
				apply()
				w.updateSQLTransactionBar(tab)
			}
			w.updateSQLControls()
		})
	}()
}

func (w *mainWindow) sqlTabIsOpen(tab *sqlWorkbenchTab) bool {
	for _, existing := range w.sqlTabs {
		if existing == tab {
			return true
		}
	}
	return false
}

func (w *mainWindow) sqlTabName(tab *sqlWorkbenchTab) string {
	if title := strings.TrimSpace(tab.state.Title); title != "" {
		return title
	}
	return tab.state.ProfileName
}

func (w *mainWindow) sqlTransactionOpen(tab *sqlWorkbenchTab) bool {
	return tab != nil && w.sqlExecutor != nil && w.sqlExecutor.SessionStatus(tab.state.ID).State == sqlworkbench.TxOpen
}

// updateSQLTransactionBar shows the tab's open transaction, refreshing its
// age every second while it stays open.
func (w *mainWindow) updateSQLTransactionBar(tab *sqlWorkbenchTab) bool {
	if tab == nil || w.sqlExecutor == nil {
		return false
	}
	text := w.sqlExecutor.SessionStatus(tab.state.ID).Describe(time.Now())
	tab.transactionLabel.SetLabel("● " + text)
	tab.transactionBar.SetVisible(text != "")
	tab.commitButton.SetSensitive(!w.sqlActionPending)
	tab.rollbackButton.SetSensitive(!w.sqlActionPending)
	open := text != ""
	if open && tab.transactionTimer == 0 {
		tab.transactionTimer = glib.TimeoutSecondsAdd(1, func() bool {
			if !w.sqlTabIsOpen(tab) || !w.updateSQLTransactionBar(tab) {
				tab.transactionTimer = 0
				return false
			}
			return true
		})
	}
	return open
}

func (w *mainWindow) confirmCommitSQLTransaction(tab *sqlWorkbenchTab) {
	if !w.sqlTransactionOpen(tab) || w.sqlActionPending {
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetTitle("Commit transaction")
	dialog.SetMarkup("Commit the open transaction in <b>" + glib.MarkupEscapeText(w.sqlTabName(tab)) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "Its changes become permanent and visible to other sessions.")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Commit", int(gtk.ResponseAccept))
	dialog.SetDefaultResponse(int(gtk.ResponseCancel))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseAccept) {
			w.endSQLTransaction(tab, true)
		}
	})
	dialog.Present()
}

func (w *mainWindow) endSQLTransaction(tab *sqlWorkbenchTab, commit bool) {
	if !w.sqlTransactionOpen(tab) || w.sqlActionPending {
		return
	}
	label := "Rolling back…"
	if commit {
		label = "Committing…"
	}
	tabID := tab.state.ID
	w.startSQLAction(tab, label, func(context.Context) func() {
		tag, err := w.sqlExecutor.EndTransaction(tabID, commit)
		return func() {
			switch {
			case err != nil:
				w.setStatus(err.Error(), true)
			case commit && tag != "COMMIT":
				w.setStatus("The transaction had failed, so PostgreSQL rolled it back instead of committing ("+tag+")", true)
			default:
				w.setStatus("Transaction ended: "+tag, false)
			}
		}
	})
}

// confirmDiscardSQLTransaction asks before an action that ends the tab's
// connection and so rolls back its open transaction.
func (w *mainWindow) confirmDiscardSQLTransaction(tab *sqlWorkbenchTab, question, action string, proceed func()) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetTitle("Open transaction")
	dialog.SetMarkup(glib.MarkupEscapeText(question))
	dialog.SetObjectProperty("secondary-text", w.sqlTabName(tab)+" has an open transaction. Continuing rolls it back: none of its changes are kept.")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton(action, int(gtk.ResponseAccept))
	dialog.SetDefaultResponse(int(gtk.ResponseCancel))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseAccept) {
			proceed()
		}
	})
	dialog.Present()
}

// renderSQLResults lists every entry of the tab's run and opens the failed
// statement, if there is one, or else the last result.
func (w *mainWindow) renderSQLResults(tab *sqlWorkbenchTab) {
	if tab == nil {
		return
	}
	labels := make([]string, tab.run.EntryCount())
	for index := range labels {
		labels[index] = tab.run.EntryLabel(index)
	}
	tab.choosingResult = true
	tab.resultChoices.Splice(0, tab.resultChoices.NItems(), labels)
	tab.selectedResult = max(0, len(labels)-1)
	if len(labels) > 0 {
		tab.resultChooser.SetSelected(uint(tab.selectedResult))
	}
	tab.choosingResult = false
	tab.resultChooser.SetVisible(len(labels) > 1)
	tab.eventsLabel.SetLabel(strings.Join(tab.run.Events, "\n"))
	tab.eventsLabel.SetVisible(len(tab.run.Events) > 0)
	if report, ok := w.sqlReport(tab); ok {
		tab.reportBuffer.SetText(report)
	}
	w.renderSelectedSQLResult(tab)
	w.updateSQLControls()
}

func (w *mainWindow) renderSelectedSQLResult(tab *sqlWorkbenchTab) {
	// Hide before changing the schema so a result without columns is never
	// allocated in GtkColumnView's transient zero-width state.
	tab.resultScroll.SetVisible(false)
	tab.failureLabel.SetVisible(false)
	if tab.contextNotebook.CurrentPage() != sqlReportPage {
		tab.contextNotebook.SetCurrentPage(0)
	}
	if tab.run.EntryCount() == 0 {
		tab.resultLabel.SetLabel("The query completed without a result set.")
		return
	}
	result, ok := sqlTabSelectedResult(tab)
	if !ok {
		failure := tab.run.Failure
		tab.failureLabel.SetLabel(sqlFailureText(*failure))
		tab.failureLabel.SetVisible(true)
		tab.resultLabel.SetLabel(tab.run.Summary())
		w.showSQLNotices(tab, failure.Notices)
		return
	}
	w.showSQLNotices(tab, result.Notices)
	columns := make([]columnSpec, len(result.Columns))
	for index, title := range result.Columns {
		columns[index] = columnSpec{title: title, field: index, expand: true}
	}
	tab.resultTable.setColumns(columns)
	rows := make([]string, len(result.Rows))
	for index, row := range result.Rows {
		values := make([]string, len(row))
		for field, value := range row {
			values[field] = strings.NewReplacer("\t", "    ", "\r", "", "\n", " ↵ ").Replace(value)
		}
		rows[index] = strings.Join(values, "\t")
	}
	tab.resultTable.replace(rows)
	if len(columns) > 0 {
		tab.resultScroll.SetVisible(true)
	}
	status := fmt.Sprintf("%s • %d row(s) • %d ms", sqlworkbench.ResultTag(result), len(result.Rows), result.DurationMS)
	if len(columns) == 0 {
		status = fmt.Sprintf("%s • no result set • %d ms", sqlworkbench.ResultTag(result), result.DurationMS)
	}
	tab.resultLabel.SetLabel(status)
}

// sqlFailureText describes a failed statement and shows its start.
func sqlFailureText(failure sqlworkbench.StatementFailure) string {
	heading := fmt.Sprintf("Statement %d failed", failure.Index+1)
	if failure.Line > 0 {
		heading += fmt.Sprintf(" at line %d", failure.Line)
	}
	text := heading + "\n" + failure.Message
	if failure.SQL != "" {
		lines := strings.Split(failure.SQL, "\n")
		if len(lines) > 12 {
			lines = append(lines[:12], fmt.Sprintf("… %d more line(s)", len(lines)-12))
		}
		text += "\n\n" + strings.Join(lines, "\n")
	}
	return text
}

// sqlTabSelectedResult returns the selected result set of the tab's run. It
// reports false when nothing ran or the failed statement is selected.
func sqlTabSelectedResult(tab *sqlWorkbenchTab) (model.SQLQueryResult, bool) {
	if tab == nil || tab.selectedResult < 0 || tab.selectedResult >= len(tab.run.Results) {
		return model.SQLQueryResult{}, false
	}
	return tab.run.Results[tab.selectedResult], true
}

func (w *mainWindow) installSQLResultContextMenu(tab *sqlWorkbenchTab) {
	if tab == nil || tab.resultTable == nil {
		return
	}
	menu := gtk.NewBox(gtk.OrientationVertical, 2)
	menu.SetMarginTop(6)
	menu.SetMarginBottom(6)
	menu.SetMarginStart(6)
	menu.SetMarginEnd(6)

	copyRows := gtk.NewButtonWithLabel("Copy")
	copyRows.AddCSSClass("flat")
	copyRows.SetHAlign(gtk.AlignFill)
	menu.Append(copyRows)
	copyWithHeaders := gtk.NewButtonWithLabel("Copy with headers")
	copyWithHeaders.AddCSSClass("flat")
	copyWithHeaders.SetHAlign(gtk.AlignFill)
	menu.Append(copyWithHeaders)

	popover := gtk.NewPopover()
	popover.AddCSSClass("menu")
	popover.SetChild(menu)
	popover.SetParent(tab.resultTable.view)
	copyRows.ConnectClicked(func() {
		w.copySelectedSQLResultRows(tab, false)
		popover.Popdown()
	})
	copyWithHeaders.ConnectClicked(func() {
		w.copySelectedSQLResultRows(tab, true)
		popover.Popdown()
	})

	click := gtk.NewGestureClick()
	click.SetButton(gdk.BUTTON_SECONDARY)
	click.SetPropagationPhase(gtk.PhaseCapture)
	click.ConnectPressed(func(_ int, x, y float64) {
		hasSelection := len(tab.resultTable.selectedPositions()) > 0
		copyRows.SetSensitive(hasSelection)
		copyWithHeaders.SetSensitive(hasSelection)
		rectangle := gdk.NewRectangle(int(x), int(y), 1, 1)
		popover.SetPointingTo(&rectangle)
		popover.Popup()
	})
	tab.resultTable.view.AddController(click)
}

func (w *mainWindow) copySelectedSQLResultRows(tab *sqlWorkbenchTab, includeHeaders bool) {
	result, ok := sqlTabSelectedResult(tab)
	if !ok {
		return
	}
	positions := tab.resultTable.selectedPositions()
	text, rows, err := formatSQLResultRows(result, positions, includeHeaders)
	if err != nil {
		w.setStatus("Copying SQL result rows failed: "+err.Error(), true)
		return
	}
	if rows == 0 {
		w.setStatus("Select one or more SQL result rows to copy", true)
		return
	}
	w.window.Clipboard().SetText(text)
	label := fmt.Sprintf("Copied %d SQL result row(s)", rows)
	if includeHeaders {
		label += " with headers"
	}
	w.setStatus(label, false)
}

func formatSQLResultRows(result model.SQLQueryResult, positions []int, includeHeaders bool) (string, int, error) {
	var out strings.Builder
	writer := csv.NewWriter(&out)
	writer.Comma = '\t'
	if includeHeaders && len(result.Columns) > 0 {
		if err := writer.Write(result.Columns); err != nil {
			return "", 0, err
		}
	}
	written := 0
	for _, position := range positions {
		if position < 0 || position >= len(result.Rows) {
			continue
		}
		if err := writer.Write(result.Rows[position]); err != nil {
			return "", 0, err
		}
		written++
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", 0, err
	}
	return strings.TrimSuffix(out.String(), "\n"), written, nil
}

func (w *mainWindow) showSQLResultCell(tab *sqlWorkbenchTab, row, field int) {
	result, ok := sqlTabSelectedResult(tab)
	if !ok {
		return
	}
	column, value, ok := sqlResultCell(result, row, field)
	if !ok {
		return
	}

	dialog := gtk.NewDialogWithFlags("SQL result cell", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(840, 560)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)

	contextLabel := gtk.NewLabel(fmt.Sprintf("Column %s • row %d", column, row+1))
	contextLabel.SetXAlign(0)
	contextLabel.SetSelectable(true)
	content.Append(contextLabel)

	document := sourceDocument{Path: "result.txt"}
	if json.Valid([]byte(value)) {
		document = sourceDocument{Path: "result.json", Language: "json"}
	}
	editor := newSourceEditor(document)
	editor.SetText(value)
	editor.ApplyPalette(w.currentSemanticPalette(w.window.StyleContext()))
	if view, ok := editor.Widget().(*gtk.TextView); ok {
		view.SetEditable(false)
		view.SetCursorVisible(true)
		view.SetWrapMode(gtk.WrapWordChar)
	}
	scroll := gtk.NewScrolledWindow()
	scroll.SetHExpand(true)
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetChild(editor.Widget())
	content.Append(scroll)

	const copyAllResponse = 101
	dialog.AddButton("Copy all", copyAllResponse)
	dialog.AddButton("Close", int(gtk.ResponseClose))
	dialog.ConnectResponse(func(response int) {
		if response == copyAllResponse {
			w.window.Clipboard().SetText(value)
			w.setStatus(fmt.Sprintf("Copied SQL result cell from %s, row %d", column, row+1), false)
			return
		}
		dialog.Destroy()
	})
	dialog.Present()
}

func sqlResultCell(result model.SQLQueryResult, row, field int) (string, string, bool) {
	if row < 0 || row >= len(result.Rows) || field < 0 || field >= len(result.Columns) || field >= len(result.Rows[row]) {
		return "", "", false
	}
	return result.Columns[field], result.Rows[row][field], true
}

// reconnectActiveSQLTab replaces the tab's connection, asking first when
// that would discard an open transaction.
func (w *mainWindow) reconnectActiveSQLTab() {
	tab := w.activeSQLTab()
	if tab == nil || w.sqlActionPending || w.sqlExecutor == nil {
		return
	}
	profile, found := w.sqlProfileNamed(tab.state.ProfileName)
	if !found {
		w.setStatus("SQL connection profile "+tab.state.ProfileName+" is no longer configured", true)
		return
	}
	reconnect := func() {
		tabID := tab.state.ID
		w.startSQLAction(tab, "Reconnecting to "+profile.Name+"…", func(ctx context.Context) func() {
			err := w.sqlExecutor.Reconnect(ctx, profile, tabID, true)
			return func() {
				if err != nil {
					w.setStatus("SQL reconnect failed: "+err.Error(), true)
					return
				}
				w.setStatus("Connected to "+profile.Name, false)
			}
		})
	}
	if w.sqlTransactionOpen(tab) {
		w.confirmDiscardSQLTransaction(tab, "Reconnect "+w.sqlTabName(tab)+"?", "Reconnect", reconnect)
		return
	}
	reconnect()
}

func (w *mainWindow) toggleSQLWrites() {
	if w.sqlWritesUpdating {
		return
	}
	tab := w.activeSQLTab()
	if tab == nil {
		return
	}
	if !w.sqlWritesButton.Active() {
		tab.state.AllowWrites = false
		w.sqlWritesButton.SetLabel("Writes locked")
		w.scheduleSQLStateSave()
		return
	}
	if w.options.Config == nil || !w.options.Config.SQL.AllowWrites {
		w.sqlWritesButton.SetActive(false)
		w.setStatus("SQL writes are disabled globally; enable them in Settings / Safety first", true)
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Enable SQL writes for this tab")
	dialog.SetMarkup("Break the glass and permit data-changing SQL in <b>" + tab.state.ProfileName + "</b>?")
	dialog.SetObjectProperty("secondary-text", "This applies only to this tab and is restored in the locked state when e9s restarts.")
	dialog.SetDefaultResponse(int(gtk.ResponseNo))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			tab.state.AllowWrites = true
			w.sqlWritesButton.SetLabel("Writes enabled")
		} else {
			w.sqlWritesButton.SetActive(false)
		}
		w.scheduleSQLStateSave()
	})
	dialog.Present()
}

func (w *mainWindow) updateSQLControls() {
	tab := w.activeSQLTab()
	enabled := tab != nil && !w.sqlActionPending
	for _, button := range []*gtk.Button{w.sqlRunSelectionButton, w.sqlRunCurrentButton, w.sqlRunAllButton, w.sqlDryRunButton, w.sqlRunInTransactionButton, w.sqlReconnectButton} {
		if button != nil {
			button.SetSensitive(enabled)
		}
	}
	if w.sqlExportButton != nil {
		w.sqlExportButton.SetSensitive(enabled && sqlTabHasExportableResult(tab))
	}
	if w.sqlCancelButton != nil {
		w.sqlCancelButton.SetVisible(w.sqlActionCancel != nil)
	}
	if w.sqlWritesButton != nil {
		w.sqlWritesUpdating = true
		w.sqlWritesButton.SetSensitive(enabled)
		if tab == nil || !tab.state.AllowWrites {
			w.sqlWritesButton.SetActive(false)
			w.sqlWritesButton.SetLabel("Writes locked")
		} else {
			w.sqlWritesButton.SetActive(true)
			w.sqlWritesButton.SetLabel("Writes enabled")
		}
		w.sqlWritesUpdating = false
	}
	w.updateSQLObjectControls(tab)
}

func sqlTabHasExportableResult(tab *sqlWorkbenchTab) bool {
	result, ok := sqlTabSelectedResult(tab)
	return ok && len(result.Columns) > 0
}

func (w *mainWindow) exportActiveSQLResult() {
	tab := w.activeSQLTab()
	if !sqlTabHasExportableResult(tab) {
		w.setStatus("Run a query that returns columns before exporting CSV", true)
		return
	}
	result, _ := sqlTabSelectedResult(tab)
	chooser := gtk.NewFileChooserNative("Export SQL results", &w.window.Window, gtk.FileChooserActionSave, "Export", "Cancel")
	chooser.SetModal(true)
	chooser.SetCurrentName("query-results.csv")
	if w.options.Config != nil {
		_ = chooser.SetCurrentFolder(gio.NewFileForPath(w.options.Config.SaveDir()))
	}
	chooser.ConnectResponse(func(response int) {
		defer chooser.Destroy()
		if response != int(gtk.ResponseAccept) || chooser.File() == nil || chooser.File().Path() == "" {
			return
		}
		file, err := os.OpenFile(chooser.File().Path(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err == nil {
			err = sqlworkbench.WriteCSV(file, result)
			closeErr := file.Close()
			if err == nil {
				err = closeErr
			}
		}
		if err != nil {
			w.setStatus("Export SQL results: "+err.Error(), true)
			return
		}
		w.setStatus("Exported SQL results to "+chooser.File().Path(), false)
	})
	chooser.Show()
}

func (w *mainWindow) promptSQLPassword(ctx context.Context, profile config.SQLConnection) (string, error) {
	type response struct {
		password string
		err      error
	}
	result := make(chan response, 1)
	var once sync.Once
	send := func(value response) { once.Do(func() { result <- value }) }
	glib.IdleAdd(func() {
		if ctx.Err() != nil {
			send(response{err: ctx.Err()})
			return
		}
		dialog := gtk.NewDialogWithFlags("Database password", &w.window.Window, gtk.DialogModal)
		dialog.SetDestroyWithParent(true)
		content := dialog.ContentArea()
		content.SetSpacing(8)
		content.SetMarginTop(16)
		content.SetMarginBottom(16)
		content.SetMarginStart(16)
		content.SetMarginEnd(16)
		label := gtk.NewLabel("Password for " + profile.Name)
		label.SetXAlign(0)
		entry := gtk.NewEntry()
		entry.SetVisibility(false)
		entry.SetInputPurpose(gtk.InputPurposePassword)
		entry.SetActivatesDefault(true)
		content.Append(label)
		content.Append(entry)
		dialog.AddButton("Cancel", int(gtk.ResponseCancel))
		dialog.AddButton("Connect", int(gtk.ResponseOK))
		dialog.SetDefaultResponse(int(gtk.ResponseOK))
		dialog.ConnectResponse(func(code int) {
			if code == int(gtk.ResponseOK) {
				send(response{password: entry.Text()})
			} else {
				send(response{err: errors.New("password entry canceled")})
			}
			dialog.Destroy()
		})
		dialog.ConnectDestroy(func() { send(response{err: errors.New("password entry canceled")}) })
		dialog.Present()
	})
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case answer := <-result:
		return answer.password, answer.err
	}
}

func (w *mainWindow) sqlProfileNamed(name string) (config.SQLConnection, bool) {
	for _, profile := range w.sqlProfiles {
		if profile.Name == name {
			return profile, true
		}
	}
	return w.sqlProfileNamedInConfig(name)
}

func (w *mainWindow) sqlProfileNamedInConfig(name string) (config.SQLConnection, bool) {
	if w.options.Config != nil {
		for _, profile := range w.options.Config.SQL.Connections {
			if profile.Name == name {
				return profile, true
			}
		}
	}
	return config.SQLConnection{}, false
}

func (w *mainWindow) scheduleSQLStateSave() {
	if w.sqlStateSavePending {
		return
	}
	w.sqlStateSavePending = true
	glib.TimeoutAdd(500, func() bool {
		w.sqlStateSavePending = false
		w.saveSQLStateNow()
		return false
	})
}

func (w *mainWindow) saveSQLStateNow() {
	if w.sqlNotebook == nil {
		return
	}
	state := sqlworkbench.WorkbenchState{ActiveTabID: w.sqlState.ActiveTabID, Tabs: make([]sqlworkbench.TabState, 0, len(w.sqlTabs))}
	for _, tab := range w.sqlTabs {
		tab.state.Query = tab.editor.Text()
		// Write authorization is deliberately never restored across processes.
		persisted := tab.state
		persisted.AllowWrites = false
		state.Tabs = append(state.Tabs, persisted)
	}
	w.sqlState = state
	if err := sqlworkbench.SaveState(w.sqlStatePath, state); err != nil && w.status != nil {
		w.setStatus("Save SQL workbench state: "+err.Error(), true)
	}
}

func (w *mainWindow) manageSQLConnections() {
	if w.options.Config == nil {
		w.setStatus("Configuration is unavailable", true)
		return
	}
	profiles := append([]config.SQLConnection(nil), w.options.Config.SQL.Connections...)
	dialog := gtk.NewDialogWithFlags("SQL connection profiles", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(760, 720)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(12)
	content.SetMarginBottom(12)
	content.SetMarginStart(12)
	content.SetMarginEnd(12)

	profileLabels := []string{"New connection"}
	for _, profile := range profiles {
		profileLabels = append(profileLabels, profile.Name)
	}
	selector := gtk.NewDropDownFromStrings(profileLabels)
	selector.SetHExpand(true)
	name := gtk.NewEntry()
	resourceKinds := []settingsChoice{{label: "RDS instance", value: "rds-instance"}, {label: "RDS cluster", value: "rds-cluster"}}
	resourceKind := gtk.NewDropDownFromStrings(settingsChoiceLabels(resourceKinds))
	resourceID := gtk.NewEntry()
	host := gtk.NewEntry()
	port := gtk.NewSpinButtonWithRange(1, 65535, 1)
	port.SetValue(5432)
	database := gtk.NewEntry()
	user := gtk.NewEntry()
	authModes := []settingsChoice{
		{label: ".pgpass", value: "pgpass"}, {label: "RDS IAM token", value: "iam"},
		{label: "Secrets Manager", value: "secrets-manager"}, {label: "Ephemeral password prompt", value: "password"},
		{label: "RDS Data API", value: "data-api"},
	}
	auth := gtk.NewDropDownFromStrings(settingsChoiceLabels(authModes))
	pgpass := gtk.NewEntry()
	pgpass.SetPlaceholderText("Optional per-connection password file")
	secretARN := gtk.NewEntry()
	resourceARN := gtk.NewEntry()
	sslModes := []settingsChoice{{label: "Verify full", value: "verify-full"}, {label: "Require TLS", value: "require"}, {label: "Prefer TLS", value: "prefer"}, {label: "Disable TLS", value: "disable"}}
	sslMode := gtk.NewDropDownFromStrings(settingsChoiceLabels(sslModes))
	sslRootCert := gtk.NewEntry()
	sslRootCert.SetHExpand(true)
	sslRootCert.SetPlaceholderText("Optional PEM CA bundle; blank uses the default trust store")
	sslRootCertBrowse := gtk.NewButtonWithLabel("Browse…")
	sslRootCertControl := gtk.NewBox(gtk.OrientationHorizontal, 8)
	sslRootCertControl.Append(sslRootCert)
	sslRootCertControl.Append(sslRootCertBrowse)
	connectTimeout := gtk.NewSpinButtonWithRange(0, 300, 1)
	connectTimeout.SetValue(15)
	idleTransactionTimeout := gtk.NewSpinButtonWithRange(0, 86400, 30)
	idleTransactionTimeout.SetValue(0)
	statementTimeout := gtk.NewSpinButtonWithRange(0, 86400, 10)
	statementTimeout.SetValue(0)
	ssmInstance := gtk.NewEntry()
	ssmInstance.SetPlaceholderText("Blank disables SSM port forwarding")
	ssmHost := gtk.NewEntry()
	ssmPort := gtk.NewSpinButtonWithRange(0, 65535, 1)
	ssmLocalPort := gtk.NewSpinButtonWithRange(0, 65535, 1)

	content.Append(settingsRow("Profile", selector))
	content.Append(settingsRow("Name", name))
	content.Append(settingsRow("Resource kind", resourceKind))
	content.Append(settingsRow("RDS resource identifier", resourceID))
	content.Append(settingsRow("Host", host))
	content.Append(settingsRow("Port", port))
	content.Append(settingsRow("Database", database))
	content.Append(settingsRow("User", user))
	content.Append(settingsRow("Authentication", auth))
	content.Append(settingsRow("Per-connection .pgpass file", pgpass))
	content.Append(settingsRow("Secrets Manager ARN", secretARN))
	content.Append(settingsRow("Data API resource ARN", resourceARN))
	content.Append(settingsRow("TLS mode", sslMode))
	sslRootCertRow := settingsRow("TLS root certificate bundle", sslRootCertControl)
	content.Append(sslRootCertRow)
	content.Append(settingsNote("For verify-full, choose a PEM CA/root-certificate bundle only when the database certificate is not trusted by the platform or PostgreSQL default trust store."))
	content.Append(settingsRow("Connect timeout (seconds; 0 uses driver default)", connectTimeout))
	content.Append(settingsRow("Statement timeout (seconds; 0 leaves the server setting)", statementTimeout))
	content.Append(settingsRow("Idle transaction timeout (seconds; 0 uses 300)", idleTransactionTimeout))
	content.Append(settingsNote("PostgreSQL ends a query tab's session when it sits this long in an open transaction, rolling the transaction back, so a forgotten transaction cannot hold locks indefinitely."))
	content.Append(settingsNote("Optional SSM tunnel. e9s runs the AWS CLI start-session command and keeps that process scoped to the database connection."))
	content.Append(settingsRow("SSM managed instance ID", ssmInstance))
	content.Append(settingsRow("SSM remote host override", ssmHost))
	content.Append(settingsRow("SSM remote port override", ssmPort))
	content.Append(settingsRow("SSM local port (0 chooses a free port)", ssmLocalPort))
	content.Append(settingsNote("Passwords and IAM tokens are never saved. Password-mode profiles prompt at connect time; .pgpass files must use private permissions."))

	selectedProfile := func() (config.SQLConnection, bool) {
		index := int(selector.Selected()) - 1
		if index < 0 || index >= len(profiles) {
			return config.SQLConnection{}, false
		}
		return profiles[index], true
	}
	loadProfile := func() {
		profile, found := selectedProfile()
		if !found {
			profile = config.SQLConnection{Port: 5432, Auth: "pgpass", SSLMode: "verify-full", ConnectSecs: 15}
		}
		name.SetText(profile.Name)
		resourceKind.SetSelected(uint(settingsChoiceIndex(resourceKinds, profile.ResourceKind)))
		resourceID.SetText(profile.ResourceID)
		host.SetText(profile.Host)
		if profile.Port == 0 {
			port.SetValue(5432)
		} else {
			port.SetValue(float64(profile.Port))
		}
		database.SetText(profile.Database)
		user.SetText(profile.User)
		auth.SetSelected(uint(settingsChoiceIndex(authModes, sqlAuthMode(profile))))
		pgpass.SetText(profile.PGPassFile)
		secretARN.SetText(profile.SecretARN)
		resourceARN.SetText(profile.ResourceARN)
		sslMode.SetSelected(uint(settingsChoiceIndex(sslModes, firstValue(profile.SSLMode, "verify-full"))))
		sslRootCert.SetText(profile.SSLRootCert)
		connectTimeout.SetValue(float64(profile.ConnectSecs))
		idleTransactionTimeout.SetValue(float64(profile.IdleInTransactionSecs))
		statementTimeout.SetValue(float64(profile.StatementTimeoutSecs))
		if profile.SSMTunnel == nil {
			ssmInstance.SetText("")
			ssmHost.SetText("")
			ssmPort.SetValue(0)
			ssmLocalPort.SetValue(0)
		} else {
			ssmInstance.SetText(profile.SSMTunnel.InstanceID)
			ssmHost.SetText(profile.SSMTunnel.RemoteHost)
			ssmPort.SetValue(float64(profile.SSMTunnel.RemotePort))
			ssmLocalPort.SetValue(float64(profile.SSMTunnel.LocalPort))
		}
	}
	updateSSLRootCertSensitivity := func() {
		sslRootCertRow.SetSensitive(settingsChoiceValue(sslModes, sslMode.Selected()) == "verify-full")
	}
	selector.NotifyProperty("selected", loadProfile)
	sslMode.NotifyProperty("selected", updateSSLRootCertSensitivity)
	sslRootCertBrowse.ConnectClicked(func() {
		chooser := gtk.NewFileChooserNative("Choose TLS root certificate bundle", &dialog.Window, gtk.FileChooserActionOpen, "Choose", "Cancel")
		chooser.SetModal(true)
		if path := strings.TrimSpace(sslRootCert.Text()); path != "" {
			_ = chooser.SetFile(gio.NewFileForPath(path))
		}
		chooser.ConnectResponse(func(response int) {
			defer chooser.Destroy()
			if response == int(gtk.ResponseAccept) && chooser.File() != nil && chooser.File().Path() != "" {
				sslRootCert.SetText(chooser.File().Path())
			}
		})
		chooser.Show()
	})
	loadProfile()
	updateSSLRootCertSensitivity()

	dialog.AddButton("Close", int(gtk.ResponseClose))
	dialog.AddButton("Delete", int(gtk.ResponseReject))
	dialog.AddButton("Save", int(gtk.ResponseApply))
	dialog.ConnectResponse(func(response int) {
		if response == int(gtk.ResponseClose) || response == int(gtk.ResponseCancel) {
			dialog.Destroy()
			return
		}
		if response == int(gtk.ResponseReject) {
			profile, found := selectedProfile()
			if !found {
				w.setStatus("Choose an existing SQL connection to delete", true)
				return
			}
			updated := *w.options.Config
			updated.SQL.Connections = append([]config.SQLConnection(nil), w.options.Config.SQL.Connections...)
			for index := range updated.SQL.Connections {
				if updated.SQL.Connections[index].Name == profile.Name {
					updated.SQL.Connections = append(updated.SQL.Connections[:index], updated.SQL.Connections[index+1:]...)
					break
				}
			}
			if err := updated.Save(); err != nil {
				w.setStatus("Delete SQL connection: "+err.Error(), true)
				return
			}
			w.applyRuntimeSettings(updated)
			dialog.Destroy()
			w.refreshSQLProfiles(false)
			w.setStatus("Deleted SQL connection "+profile.Name, false)
			return
		}
		if response != int(gtk.ResponseApply) {
			return
		}
		profile := config.SQLConnection{
			Name: strings.TrimSpace(name.Text()), ResourceKind: settingsChoiceValue(resourceKinds, resourceKind.Selected()),
			ResourceID: strings.TrimSpace(resourceID.Text()), Host: strings.TrimSpace(host.Text()), Port: port.ValueAsInt(),
			Database: strings.TrimSpace(database.Text()), User: strings.TrimSpace(user.Text()),
			Auth: settingsChoiceValue(authModes, auth.Selected()), PGPassFile: strings.TrimSpace(pgpass.Text()),
			SecretARN: strings.TrimSpace(secretARN.Text()), ResourceARN: strings.TrimSpace(resourceARN.Text()),
			SSLMode: settingsChoiceValue(sslModes, sslMode.Selected()), SSLRootCert: strings.TrimSpace(sslRootCert.Text()),
			ConnectSecs: connectTimeout.ValueAsInt(), IdleInTransactionSecs: idleTransactionTimeout.ValueAsInt(),
			StatementTimeoutSecs: statementTimeout.ValueAsInt(),
		}
		if instance := strings.TrimSpace(ssmInstance.Text()); instance != "" {
			profile.SSMTunnel = &config.SSMTunnel{InstanceID: instance, RemoteHost: strings.TrimSpace(ssmHost.Text()),
				RemotePort: ssmPort.ValueAsInt(), LocalPort: ssmLocalPort.ValueAsInt()}
		}
		updated := *w.options.Config
		updated.SQL.Connections = append([]config.SQLConnection(nil), w.options.Config.SQL.Connections...)
		existing, selectedExisting := selectedProfile()
		replaced := false
		for index := range updated.SQL.Connections {
			if (selectedExisting && updated.SQL.Connections[index].Name == existing.Name) || updated.SQL.Connections[index].Name == profile.Name {
				updated.SQL.Connections[index] = profile
				replaced = true
				break
			}
		}
		if !replaced {
			updated.SQL.Connections = append(updated.SQL.Connections, profile)
		}
		if err := updated.Validate(); err != nil {
			w.setStatus("Invalid SQL connection: "+err.Error(), true)
			return
		}
		if err := updated.Save(); err != nil {
			w.setStatus("Save SQL connection: "+err.Error(), true)
			return
		}
		w.applyRuntimeSettings(updated)
		dialog.Destroy()
		w.refreshSQLProfiles(false)
		w.setStatus("Saved SQL connection "+profile.Name, false)
	})
	dialog.Present()
}

func settingsChoiceValue(choices []settingsChoice, selected uint) string {
	if int(selected) >= 0 && int(selected) < len(choices) {
		return choices[selected].value
	}
	return ""
}

// confirmCloseWithOpenSQLTransactions keeps the window open and asks first
// when closing it would roll back open transactions. It returns true to stop
// the close.
func (w *mainWindow) confirmCloseWithOpenSQLTransactions() bool {
	if w.sqlCloseConfirmed || w.sqlExecutor == nil {
		return false
	}
	open := w.sqlExecutor.OpenTransactions()
	if len(open) == 0 {
		return false
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetTitle("Open transactions")
	dialog.SetMarkup(fmt.Sprintf("Quit with %d open SQL transaction(s)?", len(open)))
	dialog.SetObjectProperty("secondary-text", "Quitting rolls them back: none of their changes are kept.")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Quit", int(gtk.ResponseAccept))
	dialog.SetDefaultResponse(int(gtk.ResponseCancel))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseAccept) {
			w.sqlCloseConfirmed = true
			w.window.Close()
		}
	})
	dialog.Present()
	return true
}

// sqlReportPage is the Report page of a tab's context notebook.
const sqlReportPage = 3

// sqlReport renders the tab's last run as a report.
func (w *mainWindow) sqlReport(tab *sqlWorkbenchTab) (string, bool) {
	if tab == nil || tab.run.EntryCount() == 0 {
		return "", false
	}
	context := sqlworkbench.ReportContext{Connection: tab.state.ProfileName, Tab: strings.TrimSpace(tab.state.Title), Started: tab.run.Started}
	if profile, found := w.sqlProfileNamed(tab.state.ProfileName); found {
		context.Database = profile.Database
	}
	return sqlworkbench.FormatReport(tab.run, context), true
}

func (w *mainWindow) saveSQLReport(tab *sqlWorkbenchTab) {
	report, ok := w.sqlReport(tab)
	if !ok {
		w.setStatus("Run SQL in this tab before saving a report", true)
		return
	}
	chooser := gtk.NewFileChooserNative("Save SQL report", &w.window.Window, gtk.FileChooserActionSave, "Save", "Cancel")
	chooser.SetModal(true)
	chooser.SetCurrentName("sql-report-" + time.Now().Format("20060102-150405") + ".txt")
	if w.options.Config != nil {
		_ = chooser.SetCurrentFolder(gio.NewFileForPath(w.options.Config.SaveDir()))
	}
	chooser.ConnectResponse(func(response int) {
		defer chooser.Destroy()
		if response != int(gtk.ResponseAccept) || chooser.File() == nil || chooser.File().Path() == "" {
			return
		}
		if err := os.WriteFile(chooser.File().Path(), []byte(report), 0o600); err != nil {
			w.setStatus("Save SQL report: "+err.Error(), true)
			return
		}
		w.setStatus("Saved SQL report to "+chooser.File().Path(), false)
	})
	chooser.Show()
}

// showSQLNotices shows the server's notices for the selected statement.
func (w *mainWindow) showSQLNotices(tab *sqlWorkbenchTab, notices []string) {
	tab.noticesLabel.SetLabel(strings.Join(notices, "\n"))
	tab.noticesLabel.SetVisible(len(notices) > 0)
}
