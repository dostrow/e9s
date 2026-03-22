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

type serviceDetailTab int

const (
	tabDeployments serviceDetailTab = iota
	tabEvents
)

type ServiceDetailModel struct {
	service *model.Service
	tab     serviceDetailTab
	scroll  int
	width   int
	height  int
}

func NewServiceDetail(service *model.Service) ServiceDetailModel {
	return ServiceDetailModel{service: service}
}

func (m ServiceDetailModel) Init() tea.Cmd {
	return nil
}

func (m ServiceDetailModel) Update(msg tea.Msg) (ServiceDetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if m.scroll > 0 {
				m.scroll--
			}
		case key.Matches(msg, theme.Keys.Down):
			m.scroll++
		case msg.String() == "tab":
			if m.tab == tabDeployments {
				m.tab = tabEvents
			} else {
				m.tab = tabDeployments
			}
			m.scroll = 0
		}
	}
	return m, nil
}

func (m ServiceDetailModel) View() string {
	if m.service == nil {
		return theme.HelpStyle.Render("  No service selected")
	}

	s := m.service
	var b strings.Builder

	title := fmt.Sprintf("  Service: %s", s.Name)
	b.WriteString(theme.TitleStyle.Render(title))
	b.WriteString("\n\n")

	fmt.Fprintf(&b, "  %-20s %s\n", "Status:", theme.StatusStyle(s.Status).Render(s.Status))
	fmt.Fprintf(&b, "  %-20s %s\n", "Health:", theme.HealthStyle(s.HealthStatus).Render(s.HealthStatus))
	fmt.Fprintf(&b, "  %-20s %d / %d (pending: %d)\n", "Tasks:", s.RunningCount, s.DesiredCount, s.PendingCount)
	fmt.Fprintf(&b, "  %-20s %s\n", "Task Definition:", s.TaskDefinition)
	fmt.Fprintf(&b, "  %-20s %s\n", "Launch Type:", s.LaunchType)
	if !s.CreatedAt.IsZero() {
		fmt.Fprintf(&b, "  %-20s %s (%s ago)\n", "Created:", s.CreatedAt.Format("2006-01-02 15:04:05"), formatAge(s.CreatedAt))
	}
	b.WriteString("\n")

	deployTab := theme.TitleStyle.Render("  [Deployments]  ")
	eventTab := theme.HelpStyle.Render("  [Events]  ")
	if m.tab == tabEvents {
		deployTab = theme.HelpStyle.Render("  [Deployments]  ")
		eventTab = theme.TitleStyle.Render("  [Events]  ")
	}
	b.WriteString(deployTab + eventTab + "\n\n")

	switch m.tab {
	case tabDeployments:
		b.WriteString(m.renderDeployments())
	case tabEvents:
		b.WriteString(m.renderEvents())
	}

	return b.String()
}

func (m ServiceDetailModel) renderDeployments() string {
	if len(m.service.Deployments) == 0 {
		return theme.HelpStyle.Render("  No deployments")
	}

	tbl := components.NewTable([]components.Column{
		{Title: "ID"},
		{Title: "STATUS"},
		{Title: "DESIRED", RightAlign: true},
		{Title: "RUNNING", RightAlign: true},
		{Title: "PENDING", RightAlign: true},
		{Title: "FAILED", RightAlign: true},
		{Title: "TASK DEF"},
		{Title: "ROLLOUT"},
		{Title: "AGE", RightAlign: true},
	})

	for _, d := range m.service.Deployments {
		id := d.ID
		if len(id) > 10 {
			id = id[:10]
		}
		tbl.AddRow(
			components.Plain(id),
			components.Styled(d.Status, theme.StatusStyle(d.Status)),
			components.Plain(fmt.Sprintf("%d", d.DesiredCount)),
			components.Plain(fmt.Sprintf("%d", d.RunningCount)),
			components.Plain(fmt.Sprintf("%d", d.PendingCount)),
			components.Plain(fmt.Sprintf("%d", d.FailedCount)),
			components.Plain(d.TaskDefinition),
			components.Styled(d.RolloutState, theme.HealthStyle(rolloutToHealth(d.RolloutState))),
			components.Plain(formatAge(d.CreatedAt)),
		)
	}

	return tbl.Render(-1, "", 0)
}

func (m ServiceDetailModel) renderEvents() string {
	if len(m.service.Events) == 0 {
		return theme.HelpStyle.Render("  No events")
	}

	var b strings.Builder

	maxEvents := 50
	if len(m.service.Events) < maxEvents {
		maxEvents = len(m.service.Events)
	}

	start := m.scroll
	if start >= maxEvents {
		start = maxEvents - 1
	}
	end := start + 20
	end = min(end, maxEvents)

	for _, e := range m.service.Events[start:end] {
		ts := theme.HelpStyle.Render(e.CreatedAt.Format("15:04:05"))
		fmt.Fprintf(&b, "  %s  %s\n", ts, e.Message)
	}

	if end < maxEvents {
		b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("\n  ... %d more events (scroll down)", maxEvents-end)))
	}

	return b.String()
}

func (m ServiceDetailModel) SetService(s *model.Service) ServiceDetailModel {
	m.service = s
	return m
}

func (m ServiceDetailModel) SetSize(w, h int) ServiceDetailModel {
	m.width = w
	m.height = h
	return m
}

func rolloutToHealth(state string) string {
	switch state {
	case "COMPLETED":
		return "healthy"
	case "IN_PROGRESS":
		return "deploying"
	case "FAILED":
		return "unhealthy"
	default:
		return ""
	}
}
