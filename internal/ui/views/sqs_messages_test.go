package views

import (
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestSQSMessagesMergeRefreshesReceiptAndRemove(t *testing.T) {
	view := NewSQSMessages("jobs", "queue-url").SetMessages([]model.SQSMessage{{MessageID: "one", ReceiptHandle: "old"}})
	view = view.SetMessages([]model.SQSMessage{{MessageID: "one", ReceiptHandle: "new"}, {MessageID: "two", ReceiptHandle: "two"}})
	if len(view.messages) != 2 || view.messages[0].ReceiptHandle != "new" {
		t.Fatalf("merged messages = %#v", view.messages)
	}
	view = view.RemoveMessage("one")
	if len(view.messages) != 1 || view.messages[0].MessageID != "two" {
		t.Fatalf("remaining messages = %#v", view.messages)
	}
}
