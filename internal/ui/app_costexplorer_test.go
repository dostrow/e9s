package ui

import (
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

func TestTUICostSavedViewBuildsConfiguredQuery(t *testing.T) {
	view := config.CostView{Name: "team", Days: 7, Metric: "AmortizedCost", GroupBy: "LINKED_ACCOUNT", ServiceFilter: "Amazon RDS"}
	query := tuiCostQuery("saved: team", &view)
	if got := query.End.Sub(query.Start); got != 7*24*time.Hour {
		t.Fatalf("range = %v, want 7 days", got)
	}
	if query.Metric != view.Metric || query.GroupBy != view.GroupBy || query.ServiceFilter != view.ServiceFilter {
		t.Fatalf("query = %+v", query)
	}
}

func TestTUICostReportShowsCacheAndRows(t *testing.T) {
	now := time.Now()
	app := App{width: 100, height: 30, costSubview: "overview"}
	app = app.setCostReport(model.CostReport{Total: 10, Unit: "USD", Groups: []model.CostGroup{{Name: "Amazon RDS", Amount: 4, Unit: "USD"}}},
		model.CostCacheStatus{CachedAt: now, FromCache: true})
	if app.costView.SelectedID() != "Amazon RDS" {
		t.Fatalf("selected group = %q", app.costView.SelectedID())
	}
	if app.loading {
		t.Fatal("cost view remained loading")
	}
}
