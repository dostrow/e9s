package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	tea "github.com/charmbracelet/bubbletea"
	e9saws "github.com/dostrow/e9s/internal/aws"
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
		} else if p.Stream != "" {
			label += fmt.Sprintf("  (%s / %s)", p.LogGroup, p.Stream)
		} else {
			label += fmt.Sprintf("  (%s)", p.LogGroup)
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
	client := a.client
	return a, func() tea.Msg {
		groups, err := client.ListLogGroups(context.Background(), prefix)
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
	client := a.client
	return a, func() tea.Msg {
		streams, err := client.ListLogStreams(context.Background(), logGroup, "")
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

	// Use the stream's first event time for lookback so historical logs are visible.
	// Fall back to 5 minutes if no event time is available.
	lookback := 5 * time.Minute
	if s.FirstEventTime > 0 {
		firstEvent := time.UnixMilli(s.FirstEventTime)
		lookback = time.Since(firstEvent) + time.Minute
	}

	streamName := s.Name
	return a, func() tea.Msg {
		return logReadyMsg{
			title:    fmt.Sprintf("%s / %s", logGroup, streamName),
			logGroup: logGroup,
			streams:  []string{streamName},
			follow:   &f,
			lookback: lookback,
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
			lookback:  10 * time.Second,
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
	if pattern == "" {
		return pattern
	}
	first := pattern[0]
	// JSON or space-delimited filter expressions — pass through
	if first == '{' || first == '[' {
		return pattern
	}
	// Already a single properly-quoted literal (starts and ends with " with no interior quotes)
	if first == '"' && len(pattern) > 1 && pattern[len(pattern)-1] == '"' &&
		!strings.Contains(pattern[1:len(pattern)-1], `"`) {
		return pattern
	}
	// Strip any quotes and wrap for literal matching
	cleaned := strings.ReplaceAll(pattern, `"`, ``)
	return `"` + cleaned + `"`
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

	client := a.client
	groups := a.logSearchGroups
	streams := a.logSearchStreams
	startMs := a.logSearchStartMs
	endMs := a.logSearchEndMs
	filter := a.logSearchFilter

	if len(groups) > 1 {
		// Multi-group: search each group sequentially, streaming results
		return a, searchNextGroup(client, groups, 0, filter, streams, startMs, endMs)
	}

	// Single group: paginated streaming search
	return a, searchGroupPaginated(client, groups[0], streams, filter, startMs, endMs, nil, 500)
}

// searchNextGroup searches one group and chains to the next via partial messages.
func searchNextGroup(client *e9saws.Client, groups []string, idx int, pattern string, streams []string, startMs, endMs int64) tea.Cmd {
	return func() tea.Msg {
		group := groups[idx]
		isLast := idx == len(groups)-1

		perGroup := max(50, 500/len(groups))

		results, err := client.SearchLogs(context.Background(), group, streams, pattern, startMs, endMs, perGroup)
		if err != nil {
			return views.LogSearchPartialMsg{
				Results: []e9saws.LogEntry{{
					Timestamp: startMs,
					Message:   fmt.Sprintf("[error searching %s: %v]", group, err),
					Stream:    group,
				}},
				Done:   isLast,
				Source: group,
			}
		}

		// Tag entries with "group|stream" so we can split them back on jump
		for i := range results {
			if results[i].Stream == "" {
				results[i].Stream = group
			} else {
				results[i].Stream = group + "|" + results[i].Stream
			}
		}

		return views.LogSearchPartialMsg{
			Results: results,
			Done:    isLast,
			Source:  group,
		}
	}
}

// searchGroupPaginated searches a single group page by page, streaming results.
func searchGroupPaginated(client *e9saws.Client, group string, streams []string, pattern string, startMs, endMs int64, nextToken *string, remaining int) tea.Cmd {
	return func() tea.Msg {
		input := &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName:  &group,
			FilterPattern: &pattern,
		}
		if len(streams) > 0 {
			input.LogStreamNames = streams
		}
		if startMs > 0 {
			input.StartTime = &startMs
		}
		if endMs > 0 {
			input.EndTime = &endMs
		}
		pageLimit := min(remaining, 100)
		limit := int32(pageLimit)
		input.Limit = &limit
		input.NextToken = nextToken

		page, err := client.Logs.FilterLogEvents(context.Background(), input)
		if err != nil {
			return views.LogSearchResultsMsg{Err: err}
		}

		var entries []e9saws.LogEntry
		for _, ev := range page.Events {
			entries = append(entries, e9saws.LogEntry{
				Timestamp: derefInt64Ptr(ev.Timestamp),
				Message:   derefStrPtr(ev.Message),
				Stream:    derefStrPtr(ev.LogStreamName),
			})
		}

		remaining -= len(entries)
		done := page.NextToken == nil || remaining <= 0

		return views.LogSearchPartialMsg{
			Results:   entries,
			Done:      done,
			Source:    group,
			NextToken: page.NextToken,
			Remaining: remaining,
		}
	}
}

func derefInt64Ptr(p *int64) int64 {
	if p != nil {
		return *p
	}
	return 0
}

func derefStrPtr(p *string) string {
	if p != nil {
		return *p
	}
	return ""
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
