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
	"github.com/dostrow/e9s/internal/model"
)

type costChartMode string

const (
	costChartStacked   costChartMode = "stacked"
	costChartGrouped   costChartMode = "grouped"
	costChartLines     costChartMode = "lines"
	costChartTopGroups               = 9
)

type costChartSeries struct {
	name  string
	total float64
}

type costChartBucket struct {
	start     time.Time
	end       time.Time
	values    []float64
	estimated bool
}

type costChartData struct {
	series      []costChartSeries
	buckets     []costChartBucket
	unit        string
	granularity string
}

type costChart struct {
	area         *gtk.DrawingArea
	overlay      *gtk.Overlay
	expandButton *gtk.Button
	mode         costChartMode
	data         costChartData
	hovering     bool
	hoverX       float64
	hoverY       float64
	expanded     bool
	onExpand     func()
	palette      semanticPalette
	paletteSet   bool
}

func newCostChart() *costChart {
	chart := &costChart{mode: costChartStacked}
	chart.area = gtk.NewDrawingArea()
	chart.area.SetHExpand(true)
	chart.area.SetContentHeight(380)
	chart.area.AddCSSClass("cost-chart")
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

func (c *costChart) Widget() gtk.Widgetter { return c.overlay }

func (c *costChart) SetPalette(palette semanticPalette) {
	c.palette, c.paletteSet = palette, true
	c.area.QueueDraw()
}

func (c *costChart) SetMode(mode costChartMode) {
	if mode != costChartStacked && mode != costChartGrouped && mode != costChartLines {
		mode = costChartStacked
	}
	c.mode = mode
	c.area.QueueDraw()
}

func (c *costChart) SetReport(report model.CostReport) {
	c.data = prepareCostChartData(report, costChartTopGroups)
	c.area.QueueDraw()
}

func (c *costChart) SetExpandHandler(handler func()) { c.onExpand = handler }

func (c *costChart) SetExpanded(expanded bool) {
	c.expanded = expanded
	c.overlay.SetVExpand(expanded)
	c.area.SetVExpand(expanded)
	if expanded {
		c.area.SetContentHeight(620)
		c.expandButton.SetIconName("view-restore-symbolic")
		c.expandButton.SetTooltipText("Restore chart and summary")
	} else {
		c.area.SetContentHeight(380)
		c.expandButton.SetIconName("view-fullscreen-symbolic")
		c.expandButton.SetTooltipText("Maximize this chart")
	}
	c.area.QueueDraw()
}

func prepareCostChartData(report model.CostReport, topGroups int) costChartData {
	data := costChartData{unit: report.Unit}
	groups := append([]model.CostGroup(nil), report.Groups...)
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].Amount > groups[j].Amount })
	if topGroups < 1 {
		topGroups = 1
	}
	named := min(topGroups, len(groups))
	for _, group := range groups[:named] {
		data.series = append(data.series, costChartSeries{name: compactCostSeriesName(valueOrDash(group.Name)), total: group.Amount})
		if data.unit == "" {
			data.unit = group.Unit
		}
	}
	if len(groups) > named {
		other := costChartSeries{name: "Others"}
		for _, group := range groups[named:] {
			other.total += group.Amount
		}
		data.series = append(data.series, other)
	}
	span := report.Query.End.Sub(report.Query.Start)
	data.granularity = costChartGranularity(span)
	buckets := make(map[time.Time]*costChartBucket)
	ensureBucket := func(start time.Time) *costChartBucket {
		start = costChartBucketStart(start, data.granularity)
		bucket := buckets[start]
		if bucket == nil {
			bucket = &costChartBucket{start: start, end: costChartBucketEnd(start, data.granularity), values: make([]float64, len(data.series))}
			buckets[start] = bucket
		}
		return bucket
	}
	for groupIndex, group := range groups {
		seriesIndex := groupIndex
		if groupIndex >= named {
			seriesIndex = named
		}
		for _, point := range group.Points {
			bucket := ensureBucket(point.Start)
			bucket.values[seriesIndex] += point.Amount
			bucket.estimated = bucket.estimated || point.Estimated
		}
	}
	for _, bucket := range buckets {
		data.buckets = append(data.buckets, *bucket)
	}
	sort.Slice(data.buckets, func(i, j int) bool { return data.buckets[i].start.Before(data.buckets[j].start) })
	return data
}

