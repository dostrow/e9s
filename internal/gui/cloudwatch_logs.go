//go:build gui

package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
)

func isECSPage(page string) bool {
	switch page {
	case pageClusters, pageServices, pageTasks, pageStandaloneTasks, pageStoppedTasks, pageTaskDefinitions:
		return true
	default:
		return false
	}
}

func (w *mainWindow) openLogGroupsModule() {
	if w.currentPage == pageLogGroups {
		return
	}
	w.loadLogGroups()
}

func (w *mainWindow) loadLogGroups() {
	w.resetWorkspaceForBrowserChange()
	w.clearLogGroupBrowser()
	w.currentPage = pageLogGroups
	w.selectedCluster = ""
	w.selectedService = ""
	w.selectedTask = ""
	w.selectedTaskDefinition = nil
	w.selectedLogGroup = ""
	w.updateActionSensitivity()
	w.setBreadcrumb("CloudWatch Logs / Log groups")
	w.backButton.SetSensitive(false)
	w.search.SetPlaceholderText("Filter log groups…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageLogGroups)
	w.setDetail("Loading CloudWatch log groups…", detailIntro)

	if w.options.Logs == nil {
		w.setDetail("CloudWatch Logs is unavailable because no log service was configured.", detailError)
		w.setStatus("CloudWatch Logs service unavailable", true)
		return
	}

	ctx, generation := w.startRequest("Loading CloudWatch log groups…")
	go func() {
		groups, err := w.options.Logs.ListGroups(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			w.allLogGroups = groups
			w.applyLogGroupFilter()
			w.setDetail(logGroupsSummary(len(groups)), detailIntro)
		})
	}()
}

func (w *mainWindow) refreshLogGroups(foreground bool) {
	if w.options.Logs == nil {
		return
	}
	selected := w.selectedLogGroup
	ctx, generation := w.startRefreshRequest("Refreshing CloudWatch log groups…", foreground)
	go func() {
		groups, err := w.options.Logs.ListGroups(ctx, "")
		w.finishRefreshRequest(ctx, generation, err, func() {
			w.allLogGroups = groups
			w.applyLogGroupFilter()
			if selected == "" {
				if w.detailContent == detailIntro {
					w.setDetail(logGroupsSummary(len(groups)), detailIntro)
				}
				return
			}
			group, found := findLogGroup(groups, selected)
			if !found {
				w.selectedLogGroup = ""
				w.setDetail("The selected log group is no longer available.\n\n"+logGroupsSummary(len(groups)), detailIntro)
				return
			}
			if w.detailContent == detailLogGroup {
				w.setDetail(formatLogGroupDetail(group), detailLogGroup)
			}
		})
	}()
}

func (w *mainWindow) clearLogGroupBrowser() {
	w.allLogGroups = nil
	w.filteredLogGroups = nil
	w.selectedLogGroup = ""
	if w.logGroupTable != nil {
		w.logGroupTable.clear()
	}
}

func (w *mainWindow) clearLogStreamBrowser() {
	w.allLogStreams = nil
	w.filteredLogStreams = nil
	w.selectedLogStream = ""
	if w.logStreamTable != nil {
		w.logStreamTable.clear()
	}
}

func (w *mainWindow) applyLogGroupFilter() {
	w.filteredLogGroups = filterLogGroups(w.allLogGroups, w.search.Text())
	rows := make([]string, len(w.filteredLogGroups))
	for i, group := range w.filteredLogGroups {
		rows[i] = fmt.Sprintf("%s\t%s", group.Name, formatByteSize(group.StoredBytes))
	}
	w.logGroupTable.replace(rows)
}

func (w *mainWindow) openLogGroupAt(position uint) {
	if int(position) >= len(w.filteredLogGroups) {
		return
	}
	w.loadLogStreams(w.filteredLogGroups[position].Name)
}

