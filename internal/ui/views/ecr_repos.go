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

type ECRReposModel struct {
	repos       []aws.ECRRepo
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewECRRepos() ECRReposModel {
	return ECRReposModel{}
}

func (m ECRReposModel) Update(msg tea.Msg) (ECRReposModel, tea.Cmd) {
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
			filtered := m.filteredRepos()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredRepos()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter repos..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m ECRReposModel) View() string {
	filtered := m.filteredRepos()
	var b strings.Builder

	title := fmt.Sprintf("  ECR Repositories (%d)", len(filtered))
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
			b.WriteString(theme.HelpStyle.Render("  No repositories found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "REPOSITORY"},
		{Title: "SCAN"},
		{Title: "TAGS"},
		{Title: "ENCRYPTION"},
	})
	for _, r := range filtered {
		scanStyle := lipgloss.NewStyle().Foreground(theme.ColorDim)
		scanLabel := "off"
		if r.ScanOnPush {
			scanStyle = lipgloss.NewStyle().Foreground(theme.ColorGreen)
			scanLabel = "on-push"
		}
		mutability := r.TagMutability
		mutStyle := lipgloss.NewStyle()
		if mutability == "IMMUTABLE" {
			mutStyle = mutStyle.Foreground(theme.ColorCyan)
		}
		tbl.AddRow(
			components.Plain(r.Name),
			components.Styled(scanLabel, scanStyle),
			components.Styled(mutability, mutStyle),
			components.Plain(r.EncryptionType),
		)
	}
	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m ECRReposModel) filteredRepos() []aws.ECRRepo {
	if m.filter == "" {
		return m.repos
	}
	lf := strings.ToLower(m.filter)
	var out []aws.ECRRepo
	for _, r := range m.repos {
		if strings.Contains(strings.ToLower(r.Name), lf) {
			out = append(out, r)
		}
	}
	return out
}

func (m ECRReposModel) SetRepos(repos []aws.ECRRepo) ECRReposModel {
	m.repos = repos
	m.loaded = true
	filtered := m.filteredRepos()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m ECRReposModel) SelectedRepo() *aws.ECRRepo {
	filtered := m.filteredRepos()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	r := filtered[m.cursor]
	return &r
}

func (m ECRReposModel) IsFiltering() bool { return m.filtering }

func (m ECRReposModel) visibleRows() int {
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

func (m ECRReposModel) SetSize(w, h int) ECRReposModel {
	m.width = w
	m.height = h
	return m
}
