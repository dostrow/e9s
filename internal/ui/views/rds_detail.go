package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type RDSDetailModel struct {
	detail *aws.RDSInstanceDetail
	scroll int
	width  int
	height int
}

func NewRDSDetail(detail *aws.RDSInstanceDetail) RDSDetailModel {
	return RDSDetailModel{detail: detail}
}

func (m RDSDetailModel) Update(msg tea.Msg) (RDSDetailModel, tea.Cmd) {
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
			m.scroll = max(0, m.scroll-m.visibleRows())
		case msg.String() == "pgdown":
			m.scroll += m.visibleRows()
		case msg.String() == "g":
			m.scroll = 0
		case msg.String() == "G":
			m.scroll = 9999
		}
	}
	return m, nil
}

func (m RDSDetailModel) View() string {
	if m.detail == nil {
		return theme.HelpStyle.Render("  Loading...")
	}
	d := m.detail
	var lines []string

	lines = append(lines, theme.TitleStyle.Render(fmt.Sprintf("  RDS: %s", d.Identifier)))
	lines = append(lines, "")

	// Status + role
	lines = append(lines, fmt.Sprintf("  %-24s %s", "Status:", lipgloss.NewStyle().Foreground(rdsStatusColor(d.Status)).Render(d.Status)))
	if d.Role != "" {
		lines = append(lines, fmt.Sprintf("  %-24s %s", "Role:", rdsRoleStyle(d.Role).Render(d.Role)))
	}
	if d.ClusterID != "" {
		lines = append(lines, fmt.Sprintf("  %-24s %s", "Cluster:", d.ClusterID))
		if d.Role == "reader" && d.PromotionTier > 0 {
			lines = append(lines, fmt.Sprintf("  %-24s %d", "Failover Priority:", d.PromotionTier))
		}
	}
	lines = append(lines, "")

	// Instance info
	lines = append(lines, theme.TitleStyle.Render("  Instance"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %-24s %s", "Engine:", d.Engine+" "+d.Version))
	lines = append(lines, fmt.Sprintf("  %-24s %s", "Class:", d.Class))
	lines = append(lines, fmt.Sprintf("  %-24s %s", "AZ:", d.AZ))
	if d.MultiAZ {
		lines = append(lines, fmt.Sprintf("  %-24s yes", "Multi-AZ:"))
	}
	if !d.Created.IsZero() {
		lines = append(lines, fmt.Sprintf("  %-24s %s", "Created:", d.Created.Local().Format(time.RFC3339)))
	}
	if d.CACertificate != "" {
		lines = append(lines, fmt.Sprintf("  %-24s %s", "CA Certificate:", d.CACertificate))
	}
	lines = append(lines, "")

	// Network
	lines = append(lines, theme.TitleStyle.Render("  Network"))
	lines = append(lines, "")
	if d.Endpoint != "" {
		lines = append(lines, fmt.Sprintf("  %-24s %s:%d", "Endpoint:", d.Endpoint, d.Port))
	}
	if d.VPCID != "" {
		lines = append(lines, fmt.Sprintf("  %-24s %s", "VPC:", d.VPCID))
	}
	if d.SubnetGroup != "" {
		lines = append(lines, fmt.Sprintf("  %-24s %s", "Subnet Group:", d.SubnetGroup))
	}
	for i, sg := range d.SecurityGroups {
		label := ""
		if i == 0 {
			label = "Security Groups:"
		}
		lines = append(lines, fmt.Sprintf("  %-24s %s", label, sg))
	}
	lines = append(lines, "")

	// Storage
	lines = append(lines, theme.TitleStyle.Render("  Storage"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %-24s %d GiB (%s)", "Allocated:", d.StorageGB, d.StorageType))
	enc := "no"
	if d.Encrypted {
		enc = "yes"
	}
	lines = append(lines, fmt.Sprintf("  %-24s %s", "Encrypted:", enc))
	dp := "no"
	if d.DeletionProtection {
		dp = "yes"
	}
	lines = append(lines, fmt.Sprintf("  %-24s %s", "Deletion Protection:", dp))
	lines = append(lines, "")

	// Backup & maintenance
	lines = append(lines, theme.TitleStyle.Render("  Backup & Maintenance"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %-24s %d days", "Backup Retention:", d.BackupRetentionDays))
	if d.BackupWindow != "" {
		lines = append(lines, fmt.Sprintf("  %-24s %s UTC", "Backup Window:", d.BackupWindow))
	}
	if !d.LatestRestorableTime.IsZero() {
		lines = append(lines, fmt.Sprintf("  %-24s %s", "Latest Restorable:", d.LatestRestorableTime.Local().Format(time.RFC3339)))
	}
	if d.MaintenanceWindow != "" {
		lines = append(lines, fmt.Sprintf("  %-24s %s UTC", "Maintenance Window:", d.MaintenanceWindow))
	}
	if d.ReadReplicaSource != "" {
		lines = append(lines, fmt.Sprintf("  %-24s %s", "Replica Source:", d.ReadReplicaSource))
	}
	for i, pg := range d.ParameterGroups {
		label := ""
		if i == 0 {
			label = "Parameter Groups:"
		}
		lines = append(lines, fmt.Sprintf("  %-24s %s", label, pg))
	}
	lines = append(lines, "")

	// CloudWatch metrics
	lines = append(lines, theme.TitleStyle.Render("  Metrics (last 5 min, avg)"))
	lines = append(lines, "")
	if !d.MetricsLoaded {
		lines = append(lines, theme.HelpStyle.Render("  Loading metrics..."))
	} else {
		lines = append(lines, fmt.Sprintf("  %-24s %s", "CPU:", fmtPct(d.CPUPercent)))
		lines = append(lines, fmt.Sprintf("  %-24s %.0f", "Connections:", d.DBConnections))
		lines = append(lines, fmt.Sprintf("  %-24s %.1f GiB", "Free Storage:", d.FreeStorageGB))
		lines = append(lines, fmt.Sprintf("  %-24s %.1f / %.1f", "Read/Write IOPS:", d.ReadIOPS, d.WriteIOPS))
		lines = append(lines, fmt.Sprintf("  %-24s %.2f ms / %.2f ms", "Read/Write Latency:", d.ReadLatencyMs, d.WriteLatencyMs))
	}

	// Tags
	if len(d.Tags) > 0 {
		lines = append(lines, "")
		lines = append(lines, theme.TitleStyle.Render("  Tags"))
		lines = append(lines, "")
		keys := sortedKeys(d.Tags)
		for _, k := range keys {
			lines = append(lines, fmt.Sprintf("  %-24s %s", k+":", d.Tags[k]))
		}
	}

	// Apply scroll
	visible := m.visibleRows()
	start := m.scroll
	if start > len(lines)-visible {
		start = max(0, len(lines)-visible)
	}
	end := start + visible
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

func rdsStatusColor(status string) lipgloss.TerminalColor {
	switch status {
	case "available":
		return theme.ColorGreen
	case "stopped":
		return theme.ColorDim
	case "starting", "stopping", "modifying", "upgrading", "rebooting",
		"backing-up", "maintenance", "renaming", "restoring":
		return theme.ColorYellow
	case "failed", "incompatible-restore", "incompatible-network",
		"incompatible-parameters", "restore-error":
		return theme.ColorRed
	}
	return theme.ColorWhite
}

func rdsRoleStyle(role string) lipgloss.Style {
	switch role {
	case "writer", "primary":
		return lipgloss.NewStyle().Foreground(theme.ColorCyan).Bold(true)
	case "reader", "replica":
		return lipgloss.NewStyle().Foreground(theme.ColorDim)
	}
	return lipgloss.NewStyle()
}

func fmtPct(v float64) string {
	style := lipgloss.NewStyle()
	switch {
	case v >= 90:
		style = style.Foreground(theme.ColorRed)
	case v >= 70:
		style = style.Foreground(theme.ColorYellow)
	default:
		style = style.Foreground(theme.ColorGreen)
	}
	return style.Render(fmt.Sprintf("%.1f%%", v))
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func (m RDSDetailModel) InstanceID() string {
	if m.detail == nil {
		return ""
	}
	return m.detail.Identifier
}

func (m RDSDetailModel) visibleRows() int {
	rows := m.height - 6
	if rows < 5 {
		return 5
	}
	return rows
}

func (m RDSDetailModel) SetSize(w, h int) RDSDetailModel {
	m.width = w
	m.height = h
	return m
}
