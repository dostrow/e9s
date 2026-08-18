//go:build gui

package gui

import (
	"context"
	"fmt"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
)

const (
	maxGUILogEntries = 2000
	logPollInterval  = 2 * time.Second
)

func (w *mainWindow) buildLogPane() gtk.Widgetter {
	back := gtk.NewButtonWithLabel("Back to details")
	back.ConnectClicked(w.closeLogs)
	w.logPauseButton = gtk.NewButtonWithLabel("Pause")
	w.logPauseButton.ConnectClicked(w.toggleLogFollow)
	w.logOlderButton = gtk.NewButtonWithLabel("Older")
	w.logOlderButton.ConnectClicked(func() { w.loadAdjacentCloudWatchRange(-1) })
	w.logNewerButton = gtk.NewButtonWithLabel("Newer")
	w.logNewerButton.ConnectClicked(func() { w.loadAdjacentCloudWatchRange(1) })
	w.logCorrelateButton = gtk.NewButtonWithLabel("Correlate at cursor")
	w.logCorrelateButton.ConnectClicked(w.promptLogCorrelation)
	copyButton := gtk.NewButtonWithLabel("Copy")
	copyButton.ConnectClicked(w.copyLogs)
	clearButton := gtk.NewButtonWithLabel("Clear")
	clearButton.ConnectClicked(w.clearLogs)
	w.logSearch = gtk.NewSearchEntry()
	w.logSearch.SetPlaceholderText("Filter buffered logs…")
	w.logSearch.SetHExpand(true)
	w.logSearch.ConnectSearchChanged(w.renderLogs)

	toolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	toolbar.AddCSSClass("log-toolbar")
	toolbar.Append(back)
	toolbar.Append(w.logPauseButton)
	toolbar.Append(w.logOlderButton)
	toolbar.Append(w.logNewerButton)
	toolbar.Append(w.logCorrelateButton)
	toolbar.Append(copyButton)
	toolbar.Append(clearButton)
	toolbar.Append(w.logSearch)

	w.logTextBuffer = gtk.NewTextBuffer(nil)
	w.logView = gtk.NewTextViewWithBuffer(w.logTextBuffer)
	w.logView.SetEditable(false)
	w.logView.SetCursorVisible(false)
	w.logView.SetMonospace(true)
	w.logView.SetWrapMode(gtk.WrapWordChar)
	w.logView.AddCSSClass("log-view")
	logScroll := gtk.NewScrolledWindow()
	logScroll.SetVExpand(true)
	logScroll.SetHExpand(true)
	logScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	logScroll.SetChild(w.logView)

	pane := gtk.NewBox(gtk.OrientationVertical, 0)
	pane.Append(toolbar)
	pane.Append(logScroll)
	return pane
}

func (w *mainWindow) openServiceLogs() {
	if w.selectedCluster == "" || w.selectedService == "" || w.options.Logs == nil {
		return
	}

	ctx, generation := w.startRequest("Resolving service log streams…")
	cluster, service := w.selectedCluster, w.selectedService
	go func() {
		source, err := w.options.ECS.ServiceLogSource(ctx, cluster, service)
		w.finishRequestWithStatus(ctx, generation, err, "Following logs for "+service, func() {
			w.showLogFollow(source, service)
		})
	}()
}

func (w *mainWindow) openTaskLogs() {
	if w.selectedTask == "" || w.options.Logs == nil {
		return
	}
	task, found := findTask(w.allTasks, w.selectedTask)
	if !found {
		w.setStatus("The selected task is no longer available", true)
		return
	}
	names := taskContainerNames(task)
	if len(names) == 0 {
		w.setStatus("The selected task has no containers", true)
		return
	}
	if len(names) == 1 {
		w.openTaskContainerLogs(task, names[0])
		return
	}

	dialog := gtk.NewDialogWithFlags("Choose a container", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(12)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	label := gtk.NewLabel("Follow logs from container:")
	label.SetXAlign(0)
	selector := gtk.NewDropDownFromStrings(names)
	selector.SetHExpand(true)
	content.Append(label)
	content.Append(selector)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Follow logs", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		selected := -1
		if response == int(gtk.ResponseOK) {
			selected = int(selector.Selected())
		}
		dialog.Destroy()
		if selected >= 0 && selected < len(names) {
			w.openTaskContainerLogs(task, names[selected])
		}
	})
	dialog.Present()
}

