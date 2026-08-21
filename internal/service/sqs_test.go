package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

type fakeSQSAPI struct {
	queues      []model.SQSQueue
	stats       *model.SQSQueueStats
	messages    []model.SQSMessage
	queueURL    string
	messageID   string
	err         error
	maxMessages int
	waitSeconds int
	sent        model.SQSSendTemplate
}

func (f *fakeSQSAPI) ListSQSQueues(context.Context, string) ([]model.SQSQueue, error) {
	return append([]model.SQSQueue(nil), f.queues...), f.err
}
func (f *fakeSQSAPI) GetQueueStats(context.Context, string) (*model.SQSQueueStats, error) {
	return f.stats, f.err
}
func (f *fakeSQSAPI) ReceiveSQSMessages(_ context.Context, _ string, maxMessages, waitSeconds int) ([]model.SQSMessage, error) {
	f.maxMessages, f.waitSeconds = maxMessages, waitSeconds
	return append([]model.SQSMessage(nil), f.messages...), f.err
}
func (f *fakeSQSAPI) DeleteSQSMessage(context.Context, string, string) error { return f.err }
func (f *fakeSQSAPI) SendSQSMessage(_ context.Context, _ string, template model.SQSSendTemplate) (string, error) {
	f.sent = template
	return f.messageID, f.err
}
func (f *fakeSQSAPI) GetQueueURL(context.Context, string) (string, error) {
	return f.queueURL, f.err
}

func TestSQSQueuesSortAndWrapErrors(t *testing.T) {
	api := &fakeSQSAPI{queues: []model.SQSQueue{{Name: "zeta"}, {Name: "Alpha"}, {Name: "beta"}}}
	queues, err := NewSQS(api).Queues(context.Background(), "")
	if err != nil || !reflect.DeepEqual([]string{queues[0].Name, queues[1].Name, queues[2].Name}, []string{"Alpha", "beta", "zeta"}) {
		t.Fatalf("Queues() = %#v, %v", queues, err)
	}
	api.err = errors.New("denied")
	if _, err := NewSQS(api).Queues(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "list SQS queues") {
		t.Fatalf("Queues() error = %v", err)
	}
}

func TestSQSMessagesClampsBatchAndValidatesWait(t *testing.T) {
	api := &fakeSQSAPI{}
	sqs := NewSQS(api)
	_, err := sqs.Messages(context.Background(), model.SQSReceiveRequest{QueueURL: "queue", MaxMessages: 25, WaitSeconds: 5})
	if err != nil || api.maxMessages != 10 || api.waitSeconds != 5 {
		t.Fatalf("Messages() error = %v; max=%d wait=%d", err, api.maxMessages, api.waitSeconds)
	}
	if _, err := sqs.Messages(context.Background(), model.SQSReceiveRequest{QueueURL: "queue", WaitSeconds: 21}); err == nil {
		t.Fatal("Messages() accepted an invalid long-poll wait")
	}
}

func TestSQSSendTemplateValidation(t *testing.T) {
	template := model.SQSSendTemplate{Body: "hello", GroupID: "workers", Attributes: map[string]model.SQSAttr{
		"payload": {DataType: "Binary", Value: "aGVsbG8="},
	}}
	if err := ValidateSQSSendTemplate("jobs.fifo", template); err != nil {
		t.Fatal(err)
	}
	template.GroupID = ""
	if err := ValidateSQSSendTemplate("jobs.fifo", template); err == nil {
		t.Fatal("ValidateSQSSendTemplate() accepted a FIFO message without group ID")
	}
	template.GroupID = "workers"
	template.Attributes["payload"] = model.SQSAttr{DataType: "Binary", Value: "not base64"}
	if err := ValidateSQSSendTemplate("jobs.fifo", template); err == nil {
		t.Fatal("ValidateSQSSendTemplate() accepted invalid binary data")
	}
}

func TestSQSTemplateRoundTripFromMessage(t *testing.T) {
	message := model.SQSMessage{
		Body:       `{"test":"data"}`,
		Attributes: map[string]string{"MessageGroupId": "group1"},
		UserAttributes: map[string]model.SQSMessageAttribute{
			"custom": {DataType: "String", StringValue: "value"},
			"blob":   {DataType: "Binary", BinaryValue: []byte("hello")},
		},
	}
	document := BuildSQSSendTemplateFromMessage(message)
	template, err := ParseSQSSendTemplate(document)
	if err != nil {
		t.Fatal(err)
	}
	if template.GroupID != "group1" || template.Attributes["custom"].Value != "value" || template.Attributes["blob"].Value != "aGVsbG8=" {
		t.Fatalf("template = %#v", template)
	}
}

func TestQueueNameHelpers(t *testing.T) {
	if got := QueueNameFromARN("arn:aws:sqs:us-east-1:123:my-dlq"); got != "my-dlq" {
		t.Fatalf("QueueNameFromARN() = %q", got)
	}
	if got := QueueNameFromURL("https://sqs.us-east-1.amazonaws.com/123/my-queue/"); got != "my-queue" {
		t.Fatalf("QueueNameFromURL() = %q", got)
	}
}
