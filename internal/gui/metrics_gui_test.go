//go:build gui

package gui

import (
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

func TestMetricFractionClampsToProgressRange(t *testing.T) {
	for _, test := range []struct {
		value float64
		want  float64
	}{
		{value: -4, want: 0},
		{value: 0, want: 0},
		{value: 42.5, want: 0.425},
		{value: 100, want: 1},
		{value: 180, want: 1},
	} {
		if got := metricFraction(test.value); got != test.want {
			t.Errorf("metricFraction(%v) = %v, want %v", test.value, got, test.want)
		}
	}
}

func TestNearestMetricPoint(t *testing.T) {
	start := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	points := []model.MetricPoint{
		{Timestamp: start, Value: 1},
		{Timestamp: start.Add(time.Minute), Value: 2},
		{Timestamp: start.Add(2 * time.Minute), Value: 3},
	}
	point, ok := nearestMetricPoint(points, start.Add(40*time.Second))
	if !ok || point.Value != 2 {
		t.Fatalf("nearestMetricPoint() = %#v, %v", point, ok)
	}
	point, ok = nearestMetricPoint(points, start.Add(-time.Hour))
	if !ok || point.Value != 1 {
		t.Fatalf("nearestMetricPoint(before) = %#v, %v", point, ok)
	}
}

func TestChartTimeTicksUseReadableTwoDayIntervalsForTwoWeeks(t *testing.T) {
	start := time.Date(2026, 8, 5, 15, 19, 0, 0, time.Local)
	end := start.Add(14 * 24 * time.Hour)
	ticks := chartTimeTicks(start, end, 9)
	if len(ticks) != 8 {
		t.Fatalf("chartTimeTicks() returned %d ticks: %#v", len(ticks), ticks)
	}
	for index := 1; index < len(ticks); index++ {
		if interval := ticks[index].Sub(ticks[index-1]); interval != 2*24*time.Hour {
			t.Fatalf("tick interval %d = %s", index, interval)
		}
	}
	if got := formatChartAxisTime(start, 14*24*time.Hour); got != "Aug 5" {
		t.Fatalf("formatChartAxisTime() = %q", got)
	}
}

func TestChartTimeTicksAlwaysIncludeEndpoints(t *testing.T) {
	start := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
	end := start.Add(75 * time.Minute)
	ticks := chartTimeTicks(start, end, 5)
	if len(ticks) < 2 || !ticks[0].Equal(start) || !ticks[len(ticks)-1].Equal(end) {
		t.Fatalf("chartTimeTicks() = %#v", ticks)
	}
}
