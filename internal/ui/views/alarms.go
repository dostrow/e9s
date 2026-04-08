package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type AlarmsModel struct {
	alarms      []aws.CWAlarm
	stateFilter string
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	utcTime     bool
	width       int
	height      int
	loaded      bool
}

func NewAlarms(stateFilter string) AlarmsModel {
	return AlarmsModel{stateFilter: stateFilter}
}

func (m AlarmsModel) Update(msg tea.Msg) (AlarmsModel, tea.Cmd) {
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
			filtered := m.filteredAlarms()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredAlarms()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case msg.String() == "t":
			m.utcTime = !m.utcTime
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter alarms..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m AlarmsModel) View() string {
	filtered := m.filteredAlarms()
	var b strings.Builder

	title := fmt.Sprintf("  CloudWatch Alarms (%d)", len(filtered))
	if m.stateFilter != "" {
		title += fmt.Sprintf(" — state: %s", m.stateFilter)
	}
	b.WriteString(theme.TitleStyle.Render(title))
	if m.filter != "" {
		b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("  filter: %q", m.filter)))
	}
	b.WriteString("\n")
	if m.filtering {
		b.WriteString("  / " + m.filterInput.View() + "\n")
	}
	b.WriteString("\n")

	if len(filtered) == 0 {
		if !m.loaded {
			b.WriteString(theme.HelpStyle.Render("  Loading..."))
		} else {
			b.WriteString(theme.HelpStyle.Render("  No alarms found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "STATE"},
		{Title: "NAME"},
		{Title: "METRIC"},
		{Title: "NAMESPACE"},
		{Title: "UPDATED"},
	})
	for _, a := range filtered {
		stateCell := stateStyledCell(a.State)
		var ts string
		if m.utcTime {
			ts = a.StateUpdatedAt.UTC().Format("2006-01-02 15:04:05 UTC")
		} else {
			ts = a.StateUpdatedAt.Local().Format("2006-01-02 15:04:05")
		}
		tbl.AddRow(
			stateCell,
			components.Plain(a.Name),
			components.Plain(a.MetricName),
			components.Plain(a.Namespace),
			components.Plain(ts),
		)
	}
	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func stateStyledCell(state string) components.Cell {
	var style lipgloss.Style
	switch state {
	case "OK":
		style = lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	case "ALARM":
		style = lipgloss.NewStyle().Foreground(theme.ColorRed).Bold(true)
	default:
		style = lipgloss.NewStyle().Foreground(theme.ColorYellow)
	}
	return components.Styled(state, style)
}

func (m AlarmsModel) filteredAlarms() []aws.CWAlarm {
	if m.filter == "" {
		return m.alarms
	}
	lf := strings.ToLower(m.filter)
	var out []aws.CWAlarm
	for _, a := range m.alarms {
		if strings.Contains(strings.ToLower(a.Name), lf) ||
			strings.Contains(strings.ToLower(a.MetricName), lf) ||
			strings.Contains(strings.ToLower(a.Namespace), lf) {
			out = append(out, a)
		}
	}
	return out
}

func (m AlarmsModel) SetAlarms(alarms []aws.CWAlarm) AlarmsModel {
	sort.Slice(alarms, func(i, j int) bool {
		return alarms[i].StateUpdatedAt.After(alarms[j].StateUpdatedAt)
	})
	m.alarms = alarms
	m.loaded = true
	filtered := m.filteredAlarms()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m AlarmsModel) SelectedAlarm() *aws.CWAlarm {
	filtered := m.filteredAlarms()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	a := filtered[m.cursor]
	return &a
}

func (m AlarmsModel) IsFiltering() bool    { return m.filtering }
func (m AlarmsModel) StateFilter() string  { return m.stateFilter }

func (m AlarmsModel) visibleRows() int {
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

func (m AlarmsModel) SetSize(w, h int) AlarmsModel {
	m.width = w
	m.height = h
	return m
}
