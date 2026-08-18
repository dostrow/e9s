//go:build gui

package gui

import "testing"

func TestHangingIndentTagLeavesFirstLineAtViewportEdge(t *testing.T) {
	tag := newHangingIndentTag(137)
	if got := tag.ObjectProperty("left-margin"); got != 0 {
		t.Fatalf("left-margin = %#v, want 0", got)
	}
	if got := tag.ObjectProperty("indent"); got != -137 {
		t.Fatalf("indent = %#v, want -137", got)
	}
}
