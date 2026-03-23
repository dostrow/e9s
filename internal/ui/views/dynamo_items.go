package views

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type DynamoItemsModel struct {
	tableName string
	keyNames  []string // partition key, then sort key (for column ordering)
	items     []aws.DynamoItem
	columns   []string // discovered attribute names
	cursor    int
	hasMore   bool // whether there are more pages
	width     int
	height    int
}

func NewDynamoItems(tableName string, keyNames []string) DynamoItemsModel {
	return DynamoItemsModel{tableName: tableName, keyNames: keyNames}
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
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(m.items)-1))
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

	// Build table columns from discovered attributes, keys first
	cols := m.columns
	if len(cols) == 0 {
		cols = discoverColumns(m.items, m.keyNames)
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
			if len(val) > 50 {
				cells[i] = components.Styled("[enter for detail]", lipgloss.NewStyle().Foreground(theme.ColorDim).Italic(true))
			} else {
				cells[i] = components.Plain(val)
			}
		}
		tbl.AddRow(cells...)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func discoverColumns(items []aws.DynamoItem, keyNames []string) []string {
	seen := map[string]bool{}
	var rest []string
	for _, item := range items {
		for k := range item {
			if !seen[k] {
				seen[k] = true
				rest = append(rest, k)
			}
		}
	}
	sort.Strings(rest)

	// Put key columns first in order, then the rest
	keySet := map[string]bool{}
	var cols []string
	for _, k := range keyNames {
		if seen[k] {
			cols = append(cols, k)
			keySet[k] = true
		}
	}
	for _, c := range rest {
		if !keySet[c] {
			cols = append(cols, c)
		}
	}
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
	m.columns = discoverColumns(items, m.keyNames)
	if m.cursor >= len(items) && len(items) > 0 {
		m.cursor = len(items) - 1
	}
	return m
}

func (m DynamoItemsModel) AppendItems(items []aws.DynamoItem, hasMore bool) DynamoItemsModel {
	m.items = append(m.items, items...)
	m.hasMore = hasMore
	m.columns = discoverColumns(m.items, m.keyNames)
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
