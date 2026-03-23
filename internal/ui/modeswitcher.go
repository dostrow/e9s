package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/ui/theme"
)

// ModeSwitchSelectedMsg is sent when the user picks a mode.
type ModeSwitchSelectedMsg struct {
	Mode topMode
}

type ModeSwitcherModel struct {
	Active bool
	tabs   []ModeTab
	cursor int
	current topMode
}

// ModeSaveDefaultMsg is sent when the user wants to save the current mode as default.
type ModeSaveDefaultMsg struct {
	Mode topMode
}

func NewModeSwitcher(tabs []ModeTab, current topMode) ModeSwitcherModel {
	// Sort alphabetically by display name
	sorted := make([]ModeTab, len(tabs))
	copy(sorted, tabs)
	sort.Slice(sorted, func(i, j int) bool {
		return modeDisplayName(sorted[i].Mode) < modeDisplayName(sorted[j].Mode)
	})

	cursor := 0
	for i, t := range sorted {
		if t.Mode == current {
			cursor = i
			break
		}
	}
	return ModeSwitcherModel{
		Active:  true,
		tabs:    sorted,
		cursor:  cursor,
		current: current,
	}
}

func (m ModeSwitcherModel) Update(msg tea.Msg) (ModeSwitcherModel, tea.Cmd) {
	if !m.Active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.tabs)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			m.Active = false
			mode := m.tabs[m.cursor].Mode
			return m, func() tea.Msg {
				return ModeSwitchSelectedMsg{Mode: mode}
			}
		case "d":
			// Save current selection as default
			mode := m.tabs[m.cursor].Mode
			return m, func() tea.Msg {
				return ModeSaveDefaultMsg{Mode: mode}
			}
		case "esc", "`":
			m.Active = false
			return m, nil
		}
	}
	return m, nil
}

func (m ModeSwitcherModel) View() string {
	if !m.Active {
		return ""
	}

	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render("Switch Mode"))
	b.WriteString("\n\n")

	for i, t := range m.tabs {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "► "
			style = theme.SelectedRowStyle
		}

		label := modeDisplayName(t.Mode)
		indicator := "  "
		if t.Mode == m.current {
			indicator = lipgloss.NewStyle().Foreground(theme.ColorGreen).Render("● ")
		}

		b.WriteString(style.Render(fmt.Sprintf("%s%s%s", cursor, indicator, label)))
		b.WriteString("\n")
	}

	b.WriteString(theme.HelpStyle.Render("\n[enter] select  [d] set default  [esc] cancel"))

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorMauve).
		Padding(1, 3).
		Render(b.String())

	return box
}
