//go:build gui

package gui

import (
	"context"
	"fmt"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/dostrow/e9s/internal/highlight"
	"github.com/dostrow/e9s/internal/model"
)

const (
	maxGUILogEntries = 2000
	logPollInterval  = 2 * time.Second
	logPollOverlap   = 30 * time.Second
)

func (w *mainWindow) buildLogPane() gtk.Widgetter {
	back := gtk.NewButtonWithLabel("Back to details")
	back.ConnectClicked(w.closeLogs)
	w.logPauseButton = gtk.NewButtonWithLabel("Pause")
	w.logPauseButton.ConnectClicked(w.toggleLogFollow)
	w.logOlderButton = gtk.NewButtonWithLabel("Older")
	w.logOlderButton.SetVisible(false)
	w.logOlderButton.ConnectClicked(w.loadOlderLogEntries)
	w.logNewerButton = gtk.NewButtonWithLabel("Newer")
	w.logNewerButton.SetVisible(false)
	w.logNewerButton.ConnectClicked(w.loadNewerLogEntries)
	w.logCorrelateButton = gtk.NewButtonWithLabel("Correlate at cursor")
	w.logCorrelateButton.SetVisible(false)
	w.logCorrelateButton.ConnectClicked(w.promptLogCorrelation)
	w.logTimestampButton = gtk.NewButtonWithLabel("Time: Local")
	w.logTimestampButton.ConnectClicked(w.cycleLogTimestamps)
	w.logStreamsButton = gtk.NewButtonWithLabel("Streams…")
	w.logStreamsButton.SetVisible(false)
	w.logStreamsButton.ConnectClicked(w.promptLogStreams)
	w.logHighlightsButton = gtk.NewButtonWithLabel("Highlights…")
	w.logHighlightsButton.ConnectClicked(w.promptLogHighlights)
	copyButton := gtk.NewButtonWithLabel("Copy")
	copyButton.ConnectClicked(w.copyLogs)
	clearButton := gtk.NewButtonWithLabel("Clear")
	clearButton.ConnectClicked(w.clearLogs)
	exportButton := gtk.NewButtonWithLabel("Save buffer…")
	exportButton.ConnectClicked(w.exportLogs)
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
	toolbar.Append(w.logTimestampButton)
	toolbar.Append(w.logStreamsButton)
	toolbar.Append(w.logHighlightsButton)
	toolbar.Append(copyButton)
	toolbar.Append(clearButton)
	toolbar.Append(exportButton)
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
	w.showLogFollowFrom(source, title, time.Now().Add(-15*time.Minute).UnixMilli())
}

func (w *mainWindow) showLogFollowFrom(source model.LogSource, title string, startTime int64) {
	if w.showingTerminal {
		w.closeTerminalNow(false)
	}
	w.showingMetrics = false
	w.showingLogs = true
	w.detailStack.SetVisibleChildName("logs")
	w.logSource = source
	w.logTitle = title
	w.logSearchSpec = nil
	w.applyContextLogHighlights(nil, true)
	w.logPauseButton.SetVisible(true)
	w.updateLogSearchControls()
	w.logStore = newBoundedLogs(maxGUILogEntries)
	w.logHiddenStreams = nil
	w.logLastTS = max(int64(0), startTime)
	w.logSearch.SetText("")
	w.renderLogs()
	w.startLogFollowWithFallback(source, true)
	if path, ok := w.activeSavedLogPath(); ok {
		w.logHiddenStreams = stringSet(path.HiddenStreams)
		w.renderLogs()
	}
	w.setStatus("Following logs for "+title, false)
	w.updateActionSensitivity()
}

func (w *mainWindow) showLogSnapshot(source model.LogSource, title string, page model.LogPage) {
	w.logSearchSpec = nil
	w.showLogSnapshotData(source, title, page)
	w.updateActionSensitivity()
}

