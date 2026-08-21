//go:build gui

package gui

import (
	"reflect"
	"strings"
	"testing"
	"time"

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

func TestFilterSQSMessagesSearchesBodyAndAttributes(t *testing.T) {
	messages := []model.SQSMessage{
		{MessageID: "one", Body: `{"event":"created"}`},
		{MessageID: "two", Body: "plain", UserAttributes: map[string]model.SQSMessageAttribute{
			"Environment": {DataType: "String", StringValue: "Production"},
		}},
	}
	if got := filterSQSMessages(messages, "CREATED"); !reflect.DeepEqual(got, messages[:1]) {
		t.Fatalf("filterSQSMessages(body) = %#v", got)
	}
	if got := filterSQSMessages(messages, "production"); !reflect.DeepEqual(got, messages[1:]) {
		t.Fatalf("filterSQSMessages(attribute) = %#v", got)
	}
}

func TestMergeSQSMessagesReplacesReceiptAndAppendsNewMessages(t *testing.T) {
	existing := []model.SQSMessage{{MessageID: "one", ReceiptHandle: "old", Body: "before"}}
	received := []model.SQSMessage{
		{MessageID: "one", ReceiptHandle: "new", Body: "after"},
		{MessageID: "two", ReceiptHandle: "two", Body: "second"},
	}
	got := mergeSQSMessages(existing, received)
	if len(got) != 2 || got[0].ReceiptHandle != "new" || got[0].Body != "after" || got[1].MessageID != "two" {
		t.Fatalf("mergeSQSMessages() = %#v", got)
	}
}

func TestFormatSQSMessagePrettyPrintsJSONAndHidesReceiptHandle(t *testing.T) {
	message := model.SQSMessage{
		MessageID: "message-1", ReceiptHandle: "secret-receipt", Body: `{"b":2,"a":1}`,
		UserAttributes: map[string]model.SQSMessageAttribute{
			"kind": {DataType: "String", StringValue: "event"},
		},
	}
	got := formatSQSMessage(model.SQSQueue{Name: "events"}, message)
	if !strings.Contains(got, "\n  \"a\": 1") || !strings.Contains(got, "kind (String)  event") {
		t.Fatalf("formatSQSMessage() = %q", got)
	}
	if strings.Contains(got, message.ReceiptHandle) {
		t.Fatalf("formatSQSMessage() exposed receipt handle: %q", got)
	}
}

func TestSQSMessageBreadcrumb(t *testing.T) {
	got := sqsMessageBreadcrumb("Production", "events", "abc…")
	if got != "SQS / Production / events / Messages / abc…" {
		t.Fatalf("sqsMessageBreadcrumb() = %q", got)
	}
}

func TestWithoutSQSMessageRemovesOnlySelectedMessage(t *testing.T) {
	messages := []model.SQSMessage{{MessageID: "one"}, {MessageID: "two"}, {MessageID: "three"}}
	got := withoutSQSMessage(messages, "two")
	if !reflect.DeepEqual(got, []model.SQSMessage{{MessageID: "one"}, {MessageID: "three"}}) {
		t.Fatalf("withoutSQSMessage() = %#v", got)
	}
}

func TestSQSMessageTimesFormatForBrowser(t *testing.T) {
	want := time.Date(2026, 8, 21, 14, 30, 0, 0, time.Local)
	if got := formatSQSCapturedAt(want); got != formatTime(want) {
		t.Fatalf("formatSQSCapturedAt() = %q", got)
	}
	if got := formatSQSMillis("not-a-timestamp"); got != "—" {
		t.Fatalf("formatSQSMillis(invalid) = %q", got)
	}
	if got := formatSQSMillis("1787322600000"); got == "—" {
		t.Fatal("formatSQSMillis(valid) returned a placeholder")
	}
}
