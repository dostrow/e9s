//go:build gui

package gui

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

func (o Options) ConfigSQSQueues() []config.SQSQueueEntry {
	if o.Config == nil {
		return nil
	}
	return o.Config.SQSQueues
}

func (w *mainWindow) openSQSModule() {
	if w.currentPage == pageSQSQueues && w.activeSavedSQSQueue == "" {
		return
	}
	w.loadSQSQueues()
}

func (w *mainWindow) loadSQSQueues() {
	w.resetWorkspaceForBrowserChange()
	w.clearSQSQueues()
	w.clearSQSMessages()
	w.currentPage = pageSQSQueues
	w.activeSavedSQSQueue = ""
	w.search.SetPlaceholderText("Filter queues…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageSQSQueues)
	w.backButton.SetSensitive(false)
	w.setBreadcrumb("SQS / Queues")
	w.setDetail("Loading SQS queues…", detailIntro)
	w.updateActionSensitivity()
	if w.options.SQS == nil {
		w.setDetail("SQS is unavailable because no SQS service was configured.", detailError)
		w.setStatus("SQS service unavailable", true)
		return
	}
	ctx, generation := w.startRequest("Loading SQS queues…")
	go func() {
		queues, err := w.options.SQS.Queues(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			w.allSQSQueues = queues
			w.applySQSQueueFilter()
			w.setDetail(sqsQueueListSummary(len(queues)), detailIntro)
		})
	}()
}

func (w *mainWindow) openSavedSQSQueue(saved config.SQSQueueEntry) {
	w.resetWorkspaceForBrowserChange()
	w.clearSQSQueues()
	w.clearSQSMessages()
	w.currentPage = pageSQSQueues
	w.activeSavedSQSQueue = saved.Name
	w.search.SetPlaceholderText("Filter queues…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageSQSQueues)
	w.backButton.SetSensitive(false)
	queue := model.SQSQueue{Name: service.QueueNameFromURL(saved.URL), URL: saved.URL}
	w.allSQSQueues = []model.SQSQueue{queue}
	w.filteredSQSQueues = append([]model.SQSQueue(nil), w.allSQSQueues...)
	w.sqsQueueTable.replace([]string{queue.Name + "\t" + queue.URL})
	w.selectedSQSQueue = queue.URL
	w.sqsQueueTable.selection.SetSelected(0)
	w.setBreadcrumb(sqsQueueBreadcrumb(saved.Name, queue.Name))
	w.setDetail("Loading SQS queue configuration…", detailIntro)
	w.updateActionSensitivity()
	w.loadSQSQueueDetail(queue)
}

func (w *mainWindow) refreshSQSQueues(foreground bool) {
	if w.options.SQS == nil || w.sqsActionPending {
		return
	}
	if w.activeSavedSQSQueue != "" {
		if foreground && !w.reloadSQSConfig() {
			return
		}
		if saved, found := findSavedSQSQueue(w.options.ConfigSQSQueues(), w.activeSavedSQSQueue); found {
			w.openSavedSQSQueue(saved)
			return
		}
		w.loadSQSQueues()
		return
	}
	selected := w.selectedSQSQueue
	ctx, generation := w.startRefreshRequest("Refreshing SQS queues…", foreground)
	go func() {
		queues, err := w.options.SQS.Queues(ctx, "")
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allSQSQueues = queues
			w.applySQSQueueFilter()
			if selected == "" {
				w.setDetail(sqsQueueListSummary(len(queues)), detailIntro)
				return
			}
			if index := findSQSQueueIndex(w.filteredSQSQueues, selected); index >= 0 {
				w.sqsQueueTable.selection.SetSelected(uint(index))
				return
			}
			w.selectedSQSQueue = ""
			w.sqsQueueStats = nil
			w.setBreadcrumb("SQS / Queues")
			w.setDetail("The selected SQS queue is no longer available.", detailIntro)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) clearSQSQueues() {
	w.allSQSQueues = nil
	w.filteredSQSQueues = nil
	w.selectedSQSQueue = ""
	w.sqsQueueStats = nil
	if w.sqsQueueTable != nil {
		w.sqsQueueTable.clear()
	}
}

func (w *mainWindow) clearSQSMessages() {
	w.allSQSMessages = nil
	w.filteredSQSMessages = nil
	w.selectedSQSMessage = ""
	w.sqsMessageQueue = model.SQSQueue{}
	w.sqsMessageQueueStats = nil
	w.sqsMessagesParentURL = ""
	w.sqsMessagesParentSavedName = ""
	if w.sqsMessageTable != nil {
		w.sqsMessageTable.clear()
	}
}

func (w *mainWindow) applySQSQueueFilter() {
	w.filteredSQSQueues = filterSQSQueues(w.allSQSQueues, w.search.Text())
	rows := make([]string, 0, len(w.filteredSQSQueues))
	for _, queue := range w.filteredSQSQueues {
		rows = append(rows, queue.Name+"\t"+queue.URL)
	}
	w.sqsQueueTable.replace(rows)
}

func filterSQSQueues(queues []model.SQSQueue, query string) []model.SQSQueue {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]model.SQSQueue, 0, len(queues))
	for _, queue := range queues {
		if query == "" || strings.Contains(strings.ToLower(queue.Name), query) || strings.Contains(strings.ToLower(queue.URL), query) {
			filtered = append(filtered, queue)
		}
	}
	return filtered
}

