//go:build gui && !sourceview

package gui

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

func newSourceEditor(sourceDocument) *sourceEditor {
	buffer := gtk.NewTextBuffer(nil)
	view := gtk.NewTextViewWithBuffer(buffer)
	configurePlainSourceView(view)
	return &sourceEditor{buffer: buffer, view: view}
}

func sourceViewLanguageID(sourceDocument) string { return "" }
