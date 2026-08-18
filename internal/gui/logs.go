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
			w.showingLogs = true
			w.detailStack.SetVisibleChildName("logs")
			w.startLogFollow(source, false)
		})
	}()
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
		w.setStatus(fmt.Sprintf("Following %s • %d buffered lines", w.selectedService, w.logStore.len()), false)
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
