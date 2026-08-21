//go:build gui

package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"reflect"
	"sort"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
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
			w.dynamoScannedCount = page.ScannedCount
			w.applyDynamoItemFilter()
			w.setDetail(w.currentDynamoItemSummary(), detailIntro)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) clearDynamoItems() {
	w.allDynamoItems = nil
	w.filteredDynamoItems = nil
	w.selectedDynamoItem = -1
	w.dynamoKeyNames = nil
	w.dynamoItemColumns = nil
	w.dynamoNextToken = ""
	w.dynamoScannedCount = 0
	if w.dynamoItemTable != nil {
		w.dynamoItemTable.clear()
		w.dynamoItemTable.setColumns(nil)
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
				selected, preserveSelection := w.selectedDynamoItemValue()
				w.allDynamoItems = items
				w.dynamoNextToken = ""
				w.selectedDynamoItem = -1
				w.dynamoItemTable.selection.SetSelected(gtk.InvalidListPosition)
				w.applyDynamoItemFilter()
				if !preserveSelection || !w.trySelectDynamoItem(selected) {
					w.setDetail(fmt.Sprintf("DYNAMODB PARTIQL RESULTS\n\nRows       %d\nStatement  %s", len(items), statement), detailIntro)
				}
				w.updateActionSensitivity()
			})
			return
		}
		page, err := w.options.DynamoDB.Scan(ctx, model.DynamoScanRequest{Table: table, Limit: 50, Filter: filter})
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			selected, preserveSelection := w.selectedDynamoItemValue()
			w.allDynamoItems = page.Items
			w.dynamoNextToken = page.NextToken
			w.dynamoScannedCount = page.ScannedCount
			w.selectedDynamoItem = -1
			w.dynamoItemTable.selection.SetSelected(gtk.InvalidListPosition)
			w.applyDynamoItemFilter()
			if !preserveSelection || !w.trySelectDynamoItem(selected) {
				w.setDetail(w.currentDynamoItemSummary(), detailIntro)
			}
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) loadMoreDynamoItems() {
	if w.options.DynamoDB == nil || w.currentPage != pageDynamoItems || w.selectedDynamoTable == "" || w.dynamoNextToken == "" || w.dynamoActionPending {
		return
	}
	w.dynamoActionPending = true
	w.updateActionSensitivity()
	token := w.dynamoNextToken
	request := model.DynamoScanRequest{Table: w.selectedDynamoTable, Limit: 50, NextToken: token, Filter: w.dynamoFilter}
	ctx, generation := w.startRequest("Loading more DynamoDB items…")
	go func() {
		page, err := w.options.DynamoDB.Scan(ctx, request)
		w.finishDynamoAction(ctx, generation, err, "Loaded more DynamoDB items", func() {
			selected, preserveSelection := w.selectedDynamoItemValue()
			w.allDynamoItems = append(w.allDynamoItems, page.Items...)
			w.dynamoNextToken = page.NextToken
			w.dynamoScannedCount += page.ScannedCount
			w.applyDynamoItemFilter()
			if !preserveSelection || !w.trySelectDynamoItem(selected) {
				w.setDetail(w.currentDynamoItemSummary(), detailIntro)
			}
		})
	}()
}

func (w *mainWindow) promptDynamoFilter() {
	if w.currentPage != pageDynamoItems || w.selectedDynamoTable == "" || w.dynamoActionPending {
		return
	}
	dialog := gtk.NewDialogWithFlags("Filter DynamoDB scan", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	attribute := gtk.NewEntry()
	value := gtk.NewEntry()
	operators := []string{"=", "<>", "<", "<=", ">", ">=", "begins_with", "contains"}
	operator := gtk.NewDropDownFromStrings(operators)
	if w.dynamoFilter != nil {
		attribute.SetText(w.dynamoFilter.Attribute)
		value.SetText(w.dynamoFilter.Value)
		for i, candidate := range operators {
			if candidate == w.dynamoFilter.Operator {
				operator.SetSelected(uint(i))
			}
		}
	}
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.AddCSSClass("error")
	for _, row := range []struct {
		label  string
		widget gtk.Widgetter
	}{{"Attribute", attribute}, {"Operator", operator}, {"String value", value}} {
		label := gtk.NewLabel(row.label)
		label.SetXAlign(0)
		content.Append(label)
		content.Append(row.widget)
	}
	content.Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Clear", 101)
	dialog.AddButton("Apply", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response == 101 {
			dialog.Destroy()
			w.runDynamoFilter(nil)
			return
		}
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		name := strings.TrimSpace(attribute.Text())
		if name == "" {
			errorLabel.SetLabel("Enter an attribute name")
			return
		}
		index := int(operator.Selected())
		if index < 0 || index >= len(operators) {
			return
		}
		filter := &model.DynamoFilter{Attribute: name, Operator: operators[index], Value: value.Text()}
		dialog.Destroy()
		w.runDynamoFilter(filter)
	})
	dialog.Present()
}

