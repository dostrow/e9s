package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type ModeTab struct {
	Mode  topMode
	Label string
	Key   string // the number key to press
}

func RenderStatusBar(width int, active topMode, tabs []ModeTab, breadcrumbs []string, region string, lastRefresh time.Time, err error) string {
	return renderBar(width, active, tabs, breadcrumbs, region, "", err, lastRefresh)
}

func RenderStatusBarWithFlash(width int, active topMode, tabs []ModeTab, breadcrumbs []string, region, flash string) string {
	return renderBar(width, active, tabs, breadcrumbs, region, flash, nil, time.Time{})
}

func renderBar(width int, active topMode, tabs []ModeTab, breadcrumbs []string, region, flash string, err error, lastRefresh time.Time) string {
	style := lipgloss.NewStyle().
		Width(width).
		Foreground(theme.ColorDim).
		Padding(0, 1)

	left := "e9s " + renderModeTabs(active, tabs)

	if len(breadcrumbs) > 0 {
		left += " ── " + strings.Join(breadcrumbs, " ► ")
	}
	if region != "" {
		left += fmt.Sprintf(" ── region: %s", region)
	}

	right := ""
	if flash != "" {
		right = lipgloss.NewStyle().Foreground(theme.ColorGreen).Render(flash)
	} else if err != nil {
		right = theme.ErrorStyle.Render(fmt.Sprintf("error: %s", err))
	} else if !lastRefresh.IsZero() {
		ago := time.Since(lastRefresh).Truncate(time.Second)
		right = fmt.Sprintf("↻ %s ago", ago)
	}

	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right)-2)
	content := left + fmt.Sprintf("%*s", gap, "") + right

	return style.Render(content)
}

func renderModeTabs(active topMode, tabs []ModeTab) string {
	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.ColorMauve)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(theme.ColorDim)

	var b strings.Builder
	for i, t := range tabs {
		label := t.Key + ":" + t.Label
		if t.Mode == active {
			b.WriteString(activeStyle.Render("[" + label + "]"))
		} else {
			b.WriteString(inactiveStyle.Render("[" + label + "]"))
		}
		if i < len(tabs)-1 {
			b.WriteString(" ")
		}
	}
	return b.String()
}
