package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/theme"
)

// DynamoEditFieldMsg is emitted when the user wants to edit a field.
type DynamoEditFieldMsg struct {
	TableName string
	Item      *aws.DynamoItem
	KeyNames  []string
	FieldName string
	FieldValue string
	IsKeyField bool
}

// DynamoCloneItemMsg is emitted when the user wants to clone the item.
type DynamoCloneItemMsg struct {
	TableName string
	Item      *aws.DynamoItem
	JSON      string
}

type fieldEntry struct {
	key      string
	lines    []string // rendered lines for this field
	isKey    bool     // is a key attribute (read-only for edit)
}

// DynamoItemDetailModel displays a single DynamoDB item with field-level cursor.
type DynamoItemDetailModel struct {
	tableName string
	keyNames  []string
	item      *aws.DynamoItem
	fields    []fieldEntry
	cursor    int // which field is selected
	scroll    int
	width     int
	height    int
}

func NewDynamoItemDetail(tableName string, keyNames []string, item *aws.DynamoItem) DynamoItemDetailModel {
	var fields []fieldEntry
	if item != nil {
		fields = buildFieldEntries(*item, keyNames)
	}
	return DynamoItemDetailModel{
		tableName: tableName,
		keyNames:  keyNames,
		item:      item,
		fields:    fields,
	}
}

func buildFieldEntries(item aws.DynamoItem, keyNames []string) []fieldEntry {
	keySet := map[string]bool{}
	for _, k := range keyNames {
		keySet[k] = true
	}

	keys := make([]string, 0, len(item))
	for k := range item {
		keys = append(keys, k)
	}

	// Put key fields first, then sort the rest
	sort.SliceStable(keys, func(i, j int) bool {
		iKey := keySet[keys[i]]
		jKey := keySet[keys[j]]
		if iKey != jKey {
			return iKey
		}
		return keys[i] < keys[j]
	})

	indent := "      "
	var fields []fieldEntry
	for _, k := range keys {
		v := item[k]
		valStr := formatDetailValue(v)
		isKey := keySet[k]

		var lines []string
		if strings.Contains(valStr, "\n") {
			lines = append(lines, renderFieldKey(k, isKey)+":")
			for _, vline := range strings.Split(valStr, "\n") {
				lines = append(lines, indent+vline)
			}
		} else if len(valStr) > 80 {
			lines = append(lines, renderFieldKey(k, isKey)+":")
			lines = append(lines, indent+valStr)
		} else {
			lines = append(lines, fmt.Sprintf("%s: %s", renderFieldKey(k, isKey), valStr))
		}

		fields = append(fields, fieldEntry{
			key:   k,
			lines: lines,
			isKey: isKey,
		})
	}
	return fields
}

func renderFieldKey(name string, isKey bool) string {
	if isKey {
		return lipgloss.NewStyle().Foreground(theme.ColorYellow).Bold(true).Render(name + " (key)")
	}
	return theme.HeaderStyle.Render(name)
}

func (m DynamoItemDetailModel) Update(msg tea.Msg) (DynamoItemDetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
			}
		case key.Matches(msg, theme.Keys.Down):
			if m.cursor < len(m.fields)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-5)
			m.ensureCursorVisible()
		case msg.String() == "pgdown":
			m.cursor = min(m.cursor+5, len(m.fields)-1)
			m.ensureCursorVisible()
		case msg.String() == "g":
			m.cursor = 0
			m.scroll = 0
		case msg.String() == "G":
			if len(m.fields) > 0 {
				m.cursor = len(m.fields) - 1
				m.ensureCursorVisible()
			}
		}
	}
	return m, nil
}

func (m DynamoItemDetailModel) View() string {
	var b strings.Builder

	b.WriteString(theme.TitleStyle.Render(fmt.Sprintf("  Item: %s", m.tableName)))
	b.WriteString("\n\n")

	if m.item == nil || len(m.fields) == 0 {
		b.WriteString(theme.HelpStyle.Render("  No item selected"))
		return b.String()
	}

	// Build all display lines with field indices
	type displayLine struct {
		text     string
		fieldIdx int
		isFirst  bool // first line of a field (gets the cursor)
	}
	var allLines []displayLine
	for fi, f := range m.fields {
		for li, line := range f.lines {
			allLines = append(allLines, displayLine{
				text:     line,
				fieldIdx: fi,
				isFirst:  li == 0,
			})
		}
		// Blank separator between fields
		allLines = append(allLines, displayLine{text: "", fieldIdx: fi})
	}

	visible := m.visibleLines()
	start := m.scroll
	end := min(start+visible, len(allLines))

	for _, dl := range allLines[start:end] {
		marker := "  "
		if dl.fieldIdx == m.cursor && dl.isFirst {
			marker = "► "
		}
		b.WriteString(fmt.Sprintf("  %s%s\n", marker, dl.text))
	}

	if len(allLines) > visible {
		fmt.Fprintf(&b, "\n  %d–%d of %d lines", start+1, end, len(allLines))
		if start > 0 {
			b.WriteString(" ↑")
		}
		if end < len(allLines) {
			b.WriteString(" ↓")
		}
	}

	return b.String()
}

func (m *DynamoItemDetailModel) ensureCursorVisible() {
	// Find the line index of the cursor field's first line
	lineIdx := 0
	for i := 0; i < m.cursor && i < len(m.fields); i++ {
		lineIdx += len(m.fields[i].lines) + 1 // +1 for separator
	}

	visible := m.visibleLines()
	if lineIdx < m.scroll {
		m.scroll = lineIdx
	}
	if lineIdx >= m.scroll+visible {
		m.scroll = lineIdx - visible + 1
	}
	m.scroll = max(0, m.scroll)
}

func (m DynamoItemDetailModel) SelectedField() (string, string, bool) {
	if m.cursor >= len(m.fields) {
		return "", "", false
	}
	f := m.fields[m.cursor]
	val := ""
	if m.item != nil {
		val = formatDetailValue((*m.item)[f.key])
	}
	return f.key, val, f.isKey
}

func (m DynamoItemDetailModel) Item() *aws.DynamoItem   { return m.item }
func (m DynamoItemDetailModel) TableName() string        { return m.tableName }
func (m DynamoItemDetailModel) KeyNames() []string       { return m.keyNames }

func (m DynamoItemDetailModel) visibleLines() int {
	h := m.height - 6
	if h < 5 {
		return 20
	}
	return h
}

func (m DynamoItemDetailModel) SetSize(w, h int) DynamoItemDetailModel {
	m.width = w
	m.height = h
	return m
}

// formatDetailValue formats a value for the detail view, preserving newlines in strings.
func formatDetailValue(v interface{}) string {
	if v == nil {
		return "(null)"
	}
	switch val := v.(type) {
	case string:
		return val
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
