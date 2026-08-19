package gui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

func TestBoundedLogsTrimsOldestEntries(t *testing.T) {
	logs := newBoundedLogs(3)
	logs.append([]model.LogEntry{
		{Timestamp: 1, Message: "one"},
		{Timestamp: 2, Message: "two"},
		{Timestamp: 3, Message: "three"},
		{Timestamp: 4, Message: "four"},
	})

	if logs.len() != 3 {
		t.Fatalf("len() = %d, want 3", logs.len())
	}
	if cap(logs.entries) != 3 {
		t.Fatalf("cap(entries) = %d, want 3", cap(logs.entries))
	}
	text := logs.text("")
	if strings.Contains(text, "one") || !strings.Contains(text, "four") {
		t.Fatalf("text() = %q", text)
	}
}

func TestBoundedLogsPrependTrimsNewestEntries(t *testing.T) {
	logs := newBoundedLogs(3)
	logs.append([]model.LogEntry{
		{Timestamp: 3, Message: "three"},
		{Timestamp: 4, Message: "four"},
		{Timestamp: 5, Message: "five"},
	})

	added, evicted := logs.prepend([]model.LogEntry{
		{Timestamp: 1, Message: "one"},
		{Timestamp: 2, Message: "two"},
	})
	if added != 2 || evicted != 2 {
		t.Fatalf("prepend() = added %d, evicted %d; want 2, 2", added, evicted)
	}
	if logs.len() != 3 || logs.firstTimestamp() != 1 || logs.lastTimestamp() != 3 {
		t.Fatalf("retained timestamps = %d–%d (%d entries), want 1–3 (3 entries)",
			logs.firstTimestamp(), logs.lastTimestamp(), logs.len())
	}
	text := logs.text("")
	if !strings.Contains(text, "one") || !strings.Contains(text, "two") || strings.Contains(text, "five") {
		t.Fatalf("text() = %q", text)
	}
}

func TestBoundedLogsAppendNewerTrimsOldestAndSuppressesBoundaryOverlap(t *testing.T) {
	logs := newBoundedLogs(3)
	logs.append([]model.LogEntry{
		{Timestamp: 1, Message: "one"},
		{Timestamp: 2, Message: "two"},
		{Timestamp: 3, Message: "three", Stream: "api"},
	})

	added, evicted := logs.appendNewer([]model.LogEntry{
		{ID: "now-has-an-id", Timestamp: 3, Message: "three", Stream: "api"},
		{ID: "four", Timestamp: 4, Message: "four", Stream: "api"},
	})
	if added != 1 || evicted != 1 {
		t.Fatalf("appendNewer() = added %d, evicted %d; want 1, 1", added, evicted)
	}
	if logs.len() != 3 || logs.firstTimestamp() != 2 || logs.lastTimestamp() != 4 {
		t.Fatalf("retained timestamps = %d–%d (%d entries), want 2–4 (3 entries)",
			logs.firstTimestamp(), logs.lastTimestamp(), logs.len())
	}
}

func TestBoundedLogsFiltersCaseInsensitively(t *testing.T) {
	logs := newBoundedLogs(10)
	logs.append([]model.LogEntry{
		{Timestamp: 1, Stream: "api", Message: "Request complete"},
		{Timestamp: 2, Stream: "worker", Message: "Job failed"},
	})

	text := logs.text("FAILED")
	if strings.Contains(text, "Request complete") || !strings.Contains(text, "Job failed") {
		t.Fatalf("text(FAILED) = %q", text)
	}
}

func TestBoundedLogsHideStreamsOnlyFromFormattedDisplay(t *testing.T) {
	logs := newBoundedLogs(10)
	logs.append([]model.LogEntry{
		{Timestamp: 1, Stream: "api", Message: "api event"},
		{Timestamp: 2, Stream: "worker", Message: "worker event"},
	})

	formatted := logs.formatVisibleWithTimestamps("", logTimestampUTC, time.Now(), map[string]struct{}{"worker": {}})
	if strings.Contains(formatted.text, "worker event") || !strings.Contains(formatted.text, "api event") {
		t.Fatalf("visible text = %q", formatted.text)
	}
	if logs.len() != 2 || len(logs.streams()) != 2 {
		t.Fatalf("stream filtering altered buffered results: %d entries, %v", logs.len(), logs.streams())
	}
}

