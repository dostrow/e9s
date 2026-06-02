package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type RDSInstancesModel struct {
	instances   []aws.RDSInstance
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewRDSInstances() RDSInstancesModel {
	return RDSInstancesModel{}
}

func (m RDSInstancesModel) Update(msg tea.Msg) (RDSInstancesModel, tea.Cmd) {
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
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m RDSInstancesModel) View() string {
	filtered := m.filteredInstances()
	var b strings.Builder

	title := fmt.Sprintf("  RDS Instances (%d)", len(filtered))
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
			b.WriteString(theme.HelpStyle.Render("  No RDS instances found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "IDENTIFIER"},
		{Title: "ENGINE"},
		{Title: "CLASS"},
		{Title: "STATUS"},
		{Title: "ROLE"},
		{Title: "AZ"},
		{Title: "ENDPOINT"},
		{Title: "CREATED"},
	})

	for _, inst := range filtered {
		engineVer := inst.Engine
		if inst.Version != "" {
			engineVer = inst.Engine + " " + inst.Version
		}

		endpoint := inst.Endpoint
		if endpoint != "" && inst.Port > 0 {
			endpoint = fmt.Sprintf("%s:%d", endpoint, inst.Port)
		}
		if len(endpoint) > 30 {
			endpoint = endpoint[:30] + ".."
		}

		created := ""
		if !inst.Created.IsZero() {
			created = formatAge(inst.Created) + " ago"
		}

		tbl.AddRow(
			components.Plain(inst.Identifier),
			components.Plain(engineVer),
			components.Plain(inst.Class),
			components.Styled(inst.Status, rdsStatusStyle(inst.Status)),
			components.Styled(inst.Role, rdsRoleStyle(inst.Role)),
			components.Plain(inst.AZ),
			components.Plain(endpoint),
			components.Plain(created),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func rdsStatusStyle(status string) lipgloss.Style {
	switch status {
	case "available":
		return lipgloss.NewStyle().Foreground(theme.ColorGreen)
	case "stopped":
		return lipgloss.NewStyle().Foreground(theme.ColorDim)
	case "starting", "stopping", "modifying", "upgrading", "rebooting",
		"backing-up", "maintenance", "renaming", "restoring", "configuring-enhanced-monitoring":
		return lipgloss.NewStyle().Foreground(theme.ColorYellow)
	case "failed", "incompatible-restore", "incompatible-network", "incompatible-parameters",
		"incompatible-option-group", "restore-error":
		return lipgloss.NewStyle().Foreground(theme.ColorRed)
	}
	return lipgloss.NewStyle().Foreground(theme.ColorWhite)
}

func (m RDSInstancesModel) filteredInstances() []aws.RDSInstance {
	if m.filter == "" {
		return m.instances
	}
	lf := strings.ToLower(m.filter)
	var out []aws.RDSInstance
	for _, inst := range m.instances {
		if strings.Contains(strings.ToLower(inst.Identifier), lf) ||
			strings.Contains(strings.ToLower(inst.Engine), lf) ||
			strings.Contains(strings.ToLower(inst.Status), lf) ||
			strings.Contains(strings.ToLower(inst.Role), lf) ||
			strings.Contains(strings.ToLower(inst.ClusterID), lf) {
			out = append(out, inst)
		}
	}
	return out
}

func (m RDSInstancesModel) SetInstances(instances []aws.RDSInstance) RDSInstancesModel {
	m.instances = instances
	m.loaded = true
	filtered := m.filteredInstances()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m RDSInstancesModel) SelectedInstance() *aws.RDSInstance {
	filtered := m.filteredInstances()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	inst := filtered[m.cursor]
	return &inst
}

func (m RDSInstancesModel) IsFiltering() bool { return m.filtering }

func (m RDSInstancesModel) visibleRows() int {
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

func (m RDSInstancesModel) SetSize(w, h int) RDSInstancesModel {
	m.width = w
	m.height = h
	return m
}
