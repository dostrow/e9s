package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type LambdaListModel struct {
	functions   []aws.LambdaFunction
	searchTerm  string
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
}

func NewLambdaList(searchTerm string) LambdaListModel {
	return LambdaListModel{searchTerm: searchTerm}
}

func (m LambdaListModel) Update(msg tea.Msg) (LambdaListModel, tea.Cmd) {
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
			filtered := m.filteredFunctions()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredFunctions()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter functions..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 50
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m LambdaListModel) View() string {
	filtered := m.filteredFunctions()
	var b strings.Builder

	title := fmt.Sprintf("  Lambda Functions (%d)", len(filtered))
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
		b.WriteString(theme.HelpStyle.Render("  No functions found"))
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "NAME"},
		{Title: "RUNTIME"},
		{Title: "STATE"},
		{Title: "MEMORY", RightAlign: true},
		{Title: "TIMEOUT", RightAlign: true},
		{Title: "MODIFIED"},
	})

	for _, fn := range filtered {
		stateStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)
		switch fn.State {
		case "Active":
			stateStyle = lipgloss.NewStyle().Foreground(theme.ColorGreen)
		case "Inactive":
			stateStyle = lipgloss.NewStyle().Foreground(theme.ColorDim)
		case "Pending":
			stateStyle = lipgloss.NewStyle().Foreground(theme.ColorYellow)
		case "Failed":
			stateStyle = lipgloss.NewStyle().Foreground(theme.ColorRed)
		}

		modified := ""
		if !fn.LastModified.IsZero() {
			modified = formatAge(fn.LastModified) + " ago"
		}

		tbl.AddRow(
			components.Plain(fn.Name),
			components.Plain(fn.Runtime),
			components.Styled(fn.State, stateStyle),
			components.Plain(fmt.Sprintf("%d MB", fn.MemoryMB)),
			components.Plain(fmt.Sprintf("%ds", fn.TimeoutSec)),
			components.Plain(modified),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m LambdaListModel) filteredFunctions() []aws.LambdaFunction {
	if m.filter == "" {
		return m.functions
	}
	lf := strings.ToLower(m.filter)
	var out []aws.LambdaFunction
	for _, fn := range m.functions {
		if strings.Contains(strings.ToLower(fn.Name), lf) ||
			strings.Contains(strings.ToLower(fn.Runtime), lf) ||
			strings.Contains(strings.ToLower(fn.Description), lf) {
			out = append(out, fn)
		}
	}
	return out
}

func (m LambdaListModel) SetFunctions(functions []aws.LambdaFunction) LambdaListModel {
	m.functions = functions
	filtered := m.filteredFunctions()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m LambdaListModel) SelectedFunction() *aws.LambdaFunction {
	filtered := m.filteredFunctions()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	fn := filtered[m.cursor]
	return &fn
}

func (m LambdaListModel) SearchTerm() string  { return m.searchTerm }
func (m LambdaListModel) IsFiltering() bool    { return m.filtering }

func (m LambdaListModel) visibleRows() int {
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

func (m LambdaListModel) SetSize(w, h int) LambdaListModel {
	m.width = w
	m.height = h
	return m
}
