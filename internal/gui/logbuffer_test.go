package gui

import (
	"fmt"
	"strings"
	"testing"

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
	if got := string([]rune(formatted.text)[line.start:line.end]); strings.HasSuffix(got, "\n") {
		t.Fatalf("format() span includes the terminating paragraph newline: %q", got)
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
