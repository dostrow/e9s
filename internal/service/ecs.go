// Package service implements UI-neutral e9s workflows shared by the TUI and GUI.
package service

import (
	"context"
	"fmt"
	"strings"

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
	GetLogConfig(context.Context, string, string) (string, string, error)
	ResolveTaskLogStreams(context.Context, []model.Task) (string, []string, error)
}

// ECS exposes ECS workflows without depending on either frontend toolkit.
type ECS struct {
	api ECSAPI
}

func NewECS(api ECSAPI) *ECS {
	return &ECS{api: api}
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