func (w *mainWindow) selectSQSQueueRow() {
	if w.currentPage != pageSQSQueues {
		return
	}
	position := w.sqsQueueTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredSQSQueues) {
		w.selectedSQSQueue = ""
		w.sqsQueueStats = nil
		w.setBreadcrumb(sqsQueueBreadcrumb(w.activeSavedSQSQueue, ""))
		w.setDetail(sqsQueueListSummary(len(w.allSQSQueues)), detailIntro)
		w.updateActionSensitivity()
		return
	}
	queue := w.filteredSQSQueues[position]
	w.selectedSQSQueue = queue.URL
	w.sqsQueueStats = nil
	w.setBreadcrumb(sqsQueueBreadcrumb(w.activeSavedSQSQueue, queue.Name))
	w.setDetail("Loading SQS queue configuration…", detailIntro)
	w.updateActionSensitivity()
	w.loadSQSQueueDetail(queue)
}

func (w *mainWindow) openSQSQueueAt(position uint) {
	if int(position) >= len(w.filteredSQSQueues) {
		return
	}
	queue := w.filteredSQSQueues[position]
	w.sqsQueueTable.selection.SetSelected(position)
	w.loadSQSMessages(queue, queue.URL, w.activeSavedSQSQueue)
}

func (w *mainWindow) loadSQSMessages(queue model.SQSQueue, parentURL, parentSavedName string) {
	if w.options.SQS == nil || queue.URL == "" || w.sqsActionPending {
		return
	}
	w.resetWorkspaceForBrowserChange()
	w.clearSQSMessages()
	w.currentPage = pageSQSMessages
	w.sqsMessageQueue = queue
	w.selectedSQSQueue = queue.URL
	w.sqsMessagesParentURL = parentURL
	w.sqsMessagesParentSavedName = parentSavedName
	w.search.SetPlaceholderText("Filter loaded messages…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageSQSMessages)
	w.backButton.SetSensitive(true)
	w.setBreadcrumb(sqsMessageBreadcrumb(parentSavedName, queue.Name, ""))
	w.setDetail("Loading SQS queue configuration…", detailIntro)
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Loading SQS queue configuration…")
	go func() {
		stats, err := w.options.SQS.Queue(ctx, queue.URL)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageSQSMessages || w.sqsMessageQueue.URL != queue.URL {
				return
			}
			w.sqsMessageQueueStats = stats
			w.setDetail(sqsMessageListSummary(queue, 0), detailSQSQueue)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) restoreSQSQueueBrowser() {
	parentURL, parentSaved := w.sqsMessagesParentURL, w.sqsMessagesParentSavedName
	w.resetWorkspaceForBrowserChange()
	w.clearSQSMessages()
	w.currentPage = pageSQSQueues
	w.activeSavedSQSQueue = parentSaved
	w.search.SetPlaceholderText("Filter queues…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageSQSQueues)
	w.backButton.SetSensitive(false)
	w.applySQSQueueFilter()
	if queue, found := findSQSQueue(w.allSQSQueues, parentURL); found {
		w.selectedSQSQueue = queue.URL
		w.setBreadcrumb(sqsQueueBreadcrumb(parentSaved, queue.Name))
		w.setDetail("Loading SQS queue configuration…", detailIntro)
		if index := findSQSQueueIndex(w.filteredSQSQueues, queue.URL); index >= 0 {
			w.sqsQueueTable.selection.SetSelected(uint(index))
		} else {
			w.loadSQSQueueDetail(queue)
		}
	} else {
		w.selectedSQSQueue = ""
		w.sqsQueueStats = nil
		w.setBreadcrumb(sqsQueueBreadcrumb(parentSaved, ""))
		w.setDetail(sqsQueueListSummary(len(w.allSQSQueues)), detailIntro)
	}
	w.updateActionSensitivity()
	w.setStatus("Ready", false)
}

func (w *mainWindow) applySQSMessageFilter() {
	w.filteredSQSMessages = filterSQSMessages(w.allSQSMessages, w.search.Text())
	rows := make([]string, 0, len(w.filteredSQSMessages))
	for _, message := range w.filteredSQSMessages {
		rows = append(rows, strings.Join([]string{
			message.MessageID,
			valueOrDash(message.Attributes["ApproximateReceiveCount"]),
			formatSQSMillis(message.Attributes["SentTimestamp"]),
			formatSQSCapturedAt(message.CapturedAt),
			sqsMessagePreview(message.Body),
		}, "\t"))
	}
	w.sqsMessageTable.replace(rows)
}

func formatSQSMillis(value string) string {
	millis, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || millis <= 0 {
		return "—"
	}
	return formatTime(time.UnixMilli(millis))
}

func formatSQSCapturedAt(capturedAt time.Time) string {
	if capturedAt.IsZero() {
		return "—"
	}
	return formatTime(capturedAt)
}

func filterSQSMessages(messages []model.SQSMessage, query string) []model.SQSMessage {
	query = strings.ToLower(strings.TrimSpace(query))
	filtered := make([]model.SQSMessage, 0, len(messages))
	for _, message := range messages {
		if query == "" || strings.Contains(strings.ToLower(sqsMessageSearchText(message)), query) {
			filtered = append(filtered, message)
		}
	}
	return filtered
}

func sqsMessageSearchText(message model.SQSMessage) string {
	parts := []string{message.MessageID, message.Body, message.MD5}
	for name, value := range message.Attributes {
		parts = append(parts, name, value)
	}
	for name, value := range message.UserAttributes {
		parts = append(parts, name, value.DataType, value.StringValue, service.SQSMessageAttributeDisplay(value))
	}
	return strings.Join(parts, "\n")
}

func sqsMessagePreview(body string) string {
	preview := strings.Join(strings.Fields(body), " ")
	runes := []rune(preview)
	if len(runes) > 140 {
		return string(runes[:137]) + "…"
	}
	return preview
}

func (w *mainWindow) selectSQSMessageRow() {
	if w.currentPage != pageSQSMessages {
		return
	}
	position := w.sqsMessageTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredSQSMessages) {
		w.selectedSQSMessage = ""
		w.setBreadcrumb(sqsMessageBreadcrumb(w.activeSavedSQSQueue, w.sqsMessageQueue.Name, ""))
		w.setDetail(sqsMessageListSummary(w.sqsMessageQueue, len(w.allSQSMessages)), detailSQSQueue)
		w.updateActionSensitivity()
		return
	}
	message := w.filteredSQSMessages[position]
	w.selectedSQSMessage = message.MessageID
	w.setBreadcrumb(sqsMessageBreadcrumb(w.activeSavedSQSQueue, w.sqsMessageQueue.Name, shortSQSMessageID(message.MessageID)))
	w.setDetail(formatSQSMessage(w.sqsMessageQueue, message), detailSQSMessage)
	w.updateActionSensitivity()
}

func (w *mainWindow) openSQSMessageAt(position uint) {
	if int(position) < len(w.filteredSQSMessages) {
		w.sqsMessageTable.selection.SetSelected(position)
	}
}

func (w *mainWindow) openOrPollSQSMessages() {
	if w.currentPage == pageSQSQueues {
		queue, found := findSQSQueue(w.allSQSQueues, w.selectedSQSQueue)
		if found {
			w.loadSQSMessages(queue, queue.URL, w.activeSavedSQSQueue)
		}
		return
	}
	if w.currentPage == pageSQSMessages {
		w.promptPollSQSMessages()
	}
}

func (w *mainWindow) promptPollSQSMessages() {
	if w.currentPage != pageSQSMessages || w.options.SQS == nil || w.sqsActionPending || w.sqsMessageQueue.URL == "" {
		return
	}
	maxMessages, waitSeconds := w.sqsPollMaxMessages, w.sqsPollWaitSeconds
	if maxMessages <= 0 {
		maxMessages, waitSeconds = 10, 10
	}
	dialog := gtk.NewDialogWithFlags("Poll SQS messages", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	messageCount := gtk.NewSpinButtonWithRange(1, 10, 1)
	messageCount.SetValue(float64(maxMessages))
	waitTime := gtk.NewSpinButtonWithRange(0, 20, 1)
	waitTime.SetValue(float64(waitSeconds))
	content.Append(settingsRow("Maximum messages", messageCount))
	content.Append(settingsRow("Long-poll wait (seconds)", waitTime))
	content.Append(settingsNote("Received messages are captured in the local buffer and hidden from other consumers until acknowledged, released, or their queue visibility timeout expires."))
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Poll", int(gtk.ResponseOK))
	dialog.SetDefaultResponse(int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		maxMessages, waitSeconds := messageCount.ValueAsInt(), waitTime.ValueAsInt()
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.sqsPollMaxMessages, w.sqsPollWaitSeconds = maxMessages, waitSeconds
			w.pollSQSMessages(maxMessages, waitSeconds)
		}
	})
	dialog.Present()
}

func (w *mainWindow) pollSQSMessages(maxMessages, waitSeconds int) {
	if w.currentPage != pageSQSMessages || w.options.SQS == nil || w.sqsActionPending || w.sqsMessageQueue.URL == "" {
		return
	}
	w.sqsActionPending = true
	w.updateActionSensitivity()
	queue := w.sqsMessageQueue
	ctx, generation := w.startRequest("Polling " + queue.Name + " for messages…")
	w.setWorkspaceCancellation(func() {
		if w.requestCancel != nil {
			w.requestCancel()
		}
		w.requestPending = false
		w.sqsActionPending = false
		w.spinner.Stop()
		w.setWorkspaceBusy("", false)
		w.updateActionSensitivity()
		w.setStatus("SQS poll cancelled", false)
	})
	go func() {
		messages, err := w.options.SQS.Messages(ctx, model.SQSReceiveRequest{QueueURL: queue.URL, MaxMessages: maxMessages, WaitSeconds: waitSeconds})
		w.finishSQSAction(ctx, generation, err, fmt.Sprintf("Received %d message(s) from %s", len(messages), queue.Name), func() {
			w.allSQSMessages = mergeSQSMessages(w.allSQSMessages, messages)
			w.applySQSMessageFilter()
			w.selectedSQSMessage = ""
			w.setBreadcrumb(sqsMessageBreadcrumb(w.activeSavedSQSQueue, queue.Name, ""))
			w.setDetail(sqsMessageListSummary(queue, len(w.allSQSMessages)), detailSQSQueue)
		})
	}()
}

func mergeSQSMessages(existing, received []model.SQSMessage) []model.SQSMessage {
	merged := append([]model.SQSMessage(nil), existing...)
	positions := make(map[string]int, len(merged))
	for index, message := range merged {
		positions[message.MessageID] = index
	}
	for _, message := range received {
		if index, found := positions[message.MessageID]; found {
			merged[index] = message
			continue
		}
		positions[message.MessageID] = len(merged)
		merged = append(merged, message)
	}
	return merged
}

func (w *mainWindow) clearSQSMessageBuffer() {
	if w.currentPage != pageSQSMessages || w.sqsActionPending {
		return
	}
	w.allSQSMessages = nil
	w.filteredSQSMessages = nil
	w.selectedSQSMessage = ""
	w.sqsMessageTable.clear()
	w.setBreadcrumb(sqsMessageBreadcrumb(w.activeSavedSQSQueue, w.sqsMessageQueue.Name, ""))
	w.setDetail(sqsMessageListSummary(w.sqsMessageQueue, 0), detailSQSQueue)
	w.updateActionSensitivity()
	w.setStatus("Cleared SQS message buffer", false)
}

func (w *mainWindow) openSQSDeadLetterQueue() {
	stats := w.sqsQueueStats
	queue := model.SQSQueue{}
	parentURL, parentSaved := w.selectedSQSQueue, w.activeSavedSQSQueue
	if w.currentPage == pageSQSMessages {
		stats = w.sqsMessageQueueStats
		queue = w.sqsMessageQueue
		parentURL, parentSaved = w.sqsMessagesParentURL, w.sqsMessagesParentSavedName
	} else {
		queue, _ = findSQSQueue(w.allSQSQueues, w.selectedSQSQueue)
	}
	if stats == nil || stats.DeadLetterTargetARN == "" || w.options.SQS == nil || w.sqsActionPending {
		return
	}
	w.sqsActionPending = true
	w.updateActionSensitivity()
	name := service.QueueNameFromARN(stats.DeadLetterTargetARN)
	ctx, generation := w.startRequest("Resolving dead-letter queue " + name + "…")
	go func() {
		url, err := w.options.SQS.ResolveQueueURL(ctx, name)
		var detail *model.SQSQueueStats
		if err == nil {
			detail, err = w.options.SQS.Queue(ctx, url)
		}
		w.finishSQSAction(ctx, generation, err, "Opened dead-letter queue "+name, func() {
			dlq := model.SQSQueue{Name: name, URL: url}
			w.clearSQSMessages()
			w.currentPage = pageSQSMessages
			w.activeSavedSQSQueue = ""
			w.sqsMessageQueue = dlq
			w.sqsMessageQueueStats = detail
			w.selectedSQSQueue = dlq.URL
			w.sqsMessagesParentURL = parentURL
			w.sqsMessagesParentSavedName = parentSaved
			w.search.SetPlaceholderText("Filter loaded messages…")
			w.search.SetText("")
			w.resourceStack.SetVisibleChildName(pageSQSMessages)
			w.backButton.SetSensitive(true)
			w.setBreadcrumb(sqsMessageBreadcrumb("", dlq.Name, ""))
			w.setDetail(sqsMessageListSummary(dlq, 0)+"\n\nOpened from "+queue.Name+".", detailSQSQueue)
		})
	}()
}

func (w *mainWindow) refreshSQSMessages(foreground bool) {
	if w.options.SQS == nil || w.sqsActionPending || w.sqsMessageQueue.URL == "" {
		return
	}
	queue, selected := w.sqsMessageQueue, w.selectedSQSMessage
	ctx, generation := w.startRefreshRequest("Refreshing SQS queue configuration…", foreground)
	go func() {
		stats, err := w.options.SQS.Queue(ctx, queue.URL)
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.sqsMessageQueueStats = stats
			if message, found := findSQSMessage(w.allSQSMessages, selected); found {
				w.setDetail(formatSQSMessage(queue, message), detailSQSMessage)
			} else {
				w.selectedSQSMessage = ""
				w.setDetail(sqsMessageListSummary(queue, len(w.allSQSMessages)), detailSQSQueue)
			}
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) finishSQSAction(ctx context.Context, generation uint64, err error, success string, apply func()) {
	glib.IdleAdd(func() {
		if ctx.Err() != nil || generation != w.generation {
			return
		}
		w.requestCancel = nil
		w.spinner.Stop()
		w.setWorkspaceBusy("", false)
		w.sqsActionPending = false
		if err != nil {
			w.updateActionSensitivity()
			w.setStatus(err.Error(), true)
			return
		}
		if apply != nil {
			apply()
		}
		w.updateActionSensitivity()
		w.setStatus(success, false)
	})
}

func (w *mainWindow) currentSQSQueueForAction() (model.SQSQueue, *model.SQSQueueStats, bool) {
	if w.currentPage == pageSQSMessages && w.sqsMessageQueue.URL != "" {
		return w.sqsMessageQueue, w.sqsMessageQueueStats, true
	}
	if w.currentPage == pageSQSQueues {
		queue, found := findSQSQueue(w.allSQSQueues, w.selectedSQSQueue)
		return queue, w.sqsQueueStats, found
	}
	return model.SQSQueue{}, nil, false
}

func (w *mainWindow) selectedSQSMessageValue() (model.SQSMessage, bool) {
	if w.currentPage != pageSQSMessages || w.selectedSQSMessage == "" {
		return model.SQSMessage{}, false
	}
	return findSQSMessage(w.allSQSMessages, w.selectedSQSMessage)
}

func (w *mainWindow) promptSQSSendMessage() {
	queue, stats, found := w.currentSQSQueueForAction()
	if !found || w.options.SQS == nil || w.sqsActionPending {
		return
	}
	isFIFO := stats != nil && stats.IsFIFO
	w.promptSQSSendTemplate(queue, service.BuildSQSSendTemplate(isFIFO), "Send SQS message")
}

func (w *mainWindow) promptSQSCloneMessage() {
	queue, _, queueFound := w.currentSQSQueueForAction()
	message, messageFound := w.selectedSQSMessageValue()
	if !queueFound || !messageFound || w.options.SQS == nil || w.sqsActionPending {
		return
	}
	w.promptSQSSendTemplate(queue, service.BuildSQSSendTemplateFromMessage(message), "Clone SQS message")
}

func (w *mainWindow) promptSQSSendTemplate(queue model.SQSQueue, initial, title string) {
	dialog := gtk.NewDialogWithFlags(title, &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(760, 560)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	label := gtk.NewLabel("Edit the portable JSON send document for " + queue.Name + ". Binary attribute values use base64.")
	label.SetXAlign(0)
	label.SetWrap(true)
	content.Append(label)
	editor := newSourceEditor(sourceDocument{Path: "sqs-message.json", Language: "json"})
	editor.ApplyPalette(w.currentSemanticPalette(w.window.StyleContext()))
	editor.SetText(initial)
	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetChild(editor.Widget())
	content.Append(scroll)
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.SetWrap(true)
	errorLabel.AddCSSClass("error")
	content.Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Review send…", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		template, err := service.ParseSQSSendTemplate(editor.Text())
		if err == nil {
			err = service.ValidateSQSSendTemplate(queue.URL, *template)
		}
		if err != nil {
			errorLabel.SetLabel(err.Error())
			return
		}
		dialog.Destroy()
		w.confirmSQSSendMessage(queue, *template)
	})
	dialog.Present()
}

func (w *mainWindow) confirmSQSSendMessage(queue model.SQSQueue, template model.SQSSendTemplate) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsYesNo)
	dialog.SetTitle("Send SQS message")
	dialog.SetMarkup("Send a message to <b>" + html.EscapeString(queue.Name) + "</b>?")
	secondary := fmt.Sprintf("Body: %d bytes • Attributes: %d", len([]byte(template.Body)), len(template.Attributes))
	if template.GroupID != "" {
		secondary += " • Group: " + template.GroupID
	}
	if template.DelaySeconds > 0 {
		secondary += fmt.Sprintf(" • Delay: %ds", template.DelaySeconds)
	}
	dialog.SetObjectProperty("secondary-text", secondary)
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.runSQSSendMessage(queue, template)
		}
	})
	dialog.Present()
}

