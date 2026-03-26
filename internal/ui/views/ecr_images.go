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

type ECRImagesModel struct {
	repoName    string
	repoURI     string
	images      []aws.ECRImage
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewECRImages(repoName, repoURI string) ECRImagesModel {
	return ECRImagesModel{repoName: repoName, repoURI: repoURI}
}

func (m ECRImagesModel) Update(msg tea.Msg) (ECRImagesModel, tea.Cmd) {
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
			filtered := m.filteredImages()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredImages()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter by tag or digest..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 80
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m ECRImagesModel) View() string {
	filtered := m.filteredImages()
	var b strings.Builder

	title := fmt.Sprintf("  Images: %s (%d)", m.repoName, len(filtered))
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
			b.WriteString(theme.HelpStyle.Render("  No images found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "TAGS"},
		{Title: "DIGEST"},
		{Title: "PUSHED"},
		{Title: "SIZE"},
		{Title: "SCAN"},
		{Title: "VULNERABILITIES"},
	})
	for _, img := range filtered {
		tags := "(untagged)"
		if len(img.Tags) > 0 {
			tags = strings.Join(img.Tags, ", ")
			if len(tags) > 30 {
				tags = tags[:27] + "..."
			}
		}
		digest := img.Digest
		if len(digest) > 19 {
			digest = digest[:19]
		}
		pushed := ""
		if !img.PushedAt.IsZero() {
			pushed = img.PushedAt.Local().Format("2006-01-02 15:04")
		}
		size := formatImageSize(img.SizeBytes)
		scanCell := scanStatusCell(img.ScanStatus)
		vulnCell := vulnSummaryCell(img.ScanSeverity)

		tbl.AddRow(
			components.Plain(tags),
			components.Plain(digest),
			components.Plain(pushed),
			components.Plain(size),
			scanCell,
			vulnCell,
		)
	}
	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func scanStatusCell(status string) components.Cell {
	switch status {
	case "COMPLETE":
		return components.Styled("done", lipgloss.NewStyle().Foreground(theme.ColorGreen))
	case "IN_PROGRESS", "PENDING":
		return components.Styled(status, lipgloss.NewStyle().Foreground(theme.ColorYellow))
	case "FAILED":
		return components.Styled("failed", lipgloss.NewStyle().Foreground(theme.ColorRed))
	case "":
		return components.Plain("-")
	default:
		return components.Plain(status)
	}
}

func vulnSummaryCell(counts map[string]int32) components.Cell {
	if len(counts) == 0 {
		return components.Plain("-")
	}
	var parts []string
	for _, sev := range []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"} {
		if c, ok := counts[sev]; ok && c > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", sev[:1], c))
		}
	}
	if len(parts) == 0 {
		return components.Styled("clean", lipgloss.NewStyle().Foreground(theme.ColorGreen))
	}
	summary := strings.Join(parts, " ")
	style := lipgloss.NewStyle().Foreground(theme.ColorYellow)
	if counts["CRITICAL"] > 0 {
		style = lipgloss.NewStyle().Foreground(theme.ColorRed).Bold(true)
	}
	return components.Styled(summary, style)
}

func formatImageSize(bytes int64) string {
	if bytes == 0 {
		return "-"
	}
	mb := float64(bytes) / (1024 * 1024)
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GB", mb/1024)
	}
	return fmt.Sprintf("%.1f MB", mb)
}

func (m ECRImagesModel) filteredImages() []aws.ECRImage {
	if m.filter == "" {
		return m.images
	}
	lf := strings.ToLower(m.filter)
	var out []aws.ECRImage
	for _, img := range m.images {
		if strings.Contains(strings.ToLower(img.Digest), lf) ||
			strings.Contains(strings.ToLower(strings.Join(img.Tags, " ")), lf) {
			out = append(out, img)
		}
	}
	return out
}

func (m ECRImagesModel) SetImages(images []aws.ECRImage) ECRImagesModel {
	m.images = images
	m.loaded = true
	filtered := m.filteredImages()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m ECRImagesModel) SelectedImage() *aws.ECRImage {
	filtered := m.filteredImages()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	img := filtered[m.cursor]
	return &img
}

func (m ECRImagesModel) RepoName() string  { return m.repoName }
func (m ECRImagesModel) RepoURI() string   { return m.repoURI }
func (m ECRImagesModel) IsFiltering() bool { return m.filtering }

func (m ECRImagesModel) visibleRows() int {
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

func (m ECRImagesModel) SetSize(w, h int) ECRImagesModel {
	m.width = w
	m.height = h
	return m
}
