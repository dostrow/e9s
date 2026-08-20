//go:build gui

package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/dostrow/e9s/internal/model"
)

func TestSemanticClassForValue(t *testing.T) {
	tests := map[string]string{
		"available":         "semantic-success",
		"INSUFFICIENT DATA": "semantic-warning",
		"critical":          "semantic-error",
		"writer":            "semantic-info",
		"disabled":          "semantic-muted",
		"mds-prod":          "",
	}
	for value, want := range tests {
		if got := semanticClassForValue(value); got != want {
			t.Errorf("semanticClassForValue(%q) = %q, want %q", value, got, want)
		}
	}
}

func TestDetailHeadingDetection(t *testing.T) {
	for _, heading := range []string{"RDS INSTANCE", "SCAN SEVERITY COUNTS", "DESCRIPTION"} {
		if !isDetailHeading(heading) {
			t.Errorf("isDetailHeading(%q) = false", heading)
		}
	}
	for _, body := range []string{"Identifier  mdds-prod", "Description", ""} {
		if isDetailHeading(body) {
			t.Errorf("isDetailHeading(%q) = true", body)
		}
	}
}

func TestSemanticStyleUsesDerivedPalette(t *testing.T) {
	palette := semanticPalette{
		foreground: gdk.NewRGBA(0.8, 0.7, 0.6, 1),
		surface:    gdk.NewRGBA(0.12, 0.12, 0.12, 1),
		accent:     gdk.NewRGBA(0.4, 0.5, 0.6, 1),
		success:    gdk.NewRGBA(0.2, 0.7, 0.3, 1),
		warning:    gdk.NewRGBA(0.9, 0.7, 0.2, 1),
		error:      gdk.NewRGBA(0.9, 0.2, 0.2, 1),
		info:       gdk.NewRGBA(0.2, 0.6, 0.9, 1),
		muted:      gdk.NewRGBA(0.5, 0.5, 0.5, 1),
	}
	css := semanticStyleCSS(palette)
	for _, selector := range []string{".semantic-success", ".semantic-warning", ".semantic-error"} {
		if !strings.Contains(css, selector) {
			t.Errorf("semantic CSS is missing %s", selector)
		}
	}
	for _, selector := range []string{".mode-sidebar", ".toolbar", ".resource-pane", ".breadcrumb", ".app-title"} {
		if strings.Contains(css, selector) {
			t.Errorf("semantic CSS should leave theme-owned chrome alone: %s", selector)
		}
	}
	if !strings.Contains(css, palette.success.String()) || !strings.Contains(css, palette.error.String()) {
		t.Fatal("semantic CSS did not use the supplied palette")
	}
	if !strings.Contains(css, ".e9s-table row:not(:selected) .table-cell") || !strings.Contains(css, palette.foreground.String()) {
		t.Fatal("semantic CSS did not apply the theme foreground to table content")
	}
	if !strings.Contains(css, ".e9s-table > header > button") || !strings.Contains(css, palette.accent.String()) {
		t.Fatal("semantic CSS did not apply the theme accent to table headers")
	}
}

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
	if got := formatChartAxisTime(start, 14*24*time.Hour, false); got != "Aug 5" {
		t.Fatalf("formatChartAxisTime() = %q", got)
	}
}

func TestChartTimestampFormattingCanUseUTC(t *testing.T) {
	location := time.FixedZone("test-local", -5*60*60)
	timestamp := time.Date(2026, 8, 19, 20, 30, 0, 0, time.UTC).In(location)
	if got := formatChartAxisTime(timestamp, time.Hour, true); got != "20:30" {
		t.Fatalf("UTC axis label = %q", got)
	}
	if got := chartDisplayTime(timestamp, true).Location(); got != time.UTC {
		t.Fatalf("UTC display location = %v", got)
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
