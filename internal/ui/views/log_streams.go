package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type LogStreamsModel struct {
	logGroup    string
	streams     []aws.LogStreamInfo
	selected    map[string]bool // multi-select by stream name
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewLogStreams(logGroup string) LogStreamsModel {
	return LogStreamsModel{logGroup: logGroup, selected: make(map[string]bool)}
}

func (m LogStreamsModel) Update(msg tea.Msg) (LogStreamsModel, tea.Cmd) {
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
			filtered := m.filteredStreams()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == " ":
			filtered := m.filteredStreams()
			if m.cursor < len(filtered) {
				name := filtered[m.cursor].Name
				if m.selected[name] {
					delete(m.selected, name)
				} else {
					m.selected[name] = true
				}
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredStreams()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter streams..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m LogStreamsModel) View() string {
	filtered := m.filteredStreams()
	var b strings.Builder

	title := fmt.Sprintf("  Log Streams (%d)", len(filtered))
	b.WriteString(theme.TitleStyle.Render(title))
	if len(m.selected) > 0 {
		b.WriteString(theme.HealthStyle("deploying").Render(fmt.Sprintf("  %d selected", len(m.selected))))
	}
	b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("  group: %s", m.logGroup)))
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
			b.WriteString(theme.HelpStyle.Render("  No log streams found"))
		}
		return b.String()
	}

	checkStyle := lipgloss.NewStyle().Foreground(theme.ColorGreen)

	tbl := components.NewTable([]components.Column{
		{Title: ""},
		{Title: "STREAM"},
		{Title: "LAST EVENT"},
	})

	for _, s := range filtered {
		lastEvent := "-"
		if s.LastEventTime > 0 {
			t := time.UnixMilli(s.LastEventTime)
			lastEvent = formatAge(t) + " ago"
		}
		check := " "
		if m.selected[s.Name] {
			check = checkStyle.Render("✓")
		}
		tbl.AddRow(
			components.Cell{Content: check},
			components.Plain(s.Name),
			components.Plain(lastEvent),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m LogStreamsModel) filteredStreams() []aws.LogStreamInfo {
	if m.filter == "" {
		return m.streams
	}
	lf := strings.ToLower(m.filter)
	var out []aws.LogStreamInfo
	for _, s := range m.streams {
		if strings.Contains(strings.ToLower(s.Name), lf) {
			out = append(out, s)
		}
	}
	return out
}

func (m LogStreamsModel) SetStreams(streams []aws.LogStreamInfo) LogStreamsModel {
	m.streams = streams
	m.loaded = true
	filtered := m.filteredStreams()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m LogStreamsModel) SelectedStream() *aws.LogStreamInfo {
	filtered := m.filteredStreams()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	s := filtered[m.cursor]
	return &s
}

// SelectedStreams returns all multi-selected stream names.
// If none are selected, returns just the cursor item.
func (m LogStreamsModel) SelectedStreams() []string {
	if len(m.selected) > 0 {
		names := make([]string, 0, len(m.selected))
		for name := range m.selected {
			names = append(names, name)
		}
		return names
	}
	if s := m.SelectedStream(); s != nil {
		return []string{s.Name}
	}
	return nil
}

// SelectionCount returns the number of multi-selected streams.
func (m LogStreamsModel) SelectionCount() int {
	return len(m.selected)
}

func (m LogStreamsModel) LogGroup() string {
	return m.logGroup
}

func (m LogStreamsModel) IsFiltering() bool {
	return m.filtering
}

func (m LogStreamsModel) visibleRows() int {
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

func (m LogStreamsModel) SetSize(w, h int) LogStreamsModel {
	m.width = w
	m.height = h
	return m
}
