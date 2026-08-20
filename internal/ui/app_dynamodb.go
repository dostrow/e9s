package ui

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- DynamoDB ---

func (a App) promptDynamoBrowser() (App, tea.Cmd) {
	saved := a.cfg.DynamoTables
	if len(saved) == 0 {
		a.input = NewInput(InputDynamoSearch, "Search tables (substring match, or empty for all)", "")
		return a, nil
	}
	items := make([]string, 0, len(saved)+1)
	for _, t := range saved {
		items = append(items, fmt.Sprintf("%s  (%s)", t.Name, t.Table))
	}
	savedCount := len(items)
	items = append(items, "[enter a custom search]")
	a.picker = NewPickerWithDelete(PickerDynamoTable, "Select DynamoDB table", items, savedCount)
	return a, nil
}

func (a App) openDynamoTables(filter string) (App, tea.Cmd) {
	a.mode = modeDynamoDB
	a.state = viewDynamoTables
	a.dynamoTablesView = views.NewDynamoTables(filter)
	a.dynamoTablesView = a.dynamoTablesView.SetSize(a.width, a.height-3)
	a.loading = true
	dynamoService, ctx := a.dynamoDB, a.ctx
	return a, func() tea.Msg {
		tables, err := dynamoService.Tables(ctx, filter)
		if err != nil {
			return errMsg{err}
		}
		return dynamoTablesLoadedMsg{tables}
	}
}

func (a App) openDynamoTableDirect(tableName string) (App, tea.Cmd) {
	a.mode = modeDynamoDB
	return a.scanDynamoTable(tableName)
}

func (a App) scanDynamoTable(tableName string) (App, tea.Cmd) {
	a.state = viewDynamoItems
	a.loading = true
	dynamoService, ctx := a.dynamoDB, a.ctx
	return a, func() tea.Msg {
		desc, err := dynamoService.Table(ctx, tableName)
		if err != nil {
			return errMsg{err}
		}
		keyNames := make([]string, 0, len(desc.KeySchema))
		for _, key := range desc.KeySchema {
			keyNames = append(keyNames, key.Name)
		}
		result, err := dynamoService.Scan(ctx, model.DynamoScanRequest{Table: tableName, Limit: 50})
		if err != nil {
			return errMsg{err}
		}
		return dynamoScanReadyMsg{
			tableName: tableName,
			keyNames:  keyNames,
			items:     result.Items,
			hasMore:   result.NextToken != "",
			lastKey:   result.NextToken,
		}
	}
}

func (a App) loadDynamoNextPage() (App, tea.Cmd) {
	if !a.dynamoItemsView.HasMore() || a.dynamoLastKey == "" {
		return a, nil
	}
	tableName := a.dynamoItemsView.TableName()
	a.loading = true
	dynamoService, ctx := a.dynamoDB, a.ctx
	nextToken := a.dynamoLastKey
	return a, func() tea.Msg {
		result, err := dynamoService.Scan(ctx, model.DynamoScanRequest{Table: tableName, Limit: 50, NextToken: nextToken})
		if err != nil {
			return errMsg{err}
		}
		return dynamoPageLoadedMsg{
			items:   result.Items,
			hasMore: result.NextToken != "",
			lastKey: result.NextToken,
		}
	}
}

func (a App) saveDynamoTable() (App, tea.Cmd) {
	table := a.dynamoTablesView.SelectedTable()
	if table == "" {
		return a, nil
	}
	a.input = NewInput(InputDynamoSaveName,
		fmt.Sprintf("Save table %q — enter a name", table), "")
	return a, nil
}

