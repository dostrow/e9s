package service

import (
	"context"
	"errors"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

type fakeLogsAPI struct {
	called string
	limit  int
	err    error
}

func (f *fakeLogsAPI) ListLogGroups(ctx context.Context, search string) ([]model.LogGroup, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.called = "list-groups:" + search
	return []model.LogGroup{{Name: "/aws/ecs/api", StoredBytes: 42}}, f.err
}

func (f *fakeLogsAPI) ListLogStreams(ctx context.Context, group, prefix string) ([]model.LogStream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.called = "list-streams:" + group + ":" + prefix
	return []model.LogStream{{Name: "ecs/api/one", LastEventTime: 42}}, f.err
}

func (f *fakeLogsAPI) SearchLogs(ctx context.Context, _ string, streams []string, _ string, _, _ int64, limit int) ([]model.LogEntry, error) {
	called := "search-group"
	if len(streams) == 1 {
		called = "search-stream"
	} else if len(streams) > 1 {
		called = "search-streams"
	}
	return f.rangeResult(ctx, called, limit)
}

func (f *fakeLogsAPI) SearchMultiGroupLogs(ctx context.Context, _ []string, _ string, _, _ int64, limit int) ([]model.LogEntry, error) {
	return f.rangeResult(ctx, "search-groups", limit)
}

func (f *fakeLogsAPI) result(ctx context.Context, called string, limit int) ([]model.LogEntry, int64, error) {
	f.called, f.limit = called, limit
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	return []model.LogEntry{{Timestamp: 42, Message: called}}, 42, f.err
}

func (f *fakeLogsAPI) rangeResult(ctx context.Context, called string, limit int) ([]model.LogEntry, error) {
	entries, _, err := f.result(ctx, called, limit)
	return entries, err
}

func (f *fakeLogsAPI) FetchLogs(ctx context.Context, _ string, _ string, _ int64, limit int) ([]model.LogEntry, int64, error) {
	return f.result(ctx, "fetch-stream", limit)
}
func (f *fakeLogsAPI) FetchMultiStreamLogs(ctx context.Context, _ string, _ []string, _ int64, limit int) ([]model.LogEntry, int64, error) {
	return f.result(ctx, "fetch-streams", limit)
}
func (f *fakeLogsAPI) FetchLogGroup(ctx context.Context, _ string, _ int64, limit int) ([]model.LogEntry, int64, error) {
	return f.result(ctx, "fetch-group", limit)
}
func (f *fakeLogsAPI) FetchLogsRange(ctx context.Context, _ string, _ string, _, _ int64, limit int) ([]model.LogEntry, error) {
	return f.rangeResult(ctx, "range-stream", limit)
}
func (f *fakeLogsAPI) FetchMultiStreamLogsRange(ctx context.Context, _ string, _ []string, _, _ int64, limit int) ([]model.LogEntry, error) {
	return f.rangeResult(ctx, "range-streams", limit)
}
func (f *fakeLogsAPI) FetchLogGroupRange(ctx context.Context, _ string, _, _ int64, limit int) ([]model.LogEntry, error) {
	return f.rangeResult(ctx, "range-group", limit)
}
func (f *fakeLogsAPI) FetchMultiGroupRange(ctx context.Context, _ []string, _, _ int64, limit int) ([]model.LogEntry, error) {
	return f.rangeResult(ctx, "range-groups", limit)
}
func (f *fakeLogsAPI) TailLogs(ctx context.Context, _ string, _ string, _ int64, limit int) ([]model.LogEntry, int64, error) {
	return f.result(ctx, "tail-stream", limit)
}
func (f *fakeLogsAPI) TailMultiStreamLogs(ctx context.Context, _ string, _ []string, _ int64, limit int) ([]model.LogEntry, int64, error) {
	return f.result(ctx, "tail-streams", limit)
}
func (f *fakeLogsAPI) TailLogGroup(ctx context.Context, _ string, _ int64, limit int) ([]model.LogEntry, int64, error) {
	return f.result(ctx, "tail-group", limit)
}

func TestLogsDispatchesQueries(t *testing.T) {
	tests := []struct {
		name  string
		query model.LogQuery
		want  string
	}{
		{name: "group", query: model.LogQuery{}, want: "fetch-group"},
		{name: "stream", query: model.LogQuery{Streams: []string{"one"}}, want: "fetch-stream"},
		{name: "streams", query: model.LogQuery{Streams: []string{"one", "two"}}, want: "fetch-streams"},
		{name: "tail group", query: model.LogQuery{Tail: true}, want: "tail-group"},
		{name: "tail stream", query: model.LogQuery{Tail: true, Streams: []string{"one"}}, want: "tail-stream"},
		{name: "tail streams", query: model.LogQuery{Tail: true, Streams: []string{"one", "two"}}, want: "tail-streams"},
		{name: "range group", query: model.LogQuery{EndTime: 100}, want: "range-group"},
		{name: "range stream", query: model.LogQuery{EndTime: 100, Streams: []string{"one"}}, want: "range-stream"},
		{name: "range streams", query: model.LogQuery{EndTime: 100, Streams: []string{"one", "two"}}, want: "range-streams"},
		{name: "range groups", query: model.LogQuery{Groups: []string{"one", "two"}}, want: "range-groups"},
		{name: "search group", query: model.LogQuery{Filter: `"error"`}, want: "search-group"},
		{name: "search stream", query: model.LogQuery{Filter: `"error"`, Streams: []string{"one"}}, want: "search-stream"},
		{name: "search streams", query: model.LogQuery{Filter: `"error"`, Streams: []string{"one", "two"}}, want: "search-streams"},
		{name: "search groups", query: model.LogQuery{Filter: `"error"`, Groups: []string{"one", "two"}}, want: "search-groups"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := &fakeLogsAPI{}
			page, err := NewLogs(api).Fetch(context.Background(), "group", tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if api.called != tt.want {
				t.Fatalf("called %q, want %q", api.called, tt.want)
			}
			if api.limit != 100 {
				t.Fatalf("default limit = %d, want 100", api.limit)
			}
			if len(page.Entries) != 1 || page.LastTimestamp != 42 {
				t.Fatalf("page = %#v", page)
			}
		})
	}
}

func TestLogsPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewLogs(&fakeLogsAPI{}).Fetch(ctx, "group", model.LogQuery{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Fetch() error = %v, want context.Canceled", err)
	}
}

func TestLogsRequiresGroup(t *testing.T) {
	_, err := NewLogs(&fakeLogsAPI{}).Fetch(context.Background(), "", model.LogQuery{})
	if err == nil {
		t.Fatal("Fetch() error = nil, want validation error")
	}
}

func TestLogsBrowsesGroupsAndStreams(t *testing.T) {
	api := &fakeLogsAPI{}
	logs := NewLogs(api)

	groups, err := logs.ListGroups(context.Background(), "/aws/ecs")
	if err != nil {
		t.Fatal(err)
	}
	if api.called != "list-groups:/aws/ecs" || len(groups) != 1 || groups[0].Name != "/aws/ecs/api" {
		t.Fatalf("groups = %#v, called = %q", groups, api.called)
	}

	streams, err := logs.ListStreams(context.Background(), "/aws/ecs/api", "ecs/")
	if err != nil {
		t.Fatal(err)
	}
	if api.called != "list-streams:/aws/ecs/api:ecs/" || len(streams) != 1 || streams[0].Name != "ecs/api/one" {
		t.Fatalf("streams = %#v, called = %q", streams, api.called)
	}
}

func TestLogsRequiresGroupForStreamBrowsing(t *testing.T) {
	_, err := NewLogs(&fakeLogsAPI{}).ListStreams(context.Background(), "", "")
	if err == nil {
		t.Fatal("ListStreams() error = nil, want validation error")
	}
}
