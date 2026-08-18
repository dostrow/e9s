//go:build gui

package gui

import (
	"fmt"
	"strings"

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
	group := w.filteredLogGroups[position]
	if w.selectedLogGroup != group.Name {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedLogGroup = group.Name
	w.setBreadcrumb("CloudWatch Logs / Log groups / " + group.Name)
	w.setDetail(formatLogGroupDetail(group), detailLogGroup)
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

func findLogGroup(groups []model.LogGroup, name string) (model.LogGroup, bool) {
	for _, group := range groups {
		if group.Name == name {
			return group, true
		}
	}
	return model.LogGroup{}, false
}

func logGroupsSummary(count int) string {
	if count == 0 {
		return "No CloudWatch log groups were found in this region."
	}
	return fmt.Sprintf("CloudWatch Logs\n\n%d log groups\n\nFilter the browser or select a group to inspect it.", count)
}

func formatLogGroupDetail(group model.LogGroup) string {
	return fmt.Sprintf("LOG GROUP\n\n%s\n\nStored data   %s\n\nPress Enter again in the next phase to browse streams.",
		group.Name, formatByteSize(group.StoredBytes))
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
