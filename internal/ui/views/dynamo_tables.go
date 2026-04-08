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

type DynamoTablesModel struct {
	tables      []string
	searchTerm  string
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewDynamoTables(searchTerm string) DynamoTablesModel {
	return DynamoTablesModel{searchTerm: searchTerm}
}

func (m DynamoTablesModel) Update(msg tea.Msg) (DynamoTablesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filtering {
			switch msg.String() {
			case "enter":
				m.filter = m.filterInput.Value()
				m.filtering = false
				m.cursor = 0
				return m, nil
			case "esc":
				m.filtering = false
				return m, nil
			}
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			return m, cmd
		}

		switch {
		case key.Matches(msg, theme.Keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			filtered := m.filteredTables()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredTables()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter tables..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m DynamoTablesModel) View() string {
	filtered := m.filteredTables()
	var b strings.Builder

	title := fmt.Sprintf("  DynamoDB Tables (%d)", len(filtered))
	if m.searchTerm != "" {
		title += fmt.Sprintf(" — search: %s", m.searchTerm)
	}
	b.WriteString(theme.TitleStyle.Render(title))
	if m.filter != "" {
		b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("  filter: %q", m.filter)))
	}
	b.WriteString("\n")

	if m.filtering {
		b.WriteString("  / " + m.filterInput.View() + "\n")
	}
	b.WriteString("\n")

	if len(filtered) == 0 {
		if !m.loaded {
			b.WriteString(theme.HelpStyle.Render("  Loading..."))
		} else {
			b.WriteString(theme.HelpStyle.Render("  No tables found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "TABLE NAME"},
	})

	for _, t := range filtered {
		tbl.AddRow(components.Plain(t))
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m DynamoTablesModel) filteredTables() []string {
	if m.filter == "" {
		return m.tables
	}
	lf := strings.ToLower(m.filter)
	var out []string
	for _, t := range m.tables {
		if strings.Contains(strings.ToLower(t), lf) {
			out = append(out, t)
		}
	}
	return out
}

func (m DynamoTablesModel) SetTables(tables []string) DynamoTablesModel {
	m.tables = tables
	m.loaded = true
	filtered := m.filteredTables()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m DynamoTablesModel) SelectedTable() string {
	filtered := m.filteredTables()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return ""
	}
	return filtered[m.cursor]
}

func (m DynamoTablesModel) SearchTerm() string  { return m.searchTerm }
func (m DynamoTablesModel) IsFiltering() bool    { return m.filtering }

func (m DynamoTablesModel) visibleRows() int {
	overhead := 9
	if m.filtering {
		overhead++
	}
	rows := m.height - overhead
	if rows < 5 {
		return 0
	}
	return rows
}

func (m DynamoTablesModel) SetSize(w, h int) DynamoTablesModel {
	m.width = w
	m.height = h
	return m
}
