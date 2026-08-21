package service

import (
	"context"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

type fakeCostAPI struct{ reports int }

func (f *fakeCostAPI) FetchCostReport(context.Context, model.CostQuery, bool) (model.CostReport, error) {
	f.reports++
	return model.CostReport{Total: float64(f.reports), Pages: 1}, nil
}
func (*fakeCostAPI) FetchCostForecast(context.Context, model.CostQuery) (float64, float64, float64, string, error) {
	return 10, 8, 12, "USD", nil
}
func (*fakeCostAPI) FetchCostAnomalies(context.Context, time.Time, time.Time) (model.CostAnomalyReport, error) {
	return model.CostAnomalyReport{Pages: 1}, nil
}

func TestCostExplorerCachesPerQueryUntilForced(t *testing.T) {
	api := &fakeCostAPI{}
	service := NewCostExplorer(api)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	query := model.CostQuery{Start: now.AddDate(0, 0, -7), End: now, GroupBy: "SERVICE"}
	first, status, err := service.Report(context.Background(), query, false, false, false)
	if err != nil || status.FromCache || first.Total != 1 {
		t.Fatalf("first report = %+v, %+v, %v", first, status, err)
	}
	second, status, err := service.Report(context.Background(), query, false, false, false)
	if err != nil || !status.FromCache || second.Total != 1 || api.reports != 1 {
		t.Fatalf("cached report = %+v, %+v, calls %d, %v", second, status, api.reports, err)
	}
	forced, status, err := service.Report(context.Background(), query, false, false, true)
	if err != nil || status.FromCache || forced.Total != 2 || api.reports != 2 {
		t.Fatalf("forced report = %+v, %+v, calls %d, %v", forced, status, api.reports, err)
	}
}

func TestCostExplorerCacheExpiresAfterTwentyFourHours(t *testing.T) {
	api := &fakeCostAPI{}
	service := NewCostExplorer(api)
	now := time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	query := model.CostQuery{Start: now.AddDate(0, 0, -7), End: now}
	if _, _, err := service.Report(context.Background(), query, false, false, false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(24*time.Hour + time.Second)
	report, status, err := service.Report(context.Background(), query, false, false, false)
	if err != nil || status.FromCache || report.Total != 2 || api.reports != 2 {
		t.Fatalf("expired report = %+v, %+v, calls %d, %v", report, status, api.reports, err)
	}
}
