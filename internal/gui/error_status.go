//go:build gui

package gui

import (
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func compactStatusMessage(message string) string {
	return strings.Join(strings.Fields(message), " ")
}

func (w *mainWindow) dismissStatusError() {
	w.setStatus("Ready", false)
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
