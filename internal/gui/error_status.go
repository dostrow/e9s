//go:build gui

package gui

import (
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

const (
	moduleECS              = "ecs"
	moduleCloudWatchLogs   = "cloudwatch-logs"
	moduleCloudWatchAlarms = "cloudwatch-alarms"
	moduleSSM              = "ssm"
	moduleSecrets          = "secrets-manager"
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
	default:
		return ""
	}
}

func moduleHeadingMargin(expanded bool) int {
	if expanded {
		return 4
	}
	return 0
}

func (w *mainWindow) newModuleExpander(label, module string, child gtk.Widgetter) *gtk.Expander {
	heading := gtk.NewBox(gtk.OrientationHorizontal, 6)
	heading.SetHExpand(true)
	text := gtk.NewLabel(label)
	text.SetXAlign(0)
	text.SetHExpand(true)
	glyph := gtk.NewImageFromIconName("dialog-error-symbolic")
	glyph.SetPixelSize(16)
	glyph.SetTooltipText("This module has an error")
	glyph.SetVisible(false)
	heading.Append(text)
	heading.Append(glyph)

	expander := gtk.NewExpander("")
	expander.SetLabelWidget(heading)
	expander.SetExpanded(false)
	expander.SetChild(child)
	expander.AddCSSClass("module-heading")
	updateHeadingMargin := func() {
		heading.SetMarginStart(moduleHeadingMargin(expander.Expanded()))
	}
	expander.NotifyProperty("expanded", updateHeadingMargin)
	updateHeadingMargin()
	w.moduleErrorGlyphs[module] = glyph
	return expander
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
