package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type EC2SecurityGroupDetailModel struct {
	group  *model.EC2SecurityGroup
	scroll int
	width  int
	height int
}

func NewEC2SecurityGroupDetail(group *model.EC2SecurityGroup) EC2SecurityGroupDetailModel {
	return EC2SecurityGroupDetailModel{group: group}
}

func (m EC2SecurityGroupDetailModel) Update(msg tea.Msg) (EC2SecurityGroupDetailModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, theme.Keys.Up):
			m.scroll = max(0, m.scroll-1)
		case key.Matches(keyMsg, theme.Keys.Down):
			m.scroll++
		case keyMsg.String() == "pgup":
			m.scroll = max(0, m.scroll-m.visibleRows())
		case keyMsg.String() == "pgdown":
			m.scroll += m.visibleRows()
		case keyMsg.String() == "g":
			m.scroll = 0
		case keyMsg.String() == "G":
			m.scroll = 999
		}
	}
	return m, nil
}

func (m EC2SecurityGroupDetailModel) View() string {
	if m.group == nil {
		return theme.HelpStyle.Render("  Loading...")
	}
	group := m.group
	lines := []string{
		theme.TitleStyle.Render("  Security Group: " + firstDisplay(group.Name, group.GroupID)), "",
		fmt.Sprintf("  %-14s %s", "Group ID:", group.GroupID),
		fmt.Sprintf("  %-14s %s", "VPC:", group.VpcID),
		fmt.Sprintf("  %-14s %s", "Owner:", group.OwnerID),
		fmt.Sprintf("  %-14s %s", "Description:", group.Description), "",
		theme.TitleStyle.Render("  Rules"), "",
	}
	if len(group.Rules) == 0 {
		lines = append(lines, "  None")
	} else {
		table := components.NewTable([]components.Column{
			{Title: "DIR"}, {Title: "PROTO"}, {Title: "PORTS"},
			{Title: "SOURCE/DEST"}, {Title: "RULE ID"}, {Title: "DESCRIPTION"},
		})
		for _, rule := range group.Rules {
			table.AddRow(components.Plain(rule.Direction), components.Plain(rule.Protocol),
				components.Plain(rule.PortRange), components.Plain(rule.Source),
				components.Plain(rule.RuleID), components.Plain(rule.Description))
		}
		lines = append(lines, table.Render(-1, "", 200))
	}
	lines = append(lines, "", theme.TitleStyle.Render("  Associated Resources"), "")
	if len(group.Associations) == 0 {
		lines = append(lines, "  None")
	} else {
		for _, association := range group.Associations {
			lines = append(lines, fmt.Sprintf("  %-20s %-24s %s", association.Kind, association.ID, association.Name))
		}
		lines = append(lines, theme.HelpStyle.Render("  [o] open linked resource"))
	}
	if len(group.Tags) > 0 {
		lines = append(lines, "", theme.TitleStyle.Render("  Tags"), "")
		keys := make([]string, 0, len(group.Tags))
		for tag := range group.Tags {
			keys = append(keys, tag)
		}
		sort.Strings(keys)
		for _, tag := range keys {
			lines = append(lines, fmt.Sprintf("  %-30s %s", tag, group.Tags[tag]))
		}
	}
	visible := m.visibleRows()
	if m.scroll > len(lines)-visible {
		m.scroll = max(0, len(lines)-visible)
	}
	return strings.Join(lines[m.scroll:min(len(lines), m.scroll+visible)], "\n")
}

func (m EC2SecurityGroupDetailModel) Group() *model.EC2SecurityGroup { return m.group }
func (m EC2SecurityGroupDetailModel) SetSize(width, height int) EC2SecurityGroupDetailModel {
	m.width, m.height = width, height
	return m
}
func (m EC2SecurityGroupDetailModel) visibleRows() int {
	if m.height < 8 {
		return 20
	}
	return m.height - 2
}

func firstDisplay(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "(unnamed)"
}
