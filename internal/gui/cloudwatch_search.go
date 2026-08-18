//go:build gui

package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

const cloudWatchSearchLimit = 1000

var cloudWatchTimePresets = []struct {
	label    string
	duration time.Duration
}{
	{label: "Last 15 minutes", duration: 15 * time.Minute},
	{label: "Last 1 hour", duration: time.Hour},
	{label: "Last 6 hours", duration: 6 * time.Hour},
	{label: "Last 24 hours", duration: 24 * time.Hour},
	{label: "Last 3 days", duration: 3 * 24 * time.Hour},
	{label: "Last 7 days", duration: 7 * 24 * time.Hour},
	{label: "Custom UTC range"},
}

type cloudWatchSearch struct {
	Groups    []string
	Streams   []string
	Filter    string
	Lookback  time.Duration
	StartTime int64
	EndTime   int64
	Title     string
}

func (w *mainWindow) promptCloudWatchSearch() {
	if w.options.Logs == nil || w.selectedLogGroup == "" {
		return
	}

	dialog := gtk.NewDialogWithFlags("Search CloudWatch logs", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)

	defaultGroups := []string{w.selectedLogGroup}
	defaultStreams := []string(nil)
	if w.currentPage == pageLogStreams && w.selectedLogStream != "" {
		defaultStreams = []string{w.selectedLogStream}
	}
	if path, ok := w.activeSavedLogPath(); ok {
		defaultGroups = savedLogGroups(path)
		defaultStreams = savedLogStreams(path)
	}
	groups := gtk.NewEntry()
	groups.SetText(strings.Join(defaultGroups, ", "))
	groups.SetPlaceholderText("Comma-separated log groups")
	streams := gtk.NewEntry()
	streams.SetText(strings.Join(defaultStreams, ", "))
	streams.SetPlaceholderText("Comma-separated streams; blank searches whole group")
	filter := gtk.NewEntry()
	filter.SetPlaceholderText("Plain text or a CloudWatch filter expression")

	presetLabels := make([]string, len(cloudWatchTimePresets))
	for i, preset := range cloudWatchTimePresets {
		presetLabels[i] = preset.label
	}
	preset := gtk.NewDropDownFromStrings(presetLabels)
	preset.SetSelected(1)
	now := time.Now().UTC()
	from := gtk.NewEntry()
	from.SetText(now.Add(-time.Hour).Format("2006-01-02 15:04"))
	to := gtk.NewEntry()
	to.SetText(now.Format("2006-01-02 15:04"))
	if w.logSearchSpec != nil {
		filter.SetText(w.logSearchSpec.Filter)
		from.SetText(time.UnixMilli(w.logSearchSpec.StartTime).UTC().Format("2006-01-02 15:04"))
		to.SetText(time.UnixMilli(w.logSearchSpec.EndTime).UTC().Format("2006-01-02 15:04"))
		preset.SetSelected(uint(len(cloudWatchTimePresets) - 1))
		for i, candidate := range cloudWatchTimePresets[:len(cloudWatchTimePresets)-1] {
			if candidate.duration == w.logSearchSpec.Lookback {
				preset.SetSelected(uint(i))
				break
			}
		}
	}
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.SetWrap(true)
	errorLabel.AddCSSClass("error")

	appendDialogField(content, "Log groups", groups)
	appendDialogField(content, "Log streams", streams)
	appendDialogField(content, "Filter expression", filter)
	appendDialogField(content, "Time range", preset)
	customHelp := gtk.NewLabel("Custom fields are used only when Custom UTC range is selected.")
	customHelp.SetXAlign(0)
	customHelp.AddCSSClass("muted")
	content.Append(customHelp)
	appendDialogField(content, "From (YYYY-MM-DD HH:MM, UTC)", from)
	appendDialogField(content, "To (YYYY-MM-DD HH:MM, UTC)", to)
	content.Append(errorLabel)

	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Search", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		spec, err := buildCloudWatchSearch(
			groups.Text(), streams.Text(), filter.Text(), int(preset.Selected()), from.Text(), to.Text(), time.Now(),
		)
		if err != nil {
			errorLabel.SetLabel(err.Error())
			return
		}
		dialog.Destroy()
		w.runCloudWatchSearch(spec)
	})
	dialog.Present()
}

func appendDialogField(content *gtk.Box, label string, widget gtk.Widgetter) {
	fieldLabel := gtk.NewLabel(label)
	fieldLabel.SetXAlign(0)
	content.Append(fieldLabel)
	content.Append(widget)
}

func buildCloudWatchSearch(groupsText, streamsText, filterText string, preset int, fromText, toText string, now time.Time) (cloudWatchSearch, error) {
	groups := splitLogScope(groupsText)
	if len(groups) == 0 {
		return cloudWatchSearch{}, fmt.Errorf("at least one log group is required")
	}
	streams := splitLogScope(streamsText)
	if len(groups) > 1 && len(streams) > 0 {
		return cloudWatchSearch{}, fmt.Errorf("stream selection is available only for a single log group")
	}

	now = now.UTC()
	var start, end time.Time
	var lookback time.Duration
	if preset >= 0 && preset < len(cloudWatchTimePresets)-1 {
		lookback = cloudWatchTimePresets[preset].duration
		end = now
		start = end.Add(-lookback)
	} else if preset == len(cloudWatchTimePresets)-1 {
		var err error
		start, err = parseCloudWatchUTCTime(fromText)
		if err != nil {
			return cloudWatchSearch{}, fmt.Errorf("from time: %w", err)
		}
		end, err = parseCloudWatchUTCTime(toText)
		if err != nil {
			return cloudWatchSearch{}, fmt.Errorf("to time: %w", err)
		}
	} else {
		return cloudWatchSearch{}, fmt.Errorf("select a valid time range")
	}
	if !start.Before(end) {
		return cloudWatchSearch{}, fmt.Errorf("the start time must be before the end time")
	}

	filter := quoteCloudWatchFilter(filterText)
	return cloudWatchSearch{
		Groups: groups, Streams: streams, Filter: filter, Lookback: lookback,
		StartTime: start.UnixMilli(), EndTime: end.UnixMilli(),
		Title: cloudWatchSearchTitle(groups, streams, filter),
	}, nil
}

