package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type EC2InstancesModel struct {
	instances   []aws.EC2Instance
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewEC2Instances() EC2InstancesModel {
	return EC2InstancesModel{}
}

func (m EC2InstancesModel) Update(msg tea.Msg) (EC2InstancesModel, tea.Cmd) {
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
			filtered := m.filteredInstances()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredInstances()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter instances..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m EC2InstancesModel) View() string {
	filtered := m.filteredInstances()
	var b strings.Builder

	title := fmt.Sprintf("  EC2 Instances (%d)", len(filtered))
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
			b.WriteString(theme.HelpStyle.Render("  No instances found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "NAME"},
		{Title: "INSTANCE ID"},
		{Title: "STATE"},
		{Title: "TYPE"},
		{Title: "AZ"},
		{Title: "PRIVATE IP"},
		{Title: "AGE"},
	})
	for _, inst := range filtered {
		stateCell := ec2StateCell(inst.State)
		name := inst.Name
		if name == "" {
			name = "(unnamed)"
		}
		age := ""
		if !inst.LaunchTime.IsZero() {
			age = shortDuration(time.Since(inst.LaunchTime))
		}
		tbl.AddRow(
			components.Plain(name),
			components.Plain(inst.InstanceID),
			stateCell,
			components.Plain(inst.Type),
			components.Plain(inst.AZ),
			components.Plain(inst.PrivateIP),
			components.Plain(age),
		)
	}
	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func ec2StateCell(state string) components.Cell {
	var style lipgloss.Style
	switch state {
	case "running":
		style = lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	case "stopped":
		style = lipgloss.NewStyle().Foreground(theme.ColorRed)
	case "pending", "stopping", "shutting-down":
		style = lipgloss.NewStyle().Foreground(theme.ColorYellow)
	case "terminated":
		style = lipgloss.NewStyle().Foreground(theme.ColorDim)
	default:
		style = lipgloss.NewStyle().Foreground(theme.ColorDim)
	}
	return components.Styled(state, style)
}

func shortDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours()) / 24
	if days < 365 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dy%dd", days/365, days%365)
}

func (m EC2InstancesModel) filteredInstances() []aws.EC2Instance {
	if m.filter == "" {
		return m.instances
	}
	lf := strings.ToLower(m.filter)
	var out []aws.EC2Instance
	for _, i := range m.instances {
		if strings.Contains(strings.ToLower(i.Name), lf) ||
			strings.Contains(strings.ToLower(i.InstanceID), lf) ||
			strings.Contains(strings.ToLower(i.PrivateIP), lf) ||
			strings.Contains(strings.ToLower(i.Type), lf) ||
			strings.Contains(strings.ToLower(i.State), lf) {
			out = append(out, i)
		}
	}
	return out
}

func (m EC2InstancesModel) SetInstances(instances []aws.EC2Instance) EC2InstancesModel {
	m.instances = instances
	m.loaded = true
	filtered := m.filteredInstances()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m EC2InstancesModel) SelectedInstance() *aws.EC2Instance {
	filtered := m.filteredInstances()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	i := filtered[m.cursor]
	return &i
}

func (m EC2InstancesModel) IsFiltering() bool { return m.filtering }

func (m EC2InstancesModel) visibleRows() int {
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

func (m EC2InstancesModel) SetSize(w, h int) EC2InstancesModel {
	m.width = w
	m.height = h
	return m
}
