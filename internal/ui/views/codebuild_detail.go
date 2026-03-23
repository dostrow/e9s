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

type CBBuildDetailModel struct {
	detail *aws.CBBuildDetail
	scroll int
	width  int
	height int
}

func NewCBBuildDetail(detail *aws.CBBuildDetail) CBBuildDetailModel {
	return CBBuildDetailModel{detail: detail}
}

func (m CBBuildDetailModel) Update(msg tea.Msg) (CBBuildDetailModel, tea.Cmd) {
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

func (m CBBuildDetailModel) View() string {
	if m.detail == nil {
		return theme.HelpStyle.Render("  Loading...")
	}

	d := m.detail
	var lines []string

	// Title
	lines = append(lines, theme.TitleStyle.Render(fmt.Sprintf("  Build #%d — %s", d.BuildNumber, d.ProjectName)))
	lines = append(lines, "")

	// Status
	statusStyle := buildDetailStatusStyle(d.Status)
	lines = append(lines, fmt.Sprintf("  %-22s %s", "Status:", statusStyle.Render(d.Status)))
	if d.CurrentPhase != "" && d.Status == "IN_PROGRESS" {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Current Phase:", d.CurrentPhase))
	}
	lines = append(lines, fmt.Sprintf("  %-22s %s", "Started:", d.StartTime.Local().Format(time.RFC3339)))
	if !d.EndTime.IsZero() {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Ended:", d.EndTime.Local().Format(time.RFC3339)))
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Duration:", d.Duration.Truncate(time.Second).String()))
	}
	if d.Initiator != "" {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Initiator:", d.Initiator))
	}
	lines = append(lines, "")

	// Source
	lines = append(lines, theme.TitleStyle.Render("  Source"))
	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("  %-22s %s", "Type:", d.Source.Type))
	if d.Source.Location != "" {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Location:", d.Source.Location))
	}
	if d.Source.Version != "" {
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Version:", d.Source.Version))
	}
	lines = append(lines, "")

	// Phases
	if len(d.Phases) > 0 {
		lines = append(lines, theme.TitleStyle.Render("  Build Phases"))
		lines = append(lines, "")
		for _, p := range d.Phases {
			pStyle := phaseStatusStyle(p.Status)
			dur := ""
			if p.Duration > 0 {
				dur = p.Duration.String()
			}
			lines = append(lines, fmt.Sprintf("  %-20s %s  %s",
				p.Name,
				pStyle.Render(fmt.Sprintf("%-12s", p.Status)),
				theme.HelpStyle.Render(dur)))
			for _, ctx := range p.Contexts {
				lines = append(lines, fmt.Sprintf("    %s", theme.ErrorStyle.Render(ctx)))
			}
		}
		lines = append(lines, "")
	}

	// Logs info
	if d.LogGroupName != "" {
		lines = append(lines, theme.TitleStyle.Render("  Logs"))
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("  %-22s %s", "Log Group:", d.LogGroupName))
		if d.LogStreamName != "" {
			lines = append(lines, fmt.Sprintf("  %-22s %s", "Log Stream:", d.LogStreamName))
		}
		lines = append(lines, "")
	}

	// Environment variables
	if len(d.Environment) > 0 {
		lines = append(lines, theme.TitleStyle.Render("  Environment"))
		lines = append(lines, "")
		for _, ev := range d.Environment {
			val := ev.Value
			if ev.Type != "PLAINTEXT" {
				val = fmt.Sprintf("[%s] %s", ev.Type, ev.Value)
			}
			lines = append(lines, fmt.Sprintf("  %-30s %s", ev.Name, val))
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

func (m CBBuildDetailModel) Detail() *aws.CBBuildDetail { return m.detail }
func (m CBBuildDetailModel) BuildID() string {
	if m.detail != nil {
		return m.detail.ID
	}
	return ""
}
func (m CBBuildDetailModel) ProjectName() string {
	if m.detail != nil {
		return m.detail.ProjectName
	}
	return ""
}
func (m CBBuildDetailModel) LogGroup() string {
	if m.detail != nil {
		return m.detail.LogGroupName
	}
	return ""
}
func (m CBBuildDetailModel) LogStream() string {
	if m.detail != nil {
		return m.detail.LogStreamName
	}
	return ""
}

func (m CBBuildDetailModel) visibleRows() int {
	rows := m.height - 2
	if rows < 5 {
		return 20
	}
	return rows
}

func (m CBBuildDetailModel) SetSize(w, h int) CBBuildDetailModel {
	m.width = w
	m.height = h
	return m
}

func buildDetailStatusStyle(status string) lipgloss.Style {
	switch status {
	case "SUCCEEDED":
		return lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	case "FAILED", "FAULT":
		return lipgloss.NewStyle().Foreground(theme.ColorRed).Bold(true)
	case "IN_PROGRESS":
		return lipgloss.NewStyle().Foreground(theme.ColorCyan).Bold(true)
	case "STOPPED":
		return lipgloss.NewStyle().Foreground(theme.ColorYellow)
	default:
		return lipgloss.NewStyle().Foreground(theme.ColorDim)
	}
}

func phaseStatusStyle(status string) lipgloss.Style {
	switch status {
	case "SUCCEEDED":
		return lipgloss.NewStyle().Foreground(theme.ColorGreen)
	case "FAILED", "FAULT":
		return lipgloss.NewStyle().Foreground(theme.ColorRed)
	case "IN_PROGRESS":
		return lipgloss.NewStyle().Foreground(theme.ColorCyan)
	default:
		return lipgloss.NewStyle().Foreground(theme.ColorDim)
	}
}
