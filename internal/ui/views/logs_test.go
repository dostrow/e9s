package views

import (
	"strings"
	"testing"
	"time"
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

func TestNewLogViewerWithOptions_TailStartsFromLatestWindow(t *testing.T) {
	m := NewLogViewerWithOptions("tail", nil, "/aws/ecs/example", nil, true, 10*time.Second)
	if m.lastTS != 0 {
		t.Fatalf("tail mode should start from the newest logs, got lastTS=%d", m.lastTS)
	}
	if !m.tailMode {
		t.Fatal("tail mode should be enabled when follow=true")
	}
}

func TestNewLogViewerWithOptions_HistoricalUsesLookback(t *testing.T) {
	before := time.Now().Add(-11 * time.Second).UnixMilli()
	m := NewLogViewerWithOptions("history", nil, "/aws/ecs/example", nil, false, 10*time.Second)
	after := time.Now().Add(-9 * time.Second).UnixMilli()
	if m.lastTS < before || m.lastTS > after {
		t.Fatalf("historical start timestamp %d outside expected lookback window [%d, %d]", m.lastTS, before, after)
	}
	if m.tailMode {
		t.Fatal("tail mode should be disabled when follow=false")
	}
}

func TestNewLogViewerInRange_SetsAbsoluteWindow(t *testing.T) {
	m := NewLogViewerInRange("range", nil, "/aws/ecs/example", []string{"stream-a", "stream-b"}, 1000, 2000, "")
	if m.lastTS != 1000 {
		t.Fatalf("lastTS = %d, want 1000", m.lastTS)
	}
	if m.endTS != 2000 {
		t.Fatalf("endTS = %d, want 2000", m.endTS)
	}
	if !m.showStreams {
		t.Fatal("range viewer with multiple streams should show stream labels")
	}
	if m.tailMode {
		t.Fatal("range viewer should not be in tail mode")
	}
}

func TestFormatLogSource_MultiGroup(t *testing.T) {
	got := formatLogSource("/aws/ecs/api|ecs/app/task")
	if got != "/aws/ecs/api / ecs/app/task" {
		t.Fatalf("formatLogSource() = %q", got)
	}
}
