package gui

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dostrow/e9s/internal/model"
)

type boundedLogs struct {
	max     int
	entries []model.LogEntry
}

type formattedLogBuffer struct {
	text  string
	lines []formattedLogLine
}

type formattedLogLine struct {
	start  int
	end    int
	prefix string
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
	return b.format(filter).text
}

func (b *boundedLogs) format(filter string) formattedLogBuffer {
	filter = strings.ToLower(strings.TrimSpace(filter))
	var out strings.Builder
	formatted := formattedLogBuffer{
		lines: make([]formattedLogLine, 0, len(b.entries)),
	}
	offset := 0
	for _, entry := range b.entries {
		if filter != "" && !strings.Contains(strings.ToLower(entry.Message+" "+entry.Stream), filter) {
			continue
		}
		timestamp := time.UnixMilli(entry.Timestamp).Local().Format("2006-01-02 15:04:05.000")
		prefix := timestamp + "  "
		if entry.Stream != "" {
			prefix += "[" + entry.Stream + "]  "
		}
		message := sanitizeLogText(entry.Message)
		if strings.Contains(message, "\n") {
			message = strings.ReplaceAll(message, "\n", "\n"+strings.Repeat(" ", utf8.RuneCountInString(prefix)))
		}
		line := prefix + message
		lineLength := utf8.RuneCountInString(line)
		formatted.lines = append(formatted.lines, formattedLogLine{
			start:  offset,
			end:    offset + lineLength,
			prefix: prefix,
		})
		out.WriteString(line)
		out.WriteByte('\n')
		offset += lineLength + 1
	}
	formatted.text = out.String()
	return formatted
}

func sanitizeLogText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimRight(value, "\n ")
}
