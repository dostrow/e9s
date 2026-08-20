//go:build gui

package gui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

func TestFilterSQSQueuesMatchesNameAndURLCaseInsensitively(t *testing.T) {
	queues := []model.SQSQueue{
		{Name: "Prod-Events", URL: "https://sqs.us-east-2.amazonaws.com/123/Prod-Events"},
		{Name: "archive", URL: "https://sqs.eu-west-1.amazonaws.com/123/archive"},
	}
	if got := filterSQSQueues(queues, " PROD "); !reflect.DeepEqual(got, queues[:1]) {
		t.Fatalf("filterSQSQueues(name) = %#v", got)
	}
	if got := filterSQSQueues(queues, "EU-WEST-1"); !reflect.DeepEqual(got, queues[1:]) {
		t.Fatalf("filterSQSQueues(URL) = %#v", got)
	}
}

func TestSQSQueueBreadcrumbPreservesSavedDestination(t *testing.T) {
	if got := sqsQueueBreadcrumb("Production events", "events.fifo"); got != "SQS / Production events / events.fifo" {
		t.Fatalf("sqsQueueBreadcrumb() = %q", got)
	}
	if got := sqsQueueBreadcrumb("", ""); got != "SQS / Queues" {
		t.Fatalf("sqsQueueBreadcrumb(default) = %q", got)
	}
}

func TestFormatSQSQueueIncludesConfigurationAndDeadLetterQueue(t *testing.T) {
	queue := model.SQSQueue{Name: "events", URL: "https://example.test/events"}
	stats := &model.SQSQueueStats{
		IsFIFO: true, MessagesAvailable: 12, VisibilityTimeout: 90,
		RetentionSeconds: 345600, MaxMessageSize: 262144,
		DeadLetterTargetARN: "arn:aws:sqs:us-east-2:123:events-dlq", MaxReceiveCount: 5,
	}
	got := formatSQSQueue(queue, stats)
	for _, want := range []string{"FIFO", "12", "1m", "4d", "events-dlq", "5"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatSQSQueue() = %q, missing %q", got, want)
		}
	}
}

func TestFindSavedSQSQueue(t *testing.T) {
	queues := []config.SQSQueueEntry{{Name: "events", URL: "https://example.test/events"}}
	queue, found := findSavedSQSQueue(queues, "events")
	if !found || queue.URL != "https://example.test/events" {
		t.Fatalf("findSavedSQSQueue() = %#v, %v", queue, found)
	}
	if _, found := findSavedSQSQueue(queues, "missing"); found {
		t.Fatal("findSavedSQSQueue() unexpectedly found missing queue")
	}
}
