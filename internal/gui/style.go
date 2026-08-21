package gui

import (
	_ "embed"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// styleCSS contains structural application styling only. Colors and backgrounds
// intentionally come from the active GTK theme.
//
//go:embed style.css
var styleCSS string

func newApplicationCheckButton(label string) *gtk.CheckButton {
	button := gtk.NewCheckButtonWithLabel(label)
	button.AddCSSClass("e9s-check")
	return button
}