func (w *mainWindow) openTaskContainerLogs(task model.Task, container string) {
	ctx, generation := w.startRequest("Resolving logs for " + container + "…")
	go func() {
		source, err := w.options.ECS.ContainerLogSource(ctx, task, container)
		title := shortID(task.TaskID) + " / " + container
		w.finishRequestWithStatus(ctx, generation, err, "Following logs for "+title, func() {
			w.showLogFollow(source, title)
		})
	}()
}

func (w *mainWindow) showLogFollow(source model.LogSource, title string) {
	if w.showingTerminal {
		w.closeTerminalNow(false)
	}
	w.showingMetrics = false
	w.showingLogs = true
	w.detailStack.SetVisibleChildName("logs")
	w.logTitle = title
	w.logSearchSpec = nil
	w.logPauseButton.SetVisible(true)
	w.updateLogSearchControls()
	w.startLogFollow(source, false)
	w.setStatus("Following logs for "+title, false)
}

func (w *mainWindow) showLogSnapshot(source model.LogSource, title string, page model.LogPage) {
	w.logSearchSpec = nil
	w.showLogSnapshotData(source, title, page)
}

func (w *mainWindow) showLogSnapshotData(source model.LogSource, title string, page model.LogPage) {
	if w.logCancel != nil {
		w.logCancel()
	}
	w.logGeneration++
	w.logFollowing = false
	w.logSource = source
	w.logTitle = title
	w.logStore = newBoundedLogs(maxGUILogEntries)
	w.logStore.append(page.Entries)
	w.logLastTS = page.LastTimestamp
	w.logSearch.SetText("")
	w.logPauseButton.SetVisible(false)
	w.updateLogSearchControls()
	w.showingMetrics = false
	w.showingLogs = true
	w.detailStack.SetVisibleChildName("logs")
	w.renderLogs()
}

func (w *mainWindow) updateLogSearchControls() {
	searchResult := w.logSearchSpec != nil
	if w.logOlderButton != nil {
		w.logOlderButton.SetVisible(searchResult)
		w.logNewerButton.SetVisible(searchResult)
		w.logCorrelateButton.SetVisible(searchResult)
	}
	if w.logView != nil {
		w.logView.SetCursorVisible(searchResult)
	}
}

func taskContainerNames(task model.Task) []string {
	names := make([]string, 0, len(task.Containers))
	for _, container := range task.Containers {
		if container.Name != "" {
			names = append(names, container.Name)
		}
	}
	return names
}

func (w *mainWindow) startLogFollow(source model.LogSource, preserve bool) {
	if w.logCancel != nil {
		w.logCancel()
	}
	ctx, cancel := context.WithCancel(w.ctx)
	w.logCancel = cancel
	w.logGeneration++
	generation := w.logGeneration
	w.logSource = source
	w.logFollowing = true
	w.logPauseButton.SetVisible(true)
	w.logPauseButton.SetLabel("Pause")
	if !preserve || w.logStore == nil {
		w.logStore = newBoundedLogs(maxGUILogEntries)
		w.logLastTS = time.Now().Add(-10 * time.Second).UnixMilli()
		w.logSearch.SetText("")
		w.renderLogs()
	}
	startTime := w.logLastTS

	go w.followLogs(ctx, generation, source, startTime)
}

