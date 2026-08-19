//go:build gui

package gui

import (
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
)

func (w *mainWindow) openAlarmsModule(state string) {
	if w.currentPage == pageAlarms && w.alarmStateFilter == state {
		return
	}
	w.loadAlarms(state)
}

func (w *mainWindow) loadAlarms(state string) {
	w.resetWorkspaceForBrowserChange()
	w.clearAlarmBrowser()
	w.currentPage = pageAlarms
	w.alarmStateFilter = state
	w.selectedCluster = ""
	w.selectedService = ""
	w.selectedTask = ""
	w.selectedTaskDefinition = nil
	w.updateActionSensitivity()
	w.setBreadcrumb(alarmBreadcrumb(state, ""))
	w.backButton.SetSensitive(false)
	w.search.SetPlaceholderText("Filter alarms…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageAlarms)
	w.setDetail("Loading CloudWatch alarms…", detailIntro)

	if w.options.Alarms == nil {
		w.setDetail("CloudWatch Alarms is unavailable because no alarm service was configured.", detailError)
		w.setStatus("CloudWatch Alarms service unavailable", true)
		return
	}

	ctx, generation := w.startRequest("Loading CloudWatch alarms…")
	go func() {
		alarms, err := w.options.Alarms.List(ctx, state)
		w.finishRequest(ctx, generation, err, func() {
			w.allAlarms = alarms
			w.applyAlarmFilter()
			w.setDetail(alarmListSummary(state, len(alarms)), detailIntro)
		})
	}()
}

func (w *mainWindow) refreshAlarms(foreground bool) {
	if w.options.Alarms == nil {
		return
	}
	state, selected := w.alarmStateFilter, w.selectedAlarm
	ctx, generation := w.startRefreshRequest("Refreshing CloudWatch alarms…", foreground)
	go func() {
		alarms, err := w.options.Alarms.List(ctx, state)
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allAlarms = alarms
			w.applyAlarmFilter()
			if selected == "" {
				if w.detailContent == detailIntro {
					w.setDetail(alarmListSummary(state, len(alarms)), detailIntro)
				}
				return
			}
			alarm, found := findAlarm(alarms, selected)
			if !found {
				w.selectedAlarm = ""
				w.setBreadcrumb(alarmBreadcrumb(state, ""))
				w.setDetail("The selected alarm is no longer available.\n\n"+alarmListSummary(state, len(alarms)), detailIntro)
				return
			}
			if w.detailContent == detailAlarm {
				w.setDetail(formatAlarmSummary(alarm), detailAlarm)
			}
		})
	}()
}

func (w *mainWindow) clearAlarmBrowser() {
	w.allAlarms = nil
	w.filteredAlarms = nil
	w.selectedAlarm = ""
	if w.alarmTable != nil {
		w.alarmTable.clear()
	}
}

func (w *mainWindow) applyAlarmFilter() {
	w.filteredAlarms = filterAlarms(w.allAlarms, w.search.Text())
	rows := make([]string, len(w.filteredAlarms))
	for i, alarm := range w.filteredAlarms {
		actions := "enabled"
		if !alarm.ActionsEnabled {
			actions = "disabled"
		}
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s", alarm.Name, alarm.State,
			valueOrDash(alarm.MetricName), valueOrDash(alarm.Namespace), actions, formatTime(alarm.StateUpdatedAt))
	}
	w.alarmTable.replace(rows)
}

func (w *mainWindow) selectAlarmRow() {
	if w.currentPage != pageAlarms {
		return
	}
	position := w.alarmTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredAlarms) {
		w.selectedAlarm = ""
		w.updateActionSensitivity()
		return
	}
	alarm := w.filteredAlarms[position]
	if w.selectedAlarm != alarm.Name {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedAlarm = alarm.Name
	w.setBreadcrumb(alarmBreadcrumb(w.alarmStateFilter, alarm.Name))
	w.setDetail(formatAlarmSummary(alarm), detailAlarm)
	w.updateActionSensitivity()
}

func (w *mainWindow) openAlarmAt(position uint) {
	if int(position) >= len(w.filteredAlarms) {
		return
	}
	w.alarmTable.selection.SetSelected(position)
}

func filterAlarms(alarms []model.Alarm, query string) []model.Alarm {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]model.Alarm(nil), alarms...)
	}
	filtered := make([]model.Alarm, 0, len(alarms))
	for _, alarm := range alarms {
		if strings.Contains(strings.ToLower(alarm.Name), query) ||
			strings.Contains(strings.ToLower(alarm.MetricName), query) ||
			strings.Contains(strings.ToLower(alarm.Namespace), query) {
			filtered = append(filtered, alarm)
		}
	}
	return filtered
}

func findAlarm(alarms []model.Alarm, name string) (model.Alarm, bool) {
	for _, alarm := range alarms {
		if alarm.Name == name {
			return alarm, true
		}
	}
	return model.Alarm{}, false
}

func alarmBreadcrumb(state, name string) string {
	crumb := "CloudWatch Alarms / " + alarmScopeLabel(state)
	if name != "" {
		crumb += " / " + name
	}
	return crumb
}

func alarmScopeLabel(state string) string {
	switch state {
	case model.AlarmStateAlarm:
		return "In alarm"
	case model.AlarmStateOK:
		return "OK"
	case model.AlarmStateInsufficientData:
		return "Insufficient data"
	default:
		return "All alarms"
	}
}

func alarmListSummary(state string, count int) string {
	return fmt.Sprintf("CLOUDWATCH ALARMS\n\nScope            %s\nAlarms           %d\n\nSelect an alarm to inspect its current state.",
		alarmScopeLabel(state), count)
}

func formatAlarmSummary(alarm model.Alarm) string {
	var out strings.Builder
	fmt.Fprintf(&out, "CLOUDWATCH ALARM\n\n")
	fmt.Fprintf(&out, "Name             %s\n", alarm.Name)
	fmt.Fprintf(&out, "State            %s\n", valueOrDash(alarm.State))
	fmt.Fprintf(&out, "Updated          %s\n", formatTime(alarm.StateUpdatedAt))
	fmt.Fprintf(&out, "Actions          %s\n", yesNo(alarm.ActionsEnabled))
	fmt.Fprintf(&out, "Namespace        %s\n", valueOrDash(alarm.Namespace))
	fmt.Fprintf(&out, "Metric           %s\n", valueOrDash(alarm.MetricName))
	if alarm.StateReason != "" {
		fmt.Fprintf(&out, "\nSTATE REASON\n\n%s\n", alarm.StateReason)
	}
	return out.String()
}
