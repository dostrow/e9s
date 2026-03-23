package ui

import (
	"strings"
	"testing"
)

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"\033[31mred\033[0m", "red"},
		{"\033[1;32mbold green\033[0m text", "bold green text"},
		{"", ""},
		{"no codes here", "no codes here"},
	}
	for _, tt := range tests {
		got := stripAnsi(tt.input)
		if got != tt.want {
			t.Errorf("stripAnsi(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRenderOverlay_ContainsModal(t *testing.T) {
	bg := strings.Repeat("background line\n", 20)
	modal := "[ modal content ]"

	result := renderOverlay(bg, modal, 60, 20)

	if !strings.Contains(result, "modal content") {
		t.Error("Overlay should contain the modal content")
	}
}

func TestRenderOverlay_PreservesHeight(t *testing.T) {
	bg := "line1\nline2\nline3"
	modal := "hi"

	result := renderOverlay(bg, modal, 40, 10)
	lines := strings.Split(result, "\n")

	if len(lines) != 10 {
		t.Errorf("Expected 10 lines, got %d", len(lines))
	}
}
