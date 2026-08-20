//go:build gui

package gui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotk4/pkg/pangocairo"
	"github.com/dostrow/e9s/internal/model"
)

const (
	taskHistoryBatchSize = 50

	pageClusters          = "clusters"
	pageServices          = "services"
	pageTasks             = "tasks"
	pageStandaloneTasks   = "standalone-tasks"
	pageStoppedTasks      = "stopped-standalone-tasks"
	pageTaskDefinitions   = "task-definitions"
	pageLogGroups         = "cloudwatch-log-groups"
	pageLogStreams        = "cloudwatch-log-streams"
	pageSavedLogSearch    = "cloudwatch-saved-search"
	pageAlarms            = "cloudwatch-alarms"
	pageSSM               = "ssm-parameters"
	pageSecrets           = "secrets-manager"
	pageLambda            = "lambda-functions"
	pageCodeBuildProjects = "codebuild-projects"
	pageCodeBuildBuilds   = "codebuild-builds"
	pageEC2Instances      = "ec2-instances"
	pageEC2LoadBalancers  = "ec2-load-balancers"
	pageEC2TargetGroups   = "ec2-target-groups"
	pageEC2SecurityGroups = "ec2-security-groups"
	pageEC2VPCs           = "ec2-vpcs"
	pageEC2Subnets        = "ec2-subnets"
	pageEC2Volumes        = "ec2-volumes"
	pageECRRepositories   = "ecr-repositories"
	pageECRImages         = "ecr-images"
	pageECRFindings       = "ecr-findings"
	pageRDSInstances      = "rds-instances"
	pageRDSClusters       = "rds-clusters"
	pageS3Buckets         = "s3-buckets"
	pageS3Objects         = "s3-objects"
	pageDynamoTables      = "dynamodb-tables"
	pageDynamoItems       = "dynamodb-items"
	pageSQSQueues         = "sqs-queues"
	pageSQSMessages       = "sqs-messages"
	pageRoute53Zones      = "route53-zones"
	pageRoute53Records    = "route53-records"
	pageModulePicker      = "module-picker"

	detailIntro            = "intro"
	detailClusterSummary   = "cluster-summary"
	detailService          = "service"
	detailTask             = "task"
	detailHelp             = "help"
	detailError            = "error"
	detailLogGroup         = "log-group"
	detailLogStream        = "log-stream"
	detailAlarm            = "alarm"
	detailSSM              = "ssm-parameter"
	detailSecret           = "secret"
	detailLambda           = "lambda-function"
	detailCodeBuild        = "codebuild-build"
	detailEC2              = "ec2-instance"
	detailEC2LoadBalancer  = "ec2-load-balancer"
	detailEC2TargetGroup   = "ec2-target-group"
	detailEC2SecurityGroup = "ec2-security-group"
	detailEC2VPC           = "ec2-vpc"
	detailEC2Subnet        = "ec2-subnet"
	detailEC2Volume        = "ec2-volume"
	detailEC2Console       = "ec2-console"
	detailECRRepository    = "ecr-repository"
	detailECRImage         = "ecr-image"
	detailECRFinding       = "ecr-finding"
	detailRDS              = "rds-instance"
	detailRDSCluster       = "rds-cluster"
	detailS3Bucket         = "s3-bucket"
	detailS3Object         = "s3-object"
	detailDynamoTable      = "dynamodb-table"
	detailDynamoItem       = "dynamodb-item"
	detailSQSQueue         = "sqs-queue"
	detailSQSMessage       = "sqs-message"
	detailRoute53Zone      = "route53-zone"
	detailRoute53Record    = "route53-record"
)

type mainWindow struct {
	ctx     context.Context
	options Options
	window  *gtk.ApplicationWindow

	requestCancel context.CancelFunc
	generation    uint64
	logCancel     context.CancelFunc
	logGeneration uint64
	browserPane   *gtk.Widget
	workspacePane *gtk.Widget
	browserZoom   int
	workspaceZoom int
	zoomTarget    paneZoomTarget

	currentPage                 string
	detailContent               string
	selectedCluster             string
	selectedService             string
	selectedTask                string
	allClusters                 []model.Cluster
	filteredClusters            []model.Cluster
	allServices                 []model.Service
	filteredServices            []model.Service
	allTasks                    []model.Task
	filteredTasks               []model.Task
	allTaskDefinitions          []model.TaskDefRef
	filteredTaskDefinitions     []model.TaskDefRef
	allLogGroups                []model.LogGroup
	filteredLogGroups           []model.LogGroup
	selectedLogGroup            string
	allLogStreams               []model.LogStream
	filteredLogStreams          []model.LogStream
	selectedLogStream           string
	allAlarms                   []model.Alarm
	filteredAlarms              []model.Alarm
	selectedAlarm               string
	alarmStateFilter            string
	alarmDetail                 *model.AlarmDetail
	alarmActionPending          bool
	alarmUTCTime                bool
	allSSMParameters            []model.Parameter
	filteredSSMParameters       []model.Parameter
	selectedSSMParameter        string
	ssmPath                     string
	activeSSMPrefix             string
	ssmDetail                   *model.Parameter
	ssmActionPending            bool
	allSecrets                  []model.Secret
	filteredSecrets             []model.Secret
	selectedSecret              string
	secretNameFilter            string
	activeSavedSecretFilter     string
	secretDetail                *model.SecretValue
	secretActionPending         bool
	allLambdaFunctions          []model.LambdaFunction
	filteredLambdaFunctions     []model.LambdaFunction
	selectedLambdaFunction      string
	lambdaSearchTerm            string
	activeSavedLambdaSearch     string
	lambdaDetail                *model.LambdaFunction
	lambdaEnvironment           []model.EnvVar
	lambdaEnvironmentResolved   bool
	lambdaViewMode              string
	lambdaActionPending         bool
	allCodeBuildProjects        []model.CodeBuildProject
	filteredCodeBuildProjects   []model.CodeBuildProject
	allCodeBuildBuilds          []model.CodeBuildBuild
	filteredCodeBuildBuilds     []model.CodeBuildBuild
	selectedCodeBuildProject    string
	selectedCodeBuild           string
	codeBuildDetail             *model.CodeBuildDetail
	codeBuildActionPending      bool
	allEC2Instances             []model.EC2Instance
	filteredEC2Instances        []model.EC2Instance
	selectedEC2Instance         string
	ec2Detail                   *model.EC2InstanceDetail
	ec2ViewMode                 string
	ec2ActionPending            bool
	allEC2SecurityGroups        []model.EC2SecurityGroup
	filteredEC2SecurityGroups   []model.EC2SecurityGroup
	selectedEC2SecurityGroup    string
	ec2SecurityGroupDetail      *model.EC2SecurityGroup
	allEC2VPCs                  []model.EC2VPC
	filteredEC2VPCs             []model.EC2VPC
	selectedEC2VPC              string
	ec2VPCDetail                *model.EC2VPC
	allEC2Subnets               []model.EC2Subnet
	filteredEC2Subnets          []model.EC2Subnet
	selectedEC2Subnet           string
	ec2SubnetVPCFilter          string
	ec2SubnetDetail             *model.EC2Subnet
	allEC2Volumes               []model.EC2Volume
	filteredEC2Volumes          []model.EC2Volume
	selectedEC2Volume           string
	ec2VolumeDetail             *model.EC2Volume
	allEC2LoadBalancers         []model.EC2LoadBalancer
	filteredEC2LoadBalancers    []model.EC2LoadBalancer
	selectedEC2LoadBalancer     string
	ec2LoadBalancerDetail       *model.EC2LoadBalancer
	allEC2TargetGroups          []model.EC2TargetGroup
	filteredEC2TargetGroups     []model.EC2TargetGroup
	selectedEC2TargetGroup      string
	ec2TargetGroupDetail        *model.EC2TargetGroup
	allECRRepositories          []model.ECRRepo
	filteredECRRepositories     []model.ECRRepo
	selectedECRRepository       string
	allECRImages                []model.ECRImage
	filteredECRImages           []model.ECRImage
	selectedECRImage            string
	allECRFindings              []model.ECRFinding
	filteredECRFindings         []model.ECRFinding
	selectedECRFinding          string
	ecrActionPending            bool
	ecrScanCache                map[string]model.ECRScan
	allRDSInstances             []model.RDSInstance
	filteredRDSInstances        []model.RDSInstance
	selectedRDSInstance         string
	rdsDetail                   *model.RDSInstanceDetail
	allRDSClusters              []model.RDSCluster
	filteredRDSClusters         []model.RDSCluster
	selectedRDSCluster          string
	rdsClusterDetail            *model.RDSCluster
	rdsClusterContext           string
	allS3Buckets                []model.S3Bucket
	filteredS3Buckets           []model.S3Bucket
	selectedS3Bucket            string
	s3BucketFilter              string
	activeSavedS3Search         string
	allS3Objects                []model.S3Object
	filteredS3Objects           []model.S3Object
	selectedS3Object            string
	s3Prefix                    string
	s3ObjectSearch              string
	s3ObjectSearchActive        bool
	s3ObjectDetail              *model.S3ObjectDetail
	s3DownloadPending           bool
	allDynamoTables             []string
	filteredDynamoTables        []string
	selectedDynamoTable         string
	dynamoTableDetail           *model.DynamoTable
	allDynamoItems              []model.DynamoItem
	filteredDynamoItems         []model.DynamoItem
	selectedDynamoItem          int
	dynamoKeyNames              []string
	dynamoItemColumns           []string
	dynamoNextToken             string
	dynamoScannedCount          int
	dynamoFilter                *model.DynamoFilter
	dynamoPartiQL               string
	activeSavedDynamoTable      string
	activeSavedDynamoQuery      string
	dynamoActionPending         bool
	allSQSQueues                []model.SQSQueue
	filteredSQSQueues           []model.SQSQueue
	selectedSQSQueue            string
	sqsQueueStats               *model.SQSQueueStats
	activeSavedSQSQueue         string
	allSQSMessages              []model.SQSMessage
	filteredSQSMessages         []model.SQSMessage
	selectedSQSMessage          string
	sqsMessageQueue             model.SQSQueue
	sqsMessageQueueStats        *model.SQSQueueStats
	sqsMessagesParentURL        string
	sqsMessagesParentSavedName  string
	sqsActionPending            bool
	allRoute53Zones             []model.Route53Zone
	filteredRoute53Zones        []model.Route53Zone
	selectedRoute53Zone         string
	allRoute53Records           []model.Route53Record
	filteredRoute53Records      []model.Route53Record
	selectedRoute53Record       string
	route53ZoneContext          *model.Route53Zone
	route53DNSAnswer            *model.Route53DNSAnswer
	route53ActionPending        bool
	selectedTaskDefinition      *model.TaskDefSummary
	standaloneReturnPage        string
	standaloneReturnService     string
	standaloneReturnStopped     bool
	showingStoppedTasks         bool
	taskNextToken               string
	clusterTable                *stringTable
	serviceTable                *stringTable
	taskTable                   *stringTable
	stoppedTaskTable            *stringTable
	taskDefinitionTable         *stringTable
	logGroupTable               *stringTable
	logStreamTable              *stringTable
	alarmTable                  *stringTable
	ssmTable                    *stringTable
	secretTable                 *stringTable
	lambdaTable                 *stringTable
	codeBuildProjectTable       *stringTable
	codeBuildBuildTable         *stringTable
	ec2Table                    *stringTable
	ec2SecurityGroupTable       *stringTable
	ec2VPCTable                 *stringTable
	ec2SubnetTable              *stringTable
	ec2VolumeTable              *stringTable
	ec2LoadBalancerTable        *stringTable
	ec2TargetGroupTable         *stringTable
	ecrRepositoryTable          *stringTable
	ecrImageTable               *stringTable
	ecrFindingTable             *stringTable
	rdsTable                    *stringTable
	rdsClusterTable             *stringTable
	s3BucketTable               *stringTable
	s3ObjectTable               *stringTable
	dynamoTable                 *stringTable
	dynamoItemTable             *stringTable
	sqsQueueTable               *stringTable
	sqsMessageTable             *stringTable
	route53ZoneTable            *stringTable
	route53RecordTable          *stringTable
	resourceStack               *gtk.Stack
	search                      *gtk.SearchEntry
	backButton                  *gtk.Button
	headerBar                   *gtk.Box
	clustersNavButton           *gtk.ToggleButton
	taskDefinitionsNavButton    *gtk.ToggleButton
	logGroupsNavButton          *gtk.ToggleButton
	cloudWatchModuleItems       *gtk.Box
	ssmModuleItems              *gtk.Box
	secretsModuleItems          *gtk.Box
	lambdaModuleItems           *gtk.Box
	codeBuildModuleItems        *gtk.Box
	ec2ModuleItems              *gtk.Box
	ecrModuleItems              *gtk.Box
	rdsModuleItems              *gtk.Box
	s3ModuleItems               *gtk.Box
	dynamoModuleItems           *gtk.Box
	sqsModuleItems              *gtk.Box
	route53ModuleItems          *gtk.Box
	moduleErrorGlyphs           map[string]*gtk.Image
	moduleSections              []moduleRailSection
	modulePickerOpen            bool
	alarmNavButtons             map[string]*gtk.ToggleButton
	savedLogsLabel              *gtk.Label
	savedLogNavButtons          []*gtk.ToggleButton
	ssmParametersNavButton      *gtk.ToggleButton
	savedSSMPrefixLabel         *gtk.Label
	savedSSMPrefixButtons       []*gtk.ToggleButton
	secretsNavButton            *gtk.ToggleButton
	savedSecretFiltersLabel     *gtk.Label
	savedSecretFilterButtons    []*gtk.ToggleButton
	lambdaFunctionsNavButton    *gtk.ToggleButton
	codeBuildProjectsNavButton  *gtk.ToggleButton
	ec2InstancesNavButton       *gtk.ToggleButton
	ec2LoadBalancersNavButton   *gtk.ToggleButton
	ec2TargetGroupsNavButton    *gtk.ToggleButton
	ec2SecurityGroupsNavButton  *gtk.ToggleButton
	ec2VPCsNavButton            *gtk.ToggleButton
	ec2SubnetsNavButton         *gtk.ToggleButton
	ec2VolumesNavButton         *gtk.ToggleButton
	ecrRepositoriesNavButton    *gtk.ToggleButton
	rdsInstancesNavButton       *gtk.ToggleButton
	rdsClustersNavButton        *gtk.ToggleButton
	s3BucketsNavButton          *gtk.ToggleButton
	savedS3SearchesLabel        *gtk.Label
	savedS3SearchButtons        []*gtk.ToggleButton
	dynamoTablesNavButton       *gtk.ToggleButton
	savedDynamoTablesLabel      *gtk.Label
	savedDynamoTableButtons     []*gtk.ToggleButton
	savedDynamoQueriesLabel     *gtk.Label
	savedDynamoQueryButtons     []*gtk.ToggleButton
	sqsQueuesNavButton          *gtk.ToggleButton
	savedSQSQueuesLabel         *gtk.Label
	savedSQSQueueButtons        []*gtk.ToggleButton
	route53ZonesNavButton       *gtk.ToggleButton
	savedLambdaSearchesLabel    *gtk.Label
	savedLambdaSearchButtons    []*gtk.ToggleButton
	activeSavedLog              string
	peekLogStreamButton         *gtk.Button
	followLogStreamButton       *gtk.Button
	followLogGroupButton        *gtk.Button
	searchLogsButton            *gtk.Button
	saveLogDestinationButton    *gtk.Button
	saveLogSearchButton         *gtk.Button
	updateSavedLogButton        *gtk.Button
	savedLogModifiedLabel       *gtk.Label
	manageSavedLogButton        *gtk.Button
	alarmActionsButton          *gtk.Button
	alarmSetStateButton         *gtk.Button
	alarmTimestampButton        *gtk.Button
	ssmBrowsePathButton         *gtk.Button
	ssmSavePrefixButton         *gtk.Button
	ssmManagePrefixesButton     *gtk.Button
	ssmViewValueButton          *gtk.Button
	ssmEditButton               *gtk.Button
	secretFilterButton          *gtk.Button
	secretSaveFilterButton      *gtk.Button
	secretManageFiltersButton   *gtk.Button
	secretRevealButton          *gtk.Button
	secretEditButton            *gtk.Button
	secretCloneButton           *gtk.Button
	secretCopyARNButton         *gtk.Button
	lambdaSearchButton          *gtk.Button
	lambdaSaveSearchButton      *gtk.Button
	lambdaManageSearchesButton  *gtk.Button
	lambdaDetailsButton         *gtk.Button
	lambdaEnvironmentButton     *gtk.Button
	lambdaRevealSecretsButton   *gtk.Button
	lambdaFollowLogsButton      *gtk.Button
	lambdaBrowseLogsButton      *gtk.Button
	lambdaSearchLogsButton      *gtk.Button
	lambdaEditCodeButton        *gtk.Button
	codeBuildStartButton        *gtk.Button
	codeBuildLogsButton         *gtk.Button
	codeBuildSearchLogsButton   *gtk.Button
	codeBuildStopButton         *gtk.Button
	ec2DetailsButton            *gtk.Button
	ec2ConsoleButton            *gtk.Button
	ec2SessionButton            *gtk.Button
	ec2StartButton              *gtk.Button
	ec2StopButton               *gtk.Button
	ec2RebootButton             *gtk.Button
	ec2TerminateButton          *gtk.Button
	ecrCopyURIButton            *gtk.Button
	ecrStartScanButton          *gtk.Button
	ecrDeleteImageButton        *gtk.Button
	s3SaveSearchButton          *gtk.Button
	s3ManageSearchesButton      *gtk.Button
	s3KeySearchButton           *gtk.Button
	s3DownloadButton            *gtk.Button
	dynamoSaveTableButton       *gtk.Button
	dynamoManageSavedButton     *gtk.Button
	dynamoFilterButton          *gtk.Button
	dynamoPartiQLButton         *gtk.Button
	dynamoSaveQueryButton       *gtk.Button
	dynamoLoadMoreButton        *gtk.Button
	dynamoEditButton            *gtk.Button
	dynamoCloneButton           *gtk.Button
	sqsSaveQueueButton          *gtk.Button
	sqsManageSavedButton        *gtk.Button
	sqsPollButton               *gtk.Button
	sqsClearButton              *gtk.Button
	sqsDeadLetterButton         *gtk.Button
	sqsSendButton               *gtk.Button
	sqsCloneButton              *gtk.Button
	sqsDeleteButton             *gtk.Button
	logsButton                  *gtk.Button
	taskLogsButton              *gtk.Button
	standaloneButton            *gtk.Button
	runTaskButton               *gtk.Button
	taskScopeBar                *gtk.Box
	activeTasksButton           *gtk.ToggleButton
	stoppedTasksButton          *gtk.ToggleButton
	loadMoreTasksButton         *gtk.Button
	metricsButton               *gtk.Button
	execButton                  *gtk.Button
	scaleButton                 *gtk.Button
	stopTaskButton              *gtk.Button
	deployButton                *gtk.Button
	breadcrumb                  *gtk.DrawingArea
	breadcrumbText              string
	detailToolbar               *gtk.Box
	detailLinks                 *gtk.MenuButton
	detailView                  *gtk.TextView
	detailResourceTags          []detailResourceTag
	detailHeadingTag            *gtk.TextTag
	detailErrorTag              *gtk.TextTag
	detailParentButton          *gtk.Button
	detailBuffer                *gtk.TextBuffer
	detailText                  string
	detailStack                 *gtk.Stack
	workspaceBusyBar            *gtk.Box
	workspaceCancelButton       *gtk.Button
	workspaceCancel             func()
	workspaceBusySpinner        *gtk.Spinner
	workspaceBusyLabel          *gtk.Label
	workspaceBusy               bool
	metricsChartsBox            *gtk.Box
	metricsCharts               []*metricChart
	metricsRange                *gtk.DropDown
	metricsTimeButton           *gtk.Button
	metricsUTCTime              bool
	metricsRestoreButton        *gtk.Button
	metricsAlarmTable           *stringTable
	metricsScaleButton          *gtk.Button
	metricsScaleLabel           *gtk.Label
	metricsTitle                *gtk.Label
	metricsScope                *gtk.Label
	metricsNotice               *gtk.Label
	metricsTimestamp            *gtk.Label
	metricsAlarmSection         *gtk.Box
	metricsSnapshot             *model.ServiceMetrics
	metricsGenericSnapshot      *model.MetricSnapshot
	metricsKind                 string
	metricsFocusedTitle         string
	metricsRenderedSnapshot     *model.MetricSnapshot
	metricsChartSpecs           []metricChartSpec
	metricsAlarms               []model.AlarmState
	metricsTaskID               string
	scaleInSuspended            bool
	scaleInKnown                bool
	showingMetrics              bool
	taskDefinitionBuffer        *gtk.TextBuffer
	taskDefinitionSummaryButton *gtk.Button
	taskDefinitionEnvButton     *gtk.Button
	taskDefinitionRevealButton  *gtk.Button
	taskDefinitionDiffButton    *gtk.Button
	taskDefinitionEditButton    *gtk.Button
	taskDefinitionEnvContainer  string
	taskDefinitionViewMode      string
	editorBuffer                *gtk.TextBuffer
	taskDefinitionSourceEditor  *sourceEditor
	showingEditor               bool
	editorDirty                 bool
	editorLoading               bool
	editorKind                  string
	lambdaEditorBuffer          *gtk.TextBuffer
	lambdaSourceEditor          *sourceEditor
	lambdaEditorFileSelector    *gtk.DropDown
	lambdaEditorTitle           *gtk.Label
	lambdaEditorUploadButton    *gtk.Button
	lambdaEditorFiles           []lambdaEditableFile
	lambdaEditorFileIndex       int
	lambdaEditorDirectory       string
	lambdaEditorFunction        string
	lambdaEditorLoading         bool
	lambdaEditorDirty           bool
	terminal                    *vteTerminal
	terminalTitle               *gtk.Label
	terminalTask                model.Task
	terminalContainer           string
	terminalCommand             string
	terminalDescription         string
	showingTerminal             bool
	resourceHistory             []resourceNavigationState
	logView                     *gtk.TextView
	logTextBuffer               *gtk.TextBuffer
	logSearch                   *gtk.SearchEntry
	logPauseButton              *gtk.Button
	logTimestampButton          *gtk.Button
	logStreamsButton            *gtk.Button
	logOlderButton              *gtk.Button
	logNewerButton              *gtk.Button
	logNewerKnown               int
	logNewestKnownTS            int64
	logCorrelateButton          *gtk.Button
	logHighlightsButton         *gtk.Button
	logStore                    *boundedLogs
	logIndentTags               map[int]*gtk.TextTag
	logHighlightTags            map[model.LogHighlightStyle]*gtk.TextTag
	logHighlightRules           []model.LogHighlightRule
	logHiddenStreams            map[string]struct{}
	logSource                   model.LogSource
	logTitle                    string
	logLastTS                   int64
	logFollowing                bool
	logTimestampMode            logTimestampMode
	logSearchSpec               *cloudWatchSearch
	showingLogs                 bool
	status                      *gtk.Label
	statusDetailsButton         *gtk.Button
	statusDismissButton         *gtk.Button
	lastError                   string
	spinner                     *gtk.Spinner
	lastSuccessfulLoad          time.Time
}

