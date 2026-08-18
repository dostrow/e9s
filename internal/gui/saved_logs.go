//go:build gui

package gui

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

func (o Options) ConfigLogPaths() []config.LogPathEntry {
	if o.Config == nil {
		return nil
	}
	return o.Config.LogPaths
}

func (w *mainWindow) activeSavedLogPath() (config.LogPathEntry, bool) {
	for _, path := range w.options.ConfigLogPaths() {
		if path.Name == w.activeSavedLog {
			return path, true
		}
	}
	return config.LogPathEntry{}, false
}

func (w *mainWindow) rebuildSavedLogRail() {
	if w.cloudWatchModuleItems == nil {
		return
	}
	if w.savedLogsLabel != nil {
		w.cloudWatchModuleItems.Remove(w.savedLogsLabel)
	}
	for _, button := range w.savedLogNavButtons {
		w.cloudWatchModuleItems.Remove(button)
	}
	w.savedLogsLabel = nil
	w.savedLogNavButtons = nil

	paths := w.options.ConfigLogPaths()
	if len(paths) == 0 {
		return
	}
	w.savedLogsLabel = gtk.NewLabel("SAVED SEARCHES")
	w.savedLogsLabel.SetXAlign(0)
	w.savedLogsLabel.AddCSSClass("section-title")
	w.cloudWatchModuleItems.Append(w.savedLogsLabel)
	for _, path := range paths {
		path := path
		button := newModuleRailButton(path.Name, func() { w.openSavedLog(path) })
		button.SetGroup(w.clustersNavButton)
		button.SetTooltipText(savedLogTooltip(path))
		w.cloudWatchModuleItems.Append(button)
		w.savedLogNavButtons = append(w.savedLogNavButtons, button)
	}
}

func (w *mainWindow) reloadSavedLogConfig() bool {
	if w.options.Config == nil || w.options.ReloadConfig == nil {
		return true
	}
	fresh := w.options.ReloadConfig()
	w.options.Config.LogPaths = cloneLogPaths(fresh.LogPaths)
	w.rebuildSavedLogRail()
	if w.activeSavedLog == "" {
		w.updateActionSensitivity()
		return true
	}
	if _, found := w.activeSavedLogPath(); found {
		w.updateActionSensitivity()
		return true
	}
	w.activeSavedLog = ""
	w.loadLogGroups()
	w.setStatus("The active saved CloudWatch destination was removed from the configuration", false)
	return false
}

func (w *mainWindow) openSavedLog(path config.LogPathEntry) {
	groups := savedLogGroups(path)
	if len(groups) == 0 {
		w.setStatus(fmt.Sprintf("Saved search %q has no log group", path.Name), true)
		return
	}
	streams := savedLogStreams(path)
	if savedLogHasQuery(path) {
		spec, err := searchFromSavedLog(path, time.Now())
		if err != nil {
			w.setStatus(err.Error(), true)
			return
		}
		w.prepareSavedLogPage(path, groups, streams)
		w.runCloudWatchSearch(spec)
		return
	}
	if len(groups) == 1 && len(streams) == 0 {
		w.loadLogStreams(groups[0])
		w.activeSavedLog = path.Name
		w.updateActionSensitivity()
		return
	}

	w.prepareSavedLogPage(path, groups, streams)
	if len(groups) == 1 && len(streams) == 1 {
		w.showLogFollow(model.LogSource{Group: groups[0], Streams: streams}, path.Name)
		return
	}
	w.setDetail("SAVED CLOUDWATCH DESTINATION\n\n"+savedLogTooltip(path)+
		"\n\nUse Search logs to choose a filter and time range.", detailLogGroup)
}

func (w *mainWindow) prepareSavedLogPage(path config.LogPathEntry, groups, streams []string) {
	w.resetWorkspaceForBrowserChange()
	w.clearLogGroupBrowser()
	w.clearLogStreamBrowser()
	w.currentPage = pageSavedLogSearch
	w.activeSavedLog = path.Name
	w.selectedLogGroup = groups[0]
	w.selectedLogStream = ""
	if len(streams) == 1 {
		w.selectedLogStream = streams[0]
	}
	w.search.SetText("")
	w.search.SetPlaceholderText("Saved CloudWatch scope")
	w.resourceStack.SetVisibleChildName(pageSavedLogSearch)
	w.backButton.SetSensitive(false)
	w.setBreadcrumb("CloudWatch Logs / " + path.Name)
	w.setDetail("Loading saved CloudWatch search…", detailIntro)
	w.updateActionSensitivity()
}

