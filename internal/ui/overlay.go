package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

// renderOverlay composites a modal dialog box centered over a dimmed background.
func renderOverlay(background, modal string, width, height int) string {
	// Build dimmed background lines
	bgLines := strings.Split(background, "\n")
	dimStyle := lipgloss.NewStyle().Foreground(theme.ColorDim)

	// Dim each background line
	dimmedLines := make([]string, height)
	for i := 0; i < height; i++ {
		if i < len(bgLines) {
			dimmedLines[i] = dimStyle.Render(stripAnsi(bgLines[i]))
		} else {
			dimmedLines[i] = ""
		}
		// Pad to full width
		w := lipgloss.Width(dimmedLines[i])
		if w < width {
			dimmedLines[i] += strings.Repeat(" ", width-w)
		}
	}

	// Measure modal
	modalLines := strings.Split(modal, "\n")
	modalHeight := len(modalLines)
	modalWidth := 0
	for _, line := range modalLines {
		if w := lipgloss.Width(line); w > modalWidth {
			modalWidth = w
		}
	}

	// Calculate centered position
	startRow := (height - modalHeight) / 2
	startCol := (width - modalWidth) / 2
	if startRow < 1 {
		startRow = 1
	}
	if startCol < 1 {
		startCol = 1
	}

	// Composite modal over background
	for i, mLine := range modalLines {
		row := startRow + i
		if row >= height {
			break
		}

		bg := dimmedLines[row]
		bgRunes := []rune(stripAnsi(bg))

		// Build the composited line: background prefix + modal + background suffix
		prefix := ""
		if startCol > 0 && startCol < len(bgRunes) {
			prefix = dimStyle.Render(string(bgRunes[:startCol]))
		} else {
			prefix = strings.Repeat(" ", startCol)
		}

		mWidth := lipgloss.Width(mLine)
		suffixStart := startCol + mWidth
		suffix := ""
		if suffixStart < len(bgRunes) {
			suffix = dimStyle.Render(string(bgRunes[suffixStart:]))
		}

		dimmedLines[row] = prefix + mLine + suffix
	}

	return strings.Join(dimmedLines, "\n")
}

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	var result strings.Builder
	inEsc := false
	for _, r := range s {
		if r == '\033' {
			inEsc = true
			continue
		}
		if inEsc {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEsc = false
			}
			continue
		}
		result.WriteRune(r)
	}
	return result.String()
}
