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

func (terminal *vteTerminal) SpawnInDirectory(string, []string, string) error {
	return fmt.Errorf("embedded terminal unavailable: rebuild e9s-gui with GTK 4 VTE support")
}

func (terminal *vteTerminal) GrabFocus() {}

func (terminal *vteTerminal) HasFocus() bool { return false }

func (terminal *vteTerminal) CopyClipboard() {}

func (terminal *vteTerminal) PasteClipboard() {}

func (terminal *vteTerminal) HasSelection() bool { return false }

func (terminal *vteTerminal) SelectAll() {}

func (terminal *vteTerminal) NoteKeyPressed() {}

func (terminal *vteTerminal) SuppressSyntheticMouseMotion(float64, float64, bool, bool) bool {
	return false
}

func (terminal *vteTerminal) Stop() {}

func (terminal *vteTerminal) Running() bool { return false }

func (terminal *vteTerminal) ExitStatus() int { return 0 }

func (terminal *vteTerminal) WindowTitle() string { return "" }

func (terminal *vteTerminal) CurrentDirectory() string { return "" }

func (terminal *vteTerminal) ConnectWindowTitleChanged(func()) {}

func (terminal *vteTerminal) SetFontScale(float64) {}

func (terminal *vteTerminal) SetFont(string) {}

func (terminal *vteTerminal) SetScrollbackLines(int) {}

func (terminal *vteTerminal) SetPalette(semanticPalette) {}

func vteAvailable() bool { return false }
