package gui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

func TestFilterClusters(t *testing.T) {
	clusters := []model.Cluster{
		{Name: "production", Status: "ACTIVE"},
		{Name: "sandbox", Status: "DRAINING"},
	}

	got := filterClusters(clusters, "PROD")
	if len(got) != 1 || got[0].Name != "production" {
		t.Fatalf("filterClusters() = %#v", got)
	}
	got = filterClusters(clusters, "draining")
	if len(got) != 1 || got[0].Name != "sandbox" {
		t.Fatalf("filterClusters() status match = %#v", got)
	}
}

func TestFilterServicesAcrossVisibleFields(t *testing.T) {
	services := []model.Service{
		{Name: "api", Status: "ACTIVE", HealthStatus: "healthy", TaskDefinition: "api:42", LaunchType: "FARGATE"},
		{Name: "worker", Status: "DRAINING", HealthStatus: "degraded", TaskDefinition: "worker:8", LaunchType: "EC2"},
	}

	for _, query := range []string{"worker", "degraded", "worker:8", "ec2"} {
		got := filterServices(services, query)
		if len(got) != 1 || got[0].Name != "worker" {
			t.Fatalf("filterServices(%q) = %#v", query, got)
		}
	}
}

func TestFindServicePreservesSelectedServiceAcrossRefresh(t *testing.T) {
	services := []model.Service{
		{Name: "api", RunningCount: 2},
		{Name: "worker", RunningCount: 4},
	}

	got, found := findService(services, "worker")
	if !found || got.Name != "worker" || got.RunningCount != 4 {
		t.Fatalf("findService() = %#v, %v", got, found)
	}
	if _, found := findService(services, "removed"); found {
		t.Fatal("findService() found a removed service")
	}
}

func TestFilterAndFindTasks(t *testing.T) {
	tasks := []model.Task{
		{TaskID: "task-api", TaskARN: "arn:api", Status: "RUNNING", PrivateIP: "10.0.0.1"},
		{TaskID: "task-worker", TaskARN: "arn:worker", Status: "STOPPED", StopCode: "EssentialContainerExited", StoppedReason: "essential container exited", Containers: []model.Container{{ExitCode: intPtr(17)}}},
	}
	for _, query := range []string{"worker", "stopped", "essential", "17"} {
		got := filterTasks(tasks, query)
		if len(got) != 1 || got[0].TaskARN != "arn:worker" {
			t.Fatalf("filterTasks(%q) = %#v", query, got)
		}
	}
	got, found := findTask(tasks, "arn:api")
	if !found || got.TaskID != "task-api" {
		t.Fatalf("findTask() = %#v, %v", got, found)
	}
}

func intPtr(value int) *int {
	return &value
}

func TestClusterSummary(t *testing.T) {
	if got := clusterSummary("production", 0); got != "No ECS services found in production." {
		t.Fatalf("clusterSummary(empty) = %q", got)
	}
	got := clusterSummary("production", 3)
	if !strings.Contains(got, "production") || !strings.Contains(got, "3 services") {
		t.Fatalf("clusterSummary() = %q", got)
	}
}

func TestStandaloneTaskSummary(t *testing.T) {
	if got := standaloneTaskSummary("prod", nil); got != "No active standalone ECS tasks found in prod." {
		t.Fatalf("standaloneTaskSummary(empty) = %q", got)
	}
	got := standaloneTaskSummary("prod", []model.Task{{TaskID: "one"}, {TaskID: "two"}})
	if !strings.Contains(got, "2 active standalone tasks") {
		t.Fatalf("standaloneTaskSummary() = %q", got)
	}
}

