//go:build gui

package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

func (w *mainWindow) openCostExplorerView(page string, saved config.CostView) {
	if w.costRequestPending {
		return
	}
	w.resetWorkspaceForBrowserChange()
	w.costGroupTable.clear()
	w.costAnomalyTable.clear()
	w.filteredCostGroups = nil
	w.filteredCostAnomalies = nil
	w.currentPage = page
	w.costCurrentSavedView = saved.Name
	w.costCurrentResources = page == pageCostResources
	w.costCurrentForecast = page == pageCostOverview
	w.costQuery = costQueryForView(page, saved)
	w.search.SetText("")
	w.search.SetSensitive(true)
	w.search.SetPlaceholderText("Filter cost results…")
	w.backButton.SetSensitive(false)
	w.resourceStack.SetVisibleChildName(costStackPage(page))
	w.setBreadcrumb(costBreadcrumb(page, saved.Name))
	if page == pageCostAnomalies {
		w.setDetail("Loading Cost Explorer data…", detailCost)
	} else {
		w.setCostSummary("Loading Cost Explorer data…")
		w.costChart.SetReport(model.CostReport{Query: w.costQuery})
		w.costChart.SetExpanded(false)
		w.costSummaryScroll.SetVisible(true)
		w.detailStack.SetVisibleChildName("cost")
	}
	w.updateActionSensitivity()
	w.loadCurrentCostView(false)
}

func costQueryForView(page string, saved config.CostView) model.CostQuery {
	now := time.Now().UTC()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	days := 30
	query := model.CostQuery{End: end, Metric: "UnblendedCost", GroupBy: "SERVICE"}
	if page == pageCostBreakdown {
		query.GroupBy = "USAGE_TYPE"
	}
	if page == pageCostResources {
		days = 14
		query.GroupBy = "RESOURCE_ID"
		query.ServiceFilter = "Amazon Elastic Compute Cloud - Compute"
	}
	if page == pageCostSavedView {
		if saved.Days > 0 {
			days = saved.Days
		}
		if saved.Metric != "" {
			query.Metric = saved.Metric
		}
		if saved.GroupBy != "" {
			query.GroupBy = saved.GroupBy
		}
		query.ServiceFilter = saved.ServiceFilter
		query.BillingView = saved.BillingView
	}
	query.Start = end.AddDate(0, 0, -days)
	return query
}

func costStackPage(page string) string {
	if page == pageCostAnomalies {
		return pageCostAnomalies
	}
	return pageCostOverview
}

func costBreadcrumb(page, savedName string) string {
	switch page {
	case pageCostOverview:
		return "Cost Explorer / Overview"
	case pageCostBreakdown:
		return "Cost Explorer / Breakdown"
	case pageCostAnomalies:
		return "Cost Explorer / Anomalies"
	case pageCostResources:
		return "Cost Explorer / Resources"
	default:
		return "Cost Explorer / Saved Views / " + savedName
	}
}

func (w *mainWindow) loadCurrentCostView(force bool) {
	if w.options.CostExplorer == nil || w.costRequestPending {
		if w.options.CostExplorer == nil {
			w.setDetail("Cost Explorer is unavailable because no service was configured.", detailError)
			w.setStatus("Cost Explorer service unavailable", true)
		}
		return
	}
	w.costRequestPending = true
	w.updateActionSensitivity()
	page, query := w.currentPage, w.costQuery
	ctx, generation := w.startRequest("Loading Cost Explorer data…")
	w.costRequestGeneration = generation
	go func() {
		if page == pageCostAnomalies {
			report, status, err := w.options.CostExplorer.Anomalies(ctx, query.End.AddDate(0, 0, -90), query.End, force)
			w.finishRequest(ctx, generation, err, func() {
				w.costAnomalyReport, w.costCacheStatus = report, status
				w.showCostAnomalies()
			})
			w.finishCostRequest(generation)
			return
		}
		report, status, err := w.options.CostExplorer.Report(ctx, query, w.costCurrentResources, w.costCurrentForecast, force)
		if err != nil {
			glib.IdleAdd(func() {
				if ctx.Err() == nil && generation == w.generation {
					w.setCostSummary("ERROR\n\n" + err.Error())
					w.detailStack.SetVisibleChildName("cost")
				}
			})
		}
		w.finishRequestResult(ctx, generation, err, "", false, true, func() {
			w.costReport, w.costCacheStatus = report, status
			w.showCostReport()
		})
		w.finishCostRequest(generation)
	}()
}

