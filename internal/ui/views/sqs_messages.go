package views

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type SQSMessagesModel struct {
	queueName string
	queueURL  string
	messages  []aws.SQSMessage
	cursor    int
	width     int
	height    int
}

func NewSQSMessages(queueName, queueURL string) SQSMessagesModel {
	return SQSMessagesModel{queueName: queueName, queueURL: queueURL}
}

func (m SQSMessagesModel) Update(msg tea.Msg) (SQSMessagesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, theme.Keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, theme.Keys.Down):
			if m.cursor < len(m.messages)-1 {
				m.cursor++
			}
		case msg.String() == "pgup":
			m.cursor = max(0, m.cursor-m.visibleRows())
		case msg.String() == "pgdown":
			m.cursor = min(m.cursor+m.visibleRows(), max(0, len(m.messages)-1))
		}
	}
	return m, nil
}

func (m SQSMessagesModel) View() string {
	var b strings.Builder

	title := fmt.Sprintf("  Messages: %s (%d)", m.queueName, len(m.messages))
	b.WriteString(theme.TitleStyle.Render(title))
	b.WriteString("\n\n")

	if len(m.messages) == 0 {
		b.WriteString(theme.HelpStyle.Render("  No messages (press [p] to poll)"))
		return b.String()
	}

	tbl := components.NewTable([]components.Column{
		{Title: "MESSAGE ID"},
		{Title: "BODY PREVIEW"},
	})

	for _, msg := range m.messages {
		id := msg.MessageID
		if len(id) > 12 {
			id = id[:12]
		}
		body := msg.Body
		// Collapse and truncate for table
		body = strings.ReplaceAll(body, "\n", "\\n")
		body = strings.ReplaceAll(body, "\r", "")
		if len(body) > 60 {
			body = body[:60] + ".."
		}
		// Try to detect JSON and show a hint
		if strings.HasPrefix(strings.TrimSpace(msg.Body), "{") {
			var j map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Body), &j); err == nil {
				body = fmt.Sprintf("{...} (%d keys)", len(j))
			}
		}
		tbl.AddRow(
			components.Plain(id),
			components.Plain(body),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m SQSMessagesModel) SetMessages(messages []aws.SQSMessage) SQSMessagesModel {
	m.messages = append(m.messages, messages...)
	return m
}

func (m SQSMessagesModel) ClearMessages() SQSMessagesModel {
	m.messages = nil
	m.cursor = 0
	return m
}

func (m SQSMessagesModel) SelectedMessage() *aws.SQSMessage {
	if len(m.messages) == 0 || m.cursor >= len(m.messages) {
		return nil
	}
	msg := m.messages[m.cursor]
	return &msg
}

func (m SQSMessagesModel) QueueName() string { return m.queueName }
func (m SQSMessagesModel) QueueURL() string  { return m.queueURL }

func (m SQSMessagesModel) visibleRows() int {
	overhead := 9
	rows := m.height - overhead
	if rows < 5 {
		return 0
	}
	return rows
}

func (m SQSMessagesModel) SetSize(w, h int) SQSMessagesModel {
	m.width = w
	m.height = h
	return m
}