func costChartGranularity(span time.Duration) string {
	switch {
	case span > 120*24*time.Hour:
		return "monthly"
	case span > 45*24*time.Hour:
		return "weekly"
	default:
		return "daily"
	}
}

func costChartBucketStart(value time.Time, granularity string) time.Time {
	value = value.UTC()
	switch granularity {
	case "monthly":
		return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, time.UTC)
	case "weekly":
		day := (int(value.Weekday()) + 6) % 7
		return time.Date(value.Year(), value.Month(), value.Day()-day, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
	}
}

func costChartBucketEnd(start time.Time, granularity string) time.Time {
	switch granularity {
	case "monthly":
		return start.AddDate(0, 1, 0)
	case "weekly":
		return start.AddDate(0, 0, 7)
	default:
		return start.AddDate(0, 0, 1)
	}
}

func (c *costChart) draw(area *gtk.DrawingArea, cr *cairo.Context, width, height int) {
	if width < 180 || height < 140 {
		return
	}
	palette := c.palette
	if !c.paletteSet {
		palette = semanticPaletteFromStyle(area.StyleContext())
	}
	cr.SetSourceRGBA(float64(palette.surface.Red()), float64(palette.surface.Green()), float64(palette.surface.Blue()), float64(palette.surface.Alpha()))
	cr.Rectangle(0, 0, float64(width), float64(height))
	cr.Fill()

	colors := costChartColors(palette, len(c.data.series))
	left := c.axisLeftGutter(area)
	right, bottom := 14.0, 34.0
	legendRows := c.legendRowCount(area, left, float64(width)-right)
	top := max(64, 38+float64(legendRows*21))
	plotWidth := float64(width) - left - right
	plotHeight := float64(height) - top - bottom
	if plotWidth <= 0 || plotHeight <= 0 {
		return
	}
	minimum, maximum, hasData := c.bounds()
	if !hasData {
		minimum, maximum = 0, 1
	}
	if minimum >= 0 {
		minimum = 0
	}
	if maximum <= 0 {
		maximum = 0
	}
	if maximum <= minimum {
		maximum = minimum + 1
	}
	padding := (maximum - minimum) * 0.05
	if maximum > 0 {
		maximum += padding
	}
	if minimum < 0 {
		minimum -= padding
	}

	foreground := palette.foreground
	cr.SetLineWidth(1)
	for i := 0; i <= 4; i++ {
		value := maximum - (maximum-minimum)*float64(i)/4
		y := costValueY(value, minimum, maximum, top, plotHeight)
		cr.SetSourceRGBA(float64(foreground.Red()), float64(foreground.Green()), float64(foreground.Blue()), 0.16)
		cr.MoveTo(left, y)
		cr.LineTo(left+plotWidth, y)
		cr.Stroke()
		drawChartText(area, cr, formatCostCompact(value, c.data.unit), 4, y-8, foreground, 0.72)
	}

	if hasData {
		switch c.mode {
		case costChartGrouped:
			c.drawGroupedBars(cr, left, top, plotWidth, plotHeight, minimum, maximum, colors)
		case costChartLines:
			c.drawLines(cr, left, top, plotWidth, plotHeight, minimum, maximum, colors)
		default:
			c.drawStackedBars(cr, left, top, plotWidth, plotHeight, minimum, maximum, colors, foreground)
		}
	}
	c.drawXAxis(area, cr, left, top, plotWidth, plotHeight, foreground)
	if c.hovering && hasData && c.hoverX >= left && c.hoverX <= left+plotWidth && c.hoverY >= top && c.hoverY <= top+plotHeight {
		c.drawHover(area, cr, left, top, plotWidth, plotHeight, palette)
	}
	title := "COST AND USAGE"
	if c.data.granularity != "" {
		title += " — " + strings.ToUpper(c.data.granularity)
	}
	drawChartText(area, cr, title, 4, 5, foreground, 1)
	c.drawLegend(area, cr, left, 30, left+plotWidth, foreground, colors)
}

