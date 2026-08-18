package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- CloudWatch Log Browser ---

func (a App) promptCloudWatchBrowser() (App, tea.Cmd) {
	saved := a.cfg.LogPaths
	if len(saved) == 0 {
		a.input = NewInput(InputLogGroupPrefix, "Search log groups (prefix with / or substring match)", "")
		return a, nil
	}

	items := make([]string, 0, len(saved)+1)
	for _, p := range saved {
		label := p.Name
		if len(p.LogGroups) > 1 {
			label += fmt.Sprintf("  (%d groups)", len(p.LogGroups))
		} else if len(savedLogPathStreams(p)) > 0 {
			label += fmt.Sprintf("  (%s / %s)", p.LogGroup, strings.Join(savedLogPathStreams(p), ", "))
		} else {
			label += fmt.Sprintf("  (%s)", p.LogGroup)
		}
		if p.Filter != "" {
			label += "  " + p.Filter
		}
		items = append(items, label)
	}
	savedCount := len(items)
	items = append(items, "[enter a custom log group]")
	a.picker = NewPickerWithDelete(PickerLogPath, "Select log path", items, savedCount)
	return a, nil
}

func (a App) openLogGroups(prefix string) (App, tea.Cmd) {
	a.mode = modeCWLogs
	a.state = viewLogGroups
	a.logGroupsView = views.NewLogGroups()
	a.logGroupsView = a.logGroupsView.SetSize(a.width, a.height-3)
	a.loading = true
	logs := a.logs
	ctx := a.ctx
	return a, func() tea.Msg {
		groups, err := logs.ListGroups(ctx, prefix)
		if err != nil {
			return errMsg{err}
		}
		return logGroupsLoadedMsg{groups}
	}
}

func (a App) openLogStreams(logGroup string) (App, tea.Cmd) {
	a.mode = modeCWLogs
	a.state = viewLogStreams
	a.logStreamsView = views.NewLogStreams(logGroup)
	a.logStreamsView = a.logStreamsView.SetSize(a.width, a.height-3)
	a.loading = true
	logs := a.logs
	ctx := a.ctx
	return a, func() tea.Msg {
		streams, err := logs.ListStreams(ctx, logGroup, "")
		if err != nil {
			return errMsg{err}
		}
		return logStreamsLoadedMsg{streams}
	}
}

func (a App) tailLogGroup() (App, tea.Cmd) {
	g := a.logGroupsView.SelectedGroup()
	if g == nil {
		return a, nil
	}
	a.prevState = viewLogGroups
	return a, a.startLogTail(g.Name, nil, g.Name)
}

func (a App) peekLogStream() (App, tea.Cmd) {
	s := a.logStreamsView.SelectedStream()
	if s == nil {
		return a, nil
	}
	a.prevState = viewLogStreams
	logGroup := a.logStreamsView.LogGroup()
	f := false

	streamName := s.Name
	return a, func() tea.Msg {
		return logReadyMsg{
			title:    fmt.Sprintf("%s / %s", logGroup, streamName),
			logGroup: logGroup,
			streams:  []string{streamName},
			follow:   &f,
			lookback: 15 * time.Minute,
		}
	}
}

func (a App) tailLogStream() (App, tea.Cmd) {
	s := a.logStreamsView.SelectedStream()
	if s == nil {
		return a, nil
	}
	a.prevState = viewLogStreams
	logGroup := a.logStreamsView.LogGroup()
	return a, a.startLogTail(logGroup, []string{s.Name}, fmt.Sprintf("%s / %s", logGroup, s.Name))
}

func (a App) tailEntireLogGroup() (App, tea.Cmd) {
	logGroup := a.logStreamsView.LogGroup()
	a.prevState = viewLogStreams
	return a, a.startLogTail(logGroup, nil, logGroup+" (all streams)")
}

func (a App) startLogTail(logGroup string, streams []string, title string) tea.Cmd {
	return func() tea.Msg {
		return logReadyMsg{
			title:     title,
			logGroup:  logGroup,
			logGroups: []string{logGroup},
			streams:   streams,
			lookback:  15 * time.Minute,
		}
	}
}

