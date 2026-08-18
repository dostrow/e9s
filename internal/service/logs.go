package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

// LogsAPI is the low-level CloudWatch Logs behavior used by the shared query
// dispatcher. The AWS client satisfies this interface.
type LogsAPI interface {
	ListLogGroups(context.Context, string) ([]model.LogGroup, error)
	ListLogStreams(context.Context, string, string) ([]model.LogStream, error)
	SearchLogs(context.Context, string, []string, string, int64, int64, int) ([]model.LogEntry, error)
	SearchMultiGroupLogs(context.Context, []string, string, int64, int64, int) ([]model.LogEntry, error)
	FetchLogs(context.Context, string, string, int64, int) ([]model.LogEntry, int64, error)
	FetchMultiStreamLogs(context.Context, string, []string, int64, int) ([]model.LogEntry, int64, error)
	FetchLogGroup(context.Context, string, int64, int) ([]model.LogEntry, int64, error)
	FetchLogsRange(context.Context, string, string, int64, int64, int) ([]model.LogEntry, error)
	FetchMultiStreamLogsRange(context.Context, string, []string, int64, int64, int) ([]model.LogEntry, error)
	FetchLogGroupRange(context.Context, string, int64, int64, int) ([]model.LogEntry, error)
	FetchMultiGroupRange(context.Context, []string, int64, int64, int) ([]model.LogEntry, error)
	TailLogs(context.Context, string, string, int64, int) ([]model.LogEntry, int64, error)
	TailMultiStreamLogs(context.Context, string, []string, int64, int) ([]model.LogEntry, int64, error)
	TailLogGroup(context.Context, string, int64, int) ([]model.LogEntry, int64, error)
	FetchEarlierLogs(context.Context, string, []string, int64, int) ([]model.LogEntry, error)
}

// Logs routes a UI-neutral query to the appropriate CloudWatch operation.
type Logs struct {
	api LogsAPI
}

func NewLogs(api LogsAPI) *Logs {
	return &Logs{api: api}
}

// NormalizeFilterPattern turns plain text into a valid CloudWatch literal
// filter while preserving JSON, space-delimited, and quoted expressions.
func NormalizeFilterPattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return ""
	}
	if pattern[0] == '{' || pattern[0] == '[' {
		return pattern
	}
	if pattern[0] == '"' && len(pattern) > 1 && pattern[len(pattern)-1] == '"' &&
		!strings.Contains(pattern[1:len(pattern)-1], `"`) {
		return pattern
	}
	return `"` + strings.ReplaceAll(pattern, `"`, "") + `"`
}

func (s *Logs) ListGroups(ctx context.Context, search string) ([]model.LogGroup, error) {
	groups, err := s.api.ListLogGroups(ctx, search)
	if err != nil {
		return nil, fmt.Errorf("list CloudWatch log groups: %w", err)
	}
	return groups, nil
}

func (s *Logs) ListStreams(ctx context.Context, group, prefix string) ([]model.LogStream, error) {
	if group == "" {
		return nil, fmt.Errorf("list CloudWatch log streams: log group is required")
	}
	streams, err := s.api.ListLogStreams(ctx, group, prefix)
	if err != nil {
		return nil, fmt.Errorf("list CloudWatch log streams for %q: %w", group, err)
	}
	return streams, nil
}