func (w *mainWindow) runDynamoFilter(filter *model.DynamoFilter) {
	if w.options.DynamoDB == nil || w.selectedDynamoTable == "" || w.dynamoActionPending {
		return
	}
	w.dynamoActionPending = true
	w.updateActionSensitivity()
	table := w.selectedDynamoTable
	ctx, generation := w.startRequest("Filtering DynamoDB items…")
	go func() {
		page, err := w.options.DynamoDB.Scan(ctx, model.DynamoScanRequest{Table: table, Limit: 100, Filter: filter})
		w.finishDynamoAction(ctx, generation, err, "Filtered DynamoDB items", func() {
			w.dynamoFilter = filter
			w.dynamoPartiQL = ""
			w.activeSavedDynamoQuery = ""
			w.allDynamoItems = page.Items
			w.dynamoNextToken = page.NextToken
			w.dynamoScannedCount = page.ScannedCount
			w.selectedDynamoItem = -1
			w.applyDynamoItemFilter()
			w.setDetail(w.currentDynamoItemSummary(), detailIntro)
		})
	}()
}

func (w *mainWindow) promptDynamoPartiQL() {
	if w.currentPage != pageDynamoItems || w.dynamoActionPending {
		return
	}
	initial := w.dynamoPartiQL
	if initial == "" {
		table := w.selectedDynamoTable
		if table == "" {
			table = "table-name"
		}
		initial = fmt.Sprintf("SELECT * FROM \"%s\" WHERE ", table)
	}
	dialog, entry := w.newSavedLogNameDialog("DynamoDB PartiQL", "Statement", initial)
	entry.SetHExpand(true)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Execute", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		statement := strings.TrimSpace(entry.Text())
		dialog.Destroy()
		if response == int(gtk.ResponseOK) && statement != "" {
			w.loadDynamoPartiQL(statement, "")
		}
	})
	dialog.Present()
}

func (w *mainWindow) finishDynamoAction(ctx context.Context, generation uint64, err error, success string, apply func()) {
	glib.IdleAdd(func() {
		if ctx.Err() != nil || generation != w.generation {
			return
		}
		w.requestCancel = nil
		w.spinner.Stop()
		w.setWorkspaceBusy("", false)
		w.dynamoActionPending = false
		w.updateActionSensitivity()
		if err != nil {
			w.setStatus(err.Error(), true)
			return
		}
		apply()
		w.updateActionSensitivity()
		w.setStatus(success, false)
	})
}

func (w *mainWindow) applyDynamoItemFilter() {
	query := strings.ToLower(strings.TrimSpace(w.search.Text()))
	w.filteredDynamoItems = w.filteredDynamoItems[:0]
	rows := make([]string, 0, len(w.allDynamoItems))
	for _, item := range w.allDynamoItems {
		row := compactDynamoItem(item)
		if query == "" || strings.Contains(strings.ToLower(row), query) {
			w.filteredDynamoItems = append(w.filteredDynamoItems, item)
		}
	}
	w.dynamoItemColumns = discoverDynamoItemColumns(w.allDynamoItems, w.dynamoKeyNames)
	specs := make([]columnSpec, len(w.dynamoItemColumns))
	for index, name := range w.dynamoItemColumns {
		specs[index] = columnSpec{title: name, field: index, expand: true}
	}
	w.dynamoItemTable.setColumns(specs)
	for _, item := range w.filteredDynamoItems {
		cells := make([]string, len(w.dynamoItemColumns))
		for index, name := range w.dynamoItemColumns {
			cells[index] = formatDynamoBrowserCell(item[name])
		}
		rows = append(rows, strings.Join(cells, "\t"))
	}
	w.dynamoItemTable.replace(rows)
}

