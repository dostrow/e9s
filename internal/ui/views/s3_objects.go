package views

import (
	"fmt"
	"path"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type S3ObjectsModel struct {
	bucket      string
	prefix      string // current "directory" prefix
	objects     []aws.S3Object
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewS3Objects(bucket, prefix string) S3ObjectsModel {
	return S3ObjectsModel{bucket: bucket, prefix: prefix}
}

func (m S3ObjectsModel) Update(msg tea.Msg) (S3ObjectsModel, tea.Cmd) {
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
			filtered := m.filteredObjects()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredObjects()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter objects..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.Width = 40
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m S3ObjectsModel) View() string {
	filtered := m.filteredObjects()
	var b strings.Builder

	title := fmt.Sprintf("  s3://%s/%s (%d)", m.bucket, m.prefix, len(filtered))
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
			b.WriteString(theme.HelpStyle.Render("  No objects found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "NAME"},
		{Title: "SIZE", RightAlign: true},
		{Title: "MODIFIED"},
	})

	for _, obj := range filtered {
		name := displayName(obj.Key, m.prefix)

		if obj.IsPrefix {
			// Folder
			tbl.AddRow(
				components.Styled(name+"/", lipgloss.NewStyle().Foreground(theme.ColorCyan).Bold(true)),
				components.Plain("-"),
				components.Plain("-"),
			)
		} else {
			modified := ""
			if !obj.LastModified.IsZero() {
				modified = formatAge(obj.LastModified) + " ago"
			}
			tbl.AddRow(
				components.Plain(name),
				components.Plain(formatBytesS3(obj.Size)),
				components.Plain(modified),
			)
		}
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

// displayName strips the prefix to show just the relative name.
func displayName(key, prefix string) string {
	rel := strings.TrimPrefix(key, prefix)
	rel = strings.TrimSuffix(rel, "/")
	if rel == "" {
		return path.Base(strings.TrimSuffix(key, "/"))
	}
	return rel
}

func formatBytesS3(b int64) string {
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

func (m S3ObjectsModel) filteredObjects() []aws.S3Object {
	if m.filter == "" {
		return m.objects
	}
	lf := strings.ToLower(m.filter)
	var out []aws.S3Object
	for _, o := range m.objects {
		if strings.Contains(strings.ToLower(displayName(o.Key, m.prefix)), lf) {
			out = append(out, o)
		}
	}
	return out
}

func (m S3ObjectsModel) SetObjects(objects []aws.S3Object) S3ObjectsModel {
	m.objects = objects
	m.loaded = true
	filtered := m.filteredObjects()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m S3ObjectsModel) SelectedObject() *aws.S3Object {
	filtered := m.filteredObjects()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	o := filtered[m.cursor]
	return &o
}

func (m S3ObjectsModel) Bucket() string    { return m.bucket }
func (m S3ObjectsModel) Prefix() string    { return m.prefix }
func (m S3ObjectsModel) IsFiltering() bool { return m.filtering }

// ParentPrefix returns the parent prefix, or "" if already at root.
func (m S3ObjectsModel) ParentPrefix() string {
	p := strings.TrimSuffix(m.prefix, "/")
	if p == "" {
		return ""
	}
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return ""
	}
	return p[:idx+1]
}

func (m S3ObjectsModel) visibleRows() int {
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

func (m S3ObjectsModel) SetSize(w, h int) S3ObjectsModel {
	m.width = w
	m.height = h
	return m
}