type moduleRailSection struct {
	key         string
	name        string
	defaultItem string
	aliases     []string
	expander    *gtk.Expander
	activate    func()
}

func newMainWindow(ctx context.Context, app *gtk.Application, options Options) *mainWindow {
	w := &mainWindow{
		ctx:                ctx,
		options:            options,
		currentPage:        pageModulePicker,
		detailContent:      detailIntro,
		selectedDynamoItem: -1,
	}

	w.clusterTable = newStringTable([]columnSpec{
		{title: "CLUSTER", field: 0, expand: true},
		{title: "STATUS", field: 1},
		{title: "SERVICES", field: 2},
		{title: "RUNNING", field: 3},
		{title: "PENDING", field: 4},
	})
	w.serviceTable = newStringTable([]columnSpec{
		{title: "SERVICE", field: 0, expand: true},
		{title: "HEALTH", field: 1},
		{title: "STATUS", field: 2},
		{title: "RUNNING", field: 3},
		{title: "PENDING", field: 4},
		{title: "TASK DEFINITION", field: 5},
	})
	w.taskTable = newStringTable([]columnSpec{
		{title: "TASK", field: 0, expand: true},
		{title: "HEALTH", field: 1},
		{title: "STATUS", field: 2},
		{title: "GROUP", field: 3},
		{title: "AZ", field: 4},
		{title: "IP", field: 5},
		{title: "TASK DEFINITION", field: 6},
	})
	w.stoppedTaskTable = newStringTable([]columnSpec{
		{title: "TASK", field: 0, expand: true},
		{title: "STOPPED", field: 1},
		{title: "EXIT", field: 2},
		{title: "STOP CODE", field: 3},
		{title: "TASK DEFINITION", field: 4},
		{title: "REASON", field: 5, expand: true},
	})
	w.taskDefinitionTable = newStringTable([]columnSpec{
		{title: "FAMILY", field: 0, expand: true},
		{title: "REVISION", field: 1},
		{title: "TASK DEFINITION ARN", field: 2, expand: true},
	})
	w.logGroupTable = newStringTable([]columnSpec{
		{title: "LOG GROUP", field: 0, expand: true},
		{title: "STORED", field: 1},
	})
	w.logStreamTable = newStringTable([]columnSpec{
		{title: "LOG STREAM", field: 0, expand: true},
		{title: "LAST EVENT", field: 1},
		{title: "FIRST EVENT", field: 2},
	})
	w.alarmTable = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true},
		{title: "STATE", field: 1},
		{title: "METRIC", field: 2},
		{title: "NAMESPACE", field: 3},
		{title: "ACTIONS", field: 4},
		{title: "UPDATED", field: 5},
	})
	w.ssmTable = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true},
		{title: "TYPE", field: 1},
		{title: "VERSION", field: 2},
		{title: "VALUE", field: 3, expand: true},
		{title: "MODIFIED", field: 4},
	})
	w.secretTable = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true},
		{title: "DESCRIPTION", field: 1, expand: true},
		{title: "LAST CHANGED", field: 2},
		{title: "LAST ACCESSED", field: 3},
	})
	w.lambdaTable = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true},
		{title: "RUNTIME", field: 1},
		{title: "STATE", field: 2},
		{title: "MEMORY", field: 3},
		{title: "TIMEOUT", field: 4},
		{title: "MODIFIED", field: 5},
	})
	w.codeBuildProjectTable = newStringTable([]columnSpec{
		{title: "PROJECT", field: 0, expand: true},
		{title: "SOURCE", field: 1},
		{title: "DESCRIPTION", field: 2, expand: true},
		{title: "MODIFIED", field: 3},
	})
	w.codeBuildBuildTable = newStringTable([]columnSpec{
		{title: "#", field: 0},
		{title: "STATUS", field: 1},
		{title: "STARTED", field: 2},
		{title: "DURATION", field: 3},
		{title: "INITIATOR", field: 4, expand: true},
		{title: "SOURCE VERSION", field: 5, expand: true},
	})
	w.ec2Table = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true},
		{title: "INSTANCE ID", field: 1},
		{title: "STATE", field: 2},
		{title: "TYPE", field: 3},
		{title: "AZ", field: 4},
		{title: "PRIVATE IP", field: 5},
		{title: "PUBLIC IP", field: 6},
		{title: "AGE", field: 7},
	})
	w.ec2SecurityGroupTable = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true},
		{title: "GROUP ID", field: 1},
		{title: "VPC", field: 2},
		{title: "INBOUND", field: 3},
		{title: "OUTBOUND", field: 4},
		{title: "DESCRIPTION", field: 5, expand: true},
	})
	w.ec2VPCTable = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true}, {title: "VPC ID", field: 1},
		{title: "STATE", field: 2}, {title: "CIDRS", field: 3, expand: true},
		{title: "DEFAULT", field: 4}, {title: "TENANCY", field: 5},
	})
	w.ec2SubnetTable = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true}, {title: "SUBNET ID", field: 1},
		{title: "VPC", field: 2}, {title: "AZ", field: 3}, {title: "CIDR", field: 4},
		{title: "AVAILABLE IPS", field: 5}, {title: "PUBLIC IP", field: 6},
	})
	w.ec2VolumeTable = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true}, {title: "VOLUME ID", field: 1}, {title: "STATE", field: 2},
		{title: "TYPE", field: 3}, {title: "SIZE", field: 4}, {title: "AZ", field: 5}, {title: "ATTACHED TO", field: 6, expand: true},
	})
	w.ec2LoadBalancerTable = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true}, {title: "TYPE", field: 1}, {title: "STATE", field: 2},
		{title: "SCHEME", field: 3}, {title: "VPC", field: 4}, {title: "DNS NAME", field: 5, expand: true},
	})
	w.ec2TargetGroupTable = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true}, {title: "PROTOCOL", field: 1}, {title: "PORT", field: 2},
		{title: "TARGET TYPE", field: 3}, {title: "VPC", field: 4}, {title: "LOAD BALANCERS", field: 5, expand: true},
	})
	w.ecrRepositoryTable = newStringTable([]columnSpec{
		{title: "REPOSITORY", field: 0, expand: true}, {title: "SCAN ON PUSH", field: 1},
		{title: "TAG MUTABILITY", field: 2}, {title: "ENCRYPTION", field: 3}, {title: "CREATED", field: 4},
	})
	w.ecrImageTable = newStringTable([]columnSpec{
		{title: "TAGS", field: 0, expand: true}, {title: "DIGEST", field: 1, expand: true},
		{title: "PUSHED", field: 2}, {title: "SIZE", field: 3}, {title: "SCAN", field: 4}, {title: "CRITICAL / HIGH", field: 5},
	})
	w.ecrFindingTable = newStringTable([]columnSpec{
		{title: "SEVERITY", field: 0}, {title: "FINDING", field: 1, expand: true},
		{title: "PACKAGE", field: 2, expand: true}, {title: "VERSION", field: 3},
	})
	w.rdsTable = newStringTable([]columnSpec{
		{title: "IDENTIFIER", field: 0, expand: true}, {title: "ENGINE", field: 1},
		{title: "CLASS", field: 2}, {title: "STATUS", field: 3}, {title: "ROLE", field: 4},
		{title: "AZ", field: 5}, {title: "ENDPOINT", field: 6, expand: true},
	})
	w.rdsClusterTable = newStringTable([]columnSpec{
		{title: "IDENTIFIER", field: 0, expand: true}, {title: "ENGINE", field: 1},
		{title: "STATUS", field: 2}, {title: "MEMBERS", field: 3}, {title: "WRITER", field: 4},
		{title: "ENDPOINT", field: 5, expand: true},
	})
	w.s3BucketTable = newStringTable([]columnSpec{
		{title: "BUCKET", field: 0, expand: true}, {title: "CREATED", field: 1},
	})
	w.s3ObjectTable = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true}, {title: "TYPE", field: 1},
		{title: "SIZE", field: 2}, {title: "MODIFIED", field: 3},
	})
	w.dynamoTable = newStringTable([]columnSpec{
		{title: "TABLE", field: 0, expand: true},
	})
	w.dynamoItemTable = newStringTable(nil)
	w.sqsQueueTable = newStringTable([]columnSpec{
		{title: "QUEUE", field: 0, expand: true}, {title: "URL", field: 1, expand: true},
	})
	w.sqsMessageTable = newStringTable([]columnSpec{
		{title: "MESSAGE ID", field: 0}, {title: "BODY PREVIEW", field: 1, expand: true},
	})
	w.route53ZoneTable = newStringTable([]columnSpec{
		{title: "ZONE NAME", field: 0, expand: true}, {title: "TYPE", field: 1},
		{title: "RECORDS", field: 2}, {title: "COMMENT", field: 3, expand: true},
	})
	w.route53RecordTable = newStringTable([]columnSpec{
		{title: "NAME", field: 0, expand: true}, {title: "TYPE", field: 1}, {title: "TTL", field: 2},
		{title: "VALUE", field: 3, expand: true}, {title: "ROUTING", field: 4},
	})
	w.clusterTable.view.ConnectActivate(w.openClusterAt)
	w.serviceTable.view.ConnectActivate(w.openServiceAt)
	w.taskTable.view.ConnectActivate(w.openTaskAt)
	w.stoppedTaskTable.view.ConnectActivate(w.openStoppedTaskAt)
	w.taskDefinitionTable.view.ConnectActivate(w.openTaskDefinitionAt)
	w.logGroupTable.view.ConnectActivate(w.openLogGroupAt)
	w.logStreamTable.view.ConnectActivate(w.peekLogStreamAt)
	w.alarmTable.view.ConnectActivate(w.openAlarmAt)
	w.ssmTable.view.ConnectActivate(w.openSSMParameterAt)
	w.secretTable.view.ConnectActivate(w.openSecretAt)
	w.lambdaTable.view.ConnectActivate(w.openLambdaFunctionAt)
	w.codeBuildProjectTable.view.ConnectActivate(w.openCodeBuildProjectAt)
	w.codeBuildBuildTable.view.ConnectActivate(w.openCodeBuildBuildAt)
	w.ec2Table.view.ConnectActivate(w.openEC2InstanceAt)
	w.ec2SecurityGroupTable.view.ConnectActivate(w.openEC2SecurityGroupAt)
	w.ec2VPCTable.view.ConnectActivate(w.openEC2VPCAt)
	w.ec2SubnetTable.view.ConnectActivate(w.openEC2SubnetAt)
	w.ec2VolumeTable.view.ConnectActivate(w.openEC2VolumeAt)
	w.ec2LoadBalancerTable.view.ConnectActivate(w.openEC2LoadBalancerAt)
	w.ec2TargetGroupTable.view.ConnectActivate(w.openEC2TargetGroupAt)
	w.ecrRepositoryTable.view.ConnectActivate(w.openECRRepositoryAt)
	w.ecrImageTable.view.ConnectActivate(w.openECRImageAt)
	w.ecrFindingTable.view.ConnectActivate(w.openECRFindingAt)
	w.rdsTable.view.ConnectActivate(w.openRDSInstanceAt)
	w.rdsClusterTable.view.ConnectActivate(w.openRDSClusterAt)
	w.s3BucketTable.view.ConnectActivate(w.openS3BucketAt)
	w.s3ObjectTable.view.ConnectActivate(w.openS3ObjectAt)
	w.dynamoTable.view.ConnectActivate(w.openDynamoTableAt)
	w.dynamoItemTable.view.ConnectActivate(w.openDynamoItemAt)
	w.sqsQueueTable.view.ConnectActivate(w.openSQSQueueAt)
	w.sqsMessageTable.view.ConnectActivate(w.openSQSMessageAt)
	w.route53ZoneTable.view.ConnectActivate(w.openRoute53ZoneAt)
	w.clusterTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectClusterRow() })
	w.logGroupTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectLogGroupRow() })
	w.serviceTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectServiceRow() })
	w.taskTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectTaskRow(false) })
	w.stoppedTaskTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectTaskRow(true) })
	w.logStreamTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectLogStreamRow() })
	w.alarmTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectAlarmRow() })
	w.ssmTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectSSMParameterRow() })
	w.secretTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectSecretRow() })
	w.lambdaTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectLambdaFunctionRow() })
	w.codeBuildProjectTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectCodeBuildProjectRow() })
	w.codeBuildBuildTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectCodeBuildBuildRow() })
	w.ec2Table.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectEC2InstanceRow() })
	w.ec2SecurityGroupTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectEC2SecurityGroupRow() })
	w.ec2VPCTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectEC2VPCRow() })
	w.ec2SubnetTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectEC2SubnetRow() })
	w.ec2VolumeTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectEC2VolumeRow() })
	w.ec2LoadBalancerTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectEC2LoadBalancerRow() })
	w.ec2TargetGroupTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectEC2TargetGroupRow() })
	w.ecrRepositoryTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectECRRepositoryRow() })
	w.ecrImageTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectECRImageRow() })
	w.ecrFindingTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectECRFindingRow() })
	w.rdsTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectRDSInstanceRow() })
	w.rdsClusterTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectRDSClusterRow() })
	w.s3BucketTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectS3BucketRow() })
	w.s3ObjectTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectS3ObjectRow() })
	w.dynamoTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectDynamoTableRow() })
	w.dynamoItemTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectDynamoItemRow() })
	w.sqsQueueTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectSQSQueueRow() })
	w.sqsMessageTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectSQSMessageRow() })
	w.route53ZoneTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectRoute53ZoneRow() })

	w.window = gtk.NewApplicationWindow(app)
	w.window.SetTitle("e9s")
	w.window.SetDefaultSize(1380, 820)
	w.window.SetChild(w.buildLayout())
	w.window.ConnectDestroy(func() {
		if w.terminal != nil {
			w.terminal.Stop()
		}
		w.discardLambdaEditor()
	})
	w.installActions(app)
	w.installPrintableShortcuts(app)

	go w.autoRefresh()
	return w
}

