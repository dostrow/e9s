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

type S3BucketsModel struct {
	buckets     []aws.S3Bucket
	searchTerm  string
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewS3Buckets(searchTerm string) S3BucketsModel {
	return S3BucketsModel{searchTerm: searchTerm}
}

func (m S3BucketsModel) Update(msg tea.Msg) (S3BucketsModel, tea.Cmd) {
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
			filtered := m.filteredBuckets()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredBuckets()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter buckets..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.CharLimit = 50
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m S3BucketsModel) View() string {
	filtered := m.filteredBuckets()
	var b strings.Builder

	title := fmt.Sprintf("  S3 Buckets (%d)", len(filtered))
	if m.searchTerm != "" {
		title += fmt.Sprintf(" — search: %s", m.searchTerm)
	}
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
			b.WriteString(theme.HelpStyle.Render("  No buckets found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "BUCKET"},
		{Title: "CREATED"},
	})

	for _, bkt := range filtered {
		created := ""
		if !bkt.CreatedAt.IsZero() {
			created = bkt.CreatedAt.Format("2006-01-02")
		}
		tbl.AddRow(
			components.Plain(bkt.Name),
			components.Plain(created),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m S3BucketsModel) filteredBuckets() []aws.S3Bucket {
	if m.filter == "" {
		return m.buckets
	}
	lf := strings.ToLower(m.filter)
	var out []aws.S3Bucket
	for _, bkt := range m.buckets {
		if strings.Contains(strings.ToLower(bkt.Name), lf) {
			out = append(out, bkt)
		}
	}
	return out
}

func (m S3BucketsModel) SetBuckets(buckets []aws.S3Bucket) S3BucketsModel {
	m.buckets = buckets
	m.loaded = true
	filtered := m.filteredBuckets()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m S3BucketsModel) SelectedBucket() *aws.S3Bucket {
	filtered := m.filteredBuckets()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	bkt := filtered[m.cursor]
	return &bkt
}

func (m S3BucketsModel) SearchTerm() string  { return m.searchTerm }
func (m S3BucketsModel) IsFiltering() bool    { return m.filtering }

func (m S3BucketsModel) visibleRows() int {
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

func (m S3BucketsModel) SetSize(w, h int) S3BucketsModel {
	m.width = w
	m.height = h
	return m
}
