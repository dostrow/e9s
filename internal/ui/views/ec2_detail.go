package views

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type EC2DetailModel struct {
	detail *aws.EC2InstanceDetail
	scroll int
	width  int
	height int
}

func NewEC2Detail(detail *aws.EC2InstanceDetail) EC2DetailModel {
	return EC2DetailModel{detail: detail}
}

func (m EC2DetailModel) Update(msg tea.Msg) (EC2DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up), msg.String() == "k":
			if m.scroll > 0 {
				m.scroll--
			}
		case key.Matches(msg, theme.Keys.Down), msg.String() == "j":
			m.scroll++
		case msg.String() == "pgup":
			m.scroll = max(0, m.scroll-m.visibleRows())
		case msg.String() == "pgdown":
			m.scroll += m.visibleRows()
		case msg.String() == "g":
			m.scroll = 0
		case msg.String() == "G":
			m.scroll = 999
		}
	}
	return m, nil
}

func (m EC2DetailModel) View() string {
	if m.detail == nil {
		return theme.HelpStyle.Render("  Loading...")
	}

	d := m.detail
	var lines []string

	// Title
	name := d.Name
	if name == "" {
		name = d.InstanceID
	}
	lines = append(lines, theme.TitleStyle.Render(fmt.Sprintf("  Instance: %s", name)))
	lines = append(lines, "")

	// State
	stateStyle := lipgloss.NewStyle().Bold(true)
	switch d.State {
	case "running":
		stateStyle = stateStyle.Foreground(theme.ColorGreen)
	case "stopped":
		stateStyle = stateStyle.Foreground(theme.ColorRed)
	case "pending", "stopping":
		stateStyle = stateStyle.Foreground(theme.ColorYellow)
	default:
		stateStyle = stateStyle.Foreground(theme.ColorDim)
	}

	lines = append(lines, fmt.Sprintf("  %-22s %s", "State:", stateStyle.Render(d.State)))
	lines = append(lines, fmt.Sprintf("  %-22s %s", "Instance ID:", d.InstanceID))
	lines = append(lines, fmt.Sprintf("  %-22s %s", "Type:", d.Type))
	lines = append(lines, fmt.Sprintf("  %-22s %s", "AZ:", d.AZ))
	lines = append(lines, fmt.Sprintf("  %-22s %s", "Architecture:", d.Architecture))
	lines = append(lines, fmt.Sprintf("  %-22s %s", "AMI:", d.AMI))
	if d.KeyName != "" {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Key Name:", d.KeyName))
	}
	if d.IAMRole != "" {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "IAM Role:", d.IAMRole))
	}
	lines = append(lines, fmt.Sprintf("  %-22s %s", "Launched:", d.LaunchTime.Local().Format(time.RFC3339)))
	lines = append(lines, "")

	// Networking
	lines = append(lines, theme.TitleStyle.Render("  Networking"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %-22s %s", "Private IP:", d.PrivateIP))
	if d.PublicIP != "" {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Public IP:", d.PublicIP))
	}
	lines = append(lines, fmt.Sprintf("  %-22s %s", "VPC:", d.VpcID))
	lines = append(lines, fmt.Sprintf("  %-22s %s", "Subnet:", d.SubnetID))
	lines = append(lines, "")

	// Security Groups
	if len(d.SecurityGroups) > 0 {
		lines = append(lines, theme.TitleStyle.Render("  Security Groups"))
		lines = append(lines, "")
		for _, sg := range d.SecurityGroups {
			lines = append(lines, fmt.Sprintf("  %s (%s)", sg.Name, sg.ID))
		}
		lines = append(lines, "")
	}

	// Security Group Rules
	if len(d.SecurityGroupRules) > 0 {
		lines = append(lines, theme.TitleStyle.Render("  Security Group Rules"))
		lines = append(lines, "")
		tbl := components.NewTable([]components.Column{
			{Title: "DIR"},
			{Title: "PROTO"},
			{Title: "PORTS"},
			{Title: "SOURCE/DEST"},
		})
		for _, r := range d.SecurityGroupRules {
			dirStyle := lipgloss.NewStyle()
			if r.Direction == "inbound" {
				dirStyle = dirStyle.Foreground(theme.ColorCyan)
			} else {
				dirStyle = dirStyle.Foreground(theme.ColorDim)
			}
			tbl.AddRow(
				components.Styled(r.Direction, dirStyle),
				components.Plain(r.Protocol),
				components.Plain(r.PortRange),
				components.Plain(r.Source),
			)
		}
		lines = append(lines, tbl.Render(-1, "", 50))
		lines = append(lines, "")
	}

	// Volumes
	if len(d.Volumes) > 0 {
		lines = append(lines, theme.TitleStyle.Render("  Volumes"))
		lines = append(lines, "")
		for _, v := range d.Volumes {
			sizeStr := ""
			if v.Size > 0 {
				sizeStr = fmt.Sprintf("%d GiB", v.Size)
			}
			lines = append(lines, fmt.Sprintf("  %-12s %-12s %-10s %s %s",
				v.DeviceName, v.VolumeID, v.VolumeType, sizeStr, v.State))
		}
		lines = append(lines, "")
	}

	// Tags
	if len(d.Tags) > 0 {
		lines = append(lines, theme.TitleStyle.Render("  Tags"))
		lines = append(lines, "")
		keys := make([]string, 0, len(d.Tags))
		for k := range d.Tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			lines = append(lines, fmt.Sprintf("  %-30s %s", k, d.Tags[k]))
		}
	}

	// Apply scrolling
	visRows := m.visibleRows()
	if m.scroll > len(lines)-visRows {
		m.scroll = max(0, len(lines)-visRows)
	}
	end := min(m.scroll+visRows, len(lines))
	visible := lines[m.scroll:end]

	return strings.Join(visible, "\n")
}

func (m EC2DetailModel) Detail() *aws.EC2InstanceDetail { return m.detail }
func (m EC2DetailModel) InstanceID() string {
	if m.detail != nil {
		return m.detail.InstanceID
	}
	return ""
}
func (m EC2DetailModel) InstanceState() string {
	if m.detail != nil {
		return m.detail.State
	}
	return ""
}

func (m EC2DetailModel) visibleRows() int {
	rows := m.height - 2
	if rows < 5 {
		return 20
	}
	return rows
}

func (m EC2DetailModel) SetSize(w, h int) EC2DetailModel {
	m.width = w
	m.height = h
	return m
}
