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
	case pageS3Buckets, pageS3Objects:
		return moduleS3
	default:
		return ""
	}
}

func moduleHeadingTextOffset(expanded bool) int {
	if expanded {
		return 4
	}
	return 0
}

func (w *mainWindow) newModuleExpander(label, module string, child gtk.Widgetter) *gtk.Expander {
	expander := gtk.NewExpander("")
	heading := gtk.NewBox(gtk.OrientationHorizontal, 6)
	heading.SetHExpand(true)
	text := newModuleHeadingArea(label, expander.Expanded)
	glyph := gtk.NewImageFromIconName("dialog-error-symbolic")
	glyph.SetPixelSize(16)
	glyph.SetTooltipText("This module has an error")
	glyph.SetVisible(false)
	heading.Append(text)
	heading.Append(glyph)

	expander.SetLabelWidget(heading)
	expander.SetExpanded(false)
	expander.SetChild(child)
	expander.AddCSSClass("module-heading")
	expander.NotifyProperty("expanded", text.QueueDraw)
	w.moduleErrorGlyphs[module] = glyph
	return expander
}

func newModuleHeadingArea(label string, expanded func() bool) *gtk.DrawingArea {
	area := gtk.NewDrawingArea()
	area.SetHExpand(true)
	area.SetContentWidth(1)
	area.SetContentHeight(20)
	area.SetOverflow(gtk.OverflowHidden)
	area.AddCSSClass("module-heading-text")
	area.SetDrawFunc(func(area *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		style := area.StyleContext()
		clearDrawingSurface(cr)

		foreground := style.Color()
		cr.SetSourceRGBA(
			float64(foreground.Red()),
			float64(foreground.Green()),
			float64(foreground.Blue()),
			float64(foreground.Alpha()),
		)
		layout := area.CreatePangoLayout(label)
		_, textHeight := layout.PixelSize()
		cr.MoveTo(
			float64(moduleHeadingTextOffset(expanded())),
			float64(max(0, height-textHeight)/2),
		)
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