func (s *Logs) Fetch(ctx context.Context, group string, query model.LogQuery) (model.LogPage, error) {
	if group == "" && len(query.Groups) == 0 {
		return model.LogPage{}, fmt.Errorf("fetch logs: log group is required")
	}
	if query.Limit <= 0 {
		query.Limit = 100
	}
	if query.Filter != "" {
		return s.search(ctx, group, query)
	}
	if query.BeforeTime > 0 {
		entries, err := s.api.FetchEarlierLogs(ctx, group, query.Streams, query.BeforeTime, query.Limit)
		if err != nil {
			return model.LogPage{}, fmt.Errorf("fetch logs before %d from %q: %w", query.BeforeTime, group, err)
		}
		return page(entries, 0), nil
	}

	if query.EndTime > 0 || len(query.Groups) > 1 {
		entries, err := s.fetchRange(ctx, group, query)
		if err != nil {
			return model.LogPage{}, fmt.Errorf("fetch log range: %w", err)
		}
		return s.withRecentFallback(ctx, group, query, page(entries, query.StartTime))
	}

	var (
		entries []model.LogEntry
		lastTS  int64
		err     error
	)
	if query.Tail {
		entries, lastTS, err = s.tail(ctx, group, query.Streams, query.StartTime, query.Limit)
	} else {
		switch len(query.Streams) {
		case 0:
			entries, lastTS, err = s.api.FetchLogGroup(ctx, group, query.StartTime, query.Limit)
		case 1:
			entries, lastTS, err = s.api.FetchLogs(ctx, group, query.Streams[0], query.StartTime, query.Limit)
		default:
			entries, lastTS, err = s.api.FetchMultiStreamLogs(ctx, group, query.Streams, query.StartTime, query.Limit)
		}
	}
	if err != nil {
		return model.LogPage{}, fmt.Errorf("fetch logs from %q: %w", group, err)
	}
	return s.withRecentFallback(ctx, group, query, model.LogPage{Entries: entries, LastTimestamp: lastTS})
}

func (s *Logs) withRecentFallback(ctx context.Context, group string, query model.LogQuery, primary model.LogPage) (model.LogPage, error) {
	if len(primary.Entries) > 0 || query.FallbackLimit <= 0 || query.StartTime <= 0 || len(query.Groups) > 1 {
		return primary, nil
	}
	if len(query.Groups) == 1 {
		group = query.Groups[0]
	}
	entries, _, err := s.tail(ctx, group, query.Streams, 0, query.FallbackLimit)
	if err != nil {
		return model.LogPage{}, fmt.Errorf("fetch latest logs from %q: %w", group, err)
	}
	if len(entries) == 0 {
		return primary, nil
	}
	fallback := page(entries, 0)
	fallback.UsedFallback = true
	return fallback, nil
}

func (s *Logs) tail(ctx context.Context, group string, streams []string, startTime int64, limit int) ([]model.LogEntry, int64, error) {
	switch len(streams) {
	case 0:
		return s.api.TailLogGroup(ctx, group, startTime, limit)
	case 1:
		return s.api.TailLogs(ctx, group, streams[0], startTime, limit)
	default:
		return s.api.TailMultiStreamLogs(ctx, group, streams, startTime, limit)
	}
}

func (s *Logs) search(ctx context.Context, group string, query model.LogQuery) (model.LogPage, error) {
	var (
		entries []model.LogEntry
		err     error
	)
	if len(query.Groups) > 1 {
		entries, err = s.api.SearchMultiGroupLogs(ctx, query.Groups, query.Filter,
			query.StartTime, query.EndTime, query.Limit)
	} else {
		if len(query.Groups) == 1 {
			group = query.Groups[0]
		}
		entries, err = s.api.SearchLogs(ctx, group, query.Streams, query.Filter,
			query.StartTime, query.EndTime, query.Limit)
	}
	if err != nil {
		return model.LogPage{}, fmt.Errorf("search CloudWatch logs: %w", err)
	}
	return page(entries, query.StartTime), nil
}

func (s *Logs) fetchRange(ctx context.Context, group string, query model.LogQuery) ([]model.LogEntry, error) {
	if len(query.Groups) > 1 {
		return s.api.FetchMultiGroupRange(ctx, query.Groups, query.StartTime, query.EndTime, query.Limit)
	}
	if len(query.Groups) == 1 {
		group = query.Groups[0]
	}
	switch len(query.Streams) {
	case 0:
		return s.api.FetchLogGroupRange(ctx, group, query.StartTime, query.EndTime, query.Limit)
	case 1:
		return s.api.FetchLogsRange(ctx, group, query.Streams[0], query.StartTime, query.EndTime, query.Limit)
	default:
		return s.api.FetchMultiStreamLogsRange(ctx, group, query.Streams, query.StartTime, query.EndTime, query.Limit)
	}
}

func page(entries []model.LogEntry, fallback int64) model.LogPage {
	lastTS := fallback
	for _, entry := range entries {
		if entry.Timestamp > lastTS {
			lastTS = entry.Timestamp
		}
	}
	return model.LogPage{Entries: entries, LastTimestamp: lastTS}
}
