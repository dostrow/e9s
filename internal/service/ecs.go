// Package service implements UI-neutral e9s workflows shared by the TUI and GUI.
package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/model"
)

// ECSAPI is the low-level AWS behavior needed by the shared ECS workflows.
// It deliberately uses e9s domain values rather than AWS SDK types.
type ECSAPI interface {
	ListClusters(context.Context) ([]model.Cluster, error)
	ListServices(context.Context, string) ([]model.Service, error)
	ListTasks(context.Context, string, string) ([]model.Task, error)
	ForceNewDeployment(context.Context, string, string) error
	ScaleService(context.Context, string, string, int) error
	StopTask(context.Context, string, string, string) error
	RunTask(context.Context, model.RunTaskRequest) ([]model.Task, error)
	GetServiceMetrics(context.Context, string, string, time.Duration) (*model.ServiceMetrics, error)
	GetTaskMetrics(context.Context, string, string, string, string, time.Duration) (*model.ServiceMetrics, error)
	ListAlarms(context.Context, string, string) ([]model.AlarmState, error)
	ScaleInSuspended(context.Context, string, string) (bool, error)
	SetScaleInSuspended(context.Context, string, string, bool) error
	ListTaskDefinitions(context.Context, string) ([]model.TaskDefRef, error)
	GetTaskDefinition(context.Context, string) (*model.TaskDefSummary, error)
	RegisterTaskDefinitionJSON(context.Context, string) (*model.TaskDefSummary, error)
	ResolveEnvVars(context.Context, []model.EnvVar) []model.EnvVar
	ExecuteCommand(context.Context, string, string, string, string) (*model.ExecSession, error)
	GetLogConfig(context.Context, string, string) (string, string, error)
	ResolveTaskLogStreams(context.Context, []model.Task) (string, []string, error)
}

// ECS exposes ECS workflows without depending on either frontend toolkit.
type ECS struct {
	api        ECSAPI
	pluginPath func() (string, error)
}

func NewECS(api ECSAPI) *ECS {
	return &ECS{api: api, pluginPath: aws.SessionManagerPluginPath}
}

func (s *ECS) ListClusters(ctx context.Context) ([]model.Cluster, error) {
	clusters, err := s.api.ListClusters(ctx)
	if err != nil {
		return nil, fmt.Errorf("list ECS clusters: %w", err)
	}
	return clusters, nil
}

func (s *ECS) ListServices(ctx context.Context, cluster string) ([]model.Service, error) {
	services, err := s.api.ListServices(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("list ECS services in %q: %w", cluster, err)
	}
	return services, nil
}

func (s *ECS) ListTasks(ctx context.Context, cluster, service string) ([]model.Task, error) {
	tasks, err := s.api.ListTasks(ctx, cluster, service)
	if err != nil {
		return nil, fmt.Errorf("list ECS tasks in %q: %w", cluster, err)
	}
	return tasks, nil
}

func (s *ECS) ListStandaloneTasks(ctx context.Context, cluster string) ([]model.Task, error) {
	tasks, err := s.ListTasks(ctx, cluster, "")
	if err != nil {
		return nil, err
	}
	standalone := make([]model.Task, 0, len(tasks))
	for _, task := range tasks {
		if !strings.HasPrefix(task.Group, "service:") {
			standalone = append(standalone, task)
		}
	}
	return standalone, nil
}

func (s *ECS) ForceDeployment(ctx context.Context, cluster, service string) error {
	if err := s.api.ForceNewDeployment(ctx, cluster, service); err != nil {
		return fmt.Errorf("force deployment for %q in %q: %w", service, cluster, err)
	}
	return nil
}

func (s *ECS) ScaleService(ctx context.Context, cluster, service string, desiredCount int) error {
	if desiredCount < 0 {
		return fmt.Errorf("scale ECS service %q: desired count cannot be negative", service)
	}
	if err := s.api.ScaleService(ctx, cluster, service, desiredCount); err != nil {
		return fmt.Errorf("scale ECS service %q in %q: %w", service, cluster, err)
	}
	return nil
}

func (s *ECS) StopTask(ctx context.Context, cluster, taskARN, reason string) error {
	if err := s.api.StopTask(ctx, cluster, taskARN, reason); err != nil {
		return fmt.Errorf("stop ECS task %q in %q: %w", taskARN, cluster, err)
	}
	return nil
}

