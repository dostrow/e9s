package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

type fakeFilterLogEventsAPI struct {
	t      *testing.T
	pages  []*cloudwatchlogs.FilterLogEventsOutput
	inputs []*cloudwatchlogs.FilterLogEventsInput
}

type fakeGetLogEventsAPI struct {
	outputs []*cloudwatchlogs.GetLogEventsOutput
	inputs  []*cloudwatchlogs.GetLogEventsInput
}

func (f *fakeGetLogEventsAPI) GetLogEvents(_ context.Context, input *cloudwatchlogs.GetLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	cloned := *input
	if input.NextToken != nil {
		cloned.NextToken = strPtrLogs(*input.NextToken)
	}
	f.inputs = append(f.inputs, &cloned)
	if len(f.outputs) == 0 {
		return &cloudwatchlogs.GetLogEventsOutput{}, nil
	}
	out := f.outputs[0]
	f.outputs = f.outputs[1:]
	return out, nil
}

func (f *fakeFilterLogEventsAPI) FilterLogEvents(_ context.Context, input *cloudwatchlogs.FilterLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	f.inputs = append(f.inputs, cloneFilterInput(input))
	if len(f.pages) == 0 {
		f.t.Fatal("unexpected FilterLogEvents call")
	}
	page := f.pages[0]
	f.pages = f.pages[1:]
	return page, nil
}

func TestTailLogsPaginatesAcrossPages(t *testing.T) {
	api := &fakeFilterLogEventsAPI{
		t: t,
		pages: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwltypes.FilteredLogEvent{
					logEvent(1000, "first"),
					logEvent(1000, "second"),
				},
				NextToken: strPtrLogs("page-2"),
			},
			{
				Events: []cwltypes.FilteredLogEvent{
					logEvent(1000, "third"),
					logEvent(1001, "fourth"),
				},
			},
		},
	}

	logGroup := "/aws/ecs/example"
	stream := "ecs/app/task"
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:   &logGroup,
		LogStreamNames: []string{stream},
		StartTime:      int64PtrLogs(900),
	}

	entries, lastTS, err := tailLogs(context.Background(), api, input, 900, 10)
	if err != nil {
		t.Fatalf("tailLogs returned error: %v", err)
	}

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if entries[2].Message != "third" || entries[3].Message != "fourth" {
		t.Fatalf("unexpected entry order: %#v", entries)
	}
	if entries[3].ID != "fourth" {
		t.Fatalf("event ID = %q, want fourth", entries[3].ID)
	}
	if lastTS != 1001 {
		t.Fatalf("lastTS = %d, want 1001", lastTS)
	}

	if got := len(api.inputs); got != 2 {
		t.Fatalf("expected 2 FilterLogEvents calls, got %d", got)
	}
	if api.inputs[0].StartTime == nil || *api.inputs[0].StartTime != 900 {
		t.Fatalf("first request start time = %v, want 900", api.inputs[0].StartTime)
	}
	if api.inputs[1].NextToken == nil || *api.inputs[1].NextToken != "page-2" {
		t.Fatalf("second request next token = %v, want page-2", api.inputs[1].NextToken)
	}
}

func TestTailLogsKeepsNewestEntriesWhenWindowExceedsLimit(t *testing.T) {
	api := &fakeFilterLogEventsAPI{
		t: t,
		pages: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwltypes.FilteredLogEvent{
					logEvent(1, "one"),
					logEvent(2, "two"),
					logEvent(3, "three"),
				},
				NextToken: strPtrLogs("page-2"),
			},
			{
				Events: []cwltypes.FilteredLogEvent{
					logEvent(4, "four"),
					logEvent(5, "five"),
				},
			},
		},
	}

	logGroup := "/aws/ecs/example"
	input := &cloudwatchlogs.FilterLogEventsInput{LogGroupName: &logGroup}

	entries, lastTS, err := tailLogs(context.Background(), api, input, 0, 3)
	if err != nil {
		t.Fatalf("tailLogs returned error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Message != "three" || entries[1].Message != "four" || entries[2].Message != "five" {
		t.Fatalf("expected newest entries to be retained, got %#v", entries)
	}
	if lastTS != 5 {
		t.Fatalf("lastTS = %d, want 5", lastTS)
	}
}

