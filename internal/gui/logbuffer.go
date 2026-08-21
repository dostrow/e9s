package gui

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/highlight"
	"github.com/dostrow/e9s/internal/model"
)

type boundedLogs struct {
	max     int
	entries []model.LogEntry
	seen    map[model.LogEntryKey]struct{}
}

type formattedLogBuffer struct {
	text  string
	lines []formattedLogLine
}

type formattedLogLine struct {
	start    int
	end      int
	prefix   string
	entry    model.LogEntry
	segments []formattedLogSegment
}

type formattedLogSegment struct {
	start int
	text  string
}

type formattedLogHighlight struct {
	start int
	end   int
	style model.LogHighlightStyle
}

type logTimestampMode int

const (
	logTimestampLocal logTimestampMode = iota
	logTimestampUTC
	logTimestampRelative
)

func newBoundedLogs(maxEntries int) *boundedLogs {
	if maxEntries <= 0 {
		maxEntries = config.DefaultMaxLogLines
	}
	return &boundedLogs{max: maxEntries}
}

func (b *boundedLogs) append(entries []model.LogEntry) (added, evicted int) {
	b.ensureSeen()
	for _, entry := range entries {
		key := entry.Key()
		if _, exists := b.seen[key]; exists {
			continue
		}
		b.seen[key] = struct{}{}
		b.entries = append(b.entries, entry)
		added++
	}
	sort.SliceStable(b.entries, func(i, j int) bool {
		return b.entries[i].Timestamp < b.entries[j].Timestamp
	})
	if extra := len(b.entries) - b.max; extra > 0 {
		evicted = extra
		for _, entry := range b.entries[:extra] {
			delete(b.seen, entry.Key())
		}
		kept := make([]model.LogEntry, b.max)
		copy(kept, b.entries[extra:])
		b.entries = kept
	}
	return added, evicted
}

// appendNewer appends a forward page while suppressing the inclusive boundary
// overlap. CloudWatch's stream API does not expose event IDs, while its group
// API does, so boundary records are also compared by their rendered identity.
func (b *boundedLogs) appendNewer(entries []model.LogEntry) (added, evicted int) {
	boundary := b.lastTimestamp()
	type contentKey struct {
		timestamp int64
		message   string
		stream    string
	}
	existingBoundary := make(map[contentKey]struct{})
	for _, entry := range b.entries {
		if entry.Timestamp == boundary {
			existingBoundary[contentKey{entry.Timestamp, entry.Message, entry.Stream}] = struct{}{}
		}
	}
	filtered := make([]model.LogEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Timestamp == boundary {
			key := contentKey{entry.Timestamp, entry.Message, entry.Stream}
			if _, exists := existingBoundary[key]; exists {
				continue
			}
		}
		filtered = append(filtered, entry)
	}
	return b.append(filtered)
}

func (b *boundedLogs) prepend(entries []model.LogEntry) (added, evicted int) {
	b.ensureSeen()
	older := make([]model.LogEntry, 0, len(entries))
	for _, entry := range entries {
		key := entry.Key()
		if _, exists := b.seen[key]; exists {
			continue
		}
		b.seen[key] = struct{}{}
		older = append(older, entry)
	}
	if len(older) == 0 {
		return 0, 0
	}
	added = len(older)
	b.entries = append(older, b.entries...)
	sort.SliceStable(b.entries, func(i, j int) bool {
		return b.entries[i].Timestamp < b.entries[j].Timestamp
	})
	if extra := len(b.entries) - b.max; extra > 0 {
		evicted = extra
		for _, entry := range b.entries[len(b.entries)-extra:] {
			delete(b.seen, entry.Key())
		}
		b.entries = b.entries[:len(b.entries)-extra]
	}
	return added, evicted
}

func (b *boundedLogs) ensureSeen() {
	if b.seen != nil {
		return
	}
	b.seen = make(map[model.LogEntryKey]struct{}, len(b.entries))
	for _, entry := range b.entries {
		b.seen[entry.Key()] = struct{}{}
	}
}

func (b *boundedLogs) firstTimestamp() int64 {
	if len(b.entries) == 0 {
		return 0
	}
	first := b.entries[0].Timestamp
	for _, entry := range b.entries[1:] {
		if entry.Timestamp < first {
			first = entry.Timestamp
		}
	}
	return first
}

func (b *boundedLogs) lastTimestamp() int64 {
	if len(b.entries) == 0 {
		return 0
	}
	last := b.entries[0].Timestamp
	for _, entry := range b.entries[1:] {
		if entry.Timestamp > last {
			last = entry.Timestamp
		}
	}
	return last
}

func (b *boundedLogs) clear() {
	b.entries = nil
	b.seen = nil
}

