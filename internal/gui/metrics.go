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
	{"2 weeks", 14 * 24 * time.Hour},
	{"30 days", 30 * 24 * time.Hour},
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
	w.metricsTimeButton = gtk.NewButtonWithLabel("Time: Local")
	w.metricsTimeButton.SetTooltipText("Toggle chart timestamps between local time and UTC")
	w.metricsTimeButton.ConnectClicked(w.toggleMetricTimestamps)
	w.metricsRestoreButton = gtk.NewButtonWithLabel("Show all charts")
	w.metricsRestoreButton.SetVisible(false)
	w.metricsRestoreButton.ConnectClicked(w.restoreMetricCharts)
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
	toolbar.Append(w.metricsTimeButton)
	toolbar.Append(w.metricsRestoreButton)
	toolbar.Append(w.metricsScaleButton)
	toolbar.Append(w.metricsScaleLabel)

	w.metricsTimestamp = gtk.NewLabel("No metrics loaded")
	w.metricsTimestamp.SetXAlign(0)
	w.metricsTimestamp.AddCSSClass("muted")
	w.metricsChartsBox = gtk.NewBox(gtk.OrientationVertical, 12)
	w.metricsChartsBox.SetHExpand(true)

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
	metrics.Append(w.metricsChartsBox)

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
	scroll := gtk.NewScrolledWindow()
	scroll.SetHExpand(true)
	scroll.SetVExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(content)

	pane := gtk.NewBox(gtk.OrientationVertical, 0)
	pane.Append(toolbar)
	pane.Append(scroll)
	return pane
}

func (w *mainWindow) toggleMetricTimestamps() {
	w.metricsUTCTime = !w.metricsUTCTime
	label := "Time: Local"
	status := "Chart timestamps use local time"
	if w.metricsUTCTime {
		label = "Time: UTC"
		status = "Chart timestamps use UTC"
	}
	w.metricsTimeButton.SetLabel(label)
	if w.metricsRenderedSnapshot != nil {
		w.setMetricCharts(w.metricsRenderedSnapshot, w.metricsChartSpecs)
	}
	w.setStatus(status, false)
}

func (w *mainWindow) openMetrics() {
	if !w.showingMetrics {
		w.metricsFocusedTitle = ""
	}
	if w.currentPage == pageRDSInstances {
		if w.selectedRDSInstance == "" {
			return
		}
		w.metricsKind = "rds"
		w.loadMetrics(true)
		return
	}
	if w.currentPage == pageRDSClusters {
		if w.selectedRDSCluster == "" || w.rdsClusterDetail == nil {
			return
		}
		w.metricsKind = "rds-cluster"
		w.loadMetrics(true)
		return
	}
	if w.currentPage == pageElastiCache {
		if w.selectedElastiCache == "" || w.elastiCacheDetail == nil {
			return
		}
		w.metricsKind = "elasticache"
		w.loadMetrics(true)
		return
	}
	if w.currentPage == pageEC2Instances {
		if w.selectedEC2Instance == "" {
			return
		}
		w.metricsKind = "ec2"
		w.loadMetrics(true)
		return
	}
	if w.selectedCluster == "" || (w.selectedService == "" && w.selectedTask == "") {
		return
	}
	w.metricsKind = "ecs"
	w.loadMetrics(true)
}

func (w *mainWindow) loadMetrics(foreground bool) {
	if w.metricsKind == "elasticache" {
		w.loadElastiCacheMetrics(foreground)
		return
	}
	if w.metricsKind == "rds-cluster" {
		w.loadRDSClusterMetrics(foreground)
		return
	}
	if w.metricsKind == "rds" {
		w.loadRDSMetrics(foreground)
		return
	}
	if w.metricsKind == "ec2" {
		w.loadEC2Metrics(foreground)
		return
	}
	w.loadECSMetrics(foreground)
}

