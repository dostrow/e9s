package aws

import "testing"

func TestQueueNameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://sqs.us-east-1.amazonaws.com/123456789012/my-queue", "my-queue"},
		{"https://sqs.us-east-1.amazonaws.com/123456789012/my-queue.fifo", "my-queue.fifo"},
		{"my-queue", "my-queue"},
	}
	for _, test := range tests {
		if got := queueNameFromURL(test.url); got != test.want {
			t.Errorf("queueNameFromURL(%q) = %q, want %q", test.url, got, test.want)
		}
	}
}

func TestAtoi(t *testing.T) {
	for input, want := range map[string]int{"42": 42, "": 0, "abc": 0} {
		if got := atoi(input); got != want {
			t.Errorf("atoi(%q) = %d, want %d", input, got, want)
		}
	}
}
