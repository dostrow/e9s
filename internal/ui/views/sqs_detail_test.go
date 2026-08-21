package views

import (
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

func TestSQSDetailIncludesLatestCloudWatchMetrics(t *testing.T) {
	now := time.Now()
	view := NewSQSDetail("jobs", "https://example.test/jobs").
		SetStats(&model.SQSQueueStats{}).
		SetMetrics(&model.MetricSnapshot{Series: []model.MetricSeries{{
			ID: "messages_visible", Label: "Available", Unit: "count",
			Points: []model.MetricPoint{{Timestamp: now.Add(-time.Minute), Value: 2}, {Timestamp: now, Value: 7}},
		}}}, "")
	rendered := view.View()
	if !strings.Contains(rendered, "LATEST CLOUDWATCH METRICS") || !strings.Contains(rendered, "7.00 count") {
		t.Fatalf("SQS detail did not include latest metrics:\n%s", rendered)
	}
}

func TestSQSDetailKeepsStatsWhenMetricsFail(t *testing.T) {
	view := NewSQSDetail("jobs", "https://example.test/jobs").
		SetStats(&model.SQSQueueStats{MessagesAvailable: 3}).
		SetMetrics(nil, "access denied")
	rendered := view.View()
	if !strings.Contains(rendered, "Messages Available:") || !strings.Contains(rendered, "CloudWatch metrics unavailable: access denied") {
		t.Fatalf("SQS detail did not retain stats and warning:\n%s", rendered)
	}
}
