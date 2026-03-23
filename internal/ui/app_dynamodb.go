package ui

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
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
	client := a.client
	return a, func() tea.Msg {
		tables, err := client.ListDynamoTables(context.Background(), filter)
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
	client := a.client
	return a, func() tea.Msg {
		// Fetch key schema first
		desc, err := client.DescribeDynamoTable(context.Background(), tableName)
		var keyNames []string
		if err == nil {
			for _, k := range desc.KeySchema {
				keyNames = append(keyNames, k.Name)
			}
		}
		result, err := client.ScanDynamoTable(context.Background(), tableName, 50, nil)
		if err != nil {
			return errMsg{err}
		}
		return dynamoScanReadyMsg{
			tableName: tableName,
			keyNames:  keyNames,
			items:     result.Items,
			hasMore:   len(result.LastEvaluatedKey) > 0,
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
		"= (equals)":          "=",
		"<> (not equals)":     "<>",
		"< (less than)":       "<",
		"<= (less or equal)":  "<=",
		"> (greater than)":    ">",
		">= (greater or equal)": ">=",
		"begins_with":         "begins_with",
		"contains":            "contains",
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
	isFunc := a.dynamoFilterExpr

	a.loading = true
	client := a.client
	keyNames := a.dynamoKeyNames

	return a, func() tea.Msg {
		_ = keyNames // used in message
		var result *aws.DynamoScanResult
		var err error
		if isFunc {
			// Use function-style filter: contains(#attr, :val)
			result, err = client.ScanDynamoTableWithFilter(context.Background(),
				tableName, attr, fmt.Sprintf("%s(#attr, :val)", op), value, 100, nil)
			// Rewrite the filter to use function syntax
			if err != nil {
				// Retry with corrected expression
				result, err = client.ScanDynamoTableWithFuncFilter(context.Background(),
					tableName, attr, op, value, 100, nil)
			}
		} else {
			result, err = client.ScanDynamoTableWithFilter(context.Background(),
				tableName, attr, op, value, 100, nil)
		}
		if err != nil {
			return errMsg{err}
		}
		return dynamoItemsLoadedMsg{items: result.Items, hasMore: len(result.LastEvaluatedKey) > 0}
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
	client := a.client
	return a, func() tea.Msg {
		items, err := client.ExecutePartiQL(context.Background(), statement)
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
	client := a.client
	tableName := a.dynamoDetailView.TableName()
	keyNames := a.dynamoDetailView.KeyNames()
	item := a.dynamoEditItem
	fieldName := a.dynamoEditField
	newValue := a.dynamoEditValue

	return func() tea.Msg {
		if item == nil {
			return errMsg{fmt.Errorf("no item to edit")}
		}
		keyAV, err := aws.BuildKeyFromItem(*item, keyNames)
		if err != nil {
			return errMsg{err}
		}
		err = client.UpdateDynamoField(context.Background(), tableName, keyAV, fieldName, newValue)
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

	jsonStr := aws.DynamoItemToJSON(*item)

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
		newItem, err := aws.ParseDynamoItemFromJSON(string(data))
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
	client := a.client
	tableName := a.dynamoDetailView.TableName()
	item := a.dynamoCloneItem

	return func() tea.Msg {
		if item == nil {
			return errMsg{fmt.Errorf("no item to clone")}
		}
		err := client.PutDynamoItem(context.Background(), tableName, *item)
		if err != nil {
			return dynamoWriteDoneMsg{err: err}
		}
		return dynamoWriteDoneMsg{message: "Item created successfully"}
	}
}
