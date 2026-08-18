package gui

import (
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
