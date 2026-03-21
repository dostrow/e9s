package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type StandaloneTasksModel struct {
	tasks       []model.Task
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
}

func NewStandaloneTasks() StandaloneTasksModel {
	return StandaloneTasksModel{}
}

func (m StandaloneTasksModel) Update(msg tea.Msg) (StandaloneTasksModel, tea.Cmd) {
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
			filtered := m.filteredTasks()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter tasks..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 50
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m StandaloneTasksModel) View() string {
	filtered := m.filteredTasks()
	var b strings.Builder

	title := fmt.Sprintf("  Standalone Tasks (%d)", len(filtered))
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
		b.WriteString(theme.HelpStyle.Render("  No standalone tasks found"))
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "TASK ID"},
		{Title: "STATUS"},
		{Title: "HEALTH"},
		{Title: "GROUP"},
		{Title: "AGE", RightAlign: true},
		{Title: "IP"},
		{Title: "STOP REASON"},
	})

	for _, t := range filtered {
		id := t.TaskID
		if len(id) > 8 {
			id = id[:8]
		}
		tbl.AddRow(
			components.Plain(id),
			components.Styled(t.Status, theme.StatusStyle(t.Status)),
			components.Styled(t.HealthStatus, theme.HealthStyle(t.HealthStatus)),
			components.Plain(t.Group),
			components.Plain(formatAge(t.StartedAt)),
			components.Plain(t.PrivateIP),
			components.Plain(t.StoppedReason),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m StandaloneTasksModel) filteredTasks() []model.Task {
	if m.filter == "" {
		return m.tasks
	}
	lf := strings.ToLower(m.filter)
	var out []model.Task
	for _, t := range m.tasks {
		match := strings.Contains(strings.ToLower(t.TaskID), lf) ||
			strings.Contains(strings.ToLower(t.Status), lf) ||
			strings.Contains(strings.ToLower(t.Group), lf) ||
			strings.Contains(strings.ToLower(t.PrivateIP), lf)
		if match {
			out = append(out, t)
		}
	}
	return out
}

func (m StandaloneTasksModel) SetTasks(tasks []model.Task) StandaloneTasksModel {
	var standalone []model.Task
	for _, t := range tasks {
		if !strings.HasPrefix(t.Group, "service:") {
			standalone = append(standalone, t)
		}
	}
	m.tasks = standalone
	filtered := m.filteredTasks()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m StandaloneTasksModel) SelectedTask() *model.Task {
	filtered := m.filteredTasks()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	t := filtered[m.cursor]
	return &t
}

func (m StandaloneTasksModel) IsFiltering() bool {
	return m.filtering
}

func (m StandaloneTasksModel) visibleRows() int {
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

func (m StandaloneTasksModel) SetSize(w, h int) StandaloneTasksModel {
	m.width = w
	m.height = h
	return m
}
