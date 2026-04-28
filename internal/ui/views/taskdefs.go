package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type TaskDefsModel struct {
	defs        []aws.TaskDefRef
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewTaskDefs() TaskDefsModel {
	return TaskDefsModel{}
}

func (m TaskDefsModel) Update(msg tea.Msg) (TaskDefsModel, tea.Cmd) {
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
			filtered := m.filteredTaskDefs()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredTaskDefs()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter task definitions..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m TaskDefsModel) View() string {
	filtered := m.filteredTaskDefs()
	var b strings.Builder

	title := fmt.Sprintf("  Task Definitions (%d)", len(filtered))
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
			b.WriteString(theme.HelpStyle.Render("  No task definitions found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "FAMILY"},
		{Title: "REV", RightAlign: true},
		{Title: "TASK DEFINITION"},
	})

	for _, td := range filtered {
		tbl.AddRow(
			components.Plain(td.Family),
			components.Plain(fmt.Sprintf("%d", td.Revision)),
			components.Plain(td.ARN),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m TaskDefsModel) filteredTaskDefs() []aws.TaskDefRef {
	if m.filter == "" {
		return m.defs
	}
	lf := strings.ToLower(m.filter)
	var out []aws.TaskDefRef
	for _, td := range m.defs {
		if strings.Contains(strings.ToLower(td.Family), lf) || strings.Contains(strings.ToLower(td.ARN), lf) {
			out = append(out, td)
		}
	}
	return out
}

func (m TaskDefsModel) SetTaskDefs(defs []aws.TaskDefRef) TaskDefsModel {
	m.defs = defs
	m.loaded = true
	filtered := m.filteredTaskDefs()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m TaskDefsModel) SelectedTaskDef() *aws.TaskDefRef {
	filtered := m.filteredTaskDefs()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	td := filtered[m.cursor]
	return &td
}

func (m TaskDefsModel) IsFiltering() bool {
	return m.filtering
}

func (m TaskDefsModel) visibleRows() int {
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

func (m TaskDefsModel) SetSize(w, h int) TaskDefsModel {
	m.width = w
	m.height = h
	return m
}
