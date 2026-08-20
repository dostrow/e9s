//go:build gui

package gui

import (
	"fmt"
	"html"
	"sort"
	"strings"

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
	w.sqsQueueTable.selection.SetSelected(position)
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
	w.savedSQSQueuesLabel = newDynamoRailLabel("SAVED QUEUES")
	w.sqsModuleItems.Append(w.savedSQSQueuesLabel)
	for _, saved := range savedQueues {
		saved := saved
		button := newModuleRailButton(saved.Name, func() { w.openSavedSQSQueue(saved) })
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
		w.activeSavedSQSQueue = name
		w.rebuildSQSRail()
		w.setBreadcrumb(sqsQueueBreadcrumb(name, queue.Name))
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