func (s *ECS) RunTask(ctx context.Context, request model.RunTaskRequest) ([]model.Task, error) {
	request.Cluster = strings.TrimSpace(request.Cluster)
	request.TaskDefinition = strings.TrimSpace(request.TaskDefinition)
	request.LaunchType = strings.ToUpper(strings.TrimSpace(request.LaunchType))
	if request.Cluster == "" {
		return nil, fmt.Errorf("run ECS task: cluster is required")
	}
	if request.TaskDefinition == "" {
		return nil, fmt.Errorf("run ECS task: task definition is required")
	}
	if request.Count < 1 {
		return nil, fmt.Errorf("run ECS task: count must be at least 1")
	}
	switch request.LaunchType {
	case "", "FARGATE", "EC2", "EXTERNAL", "MANAGED_INSTANCES":
	default:
		return nil, fmt.Errorf("run ECS task: unsupported launch type %q", request.LaunchType)
	}
	tasks, err := s.api.RunTask(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("run ECS task %q in %q: %w", request.TaskDefinition, request.Cluster, err)
	}
	return tasks, nil
}

func (s *ECS) GetServiceMetrics(ctx context.Context, cluster, service string, period time.Duration) (*model.ServiceMetrics, error) {
	metrics, err := s.api.GetServiceMetrics(ctx, cluster, service, period)
	if err != nil {
		return nil, fmt.Errorf("get metrics for ECS service %q in %q: %w", service, cluster, err)
	}
	return metrics, nil
}

func (s *ECS) GetTaskMetrics(ctx context.Context, cluster, service string, task model.Task, period time.Duration) (*model.ServiceMetrics, error) {
	metrics, err := s.api.GetTaskMetrics(ctx, cluster, service, task.TaskID, task.TaskDefinition, period)
	if err != nil {
		return nil, fmt.Errorf("get metrics for ECS task %q in %q: %w", task.TaskID, cluster, err)
	}
	return metrics, nil
}

func (s *ECS) ListServiceAlarms(ctx context.Context, cluster, service string) ([]model.AlarmState, error) {
	alarms, err := s.api.ListAlarms(ctx, cluster, service)
	if err != nil {
		return nil, fmt.Errorf("list alarms for ECS service %q in %q: %w", service, cluster, err)
	}
	return alarms, nil
}

func (s *ECS) ScaleInSuspended(ctx context.Context, cluster, service string) (bool, error) {
	suspended, err := s.api.ScaleInSuspended(ctx, cluster, service)
	if err != nil {
		return false, fmt.Errorf("get scale-in status for ECS service %q in %q: %w", service, cluster, err)
	}
	return suspended, nil
}

func (s *ECS) SetScaleInSuspended(ctx context.Context, cluster, service string, suspended bool) error {
	if err := s.api.SetScaleInSuspended(ctx, cluster, service, suspended); err != nil {
		return fmt.Errorf("set scale-in suspension for ECS service %q in %q: %w", service, cluster, err)
	}
	return nil
}

func (s *ECS) ListTaskDefinitions(ctx context.Context, familyPrefix string) ([]model.TaskDefRef, error) {
	definitions, err := s.api.ListTaskDefinitions(ctx, strings.TrimSpace(familyPrefix))
	if err != nil {
		return nil, fmt.Errorf("list ECS task definitions: %w", err)
	}
	return definitions, nil
}

func (s *ECS) GetTaskDefinition(ctx context.Context, taskDefinition string) (*model.TaskDefSummary, error) {
	definition, err := s.api.GetTaskDefinition(ctx, strings.TrimSpace(taskDefinition))
	if err != nil {
		return nil, fmt.Errorf("get ECS task definition %q: %w", taskDefinition, err)
	}
	if definition == nil {
		return nil, fmt.Errorf("get ECS task definition %q: empty response", taskDefinition)
	}
	return definition, nil
}

func (s *ECS) TaskDefinitionDiff(ctx context.Context, oldRef, newRef string) (string, error) {
	oldDefinition, err := s.GetTaskDefinition(ctx, oldRef)
	if err != nil {
		return "", err
	}
	newDefinition, err := s.GetTaskDefinition(ctx, newRef)
	if err != nil {
		return "", err
	}
	return aws.DiffTaskDefinitions(oldDefinition, newDefinition), nil
}