func (w *mainWindow) buildLayout() gtk.Widgetter {
	w.backButton = gtk.NewButtonWithLabel("Back")
	w.backButton.SetSensitive(false)
	w.backButton.ConnectClicked(w.navigateBrowserBack)

	title := gtk.NewLabel("e9s")
	title.AddCSSClass("app-title")
	w.breadcrumbText = "Modules"
	w.breadcrumb = w.newBreadcrumbArea()

	refresh := gtk.NewButtonWithLabel("Refresh")
	refresh.ConnectClicked(w.refresh)
	w.logsButton = gtk.NewButtonWithLabel("Service logs")
	w.logsButton.SetSensitive(false)
	w.logsButton.ConnectClicked(w.openServiceLogs)
	w.taskLogsButton = gtk.NewButtonWithLabel("Task logs")
	w.taskLogsButton.SetSensitive(false)
	w.taskLogsButton.ConnectClicked(w.openTaskLogs)
	w.peekLogStreamButton = gtk.NewButtonWithLabel("Peek stream")
	w.peekLogStreamButton.ConnectClicked(w.peekSelectedLogStream)
	w.followLogStreamButton = gtk.NewButtonWithLabel("Follow stream")
	w.followLogStreamButton.ConnectClicked(w.followSelectedLogStream)
	w.followLogGroupButton = gtk.NewButtonWithLabel("Follow group")
	w.followLogGroupButton.ConnectClicked(w.followSelectedLogGroup)
	w.searchLogsButton = gtk.NewButtonWithLabel("Search logs")
	w.searchLogsButton.ConnectClicked(w.promptCloudWatchSearch)
	w.saveLogDestinationButton = gtk.NewButtonWithLabel("Save destination")
	w.saveLogDestinationButton.ConnectClicked(w.promptSaveLogDestination)
	w.savedLogModifiedLabel = gtk.NewLabel("Modified")
	w.savedLogModifiedLabel.AddCSSClass("saved-log-modified")
	w.saveLogSearchButton = gtk.NewButtonWithLabel("Save as…")
	w.saveLogSearchButton.ConnectClicked(w.promptSaveLogSearch)
	w.updateSavedLogButton = gtk.NewButtonWithLabel("Update saved")
	w.updateSavedLogButton.ConnectClicked(w.updateActiveSavedLogFromWorkspace)
	w.manageSavedLogButton = gtk.NewButtonWithLabel("Saved searches…")
	w.manageSavedLogButton.ConnectClicked(w.promptManageSavedLog)
	w.alarmActionsButton = gtk.NewButtonWithLabel("Alarm actions")
	w.alarmActionsButton.ConnectClicked(w.confirmToggleAlarmActions)
	w.alarmSetStateButton = gtk.NewButtonWithLabel("Set state…")
	w.alarmSetStateButton.AddCSSClass("destructive-action")
	w.alarmSetStateButton.ConnectClicked(w.promptSetAlarmState)
	w.alarmTimestampButton = gtk.NewButtonWithLabel("Time: Local")
	w.alarmTimestampButton.ConnectClicked(w.toggleAlarmTimestamps)
	w.ssmBrowsePathButton = gtk.NewButtonWithLabel("Browse path…")
	w.ssmBrowsePathButton.ConnectClicked(w.promptSSMPath)
	w.ssmSavePrefixButton = gtk.NewButtonWithLabel("Save prefix…")
	w.ssmSavePrefixButton.ConnectClicked(w.promptSaveSSMPrefix)
	w.ssmManagePrefixesButton = gtk.NewButtonWithLabel("Saved prefixes…")
	w.ssmManagePrefixesButton.ConnectClicked(w.promptManageSSMPrefixes)
	w.ssmViewValueButton = gtk.NewButtonWithLabel("View value")
	w.ssmViewValueButton.ConnectClicked(w.promptViewSSMParameter)
	w.ssmEditButton = gtk.NewButtonWithLabel("Edit…")
	w.ssmEditButton.ConnectClicked(w.editSelectedSSMParameter)
	w.secretFilterButton = gtk.NewButtonWithLabel("Name filter…")
	w.secretFilterButton.ConnectClicked(w.promptSecretFilter)
	w.secretSaveFilterButton = gtk.NewButtonWithLabel("Save filter…")
	w.secretSaveFilterButton.ConnectClicked(w.promptSaveSecretFilter)
	w.secretManageFiltersButton = gtk.NewButtonWithLabel("Saved filters…")
	w.secretManageFiltersButton.ConnectClicked(w.promptManageSecretFilters)
	w.secretRevealButton = gtk.NewButtonWithLabel("Reveal value…")
	w.secretRevealButton.ConnectClicked(w.promptRevealSecret)
	w.secretEditButton = gtk.NewButtonWithLabel("Edit…")
	w.secretEditButton.ConnectClicked(w.editSelectedSecret)
	w.secretCloneButton = gtk.NewButtonWithLabel("Clone…")
	w.secretCloneButton.ConnectClicked(w.cloneSelectedSecret)
	w.secretCopyARNButton = gtk.NewButtonWithLabel("Copy ARN")
	w.secretCopyARNButton.ConnectClicked(w.copySelectedSecretARN)
	w.lambdaSearchButton = gtk.NewButtonWithLabel("Function search…")
	w.lambdaSearchButton.ConnectClicked(w.promptLambdaSearch)
	w.lambdaSaveSearchButton = gtk.NewButtonWithLabel("Save search…")
	w.lambdaSaveSearchButton.ConnectClicked(w.promptSaveLambdaSearch)
	w.lambdaManageSearchesButton = gtk.NewButtonWithLabel("Saved searches…")
	w.lambdaManageSearchesButton.ConnectClicked(w.promptManageLambdaSearches)
	w.lambdaDetailsButton = gtk.NewButtonWithLabel("Details")
	w.lambdaDetailsButton.ConnectClicked(w.showLambdaDetails)
	w.lambdaEnvironmentButton = gtk.NewButtonWithLabel("Environment")
	w.lambdaEnvironmentButton.ConnectClicked(w.openLambdaEnvironment)
	w.lambdaRevealSecretsButton = gtk.NewButtonWithLabel("Reveal secret values…")
	w.lambdaRevealSecretsButton.ConnectClicked(w.confirmRevealLambdaSecrets)
	w.lambdaFollowLogsButton = gtk.NewButtonWithLabel("Follow logs")
	w.lambdaFollowLogsButton.ConnectClicked(w.followLambdaLogs)
	w.lambdaBrowseLogsButton = gtk.NewButtonWithLabel("Browse logs")
	w.lambdaBrowseLogsButton.ConnectClicked(w.browseLambdaLogs)
	w.lambdaSearchLogsButton = gtk.NewButtonWithLabel("Search logs")
	w.lambdaSearchLogsButton.ConnectClicked(w.searchLambdaLogs)
	w.lambdaEditCodeButton = gtk.NewButtonWithLabel("Edit code…")
	w.lambdaEditCodeButton.ConnectClicked(w.openLambdaCodeEditor)
	w.codeBuildStartButton = gtk.NewButtonWithLabel("Start build…")
	w.codeBuildStartButton.ConnectClicked(w.confirmStartCodeBuild)
	w.codeBuildLogsButton = gtk.NewButtonWithLabel("View logs")
	w.codeBuildLogsButton.ConnectClicked(w.viewCodeBuildLogs)
	w.codeBuildSearchLogsButton = gtk.NewButtonWithLabel("Search logs")
	w.codeBuildSearchLogsButton.ConnectClicked(w.searchCodeBuildLogs)
	w.codeBuildStopButton = gtk.NewButtonWithLabel("Stop build…")
	w.codeBuildStopButton.AddCSSClass("destructive-action")
	w.codeBuildStopButton.ConnectClicked(w.confirmStopCodeBuild)
	w.ec2DetailsButton = gtk.NewButtonWithLabel("Details")
	w.ec2DetailsButton.ConnectClicked(w.showEC2Details)
	w.ec2ConsoleButton = gtk.NewButtonWithLabel("Console output")
	w.ec2ConsoleButton.ConnectClicked(w.loadEC2ConsoleOutput)
	w.ec2SessionButton = gtk.NewButtonWithLabel("Session Manager")
	w.ec2SessionButton.ConnectClicked(w.openEC2Session)
	w.ec2StartButton = gtk.NewButtonWithLabel("Start…")
	w.ec2StartButton.ConnectClicked(func() { w.confirmEC2Mutation("start") })
	w.ec2StopButton = gtk.NewButtonWithLabel("Stop…")
	w.ec2StopButton.ConnectClicked(func() { w.confirmEC2Mutation("stop") })
	w.ec2RebootButton = gtk.NewButtonWithLabel("Reboot…")
	w.ec2RebootButton.ConnectClicked(func() { w.confirmEC2Mutation("reboot") })
	w.ec2TerminateButton = gtk.NewButtonWithLabel("Terminate…")
	w.ec2TerminateButton.AddCSSClass("destructive-action")
	w.ec2TerminateButton.ConnectClicked(func() { w.confirmEC2Mutation("terminate") })
	w.ecrCopyURIButton = gtk.NewButtonWithLabel("Copy image URI")
	w.ecrCopyURIButton.ConnectClicked(w.copySelectedECRImageURI)
	w.ecrStartScanButton = gtk.NewButtonWithLabel("Start scan")
	w.ecrStartScanButton.ConnectClicked(w.startSelectedECRScan)
	w.ecrDeleteImageButton = gtk.NewButtonWithLabel("Delete image…")
	w.ecrDeleteImageButton.AddCSSClass("destructive-action")
	w.ecrDeleteImageButton.ConnectClicked(w.confirmDeleteECRImage)
	w.s3SaveSearchButton = gtk.NewButtonWithLabel("Save search…")
	w.s3SaveSearchButton.ConnectClicked(w.promptSaveS3Search)
	w.s3ManageSearchesButton = gtk.NewButtonWithLabel("Saved searches…")
	w.s3ManageSearchesButton.ConnectClicked(w.promptManageS3Searches)
	w.s3KeySearchButton = gtk.NewButtonWithLabel("Key prefix…")
	w.s3KeySearchButton.ConnectClicked(w.promptS3KeySearch)
	w.s3DownloadButton = gtk.NewButtonWithLabel("Download…")
	w.s3DownloadButton.ConnectClicked(w.promptS3Download)
	w.dynamoSaveTableButton = gtk.NewButtonWithLabel("Save table…")
	w.dynamoSaveTableButton.ConnectClicked(w.promptSaveDynamoTable)
	w.dynamoManageSavedButton = gtk.NewButtonWithLabel("Manage saved…")
	w.dynamoManageSavedButton.ConnectClicked(w.promptManageDynamoSaved)
	w.dynamoFilterButton = gtk.NewButtonWithLabel("Filter…")
	w.dynamoFilterButton.ConnectClicked(w.promptDynamoFilter)
	w.dynamoPartiQLButton = gtk.NewButtonWithLabel("PartiQL…")
	w.dynamoPartiQLButton.ConnectClicked(w.promptDynamoPartiQL)
	w.dynamoSaveQueryButton = gtk.NewButtonWithLabel("Save query…")
	w.dynamoSaveQueryButton.ConnectClicked(w.promptSaveDynamoQuery)
	w.dynamoLoadMoreButton = gtk.NewButtonWithLabel("Load more")
	w.dynamoLoadMoreButton.ConnectClicked(w.loadMoreDynamoItems)
	w.dynamoEditButton = gtk.NewButtonWithLabel("Edit field…")
	w.dynamoEditButton.ConnectClicked(w.promptDynamoFieldEdit)
	w.dynamoCloneButton = gtk.NewButtonWithLabel("Clone item…")
	w.dynamoCloneButton.ConnectClicked(w.promptDynamoClone)
	w.sqsSaveQueueButton = gtk.NewButtonWithLabel("Save queue…")
	w.sqsSaveQueueButton.ConnectClicked(w.promptSaveSQSQueue)
	w.sqsManageSavedButton = gtk.NewButtonWithLabel("Manage saved…")
	w.sqsManageSavedButton.ConnectClicked(w.promptManageSQSQueues)
	w.sqsPollButton = gtk.NewButtonWithLabel("Poll messages")
	w.sqsPollButton.ConnectClicked(w.pollSQSMessages)
	w.sqsClearButton = gtk.NewButtonWithLabel("Clear buffer")
	w.sqsClearButton.ConnectClicked(w.clearSQSMessageBuffer)
	w.sqsDeadLetterButton = gtk.NewButtonWithLabel("Open DLQ")
	w.sqsDeadLetterButton.ConnectClicked(w.openSQSDeadLetterQueue)
	w.sqsSendButton = gtk.NewButtonWithLabel("Send message…")
	w.sqsSendButton.ConnectClicked(w.promptSQSSendMessage)
	w.sqsCloneButton = gtk.NewButtonWithLabel("Clone & send…")
	w.sqsCloneButton.ConnectClicked(w.promptSQSCloneMessage)
	w.sqsDeleteButton = gtk.NewButtonWithLabel("Delete message…")
	w.sqsDeleteButton.AddCSSClass("destructive-action")
	w.sqsDeleteButton.ConnectClicked(w.confirmDeleteSQSMessage)
	w.standaloneButton = gtk.NewButtonWithLabel("Standalone")
	w.standaloneButton.SetSensitive(false)
	w.standaloneButton.ConnectClicked(w.toggleStandaloneTasks)
	w.runTaskButton = gtk.NewButtonWithLabel("Run task")
	w.runTaskButton.SetSensitive(false)
	w.runTaskButton.ConnectClicked(w.promptRunTask)
	w.metricsButton = gtk.NewButtonWithLabel("Metrics")
	w.metricsButton.SetSensitive(false)
	w.metricsButton.ConnectClicked(w.openMetrics)
	w.execButton = gtk.NewButtonWithLabel("Exec")
	w.execButton.SetSensitive(false)
	w.execButton.ConnectClicked(w.openExec)
	if !vteAvailable() {
		w.execButton.SetTooltipText("Rebuild with the gui and vte tags to enable the embedded terminal")
	}
	w.scaleButton = gtk.NewButtonWithLabel("Scale")
	w.scaleButton.SetSensitive(false)
	w.scaleButton.ConnectClicked(w.promptScaleService)
	w.stopTaskButton = gtk.NewButtonWithLabel("Stop task")
	w.stopTaskButton.SetSensitive(false)
	w.stopTaskButton.AddCSSClass("destructive-action")
	w.stopTaskButton.ConnectClicked(w.confirmStopTask)
	w.deployButton = gtk.NewButtonWithLabel("Force deploy")
	w.deployButton.SetSensitive(false)
	w.deployButton.AddCSSClass("destructive-action")
	w.deployButton.ConnectClicked(w.confirmForceDeployment)

	header := gtk.NewBox(gtk.OrientationHorizontal, 10)
	w.headerBar = header
	header.AddCSSClass("toolbar")
	header.Append(w.backButton)
	header.Append(title)
	header.Append(w.breadcrumb)
	header.Append(w.standaloneButton)
	header.Append(w.runTaskButton)
	header.Append(w.metricsButton)
	header.Append(w.execButton)
	header.Append(w.logsButton)
	header.Append(w.taskLogsButton)
	header.Append(w.peekLogStreamButton)
	header.Append(w.followLogStreamButton)
	header.Append(w.followLogGroupButton)
	header.Append(w.searchLogsButton)
	header.Append(w.saveLogDestinationButton)
	header.Append(w.savedLogModifiedLabel)
	header.Append(w.updateSavedLogButton)
	header.Append(w.saveLogSearchButton)
	header.Append(w.manageSavedLogButton)
	header.Append(w.alarmActionsButton)
	header.Append(w.alarmSetStateButton)
	header.Append(w.alarmTimestampButton)
	header.Append(w.ssmBrowsePathButton)
	header.Append(w.ssmSavePrefixButton)
	header.Append(w.ssmManagePrefixesButton)
	header.Append(w.ssmViewValueButton)
	header.Append(w.ssmEditButton)
	header.Append(w.secretFilterButton)
	header.Append(w.secretSaveFilterButton)
	header.Append(w.secretManageFiltersButton)
	header.Append(w.secretRevealButton)
	header.Append(w.secretEditButton)
	header.Append(w.secretCloneButton)
	header.Append(w.secretCopyARNButton)
	header.Append(w.lambdaSearchButton)
	header.Append(w.lambdaSaveSearchButton)
	header.Append(w.lambdaManageSearchesButton)
	header.Append(w.lambdaDetailsButton)
	header.Append(w.lambdaEnvironmentButton)
	header.Append(w.lambdaRevealSecretsButton)
	header.Append(w.lambdaFollowLogsButton)
	header.Append(w.lambdaBrowseLogsButton)
	header.Append(w.lambdaSearchLogsButton)
	header.Append(w.lambdaEditCodeButton)
	header.Append(w.codeBuildStartButton)
	header.Append(w.codeBuildLogsButton)
	header.Append(w.codeBuildSearchLogsButton)
	header.Append(w.codeBuildStopButton)
	header.Append(w.ec2DetailsButton)
	header.Append(w.ec2ConsoleButton)
	header.Append(w.ec2SessionButton)
	header.Append(w.ec2StartButton)
	header.Append(w.ec2StopButton)
	header.Append(w.ec2RebootButton)
	header.Append(w.ec2TerminateButton)
	header.Append(w.ecrCopyURIButton)
	header.Append(w.ecrStartScanButton)
	header.Append(w.ecrDeleteImageButton)
	header.Append(w.s3SaveSearchButton)
	header.Append(w.s3ManageSearchesButton)
	header.Append(w.s3KeySearchButton)
	header.Append(w.s3DownloadButton)
	header.Append(w.dynamoSaveTableButton)
	header.Append(w.dynamoManageSavedButton)
	header.Append(w.dynamoFilterButton)
	header.Append(w.dynamoPartiQLButton)
	header.Append(w.dynamoSaveQueryButton)
	header.Append(w.dynamoLoadMoreButton)
	header.Append(w.dynamoEditButton)
	header.Append(w.dynamoCloneButton)
	header.Append(w.sqsSaveQueueButton)
	header.Append(w.sqsManageSavedButton)
	header.Append(w.sqsPollButton)
	header.Append(w.sqsClearButton)
	header.Append(w.sqsDeadLetterButton)
	header.Append(w.sqsSendButton)
	header.Append(w.sqsCloneButton)
	header.Append(w.sqsDeleteButton)
	header.Append(w.scaleButton)
	header.Append(w.stopTaskButton)
	header.Append(w.deployButton)
	header.Append(refresh)

	sidebar := gtk.NewBox(gtk.OrientationVertical, 6)
	sidebar.AddCSSClass("mode-sidebar")
	sidebar.SetSizeRequest(175, -1)
	modules := gtk.NewLabel("MODULES")
	modules.SetXAlign(0)
	modules.AddCSSClass("section-title")
	w.clustersNavButton = newModuleRailButton("Clusters", w.openClustersModule)
	w.taskDefinitionsNavButton = newModuleRailButton("Task Defs", w.openTaskDefinitions)
	w.taskDefinitionsNavButton.SetGroup(w.clustersNavButton)
	moduleItems := gtk.NewBox(gtk.OrientationVertical, 2)
	moduleItems.AddCSSClass("module-subitems")
	moduleItems.Append(w.clustersNavButton)
	moduleItems.Append(w.taskDefinitionsNavButton)
	w.moduleErrorGlyphs = make(map[string]*gtk.Image, 8)
	ecs := w.newModuleExpander("ECS", moduleECS, moduleItems)
	w.codeBuildProjectsNavButton = newModuleRailButton("Projects", w.openCodeBuildModule)
	w.codeBuildProjectsNavButton.SetGroup(w.clustersNavButton)
	w.codeBuildModuleItems = gtk.NewBox(gtk.OrientationVertical, 2)
	w.codeBuildModuleItems.AddCSSClass("module-subitems")
	w.codeBuildModuleItems.Append(w.codeBuildProjectsNavButton)
	codeBuild := w.newModuleExpander("CodeBuild", moduleCodeBuild, w.codeBuildModuleItems)
	w.ec2InstancesNavButton = newModuleRailButton("Instances", w.openEC2Module)
	w.ec2InstancesNavButton.SetGroup(w.clustersNavButton)
	w.ec2LoadBalancersNavButton = newModuleRailButton("Load Balancers", w.openEC2LoadBalancersModule)
	w.ec2LoadBalancersNavButton.SetGroup(w.clustersNavButton)
	w.ec2TargetGroupsNavButton = newModuleRailButton("Target Groups", w.openEC2TargetGroupsModule)
	w.ec2TargetGroupsNavButton.SetGroup(w.clustersNavButton)
	w.ec2SecurityGroupsNavButton = newModuleRailButton("Security Groups", w.openEC2SecurityGroupsModule)
	w.ec2SecurityGroupsNavButton.SetGroup(w.clustersNavButton)
	w.ec2VPCsNavButton = newModuleRailButton("VPCs", w.openEC2VPCsModule)
	w.ec2VPCsNavButton.SetGroup(w.clustersNavButton)
	w.ec2SubnetsNavButton = newModuleRailButton("Subnets", w.openEC2SubnetsModule)
	w.ec2SubnetsNavButton.SetGroup(w.clustersNavButton)
	w.ec2VolumesNavButton = newModuleRailButton("Volumes", w.openEC2VolumesModule)
	w.ec2VolumesNavButton.SetGroup(w.clustersNavButton)
	w.ec2ModuleItems = gtk.NewBox(gtk.OrientationVertical, 2)
	w.ec2ModuleItems.AddCSSClass("module-subitems")
	w.ec2ModuleItems.Append(w.ec2InstancesNavButton)
	w.ec2ModuleItems.Append(w.ec2LoadBalancersNavButton)
	w.ec2ModuleItems.Append(w.ec2TargetGroupsNavButton)
	w.ec2ModuleItems.Append(w.ec2SecurityGroupsNavButton)
	w.ec2ModuleItems.Append(w.ec2VPCsNavButton)
	w.ec2ModuleItems.Append(w.ec2SubnetsNavButton)
	w.ec2ModuleItems.Append(w.ec2VolumesNavButton)
	ec2Instances := w.newModuleExpander("EC2", moduleEC2, w.ec2ModuleItems)
	w.ecrRepositoriesNavButton = newModuleRailButton("Repositories", w.openECRModule)
	w.ecrRepositoriesNavButton.SetGroup(w.clustersNavButton)
	w.ecrModuleItems = gtk.NewBox(gtk.OrientationVertical, 2)
	w.ecrModuleItems.AddCSSClass("module-subitems")
	w.ecrModuleItems.Append(w.ecrRepositoriesNavButton)
	ecrRepositories := w.newModuleExpander("ECR", moduleECR, w.ecrModuleItems)
	w.rdsInstancesNavButton = newModuleRailButton("Instances", w.openRDSModule)
	w.rdsInstancesNavButton.SetGroup(w.clustersNavButton)
	w.rdsClustersNavButton = newModuleRailButton("Clusters", w.openRDSClustersModule)
	w.rdsClustersNavButton.SetGroup(w.clustersNavButton)
	w.rdsModuleItems = gtk.NewBox(gtk.OrientationVertical, 2)
	w.rdsModuleItems.AddCSSClass("module-subitems")
	w.rdsModuleItems.Append(w.rdsClustersNavButton)
	w.rdsModuleItems.Append(w.rdsInstancesNavButton)
	rdsInstances := w.newModuleExpander("RDS", moduleRDS, w.rdsModuleItems)
	w.s3BucketsNavButton = newModuleRailButton("Buckets", w.openS3Module)
	w.s3BucketsNavButton.SetGroup(w.clustersNavButton)
	w.s3ModuleItems = gtk.NewBox(gtk.OrientationVertical, 2)
	w.s3ModuleItems.AddCSSClass("module-subitems")
	w.s3ModuleItems.Append(w.s3BucketsNavButton)
	w.rebuildS3SearchRail()
	s3Buckets := w.newModuleExpander("S3", moduleS3, w.s3ModuleItems)
	w.dynamoTablesNavButton = newModuleRailButton("Tables", w.openDynamoDBModule)
	w.dynamoTablesNavButton.SetGroup(w.clustersNavButton)
	w.dynamoModuleItems = gtk.NewBox(gtk.OrientationVertical, 2)
	w.dynamoModuleItems.AddCSSClass("module-subitems")
	w.dynamoModuleItems.Append(w.dynamoTablesNavButton)
	w.rebuildDynamoRail()
	dynamoDB := w.newModuleExpander("DynamoDB", moduleDynamoDB, w.dynamoModuleItems)
	w.sqsQueuesNavButton = newModuleRailButton("Queues", w.openSQSModule)
	w.sqsQueuesNavButton.SetGroup(w.clustersNavButton)
	w.sqsModuleItems = gtk.NewBox(gtk.OrientationVertical, 2)
	w.sqsModuleItems.AddCSSClass("module-subitems")
	w.sqsModuleItems.Append(w.sqsQueuesNavButton)
	w.rebuildSQSRail()
	sqsQueues := w.newModuleExpander("SQS", moduleSQS, w.sqsModuleItems)
	w.route53ZonesNavButton = newModuleRailButton("Hosted zones", w.openRoute53Module)
	w.route53ZonesNavButton.SetGroup(w.clustersNavButton)
	w.route53ModuleItems = gtk.NewBox(gtk.OrientationVertical, 2)
	w.route53ModuleItems.AddCSSClass("module-subitems")
	w.route53ModuleItems.Append(w.route53ZonesNavButton)
	route53Zones := w.newModuleExpander("Route53", moduleRoute53, w.route53ModuleItems)
	w.logGroupsNavButton = newModuleRailButton("Log groups", w.openLogGroupsModule)
	w.logGroupsNavButton.SetGroup(w.clustersNavButton)
	w.cloudWatchModuleItems = gtk.NewBox(gtk.OrientationVertical, 2)
	w.cloudWatchModuleItems.AddCSSClass("module-subitems")
	w.cloudWatchModuleItems.Append(w.logGroupsNavButton)
	w.rebuildSavedLogRail()
	cloudWatch := w.newModuleExpander("CloudWatch Logs", moduleCloudWatchLogs, w.cloudWatchModuleItems)
	alarmItems := gtk.NewBox(gtk.OrientationVertical, 2)
	alarmItems.AddCSSClass("module-subitems")
	w.alarmNavButtons = make(map[string]*gtk.ToggleButton, 4)
	for _, scope := range []struct {
		label string
		state string
	}{
		{label: "All alarms"},
		{label: "In alarm", state: model.AlarmStateAlarm},
		{label: "OK", state: model.AlarmStateOK},
		{label: "Insufficient data", state: model.AlarmStateInsufficientData},
	} {
		state := scope.state
		button := newModuleRailButton(scope.label, func() { w.openAlarmsModule(state) })
		button.SetGroup(w.clustersNavButton)
		w.alarmNavButtons[state] = button
		alarmItems.Append(button)
	}
	cloudWatchAlarms := w.newModuleExpander("CloudWatch Alarms", moduleCloudWatchAlarms, alarmItems)
	w.ssmParametersNavButton = newModuleRailButton("Parameters", w.openSSMModule)
	w.ssmParametersNavButton.SetGroup(w.clustersNavButton)
	w.ssmModuleItems = gtk.NewBox(gtk.OrientationVertical, 2)
	w.ssmModuleItems.AddCSSClass("module-subitems")
	w.ssmModuleItems.Append(w.ssmParametersNavButton)
	w.rebuildSSMPrefixRail()
	ssmParameters := w.newModuleExpander("SSM Parameter Store", moduleSSM, w.ssmModuleItems)
	w.secretsNavButton = newModuleRailButton("Secrets", w.openSecretsModule)
	w.secretsNavButton.SetGroup(w.clustersNavButton)
	w.secretsModuleItems = gtk.NewBox(gtk.OrientationVertical, 2)
	w.secretsModuleItems.AddCSSClass("module-subitems")
	w.secretsModuleItems.Append(w.secretsNavButton)
	w.rebuildSecretFilterRail()
	secretsManager := w.newModuleExpander("Secrets Manager", moduleSecrets, w.secretsModuleItems)
	w.lambdaFunctionsNavButton = newModuleRailButton("Functions", w.openLambdaModule)
	w.lambdaFunctionsNavButton.SetGroup(w.clustersNavButton)
	w.lambdaModuleItems = gtk.NewBox(gtk.OrientationVertical, 2)
	w.lambdaModuleItems.AddCSSClass("module-subitems")
	w.lambdaModuleItems.Append(w.lambdaFunctionsNavButton)
	w.rebuildLambdaSearchRail()
	lambdaFunctions := w.newModuleExpander("Lambda", moduleLambda, w.lambdaModuleItems)
	comingSoon := gtk.NewLabel("More modules planned")
	comingSoon.SetXAlign(0)
	comingSoon.SetWrap(true)
	comingSoon.AddCSSClass("muted")
	sidebar.Append(modules)
	w.moduleSections = []moduleRailSection{
		{key: moduleCodeBuild, name: "CodeBuild", defaultItem: "Projects", aliases: []string{"cb", "codebuild"}, expander: codeBuild, activate: w.loadCodeBuildProjects},
		{key: moduleEC2, name: "EC2", defaultItem: "Instances", aliases: []string{"ec2", "ec2i"}, expander: ec2Instances, activate: w.loadEC2Instances},
		{key: moduleECR, name: "ECR", defaultItem: "Repositories", aliases: []string{"ecr", "registry", "container registry"}, expander: ecrRepositories, activate: w.loadECRRepositories},
		{key: moduleRDS, name: "RDS", defaultItem: "Clusters", aliases: []string{"rds", "database", "databases"}, expander: rdsInstances, activate: w.openRDSClustersModule},
		{key: moduleS3, name: "S3", defaultItem: "Buckets", aliases: []string{"s3", "object storage", "buckets"}, expander: s3Buckets, activate: w.openS3Module},
		{key: moduleDynamoDB, name: "DynamoDB", defaultItem: "Tables", aliases: []string{"dynamodb", "ddb", "tables"}, expander: dynamoDB, activate: w.openDynamoDBModule},
		{key: moduleSQS, name: "SQS", defaultItem: "Queues", aliases: []string{"sqs", "queue", "queues"}, expander: sqsQueues, activate: w.openSQSModule},
		{key: moduleRoute53, name: "Route53", defaultItem: "Hosted zones", aliases: []string{"route53", "r53", "dns", "hosted zones"}, expander: route53Zones, activate: w.openRoute53Module},
		{key: moduleECS, name: "ECS", defaultItem: "Clusters", aliases: []string{"ecs"}, expander: ecs, activate: w.loadClusters},
		{key: moduleCloudWatchLogs, name: "CloudWatch Logs", defaultItem: "Log groups", aliases: []string{"cwl", "cw", "cloudwatch-logs", "cloudwatch logs", "cloudwatch"}, expander: cloudWatch, activate: w.loadLogGroups},
		{key: moduleCloudWatchAlarms, name: "CloudWatch Alarms", defaultItem: "All alarms", aliases: []string{"cwa", "cloudwatch-alarms", "cloudwatch alarms"}, expander: cloudWatchAlarms, activate: func() { w.loadAlarms("") }},
		{key: moduleSSM, name: "SSM Parameter Store", defaultItem: "Parameters", aliases: []string{"ssm", "parameter store", "ssm parameter store"}, expander: ssmParameters, activate: func() { w.loadSSMPath("/", "") }},
		{key: moduleSecrets, name: "Secrets Manager", defaultItem: "Secrets", aliases: []string{"sm", "secrets", "secrets manager", "secrets-manager"}, expander: secretsManager, activate: func() { w.loadSecrets("", "") }},
		{key: moduleLambda, name: "Lambda", defaultItem: "Functions", aliases: []string{"lambda", "λ"}, expander: lambdaFunctions, activate: func() { w.loadLambdaFunctions("", "") }},
	}
	sortModuleRailSections(w.moduleSections)
	for _, section := range w.moduleSections {
		sidebar.Append(section.expander)
	}
	sidebar.Append(comingSoon)

	w.search = gtk.NewSearchEntry()
	w.search.SetPlaceholderText("Choose a module…")
	w.search.SetSensitive(false)
	w.search.ConnectSearchChanged(w.applyFilter)
	w.search.AddCSSClass("resource-search")
	w.activeTasksButton = gtk.NewToggleButtonWithLabel("Active")
	w.activeTasksButton.SetActive(true)
	w.activeTasksButton.ConnectClicked(func() {
		if w.activeTasksButton.Active() {
			w.switchTaskScope(false)
		}
	})
	w.stoppedTasksButton = gtk.NewToggleButtonWithLabel("Recently stopped")
	w.stoppedTasksButton.SetGroup(w.activeTasksButton)
	w.stoppedTasksButton.ConnectClicked(func() {
		if w.stoppedTasksButton.Active() {
			w.switchTaskScope(true)
		}
	})
	scopeButtons := gtk.NewBox(gtk.OrientationHorizontal, 8)
	scopeButtons.Append(w.activeTasksButton)
	scopeButtons.Append(w.stoppedTasksButton)
	w.loadMoreTasksButton = gtk.NewButtonWithLabel("Load more")
	w.loadMoreTasksButton.ConnectClicked(w.loadMoreStoppedTasks)
	spacer := gtk.NewLabel("")
	spacer.SetHExpand(true)
	w.taskScopeBar = gtk.NewBox(gtk.OrientationHorizontal, 8)
	w.taskScopeBar.Append(scopeButtons)
	w.taskScopeBar.Append(spacer)
	w.taskScopeBar.Append(w.loadMoreTasksButton)
	w.taskScopeBar.SetVisible(false)

	clusterScroll := gtk.NewScrolledWindow()
	clusterScroll.SetVExpand(true)
	clusterScroll.SetHExpand(true)
	clusterScroll.SetChild(w.clusterTable.view)
	serviceScroll := gtk.NewScrolledWindow()
	serviceScroll.SetVExpand(true)
	serviceScroll.SetHExpand(true)
	serviceScroll.SetChild(w.serviceTable.view)
	taskScroll := gtk.NewScrolledWindow()
	taskScroll.SetVExpand(true)
	taskScroll.SetHExpand(true)
	taskScroll.SetChild(w.taskTable.view)
	stoppedTaskScroll := gtk.NewScrolledWindow()
	stoppedTaskScroll.SetVExpand(true)
	stoppedTaskScroll.SetHExpand(true)
	stoppedTaskScroll.SetChild(w.stoppedTaskTable.view)
	taskDefinitionScroll := gtk.NewScrolledWindow()
	taskDefinitionScroll.SetVExpand(true)
	taskDefinitionScroll.SetHExpand(true)
	taskDefinitionScroll.SetChild(w.taskDefinitionTable.view)
	logGroupScroll := gtk.NewScrolledWindow()
	logGroupScroll.SetVExpand(true)
	logGroupScroll.SetHExpand(true)
	logGroupScroll.SetChild(w.logGroupTable.view)
	logStreamScroll := gtk.NewScrolledWindow()
	logStreamScroll.SetVExpand(true)
	logStreamScroll.SetHExpand(true)
	logStreamScroll.SetChild(w.logStreamTable.view)
	alarmScroll := gtk.NewScrolledWindow()
	alarmScroll.SetVExpand(true)
	alarmScroll.SetHExpand(true)
	alarmScroll.SetChild(w.alarmTable.view)
	ssmScroll := gtk.NewScrolledWindow()
	ssmScroll.SetVExpand(true)
	ssmScroll.SetHExpand(true)
	ssmScroll.SetChild(w.ssmTable.view)
	secretScroll := gtk.NewScrolledWindow()
	secretScroll.SetVExpand(true)
	secretScroll.SetHExpand(true)
	secretScroll.SetChild(w.secretTable.view)
	lambdaScroll := gtk.NewScrolledWindow()
	lambdaScroll.SetVExpand(true)
	lambdaScroll.SetHExpand(true)
	lambdaScroll.SetChild(w.lambdaTable.view)
	codeBuildProjectScroll := gtk.NewScrolledWindow()
	codeBuildProjectScroll.SetVExpand(true)
	codeBuildProjectScroll.SetHExpand(true)
	codeBuildProjectScroll.SetChild(w.codeBuildProjectTable.view)
	codeBuildBuildScroll := gtk.NewScrolledWindow()
	codeBuildBuildScroll.SetVExpand(true)
	codeBuildBuildScroll.SetHExpand(true)
	codeBuildBuildScroll.SetChild(w.codeBuildBuildTable.view)
	ec2Scroll := gtk.NewScrolledWindow()
	ec2Scroll.SetVExpand(true)
	ec2Scroll.SetHExpand(true)
	ec2Scroll.SetChild(w.ec2Table.view)
	ec2LoadBalancerScroll := gtk.NewScrolledWindow()
	ec2LoadBalancerScroll.SetVExpand(true)
	ec2LoadBalancerScroll.SetHExpand(true)
	ec2LoadBalancerScroll.SetChild(w.ec2LoadBalancerTable.view)
	ec2TargetGroupScroll := gtk.NewScrolledWindow()
	ec2TargetGroupScroll.SetVExpand(true)
	ec2TargetGroupScroll.SetHExpand(true)
	ec2TargetGroupScroll.SetChild(w.ec2TargetGroupTable.view)
	ec2SecurityGroupScroll := gtk.NewScrolledWindow()
	ec2SecurityGroupScroll.SetVExpand(true)
	ec2SecurityGroupScroll.SetHExpand(true)
	ec2SecurityGroupScroll.SetChild(w.ec2SecurityGroupTable.view)
	ec2VPCScroll := gtk.NewScrolledWindow()
	ec2VPCScroll.SetVExpand(true)
	ec2VPCScroll.SetHExpand(true)
	ec2VPCScroll.SetChild(w.ec2VPCTable.view)
	ec2SubnetScroll := gtk.NewScrolledWindow()
	ec2SubnetScroll.SetVExpand(true)
	ec2SubnetScroll.SetHExpand(true)
	ec2SubnetScroll.SetChild(w.ec2SubnetTable.view)
	ec2VolumeScroll := gtk.NewScrolledWindow()
	ec2VolumeScroll.SetVExpand(true)
	ec2VolumeScroll.SetHExpand(true)
	ec2VolumeScroll.SetChild(w.ec2VolumeTable.view)
	ecrRepositoryScroll := gtk.NewScrolledWindow()
	ecrRepositoryScroll.SetVExpand(true)
	ecrRepositoryScroll.SetHExpand(true)
	ecrRepositoryScroll.SetChild(w.ecrRepositoryTable.view)
	ecrImageScroll := gtk.NewScrolledWindow()
	ecrImageScroll.SetVExpand(true)
	ecrImageScroll.SetHExpand(true)
	ecrImageScroll.SetChild(w.ecrImageTable.view)
	ecrFindingScroll := gtk.NewScrolledWindow()
	ecrFindingScroll.SetVExpand(true)
	ecrFindingScroll.SetHExpand(true)
	ecrFindingScroll.SetChild(w.ecrFindingTable.view)
	rdsScroll := gtk.NewScrolledWindow()
	rdsScroll.SetVExpand(true)
	rdsScroll.SetHExpand(true)
	rdsScroll.SetChild(w.rdsTable.view)
	rdsClusterScroll := gtk.NewScrolledWindow()
	rdsClusterScroll.SetVExpand(true)
	rdsClusterScroll.SetHExpand(true)
	rdsClusterScroll.SetChild(w.rdsClusterTable.view)
	s3BucketScroll := gtk.NewScrolledWindow()
	s3BucketScroll.SetVExpand(true)
	s3BucketScroll.SetHExpand(true)
	s3BucketScroll.SetChild(w.s3BucketTable.view)
	s3ObjectScroll := gtk.NewScrolledWindow()
	s3ObjectScroll.SetVExpand(true)
	s3ObjectScroll.SetHExpand(true)
	s3ObjectScroll.SetChild(w.s3ObjectTable.view)
	dynamoTableScroll := gtk.NewScrolledWindow()
	dynamoTableScroll.SetVExpand(true)
	dynamoTableScroll.SetHExpand(true)
	dynamoTableScroll.SetChild(w.dynamoTable.view)
	dynamoItemScroll := gtk.NewScrolledWindow()
	dynamoItemScroll.SetVExpand(true)
	dynamoItemScroll.SetHExpand(true)
	dynamoItemScroll.SetChild(w.dynamoItemTable.view)
	sqsQueueScroll := gtk.NewScrolledWindow()
	sqsQueueScroll.SetVExpand(true)
	sqsQueueScroll.SetHExpand(true)
	sqsQueueScroll.SetChild(w.sqsQueueTable.view)
	sqsMessageScroll := gtk.NewScrolledWindow()
	sqsMessageScroll.SetVExpand(true)
	sqsMessageScroll.SetHExpand(true)
	sqsMessageScroll.SetChild(w.sqsMessageTable.view)
	route53ZoneScroll := gtk.NewScrolledWindow()
	route53ZoneScroll.SetVExpand(true)
	route53ZoneScroll.SetHExpand(true)
	route53ZoneScroll.SetChild(w.route53ZoneTable.view)
	route53RecordScroll := gtk.NewScrolledWindow()
	route53RecordScroll.SetVExpand(true)
	route53RecordScroll.SetHExpand(true)
	route53RecordScroll.SetChild(w.route53RecordTable.view)

	w.resourceStack = gtk.NewStack()
	w.resourceStack.SetVExpand(true)
	w.resourceStack.SetHExpand(true)
	modulePrompt := gtk.NewLabel("Choose a module to begin.")
	modulePrompt.SetXAlign(0)
	modulePrompt.SetYAlign(0)
	modulePrompt.SetMarginTop(12)
	modulePrompt.SetMarginStart(12)
	w.resourceStack.AddNamed(modulePrompt, pageModulePicker)
	w.resourceStack.AddNamed(clusterScroll, pageClusters)
	w.resourceStack.AddNamed(serviceScroll, pageServices)
	w.resourceStack.AddNamed(taskScroll, pageTasks)
	w.resourceStack.AddNamed(stoppedTaskScroll, pageStoppedTasks)
	w.resourceStack.AddNamed(taskDefinitionScroll, pageTaskDefinitions)
	w.resourceStack.AddNamed(logGroupScroll, pageLogGroups)
	w.resourceStack.AddNamed(logStreamScroll, pageLogStreams)
	w.resourceStack.AddNamed(alarmScroll, pageAlarms)
	w.resourceStack.AddNamed(ssmScroll, pageSSM)
	w.resourceStack.AddNamed(secretScroll, pageSecrets)
	w.resourceStack.AddNamed(lambdaScroll, pageLambda)
	w.resourceStack.AddNamed(codeBuildProjectScroll, pageCodeBuildProjects)
	w.resourceStack.AddNamed(codeBuildBuildScroll, pageCodeBuildBuilds)
	w.resourceStack.AddNamed(ec2Scroll, pageEC2Instances)
	w.resourceStack.AddNamed(ec2LoadBalancerScroll, pageEC2LoadBalancers)
	w.resourceStack.AddNamed(ec2TargetGroupScroll, pageEC2TargetGroups)
	w.resourceStack.AddNamed(ec2SecurityGroupScroll, pageEC2SecurityGroups)
	w.resourceStack.AddNamed(ec2VPCScroll, pageEC2VPCs)
	w.resourceStack.AddNamed(ec2SubnetScroll, pageEC2Subnets)
	w.resourceStack.AddNamed(ec2VolumeScroll, pageEC2Volumes)
	w.resourceStack.AddNamed(ecrRepositoryScroll, pageECRRepositories)
	w.resourceStack.AddNamed(ecrImageScroll, pageECRImages)
	w.resourceStack.AddNamed(ecrFindingScroll, pageECRFindings)
	w.resourceStack.AddNamed(rdsScroll, pageRDSInstances)
	w.resourceStack.AddNamed(rdsClusterScroll, pageRDSClusters)
	w.resourceStack.AddNamed(s3BucketScroll, pageS3Buckets)
	w.resourceStack.AddNamed(s3ObjectScroll, pageS3Objects)
	w.resourceStack.AddNamed(dynamoTableScroll, pageDynamoTables)
	w.resourceStack.AddNamed(dynamoItemScroll, pageDynamoItems)
	w.resourceStack.AddNamed(sqsQueueScroll, pageSQSQueues)
	w.resourceStack.AddNamed(sqsMessageScroll, pageSQSMessages)
	w.resourceStack.AddNamed(route53ZoneScroll, pageRoute53Zones)
	w.resourceStack.AddNamed(route53RecordScroll, pageRoute53Records)
	savedLogScope := gtk.NewLabel("Saved CloudWatch Logs destination")
	savedLogScope.SetXAlign(0)
	savedLogScope.SetYAlign(0)
	savedLogScope.SetMarginTop(12)
	savedLogScope.SetMarginStart(12)
	w.resourceStack.AddNamed(savedLogScope, pageSavedLogSearch)
	w.resourceStack.SetVisibleChildName(pageModulePicker)

	resourcePane := gtk.NewBox(gtk.OrientationVertical, 8)
	resourcePane.AddCSSClass("resource-pane")
	resourcePane.Append(w.search)
	resourcePane.Append(w.taskScopeBar)
	resourcePane.Append(w.resourceStack)

	w.detailBuffer = gtk.NewTextBuffer(nil)
	w.detailText = "Choose a module from the picker or press Ctrl+P."
	w.detailBuffer.SetText(w.detailText)
	w.detailHeadingTag = gtk.NewTextTag("detail-heading")
	w.detailHeadingTag.SetObjectProperty("weight", int(pango.WeightBold))
	w.detailBuffer.TagTable().Add(w.detailHeadingTag)
	w.detailErrorTag = gtk.NewTextTag("detail-error")
	w.detailErrorTag.SetObjectProperty("weight", int(pango.WeightBold))
	w.detailBuffer.TagTable().Add(w.detailErrorTag)
	w.detailParentButton = gtk.NewButtonWithLabel("Back to service details")
	w.detailParentButton.ConnectClicked(w.showParentDetails)
	w.detailToolbar = gtk.NewBox(gtk.OrientationHorizontal, 8)
	w.detailToolbar.AddCSSClass("log-toolbar")
	w.detailToolbar.Append(w.detailParentButton)
	w.detailToolbar.SetVisible(false)
	w.detailView = gtk.NewTextViewWithBuffer(w.detailBuffer)
	w.detailView.SetEditable(false)
	w.detailView.SetCursorVisible(false)
	w.detailView.SetMonospace(true)
	w.detailView.SetWrapMode(gtk.WrapWordChar)
	w.detailView.AddCSSClass("inspector")
	w.installDetailLinkControllers()
	detailScroll := gtk.NewScrolledWindow()
	detailScroll.SetVExpand(true)
	detailScroll.SetHExpand(true)
	detailScroll.SetChild(w.detailView)
	detailPane := gtk.NewBox(gtk.OrientationVertical, 0)
	detailPane.Append(w.detailToolbar)
	w.detailLinks = gtk.NewMenuButton()
	w.detailLinks.SetLabel("Linked resources")
	w.detailLinks.SetMarginStart(8)
	w.detailLinks.SetHAlign(gtk.AlignStart)
	w.detailLinks.SetMarginTop(6)
	w.detailLinks.SetVisible(false)
	detailPane.Append(w.detailLinks)
	detailPane.Append(detailScroll)

	w.detailStack = gtk.NewStack()
	w.detailStack.SetVExpand(true)
	w.detailStack.SetHExpand(true)
	w.detailStack.AddNamed(detailPane, "detail")
	w.detailStack.AddNamed(w.buildLogPane(), "logs")
	w.detailStack.AddNamed(w.buildMetricsPane(), "metrics")
	w.detailStack.AddNamed(w.buildTaskDefinitionPane(), "task-definition")
	w.detailStack.AddNamed(w.buildTaskDefinitionEditor(), "editor")
	w.detailStack.AddNamed(w.buildLambdaCodeEditor(), "lambda-editor")
	w.detailStack.AddNamed(w.buildTerminalPane(), "terminal")
	w.detailStack.SetVisibleChildName("detail")
	w.workspaceBusySpinner = gtk.NewSpinner()
	w.workspaceBusyLabel = gtk.NewLabel("")
	w.workspaceBusyLabel.SetXAlign(0)
	w.workspaceBusyLabel.SetHExpand(true)
	w.workspaceBusyBar = gtk.NewBox(gtk.OrientationHorizontal, 8)
	w.workspaceBusyBar.AddCSSClass("workspace-busy")
	w.workspaceBusyBar.Append(w.workspaceBusySpinner)
	w.workspaceBusyBar.Append(w.workspaceBusyLabel)
	w.workspaceCancelButton = gtk.NewButtonWithLabel("Cancel")
	w.workspaceCancelButton.SetVisible(false)
	w.workspaceCancelButton.ConnectClicked(func() {
		if w.workspaceCancel != nil {
			w.workspaceCancel()
		}
	})
	w.workspaceBusyBar.Append(w.workspaceCancelButton)
	w.workspaceBusyBar.SetVisible(false)
	workspace := gtk.NewBox(gtk.OrientationVertical, 0)
	workspace.Append(w.workspaceBusyBar)
	workspace.Append(w.detailStack)

	contentSplit := gtk.NewPaned(gtk.OrientationHorizontal)
	contentSplit.SetStartChild(resourcePane)
	contentSplit.SetEndChild(workspace)
	contentSplit.SetPosition(700)
	contentSplit.SetResizeStartChild(true)
	contentSplit.SetResizeEndChild(true)
	w.installPaneZoom(&resourcePane.Widget, &workspace.Widget)

	mainSplit := gtk.NewPaned(gtk.OrientationHorizontal)
	sidebarScroll := gtk.NewScrolledWindow()
	sidebarScroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	sidebarScroll.SetChild(sidebar)
	mainSplit.SetStartChild(sidebarScroll)
	mainSplit.SetEndChild(contentSplit)
	mainSplit.SetPosition(190)
	mainSplit.SetResizeStartChild(false)
	mainSplit.SetResizeEndChild(true)
	mainSplit.SetShrinkStartChild(false)

	w.spinner = gtk.NewSpinner()
	w.status = gtk.NewLabel("Ready")
	w.status.SetXAlign(0)
	w.status.SetHExpand(true)
	w.status.SetEllipsize(pango.EllipsizeEnd)
	w.statusDetailsButton = gtk.NewButtonWithLabel("Details…")
	w.statusDetailsButton.AddCSSClass("flat")
	w.statusDetailsButton.SetVisible(false)
	w.statusDetailsButton.ConnectClicked(w.showStatusErrorDetails)
	w.statusDismissButton = gtk.NewButtonWithLabel("Dismiss")
	w.statusDismissButton.AddCSSClass("flat")
	w.statusDismissButton.SetVisible(false)
	w.statusDismissButton.ConnectClicked(w.dismissStatusError)
	identity := gtk.NewLabel(fmt.Sprintf("profile: %s   region: %s", valueOrDash(w.options.Profile), valueOrDash(w.options.Region)))
	identity.AddCSSClass("muted")
	footer := gtk.NewBox(gtk.OrientationHorizontal, 8)
	footer.AddCSSClass("status-bar")
	footer.Append(w.spinner)
	footer.Append(w.status)
	footer.Append(w.statusDetailsButton)
	footer.Append(w.statusDismissButton)
	footer.Append(identity)

	root := gtk.NewBox(gtk.OrientationVertical, 0)
	root.AddCSSClass("e9s-root")
	root.Append(header)
	root.Append(mainSplit)
	root.Append(footer)
	w.updateActionSensitivity()
	return root
}