func discoverDynamoItemColumns(items []model.DynamoItem, keyNames []string) []string {
	seen := make(map[string]bool)
	remaining := make([]string, 0)
	for _, item := range items {
		for name := range item {
			if !seen[name] {
				seen[name] = true
				remaining = append(remaining, name)
			}
		}
	}
	sort.Strings(remaining)
	columns := make([]string, 0, len(remaining))
	keys := make(map[string]bool, len(keyNames))
	for _, name := range keyNames {
		if seen[name] {
			columns = append(columns, name)
			keys[name] = true
		}
	}
	for _, name := range remaining {
		if !keys[name] {
			columns = append(columns, name)
		}
	}
	return columns
}

func formatDynamoBrowserCell(value any) string {
	if value == nil {
		return ""
	}
	var text string
	switch typed := value.(type) {
	case string:
		text = strings.ReplaceAll(typed, "\r", "")
		text = strings.ReplaceAll(text, "\n", "\\n")
		text = strings.ReplaceAll(text, "\t", " ")
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			text = fmt.Sprintf("%v", typed)
		} else {
			text = string(data)
		}
	}
	if len(text) > 50 {
		return "[select for detail]"
	}
	return text
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

func (w *mainWindow) selectedDynamoItemValue() (model.DynamoItem, bool) {
	if w.selectedDynamoItem < 0 || w.selectedDynamoItem >= len(w.filteredDynamoItems) {
		return nil, false
	}
	return w.filteredDynamoItems[w.selectedDynamoItem], true
}

func (w *mainWindow) promptDynamoFieldEdit() {
	item, found := w.selectedDynamoItemValue()
	if !found || w.options.DynamoDB == nil || w.dynamoActionPending || w.selectedDynamoTable == "" {
		return
	}
	attributes := editableDynamoAttributes(item, w.dynamoKeyNames)
	if len(attributes) == 0 {
		w.setStatus("This DynamoDB item has no editable non-key attributes", true)
		return
	}
	dialog := gtk.NewDialogWithFlags("Edit DynamoDB field", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(680, 440)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	label := gtk.NewLabel("Table " + w.selectedDynamoTable + " • key " + dynamoKeyDescription(item, w.dynamoKeyNames))
	label.SetXAlign(0)
	label.SetWrap(true)
	content.Append(label)
	selector := gtk.NewDropDownFromStrings(attributes)
	content.Append(selector)
	editor := newSourceEditor(sourceDocument{Path: "attribute.json"})
	editor.ApplyPalette(w.currentSemanticPalette(w.window.StyleContext()))
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetChild(editor.Widget())
	content.Append(scroll)
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.SetWrap(true)
	errorLabel.AddCSSClass("error")
	content.Append(errorLabel)
	loadAttribute := func() {
		index := int(selector.Selected())
		if index < 0 || index >= len(attributes) {
			return
		}
		value := item[attributes[index]]
		language := ""
		if dynamoValueUsesJSON(value) {
			language = "json"
		}
		editor.SetDocument(sourceDocument{Path: "attribute.json", Language: language})
		editor.SetText(service.DynamoValueToEditableString(value))
		errorLabel.SetLabel("")
	}
	selector.NotifyProperty("selected", loadAttribute)
	loadAttribute()
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Review update…", int(gtk.ResponseOK))
	table := w.selectedDynamoTable
	keyNames := append([]string(nil), w.dynamoKeyNames...)
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		index := int(selector.Selected())
		if index < 0 || index >= len(attributes) {
			return
		}
		attribute := attributes[index]
		value := editor.Text()
		if value == service.DynamoValueToEditableString(item[attribute]) {
			errorLabel.SetLabel("The field value has not changed")
			return
		}
		dialog.Destroy()
		w.confirmDynamoFieldEdit(table, keyNames, item, attribute, value)
	})
	dialog.Present()
}

