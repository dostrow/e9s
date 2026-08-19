//go:build gui

package gui

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
)

var metricTimeRanges = []struct {
	label    string
	duration time.Duration
}{
	{"15 minutes", 15 * time.Minute},
	{"1 hour", time.Hour},
	{"6 hours", 6 * time.Hour},
	{"24 hours", 24 * time.Hour},
	{"7 days", 7 * 24 * time.Hour},
}

func (w *mainWindow) buildMetricsPane() gtk.Widgetter {
	back := gtk.NewButtonWithLabel("Back to details")
	back.ConnectClicked(w.closeMetrics)
	refresh := gtk.NewButtonWithLabel("Refresh metrics")
	refresh.ConnectClicked(func() { w.loadMetrics(true) })
	rangeLabels := make([]string, len(metricTimeRanges))
	for i, item := range metricTimeRanges {
		rangeLabels[i] = item.label
	}
	w.metricsRange = gtk.NewDropDownFromStrings(rangeLabels)
	w.metricsRange.SetSelected(0)
	w.metricsRange.SetTooltipText("Metrics time range")
	w.metricsRange.NotifyProperty("selected", func() {
		if w.showingMetrics {
			w.loadMetrics(true)
		}
	})
	w.metricsScaleButton = gtk.NewButtonWithLabel("Toggle scale-in")
	w.metricsScaleButton.ConnectClicked(w.confirmToggleScaleIn)
	w.metricsScaleLabel = gtk.NewLabel("Scale-in status unknown")
	w.metricsScaleLabel.SetXAlign(0)
	w.metricsScaleLabel.SetHExpand(true)
	w.metricsScaleLabel.AddCSSClass("muted")

	toolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	toolbar.AddCSSClass("log-toolbar")
	toolbar.Append(back)
	toolbar.Append(refresh)
	toolbar.Append(w.metricsRange)
	toolbar.Append(w.metricsScaleButton)
	toolbar.Append(w.metricsScaleLabel)

	w.metricsTimestamp = gtk.NewLabel("No metrics loaded")
	w.metricsTimestamp.SetXAlign(0)
	w.metricsTimestamp.AddCSSClass("muted")
	w.metricsCPUChart = newMetricChart("CPU UTILIZATION", "%", true, 100)
	w.metricsMemoryChart = newMetricChart("MEMORY UTILIZATION", "%", true, 100)

	metrics := gtk.NewBox(gtk.OrientationVertical, 8)
	metrics.SetMarginTop(16)
	metrics.SetMarginBottom(16)
	metrics.SetMarginStart(16)
	metrics.SetMarginEnd(16)
	w.metricsTitle = gtk.NewLabel("SERVICE UTILIZATION")
	w.metricsTitle.SetXAlign(0)
	w.metricsTitle.AddCSSClass("section-title")
	w.metricsScope = gtk.NewLabel("")
	w.metricsScope.SetXAlign(0)
	w.metricsScope.SetWrap(true)
	w.metricsScope.AddCSSClass("muted")
	w.metricsNotice = gtk.NewLabel("")
	w.metricsNotice.SetXAlign(0)
	w.metricsNotice.SetWrap(true)
	w.metricsNotice.AddCSSClass("muted")
	w.metricsNotice.SetVisible(false)
	metrics.Append(w.metricsTitle)
	metrics.Append(w.metricsScope)
	metrics.Append(w.metricsTimestamp)
	metrics.Append(w.metricsNotice)
	metrics.Append(w.metricsCPUChart.area)
	metrics.Append(w.metricsMemoryChart.area)

	w.metricsAlarmTable = newStringTable([]columnSpec{
		{title: "ALARM", field: 0, expand: true},
		{title: "STATE", field: 1},
		{title: "METRIC", field: 2},
		{title: "UPDATED", field: 3},
	})
	alarmTitle := gtk.NewLabel("CLOUDWATCH ALARMS")
	alarmTitle.SetXAlign(0)
	alarmTitle.AddCSSClass("section-title")
	alarmScroll := gtk.NewScrolledWindow()
	alarmScroll.SetHExpand(true)
	alarmScroll.SetVExpand(true)
	alarmScroll.SetChild(w.metricsAlarmTable.view)

	w.metricsAlarmSection = gtk.NewBox(gtk.OrientationVertical, 8)
	w.metricsAlarmSection.SetVExpand(true)
	w.metricsAlarmSection.Append(alarmTitle)
	w.metricsAlarmSection.Append(alarmScroll)

	content := gtk.NewBox(gtk.OrientationVertical, 8)
	content.SetVExpand(true)
	content.Append(metrics)
	content.Append(w.metricsAlarmSection)

	pane := gtk.NewBox(gtk.OrientationVertical, 0)
	pane.Append(toolbar)
	pane.Append(content)
	return pane
}