func (w *mainWindow) followLogs(ctx context.Context, generation uint64, source model.LogSource, startTime int64) {
	next := startTime
	for {
		page, err := w.options.Logs.Fetch(ctx, source.Group, model.LogQuery{
			Streams:   append([]string(nil), source.Streams...),
			StartTime: next,
			Limit:     500,
			Tail:      true,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.applyLogError(ctx, generation, err)
		} else {
			if page.LastTimestamp >= next {
				next = page.LastTimestamp + 1
			}
			w.applyLogPage(ctx, generation, page, next)
		}

		timer := time.NewTimer(logPollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func (w *mainWindow) applyLogPage(ctx context.Context, generation uint64, page model.LogPage, next int64) {
	glib.IdleAdd(func() {
		if ctx.Err() != nil || generation != w.logGeneration || !w.showingLogs {
			return
		}
		w.logLastTS = next
		if len(page.Entries) > 0 {
			w.logStore.append(page.Entries)
			w.renderLogs()
		}
		w.setStatus(fmt.Sprintf("Following %s • %d buffered lines", w.logTitle, w.logStore.len()), false)
	})
}

func (w *mainWindow) applyLogError(ctx context.Context, generation uint64, err error) {
	glib.IdleAdd(func() {
		if ctx.Err() == nil && generation == w.logGeneration && w.showingLogs {
			w.setStatus("Log follow: "+err.Error(), true)
		}
	})
}

func (w *mainWindow) toggleLogFollow() {
	if !w.showingLogs || w.logSource.Group == "" {
		return
	}
	if w.logFollowing {
		if w.logCancel != nil {
			w.logCancel()
		}
		w.logGeneration++
		w.logFollowing = false
		w.logPauseButton.SetLabel("Resume")
		w.setStatus(fmt.Sprintf("Logs paused • %d buffered lines", w.logStore.len()), false)
		return
	}
	w.startLogFollow(w.logSource, true)
	w.setStatus("Log follow resumed", false)
}

func (w *mainWindow) renderLogs() {
	if w.logStore == nil || w.logTextBuffer == nil {
		return
	}
	formatted := w.logStore.format(w.logSearch.Text())
	w.logTextBuffer.SetText(formatted.text)
	if w.logIndentTags == nil {
		w.logIndentTags = make(map[int]*gtk.TextTag)
	}
	prefixWidths := make(map[string]int)
	for _, line := range formatted.lines {
		prefixWidth, measured := prefixWidths[line.prefix]
		if !measured {
			layout := w.logView.CreatePangoLayout(line.prefix)
			prefixWidth, _ = layout.PixelSize()
			prefixWidths[line.prefix] = prefixWidth
		}
		tag := w.logIndentTags[prefixWidth]
		if tag == nil {
			tag = newHangingIndentTag(prefixWidth)
			w.logTextBuffer.TagTable().Add(tag)
			w.logIndentTags[prefixWidth] = tag
		}
		w.logTextBuffer.ApplyTag(
			tag,
			w.logTextBuffer.IterAtOffset(line.start),
			w.logTextBuffer.IterAtOffset(line.end),
		)
	}
	if w.logFollowing {
		w.logView.ScrollToIter(w.logTextBuffer.EndIter(), 0, false, 0, 1)
	}
}

func newHangingIndentTag(prefixWidth int) *gtk.TextTag {
	tag := gtk.NewTextTag("")
	tag.SetObjectProperty("indent", -prefixWidth)
	return tag
}

func (w *mainWindow) copyLogs() {
	if !w.showingLogs || w.logStore == nil || w.logStore.len() == 0 {
		return
	}
	w.logView.Clipboard().SetText(w.logStore.text(""))
	w.setStatus(fmt.Sprintf("Copied %d log lines", w.logStore.len()), false)
}

func (w *mainWindow) clearLogs() {
	if !w.showingLogs || w.logStore == nil {
		return
	}
	w.logStore.clear()
	w.renderLogs()
	w.setStatus("Log buffer cleared", false)
}

func (w *mainWindow) logEntryAtCursor() (model.LogEntry, bool) {
	if w.logStore == nil || w.logTextBuffer == nil {
		return model.LogEntry{}, false
	}
	offset := w.logTextBuffer.IterAtMark(w.logTextBuffer.GetInsert()).Offset()
	formatted := w.logStore.format(w.logSearch.Text())
	for _, line := range formatted.lines {
		if offset >= line.start && offset <= line.end {
			return line.entry, true
		}
	}
	return model.LogEntry{}, false
}

func (w *mainWindow) closeLogs() {
	if !w.showingLogs {
		return
	}
	if w.logCancel != nil {
		w.logCancel()
	}
	w.logGeneration++
	w.logFollowing = false
	w.showingLogs = false
	w.detailStack.SetVisibleChildName("detail")
	w.setStatus("Log follow stopped", false)
}
