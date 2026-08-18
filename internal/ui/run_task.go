package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/theme"
)

const runTaskFieldCount = 8

type RunTaskSubmitMsg struct{ Request model.RunTaskRequest }
type RunTaskCancelMsg struct{}

type RunTaskFormModel struct {
	Active         bool
	cluster        string
	focus          int
	taskDefinition textinput.Model
	count          textinput.Model
	subnets        textinput.Model
	securityGroups textinput.Model
	group          textinput.Model
	launchTypes    []string
	launchType     int
	assignPublicIP bool
	enableExec     bool
	err            string
}

func NewRunTaskForm(cluster, taskDefinition string) RunTaskFormModel {
	input := func(value, placeholder string) textinput.Model {
		field := textinput.New()
		field.Width = 48
		field.CharLimit = 500
		field.Placeholder = placeholder
		field.SetValue(value)
		return field
	}
	form := RunTaskFormModel{
		Active:         true,
		cluster:        cluster,
		taskDefinition: input(taskDefinition, "family:revision or ARN"),
		count:          input("1", "number of tasks"),
		subnets:        input("", "subnet-a, subnet-b"),
		securityGroups: input("", "sg-a, sg-b"),
		group:          input("", "optional task group"),
		launchTypes:    []string{"Cluster default", "FARGATE", "EC2", "EXTERNAL", "MANAGED_INSTANCES"},
	}
	form.focusField(0)
	return form
}

func (m RunTaskFormModel) Update(msg tea.Msg) (RunTaskFormModel, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok || !m.Active {
		return m, nil
	}
	switch key.String() {
	case "esc":
		m.Active = false
		return m, func() tea.Msg { return RunTaskCancelMsg{} }
	case "ctrl+s":
		request, err := m.request()
		if err != nil {
			m.err = err.Error()
			return m, nil
		}
		m.Active = false
		return m, func() tea.Msg { return RunTaskSubmitMsg{Request: request} }
	case "tab", "down":
		m.focusField((m.focus + 1) % runTaskFieldCount)
		return m, nil
	case "shift+tab", "up":
		m.focusField((m.focus + runTaskFieldCount - 1) % runTaskFieldCount)
		return m, nil
	case "left":
		if m.focus == 1 {
			m.launchType = (m.launchType + len(m.launchTypes) - 1) % len(m.launchTypes)
			return m, nil
		}
	case "right", " ":
		switch m.focus {
		case 1:
			m.launchType = (m.launchType + 1) % len(m.launchTypes)
			return m, nil
		case 6:
			m.assignPublicIP = !m.assignPublicIP
			return m, nil
		case 7:
			m.enableExec = !m.enableExec
			return m, nil
		}
	}

	field := m.focusedInput()
	if field == nil {
		return m, nil
	}
	updated, cmd := field.Update(msg)
	m.setFocusedInput(updated)
	m.err = ""
	return m, cmd
}

func (m RunTaskFormModel) View() string {
	if !m.Active {
		return ""
	}
	label := func(index int, text string) string {
		style := theme.HelpStyle
		if m.focus == index {
			style = theme.TitleStyle
		}
		return style.Render(text)
	}
	check := func(value bool) string {
		if value {
			return "[x]"
		}
		return "[ ]"
	}
	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render("Run standalone ECS task") + "\n\n")
	fmt.Fprintf(&b, "%s\n  %s\n", label(0, "Task definition"), m.taskDefinition.View())
	fmt.Fprintf(&b, "%s\n  <%s>\n", label(1, "Launch type"), m.launchTypes[m.launchType])
	fmt.Fprintf(&b, "%s\n  %s\n", label(2, "Count"), m.count.View())
	fmt.Fprintf(&b, "%s\n  %s\n", label(3, "Subnets (required for awsvpc/Fargate)"), m.subnets.View())
	fmt.Fprintf(&b, "%s\n  %s\n", label(4, "Security groups"), m.securityGroups.View())
	fmt.Fprintf(&b, "%s\n  %s\n", label(5, "Task group"), m.group.View())
	fmt.Fprintf(&b, "%s  %s\n", label(6, check(m.assignPublicIP)), "Assign public IP")
	fmt.Fprintf(&b, "%s  %s\n", label(7, check(m.enableExec)), "Enable ECS Exec")
	if m.err != "" {
		b.WriteString("\n" + theme.ErrorStyle.Render(m.err) + "\n")
	}
	b.WriteString("\n" + theme.HelpStyle.Render("Tab/Shift+Tab move  Space/←/→ change options  Ctrl+S run  Esc cancel"))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorCyan).
		Padding(1, 3).
		Width(68).
		Render(b.String())
}

func (m *RunTaskFormModel) focusField(index int) {
	m.focus = index
	for _, field := range []*textinput.Model{&m.taskDefinition, &m.count, &m.subnets, &m.securityGroups, &m.group} {
		field.Blur()
	}
	if field := m.focusedInput(); field != nil {
		field.Focus()
	}
}

func (m *RunTaskFormModel) focusedInput() *textinput.Model {
	switch m.focus {
	case 0:
		return &m.taskDefinition
	case 2:
		return &m.count
	case 3:
		return &m.subnets
	case 4:
		return &m.securityGroups
	case 5:
		return &m.group
	default:
		return nil
	}
}

func (m *RunTaskFormModel) setFocusedInput(field textinput.Model) {
	switch m.focus {
	case 0:
		m.taskDefinition = field
	case 2:
		m.count = field
	case 3:
		m.subnets = field
	case 4:
		m.securityGroups = field
	case 5:
		m.group = field
	}
}

func (m RunTaskFormModel) request() (model.RunTaskRequest, error) {
	taskDefinition := strings.TrimSpace(m.taskDefinition.Value())
	if taskDefinition == "" {
		return model.RunTaskRequest{}, fmt.Errorf("task definition is required")
	}
	count, err := strconv.Atoi(strings.TrimSpace(m.count.Value()))
	if err != nil || count < 1 {
		return model.RunTaskRequest{}, fmt.Errorf("count must be a positive integer")
	}
	launchType := ""
	if m.launchType > 0 {
		launchType = m.launchTypes[m.launchType]
	}
	return model.RunTaskRequest{
		Cluster:              m.cluster,
		TaskDefinition:       taskDefinition,
		LaunchType:           launchType,
		Count:                count,
		Subnets:              splitRunTaskList(m.subnets.Value()),
		SecurityGroups:       splitRunTaskList(m.securityGroups.Value()),
		AssignPublicIP:       m.assignPublicIP,
		EnableExecuteCommand: m.enableExec,
		Group:                strings.TrimSpace(m.group.Value()),
	}, nil
}

func splitRunTaskList(value string) []string {
	var result []string
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}
