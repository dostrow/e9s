package model

import "time"

type Cluster struct {
	Name           string
	ARN            string
	ActiveServices int
	RunningTasks   int
	PendingTasks   int
	Status         string
}

type Service struct {
	Name           string
	Status         string
	DesiredCount   int
	RunningCount   int
	PendingCount   int
	TaskDefinition string // family:revision
	LaunchType     string
	Deployments    []Deployment
	Events         []ServiceEvent
	CreatedAt            time.Time
	HealthStatus         string // "healthy", "degraded", "unhealthy"
	EnableExecuteCommand bool
}

type Deployment struct {
	ID             string
	Status         string // PRIMARY, ACTIVE, INACTIVE
	DesiredCount   int
	RunningCount   int
	PendingCount   int
	FailedCount    int
	TaskDefinition string
	RolloutState   string // COMPLETED, IN_PROGRESS, FAILED
	CreatedAt      time.Time
}

type ServiceEvent struct {
	ID        string
	Message   string
	CreatedAt time.Time
}

type Task struct {
	TaskID         string // short ID extracted from ARN
	TaskARN        string
	TaskDefinition string
	Status         string // PROVISIONING, PENDING, ACTIVATING, RUNNING, DEACTIVATING, STOPPING, STOPPED
	HealthStatus   string
	DesiredStatus  string
	LaunchType     string
	StartedAt      time.Time
	StoppedAt      time.Time
	StoppedReason  string
	Containers        []Container
	PrivateIP         string
	Group             string // "service:name" or "family:name"
	ExecAgentRunning  bool   // whether the ExecuteCommandAgent managed agent is running
}

type Container struct {
	Name         string
	Image        string
	Status       string
	HealthStatus string
	ExitCode     *int
	Reason       string
	LogGroup     string
	LogStream    string
}
