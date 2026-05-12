package model

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func TransformCluster(c types.Cluster) Cluster {
	return Cluster{
		Name:           derefStr(c.ClusterName),
		ARN:            derefStr(c.ClusterArn),
		ActiveServices: int(c.ActiveServicesCount),
		RunningTasks:   int(c.RunningTasksCount),
		PendingTasks:   int(c.PendingTasksCount),
		Status:         derefStr(c.Status),
	}
}

func TransformService(s types.Service) Service {
	svc := Service{
		Name:                 derefStr(s.ServiceName),
		Status:               derefStr(s.Status),
		DesiredCount:         int(s.DesiredCount),
		RunningCount:         int(s.RunningCount),
		PendingCount:         int(s.PendingCount),
		TaskDefinition:       shortTaskDef(derefStr(s.TaskDefinition)),
		LaunchType:           string(s.LaunchType),
		CreatedAt:            derefTime(s.CreatedAt),
		EnableExecuteCommand: s.EnableExecuteCommand,
	}

	for _, d := range s.Deployments {
		svc.Deployments = append(svc.Deployments, TransformDeployment(d))
	}

	for _, e := range s.Events {
		svc.Events = append(svc.Events, ServiceEvent{
			ID:        derefStr(e.Id),
			Message:   derefStr(e.Message),
			CreatedAt: derefTime(e.CreatedAt),
		})
	}

	svc.HealthStatus = computeServiceHealth(svc)
	return svc
}

func TransformDeployment(d types.Deployment) Deployment {
	return Deployment{
		ID:             derefStr(d.Id),
		Status:         derefStr(d.Status),
		DesiredCount:   int(d.DesiredCount),
		RunningCount:   int(d.RunningCount),
		PendingCount:   int(d.PendingCount),
		FailedCount:    int(d.FailedTasks),
		TaskDefinition: shortTaskDef(derefStr(d.TaskDefinition)),
		RolloutState:   string(d.RolloutState),
		CreatedAt:      derefTime(d.CreatedAt),
	}
}

func TransformTask(t types.Task) Task {
	task := Task{
		TaskID:           shortTaskID(derefStr(t.TaskArn)),
		TaskARN:          derefStr(t.TaskArn),
		TaskDefinition:   shortTaskDef(derefStr(t.TaskDefinitionArn)),
		Status:           derefStr(t.LastStatus),
		HealthStatus:     string(t.HealthStatus),
		DesiredStatus:    derefStr(t.DesiredStatus),
		LaunchType:       string(t.LaunchType),
		StartedAt:        derefTime(t.StartedAt),
		StoppedAt:        derefTime(t.StoppedAt),
		StoppedReason:    derefStr(t.StoppedReason),
		AvailabilityZone: derefStr(t.AvailabilityZone),
		Group:            derefStr(t.Group),
	}

	for _, a := range t.Attachments {
		if derefStr(a.Type) == "ElasticNetworkInterface" {
			for _, d := range a.Details {
				if derefStr(d.Name) == "privateIPv4Address" {
					task.PrivateIP = derefStr(d.Value)
				}
			}
		}
	}

	for _, c := range t.Containers {
		container := Container{
			Name:         derefStr(c.Name),
			Image:        derefStr(c.Image),
			Status:       derefStr(c.LastStatus),
			HealthStatus: string(c.HealthStatus),
			Reason:       derefStr(c.Reason),
		}
		if c.ExitCode != nil {
			code := int(*c.ExitCode)
			container.ExitCode = &code
		}
		// Check managed agents for ExecuteCommandAgent
		for _, ma := range c.ManagedAgents {
			if ma.Name == "ExecuteCommandAgent" && derefStr(ma.LastStatus) == "RUNNING" {
				task.ExecAgentRunning = true
			}
		}
		// Log configuration comes from task definition, not describe-tasks.
		// We'll extract it separately if needed.
		task.Containers = append(task.Containers, container)
	}

	return task
}

func computeServiceHealth(s Service) string {
	if s.RunningCount == s.DesiredCount && s.DesiredCount > 0 {
		for _, d := range s.Deployments {
			if d.RolloutState == "FAILED" {
				return "unhealthy"
			}
			if d.Status == "PRIMARY" && d.RolloutState == "IN_PROGRESS" {
				return "deploying"
			}
		}
		return "healthy"
	}
	if s.RunningCount == 0 && s.DesiredCount > 0 {
		return "unhealthy"
	}
	if s.RunningCount < s.DesiredCount {
		return "degraded"
	}
	return "healthy"
}

// shortTaskDef extracts "family:revision" from a full task definition ARN.
func shortTaskDef(arn string) string {
	if idx := strings.LastIndex(arn, "/"); idx != -1 {
		return arn[idx+1:]
	}
	return arn
}

// shortTaskID extracts the short task ID from a full task ARN.
func shortTaskID(arn string) string {
	if idx := strings.LastIndex(arn, "/"); idx != -1 {
		return arn[idx+1:]
	}
	return arn
}

func derefStr(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func derefTime(t *time.Time) time.Time {
	if t != nil {
		return *t
	}
	return time.Time{}
}