func newModuleRailButton(label string, activate func()) *gtk.ToggleButton {
	text := gtk.NewLabel(label)
	text.SetXAlign(0)
	text.SetHAlign(gtk.AlignFill)
	text.SetHExpand(true)
	text.SetWidthChars(1)
	text.SetMaxWidthChars(18)
	text.SetEllipsize(pango.EllipsizeEnd)
	button := gtk.NewToggleButton()
	button.SetChild(text)
	button.SetHAlign(gtk.AlignFill)
	button.AddCSSClass("flat")
	button.AddCSSClass("module-subitem")
	button.ConnectClicked(activate)
	return button
}

func (w *mainWindow) installActions(app *gtk.Application) {
	w.addAction(app, "refresh", []string{"<Control>r"}, w.refresh)
	w.addAction(app, "search", nil, func() {
		if w.showingLogs {
			w.logSearch.GrabFocus()
		} else {
			w.search.GrabFocus()
		}
	})
	w.addAction(app, "back", []string{"Escape"}, w.goBack)
	w.addAction(app, "modes", []string{"<Control>p"}, w.showModulePicker)
	w.addAction(app, "help", nil, func() {
		if w.showingTerminal {
			w.setStatus("Disconnect the terminal session before opening help", false)
			return
		}
		if w.showingEditor {
			w.setStatus("Close the task-definition editor before opening help", false)
			return
		}
		w.setDetail("KEYBOARD SHORTCUTS\n\nEnter          Open selected row or task\nEscape         Back / close auxiliary view\n/              Focus active filter\nCtrl++/-       Zoom active pane in/out\nCtrl+0         Reset active pane zoom\nCtrl+R         Refresh\nShift+S        Toggle standalone/service tasks\nCtrl+Enter     Run standalone task\nShift+T        Browse task definitions\nE              Task-definition environment\nD              Diff previous revision\nCtrl+E         Edit task-definition JSON\nCtrl+S         Register edited revision\nCtrl+Shift+E   ECS Exec in embedded terminal\nM              Service or selected-task metrics\nShift+L        Follow service logs\nCtrl+Shift+L   Follow selected task logs\nCtrl+Space     Pause/resume logs\nT              Cycle log timestamps\nCtrl+Shift+C   Copy log buffer\nCtrl+L         Clear log buffer\nCtrl+Shift+S   Scale service\nCtrl+Shift+A   Toggle scale-in suspension\nCtrl+Shift+X   Stop selected task\nCtrl+Shift+R   Force deployment\nCtrl+P         Open module picker\n?              Show this help", detailHelp)
		w.detailStack.SetVisibleChildName("detail")
	})
	w.addAction(app, "logs", nil, w.openServiceLogs)
	w.addAction(app, "task-logs", []string{"<Control><Shift>l"}, w.openTaskLogs)
	w.addAction(app, "standalone-tasks", nil, w.toggleStandaloneTasks)
	w.addAction(app, "run-task", []string{"<Control>Return"}, w.promptRunTask)
	w.addAction(app, "metrics", nil, w.openMetrics)
	w.addAction(app, "toggle-scale-in", []string{"<Control><Shift>a"}, w.confirmToggleScaleIn)
	w.addAction(app, "task-definitions", nil, w.openTaskDefinitions)
	w.addAction(app, "task-definition-env", nil, w.openTaskDefinitionEnvironment)
	w.addAction(app, "task-definition-diff", nil, w.openTaskDefinitionDiff)
	w.addAction(app, "task-definition-edit", []string{"<Control>e"}, w.openTaskDefinitionEditor)
	w.addAction(app, "task-definition-register", []string{"<Control>s"}, w.confirmRegisterTaskDefinition)
	w.addAction(app, "ecs-exec", []string{"<Control><Shift>e"}, w.openExec)
	w.addAction(app, "toggle-logs", []string{"<Control>space"}, w.toggleLogFollow)
	w.addAction(app, "log-timestamps", nil, func() {
		if w.showingLogs {
			w.cycleLogTimestamps()
		}
	})
	w.addAction(app, "copy-logs", []string{"<Control><Shift>c"}, w.copyLogs)
	w.addAction(app, "clear-logs", []string{"<Control>l"}, w.clearLogs)
	w.addAction(app, "force-deploy", []string{"<Control><Shift>r"}, w.confirmForceDeployment)
	w.addAction(app, "scale-service", []string{"<Control><Shift>s"}, w.promptScaleService)
	w.addAction(app, "stop-task", []string{"<Control><Shift>x"}, w.confirmStopTask)
	w.addAction(app, "zoom-in", zoomInAccelerators, func() {
		w.adjustActivePaneZoom(1)
	})
	w.addAction(app, "zoom-out", zoomOutAccelerators, func() {
		w.adjustActivePaneZoom(-1)
	})
	w.addAction(app, "zoom-reset", zoomResetAccelerators, w.resetActivePaneZoom)
}