func (w *mainWindow) confirmDynamoFieldEdit(table string, keyNames []string, item model.DynamoItem, attribute, value string) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Update DynamoDB item")
	dialog.SetMarkup("Update <b>" + html.EscapeString(attribute) + "</b> on the selected item in <b>" + html.EscapeString(table) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "Key: "+dynamoKeyDescription(item, keyNames)+". Key attributes cannot be edited.")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.runDynamoFieldEdit(table, keyNames, item, attribute, value)
		}
	})
	dialog.Present()
}

func (w *mainWindow) runDynamoFieldEdit(table string, keyNames []string, item model.DynamoItem, attribute, value string) {
	if w.options.DynamoDB == nil || w.dynamoActionPending {
		return
	}
	w.dynamoActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Updating DynamoDB field " + attribute + "…")
	go func() {
		err := w.options.DynamoDB.UpdateField(ctx, model.DynamoFieldUpdate{
			Table: table, KeyNames: keyNames, Item: item, Attribute: attribute,
			OriginalValue: item[attribute], NewValue: value,
		})
		var refreshed *model.DynamoItem
		if err == nil {
			refreshed, err = w.options.DynamoDB.Item(ctx, table, keyNames, item)
		}
		w.finishDynamoAction(ctx, generation, err, "Updated DynamoDB field "+attribute, func() {
			w.replaceDynamoItem(item, *refreshed)
		})
	}()
}

func (w *mainWindow) promptDynamoClone() {
	item, found := w.selectedDynamoItemValue()
	if !found || w.options.DynamoDB == nil || w.dynamoActionPending || w.selectedDynamoTable == "" {
		return
	}
	dialog := gtk.NewDialogWithFlags("Clone DynamoDB item", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(720, 500)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	label := gtk.NewLabel("Edit the cloned item. At least one key attribute must change.")
	label.SetXAlign(0)
	label.SetWrap(true)
	content.Append(label)
	editor := newSourceEditor(sourceDocument{Path: "dynamodb-item.json", Language: "json"})
	editor.ApplyPalette(w.currentSemanticPalette(w.window.StyleContext()))
	editor.SetText(service.DynamoItemToJSON(item))
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetChild(editor.Widget())
	content.Append(scroll)
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.SetWrap(true)
	errorLabel.AddCSSClass("error")
	content.Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Review clone…", int(gtk.ResponseOK))
	table := w.selectedDynamoTable
	keyNames := append([]string(nil), w.dynamoKeyNames...)
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		clone, err := service.ParseDynamoItemJSON(editor.Text())
		if err != nil {
			errorLabel.SetLabel(err.Error())
			return
		}
		if _, err := service.BuildDynamoKey(clone, keyNames); err != nil {
			errorLabel.SetLabel(err.Error())
			return
		}
		if dynamoItemsShareKey(item, clone, keyNames) {
			errorLabel.SetLabel("Change at least one key attribute before creating the clone")
			return
		}
		dialog.Destroy()
		w.confirmDynamoClone(table, keyNames, clone)
	})
	dialog.Present()
}

func (w *mainWindow) confirmDynamoClone(table string, keyNames []string, item model.DynamoItem) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Create DynamoDB item")
	dialog.SetMarkup("Create a new item in <b>" + html.EscapeString(table) + "</b>?")
	dialog.SetObjectProperty("secondary-text", "Key: "+dynamoKeyDescription(item, keyNames)+". The write is conditional and will fail rather than replace an existing item.")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.runDynamoClone(table, keyNames, item)
		}
	})
	dialog.Present()
}

func (w *mainWindow) runDynamoClone(table string, keyNames []string, item model.DynamoItem) {
	if w.options.DynamoDB == nil || w.dynamoActionPending {
		return
	}
	w.dynamoActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Creating cloned DynamoDB item…")
	go func() {
		err := w.options.DynamoDB.PutItem(ctx, model.DynamoPutRequest{Table: table, KeyNames: keyNames, Item: item})
		w.finishDynamoAction(ctx, generation, err, "Created cloned DynamoDB item", func() {
			w.allDynamoItems = append(w.allDynamoItems, item)
			w.applyDynamoItemFilter()
			w.selectDynamoItemByKey(item)
		})
	}()
}