func (w *mainWindow) promptSaveLogSearch() {
	if w.logSearchSpec == nil || w.options.Config == nil {
		return
	}
	dialog, entry := w.newSavedLogNameDialog("Save CloudWatch search", "Search name", "")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Save", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		name := strings.TrimSpace(entry.Text())
		if name == "" {
			return
		}
		spec := *w.logSearchSpec
		path := config.LogPathEntry{
			Name: name, LogGroup: spec.Groups[0], LogGroups: append([]string(nil), spec.Groups...),
			Streams: append([]string(nil), spec.Streams...), Filter: spec.Filter,
			StartTime: spec.StartTime, EndTime: spec.EndTime,
			HighlightRules: append([]model.LogHighlightRule(nil), w.logHighlightRules...),
		}
		if len(spec.Streams) == 1 {
			path.Stream = spec.Streams[0]
		}
		if spec.Lookback > 0 {
			path.Lookback = spec.Lookback.String()
			path.StartTime, path.EndTime = 0, 0
		}
		dialog.Destroy()
		if !w.mutateSavedLogs(func(cfg *config.Config) { cfg.UpsertLogPath(path) }) {
			return
		}
		w.activeSavedLog = name
		w.rebuildSavedLogRail()
		w.updateActionSensitivity()
		w.setStatus("Saved CloudWatch search as "+name, false)
	})
	dialog.Present()
}

func (w *mainWindow) promptSaveLogDestination() {
	if w.options.Config == nil || w.selectedLogGroup == "" {
		return
	}
	group, stream := w.selectedLogGroup, ""
	if w.currentPage == pageLogStreams {
		stream = w.selectedLogStream
	}
	dialog, entry := w.newSavedLogNameDialog("Save CloudWatch destination", "Destination name", "")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Save", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		name := strings.TrimSpace(entry.Text())
		if name == "" {
			return
		}
		dialog.Destroy()
		path := config.LogPathEntry{Name: name, LogGroup: group, Stream: stream}
		if !w.mutateSavedLogs(func(cfg *config.Config) { cfg.UpsertLogPath(path) }) {
			return
		}
		w.activeSavedLog = name
		w.rebuildSavedLogRail()
		w.updateActionSensitivity()
		w.setStatus("Saved CloudWatch destination as "+name, false)
	})
	dialog.Present()
}

func (w *mainWindow) promptManageSavedLog() {
	if w.activeSavedLog == "" || w.options.Config == nil {
		return
	}
	dialog := gtk.NewDialogWithFlags("Manage "+w.activeSavedLog, &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	content.Append(gtk.NewLabel("Choose an action for this saved CloudWatch destination."))
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Rename…", 101)
	dialog.AddButton("Move up", 102)
	dialog.AddButton("Move down", 103)
	dialog.AddButton("Delete…", 104)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		switch response {
		case 101:
			w.promptRenameSavedLog()
		case 102:
			w.moveActiveSavedLog(-1)
		case 103:
			w.moveActiveSavedLog(1)
		case 104:
			w.confirmDeleteSavedLog()
		}
	})
	dialog.Present()
}

func (w *mainWindow) promptRenameSavedLog() {
	oldName := w.activeSavedLog
	dialog, entry := w.newSavedLogNameDialog("Rename saved search", "New name", oldName)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Rename", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		name := strings.TrimSpace(entry.Text())
		if name == "" {
			return
		}
		for _, path := range w.options.Config.LogPaths {
			if path.Name == name && name != oldName {
				w.setStatus("A saved CloudWatch destination already uses that name", true)
				return
			}
		}
		dialog.Destroy()
		if !w.mutateSavedLogs(func(cfg *config.Config) { cfg.RenameLogPath(oldName, name) }) {
			return
		}
		w.activeSavedLog = name
		w.setBreadcrumb("CloudWatch Logs / " + name)
		w.rebuildSavedLogRail()
		w.updateActionSensitivity()
	})
	dialog.Present()
}

func (w *mainWindow) moveActiveSavedLog(direction int) {
	if w.options.Config == nil {
		return
	}
	canMove := false
	for i, path := range w.options.Config.LogPaths {
		if path.Name == w.activeSavedLog {
			to := i + direction
			canMove = to >= 0 && to < len(w.options.Config.LogPaths)
			break
		}
	}
	if !canMove || !w.mutateSavedLogs(func(cfg *config.Config) { cfg.MoveLogPath(w.activeSavedLog, direction) }) {
		return
	}
	w.rebuildSavedLogRail()
	w.updateActionSensitivity()
}