func TestNewestStreamLogsWalksBackwardAcrossPartialAndEmptyPages(t *testing.T) {
	api := &fakeGetLogEventsAPI{
		outputs: []*cloudwatchlogs.GetLogEventsOutput{
			{NextBackwardToken: strPtrLogs("back-1")},
			{
				Events: []cwltypes.OutputLogEvent{
					{Timestamp: int64PtrLogs(1002), Message: strPtrLogs("newer")},
					{Timestamp: int64PtrLogs(1003), Message: strPtrLogs("newest")},
				},
				NextBackwardToken: strPtrLogs("back-2"),
			},
			{Events: []cwltypes.OutputLogEvent{
				{Timestamp: int64PtrLogs(1000), Message: strPtrLogs("oldest")},
				{Timestamp: int64PtrLogs(1001), Message: strPtrLogs("older")},
			}},
		},
	}

	entries, lastTS, err := newestStreamLogs(context.Background(), api, "group", "stream", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(api.inputs) != 3 {
		t.Fatalf("requests = %d, want 3", len(api.inputs))
	}
	if api.inputs[0].StartFromHead == nil || *api.inputs[0].StartFromHead {
		t.Fatalf("StartFromHead = %v, want false", api.inputs[0].StartFromHead)
	}
	if api.inputs[0].Limit == nil || *api.inputs[0].Limit != 3 {
		t.Fatalf("Limit = %v, want 3", api.inputs[0].Limit)
	}
	if api.inputs[1].NextToken == nil || *api.inputs[1].NextToken != "back-1" ||
		api.inputs[2].NextToken == nil || *api.inputs[2].NextToken != "back-2" {
		t.Fatalf("backward tokens = %v, %v", api.inputs[1].NextToken, api.inputs[2].NextToken)
	}
	if len(entries) != 3 || entries[0].Message != "older" || entries[1].Message != "newer" || entries[2].Message != "newest" {
		t.Fatalf("entries = %#v", entries)
	}
	if entries[0].Stream != "stream" || lastTS != 1003 {
		t.Fatalf("stream = %q, lastTS = %d", entries[0].Stream, lastTS)
	}
}

func TestFetchLogsRangeIncludesEndTimeAndPaginates(t *testing.T) {
	api := &fakeFilterLogEventsAPI{
		t: t,
		pages: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwltypes.FilteredLogEvent{
					logEvent(1000, "first"),
					logEvent(1001, "second"),
				},
				NextToken: strPtrLogs("page-2"),
			},
			{
				Events: []cwltypes.FilteredLogEvent{
					logEvent(1002, "third"),
				},
			},
		},
	}

	logGroup := "/aws/ecs/example"
	stream := "ecs/app/task"
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:   &logGroup,
		LogStreamNames: []string{stream},
		StartTime:      int64PtrLogs(900),
		EndTime:        int64PtrLogs(1100),
	}

	entries, err := fetchLogsRange(context.Background(), api, input, 10)
	if err != nil {
		t.Fatalf("fetchLogsRange returned error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if api.inputs[0].EndTime == nil || *api.inputs[0].EndTime != 1100 {
		t.Fatalf("first request end time = %v, want 1100", api.inputs[0].EndTime)
	}
	if api.inputs[1].NextToken == nil || *api.inputs[1].NextToken != "page-2" {
		t.Fatalf("second request next token = %v, want page-2", api.inputs[1].NextToken)
	}
}

func TestFetchLogsRangeKeepsWindowAroundRangeMidpointWhenTrimming(t *testing.T) {
	api := &fakeFilterLogEventsAPI{
		t: t,
		pages: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []cwltypes.FilteredLogEvent{
					logEvent(1000, "t0"),
					logEvent(1010, "t10"),
					logEvent(1020, "t20"),
					logEvent(1030, "t30"),
					logEvent(1040, "t40"),
				},
			},
		},
	}

	logGroup := "/aws/ecs/example"
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: &logGroup,
		StartTime:    int64PtrLogs(1000),
		EndTime:      int64PtrLogs(1040),
	}

	entries, err := fetchLogsRange(context.Background(), api, input, 3)
	if err != nil {
		t.Fatalf("fetchLogsRange returned error: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Timestamp != 1010 || entries[1].Timestamp != 1020 || entries[2].Timestamp != 1030 {
		t.Fatalf("expected centered timestamps [1010 1020 1030], got [%d %d %d]",
			entries[0].Timestamp, entries[1].Timestamp, entries[2].Timestamp)
	}
}

func cloneFilterInput(input *cloudwatchlogs.FilterLogEventsInput) *cloudwatchlogs.FilterLogEventsInput {
	cloned := *input
	if input.LogStreamNames != nil {
		cloned.LogStreamNames = append([]string(nil), input.LogStreamNames...)
	}
	if input.StartTime != nil {
		cloned.StartTime = int64PtrLogs(*input.StartTime)
	}
	if input.EndTime != nil {
		cloned.EndTime = int64PtrLogs(*input.EndTime)
	}
	if input.Limit != nil {
		cloned.Limit = int32PtrLogs(*input.Limit)
	}
	if input.NextToken != nil {
		cloned.NextToken = strPtrLogs(*input.NextToken)
	}
	return &cloned
}

func logEvent(ts int64, msg string) cwltypes.FilteredLogEvent {
	return cwltypes.FilteredLogEvent{
		Timestamp: int64PtrLogs(ts),
		Message:   strPtrLogs(msg),
		EventId:   strPtrLogs(msg),
	}
}

func int64PtrLogs(v int64) *int64 { return &v }
func int32PtrLogs(v int32) *int32 { return &v }
func strPtrLogs(v string) *string { return &v }
