package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type HelpModel struct {
	Active bool
}

// RenderHelp renders a context-sensitive help overlay from the provided keybinding lines.
func RenderHelp(lines []struct{ key, desc string }, width int) string {
	keyStyle := lipgloss.NewStyle().Foreground(theme.ColorCyan).Bold(true).Width(14)
	descStyle := lipgloss.NewStyle().Foreground(theme.ColorWhite)

	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render("Keyboard Shortcuts") + "\n\n")

	for _, l := range lines {
		if l.key == "" && l.desc == "" {
			b.WriteString("\n")
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

	return box
}
