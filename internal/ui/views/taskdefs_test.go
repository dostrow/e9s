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
