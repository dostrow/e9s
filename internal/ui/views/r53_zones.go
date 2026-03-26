package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type R53ZonesModel struct {
	zones       []aws.R53Zone
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewR53Zones() R53ZonesModel {
	return R53ZonesModel{}
}

func (m R53ZonesModel) Update(msg tea.Msg) (R53ZonesModel, tea.Cmd) {
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
			filtered := m.filteredZones()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredZones()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter zones..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 80
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m R53ZonesModel) View() string {
	filtered := m.filteredZones()
	var b strings.Builder

	title := fmt.Sprintf("  Route53 Hosted Zones (%d)", len(filtered))
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
			b.WriteString(theme.HelpStyle.Render("  No hosted zones found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "ZONE NAME"},
		{Title: "TYPE"},
		{Title: "RECORDS", RightAlign: true},
		{Title: "COMMENT"},
	})
	for _, z := range filtered {
		zoneType := "Public"
		typeStyle := lipgloss.NewStyle().Foreground(theme.ColorGreen)
		if z.Private {
			zoneType = "Private"
			typeStyle = lipgloss.NewStyle().Foreground(theme.ColorCyan)
		}
		comment := z.Comment
		if len(comment) > 40 {
			comment = comment[:37] + "..."
		}
		tbl.AddRow(
			components.Plain(z.Name),
			components.Styled(zoneType, typeStyle),
			components.Plain(fmt.Sprintf("%d", z.RecordCount)),
			components.Plain(comment),
		)
	}
	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m R53ZonesModel) filteredZones() []aws.R53Zone {
	if m.filter == "" {
		return m.zones
	}
	lf := strings.ToLower(m.filter)
	var out []aws.R53Zone
	for _, z := range m.zones {
		if strings.Contains(strings.ToLower(z.Name), lf) ||
			strings.Contains(strings.ToLower(z.Comment), lf) {
			out = append(out, z)
		}
	}
	return out
}

func (m R53ZonesModel) SetZones(zones []aws.R53Zone) R53ZonesModel {
	m.zones = zones
	m.loaded = true
	filtered := m.filteredZones()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m R53ZonesModel) SelectedZone() *aws.R53Zone {
	filtered := m.filteredZones()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	z := filtered[m.cursor]
	return &z
}

func (m R53ZonesModel) IsFiltering() bool { return m.filtering }

func (m R53ZonesModel) visibleRows() int {
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

func (m R53ZonesModel) SetSize(w, h int) R53ZonesModel {
	m.width = w
	m.height = h
	return m
}