func (w *mainWindow) runSQSSendMessage(queue model.SQSQueue, template model.SQSSendTemplate) {
	if w.options.SQS == nil || w.sqsActionPending {
		return
	}
	w.sqsActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Sending message to " + queue.Name + "…")
	go func() {
		messageID, err := w.options.SQS.SendMessage(ctx, model.SQSSendRequest{QueueURL: queue.URL, Template: template})
		w.finishSQSAction(ctx, generation, err, "Sent SQS message "+shortSQSMessageID(messageID), nil)
	}()
}

func (w *mainWindow) confirmDeleteSQSMessage() {
	queue, _, queueFound := w.currentSQSQueueForAction()
	message, messageFound := w.selectedSQSMessageValue()
	if !queueFound || !messageFound || w.options.SQS == nil || w.sqsActionPending {
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetTitle("Acknowledge SQS message")
	dialog.SetMarkup("Acknowledge and permanently delete message <b>" + html.EscapeString(shortSQSMessageID(message.MessageID)) + "</b> from <b>" + html.EscapeString(queue.Name) + "</b>?")
	dialog.SetObjectProperty("secondary-text", sqsMessagePreview(message.Body))
	dialog.SetDestroyWithParent(true)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Acknowledge / delete", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.runDeleteSQSMessage(queue, message)
		}
	})
	dialog.Present()
}