func TestBoundedLogsDeduplicatesOverlappingPollsButKeepsLateEvents(t *testing.T) {
	logs := newBoundedLogs(10)
	first := model.LogEntry{ID: "event-1", Timestamp: 100, Message: "running"}
	late := model.LogEntry{ID: "event-2", Timestamp: 100, Message: "final line"}
	logs.append([]model.LogEntry{first})
	logs.append([]model.LogEntry{first, late})

	if logs.len() != 2 {
		t.Fatalf("len() = %d, want 2", logs.len())
	}
	if got := logs.firstTimestamp(); got != 100 {
		t.Fatalf("firstTimestamp() = %d, want 100", got)
	}
}

func TestBoundedLogsClear(t *testing.T) {
	logs := newBoundedLogs(10)
	logs.append([]model.LogEntry{{Message: "message"}})
	logs.clear()
	if logs.len() != 0 || logs.text("") != "" {
		t.Fatalf("clear left %#v", logs.entries)
	}
}

func TestSanitizeLogTextNormalizesLineEndings(t *testing.T) {
	got := sanitizeLogText("first\r\nsecond\r\n")
	if got != "first\nsecond" {
		t.Fatalf("sanitizeLogText() = %q", got)
	}
}

func TestFormattedLogsDescribeMessageAlignedWrapping(t *testing.T) {
	logs := newBoundedLogs(10)
	logs.append([]model.LogEntry{{
		Timestamp: 1,
		Stream:    "api/worker",
		Message:   "first line\nsecond line",
	}})

	formatted := logs.format("")
	if len(formatted.lines) != 1 {
		t.Fatalf("format() produced %d line spans, want 1", len(formatted.lines))
	}
	line := formatted.lines[0]
	if line.start != 0 || line.end <= line.start {
		t.Fatalf("format() span = [%d,%d)", line.start, line.end)
	}
	if !strings.HasSuffix(line.prefix, "[api/worker]  ") {
		t.Fatalf("format() prefix = %q", line.prefix)
	}
	wantContinuation := "\n" + strings.Repeat(" ", len([]rune(line.prefix))) + "second line"
	if !strings.Contains(formatted.text, wantContinuation) {
		t.Fatalf("format() did not align the explicit continuation:\n%q", formatted.text)
	}
	if len(line.segments) != 2 || line.segments[0].text != "first line" || line.segments[1].text != "second line" {
		t.Fatalf("format() message segments = %#v", line.segments)
	}
	runes := []rune(formatted.text)
	for _, segment := range line.segments {
		if got := string(runes[segment.start : segment.start+len([]rune(segment.text))]); got != segment.text {
			t.Fatalf("segment at %d points to %q, want %q", segment.start, got, segment.text)
		}
	}
	if got := string([]rune(formatted.text)[line.start:line.end]); strings.HasSuffix(got, "\n") {
		t.Fatalf("format() span includes the terminating paragraph newline: %q", got)
	}
}

func TestFormattedLogsCanOmitASingleStreamLabel(t *testing.T) {
	logs := newBoundedLogs(10)
	logs.append([]model.LogEntry{{
		Timestamp: 1,
		Stream:    "api/worker",
		Message:   "first line\nsecond line",
	}})

	formatted := logs.formatDisplayWithTimestamps("", logTimestampUTC, time.Now(), nil, false)
	if strings.Contains(formatted.text, "[api/worker]") {
		t.Fatalf("single-stream display retained its stream label: %q", formatted.text)
	}
	if len(formatted.lines) != 1 {
		t.Fatalf("format produced %d lines, want 1", len(formatted.lines))
	}
	line := formatted.lines[0]
	wantContinuation := "\n" + strings.Repeat(" ", len([]rune(line.prefix))) + "second line"
	if !strings.Contains(formatted.text, wantContinuation) {
		t.Fatalf("continuation did not align with the message after hiding its stream label: %q", formatted.text)
	}
}

