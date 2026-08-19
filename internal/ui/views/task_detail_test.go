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
