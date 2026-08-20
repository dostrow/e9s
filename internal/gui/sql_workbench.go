//go:build gui

package gui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/sqlworkbench"
)

type sqlWorkbenchTab struct {
	state       sqlworkbench.TabState
	editor      *sourceEditor
	page        gtk.Widgetter
	tabLabel    *gtk.Label
	resultTable *stringTable
	resultLabel *gtk.Label
	results     []model.SQLQueryResult
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
	w.sqlRunSelectionButton.ConnectClicked(func() { w.runActiveSQL(sqlRunSelection) })
	w.sqlRunCurrentButton = gtk.NewButtonWithLabel("Run current")
	w.sqlRunCurrentButton.SetTooltipText("Run the statement containing the cursor")
	w.sqlRunCurrentButton.ConnectClicked(func() { w.runActiveSQL(sqlRunCurrent) })
	w.sqlRunAllButton = gtk.NewButtonWithLabel("Run all")
	w.sqlRunAllButton.ConnectClicked(func() { w.runActiveSQL(sqlRunAll) })
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
	profile, found := w.sqlProfileNamed(w.selectedSQLProfile)
	if !found {
		w.setStatus("Select a SQL connection in the Browser Pane first", true)
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

	resultTable := newStringTable(nil)
	resultScroll := gtk.NewScrolledWindow()
	resultScroll.SetHExpand(true)
	resultScroll.SetVExpand(true)
	resultScroll.SetChild(resultTable.view)
	resultLabel := gtk.NewLabel("No query has been run in this tab.")
	resultLabel.SetXAlign(0)
	resultLabel.AddCSSClass("muted")
	resultBox := gtk.NewBox(gtk.OrientationVertical, 6)
	resultBox.SetMarginTop(6)
	resultBox.Append(resultLabel)
	resultBox.Append(resultScroll)

	split := gtk.NewPaned(gtk.OrientationVertical)
	split.AddCSSClass("pane-split")
	split.SetStartChild(editorScroll)
	split.SetEndChild(resultBox)
	split.SetPosition(320)
	split.SetResizeStartChild(true)
	split.SetResizeEndChild(true)
	tab := &sqlWorkbenchTab{state: state, editor: editor, page: split, tabLabel: gtk.NewLabel(""), resultTable: resultTable, resultLabel: resultLabel}
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

func (w *mainWindow) closeSQLTab(tab *sqlWorkbenchTab) {
	if tab == nil || w.sqlActionPending {
		return
	}
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
)

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
	w.sqlActionPending = true
	w.sqlActionGeneration++
	actionGeneration := w.sqlActionGeneration
	w.updateSQLControls()
	ctx, generation := w.startRequest("Running SQL on " + profile.Name + "…")
	go func() {
		results, err := w.sqlExecutor.Execute(ctx, profile, source, tab.state.AllowWrites)
		glib.IdleAdd(func() {
			if actionGeneration != w.sqlActionGeneration {
				return
			}
			w.sqlActionPending = false
			w.updateSQLControls()
			if ctx.Err() != nil || generation != w.generation {
				return
			}
			w.requestPending = false
			w.spinner.Stop()
			w.setWorkspaceBusy("", false)
			if err != nil {
				w.setStatus("SQL query failed: "+err.Error(), true)
				return
			}
			tab.results = results
			w.renderSQLResults(tab)
			rows := 0
			for _, result := range results {
				rows += len(result.Rows)
			}
			w.setStatus(fmt.Sprintf("Executed %d statement(s); %d result row(s)", len(results), rows), false)
		})
	}()
}

func (w *mainWindow) renderSQLResults(tab *sqlWorkbenchTab) {
	if tab == nil || len(tab.results) == 0 {
		return
	}
	result := tab.results[len(tab.results)-1]
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
	status := fmt.Sprintf("%s • %d row(s) • %d ms", valueOrDash(result.CommandTag), len(result.Rows), result.DurationMS)
	if len(tab.results) > 1 {
		status = fmt.Sprintf("Statement %d of %d • %s", len(tab.results), len(tab.results), status)
	}
	if result.Truncated {
		status += fmt.Sprintf(" • truncated at %d rows", sqlworkbench.DefaultMaxRows)
	}
	tab.resultLabel.SetLabel(status)
}

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
	w.sqlActionPending = true
	w.sqlActionGeneration++
	actionGeneration := w.sqlActionGeneration
	w.updateSQLControls()
	ctx, generation := w.startRequest("Reconnecting to " + profile.Name + "…")
	go func() {
		err := w.sqlExecutor.Reconnect(ctx, profile)
		glib.IdleAdd(func() {
			if actionGeneration != w.sqlActionGeneration {
				return
			}
			w.sqlActionPending = false
			w.updateSQLControls()
			if ctx.Err() != nil || generation != w.generation {
				return
			}
			w.requestPending = false
			w.spinner.Stop()
			w.setWorkspaceBusy("", false)
			if err != nil {
				w.setStatus("SQL reconnect failed: "+err.Error(), true)
				return
			}
			w.setStatus("Connected to "+profile.Name, false)
		})
	}()
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
	for _, button := range []*gtk.Button{w.sqlRunSelectionButton, w.sqlRunCurrentButton, w.sqlRunAllButton, w.sqlReconnectButton} {
		if button != nil {
			button.SetSensitive(enabled)
		}
	}
	if w.sqlExportButton != nil {
		w.sqlExportButton.SetSensitive(enabled && len(tab.results) > 0)
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
}

func (w *mainWindow) exportActiveSQLResult() {
	tab := w.activeSQLTab()
	if tab == nil || len(tab.results) == 0 {
		return
	}
	result := tab.results[len(tab.results)-1]
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
	connectTimeout := gtk.NewSpinButtonWithRange(0, 300, 1)
	connectTimeout.SetValue(15)
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
	content.Append(settingsRow("Connect timeout (seconds; 0 uses driver default)", connectTimeout))
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
		connectTimeout.SetValue(float64(profile.ConnectSecs))
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
	selector.NotifyProperty("selected", loadProfile)
	loadProfile()

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
			SSLMode: settingsChoiceValue(sslModes, sslMode.Selected()), ConnectSecs: connectTimeout.ValueAsInt(),
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
