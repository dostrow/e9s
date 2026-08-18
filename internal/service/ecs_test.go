package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

type fakeECSAPI struct {
	clusters       []model.Cluster
	services       []model.Service
	tasks          []model.Task
	err            error
	forced         []string
	scaled         []any
	stopped        []string
	run            model.RunTaskRequest
	metrics        *model.ServiceMetrics
	taskMetricArgs []string
	alarms         []model.AlarmState
	suspended      bool
	suspensionSet  []any
	definitions    []model.TaskDefRef
	definition     *model.TaskDefSummary
	registered     string
	execSession    *model.ExecSession
	execArgs       []string
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

func (f *fakeECSAPI) GetServiceMetrics(context.Context, string, string, time.Duration) (*model.ServiceMetrics, error) {
	return f.metrics, f.err
}

func (f *fakeECSAPI) GetTaskMetrics(_ context.Context, cluster, service, taskID, taskDefinition string, _ time.Duration) (*model.ServiceMetrics, error) {
	f.taskMetricArgs = []string{cluster, service, taskID, taskDefinition}
	return f.metrics, f.err
}

func (f *fakeECSAPI) ListAlarms(context.Context, string, string) ([]model.AlarmState, error) {
	return f.alarms, f.err
}

func (f *fakeECSAPI) ScaleInSuspended(context.Context, string, string) (bool, error) {
	return f.suspended, f.err
}

func (f *fakeECSAPI) SetScaleInSuspended(_ context.Context, cluster, service string, suspended bool) error {
	f.suspensionSet = []any{cluster, service, suspended}
	return f.err
}

func (f *fakeECSAPI) ListTaskDefinitions(context.Context, string) ([]model.TaskDefRef, error) {
	return f.definitions, f.err
}

func (f *fakeECSAPI) GetTaskDefinition(context.Context, string) (*model.TaskDefSummary, error) {
	return f.definition, f.err
}

func (f *fakeECSAPI) RegisterTaskDefinitionJSON(_ context.Context, raw string) (*model.TaskDefSummary, error) {
	f.registered = raw
	return f.definition, f.err
}

func (f *fakeECSAPI) ResolveEnvVars(_ context.Context, environment []model.EnvVar) []model.EnvVar {
	resolved := append([]model.EnvVar(nil), environment...)
	for i := range resolved {
		resolved[i].ResolvedValue = "resolved:" + resolved[i].Value
	}
	return resolved
}

func (f *fakeECSAPI) ExecuteCommand(_ context.Context, cluster, taskARN, container, command string) (*model.ExecSession, error) {
	f.execArgs = []string{cluster, taskARN, container, command}
	return f.execSession, f.err
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

func TestECSMetricsAlarmsAndScaleIn(t *testing.T) {
	api := &fakeECSAPI{
		metrics:   &model.ServiceMetrics{CPUAvg: 23.5},
		alarms:    []model.AlarmState{{Name: "high-cpu", State: "OK"}},
		suspended: true,
	}
	svc := NewECS(api)
	metrics, err := svc.GetServiceMetrics(context.Background(), "prod", "api", 15*time.Minute)
	if err != nil || metrics.CPUAvg != 23.5 {
		t.Fatalf("GetServiceMetrics() = %#v, %v", metrics, err)
	}
	task := model.Task{TaskID: "task-1", TaskDefinition: "api:7"}
	metrics, err = svc.GetTaskMetrics(context.Background(), "prod", "api", task, 15*time.Minute)
	if err != nil || metrics.CPUAvg != 23.5 {
		t.Fatalf("GetTaskMetrics() = %#v, %v", metrics, err)
	}
	if got := strings.Join(api.taskMetricArgs, ","); got != "prod,api,task-1,api:7" {
		t.Fatalf("GetTaskMetrics() arguments = %q", got)
	}
	alarms, err := svc.ListServiceAlarms(context.Background(), "prod", "api")
	if err != nil || len(alarms) != 1 || alarms[0].Name != "high-cpu" {
		t.Fatalf("ListServiceAlarms() = %#v, %v", alarms, err)
	}
	suspended, err := svc.ScaleInSuspended(context.Background(), "prod", "api")
	if err != nil || !suspended {
		t.Fatalf("ScaleInSuspended() = %v, %v", suspended, err)
	}
	if err := svc.SetScaleInSuspended(context.Background(), "prod", "api", false); err != nil {
		t.Fatal(err)
	}
	if len(api.suspensionSet) != 3 || api.suspensionSet[0] != "prod" || api.suspensionSet[1] != "api" || api.suspensionSet[2] != false {
		t.Fatalf("suspension arguments = %#v", api.suspensionSet)
	}
}

func TestECSTaskDefinitionWorkflows(t *testing.T) {
	api := &fakeECSAPI{
		definitions: []model.TaskDefRef{{ARN: "arn:api:2", Family: "api", Revision: 2}},
		definition: &model.TaskDefSummary{
			Family: "api", Revision: 2,
			Containers: []model.TaskDefContainer{{Name: "api", EnvVars: []model.EnvVar{{Name: "TOKEN", Value: "secret"}}}},
		},
	}
	svc := NewECS(api)
	definitions, err := svc.ListTaskDefinitions(context.Background(), " api ")
	if err != nil || len(definitions) != 1 {
		t.Fatalf("ListTaskDefinitions() = %#v, %v", definitions, err)
	}
	environment, err := svc.TaskDefinitionEnvironment(context.Background(), "api:2", "api", true)
	if err != nil || len(environment) != 1 || environment[0].ResolvedValue != "resolved:secret" {
		t.Fatalf("TaskDefinitionEnvironment() = %#v, %v", environment, err)
	}
	registered, err := svc.RegisterTaskDefinitionJSON(context.Background(), `{"Family":"api"}`)
	if err != nil || registered.Family != "api" || api.registered == "" {
		t.Fatalf("RegisterTaskDefinitionJSON() = %#v, %v", registered, err)
	}
}

func TestECSPrepareExecSession(t *testing.T) {
	api := &fakeECSAPI{execSession: &model.ExecSession{
		SessionID: "session", Region: "us-east-1", Target: "ecs:prod_task_api",
	}}
	svc := NewECS(api)
	svc.pluginPath = func() (string, error) { return "/usr/bin/session-manager-plugin", nil }
	task := model.Task{
		TaskID: "task", TaskARN: "arn:task", Status: "RUNNING", ExecAgentRunning: true,
		Containers: []model.Container{{Name: "api"}},
	}
	launch, err := svc.PrepareExecSession(context.Background(), "prod", task, "api", "/bin/sh")
	if err != nil {
		t.Fatal(err)
	}
	if launch.Executable != "/usr/bin/session-manager-plugin" || len(launch.Args) != 6 {
		t.Fatalf("PrepareExecSession() = %#v", launch)
	}
	if len(api.execArgs) != 4 || api.execArgs[0] != "prod" || api.execArgs[1] != "arn:task" || api.execArgs[2] != "api" || api.execArgs[3] != "/bin/sh" {
		t.Fatalf("execute arguments = %#v", api.execArgs)
	}
}

func TestECSPrepareExecSessionValidatesTask(t *testing.T) {
	svc := NewECS(&fakeECSAPI{})
	svc.pluginPath = func() (string, error) { return "plugin", nil }
	base := model.Task{TaskID: "task", TaskARN: "arn:task", Status: "RUNNING", ExecAgentRunning: true, Containers: []model.Container{{Name: "api"}}}
	tests := []struct {
		name      string
		task      model.Task
		container string
		command   string
	}{
		{name: "status", task: func() model.Task { task := base; task.Status = "STOPPED"; return task }(), container: "api", command: "/bin/sh"},
		{name: "agent", task: func() model.Task { task := base; task.ExecAgentRunning = false; return task }(), container: "api", command: "/bin/sh"},
		{name: "container", task: base, container: "missing", command: "/bin/sh"},
		{name: "command", task: base, container: "api"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := svc.PrepareExecSession(context.Background(), "prod", test.task, test.container, test.command); err == nil {
				t.Fatal("PrepareExecSession() succeeded, want validation error")
			}
		})
	}
}
