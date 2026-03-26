package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type EC2ConsoleModel struct {
	instanceID string
	output     string
	scroll     int
	lines      []string
	width      int
	height     int
	loaded     bool
}

func NewEC2Console(instanceID string) EC2ConsoleModel {
	return EC2ConsoleModel{instanceID: instanceID}
}

func (m EC2ConsoleModel) Update(msg tea.Msg) (EC2ConsoleModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up), msg.String() == "k":
			if m.scroll > 0 {
				m.scroll--
			}
		case key.Matches(msg, theme.Keys.Down), msg.String() == "j":
			m.scroll++
		case msg.String() == "pgup":
			m.scroll = max(0, m.scroll-m.visibleRows())
		case msg.String() == "pgdown":
			m.scroll += m.visibleRows()
		case msg.String() == "g":
			m.scroll = 0
		case msg.String() == "G":
			m.scroll = max(0, len(m.lines)-m.visibleRows())
		}
	}
	return m, nil
}

func (m EC2ConsoleModel) View() string {
	var b strings.Builder

	title := fmt.Sprintf("  Console Output: %s", m.instanceID)
	b.WriteString(theme.TitleStyle.Render(title))
	b.WriteString("\n\n")

	if !m.loaded {
		b.WriteString(theme.HelpStyle.Render("  Loading..."))
		return b.String()
	}

	if len(m.lines) == 0 {
		b.WriteString(theme.HelpStyle.Render("  No console output available"))
		return b.String()
	}

	visRows := m.visibleRows()
	if m.scroll > len(m.lines)-visRows {
		m.scroll = max(0, len(m.lines)-visRows)
	}
	end := min(m.scroll+visRows, len(m.lines))

	for _, line := range m.lines[m.scroll:end] {
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (m EC2ConsoleModel) SetOutput(output string) EC2ConsoleModel {
	m.output = output
	m.loaded = true
	m.lines = strings.Split(output, "\n")
	// Start at the bottom (most recent output)
	visRows := m.visibleRows()
	if len(m.lines) > visRows {
		m.scroll = len(m.lines) - visRows
	}
	return m
}

func (m EC2ConsoleModel) visibleRows() int {
	rows := m.height - 4
	if rows < 5 {
		return 20
	}
	return rows
}

func (m EC2ConsoleModel) SetSize(w, h int) EC2ConsoleModel {
	m.width = w
	m.height = h
	return m
}
