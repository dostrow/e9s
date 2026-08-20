package ui

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- SQS ---

func (a App) promptSQSBrowser() (App, tea.Cmd) {
	saved := a.cfg.SQSQueues
	if len(saved) == 0 {
		a.input = NewInput(InputSQSSearch, "Search queues (substring match, or empty for all)", "")
		return a, nil
	}
	items := make([]string, 0, len(saved)+1)
	for _, q := range saved {
		items = append(items, fmt.Sprintf("%s  (%s)", q.Name, q.URL))
	}
	savedCount := len(items)
	items = append(items, "[enter a custom search]")
	a.picker = NewPickerWithDelete(PickerSQSQueue, "Select SQS queue", items, savedCount)
	return a, nil
}

func (a App) openSQSQueues(filter string) (App, tea.Cmd) {
	a.mode = modeSQS
	a.state = viewSQSQueues
	a.sqsQueuesView = views.NewSQSQueues(filter)
	a.sqsQueuesView = a.sqsQueuesView.SetSize(a.width-3, a.height-6)
	a.loading = true
	sqsService, ctx := a.sqs, a.ctx
	return a, func() tea.Msg {
		queues, err := sqsService.Queues(ctx, filter)
		if err != nil {
			return errMsg{err}
		}
		return sqsQueuesLoadedMsg{queues}
	}
}

func (a App) openSQSDetail(queueName, queueURL string) (App, tea.Cmd) {
	a.mode = modeSQS
	a.state = viewSQSDetail
	a.sqsDetailView = views.NewSQSDetail(queueName, queueURL)
	a.sqsDetailView = a.sqsDetailView.SetSize(a.width-3, a.height-6)
	a.loading = true
	sqsService, ctx := a.sqs, a.ctx
	return a, func() tea.Msg {
		stats, err := sqsService.Queue(ctx, queueURL)
		if err != nil {
			return errMsg{err}
		}
		return sqsStatsLoadedMsg{stats}
	}
}

func (a App) saveSQSQueue() (App, tea.Cmd) {
	q := a.sqsQueuesView.SelectedQueue()
	if q == nil {
		return a, nil
	}
	a.input = NewInput(InputSQSSaveName,
		fmt.Sprintf("Save queue %q — enter a name", q.Name), q.Name)
	return a, nil
}

func (a App) doSaveSQSQueue(name string) (App, tea.Cmd) {
	q := a.sqsQueuesView.SelectedQueue()
	if q == nil {
		return a, nil
	}
	a.cfg.AddSQSQueue(name, q.URL)
	if err := a.cfg.Save(); err != nil {
		a.err = err
		return a, nil
	}
	a.flashMessage = fmt.Sprintf("Saved queue %q", name)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

// --- Dead Letter Queue ---

func (a App) openSQSDLQ() (App, tea.Cmd) {
	stats := a.sqsDetailView.Stats()
	if stats == nil || stats.DeadLetterTargetARN == "" {
		a.err = fmt.Errorf("no dead letter queue configured")
		return a, nil
	}

	dlqName := service.QueueNameFromARN(stats.DeadLetterTargetARN)
	sqsService, ctx := a.sqs, a.ctx
	a.loading = true

	return a, func() tea.Msg {
		url, err := sqsService.ResolveQueueURL(ctx, dlqName)
		if err != nil {
			return errMsg{err}
		}
		return sqsDLQResolvedMsg{name: dlqName, url: url}
	}
}

// --- Message Polling ---

func (a App) openSQSMessages() (App, tea.Cmd) {
	queueName := a.sqsDetailView.QueueName()
	queueURL := a.sqsDetailView.QueueURL()
	a.state = viewSQSMessages
	a.sqsMessagesView = views.NewSQSMessages(queueName, queueURL)
	a.sqsMessagesView = a.sqsMessagesView.SetSize(a.width-3, a.height-6)
	return a, nil
}

func (a App) pollSQSMessages() (App, tea.Cmd) {
	queueURL := a.sqsMessagesView.QueueURL()
	a.loading = true
	sqsService, ctx := a.sqs, a.ctx
	return a, func() tea.Msg {
		messages, err := sqsService.Messages(ctx, model.SQSReceiveRequest{QueueURL: queueURL, MaxMessages: 10, WaitSeconds: 5})
		if err != nil {
			return errMsg{err}
		}
		return sqsMessagesReceivedMsg{messages}
	}
}

func (a App) deleteSQSMessage() (App, tea.Cmd) {
	msg := a.sqsMessagesView.SelectedMessage()
	if msg == nil {
		return a, nil
	}
	id := msg.MessageID
	if len(id) > 12 {
		id = id[:12]
	}
	a.confirm = NewConfirm(ConfirmSQSDelete,
		fmt.Sprintf("Delete message %s from queue?", id))
	return a, nil
}

func (a App) doDeleteSQSMessage() tea.Cmd {
	msg := a.sqsMessagesView.SelectedMessage()
	if msg == nil {
		return nil
	}
	sqsService, ctx := a.sqs, a.ctx
	queueURL := a.sqsMessagesView.QueueURL()
	receiptHandle := msg.ReceiptHandle

	return func() tea.Msg {
		err := sqsService.DeleteMessage(ctx, queueURL, receiptHandle)
		if err != nil {
			return errMsg{err}
		}
		return actionSuccessMsg{"Message deleted"}
	}
}

// --- Send Message ---

func (a App) sendSQSMessage() (App, tea.Cmd) {
	var queueURL string
	var isFIFO bool

	if a.state == viewSQSDetail {
		queueURL = a.sqsDetailView.QueueURL()
		if stats := a.sqsDetailView.Stats(); stats != nil {
			isFIFO = stats.IsFIFO
		}
	} else if a.state == viewSQSMessages {
		queueURL = a.sqsMessagesView.QueueURL()
	}

	template := service.BuildSQSSendTemplate(isFIFO)
	return a.openSQSSendEditor(queueURL, template)
}

func (a App) cloneSQSMessage() (App, tea.Cmd) {
	msg := a.sqsMsgDetailView.Message()
	if msg == nil {
		return a, nil
	}
	queueURL := a.sqsMsgDetailView.QueueURL()
	template := service.BuildSQSSendTemplateFromMessage(*msg)
	return a.openSQSSendEditor(queueURL, template)
}

func (a App) openSQSSendEditor(queueURL, template string) (App, tea.Cmd) {
	tmpFile, err := os.CreateTemp("", "e9s-sqs-*.json")
	if err != nil {
		a.err = err
		return a, nil
	}
	tmpPath := tmpFile.Name()
	_, _ = tmpFile.WriteString(template)
	tmpFile.Close()

	editor := NewEditorCmd(tmpPath)
	return a, tea.Exec(editor, func(err error) tea.Msg {
		defer os.Remove(tmpPath)
		if err != nil {
			return errMsg{err}
		}
		data, err := os.ReadFile(tmpPath)
		if err != nil {
			return errMsg{err}
		}
		tmpl, err := service.ParseSQSSendTemplate(string(data))
		if err != nil {
			return errMsg{err}
		}
		return sqsSendReadyMsg{queueURL: queueURL, template: tmpl}
	})
}

func (a App) doSendSQSMessage(queueURL string, tmpl *model.SQSSendTemplate) tea.Cmd {
	sqsService, ctx := a.sqs, a.ctx
	return func() tea.Msg {
		msgID, err := sqsService.SendMessage(ctx, model.SQSSendRequest{QueueURL: queueURL, Template: *tmpl})
		if err != nil {
			return errMsg{err}
		}
		return actionSuccessMsg{fmt.Sprintf("Message sent: %s", msgID)}
	}
}
