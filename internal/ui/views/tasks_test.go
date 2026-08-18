package views

import (
	"strings"
	"testing"
	"time"

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

func TestTaskListViewShowsStoppedDiagnosticsAndPagination(t *testing.T) {
	exitCode := 137
	m := NewTaskList("api").SetSize(160, 24).SetScope(true).SetTasks([]model.Task{
		{
			TaskID:         "stopped-task-123",
			Status:         "STOPPED",
			StoppedAt:      time.Now().Add(-time.Minute),
			StopCode:       "EssentialContainerExited",
			StoppedReason:  "essential container exited",
			TaskDefinition: "api:42",
			Containers:     []model.Container{{Name: "api", ExitCode: &exitCode}},
		},
	}).SetHasMore(true)

	view := m.View()
	for _, expected := range []string{"Recently stopped", "loads 50 more", "EXIT", "STOP CODE", "137", "EssentialContainerExited", "api:42"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("expected %q in stopped task view:\n%s", expected, view)
		}
	}
}

func TestTaskListAppendTasksDeduplicatesPages(t *testing.T) {
	m := NewTaskList("api").SetTasks([]model.Task{{TaskARN: "arn:task/1", TaskID: "1"}})
	m = m.AppendTasks([]model.Task{
		{TaskARN: "arn:task/1", TaskID: "1"},
		{TaskARN: "arn:task/2", TaskID: "2"},
	})
	if got := len(m.tasks); got != 2 {
		t.Fatalf("AppendTasks produced %d tasks, want 2", got)
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
