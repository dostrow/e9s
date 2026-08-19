//go:build gui

package gui

import (
	"context"
	"fmt"
	"html"
	"sort"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
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
		var detail *model.AlarmDetail
		if err == nil && foreground && selected != "" {
			detail, err = w.options.Alarms.Detail(ctx, selected)
		}
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
				w.alarmDetail = nil
				w.updateActionSensitivity()
				w.setBreadcrumb(alarmBreadcrumb(state, ""))
				w.setDetail("The selected alarm is no longer available.\n\n"+alarmListSummary(state, len(alarms)), detailIntro)
				return
			}
			if detail != nil {
				w.alarmDetail = detail
			}
			if w.alarmDetail != nil {
				w.alarmDetail.Alarm = alarm
			}
			w.updateActionSensitivity()
			if w.detailContent == detailAlarm && w.alarmDetail != nil {
				w.setDetail(formatAlarmDetail(*w.alarmDetail, w.alarmUTCTime), detailAlarm)
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
			valueOrDash(alarm.MetricName), valueOrDash(alarm.Namespace), actions, formatAlarmTime(alarm.StateUpdatedAt, w.alarmUTCTime))
	}
	w.alarmTable.replace(rows)
}

func (w *mainWindow) selectAlarmRow() {
	if w.currentPage != pageAlarms {
		return
	}
	position := w.alarmTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredAlarms) {
		if w.selectedAlarm != "" {
			w.resetWorkspaceForBrowserChange()
			w.selectedAlarm = ""
			w.setBreadcrumb(alarmBreadcrumb(w.alarmStateFilter, ""))
			w.setDetail(alarmListSummary(w.alarmStateFilter, len(w.allAlarms)), detailIntro)
		}
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
	w.setDetail("Loading details for "+alarm.Name+"…\n\n"+formatAlarmSummary(alarm, w.alarmUTCTime), detailAlarm)
	w.updateActionSensitivity()
	w.loadAlarmDetail(alarm.Name)
}

func (w *mainWindow) openAlarmAt(position uint) {
	if int(position) >= len(w.filteredAlarms) {
		return
	}
	w.alarmTable.selection.SetSelected(position)
	if w.selectedAlarm == w.filteredAlarms[position].Name && w.alarmDetail == nil {
		w.loadAlarmDetail(w.selectedAlarm)
	}
}

func (w *mainWindow) loadAlarmDetail(name string) {
	if name == "" || w.options.Alarms == nil {
		return
	}
	ctx, generation := w.startRequest("Loading alarm details for " + name + "…")
	go func() {
		detail, err := w.options.Alarms.Detail(ctx, name)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageAlarms || w.selectedAlarm != name {
				return
			}
			w.alarmDetail = detail
			w.mergeAlarmDetail(detail)
			w.setDetail(formatAlarmDetail(*detail, w.alarmUTCTime), detailAlarm)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) mergeAlarmDetail(detail *model.AlarmDetail) {
	if detail == nil {
		return
	}
	for i := range w.allAlarms {
		if w.allAlarms[i].Name == detail.Name {
			if w.alarmStateFilter != "" && detail.State != w.alarmStateFilter {
				w.allAlarms = append(w.allAlarms[:i], w.allAlarms[i+1:]...)
				if w.selectedAlarm == detail.Name {
					w.selectedAlarm = ""
					w.alarmDetail = nil
					w.setBreadcrumb(alarmBreadcrumb(w.alarmStateFilter, ""))
					w.setDetail(alarmListSummary(w.alarmStateFilter, len(w.allAlarms)), detailIntro)
				}
				w.applyAlarmFilter()
				return
			}
			w.allAlarms[i] = detail.Alarm
			w.applyAlarmFilter()
			return
		}
	}
}

func (w *mainWindow) confirmToggleAlarmActions() {
	if w.alarmDetail == nil || w.alarmDetail.Name != w.selectedAlarm || w.alarmActionPending {
		return
	}
	enable := !w.alarmDetail.ActionsEnabled
	verb := "Enable"
	messageType := gtk.MessageQuestion
	secondary := "Alarm actions will run again when this alarm changes state."
	if !enable {
		verb = "Disable"
		messageType = gtk.MessageWarning
		secondary = "Configured alarm, OK, and insufficient-data actions will not run while actions are disabled."
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, messageType, gtk.ButtonsYesNo)
	dialog.SetTitle(verb + " alarm actions")
	dialog.SetMarkup(verb + " actions for <b>" + html.EscapeString(w.selectedAlarm) + "</b>?")
	dialog.SetObjectProperty("secondary-text", secondary)
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			name := w.selectedAlarm
			w.runAlarmMutation(name, strings.ToLower(verb)+" actions for "+name+"…",
				"Actions "+map[bool]string{true: "enabled", false: "disabled"}[enable]+" for "+name,
				func(ctx context.Context) error { return w.options.Alarms.SetActionsEnabled(ctx, name, enable) },
				func(detail *model.AlarmDetail) { detail.ActionsEnabled = enable })
		}
	})
	dialog.Present()
}

