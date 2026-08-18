package gui

import (
	"fmt"
	"strings"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

type boundedLogs struct {
	max     int
	entries []model.LogEntry
}

func newBoundedLogs(maxEntries int) *boundedLogs {
	if maxEntries <= 0 {
		maxEntries = 2000
	}
	return &boundedLogs{max: maxEntries}
}

func (b *boundedLogs) append(entries []model.LogEntry) {
	b.entries = append(b.entries, entries...)
	if extra := len(b.entries) - b.max; extra > 0 {
		kept := make([]model.LogEntry, b.max)
		copy(kept, b.entries[extra:])
		b.entries = kept
	}
}

func (b *boundedLogs) clear() {
	b.entries = nil
}

func (b *boundedLogs) len() int {
	return len(b.entries)
}

func (b *boundedLogs) text(filter string) string {
	filter = strings.ToLower(strings.TrimSpace(filter))
	var out strings.Builder
	for _, entry := range b.entries {
		if filter != "" && !strings.Contains(strings.ToLower(entry.Message+" "+entry.Stream), filter) {
			continue
		}
		timestamp := time.UnixMilli(entry.Timestamp).Local().Format("2006-01-02 15:04:05.000")
		if entry.Stream == "" {
			fmt.Fprintf(&out, "%s  %s\n", timestamp, sanitizeLogText(entry.Message))
		} else {
			fmt.Fprintf(&out, "%s  [%s]  %s\n", timestamp, entry.Stream, sanitizeLogText(entry.Message))
		}
	}
	return out.String()
}

func sanitizeLogText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimRight(value, "\n ")
}
