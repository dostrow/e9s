package views

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/highlight"
	"github.com/dostrow/e9s/internal/model"
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
	before := time.Now().Add(-15*time.Minute - time.Second).UnixMilli()
	m := NewLogViewer("tail", nil, "/aws/ecs/example", nil)
	after := time.Now().Add(-15*time.Minute + time.Second).UnixMilli()
	if m.lastTS < before || m.lastTS > after {
		t.Fatalf("tail mode start timestamp %d outside recent window [%d, %d]", m.lastTS, before, after)
	}
	if !m.tailMode {
		t.Fatal("tail mode should be enabled when follow=true")
	}
}

func TestLogViewerGlobalPauseWaitsAndExplainsLiveRetrieval(t *testing.T) {
	m := NewLogViewer("tail", nil, "/aws/ecs/example", nil).WithGlobalRefreshPaused(true)
	if !m.globalRefreshPaused {
		t.Fatal("global refresh pause was not retained")
	}
	if cmd := m.Init(); cmd == nil {
		t.Fatal("paused live viewer must keep a wake-up tick scheduled")
	}
	view := m.View()
	if !strings.Contains(view, "Live log data will not be retrieved") {
		t.Fatalf("paused viewer does not explain the waiting state:\n%s", view)
	}
	if cmd := m.SetGlobalRefreshPaused(false); cmd == nil {
		t.Fatal("resuming a live viewer should request logs immediately")
	}
}

func TestLogViewerGlobalPauseDoesNotBlockHistoricalRequest(t *testing.T) {
	m := NewLogViewerWithOptions("history", nil, "/aws/ecs/example", nil, false, time.Minute).
		WithGlobalRefreshPaused(true)
	if cmd := m.SetGlobalRefreshPaused(false); cmd != nil {
		t.Fatal("resuming a historical viewer should not start live polling")
	}
	if strings.Contains(m.View(), "Live log data will not be retrieved") {
		t.Fatal("historical viewer should not display a live-refresh warning")
	}
}

func TestLogViewerKeepsInclusiveCursorAndDeduplicatesOverlappingPolls(t *testing.T) {
	m := LogViewerModel{lastTS: 100, follow: false}
	first := model.LogEntry{ID: "event-1", Timestamp: 100, Message: "running"}
	late := model.LogEntry{ID: "event-2", Timestamp: 100, Message: "final line"}

	m, _ = m.Update(LogsLoadedMsg{Entries: []model.LogEntry{first}, LastTS: 100})
	m, _ = m.Update(LogsLoadedMsg{Entries: []model.LogEntry{first, late}, LastTS: 100})

	if m.lastTS != 100 {
		t.Fatalf("inclusive cursor = %d, want 100", m.lastTS)
	}
	if len(m.lines) != 2 || m.lines[1].message != "final line" {
		t.Fatalf("lines = %#v", m.lines)
	}
}

func TestLogViewerHonorsConfiguredBufferLimit(t *testing.T) {
	m := (LogViewerModel{follow: false}).WithLimits(2, 3)
	m, _ = m.Update(LogsLoadedMsg{Entries: []model.LogEntry{
		{ID: "one", Timestamp: 1, Message: "one"},
		{ID: "two", Timestamp: 2, Message: "two"},
		{ID: "three", Timestamp: 3, Message: "three"},
		{ID: "four", Timestamp: 4, Message: "four"},
	}, LastTS: 4})

	if m.configuredPageSize() != 2 {
		t.Fatalf("page size = %d, want 2", m.configuredPageSize())
	}
	if len(m.lines) != 3 || m.lines[0].message != "two" || m.lines[2].message != "four" {
		t.Fatalf("buffered lines = %#v, want the newest three entries", m.lines)
	}
}