func (w *mainWindow) runDeleteSQSMessage(queue model.SQSQueue, message model.SQSMessage) {
	if w.options.SQS == nil || w.sqsActionPending {
		return
	}
	w.sqsActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Acknowledging SQS message " + shortSQSMessageID(message.MessageID) + "…")
	go func() {
		err := w.options.SQS.DeleteMessage(ctx, queue.URL, message.ReceiptHandle)
		w.finishSQSAction(ctx, generation, err, "Acknowledged and deleted SQS message "+shortSQSMessageID(message.MessageID), func() {
			w.allSQSMessages = withoutSQSMessage(w.allSQSMessages, message.MessageID)
			w.selectedSQSMessage = ""
			w.applySQSMessageFilter()
			w.setBreadcrumb(sqsMessageBreadcrumb(w.activeSavedSQSQueue, queue.Name, ""))
			w.setDetail(sqsMessageListSummary(queue, len(w.allSQSMessages)), detailSQSQueue)
		})
	}()
}

func (w *mainWindow) releaseSQSMessage() {
	queue, _, queueFound := w.currentSQSQueueForAction()
	message, messageFound := w.selectedSQSMessageValue()
	if !queueFound || !messageFound || w.options.SQS == nil || w.sqsActionPending {
		return
	}
	w.sqsActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest("Releasing SQS message " + shortSQSMessageID(message.MessageID) + "…")
	go func() {
		err := w.options.SQS.ReleaseMessage(ctx, queue.URL, message.ReceiptHandle)
		w.finishSQSAction(ctx, generation, err, "Released SQS message "+shortSQSMessageID(message.MessageID), func() {
			w.allSQSMessages = withoutSQSMessage(w.allSQSMessages, message.MessageID)
			w.selectedSQSMessage = ""
			w.applySQSMessageFilter()
			w.setBreadcrumb(sqsMessageBreadcrumb(w.activeSavedSQSQueue, queue.Name, ""))
			w.setDetail(sqsMessageListSummary(queue, len(w.allSQSMessages)), detailSQSQueue)
		})
	}()
}

