//go:build gui

package gui

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/sqlworkbench"
)

func newSQLObjectExplorer(saved sqlworkbench.ExplorerState) sqlObjectExplorer {
	expanded := make(map[string]bool, len(saved.Expanded))
	for _, key := range saved.Expanded {
		expanded[key] = true
	}
	return sqlObjectExplorer{
		objects: make(map[string][]sqlworkbench.DatabaseObject), loaded: make(map[string]bool),
		loading: make(map[string]bool), expanded: expanded,
	}
}

func sqlSchemaKey(schema string) string { return "schema:" + schema }

func sqlCategoryKey(schema string, kind sqlworkbench.ObjectKind) string {
	return "category:" + schema + ":" + string(kind)
}

func sqlObjectKey(object sqlworkbench.DatabaseObject) string {
	return "object:" + object.OID + ":" + string(object.Kind)
}

func (w *mainWindow) showSQLObjectBrowser(tab *sqlWorkbenchTab) {
	if tab == nil || w.sqlObjectTable == nil {
		return
	}
	if w.sqlBrowserTab != nil && w.sqlBrowserTab != tab && w.sqlObjectScroll != nil {
		w.sqlBrowserTab.state.Explorer.Scroll = w.sqlObjectScroll.VAdjustment().Value()
	}
	w.sqlBrowserTab = tab
	tab.explorer.restoreScroll = true
	w.currentPage = pageSQLObjects
	w.backButton.SetSensitive(true)
	w.search.SetSensitive(true)
	w.search.SetPlaceholderText("Filter loaded database objects…")
	w.search.SetText(tab.state.Explorer.Filter)
	w.resourceStack.SetVisibleChildName(pageSQLObjects)
	w.detailStack.SetVisibleChildName("sql-workbench")
	w.setBreadcrumb("SQL Workbench / " + tab.state.ProfileName + " / Database objects")
	w.revealModuleForPage(w.currentPage)
	w.updateActionSensitivity()
	if len(tab.explorer.schemas) == 0 && !tab.explorer.loadingSchemas {
		w.loadSQLSchemas(tab)
	} else {
		w.renderSQLObjectBrowser(tab)
	}
}

func (w *mainWindow) loadSQLSchemas(tab *sqlWorkbenchTab) {
	if tab == nil || tab.explorer.loadingSchemas || w.sqlExecutor == nil {
		return
	}
	profile, found := w.sqlProfileNamed(tab.state.ProfileName)
	if !found {
		w.setStatus("SQL connection profile "+tab.state.ProfileName+" is no longer configured", true)
		return
	}
	tab.explorer.loadingSchemas = true
	generation := tab.explorer.requestGeneration
	w.renderSQLObjectBrowser(tab)
	ctx, cancel := context.WithTimeout(w.sqlCatalogContext(tab), 45*time.Second)
	go func() {
		defer cancel()
		schemas, err := w.sqlExecutor.ListSchemas(ctx, profile)
		glib.IdleAdd(func() {
			if generation != tab.explorer.requestGeneration {
				return
			}
			tab.explorer.loadingSchemas = false
			if err != nil {
				w.renderSQLObjectBrowser(tab)
				w.setStatus("Load PostgreSQL schemas: "+err.Error(), true)
				return
			}
			tab.explorer.schemas = schemas
			w.renderSQLObjectBrowser(tab)
			for _, schema := range schemas {
				for _, kind := range sqlworkbench.CatalogKinds() {
					key := sqlCategoryKey(schema, kind)
					if tab.explorer.expanded[key] {
						w.loadSQLObjects(tab, schema, kind)
					}
				}
			}
			w.setStatus(fmt.Sprintf("Loaded %d PostgreSQL schema(s) for %s", len(schemas), profile.Name), false)
		})
	}()
}

func (w *mainWindow) loadSQLObjects(tab *sqlWorkbenchTab, schema string, kind sqlworkbench.ObjectKind) {
	key := sqlCategoryKey(schema, kind)
	if tab == nil || tab.explorer.loaded[key] || tab.explorer.loading[key] || w.sqlExecutor == nil {
		return
	}
	profile, found := w.sqlProfileNamed(tab.state.ProfileName)
	if !found {
		return
	}
	tab.explorer.loading[key] = true
	generation := tab.explorer.requestGeneration
	w.renderSQLObjectBrowser(tab)
	ctx, cancel := context.WithTimeout(w.sqlCatalogContext(tab), 45*time.Second)
	go func() {
		defer cancel()
		objects, err := w.sqlExecutor.ListObjects(ctx, profile, schema, kind)
		glib.IdleAdd(func() {
			if generation != tab.explorer.requestGeneration {
				return
			}
			delete(tab.explorer.loading, key)
			if err != nil {
				w.renderSQLObjectBrowser(tab)
				w.setStatus("Load "+sqlworkbench.ObjectKindLabel(kind)+" in "+schema+": "+err.Error(), true)
				return
			}
			tab.explorer.objects[key] = objects
			tab.explorer.loaded[key] = true
			w.renderSQLObjectBrowser(tab)
		})
	}()
}

