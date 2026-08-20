//go:build gui

package gui

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

type sourceDocument struct {
	Path     string
	Language string
}

type sourceEditor struct {
	buffer       *gtk.TextBuffer
	view         gtk.Widgetter
	setDocument  func(sourceDocument)
	applyPalette func(semanticPalette)
}

func (e *sourceEditor) Buffer() *gtk.TextBuffer { return e.buffer }

func (e *sourceEditor) Widget() gtk.Widgetter { return e.view }

func (e *sourceEditor) SetDocument(document sourceDocument) {
	if e.setDocument != nil {
		e.setDocument(document)
	}
}

func (e *sourceEditor) SetText(text string) { e.buffer.SetText(text) }

func (e *sourceEditor) Text() string {
	start, end := e.buffer.Bounds()
	return e.buffer.Text(start, end, true)
}

func (e *sourceEditor) ConnectChanged(changed func()) {
	e.buffer.ConnectChanged(changed)
}

func (e *sourceEditor) ApplyPalette(palette semanticPalette) {
	if e != nil && e.applyPalette != nil {
		e.applyPalette(palette)
	}
}

func configurePlainSourceView(view *gtk.TextView) {
	view.SetEditable(true)
	view.SetCursorVisible(true)
	view.SetMonospace(true)
	view.SetWrapMode(gtk.WrapNone)
	view.AddCSSClass("inspector")
	view.AddCSSClass("code-font")
}

func sourceEditorPalette(palette semanticPalette) []string {
	selection := blendRGBA(palette.surface, palette.accent, 0.42)
	currentLine := blendRGBA(palette.surface, palette.foreground, 0.07)
	gutter := blendRGBA(palette.surface, palette.foreground, 0.035)
	return []string{
		rgbaHex(palette.foreground), rgbaHex(palette.surface), rgbaHex(palette.accent),
		rgbaHex(palette.success), rgbaHex(palette.warning), rgbaHex(palette.error),
		rgbaHex(palette.info), rgbaHex(palette.muted), rgbaHex(selection),
		rgbaHex(currentLine), rgbaHex(gutter),
	}
}