func (w *mainWindow) runCloudWatchSearch(spec cloudWatchSearch) {
	if w.options.Logs == nil || len(spec.Groups) == 0 {
		return
	}
	spec.Groups = append([]string(nil), spec.Groups...)
	spec.Streams = append([]string(nil), spec.Streams...)
	query := model.LogQuery{
		Groups: append([]string(nil), spec.Groups...), Streams: append([]string(nil), spec.Streams...),
		Filter: spec.Filter, StartTime: spec.StartTime, EndTime: spec.EndTime, Limit: cloudWatchSearchLimit,
	}
	ctx, generation := w.startRequest("Searching CloudWatch logs…")
	go func() {
		page, err := w.options.Logs.Fetch(ctx, spec.Groups[0], query)
		status := fmt.Sprintf("Found %d events • %s", len(page.Entries), formatCloudWatchRange(spec.StartTime, spec.EndTime))
		w.finishRequestWithStatus(ctx, generation, err, status, func() {
			w.showCloudWatchSearchResults(spec, page)
		})
	}()
}

func (w *mainWindow) showCloudWatchSearchResults(spec cloudWatchSearch, page model.LogPage) {
	w.logSearchSpec = &spec
	w.showLogSnapshotData(model.LogSource{Group: spec.Groups[0], Streams: spec.Streams}, spec.Title, page)
	w.updateActionSensitivity()
}

func (w *mainWindow) loadAdjacentCloudWatchRange(direction int) {
	if w.logSearchSpec == nil || direction == 0 {
		return
	}
	spec := *w.logSearchSpec
	width := spec.EndTime - spec.StartTime
	if width <= 0 {
		return
	}
	shift := int64(direction) * width
	spec.StartTime += shift
	spec.EndTime += shift
	spec.Lookback = 0
	if spec.EndTime > time.Now().UnixMilli() {
		spec.EndTime = time.Now().UnixMilli()
		spec.StartTime = spec.EndTime - width
	}
	w.runCloudWatchSearch(spec)
}

func (w *mainWindow) promptLogCorrelation() {
	entry, ok := w.logEntryAtCursor()
	if !ok || w.logSearchSpec == nil {
		w.setStatus("Place the cursor on a search result to correlate it", false)
		return
	}
	labels := []string{"±10 seconds", "±30 seconds", "±1 minute", "±5 minutes", "±15 minutes", "±1 hour"}
	windows := []time.Duration{10 * time.Second, 30 * time.Second, time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour}
	dialog := gtk.NewDialogWithFlags("Correlate log event", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	label := gtk.NewLabel("Load unfiltered events around " + time.UnixMilli(entry.Timestamp).UTC().Format("2006-01-02 15:04:05.000 UTC"))
	label.SetXAlign(0)
	label.SetWrap(true)
	selector := gtk.NewDropDownFromStrings(labels)
	selector.SetSelected(2)
	content.Append(label)
	content.Append(selector)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Correlate", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		selected := int(selector.Selected())
		dialog.Destroy()
		if response != int(gtk.ResponseOK) || selected < 0 || selected >= len(windows) {
			return
		}
		window := windows[selected]
		spec := *w.logSearchSpec
		spec.Filter = ""
		spec.Lookback = 0
		spec.StartTime = max(int64(0), entry.Timestamp-window.Milliseconds())
		spec.EndTime = entry.Timestamp + window.Milliseconds()
		spec.Title = "Correlate: " + cloudWatchScopeTitle(spec.Groups, spec.Streams)
		w.runCloudWatchSearch(spec)
	})
	dialog.Present()
}

func splitLogScope(value string) []string {
	seen := make(map[string]struct{})
	var values []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, exists := seen[part]; exists {
			continue
		}
		seen[part] = struct{}{}
		values = append(values, part)
	}
	return values
}

func quoteCloudWatchFilter(pattern string) string {
	return service.NormalizeFilterPattern(pattern)
}

func parseCloudWatchUTCTime(value string) (time.Time, error) {
	for _, layout := range []string{
		"2006-01-02 15:04", "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02",
	} {
		if parsed, err := time.ParseInLocation(layout, strings.TrimSpace(value), time.UTC); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid UTC timestamp %q; use YYYY-MM-DD HH:MM", value)
}

func cloudWatchSearchTitle(groups, streams []string, filter string) string {
	title := "Search: " + cloudWatchScopeTitle(groups, streams)
	if filter != "" {
		title += " • " + filter
	}
	return title
}

func cloudWatchScopeTitle(groups, streams []string) string {
	switch {
	case len(groups) > 1:
		return fmt.Sprintf("%d log groups", len(groups))
	case len(groups) == 1 && len(streams) > 1:
		return fmt.Sprintf("%s / %d streams", groups[0], len(streams))
	case len(groups) == 1 && len(streams) == 1:
		return groups[0] + " / " + streams[0]
	case len(groups) == 1:
		return groups[0]
	default:
		return "CloudWatch logs"
	}
}

func formatCloudWatchRange(start, end int64) string {
	return time.UnixMilli(start).UTC().Format("2006-01-02 15:04") + "–" +
		time.UnixMilli(end).UTC().Format("15:04 UTC")
}