func (w *mainWindow) renderSQLObjectBrowser(tab *sqlWorkbenchTab) {
	if tab == nil || w.sqlObjectTable == nil || w.activeSQLTab() != tab || w.currentPage != pageSQLObjects {
		return
	}
	rows := make([]sqlObjectBrowserRow, 0)
	if tab.explorer.loadingSchemas && len(tab.explorer.schemas) == 0 {
		rows = append(rows, sqlObjectBrowserRow{kind: sqlObjectRowMessage, key: "loading", label: "Loading schemas…"})
	} else if len(tab.explorer.schemas) == 0 {
		rows = append(rows, sqlObjectBrowserRow{kind: sqlObjectRowMessage, key: "empty", label: "No user schemas found"})
	}
	for _, schema := range tab.explorer.schemas {
		schemaKey := sqlSchemaKey(schema)
		rows = append(rows, sqlObjectBrowserRow{kind: sqlObjectRowSchema, key: schemaKey, schema: schema,
			label: sqlTreeLabel(0, tab.explorer.expanded[schemaKey], schema)})
		if !tab.explorer.expanded[schemaKey] {
			continue
		}
		for _, kind := range sqlworkbench.CatalogKinds() {
			key := sqlCategoryKey(schema, kind)
			label := sqlworkbench.ObjectKindLabel(kind)
			if tab.explorer.loaded[key] {
				label += fmt.Sprintf(" (%d)", len(tab.explorer.objects[key]))
			}
			rows = append(rows, sqlObjectBrowserRow{kind: sqlObjectRowCategory, key: key, schema: schema, category: kind,
				label: sqlTreeLabel(1, tab.explorer.expanded[key], label)})
			if !tab.explorer.expanded[key] {
				continue
			}
			if tab.explorer.loading[key] {
				rows = append(rows, sqlObjectBrowserRow{kind: sqlObjectRowMessage, key: key + ":loading", schema: schema,
					label: strings.Repeat("    ", 2) + "Loading…"})
				continue
			}
			for _, object := range tab.explorer.objects[key] {
				rows = append(rows, sqlObjectBrowserRow{kind: sqlObjectRowObject, key: sqlObjectKey(object), schema: schema,
					category: kind, object: object, label: strings.Repeat("    ", 2) + object.DisplayName()})
			}
		}
	}
	rows = filterSQLObjectRows(rows, strings.TrimSpace(w.search.Text()))
	tab.explorer.rows = rows
	presentation := make([]string, len(rows))
	selected := gtk.InvalidListPosition
	for index, row := range rows {
		presentation[index] = row.label + "\t" + row.key
		if row.key == tab.state.Explorer.Selected {
			selected = index
		}
	}
	w.sqlObjectTable.replace(presentation)
	if selected != gtk.InvalidListPosition {
		w.sqlObjectTable.selection.SetSelected(uint(selected))
	}
	if tab.explorer.restoreScroll && !tab.explorer.loadingSchemas && len(tab.explorer.loading) == 0 && w.sqlObjectScroll != nil {
		tab.explorer.restoreScroll = false
		glib.IdleAdd(func() { w.sqlObjectScroll.VAdjustment().SetValue(tab.state.Explorer.Scroll) })
	}
}

func sqlTreeLabel(depth int, expanded bool, label string) string {
	chevron := "▸"
	if expanded {
		chevron = "▾"
	}
	return strings.Repeat("    ", depth) + chevron + " " + label
}