func (w *mainWindow) showLogSnapshotData(source model.LogSource, title string, page model.LogPage) {
	if w.logCancel != nil {
		w.logCancel()
	}
	w.logGeneration++
	w.setWorkspaceBusy("", false)
	w.logNewerKnown = 0
	w.logNewestKnownTS = 0
	w.logHiddenStreams = nil
	w.logFollowing = false
	w.logSource = source
	w.logTitle = title
	var rules []model.LogHighlightRule
	preferSaved := w.logSearchSpec == nil
	if w.logSearchSpec != nil {
		rules = w.logSearchSpec.HighlightRules
	}
	w.applyContextLogHighlights(rules, preferSaved)
	w.logStore = newBoundedLogs(maxGUILogEntries)
	w.logStore.append(page.Entries)
	w.logLastTS = page.LastTimestamp
	w.logSearch.SetText("")
	w.logPauseButton.SetVisible(false)
	w.showingMetrics = false
	w.showingLogs = true
	w.detailStack.SetVisibleChildName("logs")
	w.updateLogSearchControls()
	w.renderLogs()
}

func (w *mainWindow) updateLogSearchControls() {
	searchResult := w.logSearchSpec != nil
	if w.logOlderButton != nil {
		w.logOlderButton.SetVisible(w.showingLogs)
		w.logOlderButton.SetSensitive(w.showingLogs)
		w.logNewerButton.SetVisible(searchResult || w.logNewerKnown > 0)
		w.logNewerButton.SetSensitive(w.showingLogs)
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
	if !preserve || w.logStore == nil {
		w.logStore = newBoundedLogs(maxGUILogEntries)
		w.logHiddenStreams = nil
		w.logLastTS = time.Now().Add(-15 * time.Minute).UnixMilli()
		w.logSearch.SetText("")
		w.renderLogs()
	}
	w.startLogFollowWithFallback(source, !preserve)
}

func (w *mainWindow) startLogFollowWithFallback(source model.LogSource, allowFallback bool) {
	if w.logCancel != nil {
		w.logCancel()
	}
	w.setWorkspaceBusy("Loading log events…", true)
	w.logNewerKnown = 0
	w.logNewestKnownTS = 0
	if w.logOlderButton != nil {
		w.logOlderButton.SetSensitive(false)
	}
	ctx, cancel := context.WithCancel(w.ctx)
	w.logCancel = cancel
	w.logGeneration++
	generation := w.logGeneration
	w.logSource = source
	w.logFollowing = true
	w.logPauseButton.SetVisible(true)
	w.logPauseButton.SetLabel("Pause")
	if w.logStore == nil {
		w.logStore = newBoundedLogs(maxGUILogEntries)
		w.logHiddenStreams = nil
		w.logLastTS = time.Now().Add(-15 * time.Minute).UnixMilli()
		w.logSearch.SetText("")
		w.renderLogs()
	}
	startTime := w.logLastTS

	go w.followLogs(ctx, generation, source, startTime, allowFallback)
}

func (w *mainWindow) followLogs(ctx context.Context, generation uint64, source model.LogSource, startTime int64, allowFallback bool) {
	next := startTime
	for {
		fallbackLimit := 0
		queryStart := next
		if allowFallback {
			fallbackLimit = 10
		} else {
			queryStart = max(int64(0), next-logPollOverlap.Milliseconds())
		}
		page, err := w.options.Logs.Fetch(ctx, source.Group, model.LogQuery{
			Streams:       append([]string(nil), source.Streams...),
			StartTime:     queryStart,
			Limit:         500,
			Tail:          true,
			FallbackLimit: fallbackLimit,
		})
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.applyLogError(ctx, generation, err)
		} else {
			allowFallback = false
			if page.LastTimestamp >= next {
				next = page.LastTimestamp
			} else if page.UsedFallback {
				next = page.LastTimestamp
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
		w.setWorkspaceBusy("", false)
		w.logOlderButton.SetSensitive(true)
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
			w.setWorkspaceBusy("", false)
			w.logOlderButton.SetSensitive(true)
			w.setStatus("Log follow: "+err.Error(), true)
		}
	})
}