func (w *mainWindow) replaceDynamoItem(original, replacement model.DynamoItem) {
	for index, candidate := range w.allDynamoItems {
		if dynamoItemsShareKey(candidate, original, w.dynamoKeyNames) {
			w.allDynamoItems[index] = replacement
			break
		}
	}
	w.applyDynamoItemFilter()
	w.selectDynamoItemByKey(replacement)
}

func (w *mainWindow) selectDynamoItemByKey(item model.DynamoItem) {
	if w.trySelectDynamoItem(item) {
		return
	}
	w.selectedDynamoItem = -1
	w.dynamoItemTable.selection.SetSelected(gtk.InvalidListPosition)
	w.setDetail(w.currentDynamoItemSummary(), detailIntro)
}

func (w *mainWindow) trySelectDynamoItem(item model.DynamoItem) bool {
	for index, candidate := range w.filteredDynamoItems {
		if dynamoItemsSameIdentity(candidate, item, w.dynamoKeyNames) {
			w.selectedDynamoItem = index
			w.dynamoItemTable.selection.SetSelected(uint(index))
			w.setDetail(formatDynamoItem(candidate, w.dynamoKeyNames), detailDynamoItem)
			return true
		}
	}
	return false
}

func editableDynamoAttributes(item model.DynamoItem, keyNames []string) []string {
	keys := make(map[string]struct{}, len(keyNames))
	for _, name := range keyNames {
		keys[name] = struct{}{}
	}
	attributes := make([]string, 0, len(item))
	for name := range item {
		if _, key := keys[name]; !key {
			attributes = append(attributes, name)
		}
	}
	sort.Strings(attributes)
	return attributes
}

func dynamoItemsShareKey(left, right model.DynamoItem, keyNames []string) bool {
	same, err := service.DynamoItemsShareKey(left, right, keyNames)
	return err == nil && same
}

func dynamoItemsSameIdentity(left, right model.DynamoItem, keyNames []string) bool {
	if len(keyNames) > 0 {
		return dynamoItemsShareKey(left, right, keyNames)
	}
	return reflect.DeepEqual(left, right)
}

func dynamoKeyDescription(item model.DynamoItem, keyNames []string) string {
	parts := make([]string, 0, len(keyNames))
	for _, name := range keyNames {
		parts = append(parts, fmt.Sprintf("%s=%v", name, item[name]))
	}
	return strings.Join(parts, ", ")
}

func dynamoValueUsesJSON(value any) bool {
	switch value.(type) {
	case string:
		return false
	default:
		return true
	}
}

func (w *mainWindow) currentDynamoItemSummary() string {
	return dynamoItemListSummary(w.selectedDynamoTable, len(w.allDynamoItems), w.dynamoScannedCount, w.dynamoNextToken != "")
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
		w.savedDynamoTablesLabel = newModuleRailSectionLabel("SAVED TABLES")
		w.dynamoModuleItems.Append(w.savedDynamoTablesLabel)
		for _, saved := range tables {
			saved := saved
			button := newSavedModuleRailButton(saved.Name, func() { w.loadDynamoItems(saved.Table, saved.Name, "") })
			button.SetGroup(w.clustersNavButton)
			button.SetTooltipText(saved.Table)
			w.dynamoModuleItems.Append(button)
			w.savedDynamoTableButtons = append(w.savedDynamoTableButtons, button)
		}
	}
	if queries := w.options.ConfigDynamoQueries(); len(queries) > 0 {
		w.savedDynamoQueriesLabel = newModuleRailSectionLabel("SAVED QUERIES")
		w.dynamoModuleItems.Append(w.savedDynamoQueriesLabel)
		for _, saved := range queries {
			saved := saved
			button := newSavedModuleRailButton(saved.Name, func() { w.loadDynamoPartiQL(saved.Statement, saved.Name) })
			button.SetGroup(w.clustersNavButton)
			button.SetTooltipText(saved.Statement)
			w.dynamoModuleItems.Append(button)
			w.savedDynamoQueryButtons = append(w.savedDynamoQueryButtons, button)
		}
	}
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

