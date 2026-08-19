//go:build gui

package gui

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

type sourceDocument struct {
	Path     string
	Language string
}

type sourceEditor struct {
	buffer      *gtk.TextBuffer
	view        gtk.Widgetter
	setDocument func(sourceDocument)
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

func configurePlainSourceView(view *gtk.TextView) {
	view.SetEditable(true)
	view.SetCursorVisible(true)
	view.SetMonospace(true)
	view.SetWrapMode(gtk.WrapNone)
	view.AddCSSClass("inspector")
}

func darkEditorBackground(widget *gtk.Widget) bool {
	style := widget.StyleContext()
	for _, name := range []string{"theme_bg_color", "window_bg_color", "view_bg_color"} {
		color, ok := style.LookupColor(name)
		if !ok {
			continue
		}
		return editorColorIsDark(float64(color.Red()), float64(color.Green()), float64(color.Blue()))
	}
	return false
}

func editorColorIsDark(red, green, blue float64) bool {
	luminance := 0.2126*red + 0.7152*green + 0.0722*blue
	return luminance < 0.5
}