func (w *mainWindow) loadOlderLogEntries() {
	if w.logSearchSpec != nil {
		w.loadAdjacentCloudWatchRange(-1)
		return
	}
	if !w.showingLogs || w.options.Logs == nil || w.logStore == nil || w.logSource.Group == "" {
		return
	}
	cutoff := w.logStore.firstTimestamp()
	if cutoff <= 0 {
		return
	}
	if w.logFollowing {
		if w.logCancel != nil {
			w.logCancel()
		}
		w.logGeneration++
		w.logFollowing = false
		w.setWorkspaceBusy("", false)
		w.logOlderButton.SetSensitive(true)
		w.logPauseButton.SetLabel("Resume")
	}
	if w.logCancel != nil {
		w.logCancel()
	}
	ctx, cancel := context.WithCancel(w.ctx)
	w.logCancel = cancel
	w.logGeneration++
	generation := w.logGeneration
	w.logOlderButton.SetSensitive(false)
	w.spinner.Start()
	w.setStatus("Loading older log events…", false)
	w.setWorkspaceBusy("Loading older log events…", true)
	go func() {
		page, err := w.options.Logs.Fetch(ctx, w.logSource.Group, model.LogQuery{
			Streams:    append([]string(nil), w.logSource.Streams...),
			BeforeTime: cutoff,
			Limit:      500,
		})
		glib.IdleAdd(func() {
			if ctx.Err() != nil || generation != w.logGeneration || !w.showingLogs {
				return
			}
			w.spinner.Stop()
			w.setWorkspaceBusy("", false)
			w.logOlderButton.SetSensitive(true)
			if err != nil {
				w.setStatus("Loading older logs failed: "+err.Error(), true)
				return
			}
			if len(page.Entries) == 0 {
				w.setStatus("No log events exist before "+time.UnixMilli(cutoff).Local().Format("2006-01-02 15:04:05.000"), false)
				return
			}
			previousNewest := w.logStore.lastTimestamp()
			added, evicted := w.logStore.prepend(page.Entries)
			if evicted > 0 {
				w.logNewerKnown += evicted
				w.logNewestKnownTS = max(w.logNewestKnownTS, previousNewest)
			}
			w.updateLogSearchControls()
			w.renderLogs()
			w.logView.ScrollToIter(w.logTextBuffer.StartIter(), 0, false, 0, 0)
			status := fmt.Sprintf("Loaded %d older log events", added)
			if evicted > 0 {
				status += fmt.Sprintf(" • dropped %d newest", evicted)
			}
			status += fmt.Sprintf(" • %d buffered lines • %s–%s", w.logStore.len(),
				formatLogTimestamp(w.logStore.firstTimestamp(), w.logTimestampMode, time.Now()),
				formatLogTimestamp(w.logStore.lastTimestamp(), w.logTimestampMode, time.Now()))
			w.setStatus(status, false)
		})
	}()
}

