package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type HelpModel struct {
	Active bool
}

func (m HelpModel) View(width int) string {
	if !m.Active {
		return ""
	}

	keyStyle := lipgloss.NewStyle().Foreground(theme.ColorCyan).Bold(true).Width(14)
	descStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)

	lines := []struct{ key, desc string }{
		{"Navigation", ""},
		{"j/k, ↑/↓", "Move cursor up/down"},
		{"Enter", "Drill into selected item"},
		{"Esc", "Go back to parent view"},
		{"q", "Quit (or back if not at top)"},
		{"", ""},
		{"List Views", ""},
		{"/", "Filter/search"},
		{"R", "Refresh data"},
		{"1-9", "Quick-select by number"},
		{"", ""},
		{"Service List", ""},
		{"r", "Force new deployment"},
		{"s", "Scale service"},
		{"d", "Show deployment detail"},
		{"L", "Tail logs (all tasks)"},
		{"m", "CPU/memory metrics + alarms"},
		{"S", "Standalone tasks (workers)"},
		{"", ""},
		{"Task List", ""},
		{"l", "Tail logs for task"},
		{"x", "Stop task"},
		{"e", "ECS Exec (shell into container)"},
		{"", ""},
		{"Task Detail", ""},
		{"E", "View environment variables"},
		{"", ""},
		{"Log Viewer", ""},
		{"f", "Toggle follow mode"},
		{"t", "Toggle timestamp format"},
		{"/", "Filter log lines"},
		{"g/G", "Jump to top/bottom"},
		{"", ""},
		{"Service Detail", ""},
		{"D", "Task definition diff"},
		{"tab", "Switch tabs"},
		{"", ""},
		{"Log Groups/Streams", ""},
		{"l", "Tail selected stream/group"},
		{"L", "Tail entire log group"},
		{"s", "Search logs (time range)"},
		{"W", "Save log path"},
		{"", ""},
		{"Global", ""},
		{"`", "Switch mode"},
		{"ctrl+e", "Edit config"},
		{"ctrl+r", "Switch AWS region"},
		{"q", "Quit"},
		{"?", "Toggle this help"},
	}

	var b strings.Builder
	for _, l := range lines {
		if l.key == "" && l.desc == "" {
			b.WriteString("\n")
			continue
		}
		if l.desc == "" {
			b.WriteString(theme.TitleStyle.Render(l.key) + "\n")
			continue
		}
		b.WriteString(keyStyle.Render(l.key) + descStyle.Render(l.desc) + "\n")
	}
	content := b.String()

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorCyan).
		Padding(1, 3).
		Width(min(55, width-4)).
		Render(content)

	// Center horizontally
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, box)
}
