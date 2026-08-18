//go:build gui

package gui

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/highlight"
	"github.com/dostrow/e9s/internal/model"
)

const (
	savedLogModeDestination = iota
	savedLogModeRelative
	savedLogModeFixed
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
		w.logHiddenStreams = stringSet(path.HiddenStreams)
		w.renderLogs()
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
			HiddenStreams:  sortedStringSet(w.logHiddenStreams),
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
	if w.options.Config == nil || len(w.options.Config.LogPaths) == 0 {
		return
	}
	paths := cloneLogPaths(w.options.Config.LogPaths)
	names := make([]string, len(paths))
	selected := 0
	for i, path := range paths {
		names[i] = path.Name
		if path.Name == w.activeSavedLog {
			selected = i
		}
	}
	dialog := gtk.NewDialogWithFlags("Saved searches", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	selector := gtk.NewDropDownFromStrings(names)
	selector.SetSelected(uint(selected))
	selector.SetHExpand(true)
	detail := gtk.NewLabel("")
	detail.SetXAlign(0)
	detail.SetWrap(true)
	detail.AddCSSClass("muted")
	updateDetail := func() {
		index := int(selector.Selected())
		if index >= 0 && index < len(paths) {
			detail.SetLabel(savedLogTooltip(paths[index]))
		}
	}
	selector.NotifyProperty("selected", updateDetail)
	updateDetail()
	content.Append(selector)
	content.Append(detail)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Open", 101)
	dialog.AddButton("Edit…", 102)
	dialog.AddButton("Duplicate…", 103)
	dialog.AddButton("Move up", 104)
	dialog.AddButton("Move down", 105)
	dialog.AddButton("Delete…", 106)
	dialog.ConnectResponse(func(response int) {
		index := int(selector.Selected())
		dialog.Destroy()
		if index < 0 || index >= len(paths) {
			return
		}
		path := paths[index]
		switch response {
		case 101:
			w.openSavedLog(path)
		case 102:
			w.promptEditSavedLog(path, path.Name)
		case 103:
			path.Name = w.availableSavedLogName(path.Name + " copy")
			w.promptEditSavedLog(path, "")
		case 104:
			w.moveSavedLog(path.Name, -1)
		case 105:
			w.moveSavedLog(path.Name, 1)
		case 106:
			w.confirmDeleteSavedLogNamed(path.Name)
		}
	})
	dialog.Present()
}

func (w *mainWindow) promptEditSavedLog(path config.LogPathEntry, originalName string) {
	dialog := gtk.NewDialogWithFlags("Edit saved search", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(760, 640)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)

	name := gtk.NewEntry()
	name.SetText(path.Name)
	groups := gtk.NewEntry()
	groups.SetText(strings.Join(savedLogGroups(path), ", "))
	streams := gtk.NewEntry()
	streams.SetText(strings.Join(savedLogStreams(path), ", "))
	filter := gtk.NewEntry()
	filter.SetText(path.Filter)
	mode := gtk.NewDropDownFromStrings([]string{"Destination / live logs", "Relative search", "Fixed UTC range"})
	modeIndex := savedLogModeDestination
	if path.Lookback != "" {
		modeIndex = savedLogModeRelative
	} else if path.StartTime > 0 || path.EndTime > 0 || path.Filter != "" {
		modeIndex = savedLogModeFixed
	}
	mode.SetSelected(uint(modeIndex))
	lookback := gtk.NewEntry()
	lookback.SetText(path.Lookback)
	if path.Lookback == "" {
		lookback.SetText("1h")
	}
	now := time.Now().UTC()
	from := gtk.NewEntry()
	to := gtk.NewEntry()
	if path.StartTime > 0 {
		from.SetText(time.UnixMilli(path.StartTime).UTC().Format("2006-01-02 15:04:05"))
	} else {
		from.SetText(now.Add(-time.Hour).Format("2006-01-02 15:04:05"))
	}
	if path.EndTime > 0 {
		to.SetText(time.UnixMilli(path.EndTime).UTC().Format("2006-01-02 15:04:05"))
	} else {
		to.SetText(now.Format("2006-01-02 15:04:05"))
	}
	hiddenStreams := gtk.NewEntry()
	hiddenStreams.SetText(strings.Join(path.HiddenStreams, ", "))
	hiddenStreams.SetPlaceholderText("Comma-separated streams hidden by default")

	appendDialogField(content, "Name", name)
	appendDialogField(content, "Log groups", groups)
	appendDialogField(content, "Log streams (optional)", streams)
	appendDialogField(content, "Mode", mode)
	appendDialogField(content, "Search expression", filter)
	appendDialogField(content, "Relative lookback (for example 15m, 1h, 7d)", lookback)
	appendDialogField(content, "From (UTC)", from)
	appendDialogField(content, "To (UTC)", to)
	appendDialogField(content, "Hidden streams", hiddenStreams)

	rules := append([]model.LogHighlightRule(nil), path.HighlightRules...)
	highlightRow := gtk.NewBox(gtk.OrientationHorizontal, 8)
	highlightSummary := gtk.NewLabel("")
	highlightSummary.SetXAlign(0)
	highlightSummary.SetHExpand(true)
	updateHighlightSummary := func() {
		highlightSummary.SetLabel(fmt.Sprintf("%d highlight rules", len(rules)))
	}
	updateHighlightSummary()
	editHighlights := gtk.NewButtonWithLabel("Edit rules…")
	editHighlights.ConnectClicked(func() {
		w.promptHighlightRuleEditor(&dialog.Window, "Saved highlight rules", rules, "", func(updated []model.LogHighlightRule, _ bool) {
			rules = updated
			updateHighlightSummary()
		})
	})
	highlightRow.Append(highlightSummary)
	highlightRow.Append(editHighlights)
	appendDialogField(content, "Highlights", highlightRow)

	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.SetWrap(true)
	errorLabel.AddCSSClass("error")
	content.Append(errorLabel)
	updateMode := func() {
		selected := int(mode.Selected())
		filter.SetSensitive(selected != savedLogModeDestination)
		lookback.SetSensitive(selected == savedLogModeRelative)
		from.SetSensitive(selected == savedLogModeFixed)
		to.SetSensitive(selected == savedLogModeFixed)
	}
	mode.NotifyProperty("selected", updateMode)
	updateMode()

	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Save", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		updated, err := buildSavedLogDefinition(
			name.Text(), groups.Text(), streams.Text(), int(mode.Selected()), filter.Text(), lookback.Text(),
			from.Text(), to.Text(), hiddenStreams.Text(), rules,
		)
		if err != nil {
			errorLabel.SetLabel(err.Error())
			return
		}
		for _, existing := range w.options.Config.LogPaths {
			if existing.Name == updated.Name && existing.Name != originalName {
				errorLabel.SetLabel("A saved CloudWatch destination already uses that name")
				return
			}
		}
		wasActive := originalName != "" && w.activeSavedLog == originalName
		if !w.mutateSavedLogs(func(cfg *config.Config) {
			if originalName != "" && originalName != updated.Name {
				cfg.RemoveLogPath(originalName)
			}
			cfg.UpsertLogPath(updated)
		}) {
			return
		}
		dialog.Destroy()
		if wasActive {
			w.activeSavedLog = updated.Name
		}
		w.rebuildSavedLogRail()
		if wasActive {
			w.openSavedLog(updated)
		} else {
			w.updateActionSensitivity()
		}
		w.setStatus("Saved CloudWatch definition "+updated.Name, false)
	})
	dialog.Present()
}