func TestStoppedStandaloneTaskPresentation(t *testing.T) {
	now := time.Now()
	zero, failed := 0, 17
	tasks := []model.Task{
		{TaskARN: "older", StoppedAt: now.Add(-time.Minute), Containers: []model.Container{{Name: "worker", ExitCode: &zero}}},
		{TaskARN: "newer", StoppedAt: now, Containers: []model.Container{{Name: "app", ExitCode: &zero}, {Name: "sidecar", ExitCode: &failed}}},
	}
	sortStoppedTasks(tasks)
	if tasks[0].TaskARN != "newer" {
		t.Fatalf("sortStoppedTasks() = %#v", tasks)
	}
	if got := taskExitSummary(tasks[0]); got != "app=0, sidecar=17" {
		t.Fatalf("taskExitSummary() = %q", got)
	}
	summary := stoppedStandaloneTaskSummary("prod", tasks, true)
	if !strings.Contains(summary, "2 recently stopped") || !strings.Contains(summary, "more available") {
		t.Fatalf("stoppedStandaloneTaskSummary() = %q", summary)
	}
	combined := appendUniqueTasks(tasks[:1], []model.Task{tasks[0], tasks[1]})
	if len(combined) != 2 {
		t.Fatalf("appendUniqueTasks() = %#v", combined)
	}
}

func TestSplitCommaSeparated(t *testing.T) {
	got := splitCommaSeparated(" subnet-a,subnet-b, , subnet-c ")
	want := []string{"subnet-a", "subnet-b", "subnet-c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitCommaSeparated() = %#v, want %#v", got, want)
	}
}

func TestFormatServiceDetail(t *testing.T) {
	created := time.Date(2026, 8, 17, 20, 0, 0, 0, time.Local)
	svc := model.Service{
		Name:                 "api",
		Status:               "ACTIVE",
		HealthStatus:         "healthy",
		DesiredCount:         2,
		RunningCount:         2,
		TaskDefinition:       "api:42",
		EnableExecuteCommand: true,
		CreatedAt:            created,
		Deployments: []model.Deployment{{
			Status: "PRIMARY", RolloutState: "COMPLETED", RunningCount: 2, DesiredCount: 2,
		}},
		Events: []model.ServiceEvent{{Message: "deployment completed", CreatedAt: created}},
	}
	tasks := []model.Task{{TaskID: "1234567890abcdef", Status: "RUNNING", HealthStatus: "HEALTHY"}}

	got := formatServiceDetail("prod", svc, tasks)
	for _, want := range []string{"prod / api", "api:42", "DEPLOYMENTS", "PRIMARY", "TASKS", "1234567890ab", "RECENT EVENTS", "deployment completed"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatServiceDetail() missing %q:\n%s", want, got)
		}
	}
}

func TestFormatServiceStoppedTasksDetail(t *testing.T) {
	exitCode := 137
	stoppedAt := time.Date(2026, 8, 18, 10, 30, 0, 0, time.Local)
	got := formatServiceStoppedTasksDetail("prod", model.Service{Name: "api"}, []model.Task{{
		TaskID: "1234567890abcdef", StoppedAt: stoppedAt, StopCode: "EssentialContainerExited",
		Containers: []model.Container{{Name: "api", ExitCode: &exitCode}},
	}}, true)
	for _, want := range []string{"RECENTLY STOPPED TASKS", "1 LOADED", "MORE AVAILABLE", "1234567890ab", "exit 137", "EssentialContainerExited"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatServiceStoppedTasksDetail() missing %q:\n%s", want, got)
		}
	}
}

func TestFormatTaskDetail(t *testing.T) {
	exitCode := 17
	task := model.Task{
		TaskID:           "1234567890abcdef",
		TaskARN:          "arn:aws:ecs:task/1234567890abcdef",
		TaskDefinition:   "api:42",
		Status:           "STOPPED",
		DesiredStatus:    "STOPPED",
		HealthStatus:     "UNHEALTHY",
		LaunchType:       "FARGATE",
		AvailabilityZone: "us-east-1a",
		PrivateIP:        "10.0.0.5",
		StopCode:         "EssentialContainerExited",
		StoppedReason:    "container failed",
		Containers: []model.Container{{
			Name: "api", Image: "example/api:42", Status: "STOPPED",
			ExitCode: &exitCode, Reason: "process exited", LogGroup: "/ecs/api",
		}},
	}

	got := formatTaskDetail(task)
	for _, want := range []string{"1234567890abcdef", "api:42", "10.0.0.5", "EssentialContainerExited", "container failed", "example/api:42", "Exit code    17", "/ecs/api"} {
		if !strings.Contains(got, want) {
			t.Fatalf("formatTaskDetail() missing %q:\n%s", want, got)
		}
	}
}
