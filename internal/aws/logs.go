package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/dostrow/e9s/internal/model"
)

type LogEntry struct {
	Timestamp int64
	Message   string
	Stream    string
}

// GetLogConfig retrieves the awslogs configuration from a task definition
// for a given container. Returns logGroup, logStreamPrefix.
func (c *Client) GetLogConfig(ctx context.Context, taskDefARN, containerName string) (string, string, error) {
	out, err := c.ECS.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: &taskDefARN,
	})
	if err != nil {
		return "", "", err
	}

	for _, cd := range out.TaskDefinition.ContainerDefinitions {
		if cd.Name == nil || *cd.Name != containerName {
			continue
		}
		if cd.LogConfiguration == nil {
			continue
		}
		opts := cd.LogConfiguration.Options
		logGroup := opts["awslogs-group"]
		streamPrefix := opts["awslogs-stream-prefix"]
		return logGroup, streamPrefix, nil
	}

	return "", "", fmt.Errorf("no awslogs config found for container %q in %s", containerName, taskDefARN)
}

// BuildLogStreamName constructs the CloudWatch log stream name for a task container.
// Format: {prefix}/{container-name}/{task-id}
func BuildLogStreamName(streamPrefix, containerName, taskID string) string {
	return fmt.Sprintf("%s/%s/%s", streamPrefix, containerName, taskID)
}

// FetchLogs retrieves log events from a single log stream using FilterLogEvents.
// startTime is a unix timestamp in milliseconds; 0 means from the beginning.
// Returns log entries and the timestamp of the last event (for pagination).
func (c *Client) FetchLogs(ctx context.Context, logGroup, logStream string, startTime int64, limit int) ([]LogEntry, int64, error) {
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:   &logGroup,
		LogStreamNames: []string{logStream},
		Limit:          intPtr(int32(limit)),
	}
	if startTime > 0 {
		input.StartTime = &startTime
	}

	out, err := c.Logs.FilterLogEvents(ctx, input)
	if err != nil {
		return nil, startTime, err
	}

	entries := make([]LogEntry, 0, len(out.Events))
	lastTS := startTime
	for _, ev := range out.Events {
		ts := int64(0)
		if ev.Timestamp != nil {
			ts = *ev.Timestamp
		}
		msg := ""
		if ev.Message != nil {
			msg = *ev.Message
		}
		stream := ""
		if ev.LogStreamName != nil {
			stream = *ev.LogStreamName
		}
		entries = append(entries, LogEntry{
			Timestamp: ts,
			Message:   msg,
			Stream:    stream,
		})
		if ts > lastTS {
			lastTS = ts
		}
	}

	return entries, lastTS, nil
}

// FetchMultiStreamLogs retrieves log events from multiple streams (for service-level logs).
func (c *Client) FetchMultiStreamLogs(ctx context.Context, logGroup string, logStreams []string, startTime int64, limit int) ([]LogEntry, int64, error) {
	if len(logStreams) == 0 {
		return nil, startTime, nil
	}

	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:   &logGroup,
		LogStreamNames: logStreams,
		Limit:          intPtr(int32(limit)),
	}
	if startTime > 0 {
		input.StartTime = &startTime
	}

	out, err := c.Logs.FilterLogEvents(ctx, input)
	if err != nil {
		return nil, startTime, err
	}

	entries := make([]LogEntry, 0, len(out.Events))
	lastTS := startTime
	for _, ev := range out.Events {
		entry := eventToLogEntry(ev)
		entries = append(entries, entry)
		if entry.Timestamp > lastTS {
			lastTS = entry.Timestamp
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp < entries[j].Timestamp
	})

	return entries, lastTS, nil
}

// ResolveTaskLogStreams resolves log group and stream names for all containers in the given tasks.
// Returns logGroup (assumed same for all) and a list of stream names.
func (c *Client) ResolveTaskLogStreams(ctx context.Context, tasks []model.Task) (string, []string, error) {
	if len(tasks) == 0 {
		return "", nil, nil
	}

	// Cache task definition lookups
	tdCache := map[string]*ecs.DescribeTaskDefinitionOutput{}
	var logGroup string
	var streams []string

	for _, t := range tasks {
		td := t.TaskDefinition
		if _, ok := tdCache[td]; !ok {
			// Need full ARN for DescribeTaskDefinition — but short form works too
			out, err := c.ECS.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &td,
			})
			if err != nil {
				continue
			}
			tdCache[td] = out
		}

		out := tdCache[td]
		for _, cd := range out.TaskDefinition.ContainerDefinitions {
			if cd.LogConfiguration == nil {
				continue
			}
			opts := cd.LogConfiguration.Options
			lg := opts["awslogs-group"]
			prefix := opts["awslogs-stream-prefix"]
			if lg == "" {
				continue
			}
			logGroup = lg
			containerName := ""
			if cd.Name != nil {
				containerName = *cd.Name
			}
			stream := BuildLogStreamName(prefix, containerName, t.TaskID)
			streams = append(streams, stream)
		}
	}

	return logGroup, streams, nil
}

// FetchLogGroup retrieves recent log events from an entire log group (no stream filter).
func (c *Client) FetchLogGroup(ctx context.Context, logGroup string, startTime int64, limit int) ([]LogEntry, int64, error) {
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: &logGroup,
		Limit:        intPtr(int32(limit)),
	}
	if startTime > 0 {
		input.StartTime = &startTime
	}

	out, err := c.Logs.FilterLogEvents(ctx, input)
	if err != nil {
		return nil, startTime, err
	}

	entries := make([]LogEntry, 0, len(out.Events))
	lastTS := startTime
	for _, ev := range out.Events {
		entry := eventToLogEntry(ev)
		entries = append(entries, entry)
		if entry.Timestamp > lastTS {
			lastTS = entry.Timestamp
		}
	}
	return entries, lastTS, nil
}