func (w *mainWindow) loadElastiCacheMetrics(foreground bool) {
	if w.options.ElastiCache == nil || w.elastiCacheDetail == nil {
		return
	}
	resource := *w.elastiCacheDetail
	opening := !w.showingMetrics
	ctx, generation := w.startRefreshRequest("Loading metrics for ElastiCache "+resource.ID+"…", foreground)
	go func() {
		snapshot, err := w.options.ElastiCache.Metrics(ctx, resource, w.metricsWindow())
		w.finishRequestResult(ctx, generation, err, "Metrics updated for ElastiCache "+resource.ID, opening, foreground, func() {
			if w.currentPage != pageElastiCache || w.selectedElastiCache != resource.ID {
				return
			}
			w.showingMetrics = true
			w.metricsKind = "elasticache"
			w.metricsSnapshot = nil
			w.metricsGenericSnapshot = snapshot
			w.metricsTaskID = ""
			w.renderElastiCacheMetrics(resource)
			w.detailStack.SetVisibleChildName("metrics")
		})
	}()
}

func (w *mainWindow) renderElastiCacheMetrics(resource model.ElastiCacheResource) {
	snapshot := w.metricsGenericSnapshot
	if snapshot == nil {
		return
	}
	w.metricsTitle.SetLabel("ELASTICACHE METRICS — " + strings.ToUpper(w.metricsRangeLabel()))
	w.metricsScope.SetLabel("ElastiCache " + strings.ReplaceAll(string(resource.Kind), "-", " ") + " " + resource.ID)
	w.metricsTimestamp.SetLabel(fmt.Sprintf("Updated %s • %s resolution", formatTime(snapshot.EndTime), formatMetricPeriod(snapshot.Period)))
	w.metricsScaleButton.SetVisible(false)
	w.metricsScaleLabel.SetVisible(false)
	w.metricsAlarmSection.SetVisible(false)
	w.metricsNotice.SetVisible(!metricSnapshotHasData(snapshot))
	if !metricSnapshotHasData(snapshot) {
		w.metricsNotice.SetLabel("No ElastiCache datapoints were returned for the selected period.")
	}
	specs := []metricChartSpec{
		{title: "CPU UTILIZATION", unit: "%", minZero: true, maxHint: 100, ids: []string{"cpu", "engine_cpu", "ecpu"}},
		{title: "MEMORY AND CACHE STORAGE", unit: "bytes", minZero: true, ids: []string{"memory", "storage"}},
		{title: "CONNECTIONS", unit: "count", minZero: true, ids: []string{"connections"}},
		{title: "CACHE HITS / MISSES", unit: "count", minZero: true, ids: []string{"hits", "misses"}},
		{title: "EVICTIONS", unit: "count", minZero: true, ids: []string{"evictions"}},
		{title: "NETWORK THROUGHPUT", unit: "bytes/s", minZero: true, ids: []string{"network_in", "network_out"}},
		{title: "REPLICATION LAG", unit: "seconds", minZero: true, ids: []string{"replication_lag"}},
	}
	w.setMetricCharts(snapshot, specs)
}

func (w *mainWindow) loadRDSClusterMetrics(foreground bool) {
	identifier := w.selectedRDSCluster
	if identifier == "" || w.options.RDS == nil {
		return
	}
	opening := !w.showingMetrics
	ctx, generation := w.startRefreshRequest("Loading metrics for RDS cluster "+identifier+"…", foreground)
	go func() {
		snapshot, err := w.options.RDS.ClusterMetrics(ctx, identifier, w.metricsWindow())
		w.finishRequestResult(ctx, generation, err, "Metrics updated for RDS cluster "+identifier, opening, foreground, func() {
			if w.currentPage != pageRDSClusters || w.selectedRDSCluster != identifier {
				return
			}
			w.showingMetrics = true
			w.metricsKind = "rds-cluster"
			w.metricsSnapshot = nil
			w.metricsGenericSnapshot = snapshot
			w.metricsTaskID = ""
			w.renderRDSClusterMetrics()
			w.detailStack.SetVisibleChildName("metrics")
		})
	}()
}

func (w *mainWindow) renderRDSClusterMetrics() {
	snapshot := w.metricsGenericSnapshot
	if snapshot == nil {
		return
	}
	w.metricsTitle.SetLabel("DATABASE CLUSTER METRICS — " + strings.ToUpper(w.metricsRangeLabel()))
	w.metricsScope.SetLabel("RDS cluster " + w.selectedRDSCluster + " • one standard CloudWatch series per member instance")
	w.metricsTimestamp.SetLabel(fmt.Sprintf("Updated %s • %s resolution", formatTime(snapshot.EndTime), formatMetricPeriod(snapshot.Period)))
	w.metricsScaleButton.SetVisible(false)
	w.metricsScaleLabel.SetVisible(false)
	w.metricsAlarmSection.SetVisible(false)
	hasData := metricSnapshotHasData(snapshot)
	w.metricsNotice.SetVisible(!hasData)
	if !hasData {
		w.metricsNotice.SetLabel("No standard RDS datapoints were returned for the cluster members in the selected period.")
	}
	w.setMetricCharts(snapshot, rdsClusterMetricChartSpecs(snapshot))
}