func (w *mainWindow) loadNewerLogEntries() {
	if w.logSearchSpec != nil {
		w.loadAdjacentCloudWatchRange(1)
		return
	}
	if !w.showingLogs || w.options.Logs == nil || w.logNewerKnown <= 0 || w.logNewestKnownTS <= 0 || w.logStore == nil || w.logSource.Group == "" {
		return
	}
	cutoff := w.logStore.lastTimestamp()
	if cutoff > w.logNewestKnownTS {
		w.logNewerKnown = 0
		w.updateLogSearchControls()
		return
	}
	frontier := w.logNewestKnownTS
	batchSize := min(500, w.logNewerKnown)
	queryLimit := batchSize + w.logStore.countTimestamp(cutoff)
	source := model.LogSource{
		Group:   w.logSource.Group,
		Streams: append([]string(nil), w.logSource.Streams...),
	}
	if w.logCancel != nil {
		w.logCancel()
	}
	ctx, cancel := context.WithCancel(w.ctx)
	w.logCancel = cancel
	w.logGeneration++
	generation := w.logGeneration
	w.logOlderButton.SetSensitive(false)
	w.logNewerButton.SetSensitive(false)
	w.spinner.Start()
	w.setStatus("Loading newer log events…", false)
	w.setWorkspaceBusy("Loading newer log events…", true)
	go func() {
		page, err := w.options.Logs.Fetch(ctx, source.Group, model.LogQuery{
			Streams:   source.Streams,
			AfterTime: cutoff,
			UntilTime: frontier,
			Limit:     queryLimit,
		})
		glib.IdleAdd(func() {
			if ctx.Err() != nil || generation != w.logGeneration || !w.showingLogs {
				return
			}
			w.spinner.Stop()
			w.setWorkspaceBusy("", false)
			w.logOlderButton.SetSensitive(true)
			if err != nil {
				w.logNewerButton.SetSensitive(true)
				w.setStatus("Loading newer logs failed: "+err.Error(), true)
				return
			}
			if len(page.Entries) == 0 {
				w.logNewerKnown = 0
				w.updateLogSearchControls()
				w.setStatus("No newer buffered-history events remain", false)
				return
			}
			added, evicted := w.logStore.appendNewer(page.Entries)
			if added == 0 {
				w.logNewerKnown = 0
			} else {
				w.logNewerKnown = max(0, w.logNewerKnown-added)
			}
			w.updateLogSearchControls()
			w.renderLogs()
			w.logView.ScrollToIter(w.logTextBuffer.EndIter(), 0, false, 0, 1)
			status := fmt.Sprintf("Loaded %d newer log events", added)
			if evicted > 0 {
				status += fmt.Sprintf(" • dropped %d oldest", evicted)
			}
			status += fmt.Sprintf(" • %d buffered lines • %s–%s", w.logStore.len(),
				formatLogTimestamp(w.logStore.firstTimestamp(), w.logTimestampMode, time.Now()),
				formatLogTimestamp(w.logStore.lastTimestamp(), w.logTimestampMode, time.Now()))
			w.setStatus(status, false)
		})
	}()
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
		w.setWorkspaceBusy("", false)
		w.logOlderButton.SetSensitive(true)
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
	formatted := w.formatDisplayedLogs(w.logSearch.Text())
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
	w.applyLogHighlightTags(formatted)
	w.updateLogStreamsButton()
	if w.logFollowing {
		w.logView.ScrollToIter(w.logTextBuffer.EndIter(), 0, false, 0, 1)
	}
}

func (w *mainWindow) scrollLogEntryToCenter(entry model.LogEntry) {
	if w.logStore == nil || w.logView == nil || w.logTextBuffer == nil {
		return
	}
	formatted := w.formatDisplayedLogs(w.logSearch.Text())
	key := entry.Key()
	for _, line := range formatted.lines {
		if line.entry.Key() == key {
			w.logView.ScrollToIter(w.logTextBuffer.IterAtOffset(line.start), 0, true, 0, 0.5)
			return
		}
	}
}

func (w *mainWindow) formatDisplayedLogs(filter string) formattedLogBuffer {
	return w.logStore.formatDisplayWithTimestamps(
		filter,
		w.logTimestampMode,
		time.Now(),
		w.logHiddenStreams,
		logSourceShowsStreamLabels(w.logSource),
	)
}

func (w *mainWindow) updateLogStreamsButton() {
	if w.logStreamsButton == nil {
		return
	}
	if w.logStore == nil {
		w.logStreamsButton.SetVisible(false)
		return
	}
	streams := w.logStore.streams()
	if len(streams) <= 1 {
		w.logStreamsButton.SetVisible(false)
		return
	}
	hidden := 0
	for _, stream := range streams {
		if _, excluded := w.logHiddenStreams[stream]; excluded {
			hidden++
		}
	}
	w.logStreamsButton.SetLabel(fmt.Sprintf("Streams: %d/%d…", len(streams)-hidden, len(streams)))
	w.logStreamsButton.SetVisible(true)
}

