//go:build gui

package gui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

func (o Options) ConfigDynamoTables() []config.DynamoTable {
	if o.Config == nil {
		return nil
	}
	return o.Config.DynamoTables
}

func (o Options) ConfigDynamoQueries() []config.DynamoQuery {
	if o.Config == nil {
		return nil
	}
	return o.Config.DynamoQueries
}

func (w *mainWindow) openDynamoDBModule() {
	if w.currentPage == pageDynamoTables && w.activeSavedDynamoTable == "" && w.activeSavedDynamoQuery == "" {
		return
	}
	w.loadDynamoTables("", "")
}

func (w *mainWindow) loadDynamoTables(filter, savedName string) {
	w.resetWorkspaceForBrowserChange()
	w.clearDynamoTables()
	w.clearDynamoItems()
	w.currentPage = pageDynamoTables
	w.activeSavedDynamoTable = savedName
	w.activeSavedDynamoQuery = ""
	w.search.SetPlaceholderText("Filter tables…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageDynamoTables)
	w.backButton.SetSensitive(false)
	w.setBreadcrumb(dynamoBreadcrumb(savedName, ""))
	w.setDetail("Loading DynamoDB tables…", detailIntro)
	w.updateActionSensitivity()
	if w.options.DynamoDB == nil {
		w.setDetail("DynamoDB is unavailable because no DynamoDB service was configured.", detailError)
		w.setStatus("DynamoDB service unavailable", true)
		return
	}
	ctx, generation := w.startRequest("Loading DynamoDB tables…")
	go func() {
		tables, err := w.options.DynamoDB.Tables(ctx, filter)
		w.finishRequest(ctx, generation, err, func() {
			w.allDynamoTables = tables
			w.applyDynamoTableFilter()
			w.setDetail(dynamoTableListSummary(filter, len(tables)), detailIntro)
		})
	}()
}

func (w *mainWindow) refreshDynamoTables(foreground bool) {
	if w.options.DynamoDB == nil {
		return
	}
	filter := ""
	if saved, found := findDynamoTable(w.options.ConfigDynamoTables(), w.activeSavedDynamoTable); found {
		filter = saved.Table
	}
	selected := w.selectedDynamoTable
	ctx, generation := w.startRefreshRequest("Refreshing DynamoDB tables…", foreground)
	go func() {
		tables, err := w.options.DynamoDB.Tables(ctx, filter)
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allDynamoTables = tables
			w.applyDynamoTableFilter()
			if selected == "" {
				return
			}
			if !containsString(tables, selected) {
				w.selectedDynamoTable = ""
				w.dynamoTableDetail = nil
				w.setDetail("The selected DynamoDB table is no longer available.", detailIntro)
				w.updateActionSensitivity()
			}
		})
	}()
}

func (w *mainWindow) clearDynamoTables() {
	w.allDynamoTables = nil
	w.filteredDynamoTables = nil
	w.selectedDynamoTable = ""
	w.dynamoTableDetail = nil
	if w.dynamoTable != nil {
		w.dynamoTable.clear()
	}
}

func (w *mainWindow) applyDynamoTableFilter() {
	w.filteredDynamoTables = filterDynamoTables(w.allDynamoTables, w.search.Text())
	w.dynamoTable.replace(w.filteredDynamoTables)
}

func filterDynamoTables(tables []string, query string) []string {
	query = strings.ToLower(strings.TrimSpace(query))
	result := make([]string, 0, len(tables))
	for _, table := range tables {
		if query == "" || strings.Contains(strings.ToLower(table), query) {
			result = append(result, table)
		}
	}
	return result
}

func (w *mainWindow) selectDynamoTableRow() {
	if w.currentPage != pageDynamoTables {
		return
	}
	position := w.dynamoTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredDynamoTables) {
		if w.selectedDynamoTable != "" {
			w.selectedDynamoTable = ""
			w.dynamoTableDetail = nil
			w.setBreadcrumb(dynamoBreadcrumb(w.activeSavedDynamoTable, ""))
			w.setDetail(dynamoTableListSummary("", len(w.allDynamoTables)), detailIntro)
			w.updateActionSensitivity()
		}
		return
	}
	table := w.filteredDynamoTables[position]
	w.selectedDynamoTable = table
	w.dynamoTableDetail = nil
	w.setBreadcrumb(dynamoBreadcrumb(w.activeSavedDynamoTable, table))
	w.setDetail("Loading DynamoDB table configuration…", detailIntro)
	w.updateActionSensitivity()
	w.loadDynamoTableDetail(table)
}

