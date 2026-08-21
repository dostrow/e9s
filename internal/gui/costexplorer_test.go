//go:build gui

package gui

import (
	"testing"

	"github.com/dostrow/e9s/internal/config"
)

func TestCostQueryForSavedView(t *testing.T) {
	saved := config.CostView{Name: "database", Days: 14, Metric: "AmortizedCost", GroupBy: "SERVICE", ServiceFilter: "Amazon RDS"}
	query := costQueryForView(pageCostSavedView, saved)
	if query.End.Sub(query.Start).Hours() != 14*24 {
		t.Fatalf("range = %v", query.End.Sub(query.Start))
	}
	if query.Metric != saved.Metric || query.GroupBy != saved.GroupBy || query.ServiceFilter != saved.ServiceFilter {
		t.Fatalf("query = %+v", query)
	}
}

func TestCostResourceQueryIsFourteenDays(t *testing.T) {
	query := costQueryForView(pageCostResources, config.CostView{})
	if query.End.Sub(query.Start).Hours() != 14*24 || query.GroupBy != "RESOURCE_ID" || query.ServiceFilter == "" {
		t.Fatalf("resource query = %+v", query)
	}
}
