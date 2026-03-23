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
	"github.com/dostrow/e9s/internal/ui/theme"
)

type AlarmDetailModel struct {
	detail *aws.CWAlarmDetail
	scroll int
	width  int
	height int
}

func NewAlarmDetail(detail *aws.CWAlarmDetail) AlarmDetailModel {
	return AlarmDetailModel{detail: detail}
}

func (m AlarmDetailModel) Update(msg tea.Msg) (AlarmDetailModel, tea.Cmd) {
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

func (m AlarmDetailModel) View() string {
	if m.detail == nil {
		return theme.HelpStyle.Render("  Loading...")
	}

	d := m.detail
	var lines []string

	// Title
	lines = append(lines, theme.TitleStyle.Render(fmt.Sprintf("  Alarm: %s", d.Name)))
	lines = append(lines, "")

	// State
	stateStyle := lipgloss.NewStyle().Bold(true)
	switch d.State {
	case "OK":
		stateStyle = stateStyle.Foreground(theme.ColorGreen)
	case "ALARM":
		stateStyle = stateStyle.Foreground(theme.ColorRed)
	default:
		stateStyle = stateStyle.Foreground(theme.ColorYellow)
	}
	lines = append(lines, fmt.Sprintf("  %-25s %s", "State:", stateStyle.Render(d.State)))
	if d.StateReason != "" {
		lines = append(lines, fmt.Sprintf("  %-25s %s", "Reason:", truncate(d.StateReason, m.width-30)))
	}
	lines = append(lines, fmt.Sprintf("  %-25s %s", "Updated:", d.StateUpdatedAt.Local().Format(time.RFC3339)))
	lines = append(lines, "")

	// Configuration
	lines = append(lines, theme.TitleStyle.Render("  Configuration"))
	lines = append(lines, "")
	if d.Description != "" {
		lines = append(lines, fmt.Sprintf("  %-25s %s", "Description:", d.Description))
	}
	lines = append(lines, fmt.Sprintf("  %-25s %s", "Namespace:", d.Namespace))
	lines = append(lines, fmt.Sprintf("  %-25s %s", "Metric:", d.MetricName))
	if d.Statistic != "" {
		lines = append(lines, fmt.Sprintf("  %-25s %s", "Statistic:", d.Statistic))
	}
	lines = append(lines, fmt.Sprintf("  %-25s %s", "Comparison:", d.ComparisonOp))
	lines = append(lines, fmt.Sprintf("  %-25s %g", "Threshold:", d.Threshold))
	lines = append(lines, fmt.Sprintf("  %-25s %d", "Eval Periods:", d.EvalPeriods))
	lines = append(lines, fmt.Sprintf("  %-25s %ds", "Period:", d.Period))
	if d.TreatMissing != "" {
		lines = append(lines, fmt.Sprintf("  %-25s %s", "Treat Missing:", d.TreatMissing))
	}

	actionsLabel := "Enabled"
	if !d.ActionsEnabled {
		actionsLabel = lipgloss.NewStyle().Foreground(theme.ColorRed).Render("Disabled")
	}
	lines = append(lines, fmt.Sprintf("  %-25s %s", "Actions:", actionsLabel))
	lines = append(lines, "")

	// Dimensions
	if len(d.Dimensions) > 0 {
		lines = append(lines, theme.TitleStyle.Render("  Dimensions"))
		lines = append(lines, "")
		keys := make([]string, 0, len(d.Dimensions))
		for k := range d.Dimensions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			lines = append(lines, fmt.Sprintf("  %-25s %s", k+":", d.Dimensions[k]))
		}
		lines = append(lines, "")
	}

	// Actions
	if len(d.AlarmActions) > 0 {
		lines = append(lines, theme.TitleStyle.Render("  Alarm Actions"))
		lines = append(lines, "")
		for _, a := range d.AlarmActions {
			lines = append(lines, fmt.Sprintf("    %s", a))
		}
		lines = append(lines, "")
	}
	if len(d.OKActions) > 0 {
		lines = append(lines, theme.TitleStyle.Render("  OK Actions"))
		lines = append(lines, "")
		for _, a := range d.OKActions {
			lines = append(lines, fmt.Sprintf("    %s", a))
		}
		lines = append(lines, "")
	}

	// History
	if len(d.History) > 0 {
		lines = append(lines, theme.TitleStyle.Render("  Recent History"))
		lines = append(lines, "")
		for _, h := range d.History {
			ts := h.Timestamp.Local().Format("2006-01-02 15:04:05")
			typeStyle := lipgloss.NewStyle().Foreground(theme.ColorCyan)
			lines = append(lines, fmt.Sprintf("  %s  %s  %s",
				theme.HelpStyle.Render(ts),
				typeStyle.Render(h.Type),
				truncate(h.Summary, m.width-50)))
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

func (m AlarmDetailModel) Detail() *aws.CWAlarmDetail { return m.detail }
func (m AlarmDetailModel) AlarmName() string {
	if m.detail != nil {
		return m.detail.Name
	}
	return ""
}

func (m AlarmDetailModel) visibleRows() int {
	rows := m.height - 2
	if rows < 5 {
		return 20
	}
	return rows
}

func (m AlarmDetailModel) SetSize(w, h int) AlarmDetailModel {
	m.width = w
	m.height = h
	return m
}

func truncate(s string, maxLen int) string {
	if maxLen < 10 {
		maxLen = 10
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
