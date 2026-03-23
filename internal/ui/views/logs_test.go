package views

import (
	"strings"
	"testing"
)

func TestSanitizeLogMessage_StripsCR(t *testing.T) {
	got := sanitizeLogMessage("line1\r\nline2\rline3")
	if strings.Contains(got, "\r") {
		t.Error("Should not contain \\r")
	}
	// \r\n → \n, standalone \r → \n
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Errorf("Expected 3 lines, got %d: %q", len(lines), got)
	}
}

func TestSanitizeLogMessage_StripsTabs(t *testing.T) {
	got := sanitizeLogMessage("col1\tcol2")
	if strings.Contains(got, "\t") {
		t.Error("Should not contain tabs")
	}
	if !strings.Contains(got, "    ") {
		t.Error("Tabs should be replaced with spaces")
	}
}

func TestSanitizeLogMessage_StripsControlChars(t *testing.T) {
	got := sanitizeLogMessage("hello\x00world\x07test")
	if got != "helloworldtest" {
		t.Errorf("got %q, want %q", got, "helloworldtest")
	}
}

func TestSanitizeLogMessage_PreservesNewlines(t *testing.T) {
	got := sanitizeLogMessage("line1\nline2\n")
	if !strings.Contains(got, "\n") {
		t.Error("Should preserve newlines")
	}
}

func TestSanitizeLogMessage_TrimsTrailing(t *testing.T) {
	got := sanitizeLogMessage("hello  \n\r ")
	if strings.HasSuffix(got, " ") || strings.HasSuffix(got, "\n") {
		t.Errorf("Should trim trailing whitespace: %q", got)
	}
}

func TestWrapPlainText_NoWrap(t *testing.T) {
	lines := wrapPlainText("short", 100)
	if len(lines) != 1 || lines[0] != "short" {
		t.Errorf("Expected single line, got %v", lines)
	}
}

func TestWrapPlainText_Wraps(t *testing.T) {
	input := strings.Repeat("x", 100)
	lines := wrapPlainText(input, 30)
	if len(lines) != 4 { // 100/30 = 3.33 → 4
		t.Errorf("Expected 4 lines, got %d", len(lines))
	}
	for i, line := range lines {
		r := []rune(line)
		if i < len(lines)-1 && len(r) != 30 {
			t.Errorf("Line %d should be 30 runes, got %d", i, len(r))
		}
	}
}

func TestWrapPlainText_ZeroWidth(t *testing.T) {
	lines := wrapPlainText("hello", 0)
	if len(lines) != 1 {
		t.Error("Zero width should return single line")
	}
}

func TestWrapPlainText_Empty(t *testing.T) {
	lines := wrapPlainText("", 50)
	if len(lines) != 1 || lines[0] != "" {
		t.Errorf("Empty should return single empty line, got %v", lines)
	}
}

func TestWrapPlainText_ExactWidth(t *testing.T) {
	lines := wrapPlainText("12345", 5)
	if len(lines) != 1 || lines[0] != "12345" {
		t.Errorf("Exact width should return single line, got %v", lines)
	}
}

func TestWrapPlainText_Unicode(t *testing.T) {
	input := strings.Repeat("日", 10) // 10 CJK chars
	lines := wrapPlainText(input, 5)
	if len(lines) != 2 {
		t.Errorf("Expected 2 lines for 10 runes at width 5, got %d", len(lines))
	}
}