func (w *mainWindow) openMetrics() {
	if w.selectedCluster == "" || (w.selectedService == "" && w.selectedTask == "") {
		return
	}
	w.loadMetrics(true)
}

func (w *mainWindow) loadMetrics(foreground bool) {
	if w.selectedCluster == "" || (w.selectedService == "" && w.selectedTask == "") {
		return
	}
	opening := !w.showingMetrics
	cluster, service, selectedTask := w.selectedCluster, w.selectedService, w.selectedTask
	var task model.Task
	if selectedTask != "" {
		var found bool
		task, found = findTask(w.allTasks, selectedTask)
		if !found {
			w.setStatus("The selected task is no longer available", true)
			return
		}
	}
	scopeName := service
	if selectedTask != "" {
		scopeName = task.TaskID
	}
	ctx, generation := w.startRefreshRequest("Loading metrics for "+scopeName+"…", foreground)
	go func() {
		var metrics *model.ServiceMetrics
		var err error
		window := w.metricsWindow()
		if selectedTask != "" {
			metrics, err = w.options.ECS.GetTaskMetrics(ctx, cluster, service, task, window)
		} else {
			metrics, err = w.options.ECS.GetServiceMetrics(ctx, cluster, service, window)
		}
		var (
			alarms     []model.AlarmState
			suspended  bool
			scaleKnown bool
			warnings   []string
		)
		if err == nil && selectedTask == "" {
			var alarmErr error
			alarms, alarmErr = w.options.ECS.ListServiceAlarms(ctx, cluster, service)
			if alarmErr != nil {
				warnings = append(warnings, "alarms unavailable: "+alarmErr.Error())
			}
			suspended, alarmErr = w.options.ECS.ScaleInSuspended(ctx, cluster, service)
			if alarmErr != nil {
				warnings = append(warnings, "scale-in status unavailable: "+alarmErr.Error())
			} else {
				scaleKnown = true
			}
		}
		success := "Metrics updated for " + scopeName
		if len(warnings) > 0 {
			success += " • " + strings.Join(warnings, " • ")
		}
		w.finishRequestResult(ctx, generation, err, success, opening, foreground, func() {
			if w.showingTerminal {
				w.closeTerminalNow(false)
			}
			if w.logCancel != nil && w.showingLogs {
				w.logCancel()
				w.logGeneration++
			}
			w.showingLogs = false
			w.showingMetrics = true
			w.metricsSnapshot = metrics
			w.metricsAlarms = alarms
			w.metricsTaskID = task.TaskID
			w.scaleInKnown = scaleKnown
			w.scaleInSuspended = suspended
			w.renderMetrics()
			w.detailStack.SetVisibleChildName("metrics")
		})
	}()
}

