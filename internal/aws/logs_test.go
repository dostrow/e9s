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
	}
}

func int64PtrLogs(v int64) *int64 { return &v }
func int32PtrLogs(v int32) *int32 { return &v }
func strPtrLogs(v string) *string { return &v }
