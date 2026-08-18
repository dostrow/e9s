package model

// LogEntry is a UI-neutral CloudWatch log event.
type LogEntry struct {
	Timestamp int64
	Message   string
	Stream    string
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
}

// LogPage is the result of a LogQuery. LastTimestamp is StartTime when no
// entries were returned.
type LogPage struct {
	Entries       []LogEntry
	LastTimestamp int64
}
