package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/theme"
)

// DynamoItemDetailModel displays a single DynamoDB item as pretty-printed JSON.
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
		jsonStr := aws.DynamoItemToJSON(*item)
		lines = strings.Split(jsonStr, "\n")
	}
	return DynamoItemDetailModel{
		tableName: tableName,
		item:      item,
		lines:     lines,
	}
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
		b.WriteString("    " + colorJSONLineDynamo(line) + "\n")
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

func colorJSONLineDynamo(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]
	if before, after, ok := strings.Cut(trimmed, `":`); ok {
		return indent + theme.HeaderStyle.Render(before+`"`) + ":" + after
	}
	return line
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