func (w *mainWindow) loadDynamoTableDetail(table string) {
	if w.options.DynamoDB == nil || table == "" {
		return
	}
	ctx, generation := w.startRequest("Loading DynamoDB table configuration…")
	go func() {
		detail, err := w.options.DynamoDB.Table(ctx, table)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageDynamoTables || w.selectedDynamoTable != table {
				return
			}
			w.dynamoTableDetail = detail
			w.setDetail(formatDynamoTable(detail), detailDynamoTable)
		})
	}()
}

func (w *mainWindow) openDynamoTableAt(position uint) {
	if int(position) >= len(w.filteredDynamoTables) {
		return
	}
	w.loadDynamoItems(w.filteredDynamoTables[position], "", "")
}

func (w *mainWindow) loadDynamoItems(table, savedTable, queryName string) {
	w.resetWorkspaceForBrowserChange()
	w.clearDynamoItems()
	w.currentPage = pageDynamoItems
	w.selectedDynamoTable = table
	w.activeSavedDynamoTable = savedTable
	w.activeSavedDynamoQuery = queryName
	w.dynamoFilter = nil
	w.dynamoPartiQL = ""
	w.search.SetPlaceholderText("Filter loaded items…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageDynamoItems)
	w.backButton.SetSensitive(true)
	w.setBreadcrumb(dynamoItemBreadcrumb(savedTable, queryName, table))
	w.setDetail("Loading DynamoDB items…", detailIntro)
	w.updateActionSensitivity()
	if w.options.DynamoDB == nil {
		w.setDetail("DynamoDB is unavailable because no DynamoDB service was configured.", detailError)
		return
	}
	ctx, generation := w.startRequest("Loading DynamoDB items…")
	go func() {
		detail, err := w.options.DynamoDB.Table(ctx, table)
		if err != nil {
			w.finishRequest(ctx, generation, err, nil)
			return
		}
		page, err := w.options.DynamoDB.Scan(ctx, model.DynamoScanRequest{Table: table, Limit: 50})
		w.finishRequest(ctx, generation, err, func() {
			w.dynamoTableDetail = detail
			w.dynamoKeyNames = dynamoKeyNames(detail)
			w.allDynamoItems = page.Items
			w.dynamoNextToken = page.NextToken
			w.applyDynamoItemFilter()
			w.setDetail(dynamoItemListSummary(table, len(page.Items), page.ScannedCount, page.NextToken != ""), detailIntro)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) clearDynamoItems() {
	w.allDynamoItems = nil
	w.filteredDynamoItems = nil
	w.selectedDynamoItem = -1
	w.dynamoKeyNames = nil
	w.dynamoNextToken = ""
	if w.dynamoItemTable != nil {
		w.dynamoItemTable.clear()
	}
}

func (w *mainWindow) refreshDynamoItems(foreground bool) {
	if w.options.DynamoDB == nil {
		return
	}
	table, statement := w.selectedDynamoTable, w.dynamoPartiQL
	filter := w.dynamoFilter
	ctx, generation := w.startRefreshRequest("Refreshing DynamoDB items…", foreground)
	go func() {
		if statement != "" {
			items, err := w.options.DynamoDB.PartiQL(ctx, statement)
			w.finishRefreshRequest(ctx, generation, err, foreground, func() {
				w.allDynamoItems = items
				w.dynamoNextToken = ""
				w.selectedDynamoItem = -1
				w.applyDynamoItemFilter()
				w.setDetail(fmt.Sprintf("DYNAMODB PARTIQL RESULTS\n\nRows       %d\nStatement  %s", len(items), statement), detailIntro)
				w.updateActionSensitivity()
			})
			return
		}
		page, err := w.options.DynamoDB.Scan(ctx, model.DynamoScanRequest{Table: table, Limit: 50, Filter: filter})
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allDynamoItems = page.Items
			w.dynamoNextToken = page.NextToken
			w.selectedDynamoItem = -1
			w.applyDynamoItemFilter()
			w.setDetail(dynamoItemListSummary(table, len(page.Items), page.ScannedCount, page.NextToken != ""), detailIntro)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) applyDynamoItemFilter() {
	query := strings.ToLower(strings.TrimSpace(w.search.Text()))
	w.filteredDynamoItems = w.filteredDynamoItems[:0]
	rows := make([]string, 0, len(w.allDynamoItems))
	for _, item := range w.allDynamoItems {
		row := compactDynamoItem(item)
		if query == "" || strings.Contains(strings.ToLower(row), query) {
			w.filteredDynamoItems = append(w.filteredDynamoItems, item)
			rows = append(rows, row)
		}
	}
	w.dynamoItemTable.replace(rows)
}

func (w *mainWindow) selectDynamoItemRow() {
	if w.currentPage != pageDynamoItems {
		return
	}
	position := w.dynamoItemTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredDynamoItems) {
		w.selectedDynamoItem = -1
		w.setDetail(w.currentDynamoItemSummary(), detailIntro)
		w.updateActionSensitivity()
		return
	}
	w.selectedDynamoItem = int(position)
	w.setDetail(formatDynamoItem(w.filteredDynamoItems[position], w.dynamoKeyNames), detailDynamoItem)
	w.updateActionSensitivity()
}

func (w *mainWindow) openDynamoItemAt(position uint) {
	if int(position) >= len(w.filteredDynamoItems) {
		return
	}
	w.dynamoItemTable.selection.SetSelected(position)
}

func (w *mainWindow) currentDynamoItemSummary() string {
	return dynamoItemListSummary(w.selectedDynamoTable, len(w.allDynamoItems), len(w.allDynamoItems), w.dynamoNextToken != "")
}

func (w *mainWindow) rebuildDynamoRail() {
	if w.dynamoModuleItems == nil {
		return
	}
	if w.savedDynamoTablesLabel != nil {
		w.dynamoModuleItems.Remove(w.savedDynamoTablesLabel)
	}
	if w.savedDynamoQueriesLabel != nil {
		w.dynamoModuleItems.Remove(w.savedDynamoQueriesLabel)
	}
	for _, button := range append(w.savedDynamoTableButtons, w.savedDynamoQueryButtons...) {
		w.dynamoModuleItems.Remove(button)
	}
	w.savedDynamoTableButtons = nil
	w.savedDynamoQueryButtons = nil
	w.savedDynamoTablesLabel = nil
	w.savedDynamoQueriesLabel = nil
	if tables := w.options.ConfigDynamoTables(); len(tables) > 0 {
		w.savedDynamoTablesLabel = newDynamoRailLabel("SAVED TABLES")
		w.dynamoModuleItems.Append(w.savedDynamoTablesLabel)
		for _, saved := range tables {
			saved := saved
			button := newModuleRailButton(saved.Name, func() { w.loadDynamoItems(saved.Table, saved.Name, "") })
			button.SetGroup(w.clustersNavButton)
			button.SetTooltipText(saved.Table)
			w.dynamoModuleItems.Append(button)
			w.savedDynamoTableButtons = append(w.savedDynamoTableButtons, button)
		}
	}
	if queries := w.options.ConfigDynamoQueries(); len(queries) > 0 {
		w.savedDynamoQueriesLabel = newDynamoRailLabel("SAVED QUERIES")
		w.dynamoModuleItems.Append(w.savedDynamoQueriesLabel)
		for _, saved := range queries {
			saved := saved
			button := newModuleRailButton(saved.Name, func() { w.loadDynamoPartiQL(saved.Statement, saved.Name) })
			button.SetGroup(w.clustersNavButton)
			button.SetTooltipText(saved.Statement)
			w.dynamoModuleItems.Append(button)
			w.savedDynamoQueryButtons = append(w.savedDynamoQueryButtons, button)
		}
	}
}

func newDynamoRailLabel(text string) *gtk.Label {
	label := gtk.NewLabel(text)
	label.SetXAlign(0)
	label.AddCSSClass("section-title")
	return label
}

func (w *mainWindow) loadDynamoPartiQL(statement, savedName string) {
	w.resetWorkspaceForBrowserChange()
	w.clearDynamoItems()
	w.currentPage = pageDynamoItems
	w.selectedDynamoTable = ""
	w.activeSavedDynamoTable = ""
	w.activeSavedDynamoQuery = savedName
	w.dynamoPartiQL = statement
	w.search.SetPlaceholderText("Filter query results…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageDynamoItems)
	w.backButton.SetSensitive(true)
	w.setBreadcrumb(dynamoItemBreadcrumb("", savedName, ""))
	w.setDetail("Executing DynamoDB PartiQL query…", detailIntro)
	w.updateActionSensitivity()
	if w.options.DynamoDB == nil {
		w.setDetail("DynamoDB is unavailable because no DynamoDB service was configured.", detailError)
		return
	}
	ctx, generation := w.startRequest("Executing DynamoDB PartiQL query…")
	go func() {
		items, err := w.options.DynamoDB.PartiQL(ctx, statement)
		w.finishRequest(ctx, generation, err, func() {
			w.allDynamoItems = items
			w.applyDynamoItemFilter()
			w.setDetail(fmt.Sprintf("DYNAMODB PARTIQL RESULTS\n\nRows       %d\nStatement  %s", len(items), statement), detailIntro)
		})
	}()
}

func (w *mainWindow) reloadDynamoConfig() {
	if w.options.Config == nil || w.options.ReloadConfig == nil {
		return
	}
	fresh := w.options.ReloadConfig()
	w.options.Config.DynamoTables = append([]config.DynamoTable(nil), fresh.DynamoTables...)
	w.options.Config.DynamoQueries = append([]config.DynamoQuery(nil), fresh.DynamoQueries...)
	w.rebuildDynamoRail()
}

func dynamoBreadcrumb(savedName, table string) string {
	root := "Tables"
	if savedName != "" {
		root = savedName
	}
	if table == "" {
		return "DynamoDB / " + root
	}
	return "DynamoDB / " + root + " / " + table
}

func dynamoItemBreadcrumb(savedTable, savedQuery, table string) string {
	if savedQuery != "" {
		return "DynamoDB / " + savedQuery
	}
	return dynamoBreadcrumb(savedTable, table) + " / Items"
}

func dynamoTableListSummary(filter string, count int) string {
	if count == 0 {
		return "No DynamoDB tables found."
	}
	result := fmt.Sprintf("DYNAMODB TABLES\n\nTables  %d", count)
	if filter != "" {
		result += "\nFilter  " + filter
	}
	return result + "\n\nSelect a table for configuration; double-click to browse its items."
}

func formatDynamoTable(table *model.DynamoTable) string {
	if table == nil {
		return "DynamoDB table configuration is unavailable."
	}
	var out strings.Builder
	fmt.Fprintf(&out, "DYNAMODB TABLE\n\nName          %s\nStatus        %s\nItems         %d\nSize          %s\nBilling mode  %s",
		table.Name, valueOrDash(table.Status), table.ItemCount, formatS3Bytes(table.SizeBytes), valueOrDash(table.BillingMode))
	if len(table.KeySchema) > 0 {
		out.WriteString("\n\nKEY SCHEMA")
		for _, key := range table.KeySchema {
			fmt.Fprintf(&out, "\n%s  %s  %s", key.Name, key.Type, key.AttrType)
		}
	}
	if len(table.GSIs) > 0 {
		indexes := append([]string(nil), table.GSIs...)
		sort.Strings(indexes)
		out.WriteString("\n\nGLOBAL SECONDARY INDEXES\n" + strings.Join(indexes, "\n"))
	}
	return out.String()
}

func dynamoItemListSummary(table string, count, scanned int, more bool) string {
	if table == "" {
		table = "PartiQL result"
	}
	result := fmt.Sprintf("DYNAMODB ITEMS\n\nTable    %s\nLoaded   %d\nScanned  %d", table, count, scanned)
	if more {
		result += "\n\nMore items are available."
	}
	return result
}

func compactDynamoItem(item model.DynamoItem) string {
	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Sprintf("%v", item)
	}
	return string(data)
}

func formatDynamoItem(item model.DynamoItem, keyNames []string) string {
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Sprintf("DYNAMODB ITEM\n\n%v", item)
	}
	keys := make([]string, 0, len(keyNames))
	for _, name := range keyNames {
		if value, found := item[name]; found {
			keys = append(keys, fmt.Sprintf("%s = %v", name, value))
		}
	}
	heading := "DYNAMODB ITEM"
	if len(keys) > 0 {
		heading += "\n\nKEY\n" + strings.Join(keys, "\n")
	}
	return heading + "\n\nDOCUMENT\n" + string(data)
}

func dynamoKeyNames(table *model.DynamoTable) []string {
	if table == nil {
		return nil
	}
	result := make([]string, 0, len(table.KeySchema))
	for _, key := range table.KeySchema {
		result = append(result, key.Name)
	}
	return result
}

func findDynamoTable(tables []config.DynamoTable, name string) (config.DynamoTable, bool) {
	for _, table := range tables {
		if table.Name == name {
			return table, true
		}
	}
	return config.DynamoTable{}, false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
