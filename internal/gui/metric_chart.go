//go:build gui

package gui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pangocairo"
	"github.com/dostrow/e9s/internal/model"
)

type metricChart struct {
	area    *gtk.DrawingArea
	title   string
	unit    string
	minZero bool
	maxHint float64
	start   time.Time
	end     time.Time
	series  []model.MetricSeries
}

func newMetricChart(title, unit string, minZero bool, maxHint float64) *metricChart {
	chart := &metricChart{title: title, unit: unit, minZero: minZero, maxHint: maxHint}
	chart.area = gtk.NewDrawingArea()
	chart.area.SetHExpand(true)
	chart.area.SetContentHeight(220)
	chart.area.AddCSSClass("metric-chart")
	chart.area.SetDrawFunc(chart.draw)
	return chart
}

func (c *metricChart) SetData(start, end time.Time, all []model.MetricSeries, ids ...string) {
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[id] = struct{}{}
	}
	c.start, c.end = start, end
	c.series = c.series[:0]
	for _, series := range all {
		if _, ok := wanted[series.ID]; ok {
			copySeries := series
			copySeries.Points = append([]model.MetricPoint(nil), series.Points...)
			c.series = append(c.series, copySeries)
		}
	}
	c.area.QueueDraw()
}

func (c *metricChart) draw(area *gtk.DrawingArea, cr *cairo.Context, width, height int) {
	if width < 120 || height < 100 {
		return
	}
	style := area.StyleContext()
	foreground := *style.Color()
	background := lookupThemeColor(style, "theme_bg_color", "window_bg_color")
	accent := lookupThemeColor(style, "accent_color", "theme_selected_bg_color")
	if accent.Alpha() == 0 {
		accent = foreground
	}

	cr.SetSourceRGBA(float64(background.Red()), float64(background.Green()), float64(background.Blue()), float64(background.Alpha()))
	cr.Rectangle(0, 0, float64(width), float64(height))
	cr.Fill()

	left, right, top, bottom := 56.0, 14.0, 48.0, 28.0
	plotWidth := float64(width) - left - right
	plotHeight := float64(height) - top - bottom
	if plotWidth <= 0 || plotHeight <= 0 {
		return
	}

	minValue, maxValue, hasData := c.bounds()
	if !hasData {
		minValue, maxValue = 0, 1
	}
	if c.minZero && minValue > 0 {
		minValue = 0
	}
	if c.maxHint > 0 && maxValue < c.maxHint {
		maxValue = c.maxHint
	}
	if maxValue <= minValue {
		maxValue = minValue + 1
	}

	cr.SetLineWidth(1)
	cr.SetSourceRGBA(float64(foreground.Red()), float64(foreground.Green()), float64(foreground.Blue()), 0.16)
	for i := 0; i <= 4; i++ {
		y := top + plotHeight*float64(i)/4
		cr.MoveTo(left, y)
		cr.LineTo(left+plotWidth, y)
		cr.Stroke()
		value := maxValue - (maxValue-minValue)*float64(i)/4
		drawChartText(area, cr, fmtMetricValue(value, c.unit), 4, y-8, foreground, 0.72)
	}

	colors := []gdk.RGBA{accent, foreground}
	for index, series := range c.series {
		if len(series.Points) == 0 {
			continue
		}
		color := colors[index%len(colors)]
		alpha := 0.95
		if index > 1 {
			alpha = max(0.45, 0.85-float64(index)*0.1)
		}
		cr.SetSourceRGBA(float64(color.Red()), float64(color.Green()), float64(color.Blue()), alpha)
		cr.SetLineWidth(2)
		if index%2 == 1 {
			cr.SetDash([]float64{6, 4}, 0)
		} else {
			cr.SetDash(nil, 0)
		}
		started := false
		for _, point := range series.Points {
			x := left + timeFraction(point.Timestamp, c.start, c.end)*plotWidth
			y := top + (maxValue-point.Value)/(maxValue-minValue)*plotHeight
			if !started {
				cr.MoveTo(x, y)
				started = true
			} else {
				cr.LineTo(x, y)
			}
		}
		cr.Stroke()
	}
	cr.SetDash(nil, 0)

	drawChartText(area, cr, c.title, 4, 4, foreground, 1)
	legend := c.legend()
	if legend == "" {
		legend = "No data for this time range"
	}
	drawChartText(area, cr, legend, left, 24, foreground, 0.76)
	if !c.start.IsZero() && !c.end.IsZero() {
		drawChartText(area, cr, c.start.Local().Format("Jan 2 15:04"), left, top+plotHeight+7, foreground, 0.7)
		endLabel := c.end.Local().Format("Jan 2 15:04")
		layout := area.CreatePangoLayout(endLabel)
		textWidth, _ := layout.PixelSize()
		drawChartText(area, cr, endLabel, left+plotWidth-float64(textWidth), top+plotHeight+7, foreground, 0.7)
	}
}

func (c *metricChart) bounds() (float64, float64, bool) {
	minimum, maximum := math.MaxFloat64, -math.MaxFloat64
	for _, series := range c.series {
		for _, point := range series.Points {
			minimum = min(minimum, point.Value)
			maximum = max(maximum, point.Value)
		}
	}
	return minimum, maximum, minimum != math.MaxFloat64
}

func (c *metricChart) legend() string {
	parts := make([]string, 0, len(c.series))
	for _, series := range c.series {
		if len(series.Points) == 0 {
			continue
		}
		latest := series.Points[len(series.Points)-1].Value
		label := series.Label
		if label == "" {
			label = series.ID
		}
		parts = append(parts, fmt.Sprintf("%s %s", label, fmtMetricValue(latest, c.unit)))
	}
	return strings.Join(parts, "   ·   ")
}

func lookupThemeColor(style *gtk.StyleContext, names ...string) gdk.RGBA {
	for _, name := range names {
		if color, ok := style.LookupColor(name); ok {
			return *color
		}
	}
	return gdk.NewRGBA(0, 0, 0, 0)
}

func drawChartText(area *gtk.DrawingArea, cr *cairo.Context, text string, x, y float64, color gdk.RGBA, alpha float64) {
	cr.SetSourceRGBA(float64(color.Red()), float64(color.Green()), float64(color.Blue()), float64(color.Alpha())*alpha)
	layout := area.CreatePangoLayout(text)
	cr.MoveTo(x, y)
	pangocairo.ShowLayout(cr, layout)
}

func timeFraction(value, start, end time.Time) float64 {
	if start.IsZero() || end.IsZero() || !end.After(start) {
		return 0
	}
	fraction := float64(value.Sub(start)) / float64(end.Sub(start))
	return min(1, max(0, fraction))
}

func fmtMetricValue(value float64, unit string) string {
	switch unit {
	case "%":
		return fmt.Sprintf("%.1f%%", value)
	case "bytes":
		return formatMetricBytes(value)
	case "ms":
		return fmt.Sprintf("%.1f ms", value)
	case "count":
		return fmt.Sprintf("%.0f", value)
	default:
		if math.Abs(value) >= 1000 {
			return fmt.Sprintf("%.1fk", value/1000)
		}
		return fmt.Sprintf("%.1f", value)
	}
}

func formatMetricBytes(value float64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
	)
	switch {
	case math.Abs(value) >= gib:
		return fmt.Sprintf("%.1f GiB", value/gib)
	case math.Abs(value) >= mib:
		return fmt.Sprintf("%.1f MiB", value/mib)
	case math.Abs(value) >= kib:
		return fmt.Sprintf("%.1f KiB", value/kib)
	default:
		return fmt.Sprintf("%.0f B", value)
	}
}
