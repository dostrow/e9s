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

type R53RecordsModel struct {
	zoneName    string
	zoneID      string
	records     []aws.R53Record
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewR53Records(zoneName, zoneID string) R53RecordsModel {
	return R53RecordsModel{zoneName: zoneName, zoneID: zoneID}
}

func (m R53RecordsModel) Update(msg tea.Msg) (R53RecordsModel, tea.Cmd) {
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
			filtered := m.filteredRecords()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredRecords()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter records..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 80
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m R53RecordsModel) View() string {
	filtered := m.filteredRecords()
	var b strings.Builder

	title := fmt.Sprintf("  Records: %s (%d)", m.zoneName, len(filtered))
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
			b.WriteString(theme.HelpStyle.Render("  No records found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "NAME"},
		{Title: "TYPE"},
		{Title: "TTL", RightAlign: true},
		{Title: "VALUE"},
		{Title: "ROUTING"},
	})
	for _, r := range filtered {
		typeStyle := recordTypeStyle(r.Type)
		value := recordSummaryValue(r)
		ttl := ""
		if r.TTL > 0 {
			ttl = fmt.Sprintf("%d", r.TTL)
		}
		routing := r.RoutingPolicy
		if routing == "Simple" {
			routing = ""
		}
		tbl.AddRow(
			components.Plain(r.Name),
			components.Styled(r.Type, typeStyle),
			components.Plain(ttl),
			components.Plain(value),
			components.Plain(routing),
		)
	}
	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func recordTypeStyle(t string) lipgloss.Style {
	switch t {
	case "A", "AAAA":
		return lipgloss.NewStyle().Foreground(theme.ColorGreen).Bold(true)
	case "CNAME":
		return lipgloss.NewStyle().Foreground(theme.ColorCyan)
	case "MX":
		return lipgloss.NewStyle().Foreground(theme.ColorYellow)
	case "NS", "SOA":
		return lipgloss.NewStyle().Foreground(theme.ColorDim)
	default:
		return lipgloss.NewStyle().Foreground(theme.ColorWhite)
	}
}

func recordSummaryValue(r aws.R53Record) string {
	if r.AliasTarget != "" {
		return "ALIAS → " + r.AliasTarget
	}
	if len(r.Values) == 0 {
		return ""
	}
	if len(r.Values) == 1 {
		v := r.Values[0]
		if len(v) > 50 {
			return v[:47] + "..."
		}
		return v
	}
	first := r.Values[0]
	if len(first) > 35 {
		first = first[:32] + "..."
	}
	return fmt.Sprintf("%s (+%d more)", first, len(r.Values)-1)
}

func (m R53RecordsModel) filteredRecords() []aws.R53Record {
	if m.filter == "" {
		return m.records
	}
	lf := strings.ToLower(m.filter)
	var out []aws.R53Record
	for _, r := range m.records {
		if strings.Contains(strings.ToLower(r.Name), lf) ||
			strings.Contains(strings.ToLower(r.Type), lf) ||
			strings.Contains(strings.ToLower(strings.Join(r.Values, " ")), lf) {
			out = append(out, r)
		}
	}
	return out
}

func (m R53RecordsModel) SetRecords(records []aws.R53Record) R53RecordsModel {
	m.records = records
	m.loaded = true
	filtered := m.filteredRecords()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m R53RecordsModel) SelectedRecord() *aws.R53Record {
	filtered := m.filteredRecords()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	r := filtered[m.cursor]
	return &r
}

func (m R53RecordsModel) ZoneName() string    { return m.zoneName }
func (m R53RecordsModel) ZoneID() string      { return m.zoneID }
func (m R53RecordsModel) IsFiltering() bool   { return m.filtering }

func (m R53RecordsModel) visibleRows() int {
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

func (m R53RecordsModel) SetSize(w, h int) R53RecordsModel {
	m.width = w
	m.height = h
	return m
}
