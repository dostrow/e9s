//go:build gui

package gui

import (
	"testing"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
)

func TestSourceEditorPaletteUsesGTKSemanticColors(t *testing.T) {
	palette := semanticPalette{
		foreground: gdk.NewRGBA(0.8, 0.7, 0.6, 1),
		surface:    gdk.NewRGBA(0.1, 0.11, 0.12, 1),
		accent:     gdk.NewRGBA(0.7, 0.4, 0.2, 1),
		success:    gdk.NewRGBA(0.3, 0.7, 0.4, 1),
		warning:    gdk.NewRGBA(0.9, 0.7, 0.2, 1),
		error:      gdk.NewRGBA(0.9, 0.2, 0.2, 1),
		info:       gdk.NewRGBA(0.3, 0.6, 0.9, 1),
		muted:      gdk.NewRGBA(0.5, 0.5, 0.5, 1),
	}
	colors := sourceEditorPalette(palette)
	if len(colors) != 11 {
		t.Fatalf("sourceEditorPalette() returned %d colors", len(colors))
	}
	for index, want := range []string{"#CCB299", "#1A1C1F", "#B26633", "#4DB266", "#E5B233", "#E53333", "#4D99E5", "#808080"} {
		if colors[index] != want {
			t.Errorf("source editor color %d = %q, want %q", index, colors[index], want)
		}
	}
	if colors[8] == colors[1] || colors[9] == colors[1] || colors[10] == colors[1] {
		t.Fatal("selection, current-line, and gutter colors must be derived from—not equal to—the surface")
	}
}

func TestSourceEditorForwardsPaletteChanges(t *testing.T) {
	called := false
	editor := &sourceEditor{applyPalette: func(semanticPalette) { called = true }}
	editor.ApplyPalette(semanticPalette{})
	if !called {
		t.Fatal("ApplyPalette did not reach the source-view adapter")
	}
}

func TestSourceEditorForwardsDocumentChanges(t *testing.T) {
	var got sourceDocument
	editor := &sourceEditor{setDocument: func(document sourceDocument) { got = document }}
	want := sourceDocument{Path: "handler.py", Language: "python3"}
	editor.SetDocument(want)
	if got != want {
		t.Fatalf("SetDocument() forwarded %#v, want %#v", got, want)
	}
}
