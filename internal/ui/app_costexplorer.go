package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/views"
)

func (a App) openCostExplorer(subview string, saved *config.CostView) (App, tea.Cmd) {
	if subview == "" {
		subview = "overview"
	}
	a.mode, a.state, a.costSubview = modeCostExplorer, viewCostExplorer, subview
	a.costResources, a.costForecast = subview == "resources", subview == "overview"
	a.costQuery = tuiCostQuery(subview, saved)
	a.costView = views.NewEC2ResourceList("Cost Explorer — loading", []string{"GROUP", "COST", "SHARE"}).SetSize(a.width-3, a.height-6)
	a.loading = true
	return a, a.loadCostExplorer(false)
}

func tuiCostQuery(subview string, saved *config.CostView) model.CostQuery {
	now := time.Now().UTC()
	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	days := 30
	query := model.CostQuery{End: end, Metric: "UnblendedCost", GroupBy: "SERVICE"}
	if subview == "breakdown" {
		query.GroupBy = "USAGE_TYPE"
	}
	if subview == "resources" {
		days, query.GroupBy, query.ServiceFilter = 14, "RESOURCE_ID", "Amazon Elastic Compute Cloud - Compute"
	}
	if saved != nil {
		if saved.Days > 0 {
			days = saved.Days
		}
		if saved.Metric != "" {
			query.Metric = saved.Metric
		}
		if saved.GroupBy != "" {
			query.GroupBy = saved.GroupBy
		}
		query.ServiceFilter, query.BillingView = saved.ServiceFilter, saved.BillingView
	}
	query.Start = end.AddDate(0, 0, -days)
	return query
}

func (a App) loadCostExplorer(force bool) tea.Cmd {
	ctx, ce, query := a.ctx, a.costExplorer, a.costQuery
	if a.costSubview == "anomalies" {
		return func() tea.Msg {
			report, status, err := ce.Anomalies(ctx, query.End.AddDate(0, 0, -90), query.End, force)
			if err != nil {
				return errMsg{err}
			}
			return costAnomaliesLoadedMsg{report: report, status: status}
		}
	}
	resources, forecast := a.costResources, a.costForecast
	return func() tea.Msg {
		report, status, err := ce.Report(ctx, query, resources, forecast, force)
		if err != nil {
			return errMsg{err}
		}
		return costReportLoadedMsg{report: report, status: status}
	}
}

func (a App) setCostReport(report model.CostReport, status model.CostCacheStatus) App {
	a.costReport, a.costCacheStatus = report, status
	title := fmt.Sprintf("Cost Explorer / %s — total %s — %s", a.costSubview, tuiCost(report.Total, report.Unit), tuiCostCacheLabel(status))
	a.costView = views.NewEC2ResourceList(title, []string{"GROUP", "COST", "SHARE"}).SetSize(a.width-3, a.height-6)
	rows := make([]views.EC2ResourceRow, len(report.Groups))
	for i, group := range report.Groups {
		share := 0.0
		if report.Total != 0 {
			share = group.Amount / report.Total * 100
		}
		rows[i] = views.EC2ResourceRow{ID: group.Name, Search: group.Name, Cells: []string{group.Name, tuiCost(group.Amount, group.Unit), fmt.Sprintf("%.1f%%", share)}}
	}
	a.costView = a.costView.SetRows(rows)
	a.loading, a.lastRefresh = false, time.Now()
	return a
}

func (a App) setCostAnomalies(report model.CostAnomalyReport, status model.CostCacheStatus) App {
	a.costAnomalyReport, a.costCacheStatus = report, status
	title := fmt.Sprintf("Cost Explorer / anomalies — %s", tuiCostCacheLabel(status))
	a.costView = views.NewEC2ResourceList(title, []string{"SERVICE", "IMPACT", "ACTUAL", "EXPECTED", "START", "END"}).SetSize(a.width-3, a.height-6)
	rows := make([]views.EC2ResourceRow, len(report.Anomalies))
	for i, anomaly := range report.Anomalies {
		rows[i] = views.EC2ResourceRow{ID: anomaly.ID, Search: anomaly.Service + " " + anomaly.Region, Cells: []string{
			anomaly.Service, fmt.Sprintf("$%.2f", anomaly.Impact), fmt.Sprintf("$%.2f", anomaly.ActualSpend), fmt.Sprintf("$%.2f", anomaly.ExpectedSpend),
			anomaly.Start.Format("2006-01-02"), anomaly.End.Format("2006-01-02"),
		}}
	}
	a.costView = a.costView.SetRows(rows)
	a.loading, a.lastRefresh = false, time.Now()
	return a
}

func (a App) promptCostView() (App, tea.Cmd) {
	items := []string{"Overview", "Breakdown", "Anomalies", "Resources (14 days)"}
	for _, saved := range a.cfg.CostViews {
		items = append(items, "Saved: "+saved.Name)
	}
	a.picker = NewPicker(PickerCostView, "Select Cost Explorer view", items)
	return a, nil
}

func (a App) selectCostView(index int) (App, tea.Cmd) {
	switch index {
	case 0:
		return a.openCostExplorer("overview", nil)
	case 1:
		return a.openCostExplorer("breakdown", nil)
	case 2:
		return a.openCostExplorer("anomalies", nil)
	case 3:
		return a.openCostExplorer("resources", nil)
	default:
		savedIndex := index - 4
		if savedIndex >= 0 && savedIndex < len(a.cfg.CostViews) {
			return a.openCostExplorer("saved: "+a.cfg.CostViews[savedIndex].Name, &a.cfg.CostViews[savedIndex])
		}
	}
	return a, nil
}

func (a App) confirmCostRefresh() (App, tea.Cmd) {
	minimum := 1
	if a.costSubview == "overview" {
		minimum = 2
	}
	a.confirm = NewConfirm(ConfirmCostRefresh, fmt.Sprintf("Bypass the 24-hour in-memory cache? Cost Explorer costs $0.01 per paginated request. This view needs at least %d request(s) ($%.2f) and may cost more if results paginate.", minimum, float64(minimum)*0.01))
	return a, nil
}

func tuiCost(amount float64, unit string) string {
	if unit == "" || unit == "USD" {
		return fmt.Sprintf("$%.2f", amount)
	}
	return fmt.Sprintf("%.2f %s", amount, unit)
}

func tuiCostCacheLabel(status model.CostCacheStatus) string {
	if status.FromCache {
		return "cached " + status.CachedAt.Local().Format("Jan 02 15:04")
	}
	return fmt.Sprintf("%d paid API page(s), data through %s", status.Pages, status.DataThrough.Local().Format("Jan 02"))
}