func (w *mainWindow) selectLogGroupRow() {
	if w.currentPage != pageLogGroups {
		return
	}
	position := w.logGroupTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredLogGroups) {
		w.selectedLogGroup = ""
		return
	}
	group := w.filteredLogGroups[position]
	if w.selectedLogGroup != group.Name {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedLogGroup = group.Name
	w.setBreadcrumb("CloudWatch Logs / Log groups / " + group.Name)
	w.setDetail(formatLogGroupDetail(group), detailLogGroup)
	w.updateActionSensitivity()
}

func (w *mainWindow) loadLogStreams(group string) {
	if group == "" || w.options.Logs == nil {
		return
	}
	w.resetWorkspaceForBrowserChange()
	w.clearLogStreamBrowser()
	w.currentPage = pageLogStreams
	w.selectedLogGroup = group
	w.updateActionSensitivity()
	w.setBreadcrumb("CloudWatch Logs / " + group)
	w.backButton.SetSensitive(true)
	w.search.SetPlaceholderText("Filter log streams…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageLogStreams)
	w.setDetail("Loading streams in "+group+"…", detailLogGroup)

	ctx, generation := w.startRequest("Loading streams in " + group + "…")
	go func() {
		streams, err := w.options.Logs.ListStreams(ctx, group, "")
		w.finishRequest(ctx, generation, err, func() {
			w.allLogStreams = streams
			w.applyLogStreamFilter()
			w.setDetail(logStreamsSummary(group, len(streams)), detailLogGroup)
		})
	}()
}

func (w *mainWindow) refreshLogStreams(foreground bool) {
	if w.options.Logs == nil || w.selectedLogGroup == "" {
		return
	}
	group, selected := w.selectedLogGroup, w.selectedLogStream
	ctx, generation := w.startRefreshRequest("Refreshing streams in "+group+"…", foreground)
	go func() {
		streams, err := w.options.Logs.ListStreams(ctx, group, "")
		w.finishRefreshRequest(ctx, generation, err, func() {
			w.allLogStreams = streams
			w.applyLogStreamFilter()
			if selected == "" {
				if w.detailContent == detailLogGroup {
					w.setDetail(logStreamsSummary(group, len(streams)), detailLogGroup)
				}
				return
			}
			stream, found := findLogStream(streams, selected)
			if !found {
				w.selectedLogStream = ""
				w.updateActionSensitivity()
				w.setDetail("The selected log stream is no longer available.\n\n"+logStreamsSummary(group, len(streams)), detailLogGroup)
				return
			}
			if w.detailContent == detailLogStream {
				w.setDetail(formatLogStreamDetail(group, stream), detailLogStream)
			}
		})
	}()
}

func (w *mainWindow) applyLogStreamFilter() {
	w.filteredLogStreams = filterLogStreams(w.allLogStreams, w.search.Text())
	rows := make([]string, len(w.filteredLogStreams))
	for i, stream := range w.filteredLogStreams {
		rows[i] = fmt.Sprintf("%s\t%s\t%s", stream.Name,
			formatLogEventTime(stream.LastEventTime), formatLogEventTime(stream.FirstEventTime))
	}
	w.logStreamTable.replace(rows)
}

func (w *mainWindow) selectLogStreamRow() {
	if w.currentPage != pageLogStreams {
		return
	}
	position := w.logStreamTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredLogStreams) {
		w.selectedLogStream = ""
		w.updateActionSensitivity()
		return
	}
	stream := w.filteredLogStreams[position]
	if w.selectedLogStream != stream.Name {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedLogStream = stream.Name
	w.setBreadcrumb("CloudWatch Logs / " + w.selectedLogGroup + " / " + stream.Name)
	w.setDetail(formatLogStreamDetail(w.selectedLogGroup, stream), detailLogStream)
	w.updateActionSensitivity()
}

func (w *mainWindow) peekLogStreamAt(position uint) {
	if int(position) >= len(w.filteredLogStreams) {
		return
	}
	w.selectedLogStream = w.filteredLogStreams[position].Name
	w.peekSelectedLogStream()

}

func (w *mainWindow) peekSelectedLogStream() {
	stream, found := findLogStream(w.allLogStreams, w.selectedLogStream)
	if !found || w.selectedLogGroup == "" || w.options.Logs == nil {
		return
	}
	group := w.selectedLogGroup
	end := time.Now()
	start := end.Add(-15 * time.Minute)
	ctx, generation := w.startRequest("Loading recent events from " + stream.Name + "…")
	go func() {
		page, err := w.options.Logs.Fetch(ctx, group, model.LogQuery{
			Streams: []string{stream.Name}, StartTime: start.UnixMilli(), EndTime: end.UnixMilli(), Limit: 500,
		})
		w.finishRequestWithStatus(ctx, generation, err, fmt.Sprintf("Loaded %d events from %s", len(page.Entries), stream.Name), func() {
			w.showLogSnapshot(model.LogSource{Group: group, Streams: []string{stream.Name}}, stream.Name, page)
		})
	}()
}

func (w *mainWindow) followSelectedLogStream() {
	if w.selectedLogGroup == "" || w.selectedLogStream == "" || w.options.Logs == nil {
		return
	}
	w.showLogFollow(model.LogSource{Group: w.selectedLogGroup, Streams: []string{w.selectedLogStream}}, w.selectedLogStream)
}

func (w *mainWindow) followSelectedLogGroup() {
	if w.selectedLogGroup == "" || w.options.Logs == nil {
		return
	}
	w.showLogFollow(model.LogSource{Group: w.selectedLogGroup}, w.selectedLogGroup+" (all streams)")
}

func filterLogGroups(groups []model.LogGroup, query string) []model.LogGroup {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]model.LogGroup(nil), groups...)
	}
	filtered := make([]model.LogGroup, 0, len(groups))
	for _, group := range groups {
		if strings.Contains(strings.ToLower(group.Name), query) {
			filtered = append(filtered, group)
		}
	}
	return filtered
}

