package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type TaskDetailModel struct {
	task   *model.Task
	scroll int
	width  int
	height int
}

func NewTaskDetail(task *model.Task) TaskDetailModel {
	return TaskDetailModel{task: task}
}

func (m TaskDetailModel) Update(msg tea.Msg) (TaskDetailModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, theme.Keys.Up):
		m.scroll = max(0, m.scroll-1)
	case key.Matches(keyMsg, theme.Keys.Down):
		m.scroll++
	case keyMsg.String() == "pgup":
		m.scroll = max(0, m.scroll-m.visibleLines())
	case keyMsg.String() == "pgdown":
		m.scroll += m.visibleLines()
	case keyMsg.String() == "g":
		m.scroll = 0
	case keyMsg.String() == "G":
		m.scroll = max(0, len(m.lines())-m.visibleLines())
	}
	return m, nil
}

func (m TaskDetailModel) View() string {
	if m.task == nil {
		return theme.HelpStyle.Render("  No task selected")
	}

	lines := m.lines()
	visible := m.visibleLines()
	start := max(0, min(m.scroll, max(0, len(lines)-visible)))
	end := min(start+visible, len(lines))
	return strings.Join(lines[start:end], "\n")
}

func (m TaskDetailModel) lines() []string {
	t := m.task
	var lines []string

	lines = append(lines, theme.TitleStyle.Render(fmt.Sprintf("  Task: %s", t.TaskID)))
	lines = append(lines, "")

	lines = append(lines, fmt.Sprintf("  %-20s %s", "Status:", theme.StatusStyle(t.Status).Render(t.Status)))
	lines = append(lines, fmt.Sprintf("  %-20s %s", "Health:", theme.HealthStyle(t.HealthStatus).Render(t.HealthStatus)))
	lines = append(lines, fmt.Sprintf("  %-20s %s", "Desired Status:", t.DesiredStatus))
	lines = append(lines, fmt.Sprintf("  %-20s %s", "Task Definition:", t.TaskDefinition))
	lines = append(lines, fmt.Sprintf("  %-20s %s", "Launch Type:", t.LaunchType))
	lines = append(lines, fmt.Sprintf("  %-20s %s", "Private IP:", t.PrivateIP))
	lines = append(lines, fmt.Sprintf("  %-20s %s", "Group:", t.Group))

	if !t.StartedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("  %-20s %s (%s ago)", "Started:", t.StartedAt.Format("2006-01-02 15:04:05"), formatAge(t.StartedAt)))
	}
	if !t.StoppedAt.IsZero() {
		lines = append(lines, fmt.Sprintf("  %-20s %s", "Stopped:", t.StoppedAt.Format("2006-01-02 15:04:05")))
	}
	if t.StopCode != "" {
		lines = append(lines, fmt.Sprintf("  %-20s %s", "Stop Code:", t.StopCode))
	}
	if t.StoppedReason != "" {
		lines = append(lines, fmt.Sprintf("  %-20s %s", "Stop Reason:", t.StoppedReason))
	}

	if len(t.Containers) > 0 {
		lines = append(lines, "")
		lines = append(lines, theme.TitleStyle.Render("  Containers"))
		lines = append(lines, "")

		tbl := components.NewTable([]components.Column{
			{Title: "NAME"},
			{Title: "STATUS"},
			{Title: "HEALTH"},
			{Title: "IMAGE"},
		})

		for _, c := range t.Containers {
			tbl.AddRow(
				components.Plain(c.Name),
				components.Styled(c.Status, theme.StatusStyle(c.Status)),
				components.Styled(c.HealthStatus, theme.HealthStyle(c.HealthStatus)),
				components.Plain(c.Image),
			)
		}

		tableStr := tbl.Render(-1, "", 0)
		lines = append(lines, strings.Split(strings.TrimRight(tableStr, "\n"), "\n")...)

		for _, c := range t.Containers {
			if c.ExitCode != nil || c.Reason != "" {
				detail := fmt.Sprintf("  %s:", c.Name)
				if c.ExitCode != nil {
					detail += fmt.Sprintf(" exit code %d", *c.ExitCode)
				}
				if c.Reason != "" {
					detail += fmt.Sprintf(" — %s", c.Reason)
				}
				lines = append(lines, "")
				lines = append(lines, detail)
			}
		}
	}

	return lines
}

func (m TaskDetailModel) visibleLines() int {
	lines := m.height - 2
	if lines < 1 {
		return 20
	}
	return lines
}

func (m TaskDetailModel) SetSize(w, h int) TaskDetailModel {
	m.width = w
	m.height = h
	return m
}
