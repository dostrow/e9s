package views

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type SecretValueModel struct {
	name   string
	value  string // raw value
	lines  []string
	tags   map[string]string
	scroll int
	width  int
	height int
}

func NewSecretValue(name, value string, tags map[string]string) SecretValueModel {
	// Try to pretty-print as JSON
	var prettyLines []string
	var raw json.RawMessage
	if err := json.Unmarshal([]byte(value), &raw); err == nil {
		var pretty []byte
		pretty, _ = json.MarshalIndent(raw, "", "  ")
		prettyLines = strings.Split(string(pretty), "\n")
	} else {
		prettyLines = strings.Split(value, "\n")
	}

	return SecretValueModel{
		name:  name,
		value: value,
		lines: prettyLines,
		tags:  tags,
	}
}

func (m SecretValueModel) Update(msg tea.Msg) (SecretValueModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if m.scroll > 0 {
				m.scroll--
			}
		case key.Matches(msg, theme.Keys.Down):
			maxScroll := m.maxScroll()
			if m.scroll < maxScroll {
				m.scroll++
			}
		case msg.String() == "pgup":
			m.scroll -= m.visibleLines()
			m.scroll = max(0, m.scroll)
		case msg.String() == "pgdown":
			m.scroll += m.visibleLines()
			m.scroll = min(m.scroll, m.maxScroll())
		case msg.String() == "g":
			m.scroll = 0
		case msg.String() == "G":
			m.scroll = m.maxScroll()
		}
	}
	return m, nil
}

func (m SecretValueModel) View() string {
	var b strings.Builder

	title := fmt.Sprintf("  Secret: %s", m.name)
	b.WriteString(theme.TitleStyle.Render(title))
	b.WriteString("\n\n")

	// Tags
	if len(m.tags) > 0 {
		b.WriteString(theme.TitleStyle.Render("  Tags"))
		b.WriteString("\n")
		for k, v := range m.tags {
			b.WriteString(fmt.Sprintf("    %s = %s\n",
				theme.HeaderStyle.Render(k),
				v))
		}
		b.WriteString("\n")
	}

	b.WriteString(theme.TitleStyle.Render("  Value"))
	b.WriteString("\n\n")

	visible := m.visibleLines()
	totalLines := len(m.lines)

	start := m.scroll
	end := start + visible
	end = min(end, totalLines)

	for _, line := range m.lines[start:end] {
		// Syntax color JSON keys
		b.WriteString("    " + colorJSONLine(line) + "\n")
	}

	if totalLines > visible {
		info := fmt.Sprintf("\n  %d–%d of %d lines", start+1, end, totalLines)
		if start > 0 {
			info += " ↑"
		}
		if end < totalLines {
			info += " ↓"
		}
		b.WriteString(theme.HelpStyle.Render(info))
	}

	return b.String()
}

func colorJSONLine(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]

	// Check if line starts with a JSON key like "keyName":
	if strings.Contains(trimmed, `":`) {
		parts := strings.SplitN(trimmed, `":`, 2)
		if len(parts) == 2 {
			key := parts[0] + `"`
			val := parts[1]
			return indent + theme.HeaderStyle.Render(key) + ":" + val
		}
	}
	return line
}

func (m SecretValueModel) visibleLines() int {
	overhead := 8
	if len(m.tags) > 0 {
		overhead += len(m.tags) + 2
	}
	h := m.height - overhead
	if h < 5 {
		return 20
	}
	return h
}

func (m SecretValueModel) maxScroll() int {
	max := len(m.lines) - m.visibleLines()
	if max < 0 {
		return 0
	}
	return max
}

func (m SecretValueModel) SetSize(w, h int) SecretValueModel {
	m.width = w
	m.height = h
	return m
}