func withoutSQSMessage(messages []model.SQSMessage, messageID string) []model.SQSMessage {
	result := make([]model.SQSMessage, 0, len(messages))
	for _, message := range messages {
		if message.MessageID != messageID {
			result = append(result, message)
		}
	}
	return result
}

func sqsMessageBreadcrumb(savedName, queueName, messageID string) string {
	root := "Queues"
	if savedName != "" {
		root = savedName
	}
	result := "SQS / " + root + " / " + queueName + " / Messages"
	if messageID != "" {
		result += " / " + messageID
	}
	return result
}

func sqsMessageListSummary(queue model.SQSQueue, count int) string {
	return fmt.Sprintf("SQS MESSAGES\n\nQueue            %s\nCaptured         %d\n\nPolling receives messages and changes their visibility. Acknowledge/delete removes a message permanently; Release makes it immediately available again. No messages are received automatically.", queue.Name, count)
}

func formatSQSMessage(queue model.SQSQueue, message model.SQSMessage) string {
	body := message.Body
	var decoded any
	if json.Unmarshal([]byte(body), &decoded) == nil {
		if formatted, err := json.MarshalIndent(decoded, "", "  "); err == nil {
			body = string(formatted)
		}
	}
	result := fmt.Sprintf("SQS MESSAGE\n\nQueue       %s\nMessage ID  %s\nMD5         %s\nCaptured    %s\n\nBODY\n%s", queue.Name, message.MessageID, valueOrDash(message.MD5), formatSQSCapturedAt(message.CapturedAt), body)
	if len(message.Attributes) > 0 {
		result += "\n\nSYSTEM ATTRIBUTES"
		for _, name := range sortedSQSAttributeNames(message.Attributes) {
			result += fmt.Sprintf("\n%s  %s", name, message.Attributes[name])
		}
	}
	if len(message.UserAttributes) > 0 {
		result += "\n\nMESSAGE ATTRIBUTES"
		for _, name := range sortedSQSUserAttributeNames(message.UserAttributes) {
			attribute := message.UserAttributes[name]
			result += fmt.Sprintf("\n%s (%s)  %s", name, attribute.DataType, service.SQSMessageAttributeDisplay(attribute))
		}
	}
	return result
}