// --- Log Search ---

func (a App) promptLogSearchFromGroups() (App, tea.Cmd) {
	groups := a.logGroupsView.SelectedGroups()
	if len(groups) == 0 {
		return a, nil
	}
	a.prevState = viewLogGroups
	a.logSearchGroups = groups
	a.logSearchGroup = groups[0] // primary group for display
	a.logSearchStreams = nil
	return a.promptLogSearchTimeRange()
}

func (a App) promptLogSearchFromStreams() (App, tea.Cmd) {
	streams := a.logStreamsView.SelectedStreams()
	if len(streams) == 0 {
		return a, nil
	}
	a.prevState = viewLogStreams
	a.logSearchGroup = a.logStreamsView.LogGroup()
	a.logSearchGroups = []string{a.logSearchGroup}
	a.logSearchStreams = streams
	return a.promptLogSearchTimeRange()
}

func (a App) promptLogSearchTimeRange() (App, tea.Cmd) {
	a.picker = NewPicker(PickerLogSearchTimeRange, "Search time range", []string{
		"Last 15 minutes",
		"Last 1 hour",
		"Last 6 hours",
		"Last 24 hours",
		"Last 3 days",
		"Last 7 days",
		"Custom range...",
	})
	return a, nil
}

func (a App) handleTimeRangePick(value string) (App, tea.Cmd) {
	if value == "Custom range..." {
		now := time.Now().UTC()
		defaultFrom := now.Add(-1 * time.Hour).Format("2006-01-02 15:04")
		a.input = NewInput(InputLogSearchFrom, "From (YYYY-MM-DD HH:MM, UTC)", defaultFrom)
		return a, nil
	}

	durations := map[string]time.Duration{
		"Last 15 minutes": 15 * time.Minute,
		"Last 1 hour":     1 * time.Hour,
		"Last 6 hours":    6 * time.Hour,
		"Last 24 hours":   24 * time.Hour,
		"Last 3 days":     3 * 24 * time.Hour,
		"Last 7 days":     7 * 24 * time.Hour,
	}
	d, ok := durations[value]
	if !ok {
		d = 1 * time.Hour
	}
	a.logSearchStartMs = time.Now().Add(-d).UnixMilli()
	a.logSearchEndMs = time.Now().UnixMilli()

	a.input = NewInput(InputLogSearchPattern, "Search pattern (auto-quoted for literal match; use {$.field = \"val\"} for JSON)", "")
	return a, nil
}

func parseUTCTimestamp(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02 15:04",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q — use YYYY-MM-DD HH:MM", s)
}

// quoteFilterPattern ensures a pattern is valid CloudWatch filter syntax.
//
// CloudWatch filter syntax: "text" for literal match, { $.field = "val" }
// for JSON pattern, [w1, w2] for space-delimited matching.
//
// For plain text searches, we wrap in quotes. CloudWatch can't nest quotes
// in literal patterns, so interior quotes are stripped — the literal match
// still finds the text in log lines that contain quotes around it.
func quoteFilterPattern(pattern string) string {
	return service.NormalizeFilterPattern(pattern)
}

func (a App) startLogSearch(pattern string) (App, tea.Cmd) {
	a.state = viewLogSearch

	searchScope := a.logSearchGroup
	if len(a.logSearchGroups) > 1 {
		searchScope = fmt.Sprintf("%d groups", len(a.logSearchGroups))
	}
	a.logSearchView = views.NewLogSearch(searchScope, a.logSearchStreams, pattern)
	a.logSearchView = a.logSearchView.SetSize(a.width, a.height-3)

	// Auto-quote for literal matching if needed
	a.logSearchFilter = quoteFilterPattern(pattern)

	logs := a.logs
	ctx := a.ctx
	groups := a.logSearchGroups
	streams := a.logSearchStreams
	startMs := a.logSearchStartMs
	endMs := a.logSearchEndMs
	filter := a.logSearchFilter

	return a, func() tea.Msg {
		page, err := logs.Fetch(ctx, groups[0], model.LogQuery{
			Groups: groups, Streams: streams, Filter: filter,
			StartTime: startMs, EndTime: endMs, Limit: 500,
		})
		return views.LogSearchResultsMsg{Results: page.Entries, Err: err}
	}
}

