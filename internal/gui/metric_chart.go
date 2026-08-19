//go:build gui

package gui

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pangocairo"
	"github.com/dostrow/e9s/internal/model"
)

type metricChart struct {
	area         *gtk.DrawingArea
	overlay      *gtk.Overlay
	expandButton *gtk.Button
	title        string
	unit         string
	minZero      bool
	maxHint      float64
	start        time.Time
	end          time.Time
	series       []model.MetricSeries
	hovering     bool
	hoverX       float64
	hoverY       float64
	onExpand     func()
	expanded     bool
}

func newMetricChart(title, unit string, minZero bool, maxHint float64) *metricChart {
	chart := &metricChart{title: title, unit: unit, minZero: minZero, maxHint: maxHint}
	chart.area = gtk.NewDrawingArea()
	chart.area.SetHExpand(true)
	chart.area.SetContentHeight(220)
	chart.area.AddCSSClass("metric-chart")
	chart.area.SetDrawFunc(chart.draw)
	motion := gtk.NewEventControllerMotion()
	motion.ConnectMotion(func(x, y float64) {
		chart.hovering = true
		chart.hoverX, chart.hoverY = x, y
		chart.area.SetCursorFromName("crosshair")
		chart.area.QueueDraw()
	})
	motion.ConnectLeave(func() {
		chart.hovering = false
		chart.area.SetCursorFromName("default")
		chart.area.QueueDraw()
	})
	chart.area.AddController(motion)
	chart.overlay = gtk.NewOverlay()
	chart.overlay.SetChild(chart.area)
	chart.expandButton = gtk.NewButtonFromIconName("view-fullscreen-symbolic")
	chart.expandButton.AddCSSClass("flat")
	chart.expandButton.SetHAlign(gtk.AlignEnd)
	chart.expandButton.SetVAlign(gtk.AlignStart)
	chart.expandButton.SetMarginTop(4)
	chart.expandButton.SetMarginEnd(4)
	chart.expandButton.SetTooltipText("Maximize this chart")
	chart.expandButton.ConnectClicked(func() {
		if chart.onExpand != nil {
			chart.onExpand()
		}
	})
	chart.overlay.AddOverlay(chart.expandButton)
	return chart
}

func (c *metricChart) Widget() gtk.Widgetter { return c.overlay }

func (c *metricChart) SetExpandHandler(handler func()) { c.onExpand = handler }

func (c *metricChart) SetExpanded(expanded bool) {
	c.expanded = expanded
	c.expandButton.SetVisible(!expanded)
	c.area.SetVExpand(expanded)
	if expanded {
		c.area.SetContentHeight(620)
	} else {
		c.area.SetContentHeight(c.defaultContentHeight())
	}
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
	if !c.expanded {
		c.area.SetContentHeight(c.defaultContentHeight())
	}
	c.area.QueueDraw()
}