func filterSQLObjectRows(rows []sqlObjectBrowserRow, term string) []sqlObjectBrowserRow {
	term = strings.ToLower(term)
	if term == "" {
		return rows
	}
	keep := make(map[string]bool)
	for _, row := range rows {
		if strings.Contains(strings.ToLower(row.label+" "+row.schema+" "+string(row.category)), term) {
			keep[row.key] = true
			if row.schema != "" {
				keep[sqlSchemaKey(row.schema)] = true
			}
			if row.category != "" {
				keep[sqlCategoryKey(row.schema, row.category)] = true
			}
		}
	}
	filtered := make([]sqlObjectBrowserRow, 0, len(rows))
	for _, row := range rows {
		if keep[row.key] {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func (w *mainWindow) applySQLObjectFilter() {
	if tab := w.activeSQLTab(); tab != nil {
		tab.state.Explorer.Filter = w.search.Text()
		w.renderSQLObjectBrowser(tab)
		w.scheduleSQLStateSave()
	}
}

func (w *mainWindow) activateSQLObjectAt(position uint) {
	tab := w.activeSQLTab()
	if tab == nil || int(position) >= len(tab.explorer.rows) {
		return
	}
	row := tab.explorer.rows[position]
	tab.state.Explorer.Selected = row.key
	switch row.kind {
	case sqlObjectRowSchema:
		tab.explorer.expanded[row.key] = !tab.explorer.expanded[row.key]
	case sqlObjectRowCategory:
		tab.explorer.expanded[row.key] = !tab.explorer.expanded[row.key]
		if tab.explorer.expanded[row.key] {
			w.loadSQLObjects(tab, row.schema, row.category)
		}
	case sqlObjectRowObject:
		// GtkColumnView may move its selection while rows are virtualized during
		// scrolling. Activation is the deliberate single click/keyboard action.
		w.loadSQLObjectDetail(tab, row.object)
	}
	w.syncSQLExplorerState(tab)
	w.renderSQLObjectBrowser(tab)
}

func (w *mainWindow) loadSQLObjectDetail(tab *sqlWorkbenchTab, object sqlworkbench.DatabaseObject) {
	if tab == nil || w.sqlExecutor == nil {
		return
	}
	profile, found := w.sqlProfileNamed(tab.state.ProfileName)
	if !found {
		return
	}
	selected := object
	if tab.explorer.detailCancel != nil {
		tab.explorer.detailCancel()
	}
	tab.explorer.selected = &selected
	tab.explorer.detailGeneration++
	tab.explorer.detailPending = true
	generation := tab.explorer.detailGeneration
	tab.objectToolbar.SetVisible(true)
	w.updateSQLObjectControls(tab)
	tab.structureBuffer.SetText("Loading structure for " + object.QualifiedName() + "…")
	tab.definitionBuffer.SetText("Loading definition for " + object.QualifiedName() + "…")
	tab.contextNotebook.SetCurrentPage(1)
	ctx, cancel := context.WithTimeout(w.ctx, 45*time.Second)
	tab.explorer.detailCancel = cancel
	go func() {
		defer cancel()
		detail, err := w.sqlExecutor.InspectObject(ctx, profile, object)
		glib.IdleAdd(func() {
			if generation != tab.explorer.detailGeneration {
				return
			}
			tab.explorer.detailCancel = nil
			tab.explorer.detailPending = false
			w.updateSQLObjectControls(tab)
			if err != nil {
				tab.structureBuffer.SetText("Unable to load object metadata:\n\n" + err.Error())
				tab.definitionBuffer.SetText("Unable to load object definition:\n\n" + err.Error())
				w.setStatus("Inspect "+object.QualifiedName()+": "+err.Error(), true)
				return
			}
			tab.structureBuffer.SetText(sqlworkbench.ObjectStructure(detail))
			tab.definitionBuffer.SetText(detail.Definition)
			w.setStatus("Loaded metadata for "+object.QualifiedName(), false)
		})
	}()
}

func (w *mainWindow) refreshSQLObjectDetail(tab *sqlWorkbenchTab) {
	if tab == nil || tab.explorer.selected == nil || tab.explorer.detailPending {
		return
	}
	w.loadSQLObjectDetail(tab, *tab.explorer.selected)
}

func (w *mainWindow) previewSQLObject(tab *sqlWorkbenchTab) {
	if tab == nil || tab.explorer.selected == nil || w.sqlActionPending || w.sqlExecutor == nil {
		return
	}
	profile, found := w.sqlProfileNamed(tab.state.ProfileName)
	if !found {
		return
	}
	object := *tab.explorer.selected
	w.sqlActionPending = true
	w.sqlActionGeneration++
	actionGeneration := w.sqlActionGeneration
	w.updateSQLControls()
	ctx, generation := w.startRequest("Previewing " + object.QualifiedName() + "…")
	go func() {
		result, err := w.sqlExecutor.PreviewObject(ctx, profile, object, 100)
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
				w.setStatus("Preview "+object.QualifiedName()+": "+err.Error(), true)
				return
			}
			tab.results = []model.SQLQueryResult{result}
			w.renderSQLResults(tab)
			w.updateSQLControls()
			w.setStatus(fmt.Sprintf("Previewed %d row(s) from %s", len(result.Rows), object.QualifiedName()), false)
		})
	}()
}

