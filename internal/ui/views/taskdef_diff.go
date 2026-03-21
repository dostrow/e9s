package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type TaskDefDiffModel struct {
	title  string
	diff   string
	scroll int
	width  int
	height int
}

func NewTaskDefDiff(title, diff string) TaskDefDiffModel {
	return TaskDefDiffModel{
		title: title,
		diff:  diff,
	}
}

func (m TaskDefDiffModel) Update(msg tea.Msg) (TaskDefDiffModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if m.scroll > 0 {
				m.scroll--
			}
		case key.Matches(msg, theme.Keys.Down):
			m.scroll++
		case msg.String() == "g":
			m.scroll = 0
		case msg.String() == "G":
			lines := strings.Split(m.diff, "\n")
			visible := m.height - 5
			if visible < 1 {
				visible = 20
			}
			m.scroll = max(0, len(lines)-visible)
		}
	}
	return m, nil
}

func (m TaskDefDiffModel) View() string {
	var b strings.Builder

	b.WriteString(theme.TitleStyle.Render(fmt.Sprintf("  Task Definition Diff: %s", m.title)))
	b.WriteString("\n\n")

	lines := strings.Split(m.diff, "\n")
	visible := m.height - 5
	if visible < 1 {
		visible = 20
	}

	start := m.scroll
	if start > len(lines)-visible {
		start = len(lines) - visible
	}
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > len(lines) {
		end = len(lines)
	}

	for _, line := range lines[start:end] {
		styled := styleDiffLine(line)
		b.WriteString("  " + styled + "\n")
	}

	return b.String()
}

func styleDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "---"):
		return theme.HealthStyle("unhealthy").Render(line)
	case strings.HasPrefix(line, "+++"):
		return theme.HealthStyle("healthy").Render(line)
	case strings.HasPrefix(line, "  +") || strings.HasPrefix(line, "    +"):
		return theme.HealthStyle("healthy").Render(line)
	case strings.HasPrefix(line, "  -") || strings.HasPrefix(line, "    -"):
		return theme.HealthStyle("unhealthy").Render(line)
	case strings.Contains(line, "→"):
		return theme.HealthStyle("deploying").Render(line)
	default:
		return line
	}
}

func (m TaskDefDiffModel) SetSize(w, h int) TaskDefDiffModel {
	m.width = w
	m.height = h
	return m
}
