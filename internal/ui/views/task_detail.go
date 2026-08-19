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
	task                   *model.Task
	resources              []model.ResourceRef
	parentService          string
	configuredTargetGroups []model.ResourceRef
	scroll                 int
	width                  int
	height                 int
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
	lines = append(lines, fmt.Sprintf("  %-20s %s", "Network Interface:", t.NetworkInterfaceID))
	lines = append(lines, fmt.Sprintf("  %-20s %s", "Subnet:", t.SubnetID))
	lines = append(lines, fmt.Sprintf("  %-20s %s", "VPC:", t.VpcID))
	lines = append(lines, fmt.Sprintf("  %-20s %s", "EC2 Instance:", t.EC2InstanceID))
	if len(t.SecurityGroups) > 0 {
		groups := make([]string, 0, len(t.SecurityGroups))
		for _, group := range t.SecurityGroups {
			groups = append(groups, group.ID)
		}
		lines = append(lines, fmt.Sprintf("  %-20s %s", "Security Groups:", strings.Join(groups, ", ")))
	}
	if len(t.VolumeIDs) > 0 {
		lines = append(lines, fmt.Sprintf("  %-20s %s", "EBS Volumes:", strings.Join(t.VolumeIDs, ", ")))
	}
	lines = append(lines, fmt.Sprintf("  %-20s %s", "Group:", t.Group))
	if m.parentService != "" {
		lines = append(lines, "", theme.TitleStyle.Render("  Service Context"), "")
		lines = append(lines, fmt.Sprintf("  %-26s %s", "Parent Service:", m.parentService))
		if len(m.configuredTargetGroups) == 0 {
			lines = append(lines, fmt.Sprintf("  %-26s %s", "Configured Target Groups:", "none returned by ECS"))
		}
		for index, targetGroup := range m.configuredTargetGroups {
			label := ""
			if index == 0 {
				label = "Configured Target Groups:"
			}
			lines = append(lines, fmt.Sprintf("  %-26s %s", label, targetGroup.ID))
		}
	}

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
	for _, warning := range t.ResourceWarnings {
		lines = append(lines, fmt.Sprintf("  %-20s %s", "Resource Warning:", warning))
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
	if len(m.resources) > 0 {
		lines = append(lines, "", theme.TitleStyle.Render("  Linked Resources"), "")
		for _, ref := range m.resources {
			lines = append(lines, fmt.Sprintf("  %-22s %s", strings.ReplaceAll(ref.Kind, "-", " ")+":", ref.ID))
		}
		lines = append(lines, "", "  [o] open linked resource")
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

func (m TaskDetailModel) Task() *model.Task { return m.task }

func (m TaskDetailModel) SetResourceRefs(refs []model.ResourceRef) TaskDetailModel {
	m.resources = append([]model.ResourceRef(nil), refs...)
	return m
}

func (m TaskDetailModel) SetServiceContext(service *model.Service) TaskDetailModel {
	if service == nil {
		m.parentService = ""
		m.configuredTargetGroups = nil
		return m
	}
	m.parentService = service.Name
	m.configuredTargetGroups = append([]model.ResourceRef(nil), service.TargetGroups...)
	return m
}
