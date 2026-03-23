package model

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

func strPtr(s string) *string      { return &s }
func timePtr(t time.Time) *time.Time { return &t }

func TestTransformCluster(t *testing.T) {
	c := types.Cluster{
		ClusterName:         strPtr("test-cluster"),
		ClusterArn:          strPtr("arn:aws:ecs:us-east-1:123456:cluster/test-cluster"),
		ActiveServicesCount:  3,
		RunningTasksCount:    10,
		PendingTasksCount:    2,
		Status:              strPtr("ACTIVE"),
	}

	result := TransformCluster(c)

	if result.Name != "test-cluster" {
		t.Errorf("Name = %q, want %q", result.Name, "test-cluster")
	}
	if result.ActiveServices != 3 {
		t.Errorf("ActiveServices = %d, want 3", result.ActiveServices)
	}
	if result.RunningTasks != 10 {
		t.Errorf("RunningTasks = %d, want 10", result.RunningTasks)
	}
	if result.PendingTasks != 2 {
		t.Errorf("PendingTasks = %d, want 2", result.PendingTasks)
	}
	if result.Status != "ACTIVE" {
		t.Errorf("Status = %q, want %q", result.Status, "ACTIVE")
	}
}

func TestTransformClusterNilFields(t *testing.T) {
	c := types.Cluster{}
	result := TransformCluster(c)

	if result.Name != "" {
		t.Errorf("Name = %q, want empty", result.Name)
	}
	if result.Status != "" {
		t.Errorf("Status = %q, want empty", result.Status)
	}
}

func TestTransformService(t *testing.T) {
	now := time.Now()
	s := types.Service{
		ServiceName:    strPtr("my-service"),
		Status:         strPtr("ACTIVE"),
		DesiredCount:   5,
		RunningCount:   5,
		PendingCount:   0,
		TaskDefinition: strPtr("arn:aws:ecs:us-east-1:123456:task-definition/my-task:42"),
		LaunchType:     types.LaunchTypeFargate,
		CreatedAt:      timePtr(now),
		Deployments: []types.Deployment{
			{
				Id:             strPtr("deploy-1"),
				Status:         strPtr("PRIMARY"),
				DesiredCount:   5,
				RunningCount:   5,
				RolloutState:   types.DeploymentRolloutStateCompleted,
				TaskDefinition: strPtr("arn:aws:ecs:us-east-1:123456:task-definition/my-task:42"),
				CreatedAt:      timePtr(now),
			},
		},
	}

	result := TransformService(s)

	if result.Name != "my-service" {
		t.Errorf("Name = %q, want %q", result.Name, "my-service")
	}
	if result.TaskDefinition != "my-task:42" {
		t.Errorf("TaskDefinition = %q, want %q", result.TaskDefinition, "my-task:42")
	}
	if result.HealthStatus != "healthy" {
		t.Errorf("HealthStatus = %q, want %q", result.HealthStatus, "healthy")
	}
	if len(result.Deployments) != 1 {
		t.Fatalf("Deployments count = %d, want 1", len(result.Deployments))
	}
	if result.Deployments[0].Status != "PRIMARY" {
		t.Errorf("Deployment Status = %q, want %q", result.Deployments[0].Status, "PRIMARY")
	}
}

