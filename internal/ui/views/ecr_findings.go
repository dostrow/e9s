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

type ECRFindingsModel struct {
	repoName    string
	imageDigest string
	imageTags   []string
	findings    []aws.ECRFinding
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewECRFindings(repoName, imageDigest string, imageTags []string) ECRFindingsModel {
	return ECRFindingsModel{repoName: repoName, imageDigest: imageDigest, imageTags: imageTags}
}

func (m ECRFindingsModel) Update(msg tea.Msg) (ECRFindingsModel, tea.Cmd) {
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
			filtered := m.filteredFindings()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredFindings()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter findings..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 80
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m ECRFindingsModel) View() string {
	filtered := m.filteredFindings()
	var b strings.Builder

	// Title with severity summary
	tagLabel := "(untagged)"
	if len(m.imageTags) > 0 {
		tagLabel = m.imageTags[0]
	}
	title := fmt.Sprintf("  Scan Findings: %s:%s (%d)", m.repoName, tagLabel, len(filtered))
	b.WriteString(theme.TitleStyle.Render(title))

	// Severity summary
	counts := m.severityCounts(filtered)
	if len(counts) > 0 {
		b.WriteString("  ")
		b.WriteString(counts)
	}

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
			b.WriteString(theme.HelpStyle.Render("  No findings — image is clean"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "SEVERITY"},
		{Title: "CVE"},
		{Title: "PACKAGE"},
		{Title: "VERSION"},
		{Title: "DESCRIPTION"},
	})
	for _, f := range filtered {
		sevCell := severityCell(f.Severity)
		desc := f.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		tbl.AddRow(
			sevCell,
			components.Plain(f.Name),
			components.Plain(f.Package),
			components.Plain(f.Version),
			components.Plain(desc),
		)
	}
	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func severityCell(severity string) components.Cell {
	var style lipgloss.Style
	switch severity {
	case "CRITICAL":
		style = lipgloss.NewStyle().Foreground(theme.ColorRed).Bold(true)
	case "HIGH":
		style = lipgloss.NewStyle().Foreground(theme.ColorRed)
	case "MEDIUM":
		style = lipgloss.NewStyle().Foreground(theme.ColorYellow)
	case "LOW":
		style = lipgloss.NewStyle().Foreground(theme.ColorCyan)
	case "INFORMATIONAL":
		style = lipgloss.NewStyle().Foreground(theme.ColorDim)
	default:
		style = lipgloss.NewStyle().Foreground(theme.ColorDim)
	}
	return components.Styled(severity, style)
}

func (m ECRFindingsModel) severityCounts(findings []aws.ECRFinding) string {
	counts := make(map[string]int)
	for _, f := range findings {
		counts[f.Severity]++
	}
	var parts []string
	for _, sev := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW", "INFORMATIONAL"} {
		if c := counts[sev]; c > 0 {
			var style lipgloss.Style
			switch sev {
			case "CRITICAL":
				style = lipgloss.NewStyle().Foreground(theme.ColorRed).Bold(true)
			case "HIGH":
				style = lipgloss.NewStyle().Foreground(theme.ColorRed)
			case "MEDIUM":
				style = lipgloss.NewStyle().Foreground(theme.ColorYellow)
			default:
				style = lipgloss.NewStyle().Foreground(theme.ColorDim)
			}
			parts = append(parts, style.Render(fmt.Sprintf("%s:%d", sev[:1], c)))
		}
	}
	return strings.Join(parts, " ")
}

func (m ECRFindingsModel) filteredFindings() []aws.ECRFinding {
	if m.filter == "" {
		return m.findings
	}
	lf := strings.ToLower(m.filter)
	var out []aws.ECRFinding
	for _, f := range m.findings {
		if strings.Contains(strings.ToLower(f.Name), lf) ||
			strings.Contains(strings.ToLower(f.Severity), lf) ||
			strings.Contains(strings.ToLower(f.Package), lf) ||
			strings.Contains(strings.ToLower(f.Description), lf) {
			out = append(out, f)
		}
	}
	return out
}

func (m ECRFindingsModel) SetFindings(findings []aws.ECRFinding) ECRFindingsModel {
	m.findings = findings
	m.loaded = true
	filtered := m.filteredFindings()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m ECRFindingsModel) SelectedFinding() *aws.ECRFinding {
	filtered := m.filteredFindings()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	f := filtered[m.cursor]
	return &f
}

func (m ECRFindingsModel) RepoName() string    { return m.repoName }
func (m ECRFindingsModel) ImageDigest() string  { return m.imageDigest }
func (m ECRFindingsModel) IsFiltering() bool    { return m.filtering }

func (m ECRFindingsModel) visibleRows() int {
	overhead := 10
	if m.filtering {
		overhead++
	}
	rows := m.height - overhead
	if rows < 5 {
		return 0
	}
	return rows
}

func (m ECRFindingsModel) SetSize(w, h int) ECRFindingsModel {
	m.width = w
	m.height = h
	return m
}
