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

type TaskListModel struct {
	tasks       []model.Task
	serviceName string
	stopped     bool
	hasMore     bool
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewTaskList(serviceName string) TaskListModel {
	return TaskListModel{serviceName: serviceName}
}

func (m TaskListModel) Init() tea.Cmd {
	return nil
}

func (m TaskListModel) Update(msg tea.Msg) (TaskListModel, tea.Cmd) {
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
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredTasks()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter tasks..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m TaskListModel) View() string {
	filtered := m.filteredTasks()
	var b strings.Builder

	title := fmt.Sprintf("  Tasks — %s (%d)", taskScopeLabel(m.stopped), len(filtered))
	b.WriteString(theme.TitleStyle.Render(title))
	if m.filter != "" {
		b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("  filter: %q", m.filter)))
	}
	b.WriteString("\n")
	b.WriteString(renderTaskScopeTabs(m.stopped, m.hasMore))
	b.WriteString("\n")

	if m.filtering {
		b.WriteString("  / " + m.filterInput.View() + "\n")
	}
	b.WriteString("\n")

	if len(filtered) == 0 {
		if !m.loaded {
			b.WriteString(theme.HelpStyle.Render("  Loading..."))
		} else {
			b.WriteString(theme.HelpStyle.Render("  No tasks found"))
		}
		return b.String()
	}

	if m.stopped {
		b.WriteString(renderStoppedTaskTable(filtered, m.cursor, m.visibleRows()))
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "TASK ID"},
		{Title: "STATUS"},
		{Title: "HEALTH"},
		{Title: "AZ"},
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
			components.Plain(t.AvailabilityZone),
			components.Plain(formatAge(t.StartedAt)),
			components.Plain(t.PrivateIP),
			components.Plain(t.StoppedReason),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m TaskListModel) filteredTasks() []model.Task {
	if m.filter == "" {
		return m.tasks
	}
	lf := strings.ToLower(m.filter)
	var out []model.Task
	for _, t := range m.tasks {
		match := strings.Contains(strings.ToLower(t.TaskID), lf) ||
			strings.Contains(strings.ToLower(t.Status), lf) ||
			strings.Contains(strings.ToLower(t.AvailabilityZone), lf) ||
			strings.Contains(strings.ToLower(t.PrivateIP), lf) ||
			strings.Contains(strings.ToLower(t.StopCode), lf) ||
			strings.Contains(strings.ToLower(t.TaskDefinition), lf) ||
			strings.Contains(strings.ToLower(t.StoppedReason), lf)
		if match {
			out = append(out, t)
		}
	}
	return out
}

func (m TaskListModel) SetTasks(tasks []model.Task) TaskListModel {
	m.tasks = tasks
	m.loaded = true
	filtered := m.filteredTasks()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m TaskListModel) AppendTasks(tasks []model.Task) TaskListModel {
	m.tasks = appendUniqueTasks(m.tasks, tasks)
	m.loaded = true
	return m
}

func (m TaskListModel) SetScope(stopped bool) TaskListModel {
	m.stopped = stopped
	m.tasks = nil
	m.cursor = 0
	m.loaded = false
	m.hasMore = false
	return m
}

func (m TaskListModel) SetHasMore(hasMore bool) TaskListModel {
	m.hasMore = hasMore
	return m
}

func taskScopeLabel(stopped bool) string {
	if stopped {
		return "Recently stopped"
	}
	return "Active"
}

func renderTaskScopeTabs(stopped, hasMore bool) string {
	active := theme.TitleStyle.Render("[Active]")
	history := theme.HelpStyle.Render("[Recently stopped]")
	if stopped {
		active = theme.HelpStyle.Render("[Active]")
		history = theme.TitleStyle.Render("[Recently stopped]")
	}
	line := "  " + active + "  " + history + theme.HelpStyle.Render("  Tab switches scope")
	if stopped && hasMore {
		line += theme.HelpStyle.Render("  ] loads 50 more")
	}
	return line
}

func renderStoppedTaskTable(tasks []model.Task, cursor, visibleRows int) string {
	tbl := components.NewTable([]components.Column{
		{Title: "TASK ID"},
		{Title: "STOPPED"},
		{Title: "EXIT"},
		{Title: "STOP CODE"},
		{Title: "TASK DEFINITION"},
		{Title: "REASON"},
	})
	for _, task := range tasks {
		id := task.TaskID
		if len(id) > 8 {
			id = id[:8]
		}
		stopped := "—"
		if !task.StoppedAt.IsZero() {
			stopped = formatAge(task.StoppedAt) + " ago"
		}
		tbl.AddRow(
			components.Plain(id),
			components.Plain(stopped),
			components.Plain(taskExitSummary(task)),
			components.Plain(valueOrDash(task.StopCode)),
			components.Plain(valueOrDash(task.TaskDefinition)),
			components.Plain(valueOrDash(task.StoppedReason)),
		)
	}
	return tbl.Render(cursor, "", visibleRows)
}

func taskExitSummary(task model.Task) string {
	var exits []string
	for _, container := range task.Containers {
		if container.ExitCode == nil {
			continue
		}
		value := fmt.Sprintf("%d", *container.ExitCode)
		if len(task.Containers) > 1 {
			value = container.Name + ":" + value
		}
		exits = append(exits, value)
	}
	if len(exits) == 0 {
		return "—"
	}
	return strings.Join(exits, ", ")
}

func appendUniqueTasks(existing, additions []model.Task) []model.Task {
	seen := make(map[string]struct{}, len(existing)+len(additions))
	result := append([]model.Task(nil), existing...)
	for _, task := range existing {
		seen[taskIdentity(task)] = struct{}{}
	}
	for _, task := range additions {
		identity := taskIdentity(task)
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		result = append(result, task)
	}
	return result
}

func taskIdentity(task model.Task) string {
	if task.TaskARN != "" {
		return task.TaskARN
	}
	return task.TaskID
}

func valueOrDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func (m TaskListModel) SelectedTask() *model.Task {
	filtered := m.filteredTasks()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	t := filtered[m.cursor]
	return &t
}

func (m TaskListModel) IsFiltering() bool {
	return m.filtering
}

func (m TaskListModel) visibleRows() int {
	overhead := 10
	if m.filtering {
		overhead++
	}
	rows := m.height - overhead
	if rows < 5 {
		return 0
	}
	return rows
}

func (m TaskListModel) SetSize(w, h int) TaskListModel {
	m.width = w
	m.height = h
	return m
}