func rdsClusterMetricChartSpecs(snapshot *model.MetricSnapshot) []metricChartSpec {
	definitions := []struct {
		title   string
		unit    string
		baseID  string
		maxHint float64
	}{
		{"CPU UTILIZATION", "%", "cpu", 100},
		{"DATABASE CONNECTIONS", "count", "connections", 0},
		{"AVAILABLE MEMORY", "bytes", "free_memory", 0},
		{"FREE STORAGE", "GiB", "free_storage", 0},
		{"READ IOPS", "iops", "read_iops", 0},
		{"WRITE IOPS", "iops", "write_iops", 0},
		{"READ LATENCY", "ms", "read_latency", 0},
		{"WRITE LATENCY", "ms", "write_latency", 0},
		{"READ THROUGHPUT", "bytes/s", "read_throughput", 0},
		{"WRITE THROUGHPUT", "bytes/s", "write_throughput", 0},
		{"NETWORK RECEIVE", "bytes/s", "network_receive", 0},
		{"NETWORK TRANSMIT", "bytes/s", "network_transmit", 0},
		{"DISK QUEUE DEPTH", "count", "disk_queue", 0},
		{"BURST BALANCE", "%", "burst_balance", 100},
		{"REPLICA LAG", "seconds", "replica_lag", 0},
		{"DB LOAD", "sessions", "db_load", 0},
		{"DB LOAD RELATIVE TO VCPU", "ratio", "db_load_per_vcpu", 1},
	}
	specs := make([]metricChartSpec, 0, len(definitions))
	for _, definition := range definitions {
		ids := metricSeriesIDsWithBase(snapshot, definition.baseID)
		specs = append(specs, metricChartSpec{title: definition.title, unit: definition.unit,
			minZero: true, maxHint: definition.maxHint, ids: ids})
	}
	return specs
}

func metricSeriesIDsWithBase(snapshot *model.MetricSnapshot, baseID string) []string {
	if snapshot == nil {
		return nil
	}
	prefix := baseID + "__"
	ids := make([]string, 0)
	for _, series := range snapshot.Series {
		if strings.HasPrefix(series.ID, prefix) {
			ids = append(ids, series.ID)
		}
	}
	return ids
}

func (w *mainWindow) loadRDSMetrics(foreground bool) {
	identifier := w.selectedRDSInstance
	if identifier == "" || w.options.RDS == nil {
		return
	}
	opening := !w.showingMetrics
	ctx, generation := w.startRefreshRequest("Loading metrics for "+identifier+"…", foreground)
	go func() {
		snapshot, err := w.options.RDS.Metrics(ctx, identifier, w.metricsWindow())
		w.finishRequestResult(ctx, generation, err, "Metrics updated for "+identifier, opening, foreground, func() {
			if w.currentPage != pageRDSInstances || w.selectedRDSInstance != identifier {
				return
			}
			w.showingMetrics = true
			w.metricsKind = "rds"
			w.metricsSnapshot = nil
			w.metricsGenericSnapshot = snapshot
			w.metricsTaskID = ""
			w.renderRDSMetrics()
			w.detailStack.SetVisibleChildName("metrics")
		})
	}()
}

