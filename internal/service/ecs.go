// Package service implements UI-neutral e9s workflows shared by the TUI and GUI.
package service

import (
	"context"
	"fmt"

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

func (s *ECS) ForceDeployment(ctx context.Context, cluster, service string) error {
	if err := s.api.ForceNewDeployment(ctx, cluster, service); err != nil {
		return fmt.Errorf("force deployment for %q in %q: %w", service, cluster, err)
	}
	return nil
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
