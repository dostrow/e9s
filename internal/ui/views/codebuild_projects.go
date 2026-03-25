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

type CBProjectsModel struct {
	projects    []aws.CBProject
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewCBProjects() CBProjectsModel {
	return CBProjectsModel{}
}

func (m CBProjectsModel) Update(msg tea.Msg) (CBProjectsModel, tea.Cmd) {
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
			filtered := m.filteredProjects()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredProjects()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter projects..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 80
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m CBProjectsModel) View() string {
	filtered := m.filteredProjects()
	var b strings.Builder

	title := fmt.Sprintf("  CodeBuild Projects (%d)", len(filtered))
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
			b.WriteString(theme.HelpStyle.Render("  No projects found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "PROJECT"},
		{Title: "SOURCE"},
		{Title: "DESCRIPTION"},
	})
	for _, p := range filtered {
		desc := p.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		tbl.AddRow(
			components.Plain(p.Name),
			components.Plain(p.Source),
			components.Plain(desc),
		)
	}
	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m CBProjectsModel) filteredProjects() []aws.CBProject {
	if m.filter == "" {
		return m.projects
	}
	lf := strings.ToLower(m.filter)
	var out []aws.CBProject
	for _, p := range m.projects {
		if strings.Contains(strings.ToLower(p.Name), lf) ||
			strings.Contains(strings.ToLower(p.Description), lf) {
			out = append(out, p)
		}
	}
	return out
}

func (m CBProjectsModel) SetProjects(projects []aws.CBProject) CBProjectsModel {
	m.projects = projects
	m.loaded = true
	filtered := m.filteredProjects()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m CBProjectsModel) SelectedProject() *aws.CBProject {
	filtered := m.filteredProjects()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	p := filtered[m.cursor]
	return &p
}

func (m CBProjectsModel) IsFiltering() bool { return m.filtering }

func (m CBProjectsModel) visibleRows() int {
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

func (m CBProjectsModel) SetSize(w, h int) CBProjectsModel {
	m.width = w
	m.height = h
	return m
}