func (a App) openSavedLogDestination(path config.LogPathEntry) (App, tea.Cmd) {
	groups := savedLogPathGroups(path)
	streams := savedLogPathStreams(path)
	if len(groups) == 0 {
		a.err = fmt.Errorf("saved CloudWatch destination %q has no log group", path.Name)
		return a, nil
	}
	if path.Filter != "" || path.Lookback != "" || path.StartTime != 0 || path.EndTime != 0 {
		start, end := path.StartTime, path.EndTime
		if path.Lookback != "" {
			lookback, err := time.ParseDuration(path.Lookback)
			if err != nil || lookback <= 0 {
				a.err = fmt.Errorf("saved CloudWatch destination %q has invalid lookback %q", path.Name, path.Lookback)
				return a, nil
			}
			now := time.Now()
			end = now.UnixMilli()
			start = now.Add(-lookback).UnixMilli()
		}
		if start <= 0 || end <= start {
			a.err = fmt.Errorf("saved CloudWatch destination %q has an invalid time range", path.Name)
			return a, nil
		}
		a.prevState = viewLogGroups
		a.logSearchGroups = groups
		a.logSearchGroup = groups[0]
		a.logSearchStreams = streams
		a.logSearchStartMs = start
		a.logSearchEndMs = end
		return a.startLogSearch(path.Filter)
	}
	if len(groups) > 1 {
		a.prevState = viewLogGroups
		a.logSearchGroups = groups
		a.logSearchGroup = groups[0]
		a.logSearchStreams = nil
		return a.promptLogSearchTimeRange()
	}
	if len(streams) > 0 {
		return a, a.startLogTail(groups[0], streams, path.Name)
	}
	return a.openLogStreams(groups[0])
}

func savedLogPathGroups(path config.LogPathEntry) []string {
	if len(path.LogGroups) > 0 {
		return append([]string(nil), path.LogGroups...)
	}
	if strings.TrimSpace(path.LogGroup) == "" {
		return nil
	}
	return []string{strings.TrimSpace(path.LogGroup)}
}

func savedLogPathStreams(path config.LogPathEntry) []string {
	if len(path.Streams) > 0 {
		return append([]string(nil), path.Streams...)
	}
	if strings.TrimSpace(path.Stream) == "" {
		return nil
	}
	return []string{strings.TrimSpace(path.Stream)}
}

func (a App) startLogCorrelation() (App, tea.Cmd) {
	entry := a.logSearchView.SelectedResult()
	if entry == nil {
		return a, nil
	}

	a.prevState = viewLogSearch
	a.logCorrelationActive = true
	a.logCorrelationTS = entry.Timestamp
	a.logCorrelationPattern = a.logSearchView.Pattern()
	a.logCorrelationGroups = nil
	a.logCorrelationStreams = nil

	return a.openLogGroups("")
}

func (a App) promptLogCorrelationWindowForGroups() (App, tea.Cmd) {
	groups := a.logGroupsView.SelectedGroups()
	if len(groups) == 0 {
		return a, nil
	}
	a.logCorrelationGroups = groups
	a.logCorrelationStreams = nil
	return a.promptLogCorrelationWindow()
}

func (a App) promptLogCorrelationWindowForStreams() (App, tea.Cmd) {
	streams := a.logStreamsView.SelectedStreams()
	if len(streams) == 0 {
		return a, nil
	}
	a.logCorrelationGroups = []string{a.logStreamsView.LogGroup()}
	a.logCorrelationStreams = streams
	return a.promptLogCorrelationWindow()
}

func (a App) promptLogCorrelationWindow() (App, tea.Cmd) {
	a.picker = NewPicker(PickerLogCorrelationWindow, "Correlation window", []string{
		"±10 seconds",
		"±30 seconds",
		"±1 minute",
		"±5 minutes",
		"±15 minutes",
		"±1 hour",
	})
	return a, nil
}