func (w *mainWindow) promptSaveDynamoTable() {
	if w.options.Config == nil || w.selectedDynamoTable == "" {
		return
	}
	table := w.selectedDynamoTable
	dialog, entry := w.newSavedLogNameDialog("Save DynamoDB table", "Destination name", "")
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.AddCSSClass("error")
	dialog.ContentArea().Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Save", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		name := strings.TrimSpace(entry.Text())
		if name == "" {
			errorLabel.SetLabel("Enter a destination name")
			return
		}
		if _, found := findDynamoTable(w.options.Config.DynamoTables, name); found {
			errorLabel.SetLabel("A saved table already uses that name")
			return
		}
		if !w.mutateDynamoConfig(func(cfg *config.Config) { cfg.AddDynamoTable(name, table) }) {
			return
		}
		dialog.Destroy()
		w.activeSavedDynamoTable = name
		w.rebuildDynamoRail()
		w.setBreadcrumb(dynamoBreadcrumb(name, table))
		w.updateActionSensitivity()
		w.setStatus("Saved DynamoDB table "+name, false)
	})
	dialog.Present()
}

func (w *mainWindow) promptSaveDynamoQuery() {
	if w.options.Config == nil || strings.TrimSpace(w.dynamoPartiQL) == "" {
		return
	}
	statement := w.dynamoPartiQL
	dialog, entry := w.newSavedLogNameDialog("Save DynamoDB query", "Query name", "")
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.AddCSSClass("error")
	dialog.ContentArea().Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Save", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		name := strings.TrimSpace(entry.Text())
		if name == "" {
			errorLabel.SetLabel("Enter a query name")
			return
		}
		if _, found := findDynamoQuery(w.options.Config.DynamoQueries, name); found {
			errorLabel.SetLabel("A saved query already uses that name")
			return
		}
		if !w.mutateDynamoConfig(func(cfg *config.Config) { cfg.AddDynamoQuery(name, statement) }) {
			return
		}
		dialog.Destroy()
		w.activeSavedDynamoQuery = name
		w.rebuildDynamoRail()
		w.setBreadcrumb(dynamoItemBreadcrumb("", name, ""))
		w.updateActionSensitivity()
		w.setStatus("Saved DynamoDB query "+name, false)
	})
	dialog.Present()
}

type savedDynamoDestination struct {
	kind  string
	name  string
	value string
}

