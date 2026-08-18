package service

import (
	"context"
	"fmt"

	"github.com/dostrow/e9s/internal/model"
)

// LogsAPI is the low-level CloudWatch Logs behavior used by the shared query
// dispatcher. The AWS client satisfies this interface.
type LogsAPI interface {
	ListLogGroups(context.Context, string) ([]model.LogGroup, error)
	ListLogStreams(context.Context, string, string) ([]model.LogStream, error)
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
}

// Logs routes a UI-neutral query to the appropriate CloudWatch operation.
type Logs struct {
	api LogsAPI
}

func NewLogs(api LogsAPI) *Logs {
	return &Logs{api: api}
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

	if query.EndTime > 0 || len(query.Groups) > 1 {
		entries, err := s.fetchRange(ctx, group, query)
		if err != nil {
			return model.LogPage{}, fmt.Errorf("fetch log range: %w", err)
		}
		return page(entries, query.StartTime), nil
	}

	var (
		entries []model.LogEntry
		lastTS  int64
		err     error
	)
	if query.Tail {
		switch len(query.Streams) {
		case 0:
			entries, lastTS, err = s.api.TailLogGroup(ctx, group, query.StartTime, query.Limit)
		case 1:
			entries, lastTS, err = s.api.TailLogs(ctx, group, query.Streams[0], query.StartTime, query.Limit)
		default:
			entries, lastTS, err = s.api.TailMultiStreamLogs(ctx, group, query.Streams, query.StartTime, query.Limit)
		}
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
	return model.LogPage{Entries: entries, LastTimestamp: lastTS}, nil
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