func TestLogSourceStreamLabelsAreStableForViewerScope(t *testing.T) {
	for _, test := range []struct {
		name   string
		source model.LogSource
		want   bool
	}{
		{name: "entire group", source: model.LogSource{Group: "api"}, want: true},
		{name: "single stream", source: model.LogSource{Group: "api", Streams: []string{"worker"}}, want: false},
		{name: "multiple streams", source: model.LogSource{Group: "api", Streams: []string{"api", "worker"}}, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := logSourceShowsStreamLabels(test.source); got != test.want {
				t.Fatalf("logSourceShowsStreamLabels() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestFormattedLogsOffsetsAccountForUnicode(t *testing.T) {
	logs := newBoundedLogs(10)
	logs.append([]model.LogEntry{
		{Timestamp: 1, Stream: "λ", Message: "snowman ☃"},
		{Timestamp: 2, Message: "next"},
	})

	formatted := logs.format("")
	if len(formatted.lines) != 2 {
		t.Fatalf("format() produced %d line spans, want 2", len(formatted.lines))
	}
	first := formatted.lines[0]
	if formatted.lines[1].start != first.end+1 {
		t.Fatalf("second span starts at %d, want %d", formatted.lines[1].start, first.end+1)
	}
	if got := string([]rune(formatted.text)[first.start:first.end]); !strings.Contains(got, "snowman ☃") {
		t.Fatalf("first span points to %q", got)
	}
}

func TestFormattedLogHighlightsMapMessageOffsets(t *testing.T) {
	logs := newBoundedLogs(10)
	logs.append([]model.LogEntry{{
		Timestamp: 1,
		Stream:    "λ",
		Message:   "🔥 ERROR first\nsecond error",
	}})
	formatted := logs.format("")
	rules := []model.LogHighlightRule{
		{Pattern: "ERROR", Match: model.LogHighlightLiteral, Style: model.LogHighlightError},
		{Pattern: "error", Match: model.LogHighlightLiteralCI, Style: model.LogHighlightWarning},
	}
	highlights, err := formatLogHighlights(formatted, rules)
	if err != nil {
		t.Fatal(err)
	}
	if len(highlights) != 2 {
		t.Fatalf("highlights = %#v", highlights)
	}
	runes := []rune(formatted.text)
	for _, span := range highlights {
		if got := strings.ToLower(string(runes[span.start:span.end])); got != "error" {
			t.Fatalf("highlight [%d,%d) points to %q", span.start, span.end, got)
		}
	}
	if highlights[0].style != model.LogHighlightError || highlights[1].style != model.LogHighlightWarning {
		t.Fatalf("highlight styles = %#v", highlights)
	}
}

func TestFormattedLogHighlightsRejectInvalidRegex(t *testing.T) {
	_, err := formatLogHighlights(formattedLogBuffer{}, []model.LogHighlightRule{{
		Pattern: "[", Match: model.LogHighlightRegex, Style: model.LogHighlightDefault,
	}})
	if err == nil {
		t.Fatal("formatLogHighlights() error = nil")
	}
}

func TestBoundedLogsExtendedSession(t *testing.T) {
	const extendedMax = 2000
	logs := newBoundedLogs(extendedMax)
	for batch := 0; batch < 1000; batch++ {
		entries := make([]model.LogEntry, 100)
		for i := range entries {
			sequence := batch*len(entries) + i
			entries[i] = model.LogEntry{
				Timestamp: int64(sequence),
				Message:   fmt.Sprintf("event-%06d", sequence),
			}
		}
		logs.append(entries)
	}

	if logs.len() != extendedMax || cap(logs.entries) != extendedMax {
		t.Fatalf("after 100,000 events len=%d cap=%d", logs.len(), cap(logs.entries))
	}
	text := logs.text("")
	if strings.Contains(text, "event-000000") || !strings.Contains(text, "event-099999") {
		t.Fatalf("extended session retained the wrong window")
	}
}

func TestFormatLogTimestampModes(t *testing.T) {
	event := time.Date(2026, time.August, 18, 12, 0, 0, 123000000, time.UTC)
	now := event.Add(5 * time.Second)
	if got := formatLogTimestamp(event.UnixMilli(), logTimestampUTC, now); got != "2026-08-18 12:00:00.123Z" {
		t.Fatalf("UTC timestamp = %q", got)
	}
	if got := formatLogTimestamp(event.UnixMilli(), logTimestampRelative, now); got != "          5s ago" {
		t.Fatalf("relative timestamp = %q", got)
	}
}
