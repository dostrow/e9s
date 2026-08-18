package model

// LogHighlightMatch describes how a log highlight pattern is interpreted.
type LogHighlightMatch string

const (
	LogHighlightLiteral   LogHighlightMatch = "literal"
	LogHighlightLiteralCI LogHighlightMatch = "literal_ci"
	LogHighlightRegex     LogHighlightMatch = "regex"
)

// LogHighlightStyle is a semantic presentation hint shared by every frontend.
// Frontends map these values onto their own theme-aware visual styles.
type LogHighlightStyle string

const (
	LogHighlightDefault LogHighlightStyle = "highlight"
	LogHighlightInfo    LogHighlightStyle = "info"
	LogHighlightSuccess LogHighlightStyle = "success"
	LogHighlightWarning LogHighlightStyle = "warning"
	LogHighlightError   LogHighlightStyle = "error"
)

// LogHighlightRule is a portable, presentation-only rule for emphasizing
// matching spans in log messages.
type LogHighlightRule struct {
	Pattern string            `yaml:"pattern"`
	Match   LogHighlightMatch `yaml:"match"`
	Style   LogHighlightStyle `yaml:"style"`
}

// LogHighlightSpan identifies a matched rune range in a log message.
type LogHighlightSpan struct {
	Start int
	End   int
	Style LogHighlightStyle
}

// LogGroup is the UI-neutral summary used by log browsers.
type LogGroup struct {
	Name        string
	StoredBytes int64
	StreamCount int
}

// LogStream is the UI-neutral summary used by stream browsers.
type LogStream struct {
	Name           string
	LastEventTime  int64
	FirstEventTime int64
}

// LogEntry is a UI-neutral CloudWatch log event.
type LogEntry struct {
	ID        string
	Timestamp int64
	Message   string
	Stream    string
}

// LogEntryKey is a stable, comparable identity used to suppress overlapping
// results when CloudWatch polling keeps its timestamp cursor inclusive.
type LogEntryKey struct {
	ID        string
	Timestamp int64
	Message   string
	Stream    string
}

func (e LogEntry) Key() LogEntryKey {
	if e.ID != "" {
		return LogEntryKey{ID: e.ID, Stream: e.Stream}
	}
	return LogEntryKey{Timestamp: e.Timestamp, Message: e.Message, Stream: e.Stream}
}

// LogSource identifies one CloudWatch log group and an optional set of streams.
// An empty Streams slice means the entire group.
type LogSource struct {
	Group   string
	Streams []string
}

// LogQuery describes a single, cancellable log read. EndTime is zero for an
// open-ended fetch. Tail requests the pagination behavior used by live views.
type LogQuery struct {
	Groups    []string
	Streams   []string
	StartTime int64
	EndTime   int64
	Limit     int
	Tail      bool
	Filter    string
	// FallbackLimit requests the newest N entries without the time constraint
	// when the primary query returns no events.
	FallbackLimit int
}

// LogPage is the result of a LogQuery. LastTimestamp is StartTime when no
// entries were returned.
type LogPage struct {
	Entries       []LogEntry
	LastTimestamp int64
	UsedFallback  bool
}
