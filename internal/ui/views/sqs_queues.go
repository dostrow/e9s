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

type SQSQueuesModel struct {
	queues      []aws.SQSQueue
	searchTerm  string
	cursor      int
	filter      string
	filtering   bool
	filterInput textinput.Model
	width       int
	height      int
	loaded      bool
}

func NewSQSQueues(searchTerm string) SQSQueuesModel {
	return SQSQueuesModel{searchTerm: searchTerm}
}

func (m SQSQueuesModel) Update(msg tea.Msg) (SQSQueuesModel, tea.Cmd) {
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
			filtered := m.filteredQueues()
			if m.cursor < len(filtered)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			filtered := m.filteredQueues()
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(filtered)-1))
		case key.Matches(msg, theme.Keys.Filter):
			m.filtering = true
			m.filterInput = textinput.New()
			m.filterInput.Placeholder = "filter queues..."
			m.filterInput.SetValue(m.filter)
			m.filterInput.Focus()
			m.filterInput.Width = 30
			return m, m.filterInput.Focus()
		}
	}
	return m, nil
}

func (m SQSQueuesModel) View() string {
	filtered := m.filteredQueues()
	var b strings.Builder

	title := fmt.Sprintf("  SQS Queues (%d)", len(filtered))
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
			b.WriteString(theme.HelpStyle.Render("  No queues found"))
		}
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "QUEUE NAME"},
	})
	for _, q := range filtered {
		tbl.AddRow(components.Plain(q.Name))
	}
	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m SQSQueuesModel) filteredQueues() []aws.SQSQueue {
	if m.filter == "" {
		return m.queues
	}
	lf := strings.ToLower(m.filter)
	var out []aws.SQSQueue
	for _, q := range m.queues {
		if strings.Contains(strings.ToLower(q.Name), lf) {
			out = append(out, q)
		}
	}
	return out
}

func (m SQSQueuesModel) SetQueues(queues []aws.SQSQueue) SQSQueuesModel {
	m.queues = queues
	m.loaded = true
	filtered := m.filteredQueues()
	if m.cursor >= len(filtered) && len(filtered) > 0 {
		m.cursor = len(filtered) - 1
	}
	return m
}

func (m SQSQueuesModel) SelectedQueue() *aws.SQSQueue {
	filtered := m.filteredQueues()
	if len(filtered) == 0 || m.cursor >= len(filtered) {
		return nil
	}
	q := filtered[m.cursor]
	return &q
}

func (m SQSQueuesModel) SearchTerm() string  { return m.searchTerm }
func (m SQSQueuesModel) IsFiltering() bool    { return m.filtering }

func (m SQSQueuesModel) visibleRows() int {
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

func (m SQSQueuesModel) SetSize(w, h int) SQSQueuesModel {
	m.width = w
	m.height = h
	return m
}
