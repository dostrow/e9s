package aws

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestQueueNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://sqs.us-east-1.amazonaws.com/123456789012/my-queue", "my-queue"},
		{"https://sqs.us-east-1.amazonaws.com/123456789012/my-queue.fifo", "my-queue.fifo"},
		{"my-queue", "my-queue"},
	}
	for _, tt := range tests {
		got := queueNameFromURL(tt.url)
		if got != tt.want {
			t.Errorf("queueNameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestBuildSendTemplate_Standard(t *testing.T) {
	tmpl := BuildSendTemplate(false)
	var parsed SQSSendTemplate
	if err := json.Unmarshal([]byte(tmpl), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed.GroupID != "" {
		t.Error("Standard queue should not have GroupID")
	}
}

func TestBuildSendTemplate_FIFO(t *testing.T) {
	tmpl := BuildSendTemplate(true)
	if !strings.Contains(tmpl, "groupId") {
		t.Error("FIFO template should contain groupId field")
	}
	if !strings.Contains(tmpl, "deduplicationId") {
		t.Error("FIFO template should contain deduplicationId field")
	}
}

func TestBuildSendTemplateFromMessage(t *testing.T) {
	msg := SQSMessage{
		Body: `{"test": "data"}`,
		Attributes: map[string]string{
			"MessageGroupId": "group1",
		},
		UserAttrsMap: map[string]string{"custom": "value"},
	}
	tmpl := BuildSendTemplateFromMessage(msg)
	if !strings.Contains(tmpl, "test") {
		t.Error("Template should contain original body")
	}
	if !strings.Contains(tmpl, "group1") {
		t.Error("Template should contain group ID")
	}
}

func TestParseSendTemplate_Valid(t *testing.T) {
	input := `{"body": "hello world", "delaySeconds": 5}`
	tmpl, err := ParseSendTemplate(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tmpl.Body != "hello world" {
		t.Errorf("Body = %q, want %q", tmpl.Body, "hello world")
	}
	if tmpl.DelaySeconds != 5 {
		t.Errorf("DelaySeconds = %d, want 5", tmpl.DelaySeconds)
	}
}

func TestParseSendTemplate_EmptyBody(t *testing.T) {
	_, err := ParseSendTemplate(`{"body": ""}`)
	if err == nil {
		t.Error("Expected error for empty body")
	}
}

func TestParseSendTemplate_InvalidJSON(t *testing.T) {
	_, err := ParseSendTemplate("not json")
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestQueueNameFromARN(t *testing.T) {
	tests := []struct {
		arn  string
		want string
	}{
		{"arn:aws:sqs:us-east-1:123456789012:my-dlq", "my-dlq"},
		{"arn:aws:sqs:us-east-1:123456789012:my-queue.fifo", "my-queue.fifo"},
		{"not-an-arn", "not-an-arn"},
	}
	for _, tt := range tests {
		got := QueueNameFromARN(tt.arn)
		if got != tt.want {
			t.Errorf("QueueNameFromARN(%q) = %q, want %q", tt.arn, got, tt.want)
		}
	}
}

func TestAtoi(t *testing.T) {
	if atoi("42") != 42 {
		t.Error("atoi(42) failed")
	}
	if atoi("") != 0 {
		t.Error("atoi('') should be 0")
	}
	if atoi("abc") != 0 {
		t.Error("atoi('abc') should be 0")
	}
}
