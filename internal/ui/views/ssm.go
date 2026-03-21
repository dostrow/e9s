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

type SSMModel struct {
	params      []aws.Parameter
	pathPrefix  string
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
}

func NewSSM(pathPrefix string) SSMModel {
	return SSMModel{pathPrefix: pathPrefix}
}

func (m SSMModel) Update(msg tea.Msg) (SSMModel, tea.Cmd) {
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
			filtered := m.filteredParams()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter parameters..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 50
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m SSMModel) View() string {
	filtered := m.filteredParams()
	var b strings.Builder

	title := fmt.Sprintf("  SSM Parameters (%d)", len(filtered))
	if m.pathPrefix != "" {
		title += fmt.Sprintf(" — prefix: %s", m.pathPrefix)
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
		b.WriteString(theme.HelpStyle.Render("  No parameters found"))
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "NAME"},
		{Title: "TYPE"},
		{Title: "VERSION", RightAlign: true},
		{Title: "VALUE"},
		{Title: "MODIFIED"},
	})

	for _, p := range filtered {
		value := p.Value
		if p.Type == "SecureString" {
			value = "********"
		}
		if len(value) > 40 {
			value = value[:40] + ".."
		}

		modified := ""
		if !p.LastModified.IsZero() {
			modified = formatAge(p.LastModified) + " ago"
		}

		tbl.AddRow(
			components.Plain(p.Name),
			components.Styled(p.Type, ssmTypeStyle(p.Type)),
			components.Plain(fmt.Sprintf("%d", p.Version)),
			components.Plain(value),
			components.Plain(modified),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func ssmTypeStyle(t string) lipgloss.Style {
	if t == "SecureString" {
		return lipgloss.NewStyle().Foreground(theme.ColorYellow)
	}
	return lipgloss.NewStyle().Foreground(theme.ColorCyan)
}

func (m SSMModel) filteredParams() []aws.Parameter {
	if m.filter == "" {
		return m.params
	}
	lf := strings.ToLower(m.filter)
	var out []aws.Parameter
	for _, p := range m.params {
		if strings.Contains(strings.ToLower(p.Name), lf) ||
			strings.Contains(strings.ToLower(p.Value), lf) {
			out = append(out, p)
		}
	}
	return out
}

func (m SSMModel) SetParams(params []aws.Parameter) SSMModel {
	m.params = params
	filtered := m.filteredParams()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m SSMModel) SelectedParam() *aws.Parameter {
	filtered := m.filteredParams()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	p := filtered[m.cursor]
	return &p
}

func (m SSMModel) IsFiltering() bool {
	return m.filtering
}

func (m SSMModel) visibleRows() int {
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

func (m SSMModel) PathPrefix() string {
	return m.pathPrefix
}

func (m SSMModel) SetSize(w, h int) SSMModel {
	m.width = w
	m.height = h
	return m
}
