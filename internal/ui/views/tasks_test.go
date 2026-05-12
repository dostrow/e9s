package views

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestTaskListViewShowsAvailabilityZoneColumn(t *testing.T) {
	m := NewTaskList("api").SetSize(120, 20).SetTasks([]model.Task{
		{
			TaskID:           "abc123def456",
			Status:           "RUNNING",
			HealthStatus:     "HEALTHY",
			AvailabilityZone: "us-east-1a",
			PrivateIP:        "10.0.1.42",
		},
	})

	view := m.View()
	if !strings.Contains(view, "AZ") {
		t.Fatalf("expected AZ header in task list view:\n%s", view)
	}
	if !strings.Contains(view, "us-east-1a") {
		t.Fatalf("expected availability zone in task list view:\n%s", view)
	}
}

func TestTaskListFilterMatchesAvailabilityZone(t *testing.T) {
	m := NewTaskList("api").SetTasks([]model.Task{
		{TaskID: "task-a", AvailabilityZone: "us-east-1a"},
		{TaskID: "task-b", AvailabilityZone: "us-east-1b"},
	})
	m.filter = "1b"

	filtered := m.filteredTasks()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered task, got %d", len(filtered))
	}
	if filtered[0].TaskID != "task-b" {
		t.Fatalf("expected task-b, got %#v", filtered[0])
	}
}