func (c *costChart) bounds() (float64, float64, bool) {
	minimum, maximum, found := 0.0, 0.0, false
	for _, bucket := range c.data.buckets {
		positive, negative := 0.0, 0.0
		for _, value := range bucket.values {
			if value != 0 {
				found = true
			}
			if c.mode == costChartStacked {
				if value >= 0 {
					positive += value
				} else {
					negative += value
				}
			} else {
				minimum = min(minimum, value)
				maximum = max(maximum, value)
			}
		}
		if c.mode == costChartStacked {
			minimum = min(minimum, negative)
			maximum = max(maximum, positive)
		}
	}
	return minimum, maximum, found
}

func (c *costChart) axisLeftGutter(area *gtk.DrawingArea) float64 {
	minimum, maximum, _ := c.bounds()
	maxWidth := 0
	for _, value := range []float64{minimum, maximum} {
		layout := area.CreatePangoLayout(formatCostCompact(value, c.data.unit))
		width, _ := layout.PixelSize()
		maxWidth = max(maxWidth, width)
	}
	return max(72, float64(maxWidth+24))
}

func (c *costChart) drawStackedBars(cr *cairo.Context, left, top, plotWidth, plotHeight, minimum, maximum float64, colors []gdk.RGBA, foreground gdk.RGBA) {
	slot := plotWidth / float64(len(c.data.buckets))
	barWidth := min(56, max(2, slot*0.7))
	for bucketIndex, bucket := range c.data.buckets {
		x := left + slot*float64(bucketIndex) + (slot-barWidth)/2
		positive, negative := 0.0, 0.0
		for seriesIndex, value := range bucket.values {
			from := positive
			if value < 0 {
				from = negative
				negative += value
			} else {
				positive += value
			}
			to := from + value
			y1 := costValueY(from, minimum, maximum, top, plotHeight)
			y2 := costValueY(to, minimum, maximum, top, plotHeight)
			color := colors[seriesIndex]
			cr.SetSourceRGBA(float64(color.Red()), float64(color.Green()), float64(color.Blue()), 0.94)
			cr.Rectangle(x, min(y1, y2), barWidth, max(1, math.Abs(y2-y1)))
			cr.Fill()
		}
		if bucket.estimated {
			topY := costValueY(positive, minimum, maximum, top, plotHeight)
			bottomY := costValueY(negative, minimum, maximum, top, plotHeight)
			cr.SetSourceRGBA(float64(foreground.Red()), float64(foreground.Green()), float64(foreground.Blue()), 0.55)
			cr.SetDash([]float64{3, 2}, 0)
			cr.SetLineWidth(1)
			cr.Rectangle(x+0.5, min(topY, bottomY)+0.5, max(1, barWidth-1), max(1, math.Abs(bottomY-topY)-1))
			cr.Stroke()
			cr.SetDash(nil, 0)
		}
	}
}

func (c *costChart) drawGroupedBars(cr *cairo.Context, left, top, plotWidth, plotHeight, minimum, maximum float64, colors []gdk.RGBA) {
	slot := plotWidth / float64(len(c.data.buckets))
	groupWidth := min(64, max(2, slot*0.76))
	barWidth := max(1, groupWidth/float64(max(1, len(c.data.series))))
	zero := costValueY(0, minimum, maximum, top, plotHeight)
	for bucketIndex, bucket := range c.data.buckets {
		startX := left + slot*float64(bucketIndex) + (slot-groupWidth)/2
		for seriesIndex, value := range bucket.values {
			y := costValueY(value, minimum, maximum, top, plotHeight)
			color := colors[seriesIndex]
			cr.SetSourceRGBA(float64(color.Red()), float64(color.Green()), float64(color.Blue()), 0.94)
			cr.Rectangle(startX+float64(seriesIndex)*barWidth, min(zero, y), max(1, barWidth-0.5), max(1, math.Abs(zero-y)))
			cr.Fill()
		}
	}
}

