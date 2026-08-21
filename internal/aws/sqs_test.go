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

func TestSQSMetricQueries(t *testing.T) {
	queries := sqsMetricQueries("jobs.fifo")
	if len(queries) != 11 {
		t.Fatalf("len(sqsMetricQueries()) = %d, want 11", len(queries))
	}
	seen := make(map[string]struct{}, len(queries))
	for _, query := range queries {
		if query.Namespace != "AWS/SQS" {
			t.Fatalf("query %q namespace = %q", query.ID, query.Namespace)
		}
		if len(query.Dimensions) != 1 || query.Dimensions[0].Name != "QueueName" || query.Dimensions[0].Value != "jobs.fifo" {
			t.Fatalf("query %q dimensions = %#v", query.ID, query.Dimensions)
		}
		if _, exists := seen[query.ID]; exists {
			t.Fatalf("duplicate query ID %q", query.ID)
		}
		seen[query.ID] = struct{}{}
	}
	if queries[0].MetricName != "ApproximateNumberOfMessagesVisible" || queries[0].Statistic != "Average" {
		t.Fatalf("queue depth query = %#v", queries[0])
	}
	if queries[3].MetricName != "NumberOfMessagesSent" || queries[3].Statistic != "Sum" {
		t.Fatalf("traffic query = %#v", queries[3])
	}
}
