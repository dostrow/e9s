//go:build gui

package gui

import (
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

func TestFilterAlarmsMatchesNameMetricAndNamespace(t *testing.T) {
	alarms := []model.Alarm{
		{Name: "api-errors", MetricName: "Errors", Namespace: "AWS/Lambda"},
		{Name: "queue-depth", MetricName: "ApproximateNumberOfMessagesVisible", Namespace: "AWS/SQS"},
	}
	for _, query := range []string{"API", "errors", "lambda"} {
		got := filterAlarms(alarms, query)
		if len(got) != 1 || got[0].Name != "api-errors" {
			t.Fatalf("filterAlarms(%q) = %#v", query, got)
		}
	}
	if got := filterAlarms(alarms, "aws"); len(got) != 2 {
		t.Fatalf("filterAlarms(aws) returned %d alarms", len(got))
	}
}

func TestAlarmScopeLabelsAndBreadcrumbs(t *testing.T) {
	if got := alarmScopeLabel(model.AlarmStateAlarm); got != "In alarm" {
		t.Fatalf("alarm scope = %q", got)
	}
	if got := alarmBreadcrumb(model.AlarmStateOK, "api-errors"); got != "CloudWatch Alarms / OK / api-errors" {
		t.Fatalf("breadcrumb = %q", got)
	}
}

func TestFormatAlarmSummaryIncludesOperationalState(t *testing.T) {
	alarm := model.Alarm{
		Name: "api-errors", State: model.AlarmStateAlarm, StateReason: "threshold crossed",
		StateUpdatedAt: time.Date(2026, time.August, 18, 12, 0, 0, 0, time.Local),
		MetricName:     "Errors", Namespace: "AWS/Lambda", ActionsEnabled: true,
	}
	got := formatAlarmSummary(alarm)
	for _, want := range []string{"api-errors", "ALARM", "threshold crossed", "AWS/Lambda", "enabled"} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q:\n%s", want, got)
		}
	}
}
