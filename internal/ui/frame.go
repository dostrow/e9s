package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

// ModeTab represents an enabled module tab.
type ModeTab struct {
	Mode  topMode
	Label string
	Key   string
}

// renderFrame renders the full-screen bordered frame with info bar, content, action bar,
// and an optional scrollbar on the right edge of the content area.
// totalLines/visibleStart control the scrollbar; pass 0,0 to hide it.
func renderFrame(width, height int, infoBar, content, actionBar, modeLabel string) string {
	if width < 10 || height < 6 {
		return content
	}

	borderColor := theme.ColorDim
	hLine := lipgloss.NewStyle().Foreground(borderColor).Render("─")
	vLine := lipgloss.NewStyle().Foreground(borderColor).Render("│")
	tl := lipgloss.NewStyle().Foreground(borderColor).Render("╭")
	tr := lipgloss.NewStyle().Foreground(borderColor).Render("╮")
	bl := lipgloss.NewStyle().Foreground(borderColor).Render("╰")
	br := lipgloss.NewStyle().Foreground(borderColor).Render("╯")
	lT := lipgloss.NewStyle().Foreground(borderColor).Render("├")
	rT := lipgloss.NewStyle().Foreground(borderColor).Render("┤")

	innerWidth := width - 2 // subtract left and right borders

	var b strings.Builder

	// Top border: ╭──────────╮
	b.WriteString(tl + strings.Repeat(hLine, innerWidth) + tr + "\n")

	// Info bar: │ info... │
	infoPadded := padToWidth(" "+infoBar, innerWidth)
	b.WriteString(vLine + infoPadded + vLine + "\n")

	// Separator: ├──────────┤
	b.WriteString(lT + strings.Repeat(hLine, innerWidth) + rT + "\n")

	// Content area - fill available height
	// Height budget: top border(1) + info(1) + sep(1) + sep(1) + action(1) + bottom(1) = 6 chrome lines
	contentHeight := height - 6

	contentLines := strings.Split(content, "\n")
	totalLines := len(contentLines)
	hasScrollbar := totalLines > contentHeight && contentHeight > 2

	// Calculate scrollbar position
	var scrollStart, scrollLen int
	if hasScrollbar {
		scrollLen = max(1, contentHeight*contentHeight/totalLines)
		// Estimate scroll position from which lines are visible
		scrollStart = 0 // content is already windowed by the views
	}

	scrollTrack := lipgloss.NewStyle().Foreground(theme.ColorDim).Render("░")
	scrollThumb := lipgloss.NewStyle().Foreground(theme.ColorMauve).Render("█")

	for i := 0; i < contentHeight; i++ {
		line := ""
		if i < len(contentLines) {
			line = contentLines[i]
		}

		if hasScrollbar {
			padded := padToWidth(line, innerWidth-1)
			scrollChar := scrollTrack
			if i >= scrollStart && i < scrollStart+scrollLen {
				scrollChar = scrollThumb
			}
			b.WriteString(vLine + padded + scrollChar + vLine + "\n")
		} else {
			padded := padToWidth(line, innerWidth)
			b.WriteString(vLine + padded + vLine + "\n")
		}
	}

	// Separator: ├──────────┤
	b.WriteString(lT + strings.Repeat(hLine, innerWidth) + rT + "\n")

	// Action bar: │ actions... │
	actionPadded := padToWidth(" "+actionBar, innerWidth)
	b.WriteString(vLine + actionPadded + vLine + "\n")

	// Bottom border with mode label: ╰── ECS ──╯
	modeStr := ""
	if modeLabel != "" {
		prefix := lipgloss.NewStyle().Foreground(borderColor).Render("── ")
		label := lipgloss.NewStyle().Bold(true).Foreground(theme.ColorMauve).Render(modeLabel)
		suffix := lipgloss.NewStyle().Foreground(borderColor).Render(" ")
		modeStr = prefix + label + suffix
	}
	modeVisualWidth := lipgloss.Width(modeStr)
	remaining := innerWidth - modeVisualWidth
	if remaining < 0 {
		remaining = 0
	}
	b.WriteString(bl + modeStr + strings.Repeat(hLine, remaining) + br)

	return b.String()
}

// padToWidth pads or truncates a string to exactly the given visual width.
// Handles ANSI-styled text by measuring visual width with lipgloss.
func padToWidth(s string, width int) string {
	visual := lipgloss.Width(s)
	if visual == width {
		return s
	}
	if visual < width {
		return s + strings.Repeat(" ", width-visual)
	}
	// Truncate — work on plain text to find the cut point
	plain := stripAnsi(s)
	runes := []rune(plain)
	if len(runes) > width {
		return string(runes[:width])
	}
	return s
}

// buildInfoBar constructs the top info bar content.
func buildInfoBar(breadcrumbs []string, region string, lastRefresh time.Time, flashMessage string, flashExpiry time.Time, err error) string {
	left := "e9s"
	if len(breadcrumbs) > 0 {
		left += " ── " + strings.Join(breadcrumbs, " ► ")
	}
	if region != "" {
		left += fmt.Sprintf(" ── %s", region)
	}

	right := ""
	if flashMessage != "" && time.Now().Before(flashExpiry) {
		right = lipgloss.NewStyle().Foreground(theme.ColorGreen).Render(flashMessage)
	} else if err != nil {
		right = theme.ErrorStyle.Render(fmt.Sprintf("error: %s", err))
	} else if !lastRefresh.IsZero() {
		ago := time.Since(lastRefresh).Truncate(time.Second)
		right = fmt.Sprintf("↻ %s ago", ago)
	}

	return left + "  " + right
}

// modeDisplayName returns the full display name for a mode.
func modeDisplayName(mode topMode) string {
	names := map[topMode]string{
		modeECS:        "ECS",
		modeCloudWatch: "CloudWatch",
		modeSSM:        "SSM",
		modeSM:         "Secrets Manager",
		modeS3:         "S3",
		modeLambda:     "Lambda",
		modeDynamoDB:   "DynamoDB",
	}
	if name, ok := names[mode]; ok {
		return name
	}
	return "Unknown"
}

// modeShortName returns the short label for the bottom-left corner.
func modeShortName(mode topMode) string {
	names := map[topMode]string{
		modeECS:        "ECS",
		modeCloudWatch: "CW",
		modeSSM:        "SSM",
		modeSM:         "SM",
		modeS3:         "S3",
		modeLambda:     "λ",
		modeDynamoDB:   "DDB",
	}
	if name, ok := names[mode]; ok {
		return name
	}
	return ""
}