func filterLogStreams(streams []model.LogStream, query string) []model.LogStream {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]model.LogStream(nil), streams...)
	}
	filtered := make([]model.LogStream, 0, len(streams))
	for _, stream := range streams {
		if strings.Contains(strings.ToLower(stream.Name), query) {
			filtered = append(filtered, stream)
		}
	}
	return filtered
}

func findLogGroup(groups []model.LogGroup, name string) (model.LogGroup, bool) {
	for _, group := range groups {
		if group.Name == name {
			return group, true
		}
	}
	return model.LogGroup{}, false
}

func findLogStream(streams []model.LogStream, name string) (model.LogStream, bool) {
	for _, stream := range streams {
		if stream.Name == name {
			return stream, true
		}
	}
	return model.LogStream{}, false
}

func logGroupsSummary(count int) string {
	if count == 0 {
		return "No CloudWatch log groups were found in this region."
	}
	return fmt.Sprintf("CloudWatch Logs\n\n%d log groups\n\nFilter the browser or select a group to inspect it.", count)
}

func formatLogGroupDetail(group model.LogGroup) string {
	return fmt.Sprintf("LOG GROUP\n\n%s\n\nStored data   %s\n\nPress Enter to browse streams.",
		group.Name, formatByteSize(group.StoredBytes))
}

func logStreamsSummary(group string, count int) string {
	if count == 0 {
		return "No log streams were found in " + group + "."
	}
	return fmt.Sprintf("LOG GROUP\n\n%s\n\n%d streams\n\nSelect a stream to inspect it. Double-click or press Enter to peek at its recent events.", group, count)
}

func formatLogStreamDetail(group string, stream model.LogStream) string {
	return fmt.Sprintf("LOG STREAM\n\n%s\n\nGroup         %s\nFirst event   %s\nLast event    %s",
		stream.Name, group, formatLogEventTime(stream.FirstEventTime), formatLogEventTime(stream.LastEventTime))
}

func formatLogEventTime(milliseconds int64) string {
	if milliseconds <= 0 {
		return "—"
	}
	return time.UnixMilli(milliseconds).Local().Format("2006-01-02 15:04:05")
}

func formatByteSize(bytes int64) string {
	const unit = int64(1024)
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	divisor, exponent := unit, 0
	for value := bytes / unit; value >= unit; value /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(divisor), "KMGTPE"[exponent])
}
