package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type CBBuildsModel struct {
	projectName string
	builds      []aws.CBBuild
	cursor      int
	utcTime     bool
	width       int
	height      int
	loaded      bool
}

func NewCBBuilds(projectName string) CBBuildsModel {
	return CBBuildsModel{projectName: projectName}
}

func (m CBBuildsModel) Update(msg tea.Msg) (CBBuildsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			if m.cursor < len(m.builds)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(m.builds)-1))
		case msg.String() == "t":
			m.utcTime = !m.utcTime
		}
	}
	return m, nil
}

func (m CBBuildsModel) View() string {
	var b strings.Builder

	title := fmt.Sprintf("  Builds: %s (%d)", m.projectName, len(m.builds))
	b.WriteString(theme.TitleStyle.Render(title))
	b.WriteString("\n\n")

	if len(m.builds) == 0 {
		if !m.loaded {
			b.WriteString(theme.HelpStyle.Render("  Loading..."))
		} else {
			b.WriteString(theme.HelpStyle.Render("  No builds found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "#"},
		{Title: "STATUS"},
		{Title: "STARTED"},
		{Title: "DURATION"},
		{Title: "INITIATOR"},
		{Title: "SOURCE VERSION"},
	})
	for _, build := range m.builds {
		statusCell := buildStatusCell(build.Status)
		var ts string
		if m.utcTime {
			ts = build.StartTime.UTC().Format("2006-01-02 15:04:05 UTC")
		} else {
			ts = build.StartTime.Local().Format("2006-01-02 15:04:05")
		}
		dur := ""
		if build.Duration > 0 {
			dur = build.Duration.Truncate(1e9).String() // truncate to seconds
		} else if build.Status == "IN_PROGRESS" {
			dur = build.CurrentPhase
		}
		initiator := build.Initiator
		if len(initiator) > 30 {
			initiator = initiator[:27] + "..."
		}
		srcVer := build.SourceVersion
		if len(srcVer) > 20 {
			srcVer = srcVer[:17] + "..."
		}
		tbl.AddRow(
			components.Plain(fmt.Sprintf("%d", build.BuildNumber)),
			statusCell,
			components.Plain(ts),
			components.Plain(dur),
			components.Plain(initiator),
			components.Plain(srcVer),
		)
	}
	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func buildStatusCell(status string) components.Cell {
	var style lipgloss.Style
	switch status {
	case "SUCCEEDED":
		style = lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	case "FAILED", "FAULT":
		style = lipgloss.NewStyle().Foreground(theme.ColorRed).Bold(true)
	case "IN_PROGRESS":
		style = lipgloss.NewStyle().Foreground(theme.ColorCyan).Bold(true)
	case "STOPPED":
		style = lipgloss.NewStyle().Foreground(theme.ColorYellow)
	default:
		style = lipgloss.NewStyle().Foreground(theme.ColorDim)
	}
	return components.Styled(status, style)
}

func (m CBBuildsModel) SetBuilds(builds []aws.CBBuild) CBBuildsModel {
	m.builds = builds
	m.loaded = true
	if m.cursor >= len(builds) && len(builds) > 0 {
		m.cursor = len(builds) - 1
	}
	return m
}

func (m CBBuildsModel) SelectedBuild() *aws.CBBuild {
	if len(m.builds) == 0 || m.cursor >= len(m.builds) {
		return nil
	}
	b := m.builds[m.cursor]
	return &b
}

func (m CBBuildsModel) ProjectName() string { return m.projectName }

func (m CBBuildsModel) visibleRows() int {
	overhead := 9
	rows := m.height - overhead
	if rows < 5 {
		return 0
	}
	return rows
}

func (m CBBuildsModel) SetSize(w, h int) CBBuildsModel {
	m.width = w
	m.height = h
	return m
}
