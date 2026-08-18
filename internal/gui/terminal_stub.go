//go:build gui && !vte

package gui

import (
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

type vteTerminal struct {
	widget *gtk.Box
}

func newVTETerminal() *vteTerminal {
	box := gtk.NewBox(gtk.OrientationVertical, 12)
	box.SetHExpand(true)
	box.SetVExpand(true)
	box.SetHAlign(gtk.AlignCenter)
	box.SetVAlign(gtk.AlignCenter)
	title := gtk.NewLabel("Embedded terminal support is not built")
	title.AddCSSClass("app-title")
	detail := gtk.NewLabel("Install the GTK 4 VTE development package and rebuild with the “gui vte” build tags.")
	detail.SetWrap(true)
	detail.AddCSSClass("muted")
	box.Append(title)
	box.Append(detail)
	return &vteTerminal{widget: box}
}

func (terminal *vteTerminal) Widget() gtk.Widgetter { return terminal.widget }

func (terminal *vteTerminal) Spawn(string, []string) error {
	return fmt.Errorf("embedded terminal unavailable: rebuild e9s-gui with GTK 4 VTE support")
}

func (terminal *vteTerminal) Stop() {}

func vteAvailable() bool { return false }