func (c *costChart) drawLines(cr *cairo.Context, left, top, plotWidth, plotHeight, minimum, maximum float64, colors []gdk.RGBA) {
	slot := plotWidth / float64(len(c.data.buckets))
	for seriesIndex := range c.data.series {
		color := colors[seriesIndex]
		cr.SetSourceRGBA(float64(color.Red()), float64(color.Green()), float64(color.Blue()), 0.96)
		cr.SetLineWidth(2)
		cr.SetDash(metricSeriesDash(seriesIndex), 0)
		for bucketIndex, bucket := range c.data.buckets {
			x := left + slot*(float64(bucketIndex)+0.5)
			y := costValueY(bucket.values[seriesIndex], minimum, maximum, top, plotHeight)
			if bucketIndex == 0 {
				cr.MoveTo(x, y)
			} else {
				cr.LineTo(x, y)
			}
		}
		cr.Stroke()
		cr.SetDash(nil, 0)
	}
}

func (c *costChart) drawXAxis(area *gtk.DrawingArea, cr *cairo.Context, left, top, plotWidth, plotHeight float64, foreground gdk.RGBA) {
	count := len(c.data.buckets)
	if count == 0 {
		drawChartText(area, cr, "No cost data for this range", left, top+plotHeight/2, foreground, 0.72)
		return
	}
	maxLabels := max(2, min(8, int(plotWidth/95)))
	indices := evenlySpacedIndices(count, maxLabels)
	slot := plotWidth / float64(count)
	for position, index := range indices {
		label := formatCostBucketLabel(c.data.buckets[index], c.data.granularity)
		layout := area.CreatePangoLayout(label)
		textWidth, _ := layout.PixelSize()
		x := left + slot*(float64(index)+0.5) - float64(textWidth)/2
		if position == 0 {
			x = max(left, x)
		} else if position == len(indices)-1 {
			x = min(left+plotWidth-float64(textWidth), x)
		}
		drawChartText(area, cr, label, x, top+plotHeight+9, foreground, 0.72)
	}
}

func (c *costChart) drawLegend(area *gtk.DrawingArea, cr *cairo.Context, x, y, maxX float64, foreground gdk.RGBA, colors []gdk.RGBA) {
	startX := x
	if len(c.data.series) == 0 {
		drawChartText(area, cr, "No grouped costs", x, y, foreground, 0.72)
		return
	}
	for index, series := range c.data.series {
		text := fmt.Sprintf("%s  %s", series.name, formatCostCompact(series.total, c.data.unit))
		layout := area.CreatePangoLayout(text)
		textWidth, _ := layout.PixelSize()
		entryWidth := float64(textWidth) + 38
		if x > startX && x+entryWidth > maxX {
			x = startX
			y += 21
		}
		color := colors[index]
		cr.SetSourceRGBA(float64(color.Red()), float64(color.Green()), float64(color.Blue()), 0.95)
		if c.mode == costChartLines {
			cr.SetLineWidth(2)
			cr.SetDash(metricSeriesDash(index), 0)
			cr.MoveTo(x, y+7)
			cr.LineTo(x+20, y+7)
			cr.Stroke()
			cr.SetDash(nil, 0)
		} else {
			cr.Rectangle(x+4, y+2, 12, 12)
			cr.Fill()
		}
		drawChartText(area, cr, text, x+24, y, foreground, 0.82)
		x += entryWidth
	}
}

func (c *costChart) legendRowCount(area *gtk.DrawingArea, startX, maxX float64) int {
	rows, x := 1, startX
	for _, series := range c.data.series {
		layout := area.CreatePangoLayout(fmt.Sprintf("%s  %s", series.name, formatCostCompact(series.total, c.data.unit)))
		textWidth, _ := layout.PixelSize()
		entryWidth := float64(textWidth) + 38
		if x > startX && x+entryWidth > maxX {
			rows++
			x = startX
		}
		x += entryWidth
	}
	return rows
}