func (w *mainWindow) confirmDeleteSavedLog() {
	name := w.activeSavedLog
	if name == "" || w.options.Config == nil {
		return
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetMarkup("Delete saved CloudWatch destination <b>" + html.EscapeString(name) + "</b>?")
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Delete", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response != int(gtk.ResponseOK) {
			return
		}
		if !w.mutateSavedLogs(func(cfg *config.Config) { cfg.RemoveLogPath(name) }) {
			return
		}
		w.activeSavedLog = ""
		w.rebuildSavedLogRail()
		w.loadLogGroups()
	})
	dialog.Present()
}

func (w *mainWindow) newSavedLogNameDialog(title, label, value string) (*gtk.Dialog, *gtk.Entry) {
	dialog := gtk.NewDialogWithFlags(title, &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	fieldLabel := gtk.NewLabel(label)
	fieldLabel.SetXAlign(0)
	entry := gtk.NewEntry()
	entry.SetText(value)
	content.Append(fieldLabel)
	content.Append(entry)
	return dialog, entry
}

func (w *mainWindow) mutateSavedLogs(mutate func(*config.Config)) bool {
	if w.options.Config == nil {
		return false
	}
	before := cloneLogPaths(w.options.Config.LogPaths)
	mutate(w.options.Config)
	if err := w.options.Config.Save(); err != nil {
		w.options.Config.LogPaths = before
		w.setStatus("Save CloudWatch destination: "+err.Error(), true)
		return false
	}
	return true
}

func cloneLogPaths(paths []config.LogPathEntry) []config.LogPathEntry {
	cloned := append([]config.LogPathEntry(nil), paths...)
	for i := range cloned {
		cloned[i].LogGroups = append([]string(nil), cloned[i].LogGroups...)
		cloned[i].Streams = append([]string(nil), cloned[i].Streams...)
		cloned[i].HighlightRules = append([]model.LogHighlightRule(nil), cloned[i].HighlightRules...)
	}
	return cloned
}

func savedLogGroups(path config.LogPathEntry) []string {
	if len(path.LogGroups) > 0 {
		return append([]string(nil), path.LogGroups...)
	}
	return splitLogScope(path.LogGroup)
}

func savedLogStreams(path config.LogPathEntry) []string {
	if len(path.Streams) > 0 {
		return append([]string(nil), path.Streams...)
	}
	return splitLogScope(path.Stream)
}

func savedLogHasQuery(path config.LogPathEntry) bool {
	return path.Filter != "" || path.Lookback != "" || path.StartTime != 0 || path.EndTime != 0
}

func searchFromSavedLog(path config.LogPathEntry, now time.Time) (cloudWatchSearch, error) {
	groups, streams := savedLogGroups(path), savedLogStreams(path)
	if len(groups) == 0 {
		return cloudWatchSearch{}, fmt.Errorf("saved CloudWatch destination %q has no log group", path.Name)
	}
	start, end := path.StartTime, path.EndTime
	var lookback time.Duration
	if path.Lookback != "" {
		var err error
		lookback, err = time.ParseDuration(path.Lookback)
		if err != nil || lookback <= 0 {
			return cloudWatchSearch{}, fmt.Errorf("saved CloudWatch destination %q has invalid lookback %q", path.Name, path.Lookback)
		}
		end = now.UnixMilli()
		start = now.Add(-lookback).UnixMilli()
	}
	if start <= 0 || end <= start {
		return cloudWatchSearch{}, fmt.Errorf("saved CloudWatch destination %q has an invalid time range", path.Name)
	}
	return cloudWatchSearch{
		Groups: groups, Streams: streams, Filter: path.Filter, Lookback: lookback,
		StartTime: start, EndTime: end, Title: path.Name,
		HighlightRules: append([]model.LogHighlightRule(nil), path.HighlightRules...),
	}, nil
}

func savedLogTooltip(path config.LogPathEntry) string {
	groups, streams := savedLogGroups(path), savedLogStreams(path)
	text := cloudWatchScopeTitle(groups, streams)
	if path.Filter != "" {
		text += " • " + path.Filter
	}
	if path.Lookback != "" {
		text += " • " + path.Lookback
	}
	return text
}
