//go:build gui

package gui

import "testing"

func TestCurrentResourceNavigationStateCapturesECSServiceContext(t *testing.T) {
	w := &mainWindow{
		currentPage:     pageServices,
		detailContent:   detailService,
		breadcrumbText:  "ECS / prod / api",
		selectedCluster: "prod",
		selectedService: "api",
	}
	state, ok := w.currentResourceNavigationState()
	if !ok {
		t.Fatal("service detail did not produce a navigation snapshot")
	}
	if state.ref.Kind != "ecs-service" || state.ref.ID != "api" || state.cluster != "prod" || state.breadcrumb != "ECS / prod / api" {
		t.Fatalf("service snapshot = %#v", state)
	}
}

func TestCurrentResourceNavigationStateCapturesECSTaskBrowserContext(t *testing.T) {
	w := &mainWindow{
		currentPage:         pageTasks,
		detailContent:       detailTask,
		breadcrumbText:      "ECS / prod / api / task-1",
		selectedCluster:     "prod",
		selectedService:     "api",
		selectedTask:        "arn:task-1",
		showingStoppedTasks: true,
	}
	state, ok := w.currentResourceNavigationState()
	if !ok {
		t.Fatal("task detail did not produce a navigation snapshot")
	}
	if state.ref.Kind != "ecs-task" || state.task != "arn:task-1" || state.service != "api" || !state.showingStoppedTasks {
		t.Fatalf("task snapshot = %#v", state)
	}
}
