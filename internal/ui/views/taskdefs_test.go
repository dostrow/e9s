package views

import (
	"testing"

	"github.com/dostrow/e9s/internal/aws"
)

func TestTaskDefsSelectedTaskDef(t *testing.T) {
	m := NewTaskDefs().SetTaskDefs([]aws.TaskDefRef{
		{ARN: "arn:1", Family: "api", Revision: 3},
		{ARN: "arn:2", Family: "worker", Revision: 7},
	})
	m.cursor = 1

	td := m.SelectedTaskDef()
	if td == nil {
		t.Fatal("expected selected task definition")
	}
	if td.Family != "worker" || td.Revision != 7 {
		t.Fatalf("selected task definition = %#v", td)
	}
}

func TestTaskDefsFilterMatchesFamilyAndARN(t *testing.T) {
	m := NewTaskDefs().SetTaskDefs([]aws.TaskDefRef{
		{ARN: "arn:aws:ecs:::task-definition/api:3", Family: "api", Revision: 3},
		{ARN: "arn:aws:ecs:::task-definition/worker:7", Family: "worker", Revision: 7},
	})
	m.filter = "worker"
	filtered := m.filteredTaskDefs()
	if len(filtered) != 1 || filtered[0].Family != "worker" {
		t.Fatalf("filtered task definitions = %#v", filtered)
	}
}

func TestTaskDefsPreviousRevision(t *testing.T) {
	m := NewTaskDefs().SetTaskDefs([]aws.TaskDefRef{
		{ARN: "arn:api:5", Family: "api", Revision: 5},
		{ARN: "arn:worker:4", Family: "worker", Revision: 4},
		{ARN: "arn:api:3", Family: "api", Revision: 3},
		{ARN: "arn:api:2", Family: "api", Revision: 2},
	})

	previous := m.PreviousRevision("api", 5)
	if previous == nil {
		t.Fatal("expected a previous task-definition revision")
	}
	if previous.Family != "api" || previous.Revision != 3 {
		t.Fatalf("previous revision = %#v, want api:3", previous)
	}

	if oldest := m.PreviousRevision("api", 2); oldest != nil {
		t.Fatalf("oldest revision unexpectedly had predecessor %#v", oldest)
	}
}