func sortedSQSAttributeNames(attributes map[string]string) []string {
	names := make([]string, 0, len(attributes))
	for name := range attributes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedSQSUserAttributeNames(attributes map[string]model.SQSMessageAttribute) []string {
	names := make([]string, 0, len(attributes))
	for name := range attributes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func findSQSMessage(messages []model.SQSMessage, messageID string) (model.SQSMessage, bool) {
	for _, message := range messages {
		if message.MessageID == messageID {
			return message, true
		}
	}
	return model.SQSMessage{}, false
}

func shortSQSMessageID(messageID string) string {
	if len(messageID) <= 12 {
		return messageID
	}
	return messageID[:12] + "…"
}

func (w *mainWindow) loadSQSQueueDetail(queue model.SQSQueue) {
	if w.options.SQS == nil || queue.URL == "" {
		return
	}
	ctx, generation := w.startRequest("Loading SQS queue configuration…")
	go func() {
		stats, err := w.options.SQS.Queue(ctx, queue.URL)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageSQSQueues || w.selectedSQSQueue != queue.URL {
				return
			}
			w.sqsQueueStats = stats
			w.setDetail(formatSQSQueue(queue, stats), detailSQSQueue)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) rebuildSQSRail() {
	if w.sqsModuleItems == nil {
		return
	}
	if w.savedSQSQueuesLabel != nil {
		w.sqsModuleItems.Remove(w.savedSQSQueuesLabel)
	}
	for _, button := range w.savedSQSQueueButtons {
		w.sqsModuleItems.Remove(button)
	}
	w.savedSQSQueueButtons = nil
	w.savedSQSQueuesLabel = nil
	savedQueues := w.options.ConfigSQSQueues()
	if len(savedQueues) == 0 {
		return
	}
	w.savedSQSQueuesLabel = newModuleRailSectionLabel("SAVED QUEUES")
	w.sqsModuleItems.Append(w.savedSQSQueuesLabel)
	for _, saved := range savedQueues {
		saved := saved
		button := newSavedModuleRailButton(saved.Name, func() { w.openSavedSQSQueue(saved) })
		button.SetGroup(w.clustersNavButton)
		button.SetTooltipText(saved.URL)
		w.sqsModuleItems.Append(button)
		w.savedSQSQueueButtons = append(w.savedSQSQueueButtons, button)
	}
}

func (w *mainWindow) promptSaveSQSQueue() {
	queue, found := findSQSQueue(w.allSQSQueues, w.selectedSQSQueue)
	if !found || w.options.Config == nil {
		return
	}
	dialog, entry := w.newSavedLogNameDialog("Save SQS queue", "Destination name", queue.Name)
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.AddCSSClass("error")
	dialog.ContentArea().Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Save", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		name := strings.TrimSpace(entry.Text())
		if name == "" {
			errorLabel.SetLabel("Enter a destination name")
			return
		}
		if _, exists := findSavedSQSQueue(w.options.Config.SQSQueues, name); exists {
			errorLabel.SetLabel("A saved queue already uses that name")
			return
		}
		if !w.mutateSQSConfig(func(cfg *config.Config) { cfg.AddSQSQueue(name, queue.URL) }) {
			return
		}
		dialog.Destroy()
		w.rebuildSQSRail()
		w.setBreadcrumb(sqsQueueBreadcrumb(w.activeSavedSQSQueue, queue.Name))
		w.updateActionSensitivity()
		w.setStatus("Saved SQS queue "+name, false)
	})
	dialog.Present()
}

func (w *mainWindow) promptManageSQSQueues() {
	if w.options.Config == nil || len(w.options.Config.SQSQueues) == 0 {
		return
	}
	savedQueues := append([]config.SQSQueueEntry(nil), w.options.Config.SQSQueues...)
	sort.SliceStable(savedQueues, func(i, j int) bool {
		return strings.ToLower(savedQueues[i].Name) < strings.ToLower(savedQueues[j].Name)
	})
	labels := make([]string, len(savedQueues))
	for i, saved := range savedQueues {
		labels[i] = saved.Name
	}
	dialog := gtk.NewDialogWithFlags("Saved SQS queues", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	selector := gtk.NewDropDownFromStrings(labels)
	nameEntry := gtk.NewEntry()
	urlEntry := gtk.NewEntry()
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.AddCSSClass("error")
	loadSelected := func() {
		index := int(selector.Selected())
		if index < 0 || index >= len(savedQueues) {
			return
		}
		nameEntry.SetText(savedQueues[index].Name)
		urlEntry.SetText(savedQueues[index].URL)
		errorLabel.SetLabel("")
	}
	selector.NotifyProperty("selected", loadSelected)
	loadSelected()
	content.Append(selector)
	for _, row := range []struct {
		label string
		entry *gtk.Entry
	}{{"Name", nameEntry}, {"Queue URL", urlEntry}} {
		label := gtk.NewLabel(row.label)
		label.SetXAlign(0)
		content.Append(label)
		content.Append(row.entry)
	}
	content.Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Open", 101)
	dialog.AddButton("Delete…", 102)
	dialog.AddButton("Apply", 103)
	dialog.ConnectResponse(func(response int) {
		index := int(selector.Selected())
		if index < 0 || index >= len(savedQueues) {
			dialog.Destroy()
			return
		}
		current := savedQueues[index]
		switch response {
		case 101:
			dialog.Destroy()
			w.openSavedSQSQueue(current)
		case 102:
			dialog.Destroy()
			w.confirmDeleteSavedSQSQueue(current)
		case 103:
			name, url := strings.TrimSpace(nameEntry.Text()), strings.TrimSpace(urlEntry.Text())
			if name == "" || url == "" {
				errorLabel.SetLabel("Name and queue URL are required")
				return
			}
			if name != current.Name {
				if _, exists := findSavedSQSQueue(w.options.Config.SQSQueues, name); exists {
					errorLabel.SetLabel("A saved queue already uses that name")
					return
				}
			}
			if !w.mutateSQSConfig(func(cfg *config.Config) {
				cfg.RemoveSQSQueue(current.Name)
				cfg.AddSQSQueue(name, url)
			}) {
				return
			}
			dialog.Destroy()
			w.rebuildSQSRail()
			if current.Name == w.activeSavedSQSQueue {
				w.openSavedSQSQueue(config.SQSQueueEntry{Name: name, URL: url})
			} else {
				w.updateActionSensitivity()
			}
			w.setStatus("Updated saved SQS queue "+name, false)
		default:
			dialog.Destroy()
		}
	})
	dialog.Present()
}

func (w *mainWindow) confirmDeleteSavedSQSQueue(saved config.SQSQueueEntry) {
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetMarkup("Delete saved SQS queue <b>" + html.EscapeString(saved.Name) + "</b>?")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Delete", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response != int(gtk.ResponseOK) {
			return
		}
		if !w.mutateSQSConfig(func(cfg *config.Config) { cfg.RemoveSQSQueue(saved.Name) }) {
			return
		}
		wasActive := saved.Name == w.activeSavedSQSQueue
		w.rebuildSQSRail()
		if wasActive {
			w.loadSQSQueues()
		} else {
			w.updateActionSensitivity()
		}
		w.setStatus("Deleted saved SQS queue "+saved.Name, false)
	})
	dialog.Present()
}