func (w *mainWindow) installPrintableShortcuts(app *gtk.Application) {
	keys := gtk.NewEventControllerKey()
	keys.SetPropagationPhase(gtk.PhaseCapture)
	keys.ConnectKeyPressed(func(keyval, _ uint, state gdk.ModifierType) bool {
		if keyval == gdk.KEY_Escape && w.showingMetrics && w.metricsFocusedTitle != "" {
			w.restoreMetricCharts()
			return true
		}
		action := printableShortcutAction(keyval, state)
		if action == "" || w.focusAcceptsTextInput() {
			return false
		}
		app.ActivateAction(action, nil)
		return true
	})
	w.window.AddController(keys)
}

func printableShortcutAction(keyval uint, state gdk.ModifierType) string {
	modifiers := state & (gdk.ShiftMask | gdk.ControlMask | gdk.AltMask | gdk.SuperMask | gdk.HyperMask | gdk.MetaMask)
	if modifiers == 0 {
		switch keyval {
		case gdk.KEY_slash:
			return "search"
		case gdk.KEY_m:
			return "metrics"
		case gdk.KEY_e:
			return "task-definition-env"
		case gdk.KEY_d:
			return "task-definition-diff"
		case gdk.KEY_t:
			return "log-timestamps"
		}
	}
	if modifiers == gdk.ShiftMask {
		switch keyval {
		case gdk.KEY_question:
			return "help"
		case gdk.KEY_L:
			return "logs"
		case gdk.KEY_S:
			return "standalone-tasks"
		case gdk.KEY_T:
			return "task-definitions"
		}
	}
	return ""
}

func (w *mainWindow) focusAcceptsTextInput() bool {
	if w.showingTerminal {
		return true
	}
	focus := w.window.Focus()
	if focus == nil {
		return false
	}
	object := glib.BaseObject(focus)
	if object.IsA(gtk.GTypeEditableTextWidget) {
		if editable, ok := focus.(interface{ Editable() bool }); ok {
			return editable.Editable()
		}
		return true
	}
	if object.IsA(gtk.GTypeTextView) {
		if editable, ok := focus.(interface{ Editable() bool }); ok {
			return editable.Editable()
		}
		return true
	}
	return false
}

func (w *mainWindow) addAction(app *gtk.Application, name string, accels []string, run func()) {
	action := gio.NewSimpleAction(name, nil)
	action.ConnectActivate(func(_ *glib.Variant) { run() })
	app.AddAction(action)
	app.SetAccelsForAction("app."+name, accels)
}

func (w *mainWindow) startRequest(label string) (context.Context, uint64) {
	return w.startRefreshRequest(label, true)
}

func (w *mainWindow) startRefreshRequest(label string, foreground bool) (context.Context, uint64) {
	if w.requestCancel != nil {
		w.requestCancel()
	}
	ctx, cancel := context.WithCancel(w.ctx)
	w.requestCancel = cancel
	w.generation++
	if foreground {
		w.spinner.Start()
		w.setStatus(label, false)
		w.setWorkspaceBusy(label, true)
	}
	return ctx, w.generation
}

func (w *mainWindow) finishRequest(ctx context.Context, generation uint64, err error, apply func()) {
	w.finishRequestWithStatus(ctx, generation, err, "", apply)
}

func (w *mainWindow) finishRequestWithStatus(ctx context.Context, generation uint64, err error, success string, apply func()) {
	w.finishRequestResult(ctx, generation, err, success, true, true, apply)
}

func (w *mainWindow) finishRefreshRequest(ctx context.Context, generation uint64, err error, foreground bool, apply func()) {
	w.finishRequestResult(ctx, generation, err, "", false, foreground, apply)
}

func (w *mainWindow) finishRequestResult(ctx context.Context, generation uint64, err error, success string, showDetailError, foreground bool, apply func()) {
	glib.IdleAdd(func() {
		if ctx.Err() != nil || generation != w.generation {
			return
		}
		if !foreground && w.showingLogs {
			return
		}
		if foreground {
			w.spinner.Stop()
			w.setWorkspaceBusy("", false)
		}
		if err != nil {
			if foreground || !w.workspaceBusy {
				w.setStatus(err.Error(), true)
			}
			if showDetailError {
				w.setDetail("ERROR\n\n"+err.Error(), detailError)
			}
			return
		}
		w.lastSuccessfulLoad = time.Now()
		apply()
		if !foreground && w.workspaceBusy {
			return
		}
		if success == "" {
			success = "Updated " + w.lastSuccessfulLoad.Format("15:04:05")
		}
		w.setStatus(success, false)
	})
}

func (w *mainWindow) setDetail(text, content string) {
	w.updateDetailParentAction(content)
	if w.detailText == text && w.detailContent == content {
		return
	}
	w.clearDetailResourceLinks()
	w.detailBuffer.SetText(text)
	w.detailText = text
	w.detailContent = content
	w.applyDetailHeadingStyles()
}

func (w *mainWindow) setWorkspaceBusy(label string, busy bool) {
	if w.workspaceBusyBar == nil {
		return
	}
	if !busy {
		w.workspaceBusy = false
		w.workspaceBusySpinner.Stop()
		w.workspaceBusyBar.SetVisible(false)
		w.workspaceCancel = nil
		if w.workspaceCancelButton != nil {
			w.workspaceCancelButton.SetVisible(false)
			w.workspaceCancelButton.SetSensitive(true)
		}
		return
	}
	w.workspaceBusy = true
	w.workspaceBusyLabel.SetLabel(label)
	w.workspaceBusySpinner.Start()
	w.workspaceBusyBar.SetVisible(true)
}

func (w *mainWindow) setWorkspaceCancellation(cancel func()) {
	w.workspaceCancel = cancel
	if w.workspaceCancelButton != nil {
		w.workspaceCancelButton.SetSensitive(cancel != nil)
		w.workspaceCancelButton.SetVisible(cancel != nil)
	}
}

func (w *mainWindow) setBreadcrumb(text string) {
	if w.breadcrumbText == text {
		return
	}
	w.breadcrumbText = text
	w.breadcrumb.QueueDraw()
}

func (w *mainWindow) newBreadcrumbArea() *gtk.DrawingArea {
	area := gtk.NewDrawingArea()
	area.SetHExpand(true)
	area.AddCSSClass("breadcrumb")
	area.SetDrawFunc(func(area *gtk.DrawingArea, cr *cairo.Context, width, height int) {
		style := area.StyleContext()
		clearDrawingSurface(cr)

		foreground := semanticPaletteFromStyle(style).foreground
		cr.SetSourceRGBA(
			float64(foreground.Red()),
			float64(foreground.Green()),
			float64(foreground.Blue()),
			float64(foreground.Alpha()),
		)
		layout := area.CreatePangoLayout(w.breadcrumbText)
		_, textHeight := layout.PixelSize()
		cr.MoveTo(0, float64(max(0, height-textHeight)/2))
		pangocairo.ShowLayout(cr, layout)
	})
	return area
}

func (w *mainWindow) updateDetailParentAction(content string) {
	if w.detailToolbar == nil {
		return
	}
	visible := content == detailTask && (w.currentPage == pageTasks || w.currentPage == pageStandaloneTasks)
	w.detailToolbar.SetVisible(visible)
	if !visible {
		return
	}
	if w.currentPage == pageStandaloneTasks {
		if w.showingStoppedTasks {
			w.detailParentButton.SetLabel("Back to recently stopped tasks")
		} else {
			w.detailParentButton.SetLabel("Back to standalone summary")
		}
	} else {
		w.detailParentButton.SetLabel("Back to service summary")
	}
}

func (w *mainWindow) showParentDetails() {
	if w.selectedTask == "" {
		return
	}
	if w.currentPage == pageTasks {
		if _, found := findService(w.allServices, w.selectedService); !found {
			w.setStatus("The selected service is no longer available", true)
			return
		}
	}
	w.selectedTask = ""
	if w.showingStoppedTasks {
		w.stoppedTaskTable.selection.SetSelected(gtk.InvalidListPosition)
	} else {
		w.taskTable.selection.SetSelected(gtk.InvalidListPosition)
	}
	w.updateActionSensitivity()
	if w.currentPage == pageStandaloneTasks {
		w.setBreadcrumb(w.standaloneTaskBreadcrumb())
		w.setDetail(w.standaloneTaskSummary(), detailClusterSummary)
		return
	}
	service, _ := findService(w.allServices, w.selectedService)
	w.setBreadcrumb(w.serviceTaskBreadcrumb())
	w.renderServiceTaskSummary(service, "")
}

func (w *mainWindow) resetWorkspaceForBrowserChange() {
	w.setStatus("Ready", false)
	if w.requestCancel != nil {
		w.requestCancel()
		w.requestCancel = nil
		w.generation++
	}
	w.spinner.Stop()
	w.setWorkspaceBusy("", false)
	if w.logCancel != nil {
		w.logCancel()
		w.logCancel = nil
		w.logGeneration++
	}
	w.logFollowing = false
	w.logNewerKnown = 0
	w.logNewestKnownTS = 0
	w.logSearchSpec = nil
	w.logHighlightRules = nil
	w.logHiddenStreams = nil
	w.updateLogHighlightButton()
	w.activeSavedLog = ""
	w.alarmDetail = nil
	w.alarmActionPending = false
	w.ssmActionPending = false
	w.ssmDetail = nil
	w.secretActionPending = false
	w.secretDetail = nil
	w.lambdaActionPending = false
	w.lambdaDetail = nil
	w.lambdaEnvironment = nil
	w.lambdaEnvironmentResolved = false
	w.lambdaViewMode = ""
	w.codeBuildDetail = nil
	w.codeBuildActionPending = false
	w.ec2Detail = nil
	w.rdsDetail = nil
	w.s3DownloadPending = false
	w.dynamoActionPending = false
	w.sqsActionPending = false
	w.route53ActionPending = false
	w.ec2ViewMode = ""
	w.ec2ActionPending = false
	w.ec2SecurityGroupDetail = nil
	w.ec2VPCDetail = nil
	w.ec2SubnetDetail = nil
	w.ec2VolumeDetail = nil
	w.showingLogs = false
	w.showingMetrics = false
	w.metricsKind = ""
	w.metricsSnapshot = nil
	w.metricsGenericSnapshot = nil
	w.metricsFocusedTitle = ""
	w.metricsRenderedSnapshot = nil
	w.metricsChartSpecs = nil
	if w.showingTerminal {
		w.closeTerminalNow(false)
	}
	w.showingEditor = false
	w.editorDirty = false
	w.editorKind = ""
	w.discardLambdaEditor()
	if w.detailToolbar != nil {
		w.detailToolbar.SetVisible(false)
	}
	w.detailStack.SetVisibleChildName("detail")
}

func (w *mainWindow) clearTaskBrowser() {
	w.selectedTask = ""
	w.allTasks = nil
	w.filteredTasks = nil
	w.taskNextToken = ""
	w.taskTable.clear()
	w.stoppedTaskTable.clear()
	if w.resourceStack != nil {
		w.resourceStack.QueueDraw()
	}
}

func (w *mainWindow) clearClusterBrowser() {
	w.allClusters = nil
	w.filteredClusters = nil
	w.clusterTable.clear()
}

func (w *mainWindow) clearServiceBrowser() {
	w.allServices = nil
	w.filteredServices = nil
	w.serviceTable.clear()
}

func (w *mainWindow) openClustersModule() {
	if w.currentPage == pageClusters {
		return
	}
	if w.guardEditorNavigation(w.loadClusters) {
		return
	}
	w.loadClusters()
}