func (w *mainWindow) promptLogStreams() {
	if w.logStore == nil {
		return
	}
	streams := w.logStore.streams()
	if len(streams) <= 1 {
		return
	}
	dialog := gtk.NewDialogWithFlags("Visible log streams", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)

	help := gtk.NewLabel("Choose which streams are shown. Hidden streams remain buffered and are included when copying or saving.")
	help.SetXAlign(0)
	help.SetWrap(true)
	content.Append(help)

	checks := make([]*gtk.CheckButton, len(streams))
	list := gtk.NewBox(gtk.OrientationVertical, 4)
	for i, stream := range streams {
		check := gtk.NewCheckButtonWithLabel(stream)
		_, hidden := w.logHiddenStreams[stream]
		check.SetActive(!hidden)
		checks[i] = check
		list.Append(check)
	}
	scroll := gtk.NewScrolledWindow()
	scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	scroll.SetMinContentWidth(560)
	scroll.SetMinContentHeight(360)
	scroll.SetChild(list)
	content.Append(scroll)

	selectionButtons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	showAll := gtk.NewButtonWithLabel("Show all")
	showAll.ConnectClicked(func() {
		for _, check := range checks {
			check.SetActive(true)
		}
	})
	hideAll := gtk.NewButtonWithLabel("Hide all")
	hideAll.ConnectClicked(func() {
		for _, check := range checks {
			check.SetActive(false)
		}
	})
	selectionButtons.Append(showAll)
	selectionButtons.Append(hideAll)
	content.Append(selectionButtons)

	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Apply", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		defer dialog.Destroy()
		if response != int(gtk.ResponseOK) {
			return
		}
		hidden := make(map[string]struct{})
		for i, check := range checks {
			if !check.Active() {
				hidden[streams[i]] = struct{}{}
			}
		}
		w.logHiddenStreams = hidden
		w.renderLogs()
		w.updateActionSensitivity()
		w.setStatus(fmt.Sprintf("Showing %d of %d streams • %d buffered lines", len(streams)-len(hidden), len(streams), w.logStore.len()), false)
	})
	dialog.Present()
}

func (w *mainWindow) applyContextLogHighlights(rules []model.LogHighlightRule, preferSaved bool) {
	if preferSaved {
		if path, ok := w.activeSavedLogPath(); ok {
			rules = path.HighlightRules
		}
	}
	if err := w.setLogHighlightRules(rules); err != nil {
		w.logHighlightRules = nil
		w.updateLogHighlightButton()
		w.setStatus("Log highlights: "+err.Error(), true)
	}
}

func (w *mainWindow) setLogHighlightRules(rules []model.LogHighlightRule) error {
	if _, err := highlight.Compile(rules); err != nil {
		return err
	}
	w.logHighlightRules = append([]model.LogHighlightRule(nil), rules...)
	if w.logSearchSpec != nil {
		w.logSearchSpec.HighlightRules = append([]model.LogHighlightRule(nil), rules...)
	}
	w.updateLogHighlightButton()
	return nil
}

func (w *mainWindow) updateLogHighlightButton() {
	if w.logHighlightsButton == nil {
		return
	}
	if len(w.logHighlightRules) == 0 {
		w.logHighlightsButton.SetLabel("Highlights…")
		return
	}
	w.logHighlightsButton.SetLabel(fmt.Sprintf("Highlights (%d)…", len(w.logHighlightRules)))
}

func (w *mainWindow) applyLogHighlightTags(formatted formattedLogBuffer) {
	spans, err := formatLogHighlights(formatted, w.logHighlightRules)
	if err != nil || len(w.logHighlightRules) == 0 {
		return
	}
	w.ensureLogHighlightTags()
	for _, span := range spans {
		tag := w.logHighlightTags[span.style]
		if tag == nil {
			continue
		}
		w.logTextBuffer.ApplyTag(
			tag,
			w.logTextBuffer.IterAtOffset(span.start),
			w.logTextBuffer.IterAtOffset(span.end),
		)
	}
}

