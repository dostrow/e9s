package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type SQSDetailModel struct {
	queueName      string
	queueURL       string
	stats          *model.SQSQueueStats
	metrics        *model.MetricSnapshot
	metricsWarning string
	width          int
	height         int
}

func NewSQSDetail(queueName, queueURL string) SQSDetailModel {
	return SQSDetailModel{queueName: queueName, queueURL: queueURL}
}

func (m SQSDetailModel) View() string {
	var b strings.Builder

	b.WriteString(theme.TitleStyle.Render(fmt.Sprintf("  Queue: %s", m.queueName)))
	b.WriteString("\n\n")

	if m.stats == nil {
		b.WriteString(theme.HelpStyle.Render("  Loading..."))
		return b.String()
	}

	s := m.stats
	msgStyle := lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	if s.MessagesAvailable > 0 {
		msgStyle = lipgloss.NewStyle().Foreground(theme.ColorYellow).Bold(true)
	}

	fmt.Fprintf(&b, "  %-25s %s\n", "Messages Available:", msgStyle.Render(fmt.Sprintf("%d", s.MessagesAvailable)))
	fmt.Fprintf(&b, "  %-25s %d\n", "Messages In Flight:", s.MessagesInFlight)
	fmt.Fprintf(&b, "  %-25s %d\n", "Messages Delayed:", s.MessagesDelayed)
	fmt.Fprintf(&b, "  %-25s %ds\n", "Visibility Timeout:", s.VisibilityTimeout)
	fmt.Fprintf(&b, "  %-25s %ds\n", "Default Delay:", s.DelaySeconds)
	fmt.Fprintf(&b, "  %-25s %ds (%dd)\n", "Retention Period:", s.RetentionSeconds, s.RetentionSeconds/86400)
	fmt.Fprintf(&b, "  %-25s %d KB\n", "Max Message Size:", s.MaxMessageSize/1024)

	if s.IsFIFO {
		fmt.Fprintf(&b, "  %-25s %s\n", "Type:", theme.HealthStyle("deploying").Render("FIFO"))
	} else {
		fmt.Fprintf(&b, "  %-25s Standard\n", "Type:")
	}

	if s.DeadLetterTargetARN != "" {
		fmt.Fprintf(&b, "  %-25s %s\n", "Dead Letter Queue:", s.DeadLetterTargetARN)
		fmt.Fprintf(&b, "  %-25s %d\n", "Max Receive Count:", s.MaxReceiveCount)
	}

	if m.metrics != nil {
		wroteHeading := false
		for _, series := range m.metrics.Series {
			if len(series.Points) == 0 {
				continue
			}
			if !wroteHeading {
				b.WriteString("\n  " + theme.TitleStyle.Render("LATEST CLOUDWATCH METRICS (15 MINUTES)") + "\n")
				wroteHeading = true
			}
			point := series.Points[len(series.Points)-1]
			fmt.Fprintf(&b, "  %-25s %.2f %s\n", series.Label+":", point.Value, series.Unit)
		}
	}
	if m.metricsWarning != "" {
		fmt.Fprintf(&b, "\n  %s\n", theme.HelpStyle.Render("CloudWatch metrics unavailable: "+m.metricsWarning))
	}

	b.WriteString("\n")
	fmt.Fprintf(&b, "  %-25s %s\n", "URL:", theme.HelpStyle.Render(m.queueURL))

	return b.String()
}

func (m SQSDetailModel) SetStats(stats *model.SQSQueueStats) SQSDetailModel {
	m.stats = stats
	return m
}

func (m SQSDetailModel) SetMetrics(metrics *model.MetricSnapshot, warning string) SQSDetailModel {
	m.metrics = metrics
	m.metricsWarning = warning
	return m
}

func (m SQSDetailModel) QueueName() string           { return m.queueName }
func (m SQSDetailModel) QueueURL() string            { return m.queueURL }
func (m SQSDetailModel) Stats() *model.SQSQueueStats { return m.stats }

func (m SQSDetailModel) SetSize(w, h int) SQSDetailModel {
	m.width = w
	m.height = h
	return m
}