func buildSavedLogDefinition(name, groupsText, streamsText string, mode int, filterText, lookbackText, fromText, toText, hiddenStreamsText string, rules []model.LogHighlightRule) (config.LogPathEntry, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return config.LogPathEntry{}, fmt.Errorf("name is required")
	}
	groups := splitLogScope(groupsText)
	if len(groups) == 0 {
		return config.LogPathEntry{}, fmt.Errorf("at least one log group is required")
	}
	streams := splitLogScope(streamsText)
	if len(groups) > 1 && len(streams) > 0 {
		return config.LogPathEntry{}, fmt.Errorf("stream selection is available only for a single log group")
	}
	if _, err := highlight.Compile(rules); err != nil {
		return config.LogPathEntry{}, err
	}
	path := config.LogPathEntry{
		Name: name, LogGroup: groups[0], LogGroups: groups,
		Streams:        append([]string(nil), streams...),
		HighlightRules: append([]model.LogHighlightRule(nil), rules...),
		HiddenStreams:  splitLogScope(hiddenStreamsText),
	}
	if len(streams) == 1 {
		path.Stream = streams[0]
	}
	switch mode {
	case savedLogModeDestination:
	case savedLogModeRelative:
		lookback, err := time.ParseDuration(strings.TrimSpace(lookbackText))
		if err != nil || lookback <= 0 {
			return config.LogPathEntry{}, fmt.Errorf("lookback must be a positive duration such as 15m, 1h, or 7d")
		}
		path.Filter = quoteCloudWatchFilter(filterText)
		path.Lookback = lookback.String()
	case savedLogModeFixed:
		from, err := parseCloudWatchUTCTime(fromText)
		if err != nil {
			return config.LogPathEntry{}, fmt.Errorf("from time: %w", err)
		}
		to, err := parseCloudWatchUTCTime(toText)
		if err != nil {
			return config.LogPathEntry{}, fmt.Errorf("to time: %w", err)
		}
		if !from.Before(to) {
			return config.LogPathEntry{}, fmt.Errorf("the start time must be before the end time")
		}
		path.Filter = quoteCloudWatchFilter(filterText)
		path.StartTime, path.EndTime = from.UnixMilli(), to.UnixMilli()
	default:
		return config.LogPathEntry{}, fmt.Errorf("select a valid saved-search mode")
	}
	return path, nil
}

func (w *mainWindow) availableSavedLogName(base string) string {
	used := make(map[string]struct{}, len(w.options.ConfigLogPaths()))
	for _, path := range w.options.ConfigLogPaths() {
		used[path.Name] = struct{}{}
	}
	if _, exists := used[base]; !exists {
		return base
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s %d", base, suffix)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

func (w *mainWindow) moveSavedLog(name string, direction int) {
	if w.options.Config == nil {
		return
	}
	canMove := false
	for i, path := range w.options.Config.LogPaths {
		if path.Name == name {
			to := i + direction
			canMove = to >= 0 && to < len(w.options.Config.LogPaths)
			break
		}
	}
	if !canMove || !w.mutateSavedLogs(func(cfg *config.Config) { cfg.MoveLogPath(name, direction) }) {
		return
	}
	w.rebuildSavedLogRail()
	w.updateActionSensitivity()
}

func (w *mainWindow) confirmDeleteSavedLogNamed(name string) {
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
		wasActive := w.activeSavedLog == name
		if wasActive {
			w.activeSavedLog = ""
		}
		w.rebuildSavedLogRail()
		if wasActive {
			w.loadLogGroups()
		} else {
			w.updateActionSensitivity()
		}
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
		cloned[i].HiddenStreams = append([]string(nil), cloned[i].HiddenStreams...)
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
		HiddenStreams:  append([]string(nil), path.HiddenStreams...),
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
	if len(path.HiddenStreams) > 0 {
		text += fmt.Sprintf(" • %d hidden streams", len(path.HiddenStreams))
	}
	return text
}

func stringSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

func sortedStringSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
