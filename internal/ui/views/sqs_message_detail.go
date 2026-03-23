package views

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type SQSMessageDetailModel struct {
	queueName string
	queueURL  string
	message   *aws.SQSMessage
	lines     []string
	scroll    int
	width     int
	height    int
}

func NewSQSMessageDetail(queueName, queueURL string, msg *aws.SQSMessage) SQSMessageDetailModel {
	var lines []string
	if msg != nil {
		lines = buildSQSMessageLines(msg)
	}
	return SQSMessageDetailModel{
		queueName: queueName,
		queueURL:  queueURL,
		message:   msg,
		lines:     lines,
	}
}

func buildSQSMessageLines(msg *aws.SQSMessage) []string {
	var lines []string

	lines = append(lines, fmt.Sprintf("%s: %s", theme.HeaderStyle.Render("Message ID"), msg.MessageID))
	lines = append(lines, fmt.Sprintf("%s: %s", theme.HeaderStyle.Render("MD5"), msg.MD5))

	// System attributes
	if len(msg.Attributes) > 0 {
		lines = append(lines, "")
		lines = append(lines, theme.TitleStyle.Render("System Attributes"))
		keys := make([]string, 0, len(msg.Attributes))
		for k := range msg.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			lines = append(lines, fmt.Sprintf("  %s: %s", theme.HeaderStyle.Render(k), msg.Attributes[k]))
		}
	}

	// User attributes
	if len(msg.UserAttrsMap) > 0 {
		lines = append(lines, "")
		lines = append(lines, theme.TitleStyle.Render("Message Attributes"))
		keys := make([]string, 0, len(msg.UserAttrsMap))
		for k := range msg.UserAttrsMap {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			lines = append(lines, fmt.Sprintf("  %s: %s", theme.HeaderStyle.Render(k), msg.UserAttrsMap[k]))
		}
	}

	// Body
	lines = append(lines, "")
	lines = append(lines, theme.TitleStyle.Render("Body"))
	lines = append(lines, "")

	// Pretty-print JSON body if possible
	body := msg.Body
	var prettyJSON json.RawMessage
	if err := json.Unmarshal([]byte(body), &prettyJSON); err == nil {
		var pretty []byte
		pretty, _ = json.MarshalIndent(prettyJSON, "  ", "  ")
		lines = append(lines, strings.Split("  "+string(pretty), "\n")...)
	} else {
		for _, line := range strings.Split(body, "\n") {
			lines = append(lines, "  "+line)
		}
	}

	return lines
}

func (m SQSMessageDetailModel) Update(msg tea.Msg) (SQSMessageDetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			m.scroll = max(0, m.scroll-1)
		case key.Matches(msg, theme.Keys.Down):
			m.scroll = min(m.scroll+1, m.maxScroll())
		case msg.String() == "pgup":
			m.scroll = max(0, m.scroll-m.visibleLines())
		case msg.String() == "pgdown":
			m.scroll = min(m.scroll+m.visibleLines(), m.maxScroll())
		case msg.String() == "g":
			m.scroll = 0
		case msg.String() == "G":
			m.scroll = m.maxScroll()
		}
	}
	return m, nil
}

func (m SQSMessageDetailModel) View() string {
	var b strings.Builder

	b.WriteString(theme.TitleStyle.Render(fmt.Sprintf("  Message: %s", m.queueName)))
	b.WriteString("\n\n")

	if m.message == nil {
		b.WriteString(theme.HelpStyle.Render("  No message selected"))
		return b.String()
	}

	visible := m.visibleLines()
	start := m.scroll
	end := min(start+visible, len(m.lines))

	for _, line := range m.lines[start:end] {
		b.WriteString("  " + line + "\n")
	}

	if len(m.lines) > visible {
		fmt.Fprintf(&b, "\n  %d–%d of %d lines", start+1, end, len(m.lines))
		if start > 0 {
			b.WriteString(" ↑")
		}
		if end < len(m.lines) {
			b.WriteString(" ↓")
		}
	}

	return b.String()
}

func (m SQSMessageDetailModel) Message() *aws.SQSMessage { return m.message }
func (m SQSMessageDetailModel) QueueName() string         { return m.queueName }
func (m SQSMessageDetailModel) QueueURL() string           { return m.queueURL }

func (m SQSMessageDetailModel) visibleLines() int {
	h := m.height - 6
	if h < 5 {
		return 20
	}
	return h
}

func (m SQSMessageDetailModel) maxScroll() int {
	return max(0, len(m.lines)-m.visibleLines())
}

func (m SQSMessageDetailModel) SetSize(w, h int) SQSMessageDetailModel {
	m.width = w
	m.height = h
	return m
}