func (w *mainWindow) finishCostRequest(generation uint64) {
	glib.IdleAdd(func() {
		if generation == w.costRequestGeneration {
			w.costRequestPending = false
			w.updateActionSensitivity()
		}
	})
}

func (w *mainWindow) showCostReport() {
	w.applyCostFilter()
	w.costChart.SetReport(w.costReport)
	w.setCostSummary(formatCostReportSummary(w.costReport, w.costCacheStatus, w.costCurrentResources))
	w.detailStack.SetVisibleChildName("cost")
	w.setStatus(costLoadStatus(w.costCacheStatus), false)
}

func (w *mainWindow) applyCostFilter() {
	needle := strings.ToLower(strings.TrimSpace(w.search.Text()))
	w.filteredCostGroups = w.filteredCostGroups[:0]
	for _, group := range w.costReport.Groups {
		if needle == "" || strings.Contains(strings.ToLower(group.Name), needle) {
			w.filteredCostGroups = append(w.filteredCostGroups, group)
		}
	}
	w.filteredCostAnomalies = w.filteredCostAnomalies[:0]
	for _, anomaly := range w.costAnomalyReport.Anomalies {
		if needle == "" || strings.Contains(strings.ToLower(anomaly.Service+" "+anomaly.Region+" "+anomaly.Account+" "+anomaly.UsageType), needle) {
			w.filteredCostAnomalies = append(w.filteredCostAnomalies, anomaly)
		}
	}
	if w.currentPage == pageCostAnomalies {
		rows := make([]string, len(w.filteredCostAnomalies))
		for i, anomaly := range w.filteredCostAnomalies {
			rows[i] = fmt.Sprintf("%s\t$%.2f\t$%.2f\t$%.2f\t%s\t%s", valueOrDash(anomaly.Service), anomaly.Impact,
				anomaly.ActualSpend, anomaly.ExpectedSpend, anomaly.Start.Format("2006-01-02"), anomaly.End.Format("2006-01-02"))
		}
		w.costAnomalyTable.replace(rows)
		return
	}
	rows := make([]string, len(w.filteredCostGroups))
	for i, group := range w.filteredCostGroups {
		share := 0.0
		if w.costReport.Total != 0 {
			share = group.Amount / w.costReport.Total * 100
		}
		rows[i] = fmt.Sprintf("%s\t%s\t%.1f%%", valueOrDash(group.Name), formatCost(group.Amount, group.Unit), share)
	}
	w.costGroupTable.replace(rows)
}

func (w *mainWindow) showCostAnomalies() {
	w.applyCostFilter()
	w.setDetail(fmt.Sprintf("COST ANOMALIES\n\n%d anomalies in the prior 90 days.\n\n%s", len(w.costAnomalyReport.Anomalies), formatCostCache(w.costCacheStatus)), detailCost)
	w.setStatus(costLoadStatus(w.costCacheStatus), false)
}

func (w *mainWindow) selectCostGroupRow() {
	if !isCostReportPage(w.currentPage) {
		return
	}
	position := w.costGroupTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredCostGroups) {
		w.setCostSummary(formatCostReportSummary(w.costReport, w.costCacheStatus, w.costCurrentResources))
		return
	}
	group := w.filteredCostGroups[position]
	w.setCostSummary(formatCostGroup(group, w.costReport.Total))
}

func isCostReportPage(page string) bool {
	return page == pageCostOverview || page == pageCostBreakdown || page == pageCostResources || page == pageCostSavedView
}

func (w *mainWindow) openCostGroupAt(position uint) { w.costGroupTable.selection.SetSelected(position) }

func (w *mainWindow) selectCostAnomalyRow() {
	if w.currentPage != pageCostAnomalies {
		return
	}
	position := w.costAnomalyTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredCostAnomalies) {
		return
	}
	a := w.filteredCostAnomalies[position]
	w.setDetail(fmt.Sprintf("COST ANOMALY\n\nService       %s\nImpact        $%.2f (%.1f%%)\nMaximum       $%.2f\nActual spend  $%.2f\nExpected      $%.2f\nPeriod        %s — %s\nRegion        %s\nAccount       %s\nUsage type    %s\nFeedback      %s",
		valueOrDash(a.Service), a.Impact, a.ImpactPercent, a.MaximumImpact, a.ActualSpend, a.ExpectedSpend,
		a.Start.Format("2006-01-02"), a.End.Format("2006-01-02"), valueOrDash(a.Region), valueOrDash(a.Account), valueOrDash(a.UsageType), valueOrDash(a.Feedback)), detailCostAnomaly)
}

