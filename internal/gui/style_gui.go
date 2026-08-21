//go:build gui

package gui

import "github.com/diamondburned/gotk4/pkg/gtk/v4"

func newApplicationCheckButton(label string) *gtk.CheckButton {
	button := gtk.NewCheckButtonWithLabel(label)
	button.AddCSSClass("e9s-check")
	return button
}
