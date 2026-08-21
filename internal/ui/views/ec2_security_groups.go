package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type EC2SecurityGroupsModel struct {
	groups      []model.EC2SecurityGroup
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewEC2SecurityGroups() EC2SecurityGroupsModel { return EC2SecurityGroupsModel{} }

func (m EC2SecurityGroupsModel) Update(msg tea.Msg) (EC2SecurityGroupsModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	if m.filtering {
		switch keyMsg.String() {
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
		m.filterInput, cmd = m.filterInput.Update(keyMsg)
		return m, cmd
	}
	switch {
	case key.Matches(keyMsg, theme.Keys.Up):
		m.cursor = max(0, m.cursor-1)
	case key.Matches(keyMsg, theme.Keys.Down):
		m.cursor = min(m.cursor+1, max(0, len(m.filtered())-1))
	case key.Matches(keyMsg, theme.Keys.Filter):
		m.filtering = true
		m.filterInput = textinput.New()
		m.filterInput.Placeholder = "filter security groups..."
		m.filterInput.SetValue(m.filter)
		m.filterInput.Focus()
		m.filterInput.Width = 40
		return m, m.filterInput.Focus()
	}
	return m, nil
}

func (m EC2SecurityGroupsModel) View() string {
	groups := m.filtered()
	var out strings.Builder
	fmt.Fprintf(&out, "%s", theme.TitleStyle.Render(fmt.Sprintf("  EC2 Security Groups (%d)", len(groups))))
	if m.filter != "" {
		out.WriteString(theme.HelpStyle.Render(fmt.Sprintf("  filter: %q", m.filter)))
	}
	out.WriteByte('\n')
	if m.filtering {
		out.WriteString("  / " + m.filterInput.View() + "\n")
	}
	out.WriteByte('\n')
	if len(groups) == 0 {
		if m.loaded {
			out.WriteString(theme.HelpStyle.Render("  No security groups found"))
		} else {
			out.WriteString(theme.HelpStyle.Render("  Loading..."))
		}
		return out.String()
	}
	table := components.NewTable([]components.Column{
		{Title: "NAME"}, {Title: "GROUP ID"}, {Title: "VPC"},
		{Title: "IN"}, {Title: "OUT"}, {Title: "DESCRIPTION"},
	})
	for _, group := range groups {
		inbound, outbound := 0, 0
		for _, rule := range group.Rules {
			if rule.Direction == "inbound" {
				inbound++
			} else if rule.Direction == "outbound" {
				outbound++
			}
		}
		table.AddRow(components.Plain(group.Name), components.Plain(group.GroupID),
			components.Plain(group.VpcID), components.Plain(fmt.Sprint(inbound)),
			components.Plain(fmt.Sprint(outbound)), components.Plain(group.Description))
	}
	out.WriteString(table.Render(m.cursor, "", m.visibleRows()))
	return out.String()
}

func (m EC2SecurityGroupsModel) filtered() []model.EC2SecurityGroup {
	return service.FilterEC2SecurityGroups(m.groups, m.filter, "")
}

func (m EC2SecurityGroupsModel) SetGroups(groups []model.EC2SecurityGroup) EC2SecurityGroupsModel {
	m.groups = groups
	m.loaded = true
	if m.cursor >= len(m.filtered()) {
		m.cursor = max(0, len(m.filtered())-1)
	}
	return m
}

func (m EC2SecurityGroupsModel) SelectedGroup() *model.EC2SecurityGroup {
	groups := m.filtered()
	if m.cursor < 0 || m.cursor >= len(groups) {
		return nil
	}
	group := groups[m.cursor]
	return &group
}

func (m EC2SecurityGroupsModel) IsFiltering() bool { return m.filtering }
func (m EC2SecurityGroupsModel) SetSize(width, height int) EC2SecurityGroupsModel {
	m.width, m.height = width, height
	return m
}
func (m EC2SecurityGroupsModel) visibleRows() int {
	if m.height < 10 {
		return 20
	}
	return m.height - 5
}
