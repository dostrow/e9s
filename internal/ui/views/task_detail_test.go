package views

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestTaskDetailShowsStopCode(t *testing.T) {
	m := NewTaskDetail(&model.Task{
		TaskID:        "task-1",
		Status:        "STOPPED",
		StopCode:      "EssentialContainerExited",
		StoppedReason: "container failed",
		VpcID:         "vpc-1",
		SubnetID:      "subnet-1",
	}).SetSize(100, 30)

	view := m.View()
	if !strings.Contains(view, "EssentialContainerExited") || !strings.Contains(view, "container failed") || !strings.Contains(view, "vpc-1") || !strings.Contains(view, "subnet-1") {
		t.Fatalf("stopped task detail omitted diagnostics:\n%s", view)
	}
}

func TestTaskDetailShowsParentServiceTargetGroups(t *testing.T) {
	task := &model.Task{TaskID: "task-1", Status: "RUNNING"}
	service := &model.Service{
		Name:         "api",
		TargetGroups: []model.ResourceRef{{Kind: "ec2-target-group", ID: "arn:target-group:api"}},
	}
	view := NewTaskDetail(task).SetServiceContext(service).SetSize(100, 40).View()
	for _, want := range []string{"Service Context", "Parent Service:", "api", "Configured Target Groups:", "arn:target-group:api"} {
		if !strings.Contains(view, want) {
			t.Fatalf("task detail omitted %q:\n%s", want, view)
		}
	}
}