func (a App) doSaveDynamoTable(name string) (App, tea.Cmd) {
	table := a.dynamoTablesView.SelectedTable()
	a.cfg.AddDynamoTable(name, table)
	if err := a.cfg.Save(); err != nil {
		a.err = err
		return a, nil
	}
	a.flashMessage = fmt.Sprintf("Saved table %q as %q", table, name)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

// --- Simple Search (filter scan) ---

func (a App) promptDynamoFilter() (App, tea.Cmd) {
	a.input = NewInput(InputDynamoFilterAttr, "Attribute name to filter on", "")
	return a, nil
}

func (a App) promptDynamoFilterOp() (App, tea.Cmd) {
	a.picker = NewPicker(PickerDynamoFilterOp, "Select operator", []string{
		"= (equals)",
		"<> (not equals)",
		"< (less than)",
		"<= (less or equal)",
		"> (greater than)",
		">= (greater or equal)",
		"begins_with",
		"contains",
	})
	return a, nil
}

func (a App) handleDynamoFilterOp(value string) (App, tea.Cmd) {
	// Extract operator from display string
	ops := map[string]string{
		"= (equals)":            "=",
		"<> (not equals)":       "<>",
		"< (less than)":         "<",
		"<= (less or equal)":    "<=",
		"> (greater than)":      ">",
		">= (greater or equal)": ">=",
		"begins_with":           "begins_with",
		"contains":              "contains",
	}
	a.dynamoFilterOp = ops[value]
	if a.dynamoFilterOp == "" {
		a.dynamoFilterOp = "="
	}

	// Handle function-style operators
	if a.dynamoFilterOp == "begins_with" || a.dynamoFilterOp == "contains" {
		a.dynamoFilterExpr = true
	}

	a.input = NewInput(InputDynamoFilterValue,
		fmt.Sprintf("Value to match (%s %s ?)", a.dynamoFilterAttr, a.dynamoFilterOp), "")
	return a, nil
}

func (a App) executeDynamoFilter(value string) (App, tea.Cmd) {
	tableName := a.dynamoItemsView.TableName()
	attr := a.dynamoFilterAttr
	op := a.dynamoFilterOp

	a.loading = true
	dynamoService, ctx := a.dynamoDB, a.ctx

	return a, func() tea.Msg {
		result, err := dynamoService.Scan(ctx, model.DynamoScanRequest{
			Table: tableName, Limit: 100,
			Filter: &model.DynamoFilter{Attribute: attr, Operator: op, Value: value},
		})
		if err != nil {
			return errMsg{err}
		}
		return dynamoItemsLoadedMsg{items: result.Items, hasMore: result.NextToken != "", lastKey: result.NextToken}
	}
}

// --- PartiQL ---

func (a App) promptDynamoPartiQL() (App, tea.Cmd) {
	saved := a.cfg.DynamoQueries
	if len(saved) == 0 {
		tableName := a.dynamoItemsView.TableName()
		a.input = NewInput(InputDynamoPartiQL, "PartiQL statement",
			fmt.Sprintf("SELECT * FROM \"%s\" WHERE ", tableName))
		return a, nil
	}
	items := make([]string, 0, len(saved)+1)
	for _, q := range saved {
		label := q.Name
		stmt := q.Statement
		if len(stmt) > 40 {
			stmt = stmt[:40] + ".."
		}
		items = append(items, fmt.Sprintf("%s  (%s)", label, stmt))
	}
	savedCount := len(items)
	items = append(items, "[enter a custom query]")
	a.picker = NewPickerWithDelete(PickerDynamoQuery, "Select PartiQL query", items, savedCount)
	return a, nil
}

func (a App) executeDynamoPartiQL(statement string) (App, tea.Cmd) {
	a.dynamoLastPartiQL = statement
	a.loading = true
	dynamoService, ctx := a.dynamoDB, a.ctx
	return a, func() tea.Msg {
		items, err := dynamoService.PartiQL(ctx, statement)
		return dynamoPartiQLResultMsg{items: items, err: err}
	}
}

func (a App) saveDynamoQuery() (App, tea.Cmd) {
	if a.dynamoLastPartiQL == "" {
		a.err = fmt.Errorf("no PartiQL query to save")
		return a, nil
	}
	a.input = NewInput(InputDynamoQuerySaveName, "Save query — enter a name", "")
	return a, nil
}

func (a App) doSaveDynamoQuery(name string) (App, tea.Cmd) {
	a.cfg.AddDynamoQuery(name, a.dynamoLastPartiQL)
	if err := a.cfg.Save(); err != nil {
		a.err = err
		return a, nil
	}
	a.flashMessage = fmt.Sprintf("Saved PartiQL query as %q", name)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

// --- Refresh Detail ---

func (a App) refreshDynamoDetail() tea.Cmd {
	dynamoService, ctx := a.dynamoDB, a.ctx
	tableName := a.dynamoDetailView.TableName()
	keyNames := a.dynamoDetailView.KeyNames()
	item := a.dynamoDetailView.Item()
	if item == nil {
		return nil
	}

	return func() tea.Msg {
		refreshed, err := dynamoService.Item(ctx, tableName, keyNames, *item)
		if err != nil {
			return errMsg{err}
		}
		return dynamoItemRefreshedMsg{item: refreshed}
	}
}

// --- Field Edit ---

func (a App) editDynamoField() (App, tea.Cmd) {
	fieldName, fieldValue, isKey := a.dynamoDetailView.SelectedField()
	if fieldName == "" {
		return a, nil
	}
	if isKey {
		a.err = fmt.Errorf("%q is a key attribute and cannot be edited — use clone instead", fieldName)
		return a, nil
	}

	// Write current value to temp file
	tmpFile, err := os.CreateTemp("", "e9s-dynamo-*.txt")
	if err != nil {
		a.err = err
		return a, nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(fieldValue)
	tmpFile.Close()

	item := a.dynamoDetailView.Item()
	tableName := a.dynamoDetailView.TableName()
	keyNames := a.dynamoDetailView.KeyNames()

	editor := NewEditorCmd(tmpPath)
	return a, tea.Exec(editor, func(err error) tea.Msg {
		defer os.Remove(tmpPath)
		if err != nil {
			return errMsg{err}
		}
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return errMsg{err}
		}
		newValue := string(data)
		return dynamoFieldEditedMsg{
			tableName: tableName,
			keyNames:  keyNames,
			item:      item,
			fieldName: fieldName,
			newValue:  newValue,
		}
	})
}

func (a App) doDynamoFieldEdit() tea.Cmd {
	dynamoService, ctx := a.dynamoDB, a.ctx
	tableName := a.dynamoDetailView.TableName()
	keyNames := a.dynamoDetailView.KeyNames()
	item := a.dynamoEditItem
	fieldName := a.dynamoEditField
	newValue := a.dynamoEditValue

	return func() tea.Msg {
		if item == nil {
			return errMsg{fmt.Errorf("no item to edit")}
		}
		originalValue := (*item)[fieldName]
		err := dynamoService.UpdateField(ctx, model.DynamoFieldUpdate{
			Table: tableName, KeyNames: keyNames, Item: *item, Attribute: fieldName,
			OriginalValue: originalValue, NewValue: newValue,
		})
		if err != nil {
			return dynamoWriteDoneMsg{err: err}
		}
		return dynamoWriteDoneMsg{message: fmt.Sprintf("Updated %q", fieldName)}
	}
}

// --- Clone Item ---

func (a App) cloneDynamoItem() (App, tea.Cmd) {
	item := a.dynamoDetailView.Item()
	if item == nil {
		return a, nil
	}

	jsonStr := service.DynamoItemToJSON(*item)

	tmpFile, err := os.CreateTemp("", "e9s-dynamo-clone-*.json")
	if err != nil {
		a.err = err
		return a, nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(jsonStr)
	tmpFile.Close()

	tableName := a.dynamoDetailView.TableName()

	editor := NewEditorCmd(tmpPath)
	return a, tea.Exec(editor, func(err error) tea.Msg {
		defer os.Remove(tmpPath)
		if err != nil {
			return errMsg{err}
		}
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return errMsg{err}
		}
		newItem, err := service.ParseDynamoItemJSON(string(data))
		if err != nil {
			return errMsg{err}
		}
		return dynamoItemClonedMsg{
			tableName: tableName,
			newItem:   newItem,
		}
	})
}

func (a App) doDynamoClone() tea.Cmd {
	dynamoService, ctx := a.dynamoDB, a.ctx
	tableName := a.dynamoDetailView.TableName()
	item := a.dynamoCloneItem

	return func() tea.Msg {
		if item == nil {
			return errMsg{fmt.Errorf("no item to clone")}
		}
		err := dynamoService.PutItem(ctx, tableName, *item)
		if err != nil {
			return dynamoWriteDoneMsg{err: err}
		}
		return dynamoWriteDoneMsg{message: "Item created successfully"}
	}
}
