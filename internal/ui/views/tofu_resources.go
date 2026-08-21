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

type TofuResource = tofu.Resource

type TofuResourcesModel struct {
	dir         string
	resources   []tofu.Resource
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewTofuResources(dir string) TofuResourcesModel {
	return TofuResourcesModel{dir: dir}
}

func (m TofuResourcesModel) Update(msg tea.Msg) (TofuResourcesModel, tea.Cmd) {
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
			filtered := m.filteredResources()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredResources()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter resources..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m TofuResourcesModel) View() string {
	filtered := m.filteredResources()
	var b strings.Builder

	dirLabel := m.dir
	if len(dirLabel) > 40 {
		dirLabel = "..." + dirLabel[len(dirLabel)-37:]
	}
	title := fmt.Sprintf("  State: %s (%d resources)", dirLabel, len(filtered))
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
			b.WriteString(theme.HelpStyle.Render("  No resources in state"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "TYPE"},
		{Title: "NAME"},
		{Title: "MODULE"},
	})
	for _, r := range filtered {
		typeStyle := resourceTypeStyle(r.Type)
		tbl.AddRow(
			components.Styled(r.Type, typeStyle),
			components.Plain(r.Name),
			components.Plain(r.Module),
		)
	}
	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func resourceTypeStyle(t string) lipgloss.Style {
	if strings.HasPrefix(t, "aws_") {
		return lipgloss.NewStyle().Foreground(theme.ColorCyan)
	}
	if strings.HasPrefix(t, "data.") {
		return lipgloss.NewStyle().Foreground(theme.ColorDim)
	}
	return lipgloss.NewStyle().Foreground(theme.ColorWhite)
}

func (m TofuResourcesModel) filteredResources() []TofuResource {
	if m.filter == "" {
		return m.resources
	}
	lf := strings.ToLower(m.filter)
	var out []TofuResource
	for _, r := range m.resources {
		if strings.Contains(strings.ToLower(r.Address), lf) {
			out = append(out, r)
		}
	}
	return out
}

func (m TofuResourcesModel) SetResources(resources []tofu.Resource) TofuResourcesModel {
	m.resources = append([]tofu.Resource(nil), resources...)
	m.loaded = true
	filtered := m.filteredResources()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m TofuResourcesModel) SelectedResource() *tofu.Resource {
	filtered := m.filteredResources()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	r := filtered[m.cursor]
	return &r
}

func (m TofuResourcesModel) Dir() string       { return m.dir }
func (m TofuResourcesModel) IsFiltering() bool { return m.filtering }

func (m TofuResourcesModel) visibleRows() int {
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

func (m TofuResourcesModel) SetSize(w, h int) TofuResourcesModel {
	m.width = w
	m.height = h
	return m
}
