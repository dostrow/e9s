package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type MetricsModel struct {
	serviceName string
	metrics     *aws.ServiceMetrics
	alarms      []aws.AlarmState
	width       int
	height      int
}

func NewMetrics(serviceName string) MetricsModel {
	return MetricsModel{serviceName: serviceName}
}

func (m MetricsModel) View() string {
	var b strings.Builder

	b.WriteString(theme.TitleStyle.Render(fmt.Sprintf("  Metrics: %s", m.serviceName)))
	b.WriteString("\n\n")

	if m.metrics == nil {
		b.WriteString(theme.HelpStyle.Render("  Loading metrics..."))
		return b.String()
	}

	b.WriteString(theme.TitleStyle.Render("  CPU Utilization"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %-12s %s\n", "Average:", renderBar(m.metrics.CPUAvg, 50, m.width-20)))
	b.WriteString(fmt.Sprintf("  %-12s %s\n", "Maximum:", renderBar(m.metrics.CPUMax, 50, m.width-20)))
	b.WriteString("\n")

	b.WriteString(theme.TitleStyle.Render("  Memory Utilization"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %-12s %s\n", "Average:", renderBar(m.metrics.MemAvg, 50, m.width-20)))
	b.WriteString(fmt.Sprintf("  %-12s %s\n", "Maximum:", renderBar(m.metrics.MemMax, 50, m.width-20)))
	b.WriteString("\n")

	if len(m.alarms) > 0 {
		b.WriteString(theme.TitleStyle.Render("  CloudWatch Alarms"))
		b.WriteString("\n\n")

		tbl := components.NewTable([]components.Column{
			{Title: "NAME"},
			{Title: "STATE"},
			{Title: "METRIC"},
			{Title: "UPDATED"},
		})

		for _, a := range m.alarms {
			updated := ""
			if !a.UpdatedAt.IsZero() {
				updated = formatAge(a.UpdatedAt) + " ago"
			}
			tbl.AddRow(
				components.Plain(a.Name),
				components.Styled(a.State, alarmStateStyle(a.State)),
				components.Plain(a.MetricName),
				components.Plain(updated),
			)
		}

		b.WriteString(tbl.Render(-1, "", 0))
	} else {
		b.WriteString(theme.HelpStyle.Render("  No CloudWatch alarms found for this service"))
	}

	return b.String()
}

func (m MetricsModel) SetMetrics(metrics *aws.ServiceMetrics) MetricsModel {
	m.metrics = metrics
	return m
}

func (m MetricsModel) SetAlarms(alarms []aws.AlarmState) MetricsModel {
	m.alarms = alarms
	return m
}

func (m MetricsModel) SetSize(w, h int) MetricsModel {
	m.width = w
	m.height = h
	return m
}

func renderBar(pct float64, maxPct float64, barWidth int) string {
	if barWidth < 10 {
		barWidth = 40
	}
	filled := int(pct / maxPct * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	if filled < 0 {
		filled = 0
	}
	empty := barWidth - filled

	color := theme.ColorGreen
	if pct > 80 {
		color = theme.ColorRed
	} else if pct > 60 {
		color = theme.ColorYellow
	}

	bar := lipgloss.NewStyle().Foreground(color).Render(strings.Repeat("█", filled))
	bar += lipgloss.NewStyle().Foreground(theme.ColorDim).Render(strings.Repeat("░", empty))
	pctStr := fmt.Sprintf(" %.1f%%", pct)

	return bar + pctStr
}

func alarmStateStyle(state string) lipgloss.Style {
	switch state {
	case "OK":
		return lipgloss.NewStyle().Foreground(theme.ColorGreen)
	case "ALARM":
		return lipgloss.NewStyle().Foreground(theme.ColorRed).Bold(true)
	default:
		return lipgloss.NewStyle().Foreground(theme.ColorYellow)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-2] + ".."
}
