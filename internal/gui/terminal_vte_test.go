//go:build gui && vte

package gui

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

func TestVTEMotionSuppressionControllerIsAvailable(t *testing.T) {
	if !gtk.InitCheck() {
		t.Skip("GTK display unavailable")
	}

	terminal := newVTETerminal()
	if !terminal.motionSuppressionAvailable() {
		t.Fatal("VTE motion controller was not discovered")
	}

	terminal.NoteKeyPressed()
	if !terminal.motionSuppressionActive() {
		t.Fatal("VTE enter and motion handlers were not suppressed")
	}
	terminal.NotePointerActivity()
	if terminal.motionSuppressionActive() {
		t.Fatal("VTE enter and motion handlers remained suppressed after pointer activity")
	}
}

func TestVTESixelEnabledWhenRuntimeSupportsIt(t *testing.T) {
	if !gtk.InitCheck() {
		t.Skip("GTK display unavailable")
	}
	if !vteSixelAvailable() {
		t.Skip("VTE was built without SIXEL support")
	}

	terminal := newVTETerminal()
	if !terminal.sixelEnabled() {
		t.Fatal("VTE reports SIXEL support but the terminal did not enable it")
	}
}
