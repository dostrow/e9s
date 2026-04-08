package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type ServiceListModel struct {
	services    []model.Service
	clusterName string
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewServiceList(clusterName string) ServiceListModel {
	return ServiceListModel{clusterName: clusterName}
}

func (m ServiceListModel) Init() tea.Cmd {
	return nil
}

func (m ServiceListModel) Update(msg tea.Msg) (ServiceListModel, tea.Cmd) {
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
			filtered := m.filteredServices()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredServices()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter services..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m ServiceListModel) View() string {
	filtered := m.filteredServices()
	var b strings.Builder

	title := fmt.Sprintf("  Services (%d)", len(filtered))
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
			b.WriteString(theme.HelpStyle.Render("  No services found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "NAME"},
		{Title: "STATUS"},
		{Title: "DESIRED", RightAlign: true},
		{Title: "RUNNING", RightAlign: true},
		{Title: "PENDING", RightAlign: true},
		{Title: "TASK DEF"},
		{Title: "AGE", RightAlign: true},
	})

	for _, s := range filtered {
		tbl.AddRow(
			components.Plain(s.Name),
			components.Styled(s.Status, theme.HealthStyle(s.HealthStatus)),
			components.Plain(fmt.Sprintf("%d", s.DesiredCount)),
			components.Plain(fmt.Sprintf("%d", s.RunningCount)),
			components.Plain(fmt.Sprintf("%d", s.PendingCount)),
			components.Plain(s.TaskDefinition),
			components.Plain(formatAge(s.CreatedAt)),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m ServiceListModel) filteredServices() []model.Service {
	if m.filter == "" {
		return m.services
	}
	lf := strings.ToLower(m.filter)
	var out []model.Service
	for _, s := range m.services {
		if strings.Contains(strings.ToLower(s.Name), lf) {
			out = append(out, s)
		}
	}
	return out
}

func (m ServiceListModel) SetServices(services []model.Service) ServiceListModel {
	m.services = services
	m.loaded = true
	filtered := m.filteredServices()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m ServiceListModel) SelectedService() *model.Service {
	filtered := m.filteredServices()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	s := filtered[m.cursor]
	return &s
}

func (m ServiceListModel) IsFiltering() bool {
	return m.filtering
}

func (m ServiceListModel) visibleRows() int {
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

func (m ServiceListModel) SetSize(w, h int) ServiceListModel {
	m.width = w
	m.height = h
	return m
}

func formatAge(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
