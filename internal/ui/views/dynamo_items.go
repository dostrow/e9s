package views

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type DynamoItemsModel struct {
	tableName string
	items     []aws.DynamoItem
	columns   []string // discovered attribute names
	cursor    int
	hasMore   bool // whether there are more pages
	width     int
	height    int
}

func NewDynamoItems(tableName string) DynamoItemsModel {
	return DynamoItemsModel{tableName: tableName}
}

func (m DynamoItemsModel) Update(msg tea.Msg) (DynamoItemsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m DynamoItemsModel) View() string {
	var b strings.Builder

	title := fmt.Sprintf("  DynamoDB: %s (%d items)", m.tableName, len(m.items))
	b.WriteString(theme.TitleStyle.Render(title))
	if m.hasMore {
		b.WriteString(theme.HelpStyle.Render("  [more available]"))
	}
	b.WriteString("\n\n")

	if len(m.items) == 0 {
		b.WriteString(theme.HelpStyle.Render("  No items found"))
		return b.String()
	}

	// Build table columns from discovered attributes
	cols := m.columns
	if len(cols) == 0 {
		cols = discoverColumns(m.items)
	}

	tblCols := make([]components.Column, len(cols))
	for i, c := range cols {
		tblCols[i] = components.Column{Title: c}
	}
	tbl := components.NewTable(tblCols)

	for _, item := range m.items {
		cells := make([]components.Cell, len(cols))
		for i, col := range cols {
			val := formatDynamoValue(item[col])
			if len(val) > 40 {
				val = val[:40] + ".."
			}
			cells[i] = components.Plain(val)
		}
		tbl.AddRow(cells...)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func discoverColumns(items []aws.DynamoItem) []string {
	seen := map[string]bool{}
	var cols []string
	for _, item := range items {
		for k := range item {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	sort.Strings(cols)
	return cols
}

func formatDynamoValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		// Collapse multi-line values to single line for table display
		s := strings.ReplaceAll(val, "\n", "\\n")
		s = strings.ReplaceAll(s, "\r", "")
		s = strings.ReplaceAll(s, "\t", " ")
		return s
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case map[string]interface{}:
		b, _ := json.Marshal(val)
		return string(b)
	case []interface{}:
		b, _ := json.Marshal(val)
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (m DynamoItemsModel) SetItems(items []aws.DynamoItem, hasMore bool) DynamoItemsModel {
	m.items = items
	m.hasMore = hasMore
	m.columns = discoverColumns(items)
	if m.cursor >= len(items) && len(items) > 0 {
		m.cursor = len(items) - 1
	}
	return m
}

func (m DynamoItemsModel) AppendItems(items []aws.DynamoItem, hasMore bool) DynamoItemsModel {
	m.items = append(m.items, items...)
	m.hasMore = hasMore
	m.columns = discoverColumns(m.items)
	return m
}

func (m DynamoItemsModel) SelectedItem() *aws.DynamoItem {
	if len(m.items) == 0 || m.cursor >= len(m.items) {
		return nil
	}
	item := m.items[m.cursor]
	return &item
}

func (m DynamoItemsModel) TableName() string { return m.tableName }
func (m DynamoItemsModel) HasMore() bool     { return m.hasMore }

func (m DynamoItemsModel) visibleRows() int {
	overhead := 9
	rows := m.height - overhead
	if rows < 5 {
		return 0
	}
	return rows
}

func (m DynamoItemsModel) SetSize(w, h int) DynamoItemsModel {
	m.width = w
	m.height = h
	return m
}
