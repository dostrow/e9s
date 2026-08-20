//go:build gui

package gui

import (
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

func TestPrepareCostChartDataKeepsTopGroupsAndCombinesOthers(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	report := model.CostReport{
		Query: model.CostQuery{Start: start, End: start.AddDate(0, 0, 2)},
		Unit:  "USD",
		Groups: []model.CostGroup{
			{Name: "S3", Amount: 30, Points: []model.CostPoint{{Start: start, Amount: 10}, {Start: start.AddDate(0, 0, 1), Amount: 20}}},
			{Name: "RDS", Amount: 20, Points: []model.CostPoint{{Start: start, Amount: 8}, {Start: start.AddDate(0, 0, 1), Amount: 12}}},
			{Name: "EC2", Amount: 10, Points: []model.CostPoint{{Start: start, Amount: 4}, {Start: start.AddDate(0, 0, 1), Amount: 6}}},
		},
	}
	data := prepareCostChartData(report, 2)
	if len(data.series) != 3 || data.series[0].name != "S3" || data.series[1].name != "RDS" || data.series[2].name != "Others" {
		t.Fatalf("series = %+v", data.series)
	}
	if data.series[2].total != 10 || len(data.buckets) != 2 {
		t.Fatalf("combined chart data = %+v", data)
	}
	if got := data.buckets[1].values; len(got) != 3 || got[0] != 20 || got[1] != 12 || got[2] != 6 {
		t.Fatalf("second bucket = %v", got)
	}
}

func TestPrepareCostChartDataAggregatesLongRanges(t *testing.T) {
	start := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	report := model.CostReport{
		Query: model.CostQuery{Start: start, End: start.AddDate(0, 6, 0)},
		Groups: []model.CostGroup{{Name: "S3", Amount: 7, Points: []model.CostPoint{
			{Start: start, Amount: 3},
			{Start: start.AddDate(0, 0, 10), Amount: 4},
		}}},
	}
	data := prepareCostChartData(report, 9)
	if data.granularity != "monthly" || len(data.buckets) != 1 || data.buckets[0].values[0] != 7 {
		t.Fatalf("monthly chart data = %+v", data)
	}
}

func TestEvenlySpacedIndicesIncludesEndpoints(t *testing.T) {
	indices := evenlySpacedIndices(30, 7)
	if len(indices) != 7 || indices[0] != 0 || indices[len(indices)-1] != 29 {
		t.Fatalf("indices = %v", indices)
	}
}

func TestCompactCostSeriesNameBoundsLongResourceIDs(t *testing.T) {
	name := "arn:aws:ec2:us-east-2:123456789012:instance/i-0123456789abcdef0"
	got := compactCostSeriesName(name)
	if len([]rune(got)) != 42 || got[:3] != "arn" {
		t.Fatalf("compact name = %q", got)
	}
}