func (w *mainWindow) ensureLogHighlightTags() {
	if w.logHighlightTags == nil {
		w.logHighlightTags = make(map[model.LogHighlightStyle]*gtk.TextTag)
		for _, style := range []model.LogHighlightStyle{
			model.LogHighlightDefault, model.LogHighlightInfo, model.LogHighlightSuccess,
			model.LogHighlightWarning, model.LogHighlightError,
		} {
			tag := gtk.NewTextTag("")
			tag.SetObjectProperty("weight", int(pango.WeightBold))
			w.logTextBuffer.TagTable().Add(tag)
			w.logHighlightTags[style] = tag
		}
	}
	styleContext := w.logView.StyleContext()
	palette := semanticPaletteFromStyle(styleContext)
	colors := map[model.LogHighlightStyle]gdk.RGBA{
		model.LogHighlightInfo:    palette.info,
		model.LogHighlightSuccess: palette.success,
		model.LogHighlightWarning: palette.warning,
		model.LogHighlightError:   palette.error,
	}
	for style, color := range colors {
		w.logHighlightTags[style].SetObjectProperty("foreground", color.String())
	}
	accentColor := themeColorOr(styleContext, palette.accent,
		"accent_bg_color", "theme_selected_bg_color", "accent_color")
	accent := accentColor.Copy()
	accent.SetAlpha(0.28)
	w.logHighlightTags[model.LogHighlightDefault].SetObjectProperty("background", accent.String())
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
	w.logView.Clipboard().SetText(w.logStore.formatWithTimestamps("", w.logTimestampMode, time.Now()).text)
	w.setStatus(fmt.Sprintf("Copied %d log lines", w.logStore.len()), false)
}

func (w *mainWindow) clearLogs() {
	if !w.showingLogs || w.logStore == nil {
		return
	}
	w.logStore.clear()
	w.logNewerKnown = 0
	w.logNewestKnownTS = 0
	w.logHiddenStreams = nil
	w.updateLogSearchControls()
	w.renderLogs()
	w.setStatus("Log buffer cleared", false)
}

func (w *mainWindow) logEntryAtCursor() (model.LogEntry, bool) {
	if w.logStore == nil || w.logTextBuffer == nil {
		return model.LogEntry{}, false
	}
	offset := w.logTextBuffer.IterAtMark(w.logTextBuffer.GetInsert()).Offset()
	formatted := w.formatDisplayedLogs(w.logSearch.Text())
	for _, line := range formatted.lines {
		if offset >= line.start && offset <= line.end {
			return line.entry, true
		}
	}
	return model.LogEntry{}, false
}

func (w *mainWindow) cycleLogTimestamps() {
	w.logTimestampMode = (w.logTimestampMode + 1) % 3
	labels := []string{"Time: Local", "Time: UTC", "Time: Relative"}
	w.logTimestampButton.SetLabel(labels[w.logTimestampMode])
	w.renderLogs()
}

func (w *mainWindow) exportLogs() {
	if !w.showingLogs || w.logStore == nil || w.logStore.len() == 0 {
		w.setStatus("There are no buffered logs to save", false)
		return
	}
	contents := w.logStore.formatWithTimestamps("", w.logTimestampMode, time.Now()).text
	lineCount := w.logStore.len()
	dialog := gtk.NewFileDialog()
	dialog.SetTitle("Save log buffer")
	dialog.SetAcceptLabel("Save")
	dialog.SetInitialName("e9s-logs.txt")
	dialog.Save(w.ctx, &w.window.Window, func(result gio.AsyncResulter) {
		file, err := dialog.SaveFinish(result)
		if err != nil || file == nil {
			return
		}
		go func() {
			_, writeErr := file.ReplaceContents(w.ctx, contents, "", false, gio.FileCreateReplaceDestination)
			glib.IdleAdd(func() {
				if writeErr != nil {
					w.setStatus("Save log buffer: "+writeErr.Error(), true)
					return
				}
				w.setStatus(fmt.Sprintf("Saved %d buffered log lines", lineCount), false)
			})
		}()
	})
}

func (w *mainWindow) closeLogs() {
	if !w.showingLogs {
		return
	}
	if w.logCancel != nil {
		w.logCancel()
	}
	w.logGeneration++
	w.setWorkspaceBusy("", false)
	w.logFollowing = false
	w.logNewerKnown = 0
	w.logNewestKnownTS = 0
	w.showingLogs = false
	w.logHighlightRules = nil
	w.updateLogHighlightButton()
	w.detailStack.SetVisibleChildName("detail")
	w.setStatus("Log follow stopped", false)
	w.updateActionSensitivity()
}
