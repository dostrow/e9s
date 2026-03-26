package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/tofu"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type TofuPlanModel struct {
	dir         string
	plan        *tofu.PlanResult
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewTofuPlan(dir string) TofuPlanModel {
	return TofuPlanModel{dir: dir}
}

func (m TofuPlanModel) Update(msg tea.Msg) (TofuPlanModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filtering {
			switch msg.String() {
			case "enter":
				m.filter = m.filterInput.Value()
				m.filtering = false
				m.cursor = 0
				return m, nil
			case "esc":
				m.filtering = false
				return m, nil
			}
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			return m, cmd
		}
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			filtered := m.filteredChanges()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredChanges()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter changes..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 80
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m TofuPlanModel) View() string {
	var b strings.Builder

	dirLabel := m.dir
	if len(dirLabel) > 40 {
		dirLabel = "..." + dirLabel[len(dirLabel)-37:]
	}

	if !m.loaded {
		b.WriteString(theme.TitleStyle.Render(fmt.Sprintf("  Plan: %s", dirLabel)))
		b.WriteString("\n\n")
		b.WriteString(theme.HelpStyle.Render("  Running plan..."))
		return b.String()
	}

	if m.plan == nil {
		b.WriteString(theme.TitleStyle.Render(fmt.Sprintf("  Plan: %s", dirLabel)))
		b.WriteString("\n\n")
		b.WriteString(theme.HelpStyle.Render("  No plan results"))
		return b.String()
	}

	filtered := m.filteredChanges()

	// Summary header
	summary := tofu.FormatPlanSummary(m.plan)
	title := fmt.Sprintf("  Plan: %s", dirLabel)
	b.WriteString(theme.TitleStyle.Render(title))
	b.WriteString("  ")
	b.WriteString(planSummaryStyled(m.plan))
	if m.filter != "" {
		b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("  filter: %q", m.filter)))
	}
	_ = summary
	b.WriteString("\n")
	if m.filtering {
		b.WriteString("  / " + m.filterInput.View() + "\n")
	}
	b.WriteString("\n")

	if len(filtered) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true).Render("  No changes. Infrastructure is up-to-date."))
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "ACTION"},
		{Title: "RESOURCE"},
		{Title: "CHANGES"},
	})
	for _, c := range filtered {
		actionCell := actionStyledCell(c.Action)
		changeSummary := formatChangeSummary(c)
		tbl.AddRow(
			actionCell,
			components.Plain(c.Address),
			components.Plain(changeSummary),
		)
	}
	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func actionStyledCell(action string) components.Cell {
	var style lipgloss.Style
	switch action {
	case "create":
		style = lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
		return components.Styled("+ create", style)
	case "update":
		style = lipgloss.NewStyle().Foreground(theme.ColorYellow).Bold(true)
		return components.Styled("~ update", style)
	case "delete":
		style = lipgloss.NewStyle().Foreground(theme.ColorRed).Bold(true)
		return components.Styled("- delete", style)
	case "replace":
		style = lipgloss.NewStyle().Foreground(theme.ColorMauve).Bold(true)
		return components.Styled("-/+ replace", style)
	default:
		return components.Plain(action)
	}
}

func formatChangeSummary(c tofu.ResourceChange) string {
	if len(c.Diffs) == 0 {
		return ""
	}
	if c.Action == "create" || c.Action == "delete" {
		return fmt.Sprintf("%d attributes", len(c.Diffs))
	}
	// For updates, show first few changed attrs
	var changed []string
	for _, d := range c.Diffs {
		if d.Action == "change" {
			changed = append(changed, d.Path)
		} else if d.Action == "add" {
			changed = append(changed, "+"+d.Path)
		} else if d.Action == "remove" {
			changed = append(changed, "-"+d.Path)
		}
		if len(changed) >= 3 {
			break
		}
	}
	summary := strings.Join(changed, ", ")
	remaining := len(c.Diffs) - len(changed)
	if remaining > 0 {
		summary += fmt.Sprintf(" (+%d more)", remaining)
	}
	return summary
}

func planSummaryStyled(p *tofu.PlanResult) string {
	var parts []string
	if p.CreateCount > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true).
			Render(fmt.Sprintf("+%d", p.CreateCount)))
	}
	if p.UpdateCount > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.ColorYellow).Bold(true).
			Render(fmt.Sprintf("~%d", p.UpdateCount)))
	}
	if p.ReplaceCount > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.ColorMauve).Bold(true).
			Render(fmt.Sprintf("-/+%d", p.ReplaceCount)))
	}
	if p.DeleteCount > 0 {
		parts = append(parts, lipgloss.NewStyle().Foreground(theme.ColorRed).Bold(true).
			Render(fmt.Sprintf("-%d", p.DeleteCount)))
	}
	if len(parts) == 0 {
		return lipgloss.NewStyle().Foreground(theme.ColorGreen).Render("no changes")
	}
	return strings.Join(parts, " ")
}

func (m TofuPlanModel) filteredChanges() []tofu.ResourceChange {
	if m.plan == nil {
		return nil
	}
	if m.filter == "" {
		return m.plan.Changes
	}
	lf := strings.ToLower(m.filter)
	var out []tofu.ResourceChange
	for _, c := range m.plan.Changes {
		if strings.Contains(strings.ToLower(c.Address), lf) ||
			strings.Contains(strings.ToLower(c.Action), lf) ||
			strings.Contains(strings.ToLower(c.Type), lf) {
			out = append(out, c)
		}
	}
	return out
}

func (m TofuPlanModel) SetPlan(plan *tofu.PlanResult) TofuPlanModel {
	m.plan = plan
	m.loaded = true
	filtered := m.filteredChanges()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m TofuPlanModel) SelectedChange() *tofu.ResourceChange {
	filtered := m.filteredChanges()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	c := filtered[m.cursor]
	return &c
}

func (m TofuPlanModel) Plan() *tofu.PlanResult { return m.plan }
func (m TofuPlanModel) Dir() string            { return m.dir }
func (m TofuPlanModel) IsFiltering() bool      { return m.filtering }

func (m TofuPlanModel) visibleRows() int {
	overhead := 9
	if m.filtering {
		overhead++
	}
	rows := m.height - overhead
	if rows < 5 {
		return 0
	}
	return rows
}

func (m TofuPlanModel) SetSize(w, h int) TofuPlanModel {
	m.width = w
	m.height = h
	return m
}