func (w *mainWindow) mutateSQSConfig(mutate func(*config.Config)) bool {
	if w.options.Config == nil {
		return false
	}
	before := append([]config.SQSQueueEntry(nil), w.options.Config.SQSQueues...)
	mutate(w.options.Config)
	if err := w.options.Config.Save(); err != nil {
		w.options.Config.SQSQueues = before
		w.setStatus("Save SQS queue: "+err.Error(), true)
		return false
	}
	return true
}

func (w *mainWindow) reloadSQSConfig() bool {
	if w.options.Config == nil || w.options.ReloadConfig == nil {
		return true
	}
	fresh := w.options.ReloadConfig()
	w.options.Config.SQSQueues = append([]config.SQSQueueEntry(nil), fresh.SQSQueues...)
	w.rebuildSQSRail()
	if w.activeSavedSQSQueue != "" {
		if _, found := findSavedSQSQueue(w.options.Config.SQSQueues, w.activeSavedSQSQueue); !found {
			w.loadSQSQueues()
			return false
		}
	}
	return true
}

func sqsQueueBreadcrumb(savedName, queueName string) string {
	root := "Queues"
	if savedName != "" {
		root = savedName
	}
	if queueName == "" {
		return "SQS / " + root
	}
	return "SQS / " + root + " / " + queueName
}

