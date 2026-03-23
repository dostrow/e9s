package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/theme"
)

// DynamoItemDetailModel displays a single DynamoDB item with multi-line value support.
type DynamoItemDetailModel struct {
	tableName string
	item      *aws.DynamoItem
	lines     []string
	scroll    int
	width     int
	height    int
}

func NewDynamoItemDetail(tableName string, item *aws.DynamoItem) DynamoItemDetailModel {
	var lines []string
	if item != nil {
		lines = formatItemForDetail(*item)
	}
	return DynamoItemDetailModel{
		tableName: tableName,
		item:      item,
		lines:     lines,
	}
}

// formatItemForDetail renders an item with multi-line values displayed
// on indented lines below their key name.
func formatItemForDetail(item aws.DynamoItem) []string {
	keys := make([]string, 0, len(item))
	for k := range item {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	indent := "      " // 6 spaces for value continuation

	for _, k := range keys {
		v := item[k]
		valStr := formatDetailValue(v)

		if strings.Contains(valStr, "\n") {
			// Multi-line value: key on its own line, value indented below
			lines = append(lines, theme.HeaderStyle.Render(k)+":")
			for _, vline := range strings.Split(valStr, "\n") {
				lines = append(lines, indent+vline)
			}
			lines = append(lines, "") // blank separator
		} else if len(valStr) > 80 {
			// Long single-line value: key on its own line, value indented below
			lines = append(lines, theme.HeaderStyle.Render(k)+":")
			lines = append(lines, indent+valStr)
			lines = append(lines, "")
		} else {
			// Short value: key and value on same line
			lines = append(lines, fmt.Sprintf("%s: %s", theme.HeaderStyle.Render(k), valStr))
		}
	}
	return lines
}

// formatDetailValue formats a value for the detail view, preserving newlines in strings.
func formatDetailValue(v interface{}) string {
	if v == nil {
		return "(null)"
	}
	switch val := v.(type) {
	case string:
		return val // preserve newlines
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
		return formatMapIndented(val, "  ")
	case []interface{}:
		return formatSliceIndented(val, "  ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func formatMapIndented(m map[string]interface{}, prefix string) string {
	if len(m) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("{\n")
	for i, k := range keys {
		vStr := formatDetailValue(m[k])
		// Indent nested values
		indented := strings.ReplaceAll(vStr, "\n", "\n"+prefix+"  ")
		fmt.Fprintf(&b, "%s  %q: %s", prefix, k, indented)
		if i < len(keys)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(prefix + "}")
	return b.String()
}

func formatSliceIndented(s []interface{}, prefix string) string {
	if len(s) == 0 {
		return "[]"
	}
	var b strings.Builder
	b.WriteString("[\n")
	for i, v := range s {
		vStr := formatDetailValue(v)
		indented := strings.ReplaceAll(vStr, "\n", "\n"+prefix+"  ")
		fmt.Fprintf(&b, "%s  %s", prefix, indented)
		if i < len(s)-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(prefix + "]")
	return b.String()
}

func (m DynamoItemDetailModel) Update(msg tea.Msg) (DynamoItemDetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if m.scroll > 0 {
				m.scroll--
			}
		case key.Matches(msg, theme.Keys.Down):
			m.scroll = min(m.scroll+1, m.maxScroll())
		case msg.String() == "pgup":
			m.scroll = max(0, m.scroll-m.visibleLines())
		case msg.String() == "pgdown":
			m.scroll = min(m.scroll+m.visibleLines(), m.maxScroll())
		case msg.String() == "g":
			m.scroll = 0
		case msg.String() == "G":
			m.scroll = m.maxScroll()
		}
	}
	return m, nil
}

func (m DynamoItemDetailModel) View() string {
	var b strings.Builder

	b.WriteString(theme.TitleStyle.Render(fmt.Sprintf("  Item: %s", m.tableName)))
	b.WriteString("\n\n")

	if m.item == nil {
		b.WriteString(theme.HelpStyle.Render("  No item selected"))
		return b.String()
	}

	visible := m.visibleLines()
	start := m.scroll
	end := min(start+visible, len(m.lines))

	for _, line := range m.lines[start:end] {
		b.WriteString("    " + line + "\n")
	}

	if len(m.lines) > visible {
		fmt.Fprintf(&b, "\n  %d–%d of %d lines", start+1, end, len(m.lines))
		if start > 0 {
			b.WriteString(" ↑")
		}
		if end < len(m.lines) {
			b.WriteString(" ↓")
		}
	}

	return b.String()
}

func (m DynamoItemDetailModel) visibleLines() int {
	h := m.height - 6
	if h < 5 {
		return 20
	}
	return h
}

func (m DynamoItemDetailModel) maxScroll() int {
	return max(0, len(m.lines)-m.visibleLines())
}

func (m DynamoItemDetailModel) SetSize(w, h int) DynamoItemDetailModel {
	m.width = w
	m.height = h
	return m
}