func (w *mainWindow) promptManageDynamoSaved() {
	if w.options.Config == nil {
		return
	}
	var destinations []savedDynamoDestination
	for _, table := range w.options.Config.DynamoTables {
		destinations = append(destinations, savedDynamoDestination{kind: "Table", name: table.Name, value: table.Table})
	}
	for _, query := range w.options.Config.DynamoQueries {
		destinations = append(destinations, savedDynamoDestination{kind: "Query", name: query.Name, value: query.Statement})
	}
	if len(destinations) == 0 {
		return
	}
	sort.SliceStable(destinations, func(i, j int) bool {
		if destinations[i].kind != destinations[j].kind {
			return destinations[i].kind < destinations[j].kind
		}
		return strings.ToLower(destinations[i].name) < strings.ToLower(destinations[j].name)
	})
	labels := make([]string, len(destinations))
	for i, destination := range destinations {
		labels[i] = destination.kind + ": " + destination.name
	}
	dialog := gtk.NewDialogWithFlags("Saved DynamoDB destinations", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	selector := gtk.NewDropDownFromStrings(labels)
	nameEntry := gtk.NewEntry()
	valueEntry := gtk.NewEntry()
	kindLabel := gtk.NewLabel("")
	kindLabel.SetXAlign(0)
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.AddCSSClass("error")
	loadSelected := func() {
		index := int(selector.Selected())
		if index < 0 || index >= len(destinations) {
			return
		}
		destination := destinations[index]
		kindLabel.SetLabel(destination.kind + " value")
		nameEntry.SetText(destination.name)
		valueEntry.SetText(destination.value)
		errorLabel.SetLabel("")
	}
	selector.NotifyProperty("selected", loadSelected)
	loadSelected()
	content.Append(selector)
	nameLabel := gtk.NewLabel("Name")
	nameLabel.SetXAlign(0)
	content.Append(nameLabel)
	content.Append(nameEntry)
	content.Append(kindLabel)
	content.Append(valueEntry)
	content.Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Open", 101)
	dialog.AddButton("Delete…", 102)
	dialog.AddButton("Apply", 103)
	dialog.ConnectResponse(func(response int) {
		index := int(selector.Selected())
		if index < 0 || index >= len(destinations) {
			dialog.Destroy()
			return
		}
		current := destinations[index]
		switch response {
		case 101:
			dialog.Destroy()
			w.openSavedDynamoDestination(current)
		case 102:
			dialog.Destroy()
			w.confirmDeleteDynamoDestination(current)
		case 103:
			name, value := strings.TrimSpace(nameEntry.Text()), strings.TrimSpace(valueEntry.Text())
			if name == "" || value == "" {
				errorLabel.SetLabel("Name and value are required")
				return
			}
			if name != current.name {
				if current.kind == "Table" {
					if _, found := findDynamoTable(w.options.Config.DynamoTables, name); found {
						errorLabel.SetLabel("A saved table already uses that name")
						return
					}
				} else if _, found := findDynamoQuery(w.options.Config.DynamoQueries, name); found {
					errorLabel.SetLabel("A saved query already uses that name")
					return
				}
			}
			if !w.mutateDynamoConfig(func(cfg *config.Config) {
				if current.kind == "Table" {
					cfg.RemoveDynamoTable(current.name)
					cfg.AddDynamoTable(name, value)
				} else {
					cfg.RemoveDynamoQuery(current.name)
					cfg.AddDynamoQuery(name, value)
				}
			}) {
				return
			}
			dialog.Destroy()
			w.rebuildDynamoRail()
			if (current.kind == "Table" && current.name == w.activeSavedDynamoTable) || (current.kind == "Query" && current.name == w.activeSavedDynamoQuery) {
				w.openSavedDynamoDestination(savedDynamoDestination{kind: current.kind, name: name, value: value})
			} else {
				w.updateActionSensitivity()
			}
			w.setStatus("Updated saved DynamoDB "+strings.ToLower(current.kind)+" "+name, false)
		default:
			dialog.Destroy()
		}
	})
	dialog.Present()
}

func (w *mainWindow) openSavedDynamoDestination(destination savedDynamoDestination) {
	if destination.kind == "Table" {
		w.loadDynamoItems(destination.value, destination.name, "")
		return
	}
	w.loadDynamoPartiQL(destination.value, destination.name)
}

func (w *mainWindow) confirmDeleteDynamoDestination(destination savedDynamoDestination) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetMarkup("Delete saved DynamoDB " + strings.ToLower(destination.kind) + " <b>" + html.EscapeString(destination.name) + "</b>?")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Delete", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response != int(gtk.ResponseOK) {
			return
		}
		if !w.mutateDynamoConfig(func(cfg *config.Config) {
			if destination.kind == "Table" {
				cfg.RemoveDynamoTable(destination.name)
			} else {
				cfg.RemoveDynamoQuery(destination.name)
			}
		}) {
			return
		}
		wasActive := destination.name == w.activeSavedDynamoTable || destination.name == w.activeSavedDynamoQuery
		w.rebuildDynamoRail()
		if wasActive {
			w.loadDynamoTables("", "")
		} else {
			w.updateActionSensitivity()
		}
		w.setStatus("Deleted saved DynamoDB "+strings.ToLower(destination.kind)+" "+destination.name, false)
	})
	dialog.Present()
}

func (w *mainWindow) mutateDynamoConfig(mutate func(*config.Config)) bool {
	if w.options.Config == nil {
		return false
	}
	beforeTables := append([]config.DynamoTable(nil), w.options.Config.DynamoTables...)
	beforeQueries := append([]config.DynamoQuery(nil), w.options.Config.DynamoQueries...)
	mutate(w.options.Config)
	if err := w.options.Config.Save(); err != nil {
		w.options.Config.DynamoTables = beforeTables
		w.options.Config.DynamoQueries = beforeQueries
		w.setStatus("Save DynamoDB destination: "+err.Error(), true)
		return false
	}
	return true
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

func findDynamoQuery(queries []config.DynamoQuery, name string) (config.DynamoQuery, bool) {
	for _, query := range queries {
		if query.Name == name {
			return query, true
		}
	}
	return config.DynamoQuery{}, false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
