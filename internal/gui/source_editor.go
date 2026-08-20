//go:build gui

package gui

import (
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

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
	view.Buffer().SetEnableUndo(true)
	view.AddCSSClass("inspector")
	view.AddCSSClass("code-font")
	installSourceEditorContextMenu(view)
}

// installSourceEditorContextMenu replaces the theme-owned GtkTextView menu.
// Keeping the popover parented to the editor gives it reliable pointer input
// inside nested notebooks and split panes, and lets e9s style the entire menu
// surface consistently with either the system theme or a built-in preset.
func installSourceEditorContextMenu(view *gtk.TextView) {
	buffer := view.Buffer()
	popover := gtk.NewPopover()
	popover.AddCSSClass("e9s-editor-context-menu")
	popover.SetAutohide(true)
	popover.SetHasArrow(false)
	popover.SetPosition(gtk.PosBottom)
	popover.SetParent(view)

	menu := gtk.NewBox(gtk.OrientationVertical, 2)
	menu.SetMarginTop(4)
	menu.SetMarginBottom(4)
	menu.SetMarginStart(4)
	menu.SetMarginEnd(4)
	contextButton := func(label string, run func()) *gtk.Button {
		button := gtk.NewButtonWithLabel(label)
		button.AddCSSClass("flat")
		button.AddCSSClass("editor-context-action")
		button.SetHAlign(gtk.AlignFill)
		button.ConnectClicked(func() {
			run()
			popover.Popdown()
			view.GrabFocus()
		})
		menu.Append(button)
		return button
	}
	undo := contextButton("Undo", buffer.Undo)
	redo := contextButton("Redo", buffer.Redo)
	cut := contextButton("Cut", func() { buffer.CutClipboard(view.Clipboard(), view.Editable()) })
	copy := contextButton("Copy", func() { buffer.CopyClipboard(view.Clipboard()) })
	paste := contextButton("Paste", func() { buffer.PasteClipboard(view.Clipboard(), nil, view.Editable()) })
	selectAll := contextButton("Select All", func() {
		start, end := buffer.Bounds()
		buffer.SelectRange(start, end)
	})
	popover.SetChild(menu)

	click := gtk.NewGestureClick()
	click.SetButton(3)
	click.SetPropagationPhase(gtk.PhaseCapture)
	click.ConnectPressed(func(_ int, x, y float64) {
		_, _, selected := buffer.SelectionBounds()
		undo.SetSensitive(buffer.CanUndo())
		redo.SetSensitive(buffer.CanRedo())
		cut.SetSensitive(view.Editable() && selected)
		copy.SetSensitive(selected)
		paste.SetSensitive(view.Editable())
		selectAll.SetSensitive(buffer.CharCount() > 0)
		point := gdk.NewRectangle(int(x), int(y), 1, 1)
		popover.SetPointingTo(&point)
		click.SetState(gtk.EventSequenceClaimed)
		popover.Popup()
	})
	view.AddController(click)
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
