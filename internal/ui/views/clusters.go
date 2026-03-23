// Package views implements the individual bubbletea view models for each screen.
package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type ClusterListModel struct {
	clusters    []model.Cluster
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
}

func NewClusterList() ClusterListModel {
	return ClusterListModel{}
}

func (m ClusterListModel) Init() tea.Cmd {
	return nil
}

func (m ClusterListModel) Update(msg tea.Msg) (ClusterListModel, tea.Cmd) {
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
			filtered := m.filteredClusters()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter clusters..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 50
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m ClusterListModel) View() string {
	filtered := m.filteredClusters()
	var b strings.Builder

	title := fmt.Sprintf("  Clusters (%d)", len(filtered))
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
		b.WriteString(theme.HelpStyle.Render("  No clusters found"))
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "#"},
		{Title: "NAME"},
		{Title: "STATUS"},
		{Title: "SERVICES", RightAlign: true},
		{Title: "RUNNING", RightAlign: true},
		{Title: "PENDING", RightAlign: true},
	})

	for i, c := range filtered {
		tbl.AddRow(
			components.Plain(fmt.Sprintf("%d", i+1)),
			components.Plain(c.Name),
			components.Styled(c.Status, theme.StatusStyle(c.Status)),
			components.Plain(fmt.Sprintf("%d", c.ActiveServices)),
			components.Plain(fmt.Sprintf("%d", c.RunningTasks)),
			components.Plain(fmt.Sprintf("%d", c.PendingTasks)),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m ClusterListModel) filteredClusters() []model.Cluster {
	if m.filter == "" {
		return m.clusters
	}
	lf := strings.ToLower(m.filter)
	var out []model.Cluster
	for _, c := range m.clusters {
		if strings.Contains(strings.ToLower(c.Name), lf) {
			out = append(out, c)
		}
	}
	return out
}

func (m ClusterListModel) SetClusters(clusters []model.Cluster) ClusterListModel {
	m.clusters = clusters
	filtered := m.filteredClusters()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m ClusterListModel) SelectedCluster() *model.Cluster {
	filtered := m.filteredClusters()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	c := filtered[m.cursor]
	return &c
}

// SelectIndex returns the item at the given index in the filtered list, or nil.
func (m ClusterListModel) SelectIndex(idx int) *model.Cluster {
	filtered := m.filteredClusters()
	if idx < 0 || idx >= len(filtered) {
		return nil
	}
	c := filtered[idx]
	return &c
}

// WithCursor returns a copy with the cursor set to the given index.
func (m ClusterListModel) WithCursor(idx int) ClusterListModel {
	m.cursor = idx
	return m
}

func (m ClusterListModel) IsFiltering() bool {
	return m.filtering
}

func (m ClusterListModel) visibleRows() int {
	// height minus: title, filter line, blank, top border, header, separator, bottom border, scroll indicator, help
	overhead := 9
	if m.filtering {
		overhead++
	}
	rows := m.height - overhead
	if rows < 5 {
		return 0 // no limit if too small to calculate
	}
	return rows
}

func (m ClusterListModel) SetSize(w, h int) ClusterListModel {
	m.width = w
	m.height = h
	return m
}
