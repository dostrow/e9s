// Package model defines the core ECS data types and transforms from AWS SDK types.
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
	Name                 string
	Status               string
	DesiredCount         int
	RunningCount         int
	PendingCount         int
	TaskDefinition       string // family:revision
	LaunchType           string
	Deployments          []Deployment
	Events               []ServiceEvent
	CreatedAt            time.Time
	HealthStatus         string // "healthy", "degraded", "unhealthy"
	EnableExecuteCommand bool
	TargetGroups         []ResourceRef
	SecurityGroups       []EC2SecurityGroupRef
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
	TaskID               string // short ID extracted from ARN
	TaskARN              string
	TaskDefinition       string
	Status               string // PROVISIONING, PENDING, ACTIVATING, RUNNING, DEACTIVATING, STOPPING, STOPPED
	HealthStatus         string
	DesiredStatus        string
	LaunchType           string
	StartedAt            time.Time
	StoppedAt            time.Time
	StopCode             string
	StoppedReason        string
	Containers           []Container
	PrivateIP            string
	NetworkInterfaceID   string
	SubnetID             string
	VpcID                string
	EC2InstanceID        string
	ContainerInstanceARN string
	SecurityGroups       []EC2SecurityGroupRef
	VolumeIDs            []string
	ResourceWarnings     []string
	AvailabilityZone     string
	Group                string // "service:name" or "family:name"
	ExecAgentRunning     bool   // whether the ExecuteCommandAgent managed agent is running
}

// TaskPage is a bounded page of tasks plus the opaque ECS continuation token.
type TaskPage struct {
	Tasks     []Task
	NextToken string
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

// RunTaskRequest contains the UI-neutral options for starting standalone ECS
// tasks. Empty LaunchType uses the cluster's default capacity-provider strategy.
type RunTaskRequest struct {
	Cluster              string
	TaskDefinition       string
	LaunchType           string
	Count                int
	Subnets              []string
	SecurityGroups       []string
	AssignPublicIP       bool
	EnableExecuteCommand bool
	Group                string
}

type ServiceMetrics struct {
	CPUAvg          float64
	CPUMax          float64
	MemAvg          float64
	MemMax          float64
	CPUAvgAvailable bool
	CPUMaxAvailable bool
	MemAvgAvailable bool
	MemMaxAvailable bool
	Timestamp       time.Time
	StartTime       time.Time
	EndTime         time.Time
	Period          time.Duration
	Series          []MetricSeries
}

// MetricPoint is one timestamped sample in a CloudWatch metric series.
type MetricPoint struct {
	Timestamp time.Time
	Value     float64
}

// MetricSeries is a UI-neutral time series. ID is stable for callers while
// Label and Unit are intended for presentation by either frontend.
type MetricSeries struct {
	ID        string
	Label     string
	Unit      string
	Statistic string
	Points    []MetricPoint
}

// MetricDimension identifies one CloudWatch metric dimension.
type MetricDimension struct {
	Name  string
	Value string
}

// MetricQuery describes a single CloudWatch series without exposing SDK
// values to the shared service and presentation layers.
type MetricQuery struct {
	ID         string
	Label      string
	Namespace  string
	MetricName string
	Dimensions []MetricDimension
	Statistic  string
	Unit       string
	Scale      float64
}

// MetricRequest describes a bounded time-series query. A zero Period lets the
// CloudWatch adapter select a resolution that stays within MaxPoints.
type MetricRequest struct {
	StartTime time.Time
	EndTime   time.Time
	Period    time.Duration
	MaxPoints int
	Queries   []MetricQuery
}

// MetricSnapshot is the complete result of a time-series request.
type MetricSnapshot struct {
	StartTime time.Time
	EndTime   time.Time
	Period    time.Duration
	Series    []MetricSeries
}

type AlarmState struct {
	Name       string
	State      string
	MetricName string
	UpdatedAt  time.Time
}