func (c *metricChart) defaultContentHeight() int {
	visible := 0
	for _, series := range c.series {
		if len(series.Points) > 0 {
			visible++
		}
	}
	return 220 + max(0, (visible-3)/3)*20
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
	left := c.axisLeftGutter(area, minValue, maxValue)
	right, bottom := 14.0, 28.0
	legendRows := c.legendRowCount(area, left, float64(width)-right)
	top := max(48, 30+float64(legendRows*20))
	plotWidth := float64(width) - left - right
	plotHeight := float64(height) - top - bottom
	if plotWidth <= 0 || plotHeight <= 0 {
		return
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

	for index, series := range c.series {
		if len(series.Points) == 0 {
			continue
		}
		color, alpha := metricSeriesColor(index, accent, foreground)
		cr.SetSourceRGBA(float64(color.Red()), float64(color.Green()), float64(color.Blue()), alpha)
		cr.SetLineWidth(2)
		cr.SetDash(metricSeriesDash(index), 0)
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
	if c.hovering && hasData && c.hoverX >= left && c.hoverX <= left+plotWidth && c.hoverY >= top && c.hoverY <= top+plotHeight {
		c.drawHover(area, cr, left, top, plotWidth, plotHeight, minValue, maxValue, foreground, background, accent)
	}

	drawChartText(area, cr, c.title, 4, 4, foreground, 1)
	c.drawLegend(area, cr, left, 27, left+plotWidth, foreground, accent)
	if !c.start.IsZero() && !c.end.IsZero() {
		drawChartText(area, cr, c.start.Local().Format("Jan 2 15:04"), left, top+plotHeight+7, foreground, 0.7)
		endLabel := c.end.Local().Format("Jan 2 15:04")
		layout := area.CreatePangoLayout(endLabel)
		textWidth, _ := layout.PixelSize()
		drawChartText(area, cr, endLabel, left+plotWidth-float64(textWidth), top+plotHeight+7, foreground, 0.7)
	}
}

func (c *metricChart) axisLeftGutter(area *gtk.DrawingArea, minValue, maxValue float64) float64 {
	maxWidth := 0
	for i := 0; i <= 4; i++ {
		value := maxValue - (maxValue-minValue)*float64(i)/4
		layout := area.CreatePangoLayout(fmtMetricValue(value, c.unit))
		width, _ := layout.PixelSize()
		if width > maxWidth {
			maxWidth = width
		}
	}
	return max(56, float64(maxWidth+12))
}

func metricSeriesColor(index int, accent, foreground gdk.RGBA) (gdk.RGBA, float64) {
	if index%2 == 0 {
		return accent, max(0.55, 0.95-float64(index/2)*0.12)
	}
	return foreground, max(0.5, 0.9-float64(index/2)*0.12)
}

func metricSeriesDash(index int) []float64 {
	switch index % 4 {
	case 1:
		return []float64{7, 4}
	case 2:
		return []float64{2, 3}
	case 3:
		return []float64{8, 3, 2, 3}
	default:
		return nil
	}
}

func (c *metricChart) drawLegend(area *gtk.DrawingArea, cr *cairo.Context, x, y, maxX float64, foreground, accent gdk.RGBA) {
	startX := x
	visible := 0
	for index, series := range c.series {
		if len(series.Points) == 0 {
			continue
		}
		label := series.Label
		if label == "" {
			label = series.ID
		}
		latest := series.Points[len(series.Points)-1].Value
		text := fmt.Sprintf("%s  %s", label, fmtMetricValue(latest, c.unit))
		layout := area.CreatePangoLayout(text)
		textWidth, _ := layout.PixelSize()
		entryWidth := 28 + float64(textWidth) + 24
		if x > startX && x+entryWidth > maxX {
			x = startX
			y += 20
		}
		color, alpha := metricSeriesColor(index, accent, foreground)
		cr.SetSourceRGBA(float64(color.Red()), float64(color.Green()), float64(color.Blue()), alpha)
		cr.SetLineWidth(2)
		cr.SetDash(metricSeriesDash(index), 0)
		cr.MoveTo(x, y+7)
		cr.LineTo(x+22, y+7)
		cr.Stroke()
		cr.SetDash(nil, 0)
		drawChartText(area, cr, text, x+28, y, foreground, 0.82)
		x += entryWidth
		visible++
	}
	if visible == 0 {
		drawChartText(area, cr, "No data for this time range", x, y, foreground, 0.76)
	}
}

func (c *metricChart) legendRowCount(area *gtk.DrawingArea, startX, maxX float64) int {
	rows := 1
	x := startX
	visible := 0
	for _, series := range c.series {
		if len(series.Points) == 0 {
			continue
		}
		label := series.Label
		if label == "" {
			label = series.ID
		}
		latest := series.Points[len(series.Points)-1].Value
		layout := area.CreatePangoLayout(fmt.Sprintf("%s  %s", label, fmtMetricValue(latest, c.unit)))
		textWidth, _ := layout.PixelSize()
		entryWidth := 28 + float64(textWidth) + 24
		if x > startX && x+entryWidth > maxX {
			rows++
			x = startX
		}
		x += entryWidth
		visible++
	}
	if visible == 0 {
		return 1
	}
	return rows
}

func (c *metricChart) drawHover(area *gtk.DrawingArea, cr *cairo.Context, left, top, plotWidth, plotHeight, minValue, maxValue float64, foreground, background, accent gdk.RGBA) {
	fraction := min(1, max(0, (c.hoverX-left)/plotWidth))
	target := c.start.Add(time.Duration(float64(c.end.Sub(c.start)) * fraction))
	x := left + fraction*plotWidth
	cr.SetDash([]float64{3, 3}, 0)
	cr.SetLineWidth(1)
	cr.SetSourceRGBA(float64(foreground.Red()), float64(foreground.Green()), float64(foreground.Blue()), 0.55)
	cr.MoveTo(x, top)
	cr.LineTo(x, top+plotHeight)
	cr.Stroke()
	cr.SetDash(nil, 0)

	lines := []string{target.Local().Format("Jan 2 2006 15:04:05")}
	for index, series := range c.series {
		point, ok := nearestMetricPoint(series.Points, target)
		if !ok {
			continue
		}
		label := series.Label
		if label == "" {
			label = series.ID
		}
		lines = append(lines, fmt.Sprintf("%s: %s", label, fmtMetricValue(point.Value, c.unit)))
		pointX := left + timeFraction(point.Timestamp, c.start, c.end)*plotWidth
		pointY := top + (maxValue-point.Value)/(maxValue-minValue)*plotHeight
		color, alpha := metricSeriesColor(index, accent, foreground)
		cr.SetSourceRGBA(float64(color.Red()), float64(color.Green()), float64(color.Blue()), alpha)
		cr.Arc(pointX, pointY, 3.5, 0, 2*math.Pi)
		cr.Fill()
	}
	if len(lines) == 1 {
		return
	}
	text := strings.Join(lines, "\n")
	layout := area.CreatePangoLayout(text)
	textWidth, textHeight := layout.PixelSize()
	boxWidth, boxHeight := float64(textWidth)+18, float64(textHeight)+14
	boxX := x + 12
	if boxX+boxWidth > left+plotWidth-4 {
		boxX = x - boxWidth - 12
	}
	boxX = max(left+4, boxX)
	boxY := top + 8
	cr.SetSourceRGBA(float64(background.Red()), float64(background.Green()), float64(background.Blue()), 0.96)
	cr.Rectangle(boxX, boxY, boxWidth, boxHeight)
	cr.Fill()
	cr.SetSourceRGBA(float64(accent.Red()), float64(accent.Green()), float64(accent.Blue()), 0.9)
	cr.SetLineWidth(1)
	cr.Rectangle(boxX+0.5, boxY+0.5, boxWidth-1, boxHeight-1)
	cr.Stroke()
	drawChartText(area, cr, text, boxX+9, boxY+7, foreground, 1)
}

func nearestMetricPoint(points []model.MetricPoint, target time.Time) (model.MetricPoint, bool) {
	if len(points) == 0 {
		return model.MetricPoint{}, false
	}
	index := sort.Search(len(points), func(i int) bool { return !points[i].Timestamp.Before(target) })
	if index == 0 {
		return points[0], true
	}
	if index == len(points) {
		return points[len(points)-1], true
	}
	before, after := points[index-1], points[index]
	if target.Sub(before.Timestamp) <= after.Timestamp.Sub(target) {
		return before, true
	}
	return after, true
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
	case "bytes/s":
		return formatMetricBytes(value) + "/s"
	case "GiB":
		return fmt.Sprintf("%.1f GiB", value)
	case "ms":
		return fmt.Sprintf("%.1f ms", value)
	case "seconds":
		return fmt.Sprintf("%.1f s", value)
	case "count":
		return fmt.Sprintf("%.0f", value)
	case "iops":
		return fmt.Sprintf("%.1f IOPS", value)
	case "sessions":
		return fmt.Sprintf("%.2f AAS", value)
	case "ratio":
		return fmt.Sprintf("%.2fx", value)
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