func (w *mainWindow) loadClusters() {
	w.resetWorkspaceForBrowserChange()
	if w.currentPage == pageTasks || w.currentPage == pageStandaloneTasks {
		w.clearTaskBrowser()
	}
	w.clearClusterBrowser()
	w.currentPage = pageClusters
	w.selectedCluster = ""
	w.selectedService = ""
	w.selectedTask = ""
	w.selectedTaskDefinition = nil
	w.updateActionSensitivity()
	w.setBreadcrumb("ECS / Clusters")
	w.backButton.SetSensitive(false)
	w.search.SetPlaceholderText("Filter clusters…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageClusters)
	w.applyClusterFilter()
	w.setDetail("Select a cluster and press Enter to browse its services.", detailIntro)
	ctx, generation := w.startRequest("Loading ECS clusters…")
	go func() {
		clusters, err := w.options.ECS.ListClusters(ctx)
		w.finishRequest(ctx, generation, err, func() {
			w.allClusters = clusters
			w.applyClusterFilter()
			if len(clusters) == 0 {
				w.setDetail("No ECS clusters found in this region.", detailIntro)
			} else {
				w.setDetail(clusterListSummary(len(clusters)), detailIntro)
			}
			if w.options.DefaultCluster != "" {
				name := w.options.DefaultCluster
				w.options.DefaultCluster = ""
				w.openClusterByName(name)
			}
		})
	}()
}

func (w *mainWindow) loadServices(cluster string) {
	w.resetWorkspaceForBrowserChange()
	if w.currentPage == pageTasks || w.currentPage == pageStandaloneTasks {
		w.clearTaskBrowser()
	}
	w.clearServiceBrowser()
	w.currentPage = pageServices
	w.selectedCluster = cluster
	w.selectedService = ""
	w.selectedTask = ""
	w.updateActionSensitivity()
	w.setBreadcrumb("ECS / " + cluster)
	w.backButton.SetSensitive(true)
	w.search.SetPlaceholderText("Filter services…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageServices)
	w.setDetail("Loading services for "+cluster+"…", detailClusterSummary)

	ctx, generation := w.startRequest("Loading services in " + cluster + "…")
	go func() {
		services, err := w.options.ECS.ListServices(ctx, cluster)
		w.finishRequest(ctx, generation, err, func() {
			w.allServices = services
			w.applyServiceFilter()
			if len(services) == 0 {
				w.setDetail("No ECS services found in "+cluster+".", detailClusterSummary)
			} else {
				w.setDetail(clusterSummary(cluster, len(services)), detailClusterSummary)
			}
		})
	}()
}

func (w *mainWindow) loadTasks(service model.Service) {
	w.loadServiceTaskScope(service, false)
}

func (w *mainWindow) loadServiceTaskScope(service model.Service, stopped bool) {
	w.resetWorkspaceForBrowserChange()
	w.currentPage = pageTasks
	w.selectedService = service.Name
	w.showingStoppedTasks = stopped
	w.clearTaskBrowser()
	w.activeTasksButton.SetActive(!stopped)
	w.stoppedTasksButton.SetActive(stopped)
	w.updateActionSensitivity()
	w.setBreadcrumb(w.serviceTaskBreadcrumb())
	w.backButton.SetSensitive(true)
	if stopped {
		w.search.SetPlaceholderText("Filter recently stopped service tasks…")
	} else {
		w.search.SetPlaceholderText("Filter active service tasks…")
	}
	w.search.SetText("")
	if stopped {
		w.resourceStack.SetVisibleChildName(pageStoppedTasks)
		w.applyTaskFilter()
		w.renderServiceTaskSummary(service, "Loading recently stopped tasks…")
	} else {
		w.resourceStack.SetVisibleChildName(pageTasks)
		w.applyTaskFilter()
		w.renderServiceTaskSummary(service, "Loading active tasks…")
	}

	cluster := w.selectedCluster
	label := "Loading active tasks for " + service.Name + "…"
	if stopped {
		label = "Loading recently stopped tasks for " + service.Name + "…"
	}
	ctx, generation := w.startRequest(label)
	go func() {
		var (
			tasks     []model.Task
			nextToken string
			err       error
		)
		if stopped {
			page, pageErr := w.options.ECS.ListStoppedServiceTasks(ctx, cluster, service.Name, "", taskHistoryBatchSize)
			tasks, nextToken, err = page.Tasks, page.NextToken, pageErr
			sortStoppedTasks(tasks)
		} else {
			tasks, err = w.options.ECS.ListTasks(ctx, cluster, service.Name)
		}
		w.finishRequest(ctx, generation, err, func() {
			w.allTasks = tasks
			w.taskNextToken = nextToken
			w.applyTaskFilter()
			w.updateActionSensitivity()
			w.renderServiceTaskSummary(service, "")
		})
	}()
}

func (w *mainWindow) loadStandaloneTasks() {
	if w.selectedCluster == "" {
		return
	}
	if w.currentPage != pageStandaloneTasks {
		w.standaloneReturnPage = w.currentPage
		w.standaloneReturnService = w.selectedService
		w.standaloneReturnStopped = w.showingStoppedTasks
	}
	w.loadStandaloneTaskScope(false)
}

func (w *mainWindow) switchTaskScope(stopped bool) {
	if (w.currentPage != pageTasks && w.currentPage != pageStandaloneTasks) || w.showingStoppedTasks == stopped {
		return
	}
	if w.currentPage == pageStandaloneTasks {
		w.loadStandaloneTaskScope(stopped)
		return
	}
	service, found := findService(w.allServices, w.selectedService)
	if !found {
		w.setStatus("The selected service is no longer available", true)
		return
	}
	w.loadServiceTaskScope(service, stopped)
}

func (w *mainWindow) loadStandaloneTaskScope(stopped bool) {
	w.resetWorkspaceForBrowserChange()
	w.currentPage = pageStandaloneTasks
	w.selectedService = ""
	w.showingStoppedTasks = stopped
	w.clearTaskBrowser()
	w.activeTasksButton.SetActive(!stopped)
	w.stoppedTasksButton.SetActive(stopped)
	w.updateActionSensitivity()
	w.setBreadcrumb(w.standaloneTaskBreadcrumb())
	w.backButton.SetSensitive(true)
	if stopped {
		w.search.SetPlaceholderText("Filter recently stopped tasks…")
	} else {
		w.search.SetPlaceholderText("Filter active standalone tasks…")
	}
	w.search.SetText("")
	if stopped {
		w.resourceStack.SetVisibleChildName(pageStoppedTasks)
		w.applyTaskFilter()
		w.setDetail("Loading recently stopped standalone tasks…", detailClusterSummary)
	} else {
		w.resourceStack.SetVisibleChildName(pageTasks)
		w.applyTaskFilter()
		w.setDetail("Loading active standalone tasks…", detailClusterSummary)
	}

	cluster := w.selectedCluster
	label := "Loading active standalone tasks in " + cluster + "…"
	if stopped {
		label = "Loading recently stopped tasks in " + cluster + "…"
	}
	ctx, generation := w.startRequest(label)
	go func() {
		var (
			tasks     []model.Task
			nextToken string
			err       error
		)
		if stopped {
			page, pageErr := w.options.ECS.ListStoppedStandaloneTasks(ctx, cluster, "", taskHistoryBatchSize)
			tasks, nextToken, err = page.Tasks, page.NextToken, pageErr
			sortStoppedTasks(tasks)
		} else {
			tasks, err = w.options.ECS.ListStandaloneTasks(ctx, cluster)
		}
		w.finishRequest(ctx, generation, err, func() {
			w.allTasks = tasks
			w.taskNextToken = nextToken
			w.applyTaskFilter()
			w.updateActionSensitivity()
			w.setDetail(w.standaloneTaskSummary(), detailClusterSummary)
		})
	}()
}

func (w *mainWindow) loadMoreStoppedTasks() {
	if (w.currentPage != pageTasks && w.currentPage != pageStandaloneTasks) || !w.showingStoppedTasks || w.taskNextToken == "" {
		return
	}
	cluster, service, nextToken := w.selectedCluster, w.selectedService, w.taskNextToken
	ctx, generation := w.startRequest("Loading more recently stopped tasks…")
	go func() {
		var page model.TaskPage
		var err error
		if service == "" {
			page, err = w.options.ECS.ListStoppedStandaloneTasks(ctx, cluster, nextToken, taskHistoryBatchSize)
		} else {
			page, err = w.options.ECS.ListStoppedServiceTasks(ctx, cluster, service, nextToken, taskHistoryBatchSize)
		}
		w.finishRequestWithStatus(ctx, generation, err,
			fmt.Sprintf("Loaded %d more recently stopped tasks", len(page.Tasks)), func() {
				w.allTasks = appendUniqueTasks(w.allTasks, page.Tasks)
				sortStoppedTasks(w.allTasks)
				w.taskNextToken = page.NextToken
				w.applyTaskFilter()
				w.updateActionSensitivity()
				if w.selectedTask == "" {
					if w.currentPage == pageStandaloneTasks && w.detailContent == detailClusterSummary {
						w.setDetail(w.standaloneTaskSummary(), detailClusterSummary)
					} else if w.currentPage == pageTasks && w.detailContent == detailService {
						if selectedService, found := findService(w.allServices, service); found {
							w.renderServiceTaskSummary(selectedService, "")
						}
					}
				}
			})
	}()
}

func (w *mainWindow) toggleStandaloneTasks() {
	if w.currentPage != pageStandaloneTasks {
		w.loadStandaloneTasks()
		return
	}
	cluster := w.selectedCluster
	if cluster == "" {
		return
	}
	if w.standaloneReturnPage == pageTasks && w.standaloneReturnService != "" {
		if service, found := findService(w.allServices, w.standaloneReturnService); found {
			w.loadServiceTaskScope(service, w.standaloneReturnStopped)
			return
		}
	}
	w.loadServices(cluster)
}

func (w *mainWindow) standaloneTaskBreadcrumb() string {
	breadcrumb := "ECS / " + w.selectedCluster + " / Standalone tasks"
	if w.showingStoppedTasks {
		breadcrumb += " / Recently stopped"
	}
	return breadcrumb
}

func (w *mainWindow) standaloneTaskSummary() string {
	if w.showingStoppedTasks {
		return stoppedStandaloneTaskSummary(w.selectedCluster, w.allTasks, w.taskNextToken != "")
	}
	return standaloneTaskSummary(w.selectedCluster, w.allTasks)
}

func (w *mainWindow) serviceTaskBreadcrumb() string {
	breadcrumb := "ECS / " + w.selectedCluster + " / " + w.selectedService
	if w.showingStoppedTasks {
		breadcrumb += " / Recently stopped"
	}
	return breadcrumb
}

func (w *mainWindow) serviceTaskSummary(service model.Service) string {
	return formatServiceTaskContext(w.selectedCluster, service, w.allTasks, w.showingStoppedTasks, w.taskNextToken != "")
}

func (w *mainWindow) openClusterAt(position uint) {
	if int(position) >= len(w.filteredClusters) {
		return
	}
	w.loadServices(w.filteredClusters[position].Name)
}

func (w *mainWindow) selectClusterRow() {
	if w.currentPage != pageClusters {
		return
	}
	position := w.clusterTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredClusters) {
		if w.selectedCluster != "" {
			w.resetWorkspaceForBrowserChange()
			w.selectedCluster = ""
			w.setBreadcrumb("ECS / Clusters")
			w.setDetail(clusterListSummary(len(w.allClusters)), detailIntro)
			w.updateActionSensitivity()
		}
		return
	}
	cluster := w.filteredClusters[position]
	if w.selectedCluster != cluster.Name {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedCluster = cluster.Name
	w.setBreadcrumb("ECS / Clusters / " + cluster.Name)
	w.setDetail(formatClusterDetail(cluster), detailClusterSummary)
	w.updateActionSensitivity()
}

func (w *mainWindow) openServiceAt(position uint) {
	if int(position) >= len(w.filteredServices) {
		return
	}
	w.loadTasks(w.filteredServices[position])
}

func (w *mainWindow) selectServiceRow() {
	if w.currentPage != pageServices {
		return
	}
	position := w.serviceTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredServices) {
		if w.selectedService != "" {
			w.resetWorkspaceForBrowserChange()
			w.selectedService = ""
			w.setBreadcrumb("ECS / " + w.selectedCluster)
			w.setDetail(clusterSummary(w.selectedCluster, len(w.allServices)), detailClusterSummary)
			w.updateActionSensitivity()
		}
		return
	}
	svc := w.filteredServices[position]
	if w.selectedService != svc.Name {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedService = svc.Name
	w.setBreadcrumb("ECS / " + w.selectedCluster + " / " + svc.Name)
	w.renderServiceOverview(svc)
	w.updateActionSensitivity()
}

func (w *mainWindow) openTaskAt(position uint) {
	w.openTaskFromFiltered(position)
}

func (w *mainWindow) openStoppedTaskAt(position uint) {
	w.openTaskFromFiltered(position)
}

func (w *mainWindow) selectTaskRow(stopped bool) {
	if w.currentPage != pageTasks && w.currentPage != pageStandaloneTasks {
		return
	}
	if stopped != w.showingStoppedTasks {
		return
	}
	table := w.taskTable
	if stopped {
		table = w.stoppedTaskTable
	}
	position := table.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredTasks) {
		if w.selectedTask != "" {
			w.resetWorkspaceForBrowserChange()
			w.selectedTask = ""
			w.setBreadcrumb(w.taskBrowserBreadcrumb())
			w.renderTaskBrowserContext()
			w.updateActionSensitivity()
		}
		return
	}
	task := w.filteredTasks[position]
	if w.selectedTask != task.TaskARN {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedTask = task.TaskARN
	w.setBreadcrumb(w.taskBrowserBreadcrumb() + " / " + shortID(task.TaskID))
	w.renderTaskDetail(task)
	w.updateActionSensitivity()
}

func (w *mainWindow) taskBrowserBreadcrumb() string {
	if w.currentPage == pageStandaloneTasks {
		return w.standaloneTaskBreadcrumb()
	}
	return w.serviceTaskBreadcrumb()
}

func (w *mainWindow) renderTaskBrowserContext() {
	if w.currentPage == pageStandaloneTasks {
		w.setDetail(w.standaloneTaskSummary(), detailClusterSummary)
		return
	}
	if svc, found := findService(w.allServices, w.selectedService); found {
		w.renderServiceTaskSummary(svc, "")
		return
	}
	w.setDetail("The selected service is no longer available.", detailError)
}

func (w *mainWindow) openTaskFromFiltered(position uint) {
	if int(position) >= len(w.filteredTasks) {
		return
	}
	task := w.filteredTasks[position]
	if w.selectedTask != task.TaskARN {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedTask = task.TaskARN
	w.updateActionSensitivity()
	if w.currentPage == pageStandaloneTasks {
		w.setBreadcrumb(w.standaloneTaskBreadcrumb() + " / " + shortID(task.TaskID))
	} else {
		w.setBreadcrumb(w.serviceTaskBreadcrumb() + " / " + shortID(task.TaskID))
	}
	w.renderTaskDetail(task)
}

func (w *mainWindow) openClusterByName(name string) {
	for _, cluster := range w.allClusters {
		if cluster.Name == name {
			w.loadServices(cluster.Name)
			return
		}
	}
	w.setStatus(fmt.Sprintf("Configured cluster %q was not found", name), true)
}

func (w *mainWindow) applyFilter() {
	if w.currentPage == pageModulePicker {
		return
	}
	if w.currentPage == pageSQSQueues {
		w.applySQSQueueFilter()
		return
	}
	if w.currentPage == pageSQSMessages {
		w.applySQSMessageFilter()
		return
	}
	if w.currentPage == pageRoute53Zones {
		w.applyRoute53ZoneFilter()
		return
	}
	if w.currentPage == pageS3Buckets {
		w.applyS3BucketFilter()
		return
	}
	if w.currentPage == pageS3Objects {
		w.applyS3ObjectFilter()
		return
	}
	if w.currentPage == pageDynamoTables {
		w.applyDynamoTableFilter()
		return
	}
	if w.currentPage == pageDynamoItems {
		w.applyDynamoItemFilter()
		return
	}
	if w.currentPage == pageECRRepositories {
		w.applyECRRepositoryFilter()
		return
	}
	if w.currentPage == pageECRImages {
		w.applyECRImageFilter()
		return
	}
	if w.currentPage == pageECRFindings {
		w.applyECRFindingFilter()
		return
	}
	if w.currentPage == pageRDSInstances {
		w.applyRDSFilter()
		return
	}
	if w.currentPage == pageRDSClusters {
		w.applyRDSClusterFilter()
		return
	}
	if w.currentPage == pageLambda {
		w.applyLambdaFilter()
		return
	}
	if w.currentPage == pageCodeBuildProjects {
		w.applyCodeBuildProjectFilter()
		return
	}
	if w.currentPage == pageCodeBuildBuilds {
		w.applyCodeBuildBuildFilter()
		return
	}
	if w.currentPage == pageEC2Instances {
		w.applyEC2Filter()
		return
	}
	if w.currentPage == pageEC2LoadBalancers {
		w.applyEC2LoadBalancerFilter()
		return
	}
	if w.currentPage == pageEC2TargetGroups {
		w.applyEC2TargetGroupFilter()
		return
	}
	if w.currentPage == pageEC2SecurityGroups {
		w.applyEC2SecurityGroupFilter()
		return
	}
	if w.currentPage == pageEC2VPCs {
		w.applyEC2VPCFilter()
		return
	}
	if w.currentPage == pageEC2Subnets {
		w.applyEC2SubnetFilter()
		return
	}
	if w.currentPage == pageEC2Volumes {
		w.applyEC2VolumeFilter()
		return
	}
	if w.currentPage == pageSecrets {
		w.applySecretFilter()
		return
	}
	if w.currentPage == pageSSM {
		w.applySSMFilter()
		return
	}
	if w.currentPage == pageAlarms {
		w.applyAlarmFilter()
		return
	}
	if w.currentPage == pageLogStreams {
		w.applyLogStreamFilter()
		return
	}
	if w.currentPage == pageLogGroups {
		w.applyLogGroupFilter()
		return
	}
	if w.currentPage == pageTaskDefinitions {
		w.applyTaskDefinitionFilter()
		return
	}
	if w.currentPage == pageTasks || w.currentPage == pageStandaloneTasks {
		w.applyTaskFilter()
		return
	}
	if w.currentPage == pageServices {
		w.applyServiceFilter()
		return
	}
	w.applyClusterFilter()
}

func (w *mainWindow) applyTaskFilter() {
	w.filteredTasks = filterTasks(w.allTasks, w.search.Text())
	if w.showingStoppedTasks {
		rows := make([]string, len(w.filteredTasks))
		for i, task := range w.filteredTasks {
			rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s", task.TaskID,
				formatTime(task.StoppedAt), taskExitSummary(task), valueOrDash(task.StopCode),
				valueOrDash(task.TaskDefinition), valueOrDash(task.StoppedReason))
		}
		w.stoppedTaskTable.replace(rows)
		return
	}
	rows := make([]string, len(w.filteredTasks))
	for i, task := range w.filteredTasks {
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s", task.TaskID,
			strings.ToUpper(valueOrDash(task.HealthStatus)), valueOrDash(task.Status),
			valueOrDash(task.Group), valueOrDash(task.AvailabilityZone), valueOrDash(task.PrivateIP),
			valueOrDash(task.TaskDefinition))
	}
	w.taskTable.replace(rows)
}

func (w *mainWindow) applyClusterFilter() {
	w.filteredClusters = filterClusters(w.allClusters, w.search.Text())
	rows := make([]string, len(w.filteredClusters))
	for i, cluster := range w.filteredClusters {
		rows[i] = fmt.Sprintf("%s\t%s\t%d\t%d\t%d", cluster.Name, cluster.Status,
			cluster.ActiveServices, cluster.RunningTasks, cluster.PendingTasks)
	}
	w.clusterTable.replace(rows)
}

func (w *mainWindow) applyServiceFilter() {
	w.filteredServices = filterServices(w.allServices, w.search.Text())
	rows := make([]string, len(w.filteredServices))
	for i, service := range w.filteredServices {
		health := strings.ToUpper(valueOrDash(service.HealthStatus))
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%d/%d\t%d\t%s", service.Name, health,
			service.Status, service.RunningCount, service.DesiredCount, service.PendingCount,
			service.TaskDefinition)
	}
	w.serviceTable.replace(rows)
}

func (w *mainWindow) goBack() {
	if w.showingTerminal {
		w.closeTerminal()
		return
	}
	if w.showingEditor {
		w.closeEditor()
		return
	}
	if w.showingLogs {
		w.closeLogs()
		return
	}
	if w.showingMetrics {
		w.closeMetrics()
		return
	}
	w.navigateBrowserBack()
}

func (w *mainWindow) navigateBrowserBack() {
	if w.showingEditor {
		w.closeEditorThen(w.navigateBrowserBack)
		return
	}
	if w.navigateResourceHistoryBack() {
		return
	}
	if w.navigateS3Back() {
		return
	}
	if w.currentPage == pageSQSMessages {
		w.restoreSQSQueueBrowser()
		return
	}
	if w.currentPage == pageDynamoItems {
		w.loadDynamoTables("", "")
		return
	}
	if w.currentPage == pageRDSInstances && w.rdsClusterContext != "" {
		clusterID := w.rdsClusterContext
		w.loadRDSClusters(clusterID)
		return
	}
	if w.currentPage == pageECRFindings {
		digest := w.selectedECRImage
		w.resetWorkspaceForBrowserChange()
		w.clearECRFindings()
		w.currentPage = pageECRImages
		w.search.SetText("")
		w.search.SetPlaceholderText("Filter images…")
		w.resourceStack.SetVisibleChildName(pageECRImages)
		w.backButton.SetSensitive(true)
		w.applyECRImageFilter()
		if image, found := findECRImage(w.allECRImages, digest); found {
			w.selectedECRImage = digest
			w.setBreadcrumb("ECR / " + w.selectedECRRepository + " / " + ecrImageLabel(image))
			w.setDetail(formatECRImage(w.selectedECRRepository, image), detailECRImage)
			for index, candidate := range w.filteredECRImages {
				if candidate.Digest == digest {
					w.ecrImageTable.selection.SetSelected(uint(index))
					break
				}
			}
		} else {
			w.selectedECRImage = ""
			w.setBreadcrumb("ECR / " + w.selectedECRRepository + " / Images")
			w.setDetail(ecrImageListSummary(w.selectedECRRepository, len(w.allECRImages)), detailECRRepository)
		}
		w.updateActionSensitivity()
		w.setStatus("Ready", false)
		return
	}
	if w.currentPage == pageECRImages {
		repositoryName := w.selectedECRRepository
		w.resetWorkspaceForBrowserChange()
		w.clearECRImages()
		w.clearECRFindings()
		w.currentPage = pageECRRepositories
		w.search.SetText("")
		w.search.SetPlaceholderText("Filter repositories…")
		w.resourceStack.SetVisibleChildName(pageECRRepositories)
		w.backButton.SetSensitive(false)
		w.applyECRRepositoryFilter()
		if repository, found := findECRRepository(w.allECRRepositories, repositoryName); found {
			w.selectedECRRepository = repositoryName
			w.setBreadcrumb("ECR / Repositories / " + repositoryName)
			w.setDetail(formatECRRepository(repository), detailECRRepository)
			for index, candidate := range w.filteredECRRepositories {
				if candidate.Name == repositoryName {
					w.ecrRepositoryTable.selection.SetSelected(uint(index))
					break
				}
			}
		} else {
			w.selectedECRRepository = ""
			w.setBreadcrumb("ECR / Repositories")
			w.setDetail(ecrRepositoryListSummary(len(w.allECRRepositories)), detailIntro)
		}
		w.updateActionSensitivity()
		w.setStatus("Ready", false)
		return
	}
	if w.currentPage == pageCodeBuildBuilds {
		projectName := w.selectedCodeBuildProject
		w.resetWorkspaceForBrowserChange()
		w.clearCodeBuildBuilds()
		w.currentPage = pageCodeBuildProjects
		w.search.SetText("")
		w.search.SetPlaceholderText("Filter projects…")
		w.resourceStack.SetVisibleChildName(pageCodeBuildProjects)
		w.backButton.SetSensitive(false)
		w.selectedCodeBuildProject = projectName
		if project, found := findCodeBuildProject(w.allCodeBuildProjects, projectName); found {
			w.setBreadcrumb("CodeBuild / Projects / " + projectName)
			w.setDetail(formatCodeBuildProject(project), detailIntro)
		} else {
			w.selectedCodeBuildProject = ""
			w.setBreadcrumb("CodeBuild / Projects")
			w.setDetail(codeBuildProjectListSummary(len(w.allCodeBuildProjects)), detailIntro)
		}
		w.applyCodeBuildProjectFilter()
		w.updateActionSensitivity()
		w.setStatus("Ready", false)
		return
	}
	if w.currentPage == pageLogStreams {
		w.resetWorkspaceForBrowserChange()
		w.currentPage = pageLogGroups
		w.selectedLogStream = ""
		w.updateActionSensitivity()
		w.search.SetText("")
		w.search.SetPlaceholderText("Filter log groups…")
		w.resourceStack.SetVisibleChildName(pageLogGroups)
		w.applyLogGroupFilter()
		w.backButton.SetSensitive(false)
		if group, found := findLogGroup(w.allLogGroups, w.selectedLogGroup); found {
			w.setBreadcrumb("CloudWatch Logs / Log groups / " + group.Name)
			w.setDetail(formatLogGroupDetail(group), detailLogGroup)
		} else {
			w.selectedLogGroup = ""
			w.setBreadcrumb("CloudWatch Logs / Log groups")
			w.setDetail(logGroupsSummary(len(w.allLogGroups)), detailIntro)
		}
		w.setStatus("Ready", false)
		return
	}
	if w.currentPage == pageTasks || w.currentPage == pageStandaloneTasks {
		serviceName := w.selectedService
		w.resetWorkspaceForBrowserChange()
		w.clearTaskBrowser()
		w.currentPage = pageServices
		w.selectedService = serviceName
		w.search.SetText("")
		w.search.SetPlaceholderText("Filter services…")
		w.resourceStack.SetVisibleChildName(pageServices)
		w.applyServiceFilter()
		if svc, found := findService(w.allServices, serviceName); found {
			w.setBreadcrumb("ECS / " + w.selectedCluster + " / " + serviceName)
			w.renderServiceOverview(svc)
			for index, candidate := range w.filteredServices {
				if candidate.Name == serviceName {
					w.serviceTable.selection.SetSelected(uint(index))
					break
				}
			}
		} else {
			w.selectedService = ""
			w.setBreadcrumb("ECS / " + w.selectedCluster)
			w.setDetail(clusterSummary(w.selectedCluster, len(w.allServices)), detailClusterSummary)
		}
		w.updateActionSensitivity()
		w.setStatus("Ready", false)
		return
	}
	if w.currentPage != pageServices {
		return
	}
	w.resetWorkspaceForBrowserChange()
	clusterName := w.selectedCluster
	w.currentPage = pageClusters
	w.selectedCluster = clusterName
	w.selectedService = ""
	w.selectedTask = ""
	w.allServices = nil
	w.search.SetText("")
	w.search.SetPlaceholderText("Filter clusters…")
	w.resourceStack.SetVisibleChildName(pageClusters)
	w.backButton.SetSensitive(false)
	w.applyClusterFilter()
	if cluster, found := findCluster(w.allClusters, clusterName); found {
		w.setBreadcrumb("ECS / Clusters / " + clusterName)
		w.setDetail(formatClusterDetail(cluster), detailClusterSummary)
		for index, candidate := range w.filteredClusters {
			if candidate.Name == clusterName {
				w.clusterTable.selection.SetSelected(uint(index))
				break
			}
		}
	} else {
		w.selectedCluster = ""
		w.setBreadcrumb("ECS / Clusters")
		w.setDetail(clusterListSummary(len(w.allClusters)), detailIntro)
	}
	w.updateActionSensitivity()
	w.setStatus("Ready", false)
}

