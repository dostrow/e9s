package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type EC2ResourceRow struct {
	ID     string
	Search string
	Cells  []string
}

type EC2ResourceListModel struct {
	title       string
	columns     []string
	rows        []EC2ResourceRow
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewEC2ResourceList(title string, columns []string) EC2ResourceListModel {
	return EC2ResourceListModel{title: title, columns: columns}
}

func (m EC2ResourceListModel) Update(msg tea.Msg) (EC2ResourceListModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.filtering {
		switch keyMsg.String() {
		case "enter":
			m.filter, m.filtering, m.cursor = m.filterInput.Value(), false, 0
			return m, nil
		case "esc":
			m.filtering = false
			return m, nil
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(keyMsg)
		return m, cmd
	}
	switch {
	case key.Matches(keyMsg, theme.Keys.Up):
		m.cursor = max(0, m.cursor-1)
	case key.Matches(keyMsg, theme.Keys.Down):
		m.cursor = min(m.cursor+1, max(0, len(m.filtered())-1))
	case key.Matches(keyMsg, theme.Keys.Filter):
		m.filtering = true
		m.filterInput = textinput.New()
		m.filterInput.SetValue(m.filter)
		m.filterInput.Placeholder = "filter..."
		m.filterInput.Focus()
		m.filterInput.Width = 40
		return m, m.filterInput.Focus()
	}
	return m, nil
}

func (m EC2ResourceListModel) View() string {
	rows := m.filtered()
	var out strings.Builder
	out.WriteString(theme.TitleStyle.Render(fmt.Sprintf("  %s (%d)", m.title, len(rows))))
	if m.filter != "" {
		out.WriteString(theme.HelpStyle.Render(fmt.Sprintf("  filter: %q", m.filter)))
	}
	out.WriteByte('\n')
	if m.filtering {
		out.WriteString("  / " + m.filterInput.View() + "\n")
	}
	out.WriteByte('\n')
	if len(rows) == 0 {
		if m.loaded {
			out.WriteString(theme.HelpStyle.Render("  No resources found"))
		} else {
			out.WriteString(theme.HelpStyle.Render("  Loading..."))
		}
		return out.String()
	}
	columns := make([]components.Column, len(m.columns))
	for index, title := range m.columns {
		columns[index] = components.Column{Title: title}
	}
	table := components.NewTable(columns)
	for _, row := range rows {
		cells := make([]components.Cell, len(row.Cells))
		for index, value := range row.Cells {
			cells[index] = components.Plain(value)
		}
		table.AddRow(cells...)
	}
	out.WriteString(table.Render(m.cursor, "", m.visibleRows()))
	return out.String()
}

func (m EC2ResourceListModel) SetRows(rows []EC2ResourceRow) EC2ResourceListModel {
	m.rows, m.loaded = rows, true
	if m.cursor >= len(m.filtered()) {
		m.cursor = max(0, len(m.filtered())-1)
	}
	return m
}
func (m EC2ResourceListModel) SelectedID() string {
	rows := m.filtered()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return ""
	}
	return rows[m.cursor].ID
}
func (m EC2ResourceListModel) IsFiltering() bool { return m.filtering }
func (m EC2ResourceListModel) SetSize(width, height int) EC2ResourceListModel {
	m.width, m.height = width, height
	return m
}
func (m EC2ResourceListModel) filtered() []EC2ResourceRow {
	filter := strings.ToLower(strings.TrimSpace(m.filter))
	if filter == "" {
		return m.rows
	}
	rows := make([]EC2ResourceRow, 0, len(m.rows))
	for _, row := range m.rows {
		if strings.Contains(strings.ToLower(row.Search+" "+strings.Join(row.Cells, " ")), filter) {
			rows = append(rows, row)
		}
	}
	return rows
}
func (m EC2ResourceListModel) visibleRows() int {
	if m.height < 10 {
		return 20
	}
	return m.height - 5
}

type EC2ResourceDetailModel struct {
	content string
	scroll  int
	width   int
	height  int
}

func NewEC2ResourceDetail(content string) EC2ResourceDetailModel {
	return EC2ResourceDetailModel{content: content}
}
func (m EC2ResourceDetailModel) Update(msg tea.Msg) (EC2ResourceDetailModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "j", "down":
			m.scroll++
		case "k", "up":
			m.scroll = max(0, m.scroll-1)
		case "g":
			m.scroll = 0
		case "G":
			m.scroll = 999999
		case "pgup":
			m.scroll = max(0, m.scroll-m.visibleRows())
		case "pgdown":
			m.scroll += m.visibleRows()
		}
	}
	return m, nil
}
func (m EC2ResourceDetailModel) View() string {
	lines := strings.Split(m.content, "\n")
	visible := m.visibleRows()
	if m.scroll > len(lines)-visible {
		m.scroll = max(0, len(lines)-visible)
	}
	return strings.Join(lines[m.scroll:min(len(lines), m.scroll+visible)], "\n")
}
func (m EC2ResourceDetailModel) SetSize(width, height int) EC2ResourceDetailModel {
	m.width, m.height = width, height
	return m
}
func (m EC2ResourceDetailModel) visibleRows() int {
	if m.height < 8 {
		return 20
	}
	return m.height - 2
}
