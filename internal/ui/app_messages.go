package ui

import (
	"time"

	e9saws "github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/tofu"
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
type scaleInStatusMsg struct {
	service   string
	cluster   string
	suspended bool
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
type smCloneReadyMsg struct {
	sourceName string
	value      string
}
type smCloneEditedMsg struct {
	name  string
	value string
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
	lastKey   any
}
type dynamoItemsLoadedMsg struct {
	items   []e9saws.DynamoItem
	hasMore bool
	lastKey any
}
type dynamoPageLoadedMsg struct {
	items   []e9saws.DynamoItem
	hasMore bool
	lastKey any
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

// --- SQS Messages ---

type sqsQueuesLoadedMsg struct{ queues []e9saws.SQSQueue }
type sqsStatsLoadedMsg struct{ stats *e9saws.SQSQueueStats }
type sqsMessagesReceivedMsg struct{ messages []e9saws.SQSMessage }
type sqsDLQResolvedMsg struct {
	name string
	url  string
}
type sqsSendReadyMsg struct {
	queueURL string
	template *e9saws.SQSSendTemplate
}

// --- Lambda Messages ---

type lambdaFunctionsLoadedMsg struct{ functions []e9saws.LambdaFunction }
type lambdaCodeReadyMsg struct {
	functionName string
	dir          string // temp directory with extracted code
}
type lambdaCodeEditedMsg struct {
	functionName string
	zipData      []byte
}
type lambdaCodeUpdatedMsg struct{ message string }

// --- CloudWatch Logs Messages ---

type logGroupsLoadedMsg struct{ groups []e9saws.LogGroupInfo }
type logStreamsLoadedMsg struct{ streams []e9saws.LogStreamInfo }

// --- CloudWatch Alarms Messages ---

type alarmsLoadedMsg struct{ alarms []e9saws.CWAlarm }
type alarmDetailLoadedMsg struct{ detail *e9saws.CWAlarmDetail }
type alarmActionDoneMsg struct {
	message   string
	alarmName string
}

// --- CodeBuild Messages ---

type cbProjectsLoadedMsg struct{ projects []e9saws.CBProject }
type cbBuildsLoadedMsg struct{ builds []e9saws.CBBuild }
type cbBuildDetailLoadedMsg struct{ detail *e9saws.CBBuildDetail }
type cbBuildStartedMsg struct{ message string }
type cbBuildStoppedMsg struct{ message string }

// --- EC2 Messages ---

type ec2InstancesLoadedMsg struct{ instances []e9saws.EC2Instance }
type ec2DetailLoadedMsg struct{ detail *e9saws.EC2InstanceDetail }
type ec2ConsoleLoadedMsg struct{ output string }
type ec2ActionDoneMsg struct{ message string }

// --- ECR Messages ---

type ecrReposLoadedMsg struct{ repos []e9saws.ECRRepo }
type ecrImagesLoadedMsg struct{ images []e9saws.ECRImage }
type ecrFindingsLoadedMsg struct{ findings []e9saws.ECRFinding }
type ecrActionDoneMsg struct{ message string }

// --- Route53 Messages ---

type r53ZonesLoadedMsg struct{ zones []e9saws.R53Zone }
type r53RecordsLoadedMsg struct{ records []e9saws.R53Record }
type r53DNSAnswerMsg struct{ answer *e9saws.R53DNSAnswer }
type r53RecordEditedMsg struct {
	record *e9saws.R53Record
	isNew  bool
}
type r53ActionDoneMsg struct{ message string }

// --- OpenTofu Messages ---

type tofuResourcesLoadedMsg struct{ resources []string }
type tofuStateDetailMsg struct{ output string }
type tofuPlanLoadedMsg struct{ plan *tofu.PlanResult }
type tofuApplyDoneMsg struct{ message string }
type tofuInitDoneMsg struct{ message string }

// --- Shared Messages ---

type regionSwitchedMsg struct{}
type showModeSwitcherMsg struct{}
type configEditedMsg struct{}
type configCheckMsg struct{}