func (a App) handleLogCorrelationWindowPick(value string) (App, tea.Cmd) {
	windows := map[string]time.Duration{
		"±10 seconds": 10 * time.Second,
		"±30 seconds": 30 * time.Second,
		"±1 minute":   1 * time.Minute,
		"±5 minutes":  5 * time.Minute,
		"±15 minutes": 15 * time.Minute,
		"±1 hour":     1 * time.Hour,
	}
	window, ok := windows[value]
	if !ok {
		window = 1 * time.Minute
	}

	startMs := a.logCorrelationTS - window.Milliseconds()
	if startMs < 0 {
		startMs = 0
	}
	endMs := a.logCorrelationTS + window.Milliseconds()
	follow := false
	title := correlationTitle(a.logCorrelationGroups, a.logCorrelationStreams, window)
	groups := append([]string(nil), a.logCorrelationGroups...)
	streams := append([]string(nil), a.logCorrelationStreams...)
	logGroup := ""
	if len(groups) > 0 {
		logGroup = groups[0]
	}

	a.prevState = viewLogSearch
	a.logCorrelationActive = false
	return a, func() tea.Msg {
		return logReadyMsg{
			title:     title,
			logGroup:  logGroup,
			logGroups: groups,
			streams:   streams,
			follow:    &follow,
			startMs:   startMs,
			endMs:     endMs,
		}
	}
}

func correlationTitle(groups, streams []string, window time.Duration) string {
	scope := ""
	switch {
	case len(groups) > 1:
		scope = fmt.Sprintf("%d groups", len(groups))
	case len(groups) == 1 && len(streams) > 1:
		scope = fmt.Sprintf("%s / %d streams", groups[0], len(streams))
	case len(groups) == 1 && len(streams) == 1:
		scope = fmt.Sprintf("%s / %s", groups[0], streams[0])
	case len(groups) == 1:
		scope = groups[0]
	default:
		scope = "logs"
	}
	return fmt.Sprintf("correlate: %s (%s)", scope, window.String())
}

// --- Log Path Saving ---

func (a App) saveLogGroupPath() (App, tea.Cmd) {
	g := a.logGroupsView.SelectedGroup()
	if g == nil {
		return a, nil
	}
	a.logSaveGroup = g.Name
	a.logSaveStream = ""
	a.input = NewInput(InputLogSaveName, fmt.Sprintf("Save log group %q — enter a name", g.Name), "")
	return a, nil
}

func (a App) saveLogStreamPath() (App, tea.Cmd) {
	s := a.logStreamsView.SelectedStream()
	if s == nil {
		return a, nil
	}
	a.logSaveGroup = a.logStreamsView.LogGroup()
	a.logSaveStream = s.Name
	label := fmt.Sprintf("%s / %s", a.logSaveGroup, s.Name)
	a.input = NewInput(InputLogSaveName, fmt.Sprintf("Save %q — enter a name", label), "")
	return a, nil
}

func (a App) saveLogSearchGroups() (App, tea.Cmd) {
	groups := a.logSearchGroups
	if len(groups) == 0 {
		return a, nil
	}
	label := fmt.Sprintf("Save %d log groups — enter a name", len(groups))
	a.input = NewInput(InputLogSearchGroupsSave, label, "")
	return a, nil
}

func (a App) doSaveLogSearchGroups(name string) (App, tea.Cmd) {
	a.cfg.AddLogPathMultiGroup(name, a.logSearchGroups)
	if err := a.cfg.Save(); err != nil {
		a.err = err
		return a, nil
	}
	a.flashMessage = fmt.Sprintf("Saved %d log groups as %q", len(a.logSearchGroups), name)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}

func (a App) doSaveLogPath(name string) (App, tea.Cmd) {
	a.cfg.AddLogPath(name, a.logSaveGroup, a.logSaveStream)
	if err := a.cfg.Save(); err != nil {
		a.err = err
		return a, nil
	}
	a.flashMessage = fmt.Sprintf("Saved log path as %q", name)
	a.flashExpiry = time.Now().Add(5 * time.Second)
	return a, nil
}