func TestLogViewerStreamVisibilityOnlyFiltersDisplay(t *testing.T) {
	m := LogViewerModel{width: 120, height: 30, search: "event"}
	m, _ = m.Update(LogsLoadedMsg{Entries: []model.LogEntry{
		{ID: "api", Timestamp: 1, Stream: "api", Message: "api event"},
		{ID: "worker", Timestamp: 2, Stream: "worker", Message: "worker event"},
	}, LastTS: 2})
	m = m.OpenStreamManager()
	if !m.streamManager {
		t.Fatal("stream manager did not open for a multi-stream buffer")
	}
	m, _ = m.handleStreamManager(tea.KeyMsg{Type: tea.KeySpace})
	m, _ = m.handleStreamManager(tea.KeyMsg{Type: tea.KeyEsc})

	view := m.View()
	if strings.Contains(view, "api event") || !strings.Contains(view, "worker event") {
		t.Fatalf("filtered view = %q", view)
	}
	if len(m.lines) != 2 || len(m.ExportLines()) != 2 {
		t.Fatalf("visibility altered buffered results: %d lines, %d exported", len(m.lines), len(m.ExportLines()))
	}
	if len(m.matchIndices) != 1 || m.lines[m.matchIndices[0]].stream != "worker" {
		t.Fatalf("visible matches = %#v", m.matchIndices)
	}

	m, _ = m.Update(LogsLoadedMsg{Entries: []model.LogEntry{{
		ID: "scheduler", Timestamp: 3, Stream: "scheduler", Message: "scheduler event",
	}}, LastTS: 3})
	if m.streamHidden("scheduler") {
		t.Fatal("newly encountered stream should default to visible")
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

func TestRangeViewerKeepsExactJumpTargetCenteredWhenCapped(t *testing.T) {
	anchor := model.LogEntry{ID: "selected", Timestamp: 1500, Message: "selected"}
	m := NewLogViewerInRange("range", nil, "/aws/ecs/example", nil, 0, 3000, "selected")
	m = m.WithJumpTarget(anchor)
	m.width, m.height = 120, 30
	entries := make([]model.LogEntry, 0, 3000)
	for i := 0; i < 3000; i++ {
		if i == 1500 {
			continue
		}
		entries = append(entries, model.LogEntry{ID: fmt.Sprintf("event-%d", i), Timestamp: int64(i), Message: "event"})
	}

	m, _ = m.Update(LogsLoadedMsg{Entries: entries, LastTS: 2999})
	if len(m.lines) != maxLogLines {
		t.Fatalf("lines = %d, want %d", len(m.lines), maxLogLines)
	}
	if m.lines[maxLogLines/2].key() != anchor.Key() {
		t.Fatalf("center line = %#v, want anchor %#v", m.lines[maxLogLines/2], anchor)
	}
	visible := m.visibleLines()
	if m.scroll != maxLogLines/2-visible/2 {
		t.Fatalf("scroll = %d, want %d", m.scroll, maxLogLines/2-visible/2)
	}
}

func TestFormatLogSource_MultiGroup(t *testing.T) {
	got := formatLogSource("/aws/ecs/api|ecs/app/task")
	if got != "/aws/ecs/api / ecs/app/task" {
		t.Fatalf("formatLogSource() = %q", got)
	}
}

func TestLogViewerHighlightsDoNotAlterExport(t *testing.T) {
	m := LogViewerModel{
		lines: []logLine{{timestamp: 1000, message: "🔥 ERROR complete"}},
	}
	m = m.SetHighlightRules([]model.LogHighlightRule{{
		Pattern: "ERROR", Match: model.LogHighlightLiteral, Style: model.LogHighlightError,
	}})
	spans := m.highlighter.Spans(m.lines[0].message)
	if len(spans) != 1 || spans[0].Start != 2 || spans[0].End != 7 {
		t.Fatalf("highlight spans = %#v", spans)
	}
	exported := m.ExportLines()
	if len(exported) != 1 || strings.Contains(exported[0], "\x1b[") || !strings.HasSuffix(exported[0], "🔥 ERROR complete") {
		t.Fatalf("ExportLines() = %#v", exported)
	}
}

func TestLogHighlightManagerAddsAndValidatesRules(t *testing.T) {
	m := LogViewerModel{}.OpenHighlightManager()
	m, _ = m.Update(keyRune('a'))
	for _, r := range "timeout" {
		m, _ = m.Update(keyRune(r))
	}
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if len(m.highlightRules) != 1 || m.highlightRules[0].Pattern != "timeout" {
		t.Fatalf("rules = %#v", m.highlightRules)
	}

	m.highlightRules[0].Pattern = "["
	m.highlighter, _ = highlight.Compile(m.highlightRules)
	m, _ = m.Update(keyRune('m'))
	m, _ = m.Update(keyRune('m'))
	if m.highlightRules[0].Match != model.LogHighlightLiteralCI {
		t.Fatalf("invalid regex changed match mode to %q", m.highlightRules[0].Match)
	}
	if m.highlightError == "" {
		t.Fatal("invalid regex did not produce an error")
	}
}

func TestLogHighlightManagerRequestsPersistence(t *testing.T) {
	m := LogViewerModel{}.SetHighlightRules([]model.LogHighlightRule{{
		Pattern: "error", Match: model.LogHighlightLiteralCI, Style: model.LogHighlightError,
	}}).OpenHighlightManager()
	var cmd tea.Cmd
	_, cmd = m.Update(keyRune('w'))
	if cmd == nil {
		t.Fatal("save did not return a command")
	}
	msg, ok := cmd().(LogHighlightSaveMsg)
	if !ok || len(msg.Rules) != 1 || msg.Rules[0].Pattern != "error" {
		t.Fatalf("save message = %#v", msg)
	}
}

func TestLogViewerHiddenStreamsArePresentationOnly(t *testing.T) {
	m := LogViewerModel{
		lines: []logLine{
			{stream: "api", message: "visible"},
			{stream: "health", message: "hidden"},
		},
	}
	m = m.SetHiddenStreams([]string{"health"})
	if got := m.HiddenStreams(); !reflect.DeepEqual(got, []string{"health"}) {
		t.Fatalf("HiddenStreams() = %#v", got)
	}
	if got := m.displayIndices(); !reflect.DeepEqual(got, []int{0}) {
		t.Fatalf("displayIndices() = %#v", got)
	}
	if len(m.lines) != 2 {
		t.Fatalf("underlying buffer contains %d lines, want 2", len(m.lines))
	}
}

func keyRune(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}