func (c *costChart) drawHover(area *gtk.DrawingArea, cr *cairo.Context, left, top, plotWidth, plotHeight float64, palette semanticPalette) {
	index := int((c.hoverX - left) / plotWidth * float64(len(c.data.buckets)))
	index = min(len(c.data.buckets)-1, max(0, index))
	bucket := c.data.buckets[index]
	slot := plotWidth / float64(len(c.data.buckets))
	x := left + slot*float64(index)
	cr.SetSourceRGBA(float64(palette.foreground.Red()), float64(palette.foreground.Green()), float64(palette.foreground.Blue()), 0.08)
	cr.Rectangle(x, top, slot, plotHeight)
	cr.Fill()

	total := 0.0
	for _, value := range bucket.values {
		total += value
	}
	period := formatCostBucketPeriod(bucket, c.data.granularity)
	if bucket.estimated {
		period += " (estimated)"
	}
	lines := []string{period, "Total: " + formatCost(total, c.data.unit)}
	for seriesIndex, series := range c.data.series {
		lines = append(lines, fmt.Sprintf("%s: %s", series.name, formatCost(bucket.values[seriesIndex], c.data.unit)))
	}
	text := strings.Join(lines, "\n")
	layout := area.CreatePangoLayout(text)
	textWidth, textHeight := layout.PixelSize()
	boxWidth, boxHeight := float64(textWidth)+18, float64(textHeight)+14
	boxX := x + slot + 8
	if boxX+boxWidth > left+plotWidth-4 {
		boxX = x - boxWidth - 8
	}
	boxX = max(left+4, boxX)
	boxY := min(top+plotHeight-boxHeight-4, max(top+4, c.hoverY-boxHeight/2))
	cr.SetSourceRGBA(float64(palette.surface.Red()), float64(palette.surface.Green()), float64(palette.surface.Blue()), 0.98)
	cr.Rectangle(boxX, boxY, boxWidth, boxHeight)
	cr.Fill()
	cr.SetSourceRGBA(float64(palette.accent.Red()), float64(palette.accent.Green()), float64(palette.accent.Blue()), 0.9)
	cr.SetLineWidth(1)
	cr.Rectangle(boxX+0.5, boxY+0.5, boxWidth-1, boxHeight-1)
	cr.Stroke()
	drawChartText(area, cr, text, boxX+9, boxY+7, palette.foreground, 1)
}

func costChartColors(palette semanticPalette, count int) []gdk.RGBA {
	base := metricSeriesColors(palette)
	if len(base) == 0 {
		base = []gdk.RGBA{palette.foreground}
	}
	colors := make([]gdk.RGBA, 0, count)
	for index := 0; index < count; index++ {
		color := base[index%len(base)]
		cycle := index / len(base)
		if cycle > 0 {
			color = blendRGBA(color, palette.surface, min(0.5, float64(cycle)*0.2))
		}
		colors = append(colors, color)
	}
	return colors
}

func costValueY(value, minimum, maximum, top, plotHeight float64) float64 {
	return top + (maximum-value)/(maximum-minimum)*plotHeight
}

func formatCostCompact(amount float64, unit string) string {
	prefix := ""
	if unit == "" || strings.EqualFold(unit, "USD") {
		prefix = "$"
	}
	abs := math.Abs(amount)
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%s%.1fB", prefix, amount/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%s%.1fM", prefix, amount/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("%s%.1fK", prefix, amount/1_000)
	case prefix != "":
		return fmt.Sprintf("%s%.2f", prefix, amount)
	default:
		return fmt.Sprintf("%.2f %s", amount, unit)
	}
}

func compactCostSeriesName(name string) string {
	const maximumRunes = 42
	runes := []rune(strings.TrimSpace(name))
	if len(runes) <= maximumRunes {
		return string(runes)
	}
	return string(runes[:20]) + "…" + string(runes[len(runes)-21:])
}