func TestComputeServiceHealth(t *testing.T) {
	tests := []struct {
		name    string
		svc     Service
		want    string
	}{
		{
			name: "healthy - all running",
			svc:  Service{DesiredCount: 3, RunningCount: 3},
			want: "healthy",
		},
		{
			name: "unhealthy - none running",
			svc:  Service{DesiredCount: 3, RunningCount: 0},
			want: "unhealthy",
		},
		{
			name: "degraded - partial",
			svc:  Service{DesiredCount: 3, RunningCount: 1},
			want: "degraded",
		},
		{
			name: "deploying - in progress",
			svc: Service{
				DesiredCount: 3, RunningCount: 3,
				Deployments: []Deployment{{Status: "PRIMARY", RolloutState: "IN_PROGRESS"}},
			},
			want: "deploying",
		},
		{
			name: "unhealthy - failed rollout",
			svc: Service{
				DesiredCount: 3, RunningCount: 3,
				Deployments: []Deployment{{RolloutState: "FAILED"}},
			},
			want: "unhealthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeServiceHealth(tt.svc)
			if got != tt.want {
				t.Errorf("computeServiceHealth() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTransformTask(t *testing.T) {
	now := time.Now()
	task := types.Task{
		TaskArn:           strPtr("arn:aws:ecs:us-east-1:123456:task/cluster/abc123def456"),
		TaskDefinitionArn: strPtr("arn:aws:ecs:us-east-1:123456:task-definition/my-task:5"),
		LastStatus:        strPtr("RUNNING"),
		DesiredStatus:     strPtr("RUNNING"),
		HealthStatus:      types.HealthStatusHealthy,
		LaunchType:        types.LaunchTypeFargate,
		StartedAt:         timePtr(now),
		Group:             strPtr("service:my-service"),
		Containers: []types.Container{
			{
				Name:         strPtr("app"),
				Image:        strPtr("myrepo/app:latest"),
				LastStatus:   strPtr("RUNNING"),
				HealthStatus: types.HealthStatusHealthy,
			},
		},
		Attachments: []types.Attachment{
			{
				Type: strPtr("ElasticNetworkInterface"),
				Details: []types.KeyValuePair{
					{Name: strPtr("privateIPv4Address"), Value: strPtr("10.0.1.42")},
				},
			},
		},
	}

	result := TransformTask(task)

	if result.TaskID != "abc123def456" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "abc123def456")
	}
	if result.TaskDefinition != "my-task:5" {
		t.Errorf("TaskDefinition = %q, want %q", result.TaskDefinition, "my-task:5")
	}
	if result.Status != "RUNNING" {
		t.Errorf("Status = %q, want %q", result.Status, "RUNNING")
	}
	if result.PrivateIP != "10.0.1.42" {
		t.Errorf("PrivateIP = %q, want %q", result.PrivateIP, "10.0.1.42")
	}
	if result.Group != "service:my-service" {
		t.Errorf("Group = %q, want %q", result.Group, "service:my-service")
	}
	if len(result.Containers) != 1 {
		t.Fatalf("Containers count = %d, want 1", len(result.Containers))
	}
	if result.Containers[0].Name != "app" {
		t.Errorf("Container Name = %q, want %q", result.Containers[0].Name, "app")
	}
}

func TestShortTaskDef(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"arn:aws:ecs:us-east-1:123456:task-definition/my-task:42", "my-task:42"},
		{"my-task:42", "my-task:42"},
		{"", ""},
	}
	for _, tt := range tests {
		got := shortTaskDef(tt.input)
		if got != tt.want {
			t.Errorf("shortTaskDef(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestShortTaskID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"arn:aws:ecs:us-east-1:123456:task/cluster/abc123", "abc123"},
		{"abc123", "abc123"},
		{"", ""},
	}
	for _, tt := range tests {
		got := shortTaskID(tt.input)
		if got != tt.want {
			t.Errorf("shortTaskID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDerefStr(t *testing.T) {
	s := "hello"
	if got := derefStr(&s); got != "hello" {
		t.Errorf("derefStr(&hello) = %q", got)
	}
	if got := derefStr(nil); got != "" {
		t.Errorf("derefStr(nil) = %q", got)
	}
}

func TestDerefTime(t *testing.T) {
	now := time.Now()
	if got := derefTime(&now); !got.Equal(now) {
		t.Errorf("derefTime(&now) != now")
	}
	if got := derefTime(nil); !got.IsZero() {
		t.Errorf("derefTime(nil) should be zero")
	}
}
