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

type EnvVarsModel struct {
	title       string
	envVars     []aws.EnvVar
	cursor      int
	showARNs    bool // false = show resolved values, true = show ARNs
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
}

func NewEnvVars(title string, envVars []aws.EnvVar) EnvVarsModel {
	return EnvVarsModel{
		title:   title,
		envVars: envVars,
	}
}

func (m EnvVarsModel) Update(msg tea.Msg) (EnvVarsModel, tea.Cmd) {
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
			filtered := m.filteredVars()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredVars()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter env vars..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 50
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		case msg.String() == "a":
			m.showARNs = !m.showARNs
		}
	}
	return m, nil
}

func (m EnvVarsModel) View() string {
	filtered := m.filteredVars()
	var b strings.Builder

	title := fmt.Sprintf("  Environment: %s (%d vars)", m.title, len(filtered))
	b.WriteString(theme.TitleStyle.Render(title))
	if m.filter != "" {
		b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("  filter: %q", m.filter)))
	}
	if m.showARNs {
		b.WriteString(theme.HelpStyle.Render("  [showing ARNs]"))
	}
	b.WriteString("\n")

	if m.filtering {
		b.WriteString("  / " + m.filterInput.View() + "\n")
	}
	b.WriteString("\n")

	if len(filtered) == 0 {
		b.WriteString(theme.HelpStyle.Render("  No environment variables found"))
		return b.String()
	}

	hasSecrets := false
	for _, ev := range filtered {
		if ev.Source != "" {
			hasSecrets = true
			break
		}
	}

	var cols []components.Column
	if hasSecrets {
		cols = []components.Column{
			{Title: "NAME"},
			{Title: "SOURCE"},
			{Title: "VALUE"},
		}
	} else {
		cols = []components.Column{
			{Title: "NAME"},
			{Title: "VALUE"},
		}
	}
	tbl := components.NewTable(cols)

	for _, ev := range filtered {
		displayValue := m.displayValue(ev)

		if hasSecrets {
			sourceCell := components.Plain("env")
			if ev.Source != "" {
				sourceCell = components.Styled(ev.Source, theme.HealthStyle("deploying"))
			}
			tbl.AddRow(
				components.Plain(ev.Name),
				sourceCell,
				components.Plain(displayValue),
			)
		} else {
			tbl.AddRow(
				components.Plain(ev.Name),
				components.Plain(displayValue),
			)
		}
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m EnvVarsModel) displayValue(ev aws.EnvVar) string {
	if ev.Source == "" {
		// Plain env var — always show value
		return ev.Value
	}
	if m.showARNs {
		return ev.Value // the ARN
	}
	if ev.ResolvedValue != "" {
		return ev.ResolvedValue
	}
	return ev.Value // fallback to ARN if resolve failed
}

func (m EnvVarsModel) filteredVars() []aws.EnvVar {
	if m.filter == "" {
		return m.envVars
	}
	lf := strings.ToLower(m.filter)
	var out []aws.EnvVar
	for _, ev := range m.envVars {
		if strings.Contains(strings.ToLower(ev.Name), lf) ||
			strings.Contains(strings.ToLower(m.displayValue(ev)), lf) {
			out = append(out, ev)
		}
	}
	return out
}

func (m EnvVarsModel) visibleRows() int {
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

func (m EnvVarsModel) IsFiltering() bool {
	return m.filtering
}

func (m EnvVarsModel) SetSize(w, h int) EnvVarsModel {
	m.width = w
	m.height = h
	return m
}
