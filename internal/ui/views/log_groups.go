package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type LogGroupsModel struct {
	groups      []aws.LogGroupInfo
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
}

func NewLogGroups() LogGroupsModel {
	return LogGroupsModel{}
}

func (m LogGroupsModel) Update(msg tea.Msg) (LogGroupsModel, tea.Cmd) {
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
			filtered := m.filteredGroups()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter log groups..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 100
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m LogGroupsModel) View() string {
	filtered := m.filteredGroups()
	var b strings.Builder

	title := fmt.Sprintf("  Log Groups (%d)", len(filtered))
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
		b.WriteString(theme.HelpStyle.Render("  No log groups found"))
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "LOG GROUP"},
		{Title: "SIZE", RightAlign: true},
	})

	for _, g := range filtered {
		tbl.AddRow(
			components.Plain(g.Name),
			components.Plain(formatBytes(g.StoredBytes)),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m LogGroupsModel) filteredGroups() []aws.LogGroupInfo {
	if m.filter == "" {
		return m.groups
	}
	lf := strings.ToLower(m.filter)
	var out []aws.LogGroupInfo
	for _, g := range m.groups {
		if strings.Contains(strings.ToLower(g.Name), lf) {
			out = append(out, g)
		}
	}
	return out
}

func (m LogGroupsModel) SetGroups(groups []aws.LogGroupInfo) LogGroupsModel {
	m.groups = groups
	filtered := m.filteredGroups()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m LogGroupsModel) SelectedGroup() *aws.LogGroupInfo {
	filtered := m.filteredGroups()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	g := filtered[m.cursor]
	return &g
}

func (m LogGroupsModel) HasData() bool {
	return len(m.groups) > 0
}

func (m LogGroupsModel) IsFiltering() bool {
	return m.filtering
}

func (m LogGroupsModel) visibleRows() int {
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

func (m LogGroupsModel) SetSize(w, h int) LogGroupsModel {
	m.width = w
	m.height = h
	return m
}

func formatBytes(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
