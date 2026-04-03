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

type SecretsModel struct {
	secrets     []aws.Secret
	nameFilter  string
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewSecrets(nameFilter string) SecretsModel {
	return SecretsModel{nameFilter: nameFilter}
}

func (m SecretsModel) Update(msg tea.Msg) (SecretsModel, tea.Cmd) {
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
			filtered := m.filteredSecrets()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredSecrets()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter secrets..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 50
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m SecretsModel) View() string {
	filtered := m.filteredSecrets()
	var b strings.Builder

	title := fmt.Sprintf("  Secrets Manager (%d)", len(filtered))
	if m.nameFilter != "" {
		title += fmt.Sprintf(" — filter: %s", m.nameFilter)
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
			b.WriteString(theme.HelpStyle.Render("  No secrets found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "NAME"},
		{Title: "DESCRIPTION"},
		{Title: "LAST CHANGED"},
	})

	for _, s := range filtered {
		desc := s.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}

		changed := ""
		if !s.LastChanged.IsZero() {
			changed = formatAge(s.LastChanged) + " ago"
		}

		tbl.AddRow(
			components.Plain(s.Name),
			components.Plain(desc),
			components.Plain(changed),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m SecretsModel) filteredSecrets() []aws.Secret {
	if m.filter == "" {
		return m.secrets
	}
	lf := strings.ToLower(m.filter)
	var out []aws.Secret
	for _, s := range m.secrets {
		if strings.Contains(strings.ToLower(s.Name), lf) ||
			strings.Contains(strings.ToLower(s.Description), lf) {
			out = append(out, s)
		}
	}
	return out
}

func (m SecretsModel) SetSecrets(secrets []aws.Secret) SecretsModel {
	m.secrets = secrets
	m.loaded = true
	filtered := m.filteredSecrets()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m SecretsModel) SelectedSecret() *aws.Secret {
	filtered := m.filteredSecrets()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	s := filtered[m.cursor]
	return &s
}

func (m SecretsModel) IsFiltering() bool {
	return m.filtering
}

func (m SecretsModel) visibleRows() int {
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

func (m SecretsModel) NameFilter() string {
	return m.nameFilter
}

func (m SecretsModel) SetSize(w, h int) SecretsModel {
	m.width = w
	m.height = h
	return m
}