func evenlySpacedIndices(count, maximum int) []int {
	if count <= 0 || maximum <= 0 {
		return nil
	}
	if count <= maximum {
		indices := make([]int, count)
		for index := range indices {
			indices[index] = index
		}
		return indices
	}
	maximum = max(2, maximum)
	indices := make([]int, 0, maximum)
	for position := 0; position < maximum; position++ {
		index := int(math.Round(float64(position) * float64(count-1) / float64(maximum-1)))
		if len(indices) == 0 || indices[len(indices)-1] != index {
			indices = append(indices, index)
		}
	}
	return indices
}

func formatCostBucketLabel(bucket costChartBucket, granularity string) string {
	if granularity == "monthly" {
		return bucket.start.Format("Jan 2006")
	}
	return bucket.start.Format("Jan 2")
}

func formatCostBucketPeriod(bucket costChartBucket, granularity string) string {
	if granularity == "monthly" {
		return bucket.start.Format("January 2006")
	}
	if granularity == "weekly" {
		return bucket.start.Format("Jan 2") + " — " + bucket.end.AddDate(0, 0, -1).Format("Jan 2, 2006")
	}
	return bucket.start.Format("Jan 2, 2006")
}

func (w *mainWindow) buildCostPane() gtk.Widgetter {
	w.costChart = newCostChart()
	w.costChart.SetPalette(w.currentSemanticPalette(w.window.StyleContext()))
	toolbar := gtk.NewBox(gtk.OrientationHorizontal, 8)
	toolbar.AddCSSClass("log-toolbar")
	label := gtk.NewLabel("Chart:")
	label.AddCSSClass("muted")
	toolbar.Append(label)
	var group *gtk.ToggleButton
	for _, choice := range []struct {
		mode  costChartMode
		label string
	}{
		{costChartStacked, "Stacked"},
		{costChartGrouped, "Grouped"},
		{costChartLines, "Lines"},
	} {
		button := gtk.NewToggleButtonWithLabel(choice.label)
		button.AddCSSClass("flat")
		if group != nil {
			button.SetGroup(group)
		} else {
			group = button
		}
		mode := choice.mode
		if mode == costChartStacked {
			button.SetActive(true)
		}
		button.ConnectToggled(func() {
			if button.Active() {
				w.costChart.SetMode(mode)
				w.setStatus("Cost chart mode: "+strings.ToLower(choice.label), false)
			}
		})
		toolbar.Append(button)
	}

	w.costSummaryBuffer = gtk.NewTextBuffer(nil)
	costSummaryView := gtk.NewTextViewWithBuffer(w.costSummaryBuffer)
	costSummaryView.SetEditable(false)
	costSummaryView.SetCursorVisible(false)
	costSummaryView.SetMonospace(true)
	costSummaryView.SetWrapMode(gtk.WrapWordChar)
	costSummaryView.AddCSSClass("inspector")
	w.costSummaryScroll = gtk.NewScrolledWindow()
	w.costSummaryScroll.SetHExpand(true)
	w.costSummaryScroll.SetVExpand(true)
	w.costSummaryScroll.SetSizeRequest(-1, 150)
	w.costSummaryScroll.SetChild(costSummaryView)

	w.costChart.SetExpandHandler(func() {
		expanded := !w.costChart.expanded
		w.costChart.SetExpanded(expanded)
		w.costSummaryScroll.SetVisible(!expanded)
		if expanded {
			w.setStatus("Maximized Cost and Usage chart", false)
		} else {
			w.setStatus("Restored Cost Explorer summary", false)
		}
	})
	pane := gtk.NewBox(gtk.OrientationVertical, 0)
	pane.Append(toolbar)
	pane.Append(w.costChart.Widget())
	pane.Append(gtk.NewSeparator(gtk.OrientationHorizontal))
	pane.Append(w.costSummaryScroll)
	return pane
}

func (w *mainWindow) setCostSummary(text string) {
	if w.costSummaryBuffer != nil {
		w.costSummaryBuffer.SetText(text)
	}
}
