package ui

import (
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

// ErrorDetailsModel presents the complete text of the most recent application
// error. It deliberately remains separate from view navigation so inspecting an
// error cannot disturb the user's current module or selection.
type ErrorDetailsModel struct {
	Active  bool
	Message string
	scroll  int
	width   int
	height  int
}

func NewErrorDetails(message string, width, height int) ErrorDetailsModel {
	return ErrorDetailsModel{
		Active:  true,
		Message: message,
		width:   width,
		height:  height,
	}
}

func (m ErrorDetailsModel) SetSize(width, height int) ErrorDetailsModel {
	m.width = width
	m.height = height
	return m
}

func (m ErrorDetailsModel) Update(msg tea.Msg) ErrorDetailsModel {
	if !m.Active {
		return m
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m
	}

	page := max(1, m.visibleRows()-1)
	switch key.String() {
	case "esc", "enter", "!":
		m.Active = false
	case "up", "k":
		m.scroll = max(0, m.scroll-1)
	case "down", "j":
		m.scroll++
	case "pgup":
		m.scroll = max(0, m.scroll-page)
	case "pgdown":
		m.scroll += page
	case "g":
		m.scroll = 0
	case "G":
		m.scroll = max(0, len(m.lines())-m.visibleRows())
	}
	return m
}

func (m ErrorDetailsModel) View() string {
	dialogWidth := min(100, max(24, m.width-8))
	lines := m.lines()
	visible := m.visibleRows()
	maxScroll := max(0, len(lines)-visible)
	start := min(max(0, m.scroll), maxScroll)
	end := min(len(lines), start+visible)

	var b strings.Builder
	b.WriteString(theme.ErrorStyle.Render("Error details"))
	b.WriteString("\n\n")
	for _, line := range lines[start:end] {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if len(lines) > visible {
		b.WriteString("\n")
		b.WriteString(theme.HelpStyle.Render("j/k or PgUp/PgDn scroll"))
		b.WriteString("  ")
	}
	b.WriteString(theme.HelpStyle.Render("[esc] close  [d] dismiss"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorRed).
		Padding(1, 3).
		Width(dialogWidth).
		Render(strings.TrimRight(b.String(), "\n"))
}

func (m ErrorDetailsModel) lines() []string {
	dialogWidth := min(100, max(24, m.width-8))
	return wrapErrorMessage(m.Message, max(12, dialogWidth-8))
}

func (m ErrorDetailsModel) visibleRows() int {
	// Leave room for the frame, modal border/padding, title, and controls.
	return max(1, min(18, m.height-12))
}

func wrapErrorMessage(message string, width int) []string {
	if message == "" {
		return []string{"(no error details available)"}
	}
	var result []string
	for _, paragraph := range strings.Split(message, "\n") {
		if paragraph == "" {
			result = append(result, "")
			continue
		}
		for utf8.RuneCountInString(paragraph) > width {
			runes := []rune(paragraph)
			cut := min(width, len(runes))
			for i := cut; i > 0; i-- {
				if runes[i-1] == ' ' || runes[i-1] == '\t' {
					cut = i - 1
					break
				}
			}
			if cut == 0 {
				cut = min(width, len(runes))
			}
			result = append(result, string(runes[:cut]))
			paragraph = strings.TrimLeft(string(runes[cut:]), " \t")
		}
		result = append(result, paragraph)
	}
	return result
}
