package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type taskDefDetailTab int

const (
	taskDefSummaryTab taskDefDetailTab = iota
	taskDefJSONTab
)

type TaskDefDetailModel struct {
	taskDef *aws.TaskDefSummary
	tab     taskDefDetailTab
	scroll  int
	width   int
	height  int
}

func NewTaskDefDetail(taskDef *aws.TaskDefSummary) TaskDefDetailModel {
	return TaskDefDetailModel{taskDef: taskDef}
}

func (m TaskDefDetailModel) Update(msg tea.Msg) (TaskDefDetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if m.scroll > 0 {
				m.scroll--
			}
		case key.Matches(msg, theme.Keys.Down):
			m.scroll++
		case msg.String() == "pgup":
			m.scroll = max(0, m.scroll-15)
		case msg.String() == "pgdown":
			m.scroll += 15
		case msg.String() == "g":
			m.scroll = 0
		case msg.String() == "G":
			lines := m.activeLines()
			m.scroll = max(0, len(lines)-m.visibleLines())
		case msg.String() == "tab":
			if m.tab == taskDefSummaryTab {
				m.tab = taskDefJSONTab
			} else {
				m.tab = taskDefSummaryTab
			}
			m.scroll = 0
		}
	}
	return m, nil
}

func (m TaskDefDetailModel) View() string {
	if m.taskDef == nil {
		return theme.HelpStyle.Render("  No task definition selected")
	}

	var b strings.Builder
	title := fmt.Sprintf("  Task Definition: %s:%d", m.taskDef.Family, m.taskDef.Revision)
	b.WriteString(theme.TitleStyle.Render(title))
	b.WriteString("\n\n")

	summaryTab := theme.TitleStyle.Render("  [Summary]  ")
	jsonTab := theme.HelpStyle.Render("  [Raw JSON]  ")
	if m.tab == taskDefJSONTab {
		summaryTab = theme.HelpStyle.Render("  [Summary]  ")
		jsonTab = theme.TitleStyle.Render("  [Raw JSON]  ")
	}
	b.WriteString(summaryTab + jsonTab + "\n\n")

	lines := m.activeLines()
	visible := m.visibleLines()
	start := max(0, min(m.scroll, len(lines)-visible))
	end := min(start+visible, len(lines))
	for _, line := range lines[start:end] {
		b.WriteString(line + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m TaskDefDetailModel) activeLines() []string {
	if m.tab == taskDefJSONTab {
		return m.jsonLines()
	}
	return m.summaryLines()
}

func (m TaskDefDetailModel) summaryLines() []string {
	td := m.taskDef
	var lines []string

	lines = append(lines, fmt.Sprintf("  %-24s %s", "ARN:", td.ARN))
	lines = append(lines, fmt.Sprintf("  %-24s %s", "Family:", td.Family))
	lines = append(lines, fmt.Sprintf("  %-24s %d", "Revision:", td.Revision))
	lines = append(lines, fmt.Sprintf("  %-24s %s", "Status:", td.Status))
	lines = append(lines, fmt.Sprintf("  %-24s %s", "CPU:", td.CPU))
	lines = append(lines, fmt.Sprintf("  %-24s %s", "Memory:", td.Memory))
	lines = append(lines, fmt.Sprintf("  %-24s %s", "Network Mode:", td.NetworkMode))
	if len(td.RequiresCompatibilities) > 0 {
		lines = append(lines, fmt.Sprintf("  %-24s %s", "Compatibilities:", strings.Join(td.RequiresCompatibilities, ", ")))
	}
	if td.TaskRoleArn != "" {
		lines = append(lines, fmt.Sprintf("  %-24s %s", "Task Role:", td.TaskRoleArn))
	}
	if td.ExecutionRoleArn != "" {
		lines = append(lines, fmt.Sprintf("  %-24s %s", "Execution Role:", td.ExecutionRoleArn))
	}
	if !td.RegisteredAt.IsZero() {
		lines = append(lines, fmt.Sprintf("  %-24s %s (%s ago)", "Registered:", td.RegisteredAt.Format("2006-01-02 15:04:05"), formatAge(td.RegisteredAt)))
	}

	lines = append(lines, "")
	lines = append(lines, theme.TitleStyle.Render("  Containers"))
	lines = append(lines, "")

	tbl := components.NewTable([]components.Column{
		{Title: "NAME"},
		{Title: "IMAGE"},
		{Title: "CPU", RightAlign: true},
		{Title: "MEM", RightAlign: true},
		{Title: "ESSENTIAL"},
		{Title: "ENV", RightAlign: true},
	})
	for _, c := range td.Containers {
		tbl.AddRow(
			components.Plain(c.Name),
			components.Plain(c.Image),
			components.Plain(fmt.Sprintf("%d", c.CPU)),
			components.Plain(fmt.Sprintf("%d", c.Memory)),
			components.Plain(fmt.Sprintf("%t", c.Essential)),
			components.Plain(fmt.Sprintf("%d", len(c.EnvVars))),
		)
	}
	lines = append(lines, strings.Split(strings.TrimRight(tbl.Render(-1, "", 0), "\n"), "\n")...)

	for _, c := range td.Containers {
		if len(c.EnvVars) == 0 {
			continue
		}
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  %s env:", c.Name))
		for _, ev := range c.EnvVars {
			label := ev.Name
			if ev.Source != "" {
				label += " [" + ev.Source + "]"
			}
			lines = append(lines, "    "+label)
		}
	}

	return lines
}

func (m TaskDefDetailModel) jsonLines() []string {
	lines := strings.Split(m.taskDef.RawJSON, "\n")
	for i := range lines {
		lines[i] = "  " + lines[i]
	}
	return lines
}

func (m TaskDefDetailModel) visibleLines() int {
	h := m.height - 6
	if h < 1 {
		h = 20
	}
	return h
}

func (m TaskDefDetailModel) SetSize(w, h int) TaskDefDetailModel {
	m.width = w
	m.height = h
	return m
}

func (m TaskDefDetailModel) TaskDef() *aws.TaskDefSummary {
	return m.taskDef
}
