package ui

import (
	"time"

	e9saws "github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/model"
)

// --- ECS Messages ---

type clustersLoadedMsg struct{ clusters []model.Cluster }
type servicesLoadedMsg struct{ services []model.Service }
type tasksLoadedMsg struct{ tasks []model.Task }
type standaloneTasksLoadedMsg struct{ tasks []model.Task }
type taskDetailRefreshedMsg struct{ task *model.Task }
type errMsg struct{ err error }
type tickMsg time.Time
type actionSuccessMsg struct{ message string }
type logReadyMsg struct {
	title    string
	logGroup string
	streams  []string
	follow   *bool         // nil = default (true), false = paused
	lookback time.Duration // 0 = default (5min)
	search   string        // pre-set search pattern
}
type execFinishedMsg struct{ err error }
type execSessionReadyMsg struct {
	pluginPath string
	args       []string
}
type taskDefDiffReadyMsg struct {
	title string
	diff  string
}
type envVarsReadyMsg struct {
	title   string
	envVars []e9saws.EnvVar
}
type metricsLoadedMsg struct {
	metrics *e9saws.ServiceMetrics
	alarms  []e9saws.AlarmState
}

// --- SSM Messages ---

type ssmParamsLoadedMsg struct{ params []e9saws.Parameter }
type ssmEditReadyMsg struct {
	name         string
	currentValue string
	paramType    string
}
type ssmUpdatedMsg struct {
	name   string
	params []e9saws.Parameter
}

// --- Secrets Manager Messages ---

type smSecretsLoadedMsg struct{ secrets []e9saws.Secret }
type smValueReadyMsg struct {
	name  string
	value string
	tags  map[string]string
}
type smEditReadyMsg struct {
	name         string
	currentValue string
}
type smUpdatedMsg struct {
	name    string
	secrets []e9saws.Secret
}

// --- S3 Messages ---

type s3BucketsLoadedMsg struct{ buckets []e9saws.S3Bucket }
type s3ObjectsLoadedMsg struct{ objects []e9saws.S3Object }
type s3DetailLoadedMsg struct {
	bucket string
	detail *e9saws.S3ObjectDetail
}
type s3DownloadDoneMsg struct {
	message string
	err     error
}

// --- DynamoDB Messages ---

type dynamoTablesLoadedMsg struct{ tables []string }
type dynamoScanReadyMsg struct {
	tableName string
	keyNames  []string
	items     []e9saws.DynamoItem
	hasMore   bool
}
type dynamoItemsLoadedMsg struct {
	items   []e9saws.DynamoItem
	hasMore bool
}
type dynamoPartiQLResultMsg struct {
	items []e9saws.DynamoItem
	err   error
}

type dynamoItemRefreshedMsg struct {
	item *e9saws.DynamoItem
}
type dynamoFieldEditedMsg struct {
	tableName string
	keyNames  []string
	item      *e9saws.DynamoItem
	fieldName string
	newValue  string
}
type dynamoItemClonedMsg struct {
	tableName string
	newItem   e9saws.DynamoItem
}
type dynamoWriteDoneMsg struct {
	message string
	err     error
}

// --- Lambda Messages ---

type lambdaFunctionsLoadedMsg struct{ functions []e9saws.LambdaFunction }

// --- CloudWatch Messages ---

type logGroupsLoadedMsg struct{ groups []e9saws.LogGroupInfo }
type logStreamsLoadedMsg struct{ streams []e9saws.LogStreamInfo }

// --- Shared Messages ---

type regionSwitchedMsg struct{}
type showModeSwitcherMsg struct{}