func (w *mainWindow) promptSetAlarmState() {
	if w.alarmDetail == nil || w.alarmDetail.Name != w.selectedAlarm || w.alarmActionPending {
		return
	}
	dialog := gtk.NewDialogWithFlags("Set alarm state", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	content := dialog.ContentArea()
	content.SetSpacing(8)
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)
	warning := gtk.NewLabel("Manual state overrides are intended for testing alarm integrations.")
	warning.SetXAlign(0)
	warning.SetWrap(true)
	warning.AddCSSClass("error")
	content.Append(warning)
	state := gtk.NewDropDownFromStrings([]string{model.AlarmStateOK, model.AlarmStateAlarm, model.AlarmStateInsufficientData})
	state.SetSelected(uint(alarmStateOptionIndex(w.alarmDetail.State)))
	appendDialogField(content, "State", state)
	reason := gtk.NewEntry()
	reason.SetText("Set manually via e9s")
	reason.SetHExpand(true)
	appendDialogField(content, "Reason", reason)
	errorLabel := gtk.NewLabel("")
	errorLabel.SetXAlign(0)
	errorLabel.AddCSSClass("error")
	content.Append(errorLabel)
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Set state", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		if response != int(gtk.ResponseOK) {
			dialog.Destroy()
			return
		}
		reasonText := strings.TrimSpace(reason.Text())
		if reasonText == "" {
			errorLabel.SetLabel("Enter a reason for the manual state change")
			return
		}
		states := []string{model.AlarmStateOK, model.AlarmStateAlarm, model.AlarmStateInsufficientData}
		index := int(state.Selected())
		if index < 0 || index >= len(states) {
			errorLabel.SetLabel("Select a valid alarm state")
			return
		}
		newState, name := states[index], w.selectedAlarm
		dialog.Destroy()
		w.runAlarmMutation(name, "Setting state for "+name+"…", "State set to "+newState+" for "+name,
			func(ctx context.Context) error { return w.options.Alarms.SetState(ctx, name, newState, reasonText) },
			func(detail *model.AlarmDetail) {
				detail.State = newState
				detail.StateReason = reasonText
				detail.StateUpdatedAt = time.Now()
			})
	})
	dialog.Present()
}

func (w *mainWindow) runAlarmMutation(name, label, success string, mutate func(context.Context) error, optimistic func(*model.AlarmDetail)) {
	if name == "" || w.options.Alarms == nil || w.alarmActionPending {
		return
	}
	w.alarmActionPending = true
	w.updateActionSensitivity()
	ctx, generation := w.startRequest(label)
	go func() {
		err := mutate(ctx)
		var detail *model.AlarmDetail
		var refreshErr error
		if err == nil {
			detail, refreshErr = w.options.Alarms.Detail(ctx, name)
		}
		glib.IdleAdd(func() {
			if ctx.Err() != nil || generation != w.generation {
				return
			}
			w.spinner.Stop()
			w.setWorkspaceBusy("", false)
			w.alarmActionPending = false
			if err != nil {
				w.updateActionSensitivity()
				w.setStatus(err.Error(), true)
				return
			}
			if detail != nil {
				w.alarmDetail = detail
				optimistic(w.alarmDetail)
			} else if w.alarmDetail != nil {
				optimistic(w.alarmDetail)
			}
			w.mergeAlarmDetail(w.alarmDetail)
			if w.alarmDetail != nil {
				w.setDetail(formatAlarmDetail(*w.alarmDetail, w.alarmUTCTime), detailAlarm)
			}
			w.updateActionSensitivity()
			w.lastSuccessfulLoad = time.Now()
			if refreshErr != nil {
				success += " • detail refresh: " + refreshErr.Error()
			}
			w.setStatus(success, false)
		})
	}()
}

func alarmStateOptionIndex(state string) int {
	switch state {
	case model.AlarmStateAlarm:
		return 1
	case model.AlarmStateInsufficientData:
		return 2
	default:
		return 0
	}
}

