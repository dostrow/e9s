package views

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/aws"
)

func TestTaskMetricsReportsUnavailableContainerInsightsData(t *testing.T) {
	m := NewMetrics("task-1", true).SetSize(100, 30).SetMetrics(&aws.ServiceMetrics{})
	view := m.View()
	if count := strings.Count(view, "No data"); count != 4 {
		t.Fatalf("task metrics rendered %d unavailable values, want 4:\n%s", count, view)
	}
	if !strings.Contains(view, "enhanced observability") {
		t.Fatalf("task metrics omitted Container Insights guidance:\n%s", view)
	}
	if strings.Contains(view, "CloudWatch Alarms") {
		t.Fatalf("task metrics should not render service alarms:\n%s", view)
	}
}

func TestServiceMetricsShowsScaleInState(t *testing.T) {
	m := NewMetrics("api").SetSize(100, 30).
		SetMetrics(&aws.ServiceMetrics{CPUAvg: 25, CPUAvgAvailable: true}).
		SetScaleIn(true, true)
	view := m.View()
	if !strings.Contains(view, "Service scale-in: suspended") {
		t.Fatalf("service metrics omitted scale-in state:\n%s", view)
	}
}