func sqsQueueListSummary(count int) string {
	if count == 0 {
		return "No SQS queues found."
	}
	return fmt.Sprintf("SQS QUEUES\n\nQueues  %d\n\nSelect a queue for configuration; double-click to browse its messages.", count)
}

func formatSQSQueue(queue model.SQSQueue, stats *model.SQSQueueStats) string {
	if stats == nil {
		return "SQS queue configuration is unavailable."
	}
	queueType := "Standard"
	if stats.IsFIFO {
		queueType = "FIFO"
	}
	result := fmt.Sprintf("SQS QUEUE\n\nName                %s\nURL                 %s\nType                %s\nMessages available  %d\nMessages in flight  %d\nMessages delayed    %d\nVisibility timeout  %s\nDefault delay       %s\nRetention period    %s\nMaximum message     %s",
		queue.Name, queue.URL, queueType, stats.MessagesAvailable, stats.MessagesInFlight, stats.MessagesDelayed,
		formatSQSDuration(stats.VisibilityTimeout), formatSQSDuration(stats.DelaySeconds), formatSQSDuration(stats.RetentionSeconds), formatS3Bytes(int64(stats.MaxMessageSize)))
	if stats.DeadLetterTargetARN != "" {
		result += fmt.Sprintf("\n\nDEAD LETTER QUEUE\nTarget ARN         %s\nMax receive count  %d", stats.DeadLetterTargetARN, stats.MaxReceiveCount)
	}
	return result
}

func formatSQSDuration(seconds int) string {
	if seconds <= 0 {
		return "0s"
	}
	parts := make([]string, 0, 4)
	for _, unit := range []struct {
		seconds int
		suffix  string
	}{{86400, "d"}, {3600, "h"}, {60, "m"}} {
		if value := seconds / unit.seconds; value > 0 {
			parts = append(parts, fmt.Sprintf("%d%s", value, unit.suffix))
			seconds %= unit.seconds
		}
	}
	if seconds > 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, "")
}

func findSQSQueue(queues []model.SQSQueue, url string) (model.SQSQueue, bool) {
	for _, queue := range queues {
		if queue.URL == url {
			return queue, true
		}
	}
	return model.SQSQueue{}, false
}

func findSQSQueueIndex(queues []model.SQSQueue, url string) int {
	for index, queue := range queues {
		if queue.URL == url {
			return index
		}
	}
	return -1
}

func findSavedSQSQueue(queues []config.SQSQueueEntry, name string) (config.SQSQueueEntry, bool) {
	for _, queue := range queues {
		if queue.Name == name {
			return queue, true
		}
	}
	return config.SQSQueueEntry{}, false
}
