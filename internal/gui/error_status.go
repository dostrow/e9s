//go:build gui

package gui

import (
	"strings"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pangocairo"
)

const (
	moduleECS              = "ecs"
	moduleCloudWatchLogs   = "cloudwatch-logs"
	moduleCloudWatchAlarms = "cloudwatch-alarms"
	moduleSSM              = "ssm"
	moduleSecrets          = "secrets-manager"
	moduleLambda           = "lambda"
	moduleCodeBuild        = "codebuild"
	moduleEC2              = "ec2"
	moduleECR              = "ecr"
	moduleRDS              = "rds"
	moduleS3               = "s3"
	moduleDynamoDB         = "dynamodb"
	moduleSQS              = "sqs"
	moduleRoute53          = "route53"
	moduleTofu             = "tofu"
	moduleCostExplorer     = "cost-explorer"
	moduleElastiCache      = "elasticache"
)

func compactStatusMessage(message string) string {
	return strings.Join(strings.Fields(message), " ")
}

func (w *mainWindow) dismissStatusError() {
	w.setStatus("Ready", false)
}

func moduleForPage(page string) string {
	if isECSPage(page) {
		return moduleECS
	}
	switch page {
	case pageLogGroups, pageLogStreams, pageSavedLogSearch:
		return moduleCloudWatchLogs
	case pageAlarms:
		return moduleCloudWatchAlarms
	case pageSSM:
		return moduleSSM
	case pageSecrets:
		return moduleSecrets
	case pageLambda:
		return moduleLambda
	case pageCodeBuildProjects, pageCodeBuildBuilds:
		return moduleCodeBuild
	case pageEC2Instances, pageEC2LoadBalancers, pageEC2TargetGroups, pageEC2SecurityGroups, pageEC2VPCs, pageEC2Subnets, pageEC2Volumes:
		return moduleEC2
	case pageECRRepositories, pageECRImages, pageECRFindings:
		return moduleECR
	case pageRDSInstances, pageRDSClusters:
		return moduleRDS
	case pageElastiCache:
		return moduleElastiCache
	case pageS3Buckets, pageS3Objects:
		return moduleS3
	case pageDynamoTables, pageDynamoItems:
		return moduleDynamoDB
	case pageSQSQueues, pageSQSMessages:
		return moduleSQS
	case pageRoute53Zones, pageRoute53Records:
		return moduleRoute53
	case pageTofuWorkspaces, pageTofuResources, pageTofuPlan:
		return moduleTofu
	case pageCostOverview, pageCostBreakdown, pageCostAnomalies, pageCostResources, pageCostSavedView:
		return moduleCostExplorer
	default:
		return ""
	}
}

func (w *mainWindow) newModuleExpander(label, module string, child gtk.Widgetter) *gtk.Expander {
	expander := gtk.NewExpander("")
	heading := gtk.NewBox(gtk.OrientationHorizontal, 6)
	heading.SetHExpand(true)
	chevron := w.newModuleChevronArea(expander.Expanded)
	text := w.newModuleHeadingArea(label, expander.Expanded)
	glyph := gtk.NewImageFromIconName("dialog-error-symbolic")
	glyph.SetPixelSize(16)
	glyph.SetTooltipText("This module has an error")
	glyph.SetVisible(false)
	heading.Append(chevron)
	heading.Append(text)
	heading.Append(glyph)

	expander.SetLabelWidget(heading)
	expander.SetExpanded(false)
	expander.SetChild(child)
	expander.AddCSSClass("module-heading")
	expander.NotifyProperty("expanded", func() {
		chevron.QueueDraw()
		text.QueueDraw()
	})
	w.moduleErrorGlyphs[module] = glyph
	return expander
}

func (w *mainWindow) newModuleChevronArea(expanded func() bool) *gtk.DrawingArea {
	area := gtk.NewDrawingArea()
	w.semanticDrawingAreas = append(w.semanticDrawingAreas, area)
	area.SetContentWidth(12)
	area.SetContentHeight(20)
	area.SetDrawFunc(func(area *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		clearDrawingSurface(cr)
		foreground := w.currentSemanticPalette(area.StyleContext()).foreground
		cr.SetSourceRGBA(float64(foreground.Red()), float64(foreground.Green()), float64(foreground.Blue()), float64(foreground.Alpha()))
		cr.SetLineWidth(1.6)
		centerX, centerY := float64(width)/2, float64(height)/2
		if expanded() {
			cr.MoveTo(centerX-3.5, centerY-2)
			cr.LineTo(centerX, centerY+2)
			cr.LineTo(centerX+3.5, centerY-2)
		} else {
			cr.MoveTo(centerX-2, centerY-3.5)
			cr.LineTo(centerX+2, centerY)
			cr.LineTo(centerX-2, centerY+3.5)
		}
		cr.Stroke()
	})
	return area
}

func (w *mainWindow) newModuleHeadingArea(label string, expanded func() bool) *gtk.DrawingArea {
	area := gtk.NewDrawingArea()
	w.semanticDrawingAreas = append(w.semanticDrawingAreas, area)
	area.SetHExpand(true)
	area.SetContentWidth(1)
	area.SetContentHeight(20)
	area.SetOverflow(gtk.OverflowHidden)
	area.AddCSSClass("module-heading-text")
	area.SetDrawFunc(func(area *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		clearDrawingSurface(cr)

		foreground := w.currentSemanticPalette(area.StyleContext()).foreground
		cr.SetSourceRGBA(
			float64(foreground.Red()),
			float64(foreground.Green()),
			float64(foreground.Blue()),
			float64(foreground.Alpha()),
		)
		layout := area.CreatePangoLayout(label)
		_, textHeight := layout.PixelSize()
		cr.MoveTo(0, float64(max(0, height-textHeight)/2))
		pangocairo.ShowLayout(cr, layout)
	})
	return area
}

func (w *mainWindow) updateModuleErrorGlyph(visible bool) {
	module := moduleForPage(w.currentPage)
	for name, glyph := range w.moduleErrorGlyphs {
		show := visible && name == module
		glyph.SetVisible(show)
		if show {
			glyph.SetTooltipText(w.lastError)
		}
	}
}

func (w *mainWindow) showStatusErrorDetails() {
	message := w.lastError
	if message == "" {
		return
	}

	dialog := gtk.NewDialogWithFlags("Error details", &w.window.Window, gtk.DialogModal)
	dialog.SetDestroyWithParent(true)
	dialog.SetDefaultSize(720, 360)
	content := dialog.ContentArea()
	content.SetMarginTop(16)
	content.SetMarginBottom(16)
	content.SetMarginStart(16)
	content.SetMarginEnd(16)

	buffer := gtk.NewTextBuffer(nil)
	buffer.SetText(message)
	view := gtk.NewTextViewWithBuffer(buffer)
	view.SetEditable(false)
	view.SetCursorVisible(true)
	view.SetMonospace(true)
	view.SetWrapMode(gtk.WrapWordChar)
	view.AddCSSClass("inspector")

	scroll := gtk.NewScrolledWindow()
	scroll.SetVExpand(true)
	scroll.SetHExpand(true)
	scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	scroll.SetChild(view)
	content.Append(scroll)

	const dismissResponse = 101
	dialog.AddButton("Dismiss", dismissResponse)
	dialog.AddButton("Close", int(gtk.ResponseClose))
	dialog.ConnectResponse(func(response int) {
		dialog.Destroy()
		if response == dismissResponse {
			w.dismissStatusError()
		}
	})
	dialog.Present()
}