func (w *mainWindow) refresh() {
	w.refreshCurrent(true)
}

func (w *mainWindow) refreshCurrent(foreground bool) {
	if w.currentPage == pageModulePicker {
		if foreground {
			w.showModulePicker()
		}
		return
	}
	if w.currentPage == pageSQSQueues {
		w.refreshSQSQueues(foreground)
		return
	}
	if w.currentPage == pageSQSMessages {
		w.refreshSQSMessages(foreground)
		return
	}
	if w.currentPage == pageRoute53Zones {
		w.refreshRoute53Zones(foreground)
		return
	}
	if w.currentPage == pageECRRepositories || w.currentPage == pageECRImages || w.currentPage == pageECRFindings {
		if w.ecrActionPending {
			if foreground {
				w.setStatus("An ECR operation is still pending", false)
			}
			return
		}
		w.refreshECR(foreground)
		return
	}
	if w.currentPage == pageSecrets && w.secretActionPending {
		if foreground {
			w.setStatus("A Secrets Manager operation is still pending", false)
		}
		return
	}
	if w.currentPage == pageLambda && w.lambdaActionPending {
		if foreground {
			w.setStatus("A Lambda operation is still pending", false)
		}
		return
	}
	if (w.currentPage == pageCodeBuildProjects || w.currentPage == pageCodeBuildBuilds) && w.codeBuildActionPending {
		if foreground {
			w.setStatus("A CodeBuild operation is still pending", false)
		}
		return
	}
	if w.currentPage == pageEC2Instances && w.ec2ActionPending {
		if foreground {
			w.setStatus("An EC2 operation is still pending", false)
		}
		return
	}
	if w.showingTerminal {
		if foreground {
			w.setStatus("Disconnect the terminal session before refreshing", false)
		}
		return
	}
	if w.showingEditor {
		if foreground {
			w.setStatus("Editor has unsaved content; close it before refreshing", false)
		}
		return
	}
	if w.showingLogs {
		if foreground {
			w.setStatus("The log view updates independently; use its toolbar controls", false)
		}
		return
	}
	if w.showingMetrics {
		w.loadMetrics(foreground)
		return
	}
	cloudWatchPage := w.currentPage == pageLogGroups || w.currentPage == pageLogStreams || w.currentPage == pageSavedLogSearch
	if foreground && cloudWatchPage && !w.reloadSavedLogConfig() {
		return
	}
	if w.currentPage == pageSavedLogSearch {
		if path, found := w.activeSavedLogPath(); found {
			w.openSavedLog(path)
		}
		return
	}
	if w.currentPage == pageLambda {
		if foreground && !w.reloadLambdaSearchConfig() {
			return
		}
		w.refreshLambda(foreground)
		return
	}
	if w.currentPage == pageCodeBuildProjects || w.currentPage == pageCodeBuildBuilds {
		w.refreshCodeBuild(foreground)
		return
	}
	if w.currentPage == pageEC2Instances {
		w.refreshEC2(foreground)
		return
	}
	if w.currentPage == pageRDSInstances {
		w.refreshRDS(foreground)
		return
	}
	if w.currentPage == pageRDSClusters {
		w.refreshRDSClusters(foreground)
		return
	}
	if w.currentPage == pageS3Buckets {
		if foreground && !w.reloadS3SearchConfig() {
			return
		}
		w.refreshS3Buckets(foreground)
		return
	}
	if w.currentPage == pageS3Objects {
		w.refreshS3Objects(foreground)
		return
	}
	if w.currentPage == pageDynamoTables {
		if foreground {
			w.reloadDynamoConfig()
		}
		w.refreshDynamoTables(foreground)
		return
	}
	if w.currentPage == pageDynamoItems {
		w.refreshDynamoItems(foreground)
		return
	}
	if w.currentPage == pageEC2LoadBalancers {
		w.refreshEC2LoadBalancers(foreground)
		return
	}
	if w.currentPage == pageEC2TargetGroups {
		w.refreshEC2TargetGroups(foreground)
		return
	}
	if w.currentPage == pageEC2SecurityGroups {
		w.refreshEC2SecurityGroups(foreground)
		return
	}
	if w.currentPage == pageEC2VPCs {
		w.refreshEC2VPCs(foreground)
		return
	}
	if w.currentPage == pageEC2Subnets {
		w.refreshEC2Subnets(foreground)
		return
	}
	if w.currentPage == pageEC2Volumes {
		w.refreshEC2Volumes(foreground)
		return
	}
	if w.currentPage == pageSecrets {
		if foreground && !w.reloadSecretFilterConfig() {
			return
		}
		w.refreshSecrets(foreground)
		return
	}
	if w.currentPage == pageSSM {
		if foreground && !w.reloadSSMPrefixConfig() {
			return
		}
		w.refreshSSM(foreground)
		return
	}
	if w.currentPage == pageAlarms {
		w.refreshAlarms(foreground)
		return
	}
	if w.currentPage == pageLogStreams {
		w.refreshLogStreams(foreground)
		return
	}
	if w.currentPage == pageLogGroups {
		w.refreshLogGroups(foreground)
		return
	}
	if w.currentPage == pageTaskDefinitions {
		w.refreshTaskDefinitions(foreground)
		return
	}
	if w.currentPage == pageStandaloneTasks && w.selectedCluster != "" {
		w.refreshStandaloneTasks(foreground)
		return
	}
	if w.currentPage == pageTasks && w.selectedCluster != "" && w.selectedService != "" {
		w.refreshTasks(foreground)
		return
	}
	if w.currentPage == pageServices && w.selectedCluster != "" {
		w.refreshServices(foreground)
		return
	}
	w.refreshClusters(foreground)
}

func (w *mainWindow) refreshStandaloneTasks(foreground bool) {
	cluster, taskARN, stopped := w.selectedCluster, w.selectedTask, w.showingStoppedTasks
	loaded := len(w.allTasks)
	if loaded < taskHistoryBatchSize {
		loaded = taskHistoryBatchSize
	}
	label := "Refreshing active standalone tasks in " + cluster + "…"
	if stopped {
		label = "Refreshing recently stopped tasks in " + cluster + "…"
	}
	ctx, generation := w.startRefreshRequest(label, foreground)
	go func() {
		var (
			tasks     []model.Task
			nextToken string
			err       error
		)
		if stopped {
			page, pageErr := w.options.ECS.ListStoppedStandaloneTasks(ctx, cluster, "", loaded)
			tasks, nextToken, err = page.Tasks, page.NextToken, pageErr
			sortStoppedTasks(tasks)
		} else {
			tasks, err = w.options.ECS.ListStandaloneTasks(ctx, cluster)
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allTasks = tasks
			w.taskNextToken = nextToken
			w.applyTaskFilter()
			w.updateActionSensitivity()
			if taskARN == "" {
				if w.detailContent == detailClusterSummary {
					w.setDetail(w.standaloneTaskSummary(), detailClusterSummary)
				}
				return
			}
			task, found := findTask(tasks, taskARN)
			if !found {
				w.selectedTask = ""
				w.updateActionSensitivity()
				w.setBreadcrumb(w.standaloneTaskBreadcrumb())
				if w.detailContent == detailTask {
					w.setDetail("The selected task is no longer available.\n\n"+w.standaloneTaskSummary(), detailClusterSummary)
				}
				return
			}
			if w.detailContent == detailTask {
				w.renderTaskDetail(task)
			}
		})
	}()
}

func (w *mainWindow) refreshTasks(foreground bool) {
	cluster, serviceName, taskARN, stopped := w.selectedCluster, w.selectedService, w.selectedTask, w.showingStoppedTasks
	loaded := len(w.allTasks)
	if loaded < taskHistoryBatchSize {
		loaded = taskHistoryBatchSize
	}
	label := "Refreshing active tasks for " + serviceName + "…"
	if stopped {
		label = "Refreshing recently stopped tasks for " + serviceName + "…"
	}
	ctx, generation := w.startRefreshRequest(label, foreground)
	go func() {
		services, err := w.options.ECS.ListServices(ctx, cluster)
		service, serviceFound := findService(services, serviceName)
		var (
			tasks     []model.Task
			nextToken string
		)
		if err == nil && serviceFound {
			if stopped {
				page, pageErr := w.options.ECS.ListStoppedServiceTasks(ctx, cluster, serviceName, "", loaded)
				tasks, nextToken, err = page.Tasks, page.NextToken, pageErr
				sortStoppedTasks(tasks)
			} else {
				tasks, err = w.options.ECS.ListTasks(ctx, cluster, serviceName)
			}
		}

		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allServices = services
			w.allTasks = tasks
			w.taskNextToken = nextToken
			w.applyTaskFilter()
			w.updateActionSensitivity()
			if !serviceFound {
				w.navigateBrowserBack()
				w.setStatus("The selected service is no longer available", true)
				return
			}

			if taskARN == "" {
				if w.detailContent == detailService {
					w.renderServiceTaskSummary(service, "")
				}
				return
			}
			task, found := findTask(tasks, taskARN)
			if !found {
				w.selectedTask = ""
				w.updateActionSensitivity()
				w.setBreadcrumb(w.serviceTaskBreadcrumb())
				if w.detailContent == detailTask {
					w.renderServiceTaskSummary(service, "The selected task is no longer available.")
				}
				return
			}
			if w.detailContent == detailTask {
				w.renderTaskDetail(task)
			}
		})
	}()
}

func (w *mainWindow) refreshClusters(foreground bool) {
	selectedName := w.selectedCluster
	ctx, generation := w.startRefreshRequest("Refreshing ECS clusters…", foreground)
	go func() {
		clusters, err := w.options.ECS.ListClusters(ctx)
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allClusters = clusters
			w.applyClusterFilter()
			if selectedName != "" {
				if cluster, found := findCluster(clusters, selectedName); found {
					w.selectedCluster = selectedName
					w.setBreadcrumb("ECS / Clusters / " + selectedName)
					w.setDetail(formatClusterDetail(cluster), detailClusterSummary)
					return
				}
				w.selectedCluster = ""
				w.setBreadcrumb("ECS / Clusters")
			}
			if len(clusters) == 0 {
				w.setDetail("No ECS clusters found in this region.", detailIntro)
			} else {
				w.setDetail(clusterListSummary(len(clusters)), detailIntro)
			}
		})
	}()
}

func (w *mainWindow) refreshServices(foreground bool) {
	cluster, selectedName := w.selectedCluster, w.selectedService
	ctx, generation := w.startRefreshRequest("Refreshing services in "+cluster+"…", foreground)
	go func() {
		services, err := w.options.ECS.ListServices(ctx, cluster)
		var (
			selected model.Service
			found    bool
		)
		if err == nil && selectedName != "" {
			selected, found = findService(services, selectedName)
		}

		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allServices = services
			w.applyServiceFilter()

			if selectedName == "" {
				if w.detailContent == detailClusterSummary {
					w.setDetail(clusterSummary(cluster, len(services)), detailClusterSummary)
				}
				return
			}
			if !found {
				w.selectedService = ""
				w.updateActionSensitivity()
				w.setBreadcrumb("ECS / " + cluster)
				if w.detailContent == detailService {
					w.setDetail("The selected service is no longer available.\n\n"+clusterSummary(cluster, len(services)), detailClusterSummary)
				}
				return
			}
			if w.detailContent == detailService {
				w.renderServiceOverview(selected)
			}
		})
	}()
}

func (w *mainWindow) scheduleRefresh() {
	glib.IdleAdd(func() {
		if w.ctx.Err() == nil && !w.workspaceBusy {
			w.refreshCurrent(false)
		}
	})
}

func (w *mainWindow) autoRefresh() {
	interval := w.options.RefreshInterval
	if interval <= 0 {
		interval = 5
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			w.scheduleRefresh()
		}
	}
}

func (w *mainWindow) setStatus(message string, isError bool) {
	tooltip := message
	if isError {
		w.lastError = message
		message = "Error: " + compactStatusMessage(message)
	} else {
		w.lastError = ""
	}
	w.status.SetLabel(message)
	w.status.SetTooltipText(tooltip)
	w.status.RemoveCSSClass("error")
	if isError {
		w.status.AddCSSClass("error")
	}
	if w.statusDetailsButton != nil {
		w.statusDetailsButton.SetVisible(isError)
		w.statusDismissButton.SetVisible(isError)
	}
	w.updateModuleErrorGlyph(isError)
}