func (w *mainWindow) generateSQLSelect(tab *sqlWorkbenchTab) {
	if tab == nil || tab.explorer.selected == nil {
		return
	}
	object := *tab.explorer.selected
	statement := "SELECT *\nFROM " + object.QualifiedName() + "\nLIMIT 100;\n"
	buffer := tab.editor.Buffer()
	offset := buffer.IterAtMark(buffer.GetInsert()).Offset()
	content := []rune(tab.editor.Text())
	if offset < 0 || offset > len(content) {
		offset = len(content)
	}
	if offset > 0 && content[offset-1] != '\n' {
		statement = "\n" + statement
	}
	inserted := []rune(statement)
	content = append(content[:offset], append(inserted, content[offset:]...)...)
	tab.editor.SetText(string(content))
	buffer.PlaceCursor(buffer.IterAtOffset(offset + len(inserted)))
	w.setStatus("Generated a safe preview query for "+object.QualifiedName(), false)
}

func (w *mainWindow) copySQLObjectName(tab *sqlWorkbenchTab) {
	if tab == nil || tab.explorer.selected == nil {
		return
	}
	name := tab.explorer.selected.QualifiedName()
	w.window.Clipboard().SetText(name)
	w.setStatus("Copied "+name, false)
}

func (w *mainWindow) refreshSQLObjectBrowser(foreground bool) {
	tab := w.activeSQLTab()
	if tab == nil {
		return
	}
	tab.explorer.requestGeneration++
	tab.explorer.detailGeneration++
	if tab.explorer.catalogCancel != nil {
		tab.explorer.catalogCancel()
	}
	if tab.explorer.detailCancel != nil {
		tab.explorer.detailCancel()
	}
	tab.explorer.catalogContext = nil
	tab.explorer.catalogCancel = nil
	tab.explorer.detailCancel = nil
	tab.explorer.schemas = nil
	tab.explorer.objects = make(map[string][]sqlworkbench.DatabaseObject)
	tab.explorer.loaded = make(map[string]bool)
	tab.explorer.loading = make(map[string]bool)
	tab.explorer.loadingSchemas = false
	tab.explorer.selected = nil
	tab.explorer.detailPending = false
	tab.objectToolbar.SetVisible(false)
	if foreground {
		w.setStatus("Refreshing PostgreSQL database objects…", false)
	}
	w.loadSQLSchemas(tab)
}

func (w *mainWindow) sqlCatalogContext(tab *sqlWorkbenchTab) context.Context {
	if tab.explorer.catalogContext == nil {
		tab.explorer.catalogContext, tab.explorer.catalogCancel = context.WithCancel(w.ctx)
	}
	return tab.explorer.catalogContext
}

func (w *mainWindow) updateSQLObjectControls(tab *sqlWorkbenchTab) {
	if tab == nil || tab.objectToolbar == nil {
		return
	}
	selected := tab.explorer.selected
	hasObject := selected != nil
	rowObject := hasObject && selected.Kind != sqlworkbench.ObjectFunction && selected.Kind != sqlworkbench.ObjectSequence
	tab.previewButton.SetSensitive(rowObject && !w.sqlActionPending && !tab.explorer.detailPending)
	tab.generateButton.SetSensitive(rowObject && !w.sqlActionPending)
	tab.copyObjectButton.SetSensitive(hasObject)
	tab.definitionButton.SetSensitive(hasObject)
	tab.refreshObjectButton.SetSensitive(hasObject && !w.sqlActionPending && !tab.explorer.detailPending)
}

func (w *mainWindow) syncSQLExplorerState(tab *sqlWorkbenchTab) {
	if tab == nil {
		return
	}
	expanded := make([]string, 0, len(tab.explorer.expanded))
	for key, value := range tab.explorer.expanded {
		if value {
			expanded = append(expanded, key)
		}
	}
	sort.Strings(expanded)
	tab.state.Explorer.Expanded = expanded
	tab.state.UpdatedAt = time.Now()
	w.scheduleSQLStateSave()
}
