package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/tofu"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type TofuPlanDetailModel struct {
	change *tofu.ResourceChange
	scroll int
	width  int
	height int
}

func NewTofuPlanDetail(change *tofu.ResourceChange) TofuPlanDetailModel {
	return TofuPlanDetailModel{change: change}
}

func (m TofuPlanDetailModel) Update(msg tea.Msg) (TofuPlanDetailModel, tea.Cmd) {
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

func (m TofuPlanDetailModel) View() string {
	if m.change == nil {
		return theme.HelpStyle.Render("  No change selected")
	}

	c := m.change
	var lines []string

	// Title with action
	actionStyle := actionLabelStyle(c.Action)
	lines = append(lines, theme.TitleStyle.Render(fmt.Sprintf("  %s %s", actionStyle.Render(c.Action), c.Address)))
	lines = append(lines, "")

	// Metadata
	lines = append(lines, fmt.Sprintf("  %-18s %s", "Type:", c.Type))
	lines = append(lines, fmt.Sprintf("  %-18s %s", "Name:", c.Name))
	if c.Module != "" {
		lines = append(lines, fmt.Sprintf("  %-18s %s", "Module:", c.Module))
	}
	lines = append(lines, fmt.Sprintf("  %-18s %d", "Changes:", len(c.Diffs)))
	lines = append(lines, "")

	// Attribute diffs
	if len(c.Diffs) > 0 {
		lines = append(lines, theme.TitleStyle.Render("  Attribute Changes"))
		lines = append(lines, "")

		addStyle := lipgloss.NewStyle().Foreground(theme.ColorGreen)
		removeStyle := lipgloss.NewStyle().Foreground(theme.ColorRed)
		changeLabel := lipgloss.NewStyle().Foreground(theme.ColorYellow)
		dimStyle := lipgloss.NewStyle().Foreground(theme.ColorDim)

		for _, d := range c.Diffs {
			switch d.Action {
			case "add":
				lines = append(lines, fmt.Sprintf("  %s %s",
					addStyle.Render("+"),
					addStyle.Render(fmt.Sprintf("%-30s = %s", d.Path, d.After))))
			case "remove":
				lines = append(lines, fmt.Sprintf("  %s %s",
					removeStyle.Render("-"),
					removeStyle.Render(fmt.Sprintf("%-30s = %s", d.Path, d.Before))))
			case "change":
				lines = append(lines, fmt.Sprintf("  %s %s",
					changeLabel.Render("~"),
					changeLabel.Render(d.Path)))
				before := d.Before
				after := d.After
				// Multi-line values get indented
				if len(before) > 60 || len(after) > 60 {
					lines = append(lines, fmt.Sprintf("      %s %s",
						removeStyle.Render("-"), dimStyle.Render(before)))
					lines = append(lines, fmt.Sprintf("      %s %s",
						addStyle.Render("+"), after))
				} else {
					lines = append(lines, fmt.Sprintf("      %s → %s",
						dimStyle.Render(before), after))
				}
			}
		}
	}

	// Scrolling
	visRows := m.visibleRows()
	if m.scroll > len(lines)-visRows {
		m.scroll = max(0, len(lines)-visRows)
	}
	end := min(m.scroll+visRows, len(lines))
	visible := lines[m.scroll:end]

	return strings.Join(visible, "\n")
}

func actionLabelStyle(action string) lipgloss.Style {
	switch action {
	case "create":
		return lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	case "update":
		return lipgloss.NewStyle().Foreground(theme.ColorYellow).Bold(true)
	case "delete":
		return lipgloss.NewStyle().Foreground(theme.ColorRed).Bold(true)
	case "replace":
		return lipgloss.NewStyle().Foreground(theme.ColorMauve).Bold(true)
	default:
		return lipgloss.NewStyle()
	}
}

func (m TofuPlanDetailModel) Change() *tofu.ResourceChange { return m.change }

func (m TofuPlanDetailModel) visibleRows() int {
	rows := m.height - 2
	if rows < 5 {
		return 20
	}
	return rows
}

func (m TofuPlanDetailModel) SetSize(w, h int) TofuPlanDetailModel {
	m.width = w
	m.height = h
	return m
}