func (w *mainWindow) openCostAnomalyAt(position uint) {
	w.costAnomalyTable.selection.SetSelected(position)
}

func (w *mainWindow) confirmForceCostRefresh() {
	if w.costRequestPending {
		return
	}
	minimum := 1
	if w.currentPage == pageCostOverview {
		minimum = 2
	}
	dialog := gtk.NewMessageDialog(&w.window.Window, gtk.DialogModal, gtk.MessageWarning, gtk.ButtonsNone)
	dialog.SetTitle("Force paid Cost Explorer refresh")
	dialog.SetMarkup(fmt.Sprintf("Bypass the 24-hour cache and request fresh Cost Explorer data?\n\nAWS charges <b>$0.01 per paginated request</b>. This view needs at least <b>%d request%s ($%.2f)</b> and may cost more if results paginate.\n\nCached data remains available until e9s exits.",
		minimum, map[bool]string{true: "", false: "s"}[minimum == 1], float64(minimum)*0.01))
	dialog.AddButton("Cancel", int(gtk.ResponseCancel))
	dialog.AddButton("Force refresh", int(gtk.ResponseOK))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == int(gtk.ResponseOK) {
			w.loadCurrentCostView(true)
		}
	})
	dialog.Present()
}

func formatCostReportSummary(report model.CostReport, status model.CostCacheStatus, resources bool) string {
	var b strings.Builder
	b.WriteString("COST EXPLORER\n\n")
	fmt.Fprintf(&b, "Range         %s — %s\nMetric        %s\nGrouped by    %s\nTotal         %s\n", report.Query.Start.Format("2006-01-02"), report.Query.End.Format("2006-01-02"), report.Query.Metric, report.Query.GroupBy, formatCost(report.Total, report.Unit))
	if report.ForecastUnit != "" {
		fmt.Fprintf(&b, "Forecast      %s (%s — %s, 80%% interval)\n", formatCost(report.Forecast, report.ForecastUnit), formatCost(report.ForecastLower, report.ForecastUnit), formatCost(report.ForecastUpper, report.ForecastUnit))
	}
	if report.Query.ServiceFilter != "" {
		fmt.Fprintf(&b, "Service       %s\n", report.Query.ServiceFilter)
	}
	b.WriteString("\n" + formatCostCache(status))
	if resources {
		b.WriteString("\n\nResource-level data is opt-in, limited to the prior 14 days, and can lag behind aggregate costs.")
	}
	return b.String()
}

func formatCostGroup(group model.CostGroup, total float64) string {
	share := 0.0
	if total != 0 {
		share = group.Amount / total * 100
	}
	return fmt.Sprintf("COST GROUP\n\nName    %s\nCost    %s\nShare   %.1f%%\nDays    %d", group.Name, formatCost(group.Amount, group.Unit), share, len(group.Points))
}

func formatCost(amount float64, unit string) string {
	if unit == "" || strings.EqualFold(unit, "USD") {
		return fmt.Sprintf("$%.2f", amount)
	}
	return fmt.Sprintf("%.2f %s", amount, unit)
}

func formatCostCache(status model.CostCacheStatus) string {
	source := "Freshly requested"
	if status.FromCache {
		source = "24-hour in-memory cache"
	}
	return fmt.Sprintf("Source        %s\nCached at     %s\nData through  %s\nAPI pages     %d", source,
		status.CachedAt.Local().Format("2006-01-02 15:04:05 MST"), status.DataThrough.Local().Format("2006-01-02 15:04:05 MST"), status.Pages)
}

func costLoadStatus(status model.CostCacheStatus) string {
	if status.FromCache {
		return "Loaded cached Cost Explorer data from " + status.CachedAt.Local().Format("15:04:05")
	}
	return fmt.Sprintf("Loaded Cost Explorer data • %d paid API page%s", status.Pages, map[bool]string{true: "", false: "s"}[status.Pages == 1])
}
