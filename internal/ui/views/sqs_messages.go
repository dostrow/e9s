package views

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/components"
	"github.com/dostrow/e9s/internal/ui/theme"
)

type SQSMessagesModel struct {
	queueName string
	queueURL  string
	messages  []model.SQSMessage
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
		{Title: "RECEIVES"},
		{Title: "CAPTURED"},
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
			components.Plain(valueOrDashSQS(msg.Attributes["ApproximateReceiveCount"])),
			components.Plain(formatSQSCapturedAtTUI(msg.CapturedAt)),
			components.Plain(body),
		)
	}

	b.WriteString(tbl.Render(m.cursor, "", m.visibleRows()))
	return b.String()
}

func (m SQSMessagesModel) SetMessages(messages []model.SQSMessage) SQSMessagesModel {
	positions := make(map[string]int, len(m.messages))
	for i, message := range m.messages {
		positions[message.MessageID] = i
	}
	for _, message := range messages {
		if position, found := positions[message.MessageID]; found {
			m.messages[position] = message
			continue
		}
		positions[message.MessageID] = len(m.messages)
		m.messages = append(m.messages, message)
	}
	return m
}

func (m SQSMessagesModel) RemoveMessage(messageID string) SQSMessagesModel {
	remaining := make([]model.SQSMessage, 0, len(m.messages))
	for _, message := range m.messages {
		if message.MessageID != messageID {
			remaining = append(remaining, message)
		}
	}
	m.messages = remaining
	if m.cursor >= len(m.messages) {
		m.cursor = max(0, len(m.messages)-1)
	}
	return m
}

func (m SQSMessagesModel) ClearMessages() SQSMessagesModel {
	m.messages = nil
	m.cursor = 0
	return m
}

func (m SQSMessagesModel) SelectedMessage() *model.SQSMessage {
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

func valueOrDashSQS(value string) string {
	if strings.TrimSpace(value) == "" {
		return "—"
	}
	return value
}

func formatSQSCapturedAtTUI(value time.Time) string {
	if value.IsZero() {
		return "—"
	}
	return value.Local().Format("15:04:05")
}

func formatSQSSentAtTUI(value string) string {
	millis, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || millis <= 0 {
		return "—"
	}
	return time.UnixMilli(millis).Local().Format("2006-01-02 15:04:05")
}