func (w *mainWindow) toggleAlarmTimestamps() {
	if w.currentPage != pageAlarms {
		return
	}
	w.alarmUTCTime = !w.alarmUTCTime
	if w.alarmUTCTime {
		w.alarmTimestampButton.SetLabel("Time: UTC")
	} else {
		w.alarmTimestampButton.SetLabel("Time: Local")
	}
	w.applyAlarmFilter()
	if w.alarmDetail != nil && w.selectedAlarm == w.alarmDetail.Name {
		w.setDetail(formatAlarmDetail(*w.alarmDetail, w.alarmUTCTime), detailAlarm)
	}
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

func formatAlarmSummary(alarm model.Alarm, utc bool) string {
	var out strings.Builder
	fmt.Fprintf(&out, "CLOUDWATCH ALARM\n\n")
	fmt.Fprintf(&out, "Name             %s\n", alarm.Name)
	fmt.Fprintf(&out, "State            %s\n", valueOrDash(alarm.State))
	fmt.Fprintf(&out, "Updated          %s\n", formatAlarmTime(alarm.StateUpdatedAt, utc))
	fmt.Fprintf(&out, "Actions          %s\n", yesNo(alarm.ActionsEnabled))
	fmt.Fprintf(&out, "Namespace        %s\n", valueOrDash(alarm.Namespace))
	fmt.Fprintf(&out, "Metric           %s\n", valueOrDash(alarm.MetricName))
	if alarm.StateReason != "" {
		fmt.Fprintf(&out, "\nSTATE REASON\n\n%s\n", alarm.StateReason)
	}
	return out.String()
}

func formatAlarmDetail(detail model.AlarmDetail, utc bool) string {
	var out strings.Builder
	fmt.Fprintf(&out, "CLOUDWATCH ALARM\n\n")
	fmt.Fprintf(&out, "Name                 %s\n", detail.Name)
	fmt.Fprintf(&out, "State                %s\n", valueOrDash(detail.State))
	fmt.Fprintf(&out, "Updated              %s\n", formatAlarmTime(detail.StateUpdatedAt, utc))
	fmt.Fprintf(&out, "Actions              %s\n", yesNo(detail.ActionsEnabled))
	if detail.StateReason != "" {
		fmt.Fprintf(&out, "Reason               %s\n", detail.StateReason)
	}
	if detail.ARN != "" {
		fmt.Fprintf(&out, "ARN                  %s\n", detail.ARN)
	}

	fmt.Fprintf(&out, "\nCONFIGURATION\n\n")
	if detail.Description != "" {
		fmt.Fprintf(&out, "Description          %s\n", detail.Description)
	}
	fmt.Fprintf(&out, "Namespace            %s\n", valueOrDash(detail.Namespace))
	fmt.Fprintf(&out, "Metric               %s\n", valueOrDash(detail.MetricName))
	if detail.Statistic != "" {
		fmt.Fprintf(&out, "Statistic            %s\n", detail.Statistic)
	}
	fmt.Fprintf(&out, "Comparison           %s\n", valueOrDash(detail.ComparisonOp))
	fmt.Fprintf(&out, "Threshold            %g\n", detail.Threshold)
	fmt.Fprintf(&out, "Evaluation periods   %d\n", detail.EvalPeriods)
	fmt.Fprintf(&out, "Period               %ds\n", detail.Period)
	if detail.TreatMissing != "" {
		fmt.Fprintf(&out, "Treat missing        %s\n", detail.TreatMissing)
	}

	if len(detail.Dimensions) > 0 {
		fmt.Fprintf(&out, "\nDIMENSIONS\n\n")
		keys := make([]string, 0, len(detail.Dimensions))
		for key := range detail.Dimensions {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&out, "%-20s %s\n", key, detail.Dimensions[key])
		}
	}
	appendAlarmActionSection(&out, "ALARM ACTIONS", detail.AlarmActions)
	appendAlarmActionSection(&out, "OK ACTIONS", detail.OKActions)
	appendAlarmActionSection(&out, "INSUFFICIENT DATA ACTIONS", detail.InsufficientActions)

	if len(detail.History) > 0 {
		fmt.Fprintf(&out, "\nRECENT HISTORY\n\n")
		for _, item := range detail.History {
			fmt.Fprintf(&out, "%s  %-22s %s\n", formatAlarmTime(item.Timestamp, utc), valueOrDash(item.Type), item.Summary)
		}
	}
	return out.String()
}

func formatAlarmTime(value time.Time, utc bool) string {
	if value.IsZero() {
		return "—"
	}
	if utc {
		return value.UTC().Format("2006-01-02 15:04:05 UTC")
	}
	return value.Local().Format("2006-01-02 15:04:05")
}

func appendAlarmActionSection(out *strings.Builder, title string, actions []string) {
	if len(actions) == 0 {
		return
	}
	fmt.Fprintf(out, "\n%s\n\n", title)
	for _, action := range actions {
		fmt.Fprintf(out, "%s\n", action)
	}
}
