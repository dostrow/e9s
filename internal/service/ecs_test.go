package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

type fakeECSAPI struct {
	clusters []model.Cluster
	services []model.Service
	tasks    []model.Task
	err      error
	forced   []string
	scaled   []any
	stopped  []string
	run      model.RunTaskRequest
}

func (f *fakeECSAPI) ListClusters(context.Context) ([]model.Cluster, error) {
	return f.clusters, f.err
}

func (f *fakeECSAPI) ListServices(context.Context, string) ([]model.Service, error) {
	return f.services, f.err
}

func (f *fakeECSAPI) ListTasks(context.Context, string, string) ([]model.Task, error) {
	return f.tasks, f.err
}

func (f *fakeECSAPI) ForceNewDeployment(_ context.Context, cluster, service string) error {
	f.forced = []string{cluster, service}
	return f.err
}

func (f *fakeECSAPI) ScaleService(_ context.Context, cluster, service string, count int) error {
	f.scaled = []any{cluster, service, count}
	return f.err
}

func (f *fakeECSAPI) StopTask(_ context.Context, cluster, taskARN, reason string) error {
	f.stopped = []string{cluster, taskARN, reason}
	return f.err
}

func (f *fakeECSAPI) RunTask(_ context.Context, request model.RunTaskRequest) ([]model.Task, error) {
	f.run = request
	return f.tasks, f.err
}

func (f *fakeECSAPI) GetLogConfig(context.Context, string, string) (string, string, error) {
	return "/ecs/example", "ecs", f.err
}

func (f *fakeECSAPI) ResolveTaskLogStreams(context.Context, []model.Task) (string, []string, error) {
	return "/ecs/example", []string{"ecs/api/task-1"}, f.err
}

func TestECSListsDomainValues(t *testing.T) {
	api := &fakeECSAPI{
		clusters: []model.Cluster{{Name: "prod"}},
		services: []model.Service{{Name: "api"}},
		tasks:    []model.Task{{TaskID: "task-1"}},
	}
	svc := NewECS(api)

	clusters, err := svc.ListClusters(context.Background())
	if err != nil || len(clusters) != 1 || clusters[0].Name != "prod" {
		t.Fatalf("ListClusters() = %#v, %v", clusters, err)
	}
	services, err := svc.ListServices(context.Background(), "prod")
	if err != nil || len(services) != 1 || services[0].Name != "api" {
		t.Fatalf("ListServices() = %#v, %v", services, err)
	}
	tasks, err := svc.ListTasks(context.Background(), "prod", "api")
	if err != nil || len(tasks) != 1 || tasks[0].TaskID != "task-1" {
		t.Fatalf("ListTasks() = %#v, %v", tasks, err)
	}
}

func TestECSLogSources(t *testing.T) {
	api := &fakeECSAPI{tasks: []model.Task{{
		TaskID:         "task-1",
		TaskDefinition: "api:1",
	}}}
	svc := NewECS(api)

	container, err := svc.ContainerLogSource(context.Background(), api.tasks[0], "api")
	if err != nil {
		t.Fatal(err)
	}
	if container.Group != "/ecs/example" || len(container.Streams) != 1 || container.Streams[0] != "ecs/api/task-1" {
		t.Fatalf("ContainerLogSource() = %#v", container)
	}

	service, err := svc.ServiceLogSource(context.Background(), "prod", "api")
	if err != nil {
		t.Fatal(err)
	}
	if service.Group != "/ecs/example" || len(service.Streams) != 1 {
		t.Fatalf("ServiceLogSource() = %#v", service)
	}
}

func TestECSWrapsOperationErrors(t *testing.T) {
	svc := NewECS(&fakeECSAPI{err: errors.New("denied")})

	_, err := svc.ListServices(context.Background(), "prod")
	if err == nil || !strings.Contains(err.Error(), "list ECS services") || !errors.Is(err, svc.api.(*fakeECSAPI).err) {
		t.Fatalf("ListServices() error = %v", err)
	}
}

func TestECSForceDeployment(t *testing.T) {
	api := &fakeECSAPI{}
	svc := NewECS(api)
	if err := svc.ForceDeployment(context.Background(), "prod", "api"); err != nil {
		t.Fatal(err)
	}
	if len(api.forced) != 2 || api.forced[0] != "prod" || api.forced[1] != "api" {
		t.Fatalf("forced arguments = %#v", api.forced)
	}
}

func TestECSScaleAndStop(t *testing.T) {
	api := &fakeECSAPI{}
	svc := NewECS(api)
	if err := svc.ScaleService(context.Background(), "prod", "api", 4); err != nil {
		t.Fatal(err)
	}
	if len(api.scaled) != 3 || api.scaled[0] != "prod" || api.scaled[1] != "api" || api.scaled[2] != 4 {
		t.Fatalf("scaled arguments = %#v", api.scaled)
	}
	if err := svc.StopTask(context.Background(), "prod", "arn:task/one", "Stopped by e9s"); err != nil {
		t.Fatal(err)
	}
	if len(api.stopped) != 3 || api.stopped[0] != "prod" || api.stopped[1] != "arn:task/one" || api.stopped[2] != "Stopped by e9s" {
		t.Fatalf("stopped arguments = %#v", api.stopped)
	}
}

func TestECSRejectsNegativeScale(t *testing.T) {
	api := &fakeECSAPI{}
	err := NewECS(api).ScaleService(context.Background(), "prod", "api", -1)
	if err == nil || !strings.Contains(err.Error(), "cannot be negative") {
		t.Fatalf("ScaleService() error = %v", err)
	}
	if api.scaled != nil {
		t.Fatalf("low-level ScaleService called with %#v", api.scaled)
	}
}

func TestECSStandaloneTasks(t *testing.T) {
	api := &fakeECSAPI{tasks: []model.Task{
		{TaskID: "service-task", Group: "service:api"},
		{TaskID: "scheduled-task", Group: "family:nightly"},
		{TaskID: "plain-task"},
	}}
	tasks, err := NewECS(api).ListStandaloneTasks(context.Background(), "prod")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].TaskID != "scheduled-task" || tasks[1].TaskID != "plain-task" {
		t.Fatalf("ListStandaloneTasks() = %#v", tasks)
	}
}

func TestECSRunTaskNormalizesAndValidates(t *testing.T) {
	api := &fakeECSAPI{tasks: []model.Task{{TaskID: "started"}}}
	svc := NewECS(api)
	tasks, err := svc.RunTask(context.Background(), model.RunTaskRequest{
		Cluster: " prod ", TaskDefinition: " nightly:7 ", LaunchType: " fargate ", Count: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || api.run.Cluster != "prod" || api.run.TaskDefinition != "nightly:7" || api.run.LaunchType != "FARGATE" || api.run.Count != 2 {
		t.Fatalf("RunTask() = %#v; request = %#v", tasks, api.run)
	}

	for name, request := range map[string]model.RunTaskRequest{
		"cluster":         {TaskDefinition: "nightly:7", Count: 1},
		"task definition": {Cluster: "prod", Count: 1},
		"count":           {Cluster: "prod", TaskDefinition: "nightly:7"},
		"launch type":     {Cluster: "prod", TaskDefinition: "nightly:7", Count: 1, LaunchType: "spaceship"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.RunTask(context.Background(), request); err == nil {
				t.Fatal("RunTask() succeeded, want validation error")
			}
		})
	}
}
