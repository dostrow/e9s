package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/runbook"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type RunbooksModel struct {
	actions     []runbook.ConfiguredAction
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
}

func NewRunbooks(actions []runbook.ConfiguredAction) RunbooksModel {
	return RunbooksModel{actions: actions}
}

func (m RunbooksModel) Update(msg tea.Msg) (RunbooksModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.filtering {
		switch keyMsg.String() {
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
	case key.Matches(keyMsg, theme.Keys.Up):
		if m.cursor > 0 {
			m.cursor--
		}
	case key.Matches(keyMsg, theme.Keys.Down):
		if m.cursor < len(m.filtered())-1 {
			m.cursor++
		}
	case key.Matches(keyMsg, theme.Keys.Filter):
		m.filtering = true
		m.filterInput = textinput.New()
		m.filterInput.Placeholder = "filter plugins and actions..."
		m.filterInput.SetValue(m.filter)
		m.filterInput.Focus()
		m.filterInput.Width = 44
		return m, m.filterInput.Focus()
	}
	return m, nil
}

func (m RunbooksModel) View() string {
	actions := m.filtered()
	var b strings.Builder
	fmt.Fprintf(&b, "%s", theme.TitleStyle.Render(fmt.Sprintf("  Plugin Actions (%d)", len(actions))))
	if m.filter != "" {
		fmt.Fprintf(&b, "%s", theme.HelpStyle.Render(fmt.Sprintf("  filter: %q", m.filter)))
	}
	b.WriteString("\n")
	if m.filtering {
		b.WriteString("  / " + m.filterInput.View() + "\n")
	}
	b.WriteString("\n")
	if len(actions) == 0 {
		b.WriteString(theme.HelpStyle.Render("  No configured plugin actions"))
		return b.String()
	}
	table := components.NewTable([]components.Column{
		{Title: "PLUGIN"}, {Title: "ACTION"}, {Title: "RISK"}, {Title: "DESCRIPTION"},
	})
	for _, action := range actions {
		table.AddRow(
			components.Plain(action.PluginDisplayName),
			components.Plain(action.Action.Title),
			components.Plain(action.Action.Risk),
			components.Plain(action.Action.Description),
		)
	}
	b.WriteString(table.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m RunbooksModel) SelectedAction() *runbook.ConfiguredAction {
	actions := m.filtered()
	if m.cursor < 0 || m.cursor >= len(actions) {
		return nil
	}
	action := actions[m.cursor]
	return &action
}

func (m RunbooksModel) IsFiltering() bool { return m.filtering }

func (m RunbooksModel) SetSize(width, height int) RunbooksModel {
	m.width, m.height = width, height
	return m
}

func (m RunbooksModel) filtered() []runbook.ConfiguredAction {
	filter := strings.ToLower(strings.TrimSpace(m.filter))
	if filter == "" {
		return m.actions
	}
	result := make([]runbook.ConfiguredAction, 0, len(m.actions))
	for _, action := range m.actions {
		haystack := strings.ToLower(action.PluginDisplayName + " " + action.Action.Title + " " + action.Action.Description)
		if strings.Contains(haystack, filter) {
			result = append(result, action)
		}
	}
	return result
}

func (m RunbooksModel) visibleRows() int {
	rows := m.height - 9
	if m.filtering {
		rows--
	}
	return max(0, rows)
}
