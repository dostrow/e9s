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
	scopeName      string
	taskScope      bool
	metrics        *aws.ServiceMetrics
	alarms         []aws.AlarmState
	scaleKnown     bool
	scaleSuspended bool
	warnings       []string
	width          int
	height         int
}

func NewMetrics(scopeName string, taskScope ...bool) MetricsModel {
	isTask := len(taskScope) > 0 && taskScope[0]
	return MetricsModel{scopeName: scopeName, taskScope: isTask}
}

func (m MetricsModel) View() string {
	var b strings.Builder

	scope := "Service"
	if m.taskScope {
		scope = "Task"
	}
	b.WriteString(theme.TitleStyle.Render(fmt.Sprintf("  %s Metrics: %s", scope, m.scopeName)))
	b.WriteString("\n\n")

	if m.metrics == nil {
		b.WriteString(theme.HelpStyle.Render("  Loading metrics..."))
		return b.String()
	}

	b.WriteString(theme.TitleStyle.Render("  CPU Utilization"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "  %-12s %s\n", "Average:", renderMetric(m.metrics.CPUAvg, m.metrics.CPUAvgAvailable, m.width-20))
	fmt.Fprintf(&b, "  %-12s %s\n", "Maximum:", renderMetric(m.metrics.CPUMax, m.metrics.CPUMaxAvailable, m.width-20))
	b.WriteString("\n")

	b.WriteString(theme.TitleStyle.Render("  Memory Utilization"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "  %-12s %s\n", "Average:", renderMetric(m.metrics.MemAvg, m.metrics.MemAvgAvailable, m.width-20))
	fmt.Fprintf(&b, "  %-12s %s\n", "Maximum:", renderMetric(m.metrics.MemMax, m.metrics.MemMaxAvailable, m.width-20))
	b.WriteString("\n")

	if !metricsAvailable(m.metrics) {
		message := "  No service-level datapoints were returned for this period."
		if m.taskScope {
			message = "  No task-level datapoints were returned. Enable ECS Container Insights with enhanced observability and allow time for metrics to arrive."
		}
		b.WriteString(theme.HelpStyle.Render(message))
		b.WriteString("\n\n")
	}

	for _, warning := range m.warnings {
		b.WriteString(theme.HelpStyle.Render("  "+warning) + "\n")
	}
	if len(m.warnings) > 0 {
		b.WriteString("\n")
	}

	if m.taskScope {
		return b.String()
	}

	if m.scaleKnown {
		status := "enabled"
		if m.scaleSuspended {
			status = "suspended"
		}
		b.WriteString(theme.HelpStyle.Render("  Service scale-in: " + status + "  [I] toggle"))
		b.WriteString("\n\n")
	}

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

func renderMetric(value float64, available bool, width int) string {
	if !available {
		return theme.HelpStyle.Render("No data")
	}
	return renderBar(value, 100, width)
}

func metricsAvailable(metrics *aws.ServiceMetrics) bool {
	return metrics != nil && (metrics.CPUAvgAvailable || metrics.CPUMaxAvailable || metrics.MemAvgAvailable || metrics.MemMaxAvailable)
}

func (m MetricsModel) SetMetrics(metrics *aws.ServiceMetrics) MetricsModel {
	m.metrics = metrics
	return m
}

func (m MetricsModel) SetAlarms(alarms []aws.AlarmState) MetricsModel {
	m.alarms = alarms
	return m
}

func (m MetricsModel) SetScaleIn(known, suspended bool) MetricsModel {
	m.scaleKnown = known
	m.scaleSuspended = suspended
	return m
}

func (m MetricsModel) SetWarnings(warnings []string) MetricsModel {
	m.warnings = append([]string(nil), warnings...)
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
	filled = min(filled, barWidth)
	filled = max(0, filled)
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
