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

func (w *mainWindow) buildMetricsPane() gtk.Widgetter {
	back := gtk.NewButtonWithLabel("Back to details")
	back.ConnectClicked(w.closeMetrics)
	refresh := gtk.NewButtonWithLabel("Refresh metrics")
	refresh.ConnectClicked(func() { w.loadServiceMetrics(true) })
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
	toolbar.Append(w.metricsScaleButton)
	toolbar.Append(w.metricsScaleLabel)

	w.metricsTimestamp = gtk.NewLabel("No metrics loaded")
	w.metricsTimestamp.SetXAlign(0)
	w.metricsTimestamp.AddCSSClass("muted")
	w.metricsCPUAvg = newMetricBar()
	w.metricsCPUMax = newMetricBar()
	w.metricsMemAvg = newMetricBar()
	w.metricsMemMax = newMetricBar()

	metrics := gtk.NewBox(gtk.OrientationVertical, 8)
	metrics.SetMarginTop(16)
	metrics.SetMarginBottom(16)
	metrics.SetMarginStart(16)
	metrics.SetMarginEnd(16)
	title := gtk.NewLabel("SERVICE UTILIZATION — LAST 15 MINUTES")
	title.SetXAlign(0)
	title.AddCSSClass("section-title")
	metrics.Append(title)
	metrics.Append(w.metricsTimestamp)
	appendMetricBar(metrics, "CPU average", w.metricsCPUAvg)
	appendMetricBar(metrics, "CPU maximum", w.metricsCPUMax)
	appendMetricBar(metrics, "Memory average", w.metricsMemAvg)
	appendMetricBar(metrics, "Memory maximum", w.metricsMemMax)

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

	content := gtk.NewBox(gtk.OrientationVertical, 8)
	content.SetVExpand(true)
	content.Append(metrics)
	content.Append(alarmTitle)
	content.Append(alarmScroll)

	pane := gtk.NewBox(gtk.OrientationVertical, 0)
	pane.Append(toolbar)
	pane.Append(content)
	return pane
}

func newMetricBar() *gtk.ProgressBar {
	bar := gtk.NewProgressBar()
	bar.SetShowText(true)
	bar.SetText("—")
	return bar
}

func appendMetricBar(box *gtk.Box, labelText string, bar *gtk.ProgressBar) {
	label := gtk.NewLabel(labelText)
	label.SetXAlign(0)
	box.Append(label)
	box.Append(bar)
}

func (w *mainWindow) openServiceMetrics() {
	if w.selectedCluster == "" || w.selectedService == "" {
		return
	}
	w.loadServiceMetrics(true)
}

func (w *mainWindow) loadServiceMetrics(foreground bool) {
	if w.selectedCluster == "" || w.selectedService == "" {
		return
	}
	opening := !w.showingMetrics
	cluster, service := w.selectedCluster, w.selectedService
	ctx, generation := w.startRefreshRequest("Loading metrics for "+service+"…", foreground)
	go func() {
		metrics, err := w.options.ECS.GetServiceMetrics(ctx, cluster, service, 15*time.Minute)
		var (
			alarms     []model.AlarmState
			suspended  bool
			scaleKnown bool
			warnings   []string
		)
		if err == nil {
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
		success := "Metrics updated for " + service
		if len(warnings) > 0 {
			success += " • " + strings.Join(warnings, " • ")
		}
		w.finishRequestResult(ctx, generation, err, success, opening, func() {
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
			w.scaleInKnown = scaleKnown
			w.scaleInSuspended = suspended
			w.renderServiceMetrics()
			w.detailStack.SetVisibleChildName("metrics")
		})
	}()
}

func (w *mainWindow) renderServiceMetrics() {
	if w.metricsSnapshot == nil {
		return
	}
	m := w.metricsSnapshot
	setMetricBar(w.metricsCPUAvg, m.CPUAvg)
	setMetricBar(w.metricsCPUMax, m.CPUMax)
	setMetricBar(w.metricsMemAvg, m.MemAvg)
	setMetricBar(w.metricsMemMax, m.MemMax)
	w.metricsTimestamp.SetLabel("Sampled " + formatTime(m.Timestamp))

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

func setMetricBar(bar *gtk.ProgressBar, value float64) {
	bar.SetFraction(metricFraction(value))
	bar.SetText(fmt.Sprintf("%.1f%%", value))
}

func metricFraction(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 1
	}
	return value / 100
}

func (w *mainWindow) confirmToggleScaleIn() {
	if !w.showingMetrics || !w.scaleInKnown || w.selectedService == "" {
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
			w.renderServiceMetrics()
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
