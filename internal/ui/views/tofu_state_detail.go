package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type TofuStateDetailModel struct {
	address string
	output  string
	lines   []string
	scroll  int
	width   int
	height  int
	loaded  bool
}

func NewTofuStateDetail(address string) TofuStateDetailModel {
	return TofuStateDetailModel{address: address}
}

func (m TofuStateDetailModel) Update(msg tea.Msg) (TofuStateDetailModel, tea.Cmd) {
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

func (m TofuStateDetailModel) View() string {
	var b strings.Builder

	title := fmt.Sprintf("  Resource: %s", m.address)
	b.WriteString(theme.TitleStyle.Render(title))
	b.WriteString("\n\n")

	if !m.loaded {
		b.WriteString(theme.HelpStyle.Render("  Loading..."))
		return b.String()
	}

	if len(m.lines) == 0 {
		b.WriteString(theme.HelpStyle.Render("  No state data"))
		return b.String()
	}

	visRows := m.visibleRows()
	if m.scroll > len(m.lines)-visRows {
		m.scroll = max(0, len(m.lines)-visRows)
	}
	end := min(m.scroll+visRows, len(m.lines))

	for _, line := range m.lines[m.scroll:end] {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}

	return b.String()
}

func (m TofuStateDetailModel) SetOutput(output string) TofuStateDetailModel {
	m.output = output
	m.loaded = true
	m.lines = strings.Split(output, "\n")
	return m
}

func (m TofuStateDetailModel) Address() string { return m.address }

func (m TofuStateDetailModel) visibleRows() int {
	rows := m.height - 4
	if rows < 5 {
		return 20
	}
	return rows
}

func (m TofuStateDetailModel) SetSize(w, h int) TofuStateDetailModel {
	m.width = w
	m.height = h
	return m
}