func (b *boundedLogs) len() int {
	return len(b.entries)
}

func (b *boundedLogs) streams() []string {
	seen := make(map[string]struct{})
	for _, entry := range b.entries {
		if entry.Stream != "" {
			seen[entry.Stream] = struct{}{}
		}
	}
	streams := make([]string, 0, len(seen))
	for stream := range seen {
		streams = append(streams, stream)
	}
	sort.Strings(streams)
	return streams
}

func (b *boundedLogs) text(filter string) string {
	return b.format(filter).text
}

func (b *boundedLogs) format(filter string) formattedLogBuffer {
	return b.formatWithTimestamps(filter, logTimestampLocal, time.Now())
}

func (b *boundedLogs) formatWithTimestamps(filter string, mode logTimestampMode, now time.Time) formattedLogBuffer {
	return b.formatVisibleWithTimestamps(filter, mode, now, nil)
}

func (b *boundedLogs) formatVisibleWithTimestamps(filter string, mode logTimestampMode, now time.Time, hiddenStreams map[string]struct{}) formattedLogBuffer {
	return b.formatDisplayWithTimestamps(filter, mode, now, hiddenStreams, true)
}

func (b *boundedLogs) formatDisplayWithTimestamps(filter string, mode logTimestampMode, now time.Time, hiddenStreams map[string]struct{}, showStreams bool) formattedLogBuffer {
	filter = strings.ToLower(strings.TrimSpace(filter))
	var out strings.Builder
	formatted := formattedLogBuffer{
		lines: make([]formattedLogLine, 0, len(b.entries)),
	}
	offset := 0
	for _, entry := range b.entries {
		if _, hidden := hiddenStreams[entry.Stream]; hidden {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(entry.Message+" "+entry.Stream), filter) {
			continue
		}
		timestamp := formatLogTimestamp(entry.Timestamp, mode, now)
		prefix := timestamp + "  "
		if showStreams && entry.Stream != "" {
			prefix += "[" + entry.Stream + "]  "
		}
		message := sanitizeLogText(entry.Message)
		parts := strings.Split(message, "\n")
		var lineBuilder strings.Builder
		lineBuilder.WriteString(prefix)
		segments := make([]formattedLogSegment, 0, len(parts))
		lineOffset := utf8.RuneCountInString(prefix)
		for i, part := range parts {
			if i > 0 {
				indent := strings.Repeat(" ", utf8.RuneCountInString(prefix))
				lineBuilder.WriteByte('\n')
				lineBuilder.WriteString(indent)
				lineOffset += 1 + utf8.RuneCountInString(indent)
			}
			segments = append(segments, formattedLogSegment{start: offset + lineOffset, text: part})
			lineBuilder.WriteString(part)
			lineOffset += utf8.RuneCountInString(part)
		}
		line := lineBuilder.String()
		lineLength := utf8.RuneCountInString(line)
		formatted.lines = append(formatted.lines, formattedLogLine{
			start:    offset,
			end:      offset + lineLength,
			prefix:   prefix,
			entry:    entry,
			segments: segments,
		})
		out.WriteString(line)
		out.WriteByte('\n')
		offset += lineLength + 1
	}
	formatted.text = out.String()
	return formatted
}

// An empty stream list means the entire group, whose stream count can change
// while following. Only an explicitly scoped single-stream source can safely
// omit its redundant source label without changing layout as events arrive.
func logSourceShowsStreamLabels(source model.LogSource) bool {
	return len(source.Streams) != 1
}

func formatLogHighlights(formatted formattedLogBuffer, rules []model.LogHighlightRule) ([]formattedLogHighlight, error) {
	matcher, err := highlight.Compile(rules)
	if err != nil {
		return nil, err
	}
	var highlights []formattedLogHighlight
	for _, line := range formatted.lines {
		for _, segment := range line.segments {
			for _, span := range matcher.Spans(segment.text) {
				highlights = append(highlights, formattedLogHighlight{
					start: segment.start + span.Start,
					end:   segment.start + span.End,
					style: span.Style,
				})
			}
		}
	}
	return highlights, nil
}

func formatLogTimestamp(timestamp int64, mode logTimestampMode, now time.Time) string {
	eventTime := time.UnixMilli(timestamp)
	switch mode {
	case logTimestampUTC:
		return eventTime.UTC().Format("2006-01-02 15:04:05.000Z")
	case logTimestampRelative:
		age := now.Sub(eventTime)
		if age < 0 {
			age = 0
		}
		return fmt.Sprintf("%12s ago", age.Round(time.Millisecond))
	default:
		return eventTime.Local().Format("2006-01-02 15:04:05.000")
	}
}

func sanitizeLogText(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.TrimRight(value, "\n ")
}