func (s *ECS) TaskDefinitionEditorDocument(raw string) (string, error) {
	return aws.PrepareTaskDefinitionJSON(raw)
}

func (s *ECS) RegisterTaskDefinitionJSON(ctx context.Context, raw string) (*model.TaskDefSummary, error) {
	definition, err := s.api.RegisterTaskDefinitionJSON(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("register ECS task definition: %w", err)
	}
	if definition == nil {
		return nil, fmt.Errorf("register ECS task definition: empty response")
	}
	return definition, nil
}

func (s *ECS) TaskDefinitionEnvironment(ctx context.Context, taskDefinition, container string, resolveSecrets bool) ([]model.EnvVar, error) {
	definition, err := s.GetTaskDefinition(ctx, taskDefinition)
	if err != nil {
		return nil, err
	}
	for _, candidate := range definition.Containers {
		if candidate.Name == container {
			environment := append([]model.EnvVar(nil), candidate.EnvVars...)
			if resolveSecrets {
				environment = s.api.ResolveEnvVars(ctx, environment)
			}
			return environment, nil
		}
	}
	return nil, fmt.Errorf("container %q not found in task definition %q", container, taskDefinition)
}

func (s *ECS) PrepareExecSession(ctx context.Context, cluster string, task model.Task, container, command string) (model.ExecLaunch, error) {
	if task.Status != "RUNNING" {
		return model.ExecLaunch{}, fmt.Errorf("start ECS Exec: task is %s, not RUNNING", valueOrUnknown(task.Status))
	}
	if !task.ExecAgentRunning {
		return model.ExecLaunch{}, fmt.Errorf("start ECS Exec: ExecuteCommandAgent is not running on task %q", task.TaskID)
	}
	container = strings.TrimSpace(container)
	if container == "" || !taskHasContainer(task, container) {
		return model.ExecLaunch{}, fmt.Errorf("start ECS Exec: container %q is not present on task %q", container, task.TaskID)
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return model.ExecLaunch{}, fmt.Errorf("start ECS Exec: command is required")
	}
	plugin, err := s.pluginPath()
	if err != nil {
		return model.ExecLaunch{}, err
	}
	session, err := s.api.ExecuteCommand(ctx, cluster, task.TaskARN, container, command)
	if err != nil {
		return model.ExecLaunch{}, fmt.Errorf("start ECS Exec for task %q: %w", task.TaskID, err)
	}
	if session == nil {
		return model.ExecLaunch{}, fmt.Errorf("start ECS Exec for task %q: empty session", task.TaskID)
	}
	args, err := session.BuildPluginArgs()
	if err != nil {
		return model.ExecLaunch{}, fmt.Errorf("build Session Manager plugin arguments: %w", err)
	}
	return model.ExecLaunch{Executable: plugin, Args: args}, nil
}

func taskHasContainer(task model.Task, name string) bool {
	for _, container := range task.Containers {
		if container.Name == name {
			return true
		}
	}
	return false
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "UNKNOWN"
	}
	return value
}

func (s *ECS) ContainerLogSource(ctx context.Context, task model.Task, container string) (model.LogSource, error) {
	group, prefix, err := s.api.GetLogConfig(ctx, task.TaskDefinition, container)
	if err != nil {
		return model.LogSource{}, fmt.Errorf("resolve logs for container %q: %w", container, err)
	}
	return model.LogSource{
		Group:   group,
		Streams: []string{aws.BuildLogStreamName(prefix, container, task.TaskID)},
	}, nil
}

func (s *ECS) ServiceLogSource(ctx context.Context, cluster, service string) (model.LogSource, error) {
	tasks, err := s.ListTasks(ctx, cluster, service)
	if err != nil {
		return model.LogSource{}, err
	}

	group, streams, err := s.api.ResolveTaskLogStreams(ctx, tasks)
	if err != nil {
		return model.LogSource{}, fmt.Errorf("resolve logs for ECS service %q: %w", service, err)
	}
	if group == "" || len(streams) == 0 {
		return model.LogSource{}, fmt.Errorf("no log streams found for service %q", service)
	}
	return model.LogSource{Group: group, Streams: streams}, nil
}
