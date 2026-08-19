package ui

import (
	"time"

	e9saws "github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/tofu"
)

// --- ECS Messages ---

type clustersLoadedMsg struct{ clusters []model.Cluster }
type servicesLoadedMsg struct {
	cluster  string
	services []model.Service
}
type tasksLoadedMsg struct {
	cluster   string
	service   string
	tasks     []model.Task
	stopped   bool
	append    bool
	nextToken string
}
type standaloneTasksLoadedMsg struct {
	cluster   string
	tasks     []model.Task
	stopped   bool
	append    bool
	nextToken string
}
type taskDetailRefreshedMsg struct {
	taskARN string
	task    *model.Task
}
type taskDefsLoadedMsg struct{ defs []e9saws.TaskDefRef }
type taskDefLoadedMsg struct {
	taskDefinition string
	def            *e9saws.TaskDefSummary
}
type errMsg struct{ err error }
type tickMsg time.Time
type actionSuccessMsg struct{ message string }
type runTaskStartedMsg struct {
	count          int
	taskDefinition string
	cluster        string
}
type logReadyMsg struct {
	title          string
	logGroup       string
	logGroups      []string
	streams        []string
	follow         *bool         // nil = default (true), false = paused
	lookback       time.Duration // 0 = default (15min)
	search         string        // pre-set search pattern
	startMs        int64         // absolute range start (paused viewer)
	endMs          int64         // absolute range end (paused viewer)
	highlightRules []model.LogHighlightRule
	hiddenStreams  []string
	anchor         *model.LogEntry
	savedLogPath   string
	ecsGuard       bool
	returnState    viewState
	cluster        string
	service        string
	taskARN        string
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
	title       string
	diff        string
	returnState viewState
}
type envVarsReadyMsg struct {
	title          string
	envVars        []e9saws.EnvVar
	taskDefinition string
	container      string
	resolved       bool
	returnState    viewState
}
type taskDefinitionEditedMsg struct{ document string }
type taskDefinitionRegisteredMsg struct {
	baseDefinition string
	definition     *e9saws.TaskDefSummary
}
type metricsLoadedMsg struct {
	cluster        string
	service        string
	taskARN        string
	metrics        *e9saws.ServiceMetrics
	alarms         []e9saws.AlarmState
	scaleKnown     bool
	scaleSuspended bool
	warnings       []string
}

// --- SSM Messages ---

type ssmParamsLoadedMsg struct{ params []model.Parameter }
type ssmEditReadyMsg struct {
	name         string
	currentValue string
	paramType    string
}
type ssmUpdatedMsg struct {
	name   string
	params []model.Parameter
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
type smEditedMsg struct {
	name  string
	value string
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

type alarmsLoadedMsg struct{ alarms []model.Alarm }
type alarmDetailLoadedMsg struct{ detail *model.AlarmDetail }
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
type tofuPlanLoadedMsg struct {
	plan     *tofu.PlanResult
	planFile string
}
type tofuApplyDoneMsg struct{ message string }
type tofuInitDoneMsg struct{ message string }

// --- RDS Messages ---

type rdsInstancesLoadedMsg struct{ instances []e9saws.RDSInstance }
type rdsDetailLoadedMsg struct{ detail *e9saws.RDSInstanceDetail }

// --- Shared Messages ---

type regionSwitchedMsg struct{}
type showModeSwitcherMsg struct{}
type configEditedMsg struct{}
type configCheckMsg struct{}
