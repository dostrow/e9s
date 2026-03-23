package ui

import (
	"strings"
	"testing"
)

func TestWrapText_NoWrap(t *testing.T) {
	input := "  [enter] select  [esc] back  [q] quit"
	got := wrapText(input, 100)
	if got != input {
		t.Errorf("Should not wrap when within width:\n  got:  %q\n  want: %q", got, input)
	}
}

func TestWrapText_Wraps(t *testing.T) {
	input := "  [enter] select  [esc] back  [q] quit  [?] help"
	got := wrapText(input, 30)

	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Errorf("Expected multiple lines, got %d: %q", len(lines), got)
	}
}

func TestWrapText_BreaksBeforeBracket(t *testing.T) {
	input := "  [enter] select  [esc] back  [q] quit"
	got := wrapText(input, 25)

	lines := strings.Split(got, "\n")
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "[") {
			t.Errorf("Wrapped line should start with '[': %q", trimmed)
		}
	}
}

func TestWrapText_EmptyInput(t *testing.T) {
	got := wrapText("", 50)
	if got != "" {
		t.Errorf("Expected empty, got %q", got)
	}
}

func TestWrapText_ZeroWidth(t *testing.T) {
	input := "  [a] test"
	got := wrapText(input, 0)
	if got != input {
		t.Errorf("Zero width should return input unchanged")
	}
}

func TestMaxLineWidth(t *testing.T) {
	input := "short\na longer line here\nmed"
	got := maxLineWidth(input)
	// "a longer line here" = 18 chars
	if got < 18 {
		t.Errorf("maxLineWidth = %d, want >= 18", got)
	}
}

func TestMaxLineWidth_Empty(t *testing.T) {
	got := maxLineWidth("")
	if got != 0 {
		t.Errorf("maxLineWidth(\"\") = %d, want 0", got)
	}
}