func (w *mainWindow) renderRDSMetrics() {
	snapshot := w.metricsGenericSnapshot
	if snapshot == nil {
		return
	}
	w.metricsTitle.SetLabel("DATABASE METRICS — " + strings.ToUpper(w.metricsRangeLabel()))
	w.metricsScope.SetLabel("RDS instance " + w.selectedRDSInstance + " • standard CloudWatch metrics")
	w.metricsTimestamp.SetLabel(fmt.Sprintf("Updated %s • %s resolution", formatTime(snapshot.EndTime), formatMetricPeriod(snapshot.Period)))
	w.metricsScaleButton.SetVisible(false)
	w.metricsScaleLabel.SetVisible(false)
	w.metricsAlarmSection.SetVisible(false)
	hasData := metricSnapshotHasData(snapshot)
	w.metricsNotice.SetVisible(!hasData)
	if !hasData {
		w.metricsNotice.SetLabel("No standard RDS datapoints were returned for the selected period.")
	}
	w.setMetricCharts(snapshot, []metricChartSpec{
		{title: "CPU UTILIZATION", unit: "%", minZero: true, maxHint: 100, ids: []string{"cpu", "cpu_max"}},
		{title: "DATABASE CONNECTIONS", unit: "count", minZero: true, ids: []string{"connections"}},
		{title: "AVAILABLE MEMORY", unit: "bytes", minZero: true, ids: []string{"free_memory"}},
		{title: "FREE STORAGE", unit: "GiB", minZero: true, ids: []string{"free_storage"}},
		{title: "READ / WRITE IOPS", unit: "iops", minZero: true, ids: []string{"read_iops", "write_iops"}},
		{title: "READ / WRITE LATENCY", unit: "ms", minZero: true, ids: []string{"read_latency", "write_latency"}},
		{title: "READ / WRITE THROUGHPUT", unit: "bytes/s", minZero: true, ids: []string{"read_throughput", "write_throughput"}},
		{title: "NETWORK THROUGHPUT", unit: "bytes/s", minZero: true, ids: []string{"network_receive", "network_transmit"}},
		{title: "DISK QUEUE DEPTH", unit: "count", minZero: true, ids: []string{"disk_queue"}},
		{title: "BURST BALANCE", unit: "%", minZero: true, maxHint: 100, ids: []string{"burst_balance"}},
		{title: "REPLICA LAG", unit: "seconds", minZero: true, ids: []string{"replica_lag"}},
		{title: "DB LOAD", unit: "sessions", minZero: true, ids: []string{"db_load", "db_load_cpu", "db_load_non_cpu"}},
		{title: "DB LOAD RELATIVE TO VCPU", unit: "ratio", minZero: true, maxHint: 1, ids: []string{"db_load_per_vcpu"}},
	})
}