func (w *mainWindow) renderMetrics() {
	if w.metricsSnapshot == nil {
		return
	}
	m := w.metricsSnapshot
	w.metricsCPUChart.SetData(m.StartTime, m.EndTime, m.Series, "cpu_avg", "cpu_max")
	w.metricsMemoryChart.SetData(m.StartTime, m.EndTime, m.Series, "mem_avg", "mem_max")
	w.metricsTimestamp.SetLabel(fmt.Sprintf("Updated %s • %s resolution", formatTime(m.Timestamp), formatMetricPeriod(m.Period)))

	taskScope := w.metricsTaskID != ""
	if taskScope {
		w.metricsTitle.SetLabel("TASK UTILIZATION — " + strings.ToUpper(w.metricsRangeLabel()))
		scope := "Task " + w.metricsTaskID
		if w.selectedService != "" {
			scope += " • service " + w.selectedService
		}
		w.metricsScope.SetLabel(scope)
	} else {
		w.metricsTitle.SetLabel("SERVICE UTILIZATION — " + strings.ToUpper(w.metricsRangeLabel()))
		w.metricsScope.SetLabel("Service " + w.selectedService + " • all running tasks")
	}
	hasData := m.CPUAvgAvailable || m.CPUMaxAvailable || m.MemAvgAvailable || m.MemMaxAvailable
	if !hasData && taskScope {
		w.metricsNotice.SetLabel("No task-level datapoints were returned. Enable ECS Container Insights with enhanced observability and allow time for metrics to arrive.")
		w.metricsNotice.SetVisible(true)
	} else if !hasData {
		w.metricsNotice.SetLabel("No service-level datapoints were returned for the selected period.")
		w.metricsNotice.SetVisible(true)
	} else {
		w.metricsNotice.SetVisible(false)
	}
	w.metricsScaleButton.SetVisible(!taskScope)
	w.metricsScaleLabel.SetVisible(!taskScope)
	w.metricsAlarmSection.SetVisible(!taskScope)

	rows := make([]string, len(w.metricsAlarms))
	for i, alarm := range w.metricsAlarms {
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s", valueOrDash(alarm.Name), valueOrDash(alarm.State),
			valueOrDash(alarm.MetricName), formatTime(alarm.UpdatedAt))
	}
	w.metricsAlarmTable.replace(rows)
	if w.scaleInKnown {
		if w.scaleInSuspended {
			w.metricsScaleLabel.SetLabel("Scale-in is suspended")
			w.metricsScaleButton.SetLabel("Resume scale-in")
		} else {
			w.metricsScaleLabel.SetLabel("Scale-in is enabled")
			w.metricsScaleButton.SetLabel("Suspend scale-in")
		}
	} else {
		w.metricsScaleLabel.SetLabel("Scale-in status unavailable")
		w.metricsScaleButton.SetLabel("Toggle scale-in")
	}
	w.metricsScaleButton.SetSensitive(w.scaleInKnown)
}

func (w *mainWindow) metricsWindow() time.Duration {
	if w.metricsRange == nil {
		return metricTimeRanges[0].duration
	}
	selected := int(w.metricsRange.Selected())
	if selected < 0 || selected >= len(metricTimeRanges) {
		return metricTimeRanges[0].duration
	}
	return metricTimeRanges[selected].duration
}

func (w *mainWindow) metricsRangeLabel() string {
	if w.metricsRange == nil {
		return metricTimeRanges[0].label
	}
	selected := int(w.metricsRange.Selected())
	if selected < 0 || selected >= len(metricTimeRanges) {
		return metricTimeRanges[0].label
	}
	return metricTimeRanges[selected].label
}

func formatMetricPeriod(period time.Duration) string {
	if period <= 0 {
		return "unknown"
	}
	if period%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(period/time.Hour))
	}
	if period%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(period/time.Minute))
	}
	return period.String()
}

func (w *mainWindow) confirmToggleScaleIn() {
	if !w.showingMetrics || w.metricsTaskID != "" || !w.scaleInKnown || w.selectedService == "" {
		return
	}
	action := "Suspend"
	if w.scaleInSuspended {
		action = "Resume"
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageQuestion, gtk.ButtonsYesNo)
	dialog.SetTitle(action + " service scale-in")
	dialog.SetMarkup(action + " automatic scale-in for <b>" + html.EscapeString(w.selectedService) + "</b>?")
	dialog.SetDestroyWithParent(true)
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseYes) {
			w.setScaleInSuspended(!w.scaleInSuspended)
		}
	})
	dialog.Present()
}

func (w *mainWindow) setScaleInSuspended(suspended bool) {
	cluster, service := w.selectedCluster, w.selectedService
	verb := "Resuming"
	if suspended {
		verb = "Suspending"
	}
	ctx, generation := w.startRequest(verb + " scale-in for " + service + "…")
	go func() {
		err := w.options.ECS.SetScaleInSuspended(ctx, cluster, service, suspended)
		success := "Scale-in resumed for " + service
		if suspended {
			success = "Scale-in suspended for " + service
		}
		w.finishRequestWithStatus(ctx, generation, err, success, func() {
			w.scaleInSuspended = suspended
			w.scaleInKnown = true
			w.renderMetrics()
			w.refreshAfterAction()
		})
	}()
}

func (w *mainWindow) closeMetrics() {
	if !w.showingMetrics {
		return
	}
	w.showingMetrics = false
	w.detailStack.SetVisibleChildName("detail")
	w.setStatus("Metrics view closed", false)
}