type LogGroupInfo struct {
	Name      string
	StoredBytes int64
	StreamCount int
}

type LogStreamInfo struct {
	Name           string
	LastEventTime  int64
	FirstEventTime int64
}

// ListLogGroups returns log groups matching a search term.
// If the term starts with "/" it's treated as a prefix, otherwise as a substring pattern.
func (c *Client) ListLogGroups(ctx context.Context, search string) ([]LogGroupInfo, error) {
	input := &cloudwatchlogs.DescribeLogGroupsInput{}
	if search != "" {
		if strings.HasPrefix(search, "/") {
			input.LogGroupNamePrefix = &search
		} else {
			input.LogGroupNamePattern = &search
		}
	}

	var groups []LogGroupInfo
	paginator := cloudwatchlogs.NewDescribeLogGroupsPaginator(c.Logs, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range page.LogGroups {
			name := ""
			if g.LogGroupName != nil {
				name = *g.LogGroupName
			}
			var stored int64
			if g.StoredBytes != nil {
				stored = *g.StoredBytes
			}
			groups = append(groups, LogGroupInfo{
				Name:        name,
				StoredBytes: stored,
			})
		}
	}
	return groups, nil
}

// ListLogStreams returns log streams for a group, optionally filtered by prefix.
func (c *Client) ListLogStreams(ctx context.Context, logGroup, prefix string) ([]LogStreamInfo, error) {
	input := &cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: &logGroup,
		OrderBy:      cwltypes.OrderByLastEventTime,
		Descending:   boolPtr(true),
	}
	if prefix != "" {
		// OrderBy must be LogStreamName when using prefix filter
		input.OrderBy = cwltypes.OrderByLogStreamName
		input.Descending = nil
		input.LogStreamNamePrefix = &prefix
	}

	var streams []LogStreamInfo
	paginator := cloudwatchlogs.NewDescribeLogStreamsPaginator(c.Logs, input)
	// Limit to a reasonable number of pages
	pages := 0
	for paginator.HasMorePages() && pages < 5 {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range page.LogStreams {
			name := ""
			if s.LogStreamName != nil {
				name = *s.LogStreamName
			}
			var lastEvent, firstEvent int64
			if s.LastEventTimestamp != nil {
				lastEvent = *s.LastEventTimestamp
			}
			if s.FirstEventTimestamp != nil {
				firstEvent = *s.FirstEventTimestamp
			}
			streams = append(streams, LogStreamInfo{
				Name:           name,
				LastEventTime:  lastEvent,
				FirstEventTime: firstEvent,
			})
		}
		pages++
	}
	return streams, nil
}

// SearchLogs searches for a pattern in a log group within a time range.
// Returns matching events. contextLines specifies how many extra events to
// fetch around each match for context (fetched via a broader time window).
func (c *Client) SearchLogs(ctx context.Context, logGroup string, streams []string, pattern string, startTimeMs, endTimeMs int64, maxResults int) ([]LogEntry, error) {
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:  &logGroup,
		FilterPattern: &pattern,
	}
	if len(streams) > 0 {
		input.LogStreamNames = streams
	}
	if startTimeMs > 0 {
		input.StartTime = &startTimeMs
	}
	if endTimeMs > 0 {
		input.EndTime = &endTimeMs
	}

	var entries []LogEntry
	remaining := maxResults
	var nextToken *string

	for remaining > 0 {
		// Set limit per page to min(remaining, 100)
		pageLimit := remaining
		if pageLimit > 100 {
			pageLimit = 100
		}
		input.Limit = intPtr(int32(pageLimit))
		input.NextToken = nextToken

		page, err := c.Logs.FilterLogEvents(ctx, input)
		if err != nil {
			return entries, err
		}
		for _, ev := range page.Events {
			entry := eventToLogEntry(ev)
			// Double-check the event is within our time range
			if endTimeMs > 0 && entry.Timestamp > endTimeMs {
				continue
			}
			entries = append(entries, entry)
			remaining--
			if remaining <= 0 {
				break
			}
		}
		nextToken = page.NextToken
		if nextToken == nil || remaining <= 0 {
			break
		}
	}

	return entries, nil
}

// FetchLogsAroundTimestamp fetches logs from a stream around a given timestamp
// for context display.
func (c *Client) FetchLogsAroundTimestamp(ctx context.Context, logGroup, stream string, timestampMs int64, contextLines int) ([]LogEntry, error) {
	// Fetch a window: contextLines before and after
	windowMs := int64(60000) // 1 minute window
	start := timestampMs - windowMs
	end := timestampMs + windowMs
	limit := int32(contextLines * 2)

	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:   &logGroup,
		LogStreamNames: []string{stream},
		StartTime:      &start,
		EndTime:        &end,
		Limit:          &limit,
	}

	out, err := c.Logs.FilterLogEvents(ctx, input)
	if err != nil {
		return nil, err
	}

	entries := make([]LogEntry, 0, len(out.Events))
	for _, ev := range out.Events {
		entries = append(entries, eventToLogEntry(ev))
	}
	return entries, nil
}

func boolPtr(b bool) *bool {
	return &b
}

func eventToLogEntry(ev cwltypes.FilteredLogEvent) LogEntry {
	entry := LogEntry{}
	if ev.Timestamp != nil {
		entry.Timestamp = *ev.Timestamp
	}
	if ev.Message != nil {
		entry.Message = *ev.Message
	}
	if ev.LogStreamName != nil {
		entry.Stream = *ev.LogStreamName
	}
	return entry
}

func intPtr(i int32) *int32 {
	return &i
}
