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

type RDSClustersModel struct {
	clusters    []model.RDSCluster
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewRDSClusters() RDSClustersModel { return RDSClustersModel{} }

func (m RDSClustersModel) Update(msg tea.Msg) (RDSClustersModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if m.filtering {
			switch keyMsg.String() {
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
			m.filterInput, cmd = m.filterInput.Update(keyMsg)
			return m, cmd
		}
		switch {
		case key.Matches(keyMsg, theme.Keys.Up):
			m.cursor = max(0, m.cursor-1)
		case key.Matches(keyMsg, theme.Keys.Down):
			m.cursor = min(m.cursor+1, max(0, len(m.filteredClusters())-1))
		case keyMsg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case keyMsg.String() == "pgdown":
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(m.filteredClusters())-1))
		case key.Matches(keyMsg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter clusters..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m RDSClustersModel) View() string {
	clusters := m.filteredClusters()
	var b strings.Builder
	b.WriteString(theme.TitleStyle.Render(fmt.Sprintf("  RDS Clusters (%d)", len(clusters))))
	b.WriteString(theme.HelpStyle.Render("  [tab] instances"))
	if m.filter != "" {
		b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("  filter: %q", m.filter)))
	}
	b.WriteString("\n")
	if m.filtering {
		b.WriteString("  / " + m.filterInput.View() + "\n")
	}
	b.WriteString("\n")
	if len(clusters) == 0 {
		if !m.loaded {
			b.WriteString(theme.HelpStyle.Render("  Loading..."))
		} else {
			b.WriteString(theme.HelpStyle.Render("  No RDS clusters found"))
		}
		return b.String()
	}
	table := components.NewTable([]components.Column{
		{Title: "IDENTIFIER"}, {Title: "ENGINE"}, {Title: "STATUS"}, {Title: "MEMBERS"},
		{Title: "WRITER"}, {Title: "ENDPOINT"}, {Title: "CREATED"},
	})
	for _, cluster := range clusters {
		writer := ""
		for _, member := range cluster.Members {
			if member.Writer {
				writer = member.Identifier
				break
			}
		}
		engine := strings.TrimSpace(cluster.Engine + " " + cluster.Version)
		created := ""
		if !cluster.Created.IsZero() {
			created = formatAge(cluster.Created) + " ago"
		}
		table.AddRow(components.Plain(cluster.Identifier), components.Plain(engine),
			components.Styled(cluster.Status, rdsStatusStyle(cluster.Status)), components.Plain(fmt.Sprint(len(cluster.Members))),
			components.Plain(writer), components.Plain(cluster.Endpoint), components.Plain(created))
	}
	b.WriteString(table.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m RDSClustersModel) filteredClusters() []model.RDSCluster {
	if m.filter == "" {
		return m.clusters
	}
	filter := strings.ToLower(m.filter)
	result := make([]model.RDSCluster, 0, len(m.clusters))
	for _, cluster := range m.clusters {
		if strings.Contains(strings.ToLower(cluster.Identifier), filter) || strings.Contains(strings.ToLower(cluster.Engine), filter) ||
			strings.Contains(strings.ToLower(cluster.Status), filter) || strings.Contains(strings.ToLower(cluster.Endpoint), filter) {
			result = append(result, cluster)
		}
	}
	return result
}

func (m RDSClustersModel) SetClusters(clusters []model.RDSCluster) RDSClustersModel {
	m.clusters = clusters
	m.loaded = true
	if filtered := m.filteredClusters(); m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m RDSClustersModel) SelectedCluster() *model.RDSCluster {
	clusters := m.filteredClusters()
	if len(clusters) == 0 || m.cursor >= len(clusters) {
		return nil
	}
	cluster := clusters[m.cursor]
	return &cluster
}

func (m RDSClustersModel) IsFiltering() bool { return m.filtering }

func (m RDSClustersModel) visibleRows() int {
	rows := m.height - 9
	if m.filtering {
		rows--
	}
	return max(0, rows)
}

func (m RDSClustersModel) SetSize(width, height int) RDSClustersModel {
	m.width, m.height = width, height
	return m
}
