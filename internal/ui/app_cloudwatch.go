package ui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
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
		if p.Stream != "" {
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
	a.mode = modeCloudWatch
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
	a.mode = modeCloudWatch
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

func (a App) peekLogStream(streamName string) (App, tea.Cmd) {
	a.prevState = viewLogStreams
	logGroup := a.logStreamsView.LogGroup()
	f := false
	return a, func() tea.Msg {
		return logReadyMsg{
			title:    fmt.Sprintf("%s / %s", logGroup, streamName),
			logGroup: logGroup,
			streams:  []string{streamName},
			follow:   &f,
			lookback: 1 * time.Minute,
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
		if len(streams) == 0 {
			return logReadyMsg{title: title, logGroup: logGroup, streams: nil}
		}
		return logReadyMsg{title: title, logGroup: logGroup, streams: streams}
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
	a.logSearchStream = ""
	return a.promptLogSearchTimeRange()
}

func (a App) promptLogSearchFromStreams() (App, tea.Cmd) {
	s := a.logStreamsView.SelectedStream()
	if s == nil {
		return a, nil
	}
	a.prevState = viewLogStreams
	a.logSearchGroup = a.logStreamsView.LogGroup()
	a.logSearchGroups = []string{a.logSearchGroup}
	a.logSearchStream = s.Name
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
	})
	return a, nil
}

func (a App) handleTimeRangePick(value string) (App, tea.Cmd) {
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

	a.input = NewInput(InputLogSearchPattern, "Search pattern (CloudWatch filter syntax)", "")
	return a, nil
}

func (a App) startLogSearch(pattern string) (App, tea.Cmd) {
	a.state = viewLogSearch

	searchScope := a.logSearchGroup
	if len(a.logSearchGroups) > 1 {
		searchScope = fmt.Sprintf("%d groups", len(a.logSearchGroups))
	}
	a.logSearchView = views.NewLogSearch(searchScope, a.logSearchStream, pattern)
	a.logSearchView = a.logSearchView.SetSize(a.width, a.height-3)

	client := a.client
	groups := a.logSearchGroups
	stream := a.logSearchStream
	startMs := a.logSearchStartMs
	endMs := a.logSearchEndMs

	if len(groups) > 1 {
		// Multi-group: search each group sequentially, streaming results
		return a, searchNextGroup(client, groups, 0, pattern, stream, startMs, endMs)
	}

	// Single group: paginated streaming search
	return a, searchGroupPaginated(client, groups[0], stream, pattern, startMs, endMs, nil, 500)
}

// searchNextGroup searches one group and chains to the next via partial messages.
func searchNextGroup(client *e9saws.Client, groups []string, idx int, pattern, stream string, startMs, endMs int64) tea.Cmd {
	return func() tea.Msg {
		group := groups[idx]
		isLast := idx == len(groups)-1

		var streams []string
		if stream != "" {
			streams = []string{stream}
		}

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
func searchGroupPaginated(client *e9saws.Client, group, stream, pattern string, startMs, endMs int64, nextToken *string, remaining int) tea.Cmd {
	return func() tea.Msg {
		input := &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName:  &group,
			FilterPattern: &pattern,
		}
		if stream != "" {
			input.LogStreamNames = []string{stream}
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