func (w *mainWindow) updateActionSensitivity() {
	serviceSelected := w.currentPage == pageTasks && w.selectedCluster != "" && w.selectedService != ""
	standalonePage := w.currentPage == pageStandaloneTasks && w.selectedCluster != ""
	taskSelected := (serviceSelected || standalonePage) && w.selectedTask != ""
	taskRunning := false
	taskStopped := false
	if taskSelected {
		if task, found := findTask(w.allTasks, w.selectedTask); found {
			taskRunning = task.Status == "RUNNING"
			taskStopped = task.Status == "STOPPED"
		}
	}
	clusterBrowserPage := w.currentPage == pageServices || w.currentPage == pageTasks || standalonePage
	w.standaloneButton.SetVisible(clusterBrowserPage)
	w.standaloneButton.SetSensitive(clusterBrowserPage && w.selectedCluster != "")
	if standalonePage {
		if w.standaloneReturnPage == pageTasks && w.standaloneReturnService != "" {
			w.standaloneButton.SetLabel("Service tasks")
		} else {
			w.standaloneButton.SetLabel("Services")
		}
	} else {
		w.standaloneButton.SetLabel("Standalone")
	}
	if w.clustersNavButton != nil {
		w.clustersNavButton.SetActive(isECSPage(w.currentPage) && w.currentPage != pageTaskDefinitions)
		w.taskDefinitionsNavButton.SetActive(w.currentPage == pageTaskDefinitions)
		w.logGroupsNavButton.SetActive(w.activeSavedLog == "" && (w.currentPage == pageLogGroups || w.currentPage == pageLogStreams))
		cloudWatchPage := w.currentPage == pageLogGroups || w.currentPage == pageLogStreams || w.currentPage == pageSavedLogSearch
		for i, path := range w.options.ConfigLogPaths() {
			if i < len(w.savedLogNavButtons) {
				w.savedLogNavButtons[i].SetActive(cloudWatchPage && path.Name == w.activeSavedLog)
			}
		}
		for state, button := range w.alarmNavButtons {
			button.SetActive(w.currentPage == pageAlarms && state == w.alarmStateFilter)
		}
		if w.ssmParametersNavButton != nil {
			w.ssmParametersNavButton.SetActive(w.currentPage == pageSSM && w.activeSSMPrefix == "")
			for i, prefix := range w.options.ConfigSSMPrefixes() {
				if i < len(w.savedSSMPrefixButtons) {
					w.savedSSMPrefixButtons[i].SetActive(w.currentPage == pageSSM && prefix.Name == w.activeSSMPrefix)
				}
			}
		}
		if w.secretsNavButton != nil {
			w.secretsNavButton.SetActive(w.currentPage == pageSecrets && w.activeSavedSecretFilter == "")
			for i, filter := range w.options.ConfigSecretFilters() {
				if i < len(w.savedSecretFilterButtons) {
					w.savedSecretFilterButtons[i].SetActive(w.currentPage == pageSecrets && filter.Name == w.activeSavedSecretFilter)
				}
			}
		}
		if w.lambdaFunctionsNavButton != nil {
			w.lambdaFunctionsNavButton.SetActive(w.currentPage == pageLambda && w.activeSavedLambdaSearch == "")
			for i, search := range w.options.ConfigLambdaSearches() {
				if i < len(w.savedLambdaSearchButtons) {
					w.savedLambdaSearchButtons[i].SetActive(w.currentPage == pageLambda && search.Name == w.activeSavedLambdaSearch)
				}
			}
		}
		if w.codeBuildProjectsNavButton != nil {
			w.codeBuildProjectsNavButton.SetActive(w.currentPage == pageCodeBuildProjects || w.currentPage == pageCodeBuildBuilds)
		}
		if w.ec2InstancesNavButton != nil {
			w.ec2InstancesNavButton.SetActive(w.currentPage == pageEC2Instances)
			w.ec2LoadBalancersNavButton.SetActive(w.currentPage == pageEC2LoadBalancers)
			w.ec2TargetGroupsNavButton.SetActive(w.currentPage == pageEC2TargetGroups)
			w.ec2SecurityGroupsNavButton.SetActive(w.currentPage == pageEC2SecurityGroups)
			w.ec2VPCsNavButton.SetActive(w.currentPage == pageEC2VPCs)
			w.ec2SubnetsNavButton.SetActive(w.currentPage == pageEC2Subnets)
			w.ec2VolumesNavButton.SetActive(w.currentPage == pageEC2Volumes)
		}
		if w.ecrRepositoriesNavButton != nil {
			w.ecrRepositoriesNavButton.SetActive(w.currentPage == pageECRRepositories || w.currentPage == pageECRImages || w.currentPage == pageECRFindings)
		}
		if w.rdsInstancesNavButton != nil {
			w.rdsInstancesNavButton.SetActive(w.currentPage == pageRDSInstances)
			w.rdsClustersNavButton.SetActive(w.currentPage == pageRDSClusters)
		}
		if w.s3BucketsNavButton != nil {
			w.s3BucketsNavButton.SetActive((w.currentPage == pageS3Buckets || w.currentPage == pageS3Objects) && w.activeSavedS3Search == "")
			for i, search := range w.options.ConfigS3Searches() {
				if i < len(w.savedS3SearchButtons) {
					w.savedS3SearchButtons[i].SetActive((w.currentPage == pageS3Buckets || w.currentPage == pageS3Objects) && search.Name == w.activeSavedS3Search)
				}
			}
		}
		if w.dynamoTablesNavButton != nil {
			dynamoPage := w.currentPage == pageDynamoTables || w.currentPage == pageDynamoItems
			w.dynamoTablesNavButton.SetActive(dynamoPage && w.activeSavedDynamoTable == "" && w.activeSavedDynamoQuery == "")
			for i, saved := range w.options.ConfigDynamoTables() {
				if i < len(w.savedDynamoTableButtons) {
					w.savedDynamoTableButtons[i].SetActive(dynamoPage && saved.Name == w.activeSavedDynamoTable)
				}
			}
			for i, saved := range w.options.ConfigDynamoQueries() {
				if i < len(w.savedDynamoQueryButtons) {
					w.savedDynamoQueryButtons[i].SetActive(dynamoPage && saved.Name == w.activeSavedDynamoQuery)
				}
			}
		}
		if w.sqsQueuesNavButton != nil {
			sqsPage := w.currentPage == pageSQSQueues || w.currentPage == pageSQSMessages
			w.sqsQueuesNavButton.SetActive(sqsPage && w.activeSavedSQSQueue == "")
			for i, saved := range w.options.ConfigSQSQueues() {
				if i < len(w.savedSQSQueueButtons) {
					w.savedSQSQueueButtons[i].SetActive(sqsPage && saved.Name == w.activeSavedSQSQueue)
				}
			}
		}
		if w.route53ZonesNavButton != nil {
			w.route53ZonesNavButton.SetActive(w.currentPage == pageRoute53Zones || w.currentPage == pageRoute53Records)
		}
	}
	w.runTaskButton.SetVisible(standalonePage)
	w.runTaskButton.SetSensitive(standalonePage)
	if w.taskScopeBar != nil {
		taskBrowser := serviceSelected || standalonePage
		w.taskScopeBar.SetVisible(taskBrowser)
		w.loadMoreTasksButton.SetVisible(taskBrowser && w.showingStoppedTasks && w.taskNextToken != "")
		w.loadMoreTasksButton.SetSensitive(w.taskNextToken != "")
	}
	ec2MetricsSelected := w.currentPage == pageEC2Instances && w.selectedEC2Instance != "" && w.ec2Detail != nil
	rdsMetricsSelected := w.currentPage == pageRDSInstances && w.selectedRDSInstance != "" && w.rdsDetail != nil
	rdsClusterMetricsSelected := w.currentPage == pageRDSClusters && w.selectedRDSCluster != "" && w.rdsClusterDetail != nil
	w.metricsButton.SetVisible(serviceSelected || taskSelected || ec2MetricsSelected || rdsMetricsSelected || rdsClusterMetricsSelected)
	w.metricsButton.SetSensitive(serviceSelected || taskSelected || (ec2MetricsSelected && w.options.EC2 != nil) || ((rdsMetricsSelected || rdsClusterMetricsSelected) && w.options.RDS != nil))
	execEnabled := false
	if taskSelected && vteAvailable() {
		if task, found := findTask(w.allTasks, w.selectedTask); found {
			execEnabled = task.Status == "RUNNING" && task.ExecAgentRunning
			if serviceSelected {
				if service, serviceFound := findService(w.allServices, w.selectedService); serviceFound {
					execEnabled = execEnabled && service.EnableExecuteCommand
				}
			}
		}
	}
	w.execButton.SetVisible(taskSelected && taskRunning)
	w.execButton.SetSensitive(execEnabled)
	w.logsButton.SetVisible(serviceSelected)
	w.logsButton.SetSensitive(serviceSelected && w.options.Logs != nil)
	w.taskLogsButton.SetVisible(taskSelected)
	w.taskLogsButton.SetSensitive(taskSelected && w.options.Logs != nil)
	logGroupSelected := (w.currentPage == pageLogGroups || w.currentPage == pageLogStreams || w.currentPage == pageSavedLogSearch) && w.selectedLogGroup != ""
	logStreamSelected := w.currentPage == pageLogStreams && w.selectedLogStream != ""
	w.peekLogStreamButton.SetVisible(logStreamSelected)
	w.peekLogStreamButton.SetSensitive(logStreamSelected && w.options.Logs != nil)
	w.followLogStreamButton.SetVisible(logStreamSelected)
	w.followLogStreamButton.SetSensitive(logStreamSelected && w.options.Logs != nil)
	w.followLogGroupButton.SetVisible(logGroupSelected)
	w.followLogGroupButton.SetSensitive(logGroupSelected && w.options.Logs != nil)
	cloudWatchBrowser := w.currentPage == pageLogGroups || w.currentPage == pageLogStreams || w.currentPage == pageSavedLogSearch
	w.searchLogsButton.SetVisible(cloudWatchBrowser)
	w.searchLogsButton.SetSensitive(cloudWatchBrowser && logGroupSelected && w.options.Logs != nil)
	canSaveDestination := w.activeSavedLog == "" && (w.currentPage == pageLogGroups || w.currentPage == pageLogStreams) && logGroupSelected
	w.saveLogDestinationButton.SetVisible(canSaveDestination)
	w.saveLogDestinationButton.SetSensitive(canSaveDestination && w.options.Config != nil)
	_, activeSavedExists := w.activeSavedLogPath()
	hasSavedWorkspace := w.showingLogs && w.activeSavedLog != "" && activeSavedExists
	canSaveWorkspace := w.showingLogs && (w.logSearchSpec != nil || hasSavedWorkspace)
	w.saveLogSearchButton.SetVisible(canSaveWorkspace)
	w.saveLogSearchButton.SetSensitive(canSaveWorkspace && w.options.Config != nil)
	dirtySavedWorkspace := hasSavedWorkspace && w.savedLogWorkspaceDirty()
	w.savedLogModifiedLabel.SetVisible(dirtySavedWorkspace)
	w.updateSavedLogButton.SetVisible(hasSavedWorkspace)
	w.updateSavedLogButton.SetSensitive(dirtySavedWorkspace && w.options.Config != nil)
	managingSaved := cloudWatchBrowser && w.options.Config != nil && len(w.options.Config.LogPaths) > 0
	w.manageSavedLogButton.SetVisible(managingSaved)
	w.manageSavedLogButton.SetSensitive(managingSaved)
	alarmSelected := w.currentPage == pageAlarms && w.selectedAlarm != ""
	alarmDetailReady := alarmSelected && w.alarmDetail != nil && w.alarmDetail.Name == w.selectedAlarm
	w.alarmActionsButton.SetVisible(alarmSelected)
	w.alarmActionsButton.SetSensitive(alarmDetailReady && !w.alarmActionPending && w.options.Alarms != nil)
	w.alarmActionsButton.RemoveCSSClass("destructive-action")
	w.alarmActionsButton.RemoveCSSClass("suggested-action")
	if alarmDetailReady && w.alarmDetail.ActionsEnabled {
		w.alarmActionsButton.SetLabel("Disable actions")
		w.alarmActionsButton.AddCSSClass("destructive-action")
	} else if alarmDetailReady {
		w.alarmActionsButton.SetLabel("Enable actions")
		w.alarmActionsButton.AddCSSClass("suggested-action")
	} else {
		w.alarmActionsButton.SetLabel("Alarm actions")
	}
	w.alarmSetStateButton.SetVisible(alarmSelected)
	w.alarmSetStateButton.SetSensitive(alarmDetailReady && !w.alarmActionPending && w.options.Alarms != nil)
	w.alarmTimestampButton.SetVisible(w.currentPage == pageAlarms)
	w.alarmTimestampButton.SetSensitive(w.currentPage == pageAlarms)
	ssmPage := w.currentPage == pageSSM
	w.ssmBrowsePathButton.SetVisible(ssmPage)
	w.ssmBrowsePathButton.SetSensitive(ssmPage && w.options.SSM != nil)
	w.ssmSavePrefixButton.SetVisible(ssmPage)
	w.ssmSavePrefixButton.SetSensitive(ssmPage && w.options.Config != nil && w.ssmPath != "")
	hasSavedSSMPrefixes := w.options.Config != nil && len(w.options.Config.SSMPrefixes) > 0
	w.ssmManagePrefixesButton.SetVisible(ssmPage && hasSavedSSMPrefixes)
	w.ssmManagePrefixesButton.SetSensitive(ssmPage && hasSavedSSMPrefixes)
	ssmSelected, ssmSelectedFound := findSSMParameter(w.allSSMParameters, w.selectedSSMParameter)
	w.ssmViewValueButton.SetVisible(ssmPage && ssmSelectedFound)
	w.ssmViewValueButton.SetSensitive(ssmPage && ssmSelectedFound && !w.ssmActionPending && w.options.SSM != nil)
	if ssmSelected.Type == "SecureString" {
		w.ssmViewValueButton.SetLabel("Reveal value…")
	} else {
		w.ssmViewValueButton.SetLabel("View value")
	}
	w.ssmEditButton.SetVisible(ssmPage && ssmSelectedFound)
	w.ssmEditButton.SetSensitive(ssmPage && ssmSelectedFound && !w.ssmActionPending && w.options.SSM != nil)
	secretsPage := w.currentPage == pageSecrets
	w.secretFilterButton.SetVisible(secretsPage)
	w.secretFilterButton.SetSensitive(secretsPage && !w.secretActionPending && w.options.Secrets != nil)
	w.secretSaveFilterButton.SetVisible(secretsPage)
	w.secretSaveFilterButton.SetSensitive(secretsPage && !w.secretActionPending && w.options.Config != nil)
	hasSavedSecretFilters := w.options.Config != nil && len(w.options.Config.SMFilters) > 0
	w.secretManageFiltersButton.SetVisible(secretsPage && hasSavedSecretFilters)
	w.secretManageFiltersButton.SetSensitive(secretsPage && !w.secretActionPending && hasSavedSecretFilters)
	_, secretSelected := findSecret(w.allSecrets, w.selectedSecret)
	secretActionReady := secretsPage && secretSelected && !w.secretActionPending && w.options.Secrets != nil
	textSecret := w.secretDetail == nil || w.secretDetail.Name != w.selectedSecret || !w.secretDetail.Binary
	w.secretRevealButton.SetVisible(secretsPage && secretSelected)
	w.secretRevealButton.SetSensitive(secretActionReady)
	w.secretEditButton.SetVisible(secretsPage && secretSelected)
	w.secretEditButton.SetSensitive(secretActionReady && textSecret)
	w.secretCloneButton.SetVisible(secretsPage && secretSelected)
	w.secretCloneButton.SetSensitive(secretActionReady && textSecret)
	if !textSecret {
		w.secretEditButton.SetTooltipText("Binary secret values cannot be edited as text")
		w.secretCloneButton.SetTooltipText("Binary secret values cannot be cloned as text")
	} else {
		w.secretEditButton.SetTooltipText("")
		w.secretCloneButton.SetTooltipText("")
	}
	w.secretCopyARNButton.SetVisible(secretsPage && secretSelected)
	w.secretCopyARNButton.SetSensitive(secretsPage && secretSelected)
	lambdaPage := w.currentPage == pageLambda
	w.lambdaSearchButton.SetVisible(lambdaPage)
	w.lambdaSearchButton.SetSensitive(lambdaPage && !w.showingEditor && w.options.Lambda != nil)
	w.lambdaSaveSearchButton.SetVisible(lambdaPage)
	w.lambdaSaveSearchButton.SetSensitive(lambdaPage && !w.showingEditor && w.options.Config != nil)
	hasSavedLambdaSearches := w.options.Config != nil && len(w.options.Config.LambdaSearches) > 0
	w.lambdaManageSearchesButton.SetVisible(lambdaPage && hasSavedLambdaSearches)
	w.lambdaManageSearchesButton.SetSensitive(lambdaPage && !w.showingEditor && hasSavedLambdaSearches)
	lambdaSelected := lambdaPage && w.selectedLambdaFunction != ""
	lambdaReady := lambdaSelected && w.lambdaDetail != nil && w.lambdaDetail.Name == w.selectedLambdaFunction && !w.lambdaActionPending && !w.showingEditor
	w.lambdaDetailsButton.SetVisible(lambdaSelected && w.lambdaViewMode == lambdaEnvironmentMode)
	w.lambdaDetailsButton.SetSensitive(lambdaReady)
	w.lambdaEnvironmentButton.SetVisible(lambdaSelected && w.lambdaViewMode != lambdaEnvironmentMode)
	w.lambdaEnvironmentButton.SetSensitive(lambdaReady && w.options.Lambda != nil)
	canRevealLambdaSecrets := lambdaReady && w.lambdaViewMode == lambdaEnvironmentMode && !w.lambdaEnvironmentResolved && environmentHasSecrets(w.lambdaEnvironment)
	w.lambdaRevealSecretsButton.SetVisible(canRevealLambdaSecrets)
	w.lambdaRevealSecretsButton.SetSensitive(canRevealLambdaSecrets)
	hasLambdaLogs := lambdaReady && w.lambdaDetail.LogGroup != "" && w.options.Logs != nil
	w.lambdaFollowLogsButton.SetVisible(lambdaSelected)
	w.lambdaFollowLogsButton.SetSensitive(hasLambdaLogs)
	w.lambdaBrowseLogsButton.SetVisible(lambdaSelected)
	w.lambdaBrowseLogsButton.SetSensitive(hasLambdaLogs)
	w.lambdaSearchLogsButton.SetVisible(lambdaSelected)
	w.lambdaSearchLogsButton.SetSensitive(hasLambdaLogs)
	zipLambda := lambdaReady && !strings.EqualFold(w.lambdaDetail.PackageType, "Image")
	w.lambdaEditCodeButton.SetVisible(lambdaSelected)
	w.lambdaEditCodeButton.SetSensitive(zipLambda && w.options.Lambda != nil)
	if lambdaReady && !zipLambda {
		w.lambdaEditCodeButton.SetTooltipText("Container-image functions cannot be edited as ZIP deployments")
	} else {
		w.lambdaEditCodeButton.SetTooltipText("")
	}
	codeBuildPage := w.currentPage == pageCodeBuildProjects || w.currentPage == pageCodeBuildBuilds
	codeBuildProject := w.currentCodeBuildProject()
	codeBuildDetailReady := codeBuildPage && w.codeBuildDetail != nil && w.codeBuildDetail.ID == w.selectedCodeBuild
	w.codeBuildStartButton.SetVisible(codeBuildPage && codeBuildProject != "")
	w.codeBuildStartButton.SetSensitive(codeBuildPage && codeBuildProject != "" && !w.codeBuildActionPending && w.options.CodeBuild != nil)
	hasCodeBuildLogs := codeBuildDetailReady && w.codeBuildDetail.LogGroupName != "" && w.codeBuildDetail.LogStreamName != "" && w.options.Logs != nil
	w.codeBuildLogsButton.SetVisible(codeBuildDetailReady)
	w.codeBuildLogsButton.SetSensitive(hasCodeBuildLogs && !w.codeBuildActionPending)
	w.codeBuildSearchLogsButton.SetVisible(codeBuildDetailReady)
	w.codeBuildSearchLogsButton.SetSensitive(hasCodeBuildLogs && !w.codeBuildActionPending)
	canStopCodeBuild := codeBuildDetailReady && w.codeBuildDetail.Status == "IN_PROGRESS"
	w.codeBuildStopButton.SetVisible(canStopCodeBuild)
	w.codeBuildStopButton.SetSensitive(canStopCodeBuild && !w.codeBuildActionPending && w.options.CodeBuild != nil)
	ecrPage := w.currentPage == pageECRImages || w.currentPage == pageECRFindings
	ecrImage, ecrImageSelected := w.selectedECRImageValue()
	ecrImageReady := ecrPage && ecrImageSelected && w.options.ECR != nil && !w.ecrActionPending
	w.ecrCopyURIButton.SetVisible(ecrPage && ecrImageSelected)
	w.ecrCopyURIButton.SetSensitive(ecrImageReady)
	w.ecrStartScanButton.SetVisible(ecrPage && ecrImageSelected)
	w.ecrStartScanButton.SetSensitive(ecrImageReady && canStartECRScan(ecrImage))
	if ecrImageSelected && !canStartECRScan(ecrImage) {
		w.ecrStartScanButton.SetTooltipText("An image scan is already active, pending, or in progress")
	} else {
		w.ecrStartScanButton.SetTooltipText("")
	}
	w.ecrDeleteImageButton.SetVisible(ecrPage && ecrImageSelected)
	w.ecrDeleteImageButton.SetSensitive(ecrImageReady)
	s3Page := w.currentPage == pageS3Buckets
	w.s3SaveSearchButton.SetVisible(s3Page)
	w.s3SaveSearchButton.SetSensitive(s3Page && w.options.Config != nil)
	hasSavedS3Searches := w.options.Config != nil && len(w.options.Config.S3Searches) > 0
	w.s3ManageSearchesButton.SetVisible(s3Page && hasSavedS3Searches)
	w.s3ManageSearchesButton.SetSensitive(s3Page && hasSavedS3Searches)
	s3ObjectPage := w.currentPage == pageS3Objects
	w.s3KeySearchButton.SetVisible(s3ObjectPage)
	w.s3KeySearchButton.SetSensitive(s3ObjectPage && !w.s3DownloadPending && w.options.S3 != nil)
	_, s3ObjectSelected := findS3Object(w.allS3Objects, w.selectedS3Object)
	w.s3DownloadButton.SetVisible(s3ObjectPage && s3ObjectSelected)
	w.s3DownloadButton.SetSensitive(s3ObjectPage && s3ObjectSelected && !w.s3DownloadPending && w.options.S3 != nil)
	dynamoTablePage := w.currentPage == pageDynamoTables
	dynamoItemsPage := w.currentPage == pageDynamoItems
	dynamoReady := w.options.DynamoDB != nil && !w.dynamoActionPending
	w.dynamoSaveTableButton.SetVisible(dynamoTablePage && w.selectedDynamoTable != "")
	w.dynamoSaveTableButton.SetSensitive(dynamoTablePage && w.selectedDynamoTable != "" && w.options.Config != nil && dynamoReady)
	hasSavedDynamo := w.options.Config != nil && (len(w.options.Config.DynamoTables) > 0 || len(w.options.Config.DynamoQueries) > 0)
	w.dynamoManageSavedButton.SetVisible((dynamoTablePage || dynamoItemsPage) && hasSavedDynamo)
	w.dynamoManageSavedButton.SetSensitive(hasSavedDynamo && dynamoReady)
	w.dynamoFilterButton.SetVisible(dynamoItemsPage && w.selectedDynamoTable != "")
	w.dynamoFilterButton.SetSensitive(dynamoItemsPage && w.selectedDynamoTable != "" && dynamoReady)
	w.dynamoPartiQLButton.SetVisible(dynamoItemsPage)
	w.dynamoPartiQLButton.SetSensitive(dynamoItemsPage && dynamoReady)
	w.dynamoSaveQueryButton.SetVisible(dynamoItemsPage && w.dynamoPartiQL != "")
	w.dynamoSaveQueryButton.SetSensitive(dynamoItemsPage && w.dynamoPartiQL != "" && w.options.Config != nil && dynamoReady)
	w.dynamoLoadMoreButton.SetVisible(dynamoItemsPage && w.dynamoNextToken != "")
	w.dynamoLoadMoreButton.SetSensitive(dynamoItemsPage && w.dynamoNextToken != "" && dynamoReady)
	_, dynamoItemSelected := w.selectedDynamoItemValue()
	dynamoItemMutable := dynamoItemsPage && dynamoItemSelected && w.selectedDynamoTable != "" && len(w.dynamoKeyNames) > 0
	w.dynamoEditButton.SetVisible(dynamoItemMutable)
	w.dynamoEditButton.SetSensitive(dynamoItemMutable && dynamoReady)
	w.dynamoCloneButton.SetVisible(dynamoItemMutable)
	w.dynamoCloneButton.SetSensitive(dynamoItemMutable && dynamoReady)
	sqsPage := w.currentPage == pageSQSQueues || w.currentPage == pageSQSMessages
	_, sqsQueueSelected := findSQSQueue(w.allSQSQueues, w.selectedSQSQueue)
	sqsReady := w.options.SQS != nil && !w.sqsActionPending
	w.sqsSaveQueueButton.SetVisible(w.currentPage == pageSQSQueues && sqsQueueSelected && w.activeSavedSQSQueue == "")
	w.sqsSaveQueueButton.SetSensitive(w.currentPage == pageSQSQueues && sqsQueueSelected && w.activeSavedSQSQueue == "" && w.options.Config != nil && sqsReady)
	hasSavedSQSQueues := w.options.Config != nil && len(w.options.Config.SQSQueues) > 0
	w.sqsManageSavedButton.SetVisible(sqsPage && hasSavedSQSQueues)
	w.sqsManageSavedButton.SetSensitive(sqsPage && hasSavedSQSQueues && sqsReady)
	w.sqsPollButton.SetVisible(w.currentPage == pageSQSMessages)
	w.sqsPollButton.SetSensitive(w.currentPage == pageSQSMessages && sqsReady)
	w.sqsClearButton.SetVisible(w.currentPage == pageSQSMessages && len(w.allSQSMessages) > 0)
	w.sqsClearButton.SetSensitive(w.currentPage == pageSQSMessages && len(w.allSQSMessages) > 0 && sqsReady)
	deadLetterAvailable := false
	if w.currentPage == pageSQSQueues {
		deadLetterAvailable = w.sqsQueueStats != nil && w.sqsQueueStats.DeadLetterTargetARN != ""
	} else if w.currentPage == pageSQSMessages {
		deadLetterAvailable = w.sqsMessageQueueStats != nil && w.sqsMessageQueueStats.DeadLetterTargetARN != ""
	}
	w.sqsDeadLetterButton.SetVisible(deadLetterAvailable)
	w.sqsDeadLetterButton.SetSensitive(deadLetterAvailable && sqsReady)
	_, _, sqsQueueActionReady := w.currentSQSQueueForAction()
	w.sqsSendButton.SetVisible(sqsPage && sqsQueueActionReady)
	w.sqsSendButton.SetSensitive(sqsPage && sqsQueueActionReady && sqsReady)
	_, sqsMessageSelected := w.selectedSQSMessageValue()
	w.sqsCloneButton.SetVisible(w.currentPage == pageSQSMessages && sqsMessageSelected)
	w.sqsCloneButton.SetSensitive(w.currentPage == pageSQSMessages && sqsMessageSelected && sqsReady)
	w.sqsDeleteButton.SetVisible(w.currentPage == pageSQSMessages && sqsMessageSelected)
	w.sqsDeleteButton.SetSensitive(w.currentPage == pageSQSMessages && sqsMessageSelected && sqsReady)
	ec2Page := w.currentPage == pageEC2Instances
	ec2Ready := ec2Page && w.ec2Detail != nil && w.ec2Detail.InstanceID == w.selectedEC2Instance
	ec2State := ""
	if ec2Ready {
		ec2State = normalizedEC2State(w.ec2Detail.State)
	}
	ec2ActionsReady := ec2Ready && !w.ec2ActionPending && w.options.EC2 != nil
	w.ec2DetailsButton.SetVisible(ec2Ready && w.ec2ViewMode == ec2ConsoleMode)
	w.ec2DetailsButton.SetSensitive(ec2ActionsReady)
	w.ec2ConsoleButton.SetVisible(ec2Ready && w.ec2ViewMode != ec2ConsoleMode)
	w.ec2ConsoleButton.SetSensitive(ec2ActionsReady)
	w.ec2SessionButton.SetVisible(ec2Ready && ec2State == "running")
	w.ec2SessionButton.SetSensitive(ec2ActionsReady && vteAvailable())
	if ec2Ready && ec2State == "running" && !vteAvailable() {
		w.ec2SessionButton.SetTooltipText("Rebuild with the gui and vte tags to enable the embedded terminal")
	} else {
		w.ec2SessionButton.SetTooltipText("")
	}
	w.ec2StartButton.SetVisible(ec2Ready && ec2State == "stopped")
	w.ec2StartButton.SetSensitive(ec2ActionsReady && ec2State == "stopped")
	w.ec2StopButton.SetVisible(ec2Ready && ec2State == "running")
	w.ec2StopButton.SetSensitive(ec2ActionsReady && ec2State == "running")
	w.ec2RebootButton.SetVisible(ec2Ready && ec2State == "running")
	w.ec2RebootButton.SetSensitive(ec2ActionsReady && ec2State == "running")
	w.ec2TerminateButton.SetVisible(ec2Ready && ec2CanTerminateState(ec2State))
	w.ec2TerminateButton.SetSensitive(ec2ActionsReady && ec2CanTerminateState(ec2State))
	w.scaleButton.SetVisible(serviceSelected)
	w.scaleButton.SetSensitive(serviceSelected)
	w.stopTaskButton.SetVisible(taskSelected && !taskStopped)
	w.stopTaskButton.SetSensitive(taskSelected && !taskStopped)
	w.deployButton.SetVisible(serviceSelected)
	w.deployButton.SetSensitive(serviceSelected)
}