func (w *mainWindow) loadECSMetrics(foreground bool) {
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
			w.metricsKind = "ecs"
			w.metricsSnapshot = metrics
			w.metricsGenericSnapshot = nil
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
	w.setMetricCharts(&model.MetricSnapshot{StartTime: m.StartTime, EndTime: m.EndTime, Period: m.Period, Series: m.Series}, []metricChartSpec{
		{title: "CPU UTILIZATION", unit: "%", minZero: true, maxHint: 100, ids: []string{"cpu_avg", "cpu_max"}},
		{title: "MEMORY UTILIZATION", unit: "%", minZero: true, maxHint: 100, ids: []string{"mem_avg", "mem_max"}},
	})
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

type metricChartSpec struct {
	title   string
	unit    string
	minZero bool
	maxHint float64
	ids     []string
}

func (w *mainWindow) setMetricCharts(snapshot *model.MetricSnapshot, specs []metricChartSpec) {
	if w.metricsChartsBox == nil || snapshot == nil {
		return
	}
	w.metricsRenderedSnapshot = snapshot
	w.metricsChartSpecs = append([]metricChartSpec(nil), specs...)
	if w.metricsFocusedTitle != "" {
		found := false
		for _, spec := range specs {
			if spec.title == w.metricsFocusedTitle {
				found = true
				break
			}
		}
		if !found {
			w.metricsFocusedTitle = ""
		}
	}
	w.metricsRestoreButton.SetVisible(w.metricsFocusedTitle != "")
	for child := w.metricsChartsBox.FirstChild(); child != nil; child = w.metricsChartsBox.FirstChild() {
		w.metricsChartsBox.Remove(child)
	}
	w.metricsCharts = make([]*metricChart, 0, len(specs))
	for _, spec := range specs {
		if w.metricsFocusedTitle != "" && spec.title != w.metricsFocusedTitle {
			continue
		}
		if !metricSpecHasData(snapshot, spec.ids) {
			continue
		}
		chart := newMetricChart(spec.title, spec.unit, spec.minZero, spec.maxHint)
		chart.SetPalette(w.currentSemanticPalette(chart.area.StyleContext()))
		chart.SetUTC(w.metricsUTCTime)
		chart.SetData(snapshot.StartTime, snapshot.EndTime, snapshot.Series, spec.ids...)
		chart.SetExpanded(w.metricsFocusedTitle != "")
		title := spec.title
		chart.SetExpandHandler(func() { w.focusMetricChart(title) })
		w.metricsCharts = append(w.metricsCharts, chart)
		w.metricsChartsBox.Append(chart.Widget())
	}
}

func (w *mainWindow) focusMetricChart(title string) {
	if title == "" || w.metricsRenderedSnapshot == nil {
		return
	}
	w.metricsFocusedTitle = title
	w.setMetricCharts(w.metricsRenderedSnapshot, w.metricsChartSpecs)
	w.setStatus("Maximized "+strings.ToLower(title)+" • Escape returns to all charts", false)
}

func (w *mainWindow) restoreMetricCharts() {
	if w.metricsFocusedTitle == "" || w.metricsRenderedSnapshot == nil {
		return
	}
	w.metricsFocusedTitle = ""
	w.setMetricCharts(w.metricsRenderedSnapshot, w.metricsChartSpecs)
	w.setStatus("Showing all metric charts", false)
}

func metricSpecHasData(snapshot *model.MetricSnapshot, ids []string) bool {
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	for _, series := range snapshot.Series {
		if _, ok := wanted[series.ID]; ok && len(series.Points) > 0 {
			return true
		}
	}
	return false
}

func (w *mainWindow) loadEC2Metrics(foreground bool) {
	instanceID := w.selectedEC2Instance
	if instanceID == "" || w.options.EC2 == nil {
		return
	}
	opening := !w.showingMetrics
	ctx, generation := w.startRefreshRequest("Loading metrics for "+instanceID+"…", foreground)
	go func() {
		snapshot, err := w.options.EC2.Metrics(ctx, instanceID, w.metricsWindow())
		w.finishRequestResult(ctx, generation, err, "Metrics updated for "+instanceID, opening, foreground, func() {
			if w.currentPage != pageEC2Instances || w.selectedEC2Instance != instanceID {
				return
			}
			w.showingMetrics = true
			w.metricsKind = "ec2"
			w.metricsSnapshot = nil
			w.metricsGenericSnapshot = snapshot
			w.metricsTaskID = ""
			w.renderEC2Metrics()
			w.detailStack.SetVisibleChildName("metrics")
		})
	}()
}

func (w *mainWindow) renderEC2Metrics() {
	snapshot := w.metricsGenericSnapshot
	if snapshot == nil {
		return
	}
	w.metricsTitle.SetLabel("INSTANCE METRICS — " + strings.ToUpper(w.metricsRangeLabel()))
	w.metricsScope.SetLabel("EC2 instance " + w.selectedEC2Instance)
	w.metricsTimestamp.SetLabel(fmt.Sprintf("Updated %s • %s resolution", formatTime(snapshot.EndTime), formatMetricPeriod(snapshot.Period)))
	w.metricsScaleButton.SetVisible(false)
	w.metricsScaleLabel.SetVisible(false)
	w.metricsAlarmSection.SetVisible(false)
	w.metricsNotice.SetVisible(!metricSnapshotHasData(snapshot))
	if !metricSnapshotHasData(snapshot) {
		w.metricsNotice.SetLabel("No instance datapoints were returned for the selected period.")
	}
	w.setMetricCharts(snapshot, []metricChartSpec{
		{title: "CPU UTILIZATION", unit: "%", minZero: true, maxHint: 100, ids: []string{"cpu_avg", "cpu_max"}},
		{title: "NETWORK TRAFFIC PER PERIOD", unit: "bytes", minZero: true, ids: []string{"network_in", "network_out"}},
		{title: "INSTANCE STORE I/O PER PERIOD", unit: "bytes", minZero: true, ids: []string{"disk_read", "disk_write"}},
		{title: "FAILED STATUS CHECKS", unit: "count", minZero: true, maxHint: 1, ids: []string{"status_failed"}},
	})
}

func metricSnapshotHasData(snapshot *model.MetricSnapshot) bool {
	if snapshot == nil {
		return false
	}
	for _, series := range snapshot.Series {
		if len(series.Points) > 0 {
			return true
		}
	}
	return false
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

func metricFraction(value float64) float64 {
	return min(1, max(0, value/100))
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
	w.metricsKind = ""
	w.metricsFocusedTitle = ""
	w.metricsRestoreButton.SetVisible(false)
	w.detailStack.SetVisibleChildName("detail")
	w.setStatus("Metrics view closed", false)
}
