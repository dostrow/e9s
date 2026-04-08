// Package ui implements the bubbletea TUI application, views, and modal dialogs.
package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	e9saws "github.com/dostrow/e9s/internal/aws"
	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/theme"
	"github.com/dostrow/e9s/internal/ui/views"
)

type topMode int

const (
	modeECS topMode = iota
	modeCWLogs
	modeCWAlarms
	modeSSM
	modeSM
	modeS3
	modeLambda
	modeDynamoDB
	modeSQS
	modeCodeBuild
	modeEC2
	modeECR
	modeTofu
	modeRoute53
)

type viewState int

const (
	viewClusters viewState = iota
	viewServices
	viewTasks
	viewTaskDetail
	viewServiceDetail
	viewLogs
	viewStandaloneTasks
	viewTaskDefDiff
	viewMetrics
	viewSSM
	viewEnvVars
	viewSecrets
	viewSecretValue
	viewS3Buckets
	viewS3Objects
	viewS3Detail
	viewLambdaList
	viewLambdaDetail
	viewDynamoTables
	viewDynamoItems
	viewDynamoItemDetail
	viewSQSQueues
	viewSQSDetail
	viewSQSMessages
	viewSQSMessageDetail
	viewLogGroups
	viewLogStreams
	viewLogSearch
	viewAlarms
	viewAlarmDetail
	viewCBProjects
	viewCBBuilds
	viewCBBuildDetail
	viewEC2Instances
	viewEC2Detail
	viewEC2Console
	viewECRRepos
	viewECRImages
	viewECRFindings
	viewTofuResources
	viewTofuStateDetail
	viewTofuPlan
	viewTofuPlanDetail
	viewR53Zones
	viewR53Records
	viewR53RecordDetail
)

type App struct {
	client            *e9saws.Client
	cfg               *config.Config
	mode              topMode
	state             viewState
	prevState         viewState
	clusterView       views.ClusterListModel
	serviceView       views.ServiceListModel
	taskView          views.TaskListModel
	detailView        views.TaskDetailModel
	serviceDetailView views.ServiceDetailModel
	logView           views.LogViewerModel
	standaloneView    views.StandaloneTasksModel
	diffView          views.TaskDefDiffModel
	metricsView       views.MetricsModel
	envVarsView       views.EnvVarsModel
	logGroupsView     views.LogGroupsModel
	logStreamsView     views.LogStreamsModel
	logSearchView     views.LogSearchModel
	ssmView           views.SSMModel
	secretsView       views.SecretsModel
	secretValueView   views.SecretValueModel
	s3BucketsView     views.S3BucketsModel
	s3ObjectsView     views.S3ObjectsModel
	s3DetailView      views.S3DetailModel
	lambdaListView    views.LambdaListModel
	lambdaDetailView  views.LambdaDetailModel
	dynamoTablesView  views.DynamoTablesModel
	dynamoItemsView   views.DynamoItemsModel
	dynamoDetailView  views.DynamoItemDetailModel
	sqsQueuesView     views.SQSQueuesModel
	sqsDetailView     views.SQSDetailModel
	sqsMessagesView   views.SQSMessagesModel
	sqsMsgDetailView  views.SQSMessageDetailModel
	alarmsView        views.AlarmsModel
	alarmDetailView   views.AlarmDetailModel
	cbProjectsView    views.CBProjectsModel
	cbBuildsView      views.CBBuildsModel
	cbBuildDetailView views.CBBuildDetailModel
	ec2InstancesView  views.EC2InstancesModel
	ec2DetailView     views.EC2DetailModel
	ec2ConsoleView    views.EC2ConsoleModel
	ecrReposView      views.ECRReposModel
	ecrImagesView     views.ECRImagesModel
	ecrFindingsView   views.ECRFindingsModel
	tofuResourcesView   views.TofuResourcesModel
	tofuStateDetailView views.TofuStateDetailModel
	tofuPlanView        views.TofuPlanModel
	tofuPlanDetailView  views.TofuPlanDetailModel
	r53ZonesView        views.R53ZonesModel
	r53RecordsView    views.R53RecordsModel
	r53DetailView     views.R53RecordDetailModel
	regionPicker      views.RegionPickerModel

	// Navigation context
	selectedCluster    *model.Cluster
	selectedService    *model.Service
	selectedTask       *model.Task
	execContainerName  string
	logSearchGroup     string
	logSearchGroups    []string // multi-group search
	logSearchStreams    []string
	logSearchStartMs   int64
	logSearchEndMs     int64
	logSearchFilter    string // quoted/processed filter pattern for CW API
	logSaveGroup       string
	logSaveStream      string
	ssmEditName        string
	ssmEditValue       string
	smEditName         string
	smEditValue        string
	smCloneName        string
	smCloneValue       string
	s3DownloadBucket   string
	s3DownloadKey      string
	s3DownloadIsPrefix bool
	dynamoKeyNames []string
	dynamoLastKey  any // stores map[string]dbtypes.AttributeValue for pagination
	dynamoFilterAttr   string
	dynamoFilterOp     string
	dynamoFilterExpr   bool
	dynamoLastPartiQL  string
	sqsSendQueueURL    string
	sqsSendTemplate    *e9saws.SQSSendTemplate
	cbTriggerProject   string
	pathInput          *PathInput
	tofuDir            string
	r53EditZoneID      string
	r53EditRecord      *e9saws.R53Record
	r53EditOriginal    *e9saws.R53Record
	lambdaEditDir      string
	lambdaEditFunc     string
	lambdaEditZip      []byte
	dynamoEditField    string
	dynamoEditValue    string
	dynamoEditItem     *e9saws.DynamoItem
	dynamoCloneItem    *e9saws.DynamoItem

	// Modal dialogs
	confirm       ConfirmModel
	input         InputModel
	picker        PickerModel
	help          HelpModel
	modeSwitcher  ModeSwitcherModel

	// Mode tabs (built from config)
	modeTabs []ModeTab

	// State
	lastRefresh  time.Time
	lastActivity time.Time // updated on every keypress
	idleTimeout  time.Duration
	paused       bool // true when polling is paused (idle or manual)
	manualPause  bool // true when paused via ctrl+s (not auto-resumed by keypress)
	refreshSec   int
	kb           KeyBindings
	loading      bool
	err          error
	flashMessage   string
	flashExpiry    time.Time
	configModTime  time.Time // last known config file mod time
	width          int
	height         int
}

func NewApp(client *e9saws.Client, cfg *config.Config, defaultCluster string, refreshSec int) App {
	idleTimeout := 5 * time.Minute
	if cfg.Defaults.IdleTimeout > 0 {
		idleTimeout = time.Duration(cfg.Defaults.IdleTimeout) * time.Second
	}

	app := App{
		client:       client,
		cfg:          cfg,
		state:        viewClusters,
		clusterView:  views.NewClusterList(),
		refreshSec:   refreshSec,
		lastActivity: time.Now(),
		kb:           func() KeyBindings {
			kb := NewKeyBindings()
			kb.ApplyOverrides(cfg.KeyBindings)
			return kb
		}(),
		idleTimeout:  idleTimeout,
	}

	allModes := []struct {
		mode    topMode
		label   string
		enabled bool
	}{
		{modeECS, "ECS", cfg.ModuleECS()},
		{modeCWLogs, "CWL", cfg.ModuleCWLogs()},
		{modeCWAlarms, "CWA", cfg.ModuleCWAlarms()},
		{modeSSM, "SSM", cfg.ModuleSSM()},
		{modeSM, "SM", cfg.ModuleSM()},
		{modeS3, "S3", cfg.ModuleS3()},
		{modeLambda, "λ", cfg.ModuleLambda()},
		{modeDynamoDB, "DDB", cfg.ModuleDynamoDB()},
		{modeSQS, "SQS", cfg.ModuleSQS()},
		{modeCodeBuild, "CB", cfg.ModuleCodeBuild()},
		{modeEC2, "EC2i", cfg.ModuleEC2()},
		{modeECR, "ECR", cfg.ModuleECR()},
		{modeTofu, "TF", cfg.ModuleTofu()},
		{modeRoute53, "R53", cfg.ModuleRoute53()},
	}
	idx := 1
	for _, m := range allModes {
		if m.enabled {
			app.modeTabs = append(app.modeTabs, ModeTab{
				Mode:  m.mode,
				Label: m.label,
				Key:   fmt.Sprintf("%d", idx),
			})
			idx++
		}
	}

	app.configModTime = config.ModTime()

	// Determine startup mode
	defaultMode := resolveDefaultMode(cfg.Defaults.DefaultMode)

	if defaultCluster != "" {
		// CLI flag overrides — go straight to ECS services
		app.selectedCluster = &model.Cluster{Name: defaultCluster}
		app.state = viewServices
		app.serviceView = views.NewServiceList(defaultCluster)
	} else if defaultMode != nil {
		// Config default mode — will be applied in Init
		app.mode = *defaultMode
	}

	return app
}

// resolveDefaultMode maps a config string to a topMode, or nil for picker.
func resolveDefaultMode(s string) *topMode {
	modes := map[string]topMode{
		"ECS": modeECS, "ecs": modeECS,
		"CWL": modeCWLogs, "cwl": modeCWLogs, "cloudwatch-logs": modeCWLogs, "CloudWatch Logs": modeCWLogs,
		"CW": modeCWLogs, "cw": modeCWLogs, "cloudwatch": modeCWLogs, "CloudWatch": modeCWLogs,
		"CWA": modeCWAlarms, "cwa": modeCWAlarms, "cloudwatch-alarms": modeCWAlarms, "CloudWatch Alarms": modeCWAlarms,
		"SSM": modeSSM, "ssm": modeSSM,
		"SM": modeSM, "sm": modeSM, "secrets": modeSM,
		"S3": modeS3, "s3": modeS3,
		"Lambda": modeLambda, "lambda": modeLambda,
		"DynamoDB": modeDynamoDB, "dynamodb": modeDynamoDB, "DDB": modeDynamoDB, "ddb": modeDynamoDB,
		"SQS": modeSQS, "sqs": modeSQS,
		"CodeBuild": modeCodeBuild, "codebuild": modeCodeBuild, "CB": modeCodeBuild, "cb": modeCodeBuild,
		"EC2": modeEC2, "ec2": modeEC2, "EC2i": modeEC2, "ec2i": modeEC2,
		"ECR": modeECR, "ecr": modeECR,
		"Tofu": modeTofu, "tofu": modeTofu, "TF": modeTofu, "tf": modeTofu, "terraform": modeTofu, "opentofu": modeTofu,
		"Route53": modeRoute53, "route53": modeRoute53, "R53": modeRoute53, "r53": modeRoute53, "dns": modeRoute53,
	}
	if m, ok := modes[s]; ok {
		return &m
	}
	return nil
}

func (a App) Init() tea.Cmd {
	// If a cluster was specified via CLI, go to services
	if a.state == viewServices {
		return tea.Batch(a.loadServices(), a.tick())
	}

	// If a default mode is configured, switch to it
	if a.cfg.Defaults.DefaultMode != "" {
		if m := resolveDefaultMode(a.cfg.Defaults.DefaultMode); m != nil {
			// switchMode will be called on the first Update via a command
			mode := *m
			return tea.Batch(a.tick(), func() tea.Msg {
				return ModeSwitchSelectedMsg{Mode: mode}
			})
		}
	}

	// No default mode — show the mode switcher on first render
	// (Init can't set modal state with value receiver, so we use a startup message)
	return tea.Batch(a.tick(), func() tea.Msg {
		return showModeSwitcherMsg{}
	})
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Always handle window resize
	if wsm, ok := msg.(tea.WindowSizeMsg); ok {
		a.width = wsm.Width
		a.height = wsm.Height
		// Content area: width minus 3 (2 borders + 1 scrollbar), height minus 6 for frame chrome
		w := wsm.Width - 3
		h := wsm.Height - 6
		a.clusterView = a.clusterView.SetSize(w, h)
		a.serviceView = a.serviceView.SetSize(w, h)
		a.taskView = a.taskView.SetSize(w, h)
		a.detailView = a.detailView.SetSize(w, h)
		a.serviceDetailView = a.serviceDetailView.SetSize(w, h)
		a.logView = a.logView.SetSize(w, h)
		a.standaloneView = a.standaloneView.SetSize(w, h)
		a.diffView = a.diffView.SetSize(w, h)
		a.metricsView = a.metricsView.SetSize(w, h)
		a.ssmView = a.ssmView.SetSize(w, h)
		a.secretsView = a.secretsView.SetSize(w, h)
		a.secretValueView = a.secretValueView.SetSize(w, h)
		a.s3BucketsView = a.s3BucketsView.SetSize(w, h)
		a.s3ObjectsView = a.s3ObjectsView.SetSize(w, h)
		a.s3DetailView = a.s3DetailView.SetSize(w, h)
		a.lambdaListView = a.lambdaListView.SetSize(w, h)
		a.lambdaDetailView = a.lambdaDetailView.SetSize(w, h)
		a.dynamoTablesView = a.dynamoTablesView.SetSize(w, h)
		a.dynamoItemsView = a.dynamoItemsView.SetSize(w, h)
		a.dynamoDetailView = a.dynamoDetailView.SetSize(w, h)
		a.sqsQueuesView = a.sqsQueuesView.SetSize(w, h)
		a.sqsDetailView = a.sqsDetailView.SetSize(w, h)
		a.sqsMessagesView = a.sqsMessagesView.SetSize(w, h)
		a.sqsMsgDetailView = a.sqsMsgDetailView.SetSize(w, h)
		a.alarmsView = a.alarmsView.SetSize(w, h)
		a.alarmDetailView = a.alarmDetailView.SetSize(w, h)
		a.cbProjectsView = a.cbProjectsView.SetSize(w, h)
		a.cbBuildsView = a.cbBuildsView.SetSize(w, h)
		a.cbBuildDetailView = a.cbBuildDetailView.SetSize(w, h)
		a.ec2InstancesView = a.ec2InstancesView.SetSize(w, h)
		a.ec2DetailView = a.ec2DetailView.SetSize(w, h)
		a.ec2ConsoleView = a.ec2ConsoleView.SetSize(w, h)
		a.ecrReposView = a.ecrReposView.SetSize(w, h)
		a.ecrImagesView = a.ecrImagesView.SetSize(w, h)
		a.ecrFindingsView = a.ecrFindingsView.SetSize(w, h)
		a.tofuResourcesView = a.tofuResourcesView.SetSize(w, h)
		a.tofuStateDetailView = a.tofuStateDetailView.SetSize(w, h)
		a.tofuPlanView = a.tofuPlanView.SetSize(w, h)
		a.tofuPlanDetailView = a.tofuPlanDetailView.SetSize(w, h)
		a.r53ZonesView = a.r53ZonesView.SetSize(w, h)
		a.r53RecordsView = a.r53RecordsView.SetSize(w, h)
		a.r53DetailView = a.r53DetailView.SetSize(w, h)
		a.envVarsView = a.envVarsView.SetSize(w, h)
		a.logGroupsView = a.logGroupsView.SetSize(w, h)
		a.logStreamsView = a.logStreamsView.SetSize(w, h)
		a.logSearchView = a.logSearchView.SetSize(w, h)
		return a, nil
	}

	// Handle overlays — these consume all input when active
	if a.help.Active {
		if _, ok := msg.(tea.KeyMsg); ok {
			a.help.Active = false
			return a, nil
		}
	}
	if a.modeSwitcher.Active {
		switch msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			a.modeSwitcher, cmd = a.modeSwitcher.Update(msg)
			return a, cmd
		case ModeSaveDefaultMsg, ModeSwitchSelectedMsg:
			// Let these pass through to the main handler
		default:
			return a, nil
		}
	}
	if a.regionPicker.Active {
		switch rm := msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			a.regionPicker, cmd = a.regionPicker.Update(rm)
			return a, cmd
		case views.RegionSwitchMsg:
			return a, a.switchRegion(rm.Region)
		}
		return a, nil
	}
	if a.picker.Active {
		switch msg.(type) {
		case tea.KeyMsg:
			var cmd tea.Cmd
			a.picker, cmd = a.picker.Update(msg)
			return a, cmd
		}
		return a, nil
	}
	if a.confirm.Active {
		var cmd tea.Cmd
		a.confirm, cmd = a.confirm.Update(msg)
		return a, cmd
	}
	if a.input.Active {
		var cmd tea.Cmd
		a.input, cmd = a.input.Update(msg)
		return a, cmd
	}
	if a.pathInput != nil {
		// Let result/cancel messages pass through to the main handler
		switch msg.(type) {
		case PathInputResultMsg, PathInputCancelMsg:
			// fall through to main switch
		case tea.KeyMsg:
			var cmd tea.Cmd
			pi := *a.pathInput
			pi, cmd = pi.Update(msg)
			a.pathInput = &pi
			return a, cmd
		default:
			return a, nil
		}
	}

	// If a list view is in filter mode, delegate all key input to it
	if a.isFiltering() {
		if km, ok := msg.(tea.KeyMsg); ok {
			return a.delegateToActiveView(km)
		}
	}

	switch msg := msg.(type) {
	// --- ECS data messages ---
	case clustersLoadedMsg:
		a.loading = false
		a.lastRefresh = time.Now()
		a.err = nil
		a.clusterView = a.clusterView.SetClusters(msg.clusters)
		return a, nil

	case servicesLoadedMsg:
		a.loading = false
		a.lastRefresh = time.Now()
		a.err = nil
		a.serviceView = a.serviceView.SetServices(msg.services)
		if a.state == viewServiceDetail && a.selectedService != nil {
			for _, s := range msg.services {
				if s.Name == a.selectedService.Name {
					a.selectedService = &s
					a.serviceDetailView = a.serviceDetailView.SetService(&s)
					break
				}
			}
		}
		return a, nil

	case tasksLoadedMsg:
		a.loading = false
		a.lastRefresh = time.Now()
		a.err = nil
		a.taskView = a.taskView.SetTasks(msg.tasks)
		return a, nil

	case taskDetailRefreshedMsg:
		if msg.task != nil {
			a.selectedTask = msg.task
			a.detailView = views.NewTaskDetail(msg.task)
			a.detailView = a.detailView.SetSize(a.width, a.height-3)
			a.lastRefresh = time.Now()
		}
		return a, nil

	case standaloneTasksLoadedMsg:
		a.loading = false
		a.lastRefresh = time.Now()
		a.err = nil
		a.standaloneView = a.standaloneView.SetTasks(msg.tasks)
		return a, nil

	case errMsg:
		a.loading = false
		a.err = msg.err
		return a, nil

	// --- Log messages ---
	case logReadyMsg:
		a.state = viewLogs
		follow := true
		lookback := 10 * time.Second
		if msg.follow != nil {
			follow = *msg.follow
		}
		if msg.lookback > 0 {
			lookback = msg.lookback
		}
		if msg.search != "" {
			a.logView = views.NewLogViewerWithSearch(msg.title, a.client, msg.logGroup, msg.streams, follow, lookback, msg.search)
		} else {
			a.logView = views.NewLogViewerWithOptions(msg.title, a.client, msg.logGroup, msg.streams, follow, lookback)
		}
		a.logView = a.logView.SetSize(a.width, a.height-3)
		return a, a.logView.Init()

	case views.LogSearchJumpMsg:
		var streams []string
		if msg.Stream != "" {
			streams = []string{msg.Stream}
		}
		title := fmt.Sprintf("search: %q", msg.Pattern)
		if msg.Stream != "" {
			title += " in " + msg.Stream
		}
		a.prevState = viewLogSearch
		a.state = viewLogs
		a.logView = views.NewLogViewerAtTimestamp(title, a.client, msg.LogGroup, streams, msg.Timestamp, msg.Pattern)
		a.logView = a.logView.SetSize(a.width, a.height-3)
		return a, a.logView.Init()

	case views.LogsLoadedMsg, views.LogsErrorMsg, views.LogTickMsg, views.LogsPrependedMsg:
		if a.state == viewLogs {
			var cmd tea.Cmd
			a.logView, cmd = a.logView.Update(msg)
			return a, cmd
		}
		return a, nil

	case views.LogSearchResultsMsg, views.LogSearchErrorMsg:
		if a.state == viewLogSearch {
			var cmd tea.Cmd
			a.logSearchView, cmd = a.logSearchView.Update(msg)
			return a, cmd
		}
		return a, nil

	case views.LogSearchPartialMsg:
		if a.state == viewLogSearch {
			var viewCmd tea.Cmd
			a.logSearchView, viewCmd = a.logSearchView.Update(msg)

			// Chain the next group search if multi-group and not done
			if !msg.Done && len(a.logSearchGroups) > 1 {
				// Find which group just completed and chain the next
				nextIdx := -1
				for i, g := range a.logSearchGroups {
					if g == msg.Source {
						nextIdx = i + 1
						break
					}
				}
				if nextIdx > 0 && nextIdx < len(a.logSearchGroups) {
					nextCmd := searchNextGroup(a.client, a.logSearchGroups, nextIdx,
						a.logSearchFilter, a.logSearchStreams,
						a.logSearchStartMs, a.logSearchEndMs)
					return a, tea.Batch(viewCmd, nextCmd)
				}
			}
			// For single-group paginated search, chain next page if not done
			if !msg.Done && len(a.logSearchGroups) <= 1 && msg.NextToken != nil {
				nextCmd := searchGroupPaginated(a.client, msg.Source, a.logSearchStreams,
					a.logSearchFilter, a.logSearchStartMs, a.logSearchEndMs,
					msg.NextToken, msg.Remaining)
				return a, tea.Batch(viewCmd, nextCmd)
			}
			return a, viewCmd
		}
		return a, nil

	case logGroupsLoadedMsg:
		a.logGroupsView = a.logGroupsView.SetGroups(msg.groups)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case logStreamsLoadedMsg:
		a.logStreamsView = a.logStreamsView.SetStreams(msg.streams)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	// --- ECS detail messages ---
	case envVarsReadyMsg:
		a.state = viewEnvVars
		a.envVarsView = views.NewEnvVars(msg.title, msg.envVars)
		a.envVarsView = a.envVarsView.SetSize(a.width, a.height-3)
		return a, nil

	case taskDefDiffReadyMsg:
		a.state = viewTaskDefDiff
		a.diffView = views.NewTaskDefDiff(msg.title, msg.diff)
		a.diffView = a.diffView.SetSize(a.width, a.height-3)
		return a, nil

	case metricsLoadedMsg:
		a.metricsView = a.metricsView.SetMetrics(msg.metrics)
		a.metricsView = a.metricsView.SetAlarms(msg.alarms)
		a.loading = false
		return a, nil

	case execSessionReadyMsg:
		wrap := NewExecWrap(msg.pluginPath, msg.args)
		return a, tea.Exec(wrap, func(err error) tea.Msg {
			return execFinishedMsg{err: err}
		})

	case execFinishedMsg:
		if msg.err != nil {
			a.err = msg.err
		}
		return a, nil

	// --- SSM messages ---
	case ssmParamsLoadedMsg:
		a.ssmView = a.ssmView.SetParams(msg.params)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case ssmEditReadyMsg:
		a.ssmEditName = msg.name
		a.input = NewInput(InputSSMEditValue,
			fmt.Sprintf("Edit %s (type: %s)", msg.name, msg.paramType),
			msg.currentValue)
		return a, nil

	case ssmUpdatedMsg:
		a.ssmView = a.ssmView.SetParams(msg.params)
		a.flashMessage = fmt.Sprintf("Updated %q", msg.name)
		a.flashExpiry = time.Now().Add(5 * time.Second)
		return a, nil

	// --- Secrets Manager messages ---
	case smSecretsLoadedMsg:
		a.secretsView = a.secretsView.SetSecrets(msg.secrets)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case smValueReadyMsg:
		a.state = viewSecretValue
		a.secretValueView = views.NewSecretValue(msg.name, msg.value, msg.tags)
		a.secretValueView = a.secretValueView.SetSize(a.width, a.height-3)
		return a, nil

	case smEditReadyMsg:
		a.smEditName = msg.name
		a.input = NewInput(InputSMEditValue,
			fmt.Sprintf("Edit secret %s", msg.name),
			msg.currentValue)
		return a, nil

	case smCloneReadyMsg:
		return a.handleCloneReady(msg)

	case smCloneEditedMsg:
		a.smCloneName = msg.name
		a.smCloneValue = msg.value
		a.confirm = NewConfirm(ConfirmSMClone,
			fmt.Sprintf("Create new secret %q?", msg.name))
		return a, nil

	case smUpdatedMsg:
		a.secretsView = a.secretsView.SetSecrets(msg.secrets)
		a.flashMessage = fmt.Sprintf("Updated %q", msg.name)
		a.flashExpiry = time.Now().Add(5 * time.Second)
		return a, nil

	// --- DynamoDB messages ---
	case dynamoTablesLoadedMsg:
		a.dynamoTablesView = a.dynamoTablesView.SetTables(msg.tables)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case dynamoScanReadyMsg:
		a.dynamoKeyNames = msg.keyNames
		a.dynamoLastKey = msg.lastKey
		a.dynamoItemsView = views.NewDynamoItems(msg.tableName, msg.keyNames)
		a.dynamoItemsView = a.dynamoItemsView.SetSize(a.width, a.height-3)
		a.dynamoItemsView = a.dynamoItemsView.SetItems(msg.items, msg.hasMore)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case dynamoItemsLoadedMsg:
		a.dynamoLastKey = msg.lastKey
		a.dynamoItemsView = views.NewDynamoItems(a.dynamoItemsView.TableName(), a.dynamoKeyNames)
		a.dynamoItemsView = a.dynamoItemsView.SetSize(a.width, a.height-3)
		a.dynamoItemsView = a.dynamoItemsView.SetItems(msg.items, msg.hasMore)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case dynamoPageLoadedMsg:
		a.dynamoLastKey = msg.lastKey
		a.dynamoItemsView = a.dynamoItemsView.AppendItems(msg.items, msg.hasMore)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case dynamoItemRefreshedMsg:
		if msg.item != nil {
			a.dynamoDetailView = views.NewDynamoItemDetail(
				a.dynamoDetailView.TableName(), a.dynamoKeyNames, msg.item)
			a.dynamoDetailView = a.dynamoDetailView.SetSize(a.width, a.height-3)
		}
		return a, nil

	case dynamoFieldEditedMsg:
		// User finished editing in $EDITOR, confirm the write
		a.dynamoEditField = msg.fieldName
		a.dynamoEditValue = msg.newValue
		a.dynamoEditItem = msg.item
		a.confirm = NewConfirm(ConfirmDynamoFieldEdit,
			fmt.Sprintf("Update field %q on this item?", msg.fieldName))
		return a, nil

	case dynamoItemClonedMsg:
		a.dynamoCloneItem = &msg.newItem
		a.confirm = NewConfirm(ConfirmDynamoClone, "Create this new item?")
		return a, nil

	case dynamoWriteDoneMsg:
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		a.flashMessage = msg.message
		a.flashExpiry = time.Now().Add(5 * time.Second)
		// Re-fetch the item to refresh the detail view
		if a.state == viewDynamoItemDetail && a.dynamoDetailView.Item() != nil {
			return a, a.refreshDynamoDetail()
		}
		return a, nil

	case dynamoPartiQLResultMsg:
		a.loading = false
		if msg.err != nil {
			a.err = msg.err
			return a, nil
		}
		a.dynamoItemsView = views.NewDynamoItems(a.dynamoItemsView.TableName(), a.dynamoKeyNames)
		a.dynamoItemsView = a.dynamoItemsView.SetSize(a.width, a.height-3)
		a.dynamoItemsView = a.dynamoItemsView.SetItems(msg.items, false)
		return a, nil

	// --- SQS messages ---
	case sqsQueuesLoadedMsg:
		a.sqsQueuesView = a.sqsQueuesView.SetQueues(msg.queues)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case sqsStatsLoadedMsg:
		a.sqsDetailView = a.sqsDetailView.SetStats(msg.stats)
		a.loading = false
		return a, nil

	case sqsMessagesReceivedMsg:
		a.sqsMessagesView = a.sqsMessagesView.SetMessages(msg.messages)
		a.loading = false
		a.flashMessage = fmt.Sprintf("Received %d messages", len(msg.messages))
		a.flashExpiry = time.Now().Add(3 * time.Second)
		return a, nil

	case sqsDLQResolvedMsg:
		return a.openSQSDetail(msg.name, msg.url)

	case sqsSendReadyMsg:
		a.confirm = NewConfirm(ConfirmSQSSend, "Send this message?")
		a.sqsSendQueueURL = msg.queueURL
		a.sqsSendTemplate = msg.template
		return a, nil

	// --- Lambda messages ---
	case lambdaFunctionsLoadedMsg:
		a.lambdaListView = a.lambdaListView.SetFunctions(msg.functions)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case lambdaCodeReadyMsg:
		return a.handleLambdaCodeReady(msg)

	case lambdaCodeEditedMsg:
		a.lambdaEditFunc = msg.functionName
		a.lambdaEditZip = msg.zipData
		a.confirm = NewConfirm(ConfirmLambdaCodeUpdate,
			fmt.Sprintf("Deploy updated code to %s?", msg.functionName))
		return a, nil

	case lambdaCodeUpdatedMsg:
		a.flashMessage = msg.message
		a.flashExpiry = time.Now().Add(5 * time.Second)
		a.loading = false
		return a, nil

	// --- S3 messages ---
	case s3BucketsLoadedMsg:
		a.s3BucketsView = a.s3BucketsView.SetBuckets(msg.buckets)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case s3ObjectsLoadedMsg:
		a.s3ObjectsView = a.s3ObjectsView.SetObjects(msg.objects)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case s3DetailLoadedMsg:
		a.state = viewS3Detail
		a.s3DetailView = views.NewS3Detail(msg.bucket, msg.detail)
		a.s3DetailView = a.s3DetailView.SetSize(a.width, a.height-3)
		return a, nil

	case s3DownloadDoneMsg:
		if msg.err != nil {
			a.err = msg.err
		} else {
			a.flashMessage = msg.message
			a.flashExpiry = time.Now().Add(5 * time.Second)
		}
		return a, nil

	// --- CloudWatch Alarms messages ---
	case alarmsLoadedMsg:
		a.alarmsView = a.alarmsView.SetAlarms(msg.alarms)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case alarmDetailLoadedMsg:
		a.alarmDetailView = views.NewAlarmDetail(msg.detail)
		a.alarmDetailView = a.alarmDetailView.SetSize(a.width-3, a.height-6)
		a.state = viewAlarmDetail
		a.loading = false
		return a, nil

	case alarmActionDoneMsg:
		a.flashMessage = msg.message
		a.flashExpiry = time.Now().Add(5 * time.Second)
		a.loading = false
		// Refresh detail if we're on it
		if a.state == viewAlarmDetail {
			return a, a.refreshAlarmDetail()
		}
		return a, nil

	// --- CodeBuild messages ---
	case cbProjectsLoadedMsg:
		a.cbProjectsView = a.cbProjectsView.SetProjects(msg.projects)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case cbBuildsLoadedMsg:
		a.cbBuildsView = a.cbBuildsView.SetBuilds(msg.builds)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case cbBuildDetailLoadedMsg:
		a.cbBuildDetailView = views.NewCBBuildDetail(msg.detail)
		a.cbBuildDetailView = a.cbBuildDetailView.SetSize(a.width-3, a.height-6)
		a.state = viewCBBuildDetail
		a.loading = false
		return a, nil

	case cbBuildStartedMsg:
		return a.handleCBBuildStarted(msg)

	case cbBuildStoppedMsg:
		a.flashMessage = msg.message
		a.flashExpiry = time.Now().Add(5 * time.Second)
		a.loading = false
		if a.state == viewCBBuildDetail {
			return a, a.refreshCBBuildDetail()
		}
		return a, nil

	// --- EC2 messages ---
	case ec2InstancesLoadedMsg:
		a.ec2InstancesView = a.ec2InstancesView.SetInstances(msg.instances)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case ec2DetailLoadedMsg:
		a.ec2DetailView = views.NewEC2Detail(msg.detail)
		a.ec2DetailView = a.ec2DetailView.SetSize(a.width-3, a.height-6)
		a.state = viewEC2Detail
		a.loading = false
		return a, nil

	case ec2ConsoleLoadedMsg:
		a.ec2ConsoleView = a.ec2ConsoleView.SetOutput(msg.output)
		a.loading = false
		return a, nil

	case ec2ActionDoneMsg:
		return a.handleEC2Action(msg)

	// --- ECR messages ---
	case ecrReposLoadedMsg:
		a.ecrReposView = a.ecrReposView.SetRepos(msg.repos)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case ecrImagesLoadedMsg:
		a.ecrImagesView = a.ecrImagesView.SetImages(msg.images)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case ecrFindingsLoadedMsg:
		a.ecrFindingsView = a.ecrFindingsView.SetFindings(msg.findings)
		a.loading = false
		return a, nil

	case ecrActionDoneMsg:
		return a.handleECRAction(msg)

	// --- OpenTofu messages ---
	case tofuResourcesLoadedMsg:
		a.tofuResourcesView = a.tofuResourcesView.SetResources(msg.resources)
		a.loading = false
		return a, nil

	case tofuStateDetailMsg:
		a.tofuStateDetailView = a.tofuStateDetailView.SetOutput(msg.output)
		a.loading = false
		return a, nil

	case tofuPlanLoadedMsg:
		a.tofuPlanView = a.tofuPlanView.SetPlan(msg.plan)
		a.loading = false
		return a, nil

	case tofuApplyDoneMsg:
		a.flashMessage = msg.message
		a.flashExpiry = time.Now().Add(5 * time.Second)
		// Refresh state after apply
		if a.state == viewTofuResources || a.state == viewTofuPlan {
			a.loading = true
			return a, a.refreshTofuResources()
		}
		return a, nil

	case tofuInitDoneMsg:
		a.flashMessage = msg.message
		a.flashExpiry = time.Now().Add(5 * time.Second)
		a.loading = false
		return a, nil

	// --- Route53 messages ---
	case r53ZonesLoadedMsg:
		a.r53ZonesView = a.r53ZonesView.SetZones(msg.zones)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case r53RecordsLoadedMsg:
		a.r53RecordsView = a.r53RecordsView.SetRecords(msg.records)
		a.loading = false
		a.lastRefresh = time.Now()
		return a, nil

	case r53DNSAnswerMsg:
		a.r53DetailView = a.r53DetailView.SetDNSAnswer(msg.answer)
		a.loading = false
		return a, nil

	case r53RecordEditedMsg:
		a.r53EditRecord = msg.record
		if msg.isNew {
			a.confirm = NewConfirm(ConfirmR53Create,
				fmt.Sprintf("Create %s record %q?", msg.record.Type, msg.record.Name))
		} else {
			a.confirm = NewConfirm(ConfirmR53Update,
				fmt.Sprintf("Update %s record %q?", msg.record.Type, msg.record.Name))
		}
		return a, nil

	case r53ActionDoneMsg:
		return a.handleR53Action(msg)

	// --- Shared messages ---
	case regionSwitchedMsg:
		a.flashMessage = fmt.Sprintf("Switched to %s", a.client.Region())
		a.flashExpiry = time.Now().Add(5 * time.Second)
		a.state = viewClusters
		a.selectedCluster = nil
		a.selectedService = nil
		a.selectedTask = nil
		a.loading = true
		return a, a.loadClusters()

	case showModeSwitcherMsg:
		a.modeSwitcher = NewModeSwitcher(a.modeTabs, a.mode)
		return a, nil

	case ModeSaveDefaultMsg:
		modeName := modeDisplayName(msg.Mode)
		a.cfg.Defaults.DefaultMode = modeName
		if err := a.cfg.Save(); err != nil {
			a.err = err
		} else {
			a.flashMessage = fmt.Sprintf("Default mode set to %s", modeName)
			a.flashExpiry = time.Now().Add(3 * time.Second)
		}
		// Also switch to it
		a.modeSwitcher.Active = false
		return a.switchMode(msg.Mode)

	case ModeSwitchSelectedMsg:
		return a.switchMode(msg.Mode)

	case views.RegionSwitchMsg:
		return a, a.switchRegion(msg.Region)

	case actionSuccessMsg:
		a.flashMessage = msg.message
		a.flashExpiry = time.Now().Add(5 * time.Second)
		return a, a.refreshCurrentView()

	// --- Dialog results ---
	case ConfirmResultMsg:
		if !msg.Confirmed {
			return a, nil
		}
		switch msg.Action {
		case ConfirmForceDeploy:
			return a, a.doForceDeploy()
		case ConfirmStopTask:
			return a, a.doStopTask()
		case ConfirmSSMUpdate:
			return a, a.doSSMUpdate()
		case ConfirmSMClone:
			return a, a.doCreateClonedSecret()
		case ConfirmSMUpdate:
			return a, a.doSMUpdate()
		case ConfirmDynamoFieldEdit:
			return a, a.doDynamoFieldEdit()
		case ConfirmDynamoClone:
			return a, a.doDynamoClone()
		case ConfirmSQSDelete:
			return a, a.doDeleteSQSMessage()
		case ConfirmSQSSend:
			if a.sqsSendTemplate != nil {
				return a, a.doSendSQSMessage(a.sqsSendQueueURL, a.sqsSendTemplate)
			}
		case ConfirmCBStartBuild:
			return a, a.doStartCBBuild()
		case ConfirmCBStopBuild:
			return a, a.doStopCBBuild()
		case ConfirmLambdaCodeUpdate:
			return a, a.doLambdaCodeUpdate()
		case ConfirmEC2Start:
			return a, a.doStartEC2()
		case ConfirmEC2Stop:
			return a, a.doStopEC2()
		case ConfirmEC2Reboot:
			return a, a.doRebootEC2()
		case ConfirmECRDelete:
			return a, a.doDeleteECRImage()
		case ConfirmR53Create:
			return a, a.doCreateR53Record()
		case ConfirmR53Update:
			return a, a.doUpdateR53Record()
		case ConfirmR53Delete:
			return a, a.doDeleteR53Record()
		case ConfirmEC2Terminate:
			return a, a.doTerminateEC2()
		}
		return a, nil

	case PathInputResultMsg:
		a.pathInput = nil
		switch msg.Action {
		case InputTofuDir:
			return a.openTofuResources(msg.Value)
		}
		return a, nil

	case PathInputCancelMsg:
		a.pathInput = nil
		return a.showModePicker()

	case InputResultMsg:
		if msg.Canceled {
			return a, nil
		}
		switch msg.Action {
		case InputScale:
			count, err := ParseScaleInput(msg.Value)
			if err != nil {
				a.err = err
				return a, nil
			}
			return a, a.doScale(count)
		case InputSSMPath:
			return a.openSSM(msg.Value)
		case InputSSMSaveName:
			return a.doSaveSSMPrefix(msg.Value)
		case InputSSMEditValue:
			return a.confirmSSMUpdate(msg.Value)
		case InputExecCommand:
			return a, a.doExecWithCommand(msg.Value)
		case InputLogGroupPrefix:
			return a.openLogGroups(msg.Value)
		case InputLogSearchPattern:
			return a.startLogSearch(msg.Value)
		case InputLogSearchFrom:
			t, err := parseUTCTimestamp(msg.Value)
			if err != nil {
				a.err = err
				return a, nil
			}
			a.logSearchStartMs = t.UnixMilli()
			now := time.Now().UTC()
			defaultTo := now.Format("2006-01-02 15:04")
			a.input = NewInput(InputLogSearchTo, "To (YYYY-MM-DD HH:MM, UTC)", defaultTo)
			return a, nil
		case InputLogSearchTo:
			t, err := parseUTCTimestamp(msg.Value)
			if err != nil {
				a.err = err
				return a, nil
			}
			a.logSearchEndMs = t.UnixMilli()
			a.input = NewInput(InputLogSearchPattern, "Search pattern (CloudWatch filter syntax)", "")
			return a, nil
		case InputTofuSaveName:
			return a.doSaveTofuDir(msg.Value)
		case InputLogSearchGroupsSave:
			return a.doSaveLogSearchGroups(msg.Value)
		case InputLogSaveName:
			return a.doSaveLogPath(msg.Value)
		case InputLogSaveFile:
			return a.doSaveLogBuffer(msg.Value)
		case InputSMFilter:
			return a.openSecrets(msg.Value)
		case InputSMSaveName:
			return a.doSaveSMFilter(msg.Value)
		case InputSMEditValue:
			return a.confirmSMUpdate(msg.Value)
		case InputSMCloneName:
			return a.handleCloneName(msg.Value)
		case InputS3Search:
			return a.openS3Buckets(msg.Value)
		case InputS3SaveName:
			return a.doSaveS3Search(msg.Value)
		case InputS3Download:
			return a, a.doS3Download(msg.Value)
		case InputS3KeySearch:
			return a.searchS3Keys(msg.Value)
		case InputLambdaSearch:
			return a.openLambdaList(msg.Value)
		case InputLambdaSaveName:
			return a.doSaveLambdaSearch(msg.Value)
		case InputDynamoSearch:
			return a.openDynamoTables(msg.Value)
		case InputDynamoSaveName:
			return a.doSaveDynamoTable(msg.Value)
		case InputDynamoFilterAttr:
			a.dynamoFilterAttr = msg.Value
			a.dynamoFilterExpr = false
			return a.promptDynamoFilterOp()
		case InputDynamoFilterValue:
			return a.executeDynamoFilter(msg.Value)
		case InputDynamoPartiQL:
			return a.executeDynamoPartiQL(msg.Value)
		case InputDynamoQuerySaveName:
			return a.doSaveDynamoQuery(msg.Value)
		case InputSQSSearch:
			return a.openSQSQueues(msg.Value)
		case InputSQSSaveName:
			return a.doSaveSQSQueue(msg.Value)
		}
		return a, nil

	case PickerDeleteMsg:
		return a.handlePickerDelete(msg)

	case PickerResultMsg:
		if msg.Canceled {
			return a, nil
		}
		switch msg.Action {
		case PickerExecContainer:
			return a.doExec(msg.Value)
		case PickerLogContainer:
			return a, a.doLogForContainer(msg.Value)
		case PickerEnvContainer:
			return a, a.doShowEnvVars(msg.Value)
		case PickerSSMPrefix:
			if msg.Index == len(a.cfg.SSMPrefixes) {
				a.input = NewInput(InputSSMPath, "SSM Parameter path prefix", "/")
				return a, nil
			}
			return a.openSSM(a.cfg.SSMPrefixes[msg.Index].Prefix)
		case PickerSMFilter:
			if msg.Index == len(a.cfg.SMFilters) {
				a.input = NewInput(InputSMFilter, "Secret name filter (substring match)", "")
				return a, nil
			}
			return a.openSecrets(a.cfg.SMFilters[msg.Index].Filter)
		case PickerS3Search:
			if msg.Index == len(a.cfg.S3Searches) {
				a.input = NewInput(InputS3Search, "Search buckets (substring match)", "")
				return a, nil
			}
			return a.openS3Buckets(a.cfg.S3Searches[msg.Index].Filter)
		case PickerLambdaSearch:
			if msg.Index == len(a.cfg.LambdaSearches) {
				a.input = NewInput(InputLambdaSearch, "Search functions (substring match, or empty for all)", "")
				return a, nil
			}
			return a.openLambdaList(a.cfg.LambdaSearches[msg.Index].Filter)
		case PickerSQSQueue:
			if msg.Index == len(a.cfg.SQSQueues) {
				a.input = NewInput(InputSQSSearch, "Search queues (substring match, or empty for all)", "")
				return a, nil
			}
			q := a.cfg.SQSQueues[msg.Index]
			return a.openSQSDetail(q.Name, q.URL)
		case PickerDynamoTable:
			if msg.Index == len(a.cfg.DynamoTables) {
				a.input = NewInput(InputDynamoSearch, "Search tables (substring match, or empty for all)", "")
				return a, nil
			}
			return a.openDynamoTableDirect(a.cfg.DynamoTables[msg.Index].Table)
		case PickerDynamoQuery:
			if msg.Index == len(a.cfg.DynamoQueries) {
				tableName := a.dynamoItemsView.TableName()
				a.input = NewInput(InputDynamoPartiQL, "PartiQL statement",
					fmt.Sprintf("SELECT * FROM \"%s\" WHERE ", tableName))
				return a, nil
			}
			return a.executeDynamoPartiQL(a.cfg.DynamoQueries[msg.Index].Statement)
		case PickerDynamoFilterOp:
			return a.handleDynamoFilterOp(msg.Value)
		case PickerLogPath:
			if msg.Index == len(a.cfg.LogPaths) {
				a.input = NewInput(InputLogGroupPrefix, "Search log groups (prefix with / or substring match)", "")
				return a, nil
			}
			lp := a.cfg.LogPaths[msg.Index]
			if len(lp.LogGroups) > 1 {
				// Multi-group saved entry — go straight to search
				a.prevState = viewLogGroups
				a.logSearchGroups = lp.LogGroups
				a.logSearchGroup = lp.LogGroups[0]
				a.logSearchStreams = nil
				return a.promptLogSearchTimeRange()
			}
			if lp.Stream != "" {
				return a, a.startLogTail(lp.LogGroup, []string{lp.Stream},
					fmt.Sprintf("%s / %s", lp.LogGroup, lp.Stream))
			}
			return a.openLogStreams(lp.LogGroup)
		case PickerLogSearchTimeRange:
			return a.handleTimeRangePick(msg.Value)
		case PickerCWAlarmState:
			return a.handleCWAlarmStatePick(msg.Value)
		case PickerTofuDir:
			if msg.Index == len(a.cfg.TofuDirs) {
				cwd, _ := os.Getwd()
				pi := NewPathInput(InputTofuDir, "OpenTofu directory path", cwd+"/")
				a.pathInput = &pi
				return a, nil
			}
			td := a.cfg.TofuDirs[msg.Index]
			return a.openTofuResources(td.Dir)
		case PickerSetAlarmState:
			return a.handleSetAlarmStatePick(msg.Value)
		}
		return a, nil

	// --- Tick ---
	case configEditedMsg:
		// Config was edited in $EDITOR — reload it
		newCfg := config.Reload()
		a.cfg = &newCfg
		a.configModTime = config.ModTime()
		a.flashMessage = "Config reloaded"
		a.flashExpiry = time.Now().Add(3 * time.Second)
		return a, nil

	case configCheckMsg:
		// Periodic hot-reload check
		modTime := config.ModTime()
		if !modTime.IsZero() && modTime.After(a.configModTime) {
			newCfg := config.Reload()
			a.cfg = &newCfg
			a.configModTime = modTime
			a.flashMessage = "Config reloaded (file changed)"
			a.flashExpiry = time.Now().Add(3 * time.Second)
		}
		return a, nil

	case tickMsg:
		if !a.flashExpiry.IsZero() && time.Now().After(a.flashExpiry) {
			a.flashMessage = ""
			a.flashExpiry = time.Time{}
		}
		// Check for config changes every tick
		modTime := config.ModTime()
		if !modTime.IsZero() && modTime.After(a.configModTime) {
			newCfg := config.Reload()
			a.cfg = &newCfg
			a.configModTime = modTime
			a.flashMessage = "Config reloaded"
			a.flashExpiry = time.Now().Add(3 * time.Second)
		}
		// Pause refresh when idle or manually paused
		if a.paused || (a.idleTimeout > 0 && time.Since(a.lastActivity) > a.idleTimeout) {
			a.paused = true
			return a, a.tick() // keep ticking for flash/config but skip refresh
		}
		return a, tea.Batch(a.refreshCurrentView(), a.tick())

	// --- Key input ---
	case tea.KeyMsg:
		// Reset idle timer on any keypress
		a.lastActivity = time.Now()

		// Toggle manual pause
		if msg.String() == a.kb.PauseResume {
			a.paused = !a.paused
			a.manualPause = a.paused
			if a.paused {
				a.flashMessage = "Polling paused"
				a.flashExpiry = time.Now().Add(3 * time.Second)
			} else {
				a.flashMessage = "Polling resumed"
				a.flashExpiry = time.Now().Add(3 * time.Second)
				a.loading = true
				return a, a.refreshCurrentView()
			}
			return a, nil
		}

		// If idle-paused (not manually), any key resumes
		if a.paused && !a.manualPause {
			a.paused = false
			a.loading = true
			return a, a.refreshCurrentView()
		}

		// Global keys
		switch {
		case key.Matches(msg, theme.Keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, theme.Keys.Back):
			return a.goBack()
		case key.Matches(msg, theme.Keys.Refresh):
			a.loading = true
			return a, a.refreshCurrentView()
		case key.Matches(msg, theme.Keys.Enter):
			if a.state != viewLogSearch {
				return a.drillDown()
			}
		case key.Matches(msg, theme.Keys.Help):
			a.help.Active = true
			return a, nil
		case msg.String() == a.kb.SwitchRegion:
			a.regionPicker = views.NewRegionPicker(a.client.Region())
			return a, nil
		case msg.String() == a.kb.SwitchMode:
			a.modeSwitcher = NewModeSwitcher(a.modeTabs, a.mode)
			return a, nil
		case msg.String() == a.kb.EditConfig:
			return a.openConfigEditor()
		case msg.String() == a.kb.ReopenPicker:
			return a.reopenModePicker()
		}

		// Number keys 1-9: quick select and drill down on list views
		if s := msg.String(); len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
			idx := int(s[0] - '1')
			switch a.state {
			case viewClusters:
				if c := a.clusterView.SelectIndex(idx); c != nil {
					a.clusterView = a.clusterView.WithCursor(idx)
					a.selectedCluster = c
					a.state = viewServices
					a.serviceView = views.NewServiceList(c.Name)
					a.loading = true
					return a, a.loadServices()
				}
			}
		}

		// Context-specific keys (configurable via keybindings)
		k := msg.String()
		switch a.state {
		case viewServices:
			switch k {
			case a.kb.ForceRedeploy:
				return a.promptForceDeploy()
			case a.kb.Scale:
				return a.promptScale()
			case a.kb.ServiceDetail:
				return a.showServiceDetail()
			case a.kb.ServiceLogs:
				return a.openServiceLogs()
			case a.kb.StandaloneTasks:
				return a.showStandaloneTasks()
			case a.kb.Metrics:
				return a.showMetrics()
			}
		case viewTasks:
			switch k {
			case a.kb.StopTask:
				return a.promptStopTask()
			case a.kb.TaskLogs:
				return a.openTaskLogs()
			case a.kb.ECSExec:
				return a.execIntoTask()
			}
		case viewTaskDetail:
			switch k {
			case a.kb.EnvVars:
				return a.showEnvVars()
			}
		case viewLogs:
			switch k {
			case a.kb.LogSave:
				return a.promptSaveLogBuffer()
			case a.kb.LogCopy:
				return a.copyLogBufferToClipboard()
			case a.kb.LogOpenEditor:
				return a.openLogBufferInEditor()
			}
		case viewStandaloneTasks:
			switch k {
			case a.kb.TaskLogs:
				return a.openStandaloneTaskLogs()
			case a.kb.StopTask:
				return a.promptStopStandaloneTask()
			}
		case viewServiceDetail:
			switch k {
			case a.kb.Download:
				return a.showTaskDefDiff()
			}
		case viewSSM:
			switch k {
			case a.kb.Save:
				return a.saveSSMPrefix()
			case a.kb.EditValue:
				return a.editSSMParam()
			}
		case viewSecrets:
			switch k {
			case a.kb.Save:
				return a.saveSMFilter()
			case a.kb.EditValue:
				return a.editSecret()
			case a.kb.CloneSecret:
				return a.cloneSecret()
			case a.kb.CopyARN:
				return a.copySecretARN()
			}
		case viewS3Buckets:
			switch k {
			case a.kb.Save:
				return a.saveS3Search()
			}
		case viewS3Objects:
			switch k {
			case "i":
				return a.showS3ObjectDetail()
			case a.kb.Download:
				return a.promptS3Download()
			case a.kb.Search:
				return a.promptS3KeySearch()
			}
		case viewS3Detail:
			switch k {
			case a.kb.Download:
				return a.promptS3DownloadFromDetail()
			}
		case viewDynamoTables:
			switch k {
			case a.kb.Save:
				return a.saveDynamoTable()
			}
		case viewDynamoItems:
			switch k {
			case a.kb.FilterScan:
				return a.promptDynamoFilter()
			case a.kb.PartiQL:
				return a.promptDynamoPartiQL()
			case a.kb.Save:
				return a.saveDynamoQuery()
			case a.kb.NextPage:
				return a.loadDynamoNextPage()
			}
		case viewDynamoItemDetail:
			switch k {
			case a.kb.EditField:
				return a.editDynamoField()
			case a.kb.CloneItem:
				return a.cloneDynamoItem()
			}
		case viewSQSQueues:
			switch k {
			case a.kb.Save:
				return a.saveSQSQueue()
			}
		case viewSQSDetail:
			switch k {
			case a.kb.Messages:
				return a.openSQSMessages()
			case a.kb.SendMessage:
				return a.sendSQSMessage()
			case a.kb.NavigateDLQ:
				return a.openSQSDLQ()
			}
		case viewSQSMessages:
			switch k {
			case a.kb.PollMessages:
				return a.pollSQSMessages()
			case a.kb.DeleteMsg:
				return a.deleteSQSMessage()
			case a.kb.SendMessage:
				return a.sendSQSMessage()
			case "X":
				a.sqsMessagesView = a.sqsMessagesView.ClearMessages()
				return a, nil
			}
		case viewSQSMessageDetail:
			switch k {
			case a.kb.DeleteMsg:
				return a.deleteSQSMessage()
			case a.kb.CloneSend:
				return a.cloneSQSMessage()
			}
		case viewLambdaList:
			switch k {
			case a.kb.Save:
				return a.saveLambdaSearch()
			case a.kb.TailStream:
				return a.tailLambdaLogs()
			case a.kb.BrowseStreams:
				return a.browseLambdaLogs()
			case a.kb.Search:
				return a.searchLambdaLogs()
			}
		case viewLambdaDetail:
			switch k {
			case a.kb.TailStream:
				return a.tailLambdaDetailLogs()
			case a.kb.BrowseStreams:
				return a.browseLambdaDetailLogs()
			case a.kb.Search:
				return a.searchLambdaDetailLogs()
			case a.kb.EnvVars:
				return a.showLambdaEnvVars()
			case a.kb.EditCode:
				return a.editLambdaCode()
			}
		case viewLogGroups:
			switch k {
			case a.kb.TailStream:
				return a.tailLogGroup()
			case a.kb.Search:
				return a.promptLogSearchFromGroups()
			case a.kb.Save:
				if a.logGroupsView.SelectionCount() > 1 {
					groups := a.logGroupsView.SelectedGroups()
					a.logSearchGroups = groups
					return a.saveLogSearchGroups()
				}
				return a.saveLogGroupPath()
			}
		case viewLogStreams:
			switch k {
			case a.kb.TailStream:
				return a.tailLogStream()
			case a.kb.TailGroup:
				return a.tailEntireLogGroup()
			case a.kb.Search:
				return a.promptLogSearchFromStreams()
			case a.kb.Save:
				return a.saveLogStreamPath()
			}
		case viewAlarmDetail:
			switch k {
			case a.kb.ToggleActions:
				return a.toggleAlarmActions()
			case a.kb.SetAlarmState:
				return a.promptSetAlarmState()
			}
		case viewCBProjects:
			switch k {
			case a.kb.StartBuild:
				return a.triggerCBBuild()
			}
		case viewCBBuilds:
			switch k {
			case a.kb.StartBuild:
				return a.triggerCBBuild()
			}
		case viewCBBuildDetail:
			switch k {
			case a.kb.ViewLogs:
				return a.viewCBBuildLogs()
			case a.kb.Search:
				return a.searchCBBuildLogs()
			case a.kb.StartBuild:
				return a.triggerCBBuild()
			case a.kb.StopBuild:
				return a.stopCBBuild()
			}
		case viewTofuResources:
			switch k {
			case a.kb.RunPlan:
				return a.runTofuPlan()
			case a.kb.RunApply:
				return a.runTofuApply()
			case a.kb.RunInit:
				return a.runTofuInit()
			case a.kb.Save:
				return a.saveTofuDir()
			}
		case viewTofuPlan:
			switch k {
			case a.kb.RunApply:
				return a.runTofuApply()
			}
		case viewECRImages:
			switch k {
			case a.kb.StartScan:
				return a.startECRScan()
			case a.kb.DeleteImage:
				return a.deleteECRImage()
			case a.kb.CopyURI:
				return a.copyECRImageURI()
			}
		case viewR53Records:
			if k == a.kb.NewRecord {
				return a.createR53Record()
			}
		case viewR53RecordDetail:
			switch k {
			case a.kb.TestDNS:
				return a.testR53DNS()
			case a.kb.EditRecord:
				return a.editR53Record()
			case a.kb.DeleteRecord:
				return a.deleteR53Record()
			}
		case viewEC2Detail:
			switch k {
			case a.kb.SSMSession:
				return a.startEC2SSMSession()
			case a.kb.ConsoleOutput:
				return a.openEC2Console()
			case a.kb.StartInstance:
				return a.startEC2Instance()
			case a.kb.StopInstance:
				return a.stopEC2Instance()
			case a.kb.RebootInstance:
				return a.rebootEC2Instance()
			case a.kb.TermInstance:
				return a.terminateEC2Instance()
			}
		}

		return a.delegateToActiveView(msg)
	}

	return a, nil
}

// --- View Delegation ---

func (a App) delegateToActiveView(msg tea.KeyMsg) (App, tea.Cmd) {
	var cmd tea.Cmd
	switch a.state {
	case viewClusters:
		a.clusterView, cmd = a.clusterView.Update(msg)
	case viewServices:
		a.serviceView, cmd = a.serviceView.Update(msg)
	case viewTasks:
		a.taskView, cmd = a.taskView.Update(msg)
	case viewServiceDetail:
		a.serviceDetailView, cmd = a.serviceDetailView.Update(msg)
	case viewLogs:
		a.logView, cmd = a.logView.Update(msg)
	case viewStandaloneTasks:
		a.standaloneView, cmd = a.standaloneView.Update(msg)
	case viewTaskDefDiff:
		a.diffView, cmd = a.diffView.Update(msg)
	case viewSSM:
		a.ssmView, cmd = a.ssmView.Update(msg)
	case viewSecrets:
		a.secretsView, cmd = a.secretsView.Update(msg)
	case viewSecretValue:
		a.secretValueView, cmd = a.secretValueView.Update(msg)
	case viewS3Buckets:
		a.s3BucketsView, cmd = a.s3BucketsView.Update(msg)
	case viewS3Objects:
		a.s3ObjectsView, cmd = a.s3ObjectsView.Update(msg)
	case viewLambdaList:
		a.lambdaListView, cmd = a.lambdaListView.Update(msg)
	case viewDynamoTables:
		a.dynamoTablesView, cmd = a.dynamoTablesView.Update(msg)
	case viewDynamoItems:
		a.dynamoItemsView, cmd = a.dynamoItemsView.Update(msg)
	case viewDynamoItemDetail:
		a.dynamoDetailView, cmd = a.dynamoDetailView.Update(msg)
	case viewSQSQueues:
		a.sqsQueuesView, cmd = a.sqsQueuesView.Update(msg)
	case viewSQSMessages:
		a.sqsMessagesView, cmd = a.sqsMessagesView.Update(msg)
	case viewSQSMessageDetail:
		a.sqsMsgDetailView, cmd = a.sqsMsgDetailView.Update(msg)
	case viewEnvVars:
		a.envVarsView, cmd = a.envVarsView.Update(msg)
	case viewLogGroups:
		a.logGroupsView, cmd = a.logGroupsView.Update(msg)
	case viewLogStreams:
		a.logStreamsView, cmd = a.logStreamsView.Update(msg)
	case viewLogSearch:
		a.logSearchView, cmd = a.logSearchView.Update(msg)
	case viewAlarms:
		a.alarmsView, cmd = a.alarmsView.Update(msg)
	case viewAlarmDetail:
		a.alarmDetailView, cmd = a.alarmDetailView.Update(msg)
	case viewCBProjects:
		a.cbProjectsView, cmd = a.cbProjectsView.Update(msg)
	case viewCBBuilds:
		a.cbBuildsView, cmd = a.cbBuildsView.Update(msg)
	case viewCBBuildDetail:
		a.cbBuildDetailView, cmd = a.cbBuildDetailView.Update(msg)
	case viewTofuResources:
		a.tofuResourcesView, cmd = a.tofuResourcesView.Update(msg)
	case viewTofuStateDetail:
		a.tofuStateDetailView, cmd = a.tofuStateDetailView.Update(msg)
	case viewTofuPlan:
		a.tofuPlanView, cmd = a.tofuPlanView.Update(msg)
	case viewTofuPlanDetail:
		a.tofuPlanDetailView, cmd = a.tofuPlanDetailView.Update(msg)
	case viewECRRepos:
		a.ecrReposView, cmd = a.ecrReposView.Update(msg)
	case viewECRImages:
		a.ecrImagesView, cmd = a.ecrImagesView.Update(msg)
	case viewECRFindings:
		a.ecrFindingsView, cmd = a.ecrFindingsView.Update(msg)
	case viewR53Zones:
		a.r53ZonesView, cmd = a.r53ZonesView.Update(msg)
	case viewR53Records:
		a.r53RecordsView, cmd = a.r53RecordsView.Update(msg)
	case viewR53RecordDetail:
		a.r53DetailView, cmd = a.r53DetailView.Update(msg)
	case viewEC2Instances:
		a.ec2InstancesView, cmd = a.ec2InstancesView.Update(msg)
	case viewEC2Detail:
		a.ec2DetailView, cmd = a.ec2DetailView.Update(msg)
	case viewEC2Console:
		a.ec2ConsoleView, cmd = a.ec2ConsoleView.Update(msg)
	}
	return a, cmd
}

func (a App) isFiltering() bool {
	switch a.state {
	case viewClusters:
		return a.clusterView.IsFiltering()
	case viewServices:
		return a.serviceView.IsFiltering()
	case viewTasks:
		return a.taskView.IsFiltering()
	case viewStandaloneTasks:
		return a.standaloneView.IsFiltering()
	case viewSSM:
		return a.ssmView.IsFiltering()
	case viewSecrets:
		return a.secretsView.IsFiltering()
	case viewS3Buckets:
		return a.s3BucketsView.IsFiltering()
	case viewS3Objects:
		return a.s3ObjectsView.IsFiltering()
	case viewLambdaList:
		return a.lambdaListView.IsFiltering()
	case viewDynamoTables:
		return a.dynamoTablesView.IsFiltering()
	case viewSQSQueues:
		return a.sqsQueuesView.IsFiltering()
	case viewEnvVars:
		return a.envVarsView.IsFiltering()
	case viewLogGroups:
		return a.logGroupsView.IsFiltering()
	case viewLogStreams:
		return a.logStreamsView.IsFiltering()
	case viewLogs:
		return a.logView.IsFiltering()
	case viewAlarms:
		return a.alarmsView.IsFiltering()
	case viewCBProjects:
		return a.cbProjectsView.IsFiltering()
	case viewEC2Instances:
		return a.ec2InstancesView.IsFiltering()
	case viewTofuResources:
		return a.tofuResourcesView.IsFiltering()
	case viewTofuPlan:
		return a.tofuPlanView.IsFiltering()
	case viewECRRepos:
		return a.ecrReposView.IsFiltering()
	case viewECRImages:
		return a.ecrImagesView.IsFiltering()
	case viewECRFindings:
		return a.ecrFindingsView.IsFiltering()
	case viewR53Zones:
		return a.r53ZonesView.IsFiltering()
	case viewR53Records:
		return a.r53RecordsView.IsFiltering()
	}
	return false
}

// --- View ---

func (a App) buildBreadcrumbs() []string {
	var crumbs []string
	if a.selectedCluster != nil {
		crumbs = append(crumbs, a.selectedCluster.Name)
	}
	if a.selectedService != nil {
		crumbs = append(crumbs, a.selectedService.Name)
	}
	if a.selectedTask != nil {
		id := a.selectedTask.TaskID
		if len(id) > 8 {
			id = id[:8]
		}
		crumbs = append(crumbs, id)
	}
	return crumbs
}

func (a App) View() string {
	breadcrumbs := a.buildBreadcrumbs()
	infoBar := buildInfoBar(breadcrumbs, a.client.Region(), a.lastRefresh,
		a.paused, a.flashMessage, a.flashExpiry, a.err)

	var content string
	switch a.state {
	case viewClusters:
		content = a.clusterView.View()
	case viewServices:
		content = a.serviceView.View()
	case viewTasks:
		content = a.taskView.View()
	case viewTaskDetail:
		content = a.detailView.View()
	case viewServiceDetail:
		content = a.serviceDetailView.View()
	case viewLogs:
		content = a.logView.View()
	case viewStandaloneTasks:
		content = a.standaloneView.View()
	case viewTaskDefDiff:
		content = a.diffView.View()
	case viewMetrics:
		content = a.metricsView.View()
	case viewSSM:
		content = a.ssmView.View()
	case viewSecrets:
		content = a.secretsView.View()
	case viewSecretValue:
		content = a.secretValueView.View()
	case viewS3Buckets:
		content = a.s3BucketsView.View()
	case viewS3Objects:
		content = a.s3ObjectsView.View()
	case viewS3Detail:
		content = a.s3DetailView.View()
	case viewLambdaList:
		content = a.lambdaListView.View()
	case viewLambdaDetail:
		content = a.lambdaDetailView.View()
	case viewDynamoTables:
		content = a.dynamoTablesView.View()
	case viewDynamoItems:
		content = a.dynamoItemsView.View()
	case viewDynamoItemDetail:
		content = a.dynamoDetailView.View()
	case viewSQSQueues:
		content = a.sqsQueuesView.View()
	case viewSQSDetail:
		content = a.sqsDetailView.View()
	case viewSQSMessages:
		content = a.sqsMessagesView.View()
	case viewSQSMessageDetail:
		content = a.sqsMsgDetailView.View()
	case viewEnvVars:
		content = a.envVarsView.View()
	case viewLogGroups:
		content = a.logGroupsView.View()
	case viewLogStreams:
		content = a.logStreamsView.View()
	case viewLogSearch:
		content = a.logSearchView.View()
	case viewAlarms:
		content = a.alarmsView.View()
	case viewAlarmDetail:
		content = a.alarmDetailView.View()
	case viewCBProjects:
		content = a.cbProjectsView.View()
	case viewCBBuilds:
		content = a.cbBuildsView.View()
	case viewCBBuildDetail:
		content = a.cbBuildDetailView.View()
	case viewTofuResources:
		content = a.tofuResourcesView.View()
	case viewTofuStateDetail:
		content = a.tofuStateDetailView.View()
	case viewTofuPlan:
		content = a.tofuPlanView.View()
	case viewTofuPlanDetail:
		content = a.tofuPlanDetailView.View()
	case viewECRRepos:
		content = a.ecrReposView.View()
	case viewECRImages:
		content = a.ecrImagesView.View()
	case viewECRFindings:
		content = a.ecrFindingsView.View()
	case viewR53Zones:
		content = a.r53ZonesView.View()
	case viewR53Records:
		content = a.r53RecordsView.View()
	case viewR53RecordDetail:
		content = a.r53DetailView.View()
	case viewEC2Instances:
		content = a.ec2InstancesView.View()
	case viewEC2Detail:
		content = a.ec2DetailView.View()
	case viewEC2Console:
		content = a.ec2ConsoleView.View()
	}

	helpLine := a.helpText()
	// Wrap help to frame inner width (width - 4 for borders and padding)
	actionBar := wrapText(helpLine, a.width-4)

	modeLabel := modeShortName(a.mode)
	fullView := renderFrame(a.width, a.height, infoBar, content, actionBar, modeLabel)

	// Overlay modal dialogs on top of the frame
	if a.help.Active {
		helpContent := RenderHelp(a.contextHelpLines(), a.width-10)
		return renderOverlay(fullView, helpContent, a.width, a.height)
	}
	if a.modeSwitcher.Active {
		return renderOverlay(fullView, a.modeSwitcher.View(), a.width, a.height)
	}
	if a.regionPicker.Active {
		return renderOverlay(fullView, a.regionPicker.View(), a.width, a.height)
	}
	if a.confirm.Active {
		return renderOverlay(fullView, a.confirm.View(), a.width, a.height)
	}
	if a.picker.Active {
		return renderOverlay(fullView, a.picker.View(), a.width, a.height)
	}
	if a.input.Active {
		return renderOverlay(fullView, a.input.View(), a.width, a.height)
	}
	if a.pathInput != nil {
		return renderOverlay(fullView, a.pathInput.View(), a.width, a.height)
	}

	return fullView
}

func maxLineWidth(s string) int {
	widest := 0
	for line := range strings.SplitSeq(s, "\n") {
		widest = max(widest, lipgloss.Width(line))
	}
	return widest
}

func wrapText(s string, maxWidth int) string {
	if maxWidth <= 0 || len(s) <= maxWidth {
		return s
	}
	s = strings.TrimSpace(s)
	parts := strings.Split(s, "  [")
	if len(parts) <= 1 {
		return "  " + s
	}
	var chunks []string
	chunks = append(chunks, strings.TrimSpace(parts[0]))
	for _, p := range parts[1:] {
		chunks = append(chunks, "["+strings.TrimSpace(p))
	}
	var lines []string
	line := "  "
	for _, chunk := range chunks {
		candidate := line + "  " + chunk
		if line == "  " {
			candidate = line + chunk
		}
		if len(candidate) > maxWidth && line != "  " {
			lines = append(lines, line)
			line = "  " + chunk
		} else {
			line = candidate
		}
	}
	if line != "  " {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func (a App) helpText() string {
	// Slim footer: primary action + esc + quit + help
	primary := ""
	switch a.state {
	case viewClusters:
		primary = "[enter] services"
	case viewServices:
		primary = "[enter] tasks"
	case viewTasks, viewStandaloneTasks:
		primary = "[enter] detail"
	case viewTaskDetail:
		primary = "[E] env vars"
	case viewServiceDetail:
		primary = "[tab] switch tab"
	case viewLogs:
		primary = "[f] follow"
	case viewTaskDefDiff, viewSecretValue:
		primary = "[j/k] scroll"
	case viewMetrics:
		primary = "[R] refresh"
	case viewSSM, viewSecrets:
		primary = "[enter] view"
	case viewS3Buckets, viewDynamoTables, viewSQSQueues, viewLambdaList, viewLogGroups:
		primary = "[enter] select"
	case viewS3Objects:
		primary = "[enter] open"
	case viewS3Detail:
		primary = "[D] download"
	case viewLambdaDetail:
		primary = "[c] edit code"
	case viewDynamoItems:
		primary = "[enter] detail"
	case viewDynamoItemDetail:
		primary = "[e] edit"
	case viewSQSDetail:
		primary = "[m] messages"
	case viewSQSMessages:
		primary = "[p] poll"
	case viewSQSMessageDetail:
		primary = "[c] clone & send"
	case viewEnvVars:
		primary = "[a] toggle ARNs"
	case viewLogStreams:
		primary = "[enter] peek"
	case viewLogSearch:
		primary = "[enter] jump"
	case viewAlarms:
		primary = "[enter] detail"
	case viewAlarmDetail:
		primary = "[a] toggle actions"
	case viewCBProjects:
		primary = "[enter] builds"
	case viewCBBuilds:
		primary = "[enter] detail"
	case viewCBBuildDetail:
		primary = "[l] view logs"
	case viewTofuResources:
		primary = "[p] plan"
	case viewTofuStateDetail:
		primary = "[j/k] scroll"
	case viewTofuPlan:
		primary = "[enter] detail"
	case viewTofuPlanDetail:
		primary = "[j/k] scroll"
	case viewECRRepos:
		primary = "[enter] images"
	case viewECRImages:
		primary = "[enter] scan findings"
	case viewECRFindings:
		primary = "[/] filter"
	case viewR53Zones:
		primary = "[enter] records"
	case viewR53Records:
		primary = "[enter] detail"
	case viewR53RecordDetail:
		primary = "[t] test DNS"
	case viewEC2Instances:
		primary = "[enter] detail"
	case viewEC2Detail:
		primary = "[e] SSM session"
	case viewEC2Console:
		primary = "[j/k] scroll"
	}
	if primary != "" {
		return fmt.Sprintf("  %s  [esc] back  [q] quit  [?] help", primary)
	}
	return "  [esc] back  [q] quit  [?] help"
}

// contextHelpLines returns the full keybinding list for the current view,
// used by the help overlay.
func (a App) contextHelpLines() []struct{ key, desc string } {
	type kv = struct{ key, desc string }

	global := []kv{
		{a.kb.SwitchMode, "Switch mode"},
		{a.kb.ReopenPicker, "Reopen mode picker"},
		{a.kb.PauseResume, "Pause/resume polling"},
		{a.kb.EditConfig, "Edit config"},
		{a.kb.SwitchRegion, "Switch AWS region"},
		{"R", "Refresh data"},
		{"/", "Filter/search"},
		{"esc", "Go back"},
		{"q", "Quit"},
		{"?", "Toggle this help"},
	}

	var context []kv
	kb := a.kb
	switch a.state {
	case viewClusters:
		context = []kv{
			{"enter", "Browse services"},
			{"1-9", "Quick select"},
			{"/", "Filter"},
			{"R", "Refresh"},
		}
	case viewServices:
		context = []kv{
			{"enter", "Browse tasks"},
			{kb.ServiceDetail, "Service detail (deployments + events)"},
			{kb.ServiceLogs, "Tail logs (all tasks)"},
			{kb.ForceRedeploy, "Force new deployment"},
			{kb.Scale, "Scale service"},
			{kb.Metrics, "CPU/memory metrics + alarms"},
			{kb.StandaloneTasks, "Standalone tasks (non-service)"},
		}
	case viewTasks:
		context = []kv{
			{"enter", "Task detail"},
			{kb.TaskLogs, "Tail logs"},
			{kb.StopTask, "Stop task"},
			{kb.ECSExec, "ECS Exec (shell into container)"},
		}
	case viewTaskDetail:
		context = []kv{
			{kb.EnvVars, "View environment variables"},
		}
	case viewServiceDetail:
		context = []kv{
			{"tab", "Switch between Deployments and Events"},
			{kb.Download, "Task definition diff"},
			{"j/k", "Scroll"},
		}
	case viewLogs:
		context = []kv{
			{kb.LogFollow, "Toggle follow mode"},
			{kb.LogTimestamp, "Cycle timestamps (relative/local/UTC)"},
			{"/", "Search — jump with n/N"},
			{"n/N", "Next/previous match"},
			{kb.LogOlder + "/" + kb.LogNewer, "Load older/newer logs"},
			{kb.LogSave, "Save buffer to file"},
			{kb.LogCopy, "Copy buffer to clipboard"},
			{kb.LogOpenEditor, "Open buffer in $EDITOR"},
			{"g/G", "Jump to top/bottom"},
			{"PgUp/PgDn", "Scroll by page"},
		}
	case viewStandaloneTasks:
		context = []kv{
			{"enter", "Task detail"},
			{kb.TaskLogs, "Tail logs"},
			{kb.StopTask, "Stop task"},
		}
	case viewTaskDefDiff:
		context = []kv{
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewMetrics:
		context = []kv{
			{"R", "Refresh metrics"},
		}
	case viewSSM:
		context = []kv{
			{"enter", "View parameter value"},
			{kb.EditValue, "Edit parameter value"},
			{kb.Save, "Save prefix for quick access"},
		}
	case viewSecrets:
		context = []kv{
			{"enter", "View secret value"},
			{kb.EditValue, "Edit secret value"},
			{kb.CloneSecret, "Clone secret to new name"},
			{kb.CopyARN, "Copy ARN to clipboard"},
			{kb.Save, "Save filter for quick access"},
		}
	case viewSecretValue:
		context = []kv{
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewS3Buckets:
		context = []kv{
			{"enter", "Browse bucket"},
			{kb.Save, "Save search"},
		}
	case viewS3Objects:
		context = []kv{
			{"enter", "Open folder / view detail"},
			{"i", "View detail + tags"},
			{kb.Search, "Search by key prefix"},
			{kb.Download, "Download object or folder"},
		}
	case viewS3Detail:
		context = []kv{
			{kb.Download, "Download"},
		}
	case viewLambdaList:
		context = []kv{
			{"enter", "Function detail"},
			{kb.TailStream, "Tail function logs (live)"},
			{kb.BrowseStreams, "Browse log streams (historical)"},
			{kb.Search, "Search function logs"},
			{kb.Save, "Save search"},
		}
	case viewLambdaDetail:
		context = []kv{
			{kb.EditCode, "Edit code (download, $EDITOR, deploy)"},
			{kb.EnvVars, "View environment variables"},
			{kb.TailStream, "Tail logs (live)"},
			{kb.BrowseStreams, "Browse log streams (historical)"},
			{kb.Search, "Search logs"},
		}
	case viewDynamoTables:
		context = []kv{
			{"enter", "Browse table"},
			{kb.Save, "Save table"},
		}
	case viewDynamoItems:
		context = []kv{
			{"enter", "View item detail"},
			{kb.FilterScan, "Filter scan (attribute + operator + value)"},
			{kb.PartiQL, "PartiQL query"},
			{kb.NextPage, "Load next page"},
			{kb.Save, "Save PartiQL query"},
		}
	case viewDynamoItemDetail:
		context = []kv{
			{kb.EditField, "Edit field value"},
			{kb.CloneItem, "Clone item"},
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewSQSQueues:
		context = []kv{
			{"enter", "Queue detail"},
			{kb.Save, "Save queue"},
		}
	case viewSQSDetail:
		context = []kv{
			{kb.Messages, "Browse messages"},
			{kb.SendMessage, "Send new message"},
			{kb.NavigateDLQ, "Navigate to dead letter queue"},
			{"R", "Refresh stats"},
		}
	case viewSQSMessages:
		context = []kv{
			{"enter", "Message detail"},
			{kb.PollMessages, "Poll for messages"},
			{kb.SendMessage, "Send new message"},
			{kb.DeleteMsg, "Delete message"},
			{"X", "Clear message list"},
		}
	case viewSQSMessageDetail:
		context = []kv{
			{kb.CloneSend, "Clone & send"},
			{kb.DeleteMsg, "Delete message"},
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewEnvVars:
		context = []kv{
			{"a", "Toggle ARN/resolved values"},
		}
	case viewLogGroups:
		context = []kv{
			{"enter", "Browse streams"},
			{"space", "Multi-select for search"},
			{kb.TailStream, "Tail entire log group"},
			{kb.Search, "Search selected groups"},
			{kb.Save, "Save log path / group selection"},
		}
	case viewLogStreams:
		context = []kv{
			{"enter", "Peek (last 1 min, paused)"},
			{"space", "Multi-select for search"},
			{kb.TailStream, "Tail stream"},
			{kb.TailGroup, "Tail entire log group"},
			{kb.Search, "Search selected streams"},
			{kb.Save, "Save log path"},
		}
	case viewLogSearch:
		context = []kv{
			{"enter", "Jump to log at timestamp"},
			{kb.Timestamp, "Toggle timestamps"},
			{"g/G", "Top/bottom"},
		}
	case viewAlarms:
		context = []kv{
			{"enter", "View alarm detail"},
			{kb.Timestamp, "Toggle local/UTC timestamps"},
			{"/", "Filter alarms"},
		}
	case viewAlarmDetail:
		context = []kv{
			{kb.ToggleActions, "Enable/disable alarm actions"},
			{kb.SetAlarmState, "Set alarm state (testing)"},
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewCBProjects:
		context = []kv{
			{"enter", "View builds"},
			{kb.StartBuild, "Start new build"},
			{"/", "Filter projects"},
		}
	case viewCBBuilds:
		context = []kv{
			{"enter", "View build detail"},
			{kb.StartBuild, "Start new build"},
			{kb.Timestamp, "Toggle local/UTC timestamps"},
		}
	case viewCBBuildDetail:
		context = []kv{
			{kb.ViewLogs, "View build logs (buffer)"},
			{kb.Search, "Search build logs (full)"},
			{kb.StartBuild, "Start new build"},
			{kb.StopBuild, "Stop build (if in progress)"},
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewTofuResources:
		context = []kv{
			{"enter", "View resource state"},
			{kb.RunPlan, "Run plan (parsed view)"},
			{kb.RunApply, "Run apply (interactive)"},
			{kb.RunInit, "Run init"},
			{kb.Save, "Save workspace"},
			{"/", "Filter resources"},
		}
	case viewTofuPlan:
		context = []kv{
			{"enter", "View change detail"},
			{kb.RunApply, "Run apply (interactive)"},
			{"/", "Filter changes"},
		}
	case viewTofuPlanDetail:
		context = []kv{
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewTofuStateDetail:
		context = []kv{
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewECRRepos:
		context = []kv{
			{"enter", "View images"},
			{"/", "Filter repositories"},
		}
	case viewECRImages:
		context = []kv{
			{"enter", "View scan findings"},
			{kb.StartScan, "Start scan"},
			{kb.CopyURI, "Copy image URI to clipboard"},
			{kb.DeleteImage, "Delete image"},
			{"/", "Filter by tag or digest"},
		}
	case viewECRFindings:
		context = []kv{
			{"/", "Filter findings"},
		}
	case viewR53Zones:
		context = []kv{
			{"enter", "View records"},
			{"/", "Filter zones"},
		}
	case viewR53Records:
		context = []kv{
			{"enter", "View record detail"},
			{kb.NewRecord, "Create new record"},
			{"/", "Filter records"},
		}
	case viewR53RecordDetail:
		context = []kv{
			{kb.TestDNS, "Test DNS resolution"},
			{kb.EditRecord, "Edit record"},
			{kb.DeleteRecord, "Delete record"},
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewEC2Instances:
		context = []kv{
			{"enter", "View instance detail"},
			{"/", "Filter instances"},
		}
	case viewEC2Detail:
		context = []kv{
			{kb.SSMSession, "SSM session (shell into instance)"},
			{kb.ConsoleOutput, "View console output"},
			{kb.StartInstance, "Start instance"},
			{kb.StopInstance, "Stop instance"},
			{kb.RebootInstance, "Reboot instance"},
			{kb.TermInstance, "Terminate instance"},
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewEC2Console:
		context = []kv{
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	}

	// Combine: context first, then separator, then global
	var all []kv
	if len(context) > 0 {
		all = append(all, context...)
		all = append(all, kv{"", ""})
	}
	all = append(all, global...)
	return all
}

// --- Navigation ---

func (a App) drillDown() (App, tea.Cmd) {
	switch a.state {
	case viewClusters:
		if c := a.clusterView.SelectedCluster(); c != nil {
			a.selectedCluster = c
			a.state = viewServices
			a.serviceView = views.NewServiceList(c.Name)
			a.loading = true
			return a, a.loadServices()
		}
	case viewServices:
		if s := a.serviceView.SelectedService(); s != nil {
			a.selectedService = s
			a.state = viewTasks
			a.taskView = views.NewTaskList(s.Name)
			a.loading = true
			return a, a.loadTasks()
		}
	case viewTasks:
		if t := a.taskView.SelectedTask(); t != nil {
			a.selectedTask = t
			a.state = viewTaskDetail
			a.detailView = views.NewTaskDetail(t)
			return a, nil
		}
	case viewStandaloneTasks:
		if t := a.standaloneView.SelectedTask(); t != nil {
			a.selectedTask = t
			a.state = viewTaskDetail
			a.detailView = views.NewTaskDetail(t)
			return a, nil
		}
	case viewSSM:
		if p := a.ssmView.SelectedParam(); p != nil {
			a.flashMessage = fmt.Sprintf("%s = %s", p.Name, p.Value)
			a.flashExpiry = time.Now().Add(10 * time.Second)
			return a, nil
		}
	case viewSecrets:
		if s := a.secretsView.SelectedSecret(); s != nil {
			return a, a.fetchSecretValue(s.Name, s.Tags)
		}
	case viewS3Buckets:
		if bkt := a.s3BucketsView.SelectedBucket(); bkt != nil {
			return a.openS3Objects(bkt.Name, "")
		}
	case viewS3Objects:
		if obj := a.s3ObjectsView.SelectedObject(); obj != nil {
			if obj.IsPrefix {
				return a.openS3Objects(a.s3ObjectsView.Bucket(), obj.Key)
			}
			return a, a.loadS3Detail(a.s3ObjectsView.Bucket(), obj.Key)
		}
	case viewSQSQueues:
		if q := a.sqsQueuesView.SelectedQueue(); q != nil {
			return a.openSQSDetail(q.Name, q.URL)
		}
	case viewSQSMessages:
		if msg := a.sqsMessagesView.SelectedMessage(); msg != nil {
			a.state = viewSQSMessageDetail
			qn := a.sqsMessagesView.QueueName()
			qu := a.sqsMessagesView.QueueURL()
			a.sqsMsgDetailView = views.NewSQSMessageDetail(qn, qu, msg)
			a.sqsMsgDetailView = a.sqsMsgDetailView.SetSize(a.width-3, a.height-6)
			return a, nil
		}
	case viewDynamoTables:
		table := a.dynamoTablesView.SelectedTable()
		if table != "" {
			return a.scanDynamoTable(table)
		}
	case viewDynamoItems:
		if item := a.dynamoItemsView.SelectedItem(); item != nil {
			a.state = viewDynamoItemDetail
			a.dynamoDetailView = views.NewDynamoItemDetail(a.dynamoItemsView.TableName(), a.dynamoKeyNames, item)
			a.dynamoDetailView = a.dynamoDetailView.SetSize(a.width, a.height-3)
			return a, nil
		}
	case viewLambdaList:
		if fn := a.lambdaListView.SelectedFunction(); fn != nil {
			a.state = viewLambdaDetail
			a.lambdaDetailView = views.NewLambdaDetail(fn)
			a.lambdaDetailView = a.lambdaDetailView.SetSize(a.width, a.height-3)
			return a, nil
		}
	case viewLogGroups:
		if g := a.logGroupsView.SelectedGroup(); g != nil {
			return a.openLogStreams(g.Name)
		}
	case viewLogStreams:
		return a.peekLogStream()
	case viewAlarms:
		return a.openAlarmDetail()
	case viewCBProjects:
		if p := a.cbProjectsView.SelectedProject(); p != nil {
			return a.openCBBuilds(p.Name)
		}
	case viewCBBuilds:
		return a.openCBBuildDetail()
	case viewTofuResources:
		return a.openTofuStateDetail()
	case viewTofuPlan:
		return a.openTofuPlanDetail()
	case viewECRRepos:
		if r := a.ecrReposView.SelectedRepo(); r != nil {
			return a.openECRImages(r.Name, r.URI)
		}
	case viewECRImages:
		return a.openECRFindings()
	case viewR53Zones:
		if z := a.r53ZonesView.SelectedZone(); z != nil {
			return a.openR53Records(z.Name, z.ID)
		}
	case viewR53Records:
		return a.openR53RecordDetail()
	case viewEC2Instances:
		return a.openEC2Detail()
	}
	return a, nil
}

// reopenModePicker re-launches the current mode's entry picker/prompt.
func (a App) reopenModePicker() (App, tea.Cmd) {
	switch a.mode {
	case modeECS:
		a.state = viewClusters
		a.selectedCluster = nil
		a.selectedService = nil
		a.selectedTask = nil
		a.loading = true
		return a, a.loadClusters()
	case modeCWLogs:
		return a.promptCloudWatchBrowser()
	case modeCWAlarms:
		return a.promptCWAlarmsBrowser()
	case modeSSM:
		return a.promptSSMPath()
	case modeSM:
		return a.promptSMFilter()
	case modeS3:
		return a.promptS3Browser()
	case modeLambda:
		return a.promptLambdaBrowser()
	case modeDynamoDB:
		return a.promptDynamoBrowser()
	case modeSQS:
		return a.promptSQSBrowser()
	case modeCodeBuild:
		return a.openCBProjects()
	case modeEC2:
		return a.openEC2Instances("")
	case modeECR:
		return a.openECRRepos()
	case modeTofu:
		return a.promptTofuBrowser()
	case modeRoute53:
		return a.openR53Zones()
	}
	return a, nil
}

func (a App) switchMode(mode topMode) (App, tea.Cmd) {
	if mode == a.mode {
		return a, nil
	}
	a.mode = mode
	switch mode {
	case modeECS:
		a.state = viewClusters
		a.selectedCluster = nil
		a.selectedService = nil
		a.selectedTask = nil
		a.loading = true
		return a, a.loadClusters()
	case modeCWLogs:
		return a.promptCloudWatchBrowser()
	case modeCWAlarms:
		return a.promptCWAlarmsBrowser()
	case modeSSM:
		return a.promptSSMPath()
	case modeSM:
		return a.promptSMFilter()
	case modeS3:
		return a.promptS3Browser()
	case modeLambda:
		return a.promptLambdaBrowser()
	case modeDynamoDB:
		return a.promptDynamoBrowser()
	case modeSQS:
		return a.promptSQSBrowser()
	case modeCodeBuild:
		return a.openCBProjects()
	case modeEC2:
		return a.openEC2Instances("")
	case modeECR:
		return a.openECRRepos()
	case modeTofu:
		return a.promptTofuBrowser()
	case modeRoute53:
		return a.openR53Zones()
	}
	return a, nil
}

func (a App) showModePicker() (App, tea.Cmd) {
	a.modeSwitcher = NewModeSwitcher(a.modeTabs, a.mode)
	return a, nil
}

func (a App) goBack() (App, tea.Cmd) {
	switch a.state {
	case viewClusters:
		// Root of ECS — show mode picker
		return a.showModePicker()
	case viewServices:
		a.state = viewClusters
		a.selectedCluster = nil
		a.loading = true
		return a, a.loadClusters()
	case viewTasks:
		a.state = viewServices
		a.selectedService = nil
		a.loading = true
		return a, a.loadServices()
	case viewTaskDetail:
		if a.prevState == viewStandaloneTasks {
			a.state = viewStandaloneTasks
		} else {
			a.state = viewTasks
		}
		a.selectedTask = nil
		return a, nil
	case viewServiceDetail:
		a.state = viewServices
		a.selectedService = nil
		return a, nil
	case viewLogs:
		if a.prevState == viewLogSearch {
			a.state = viewLogSearch
			return a, nil
		}
		if a.prevState == viewLambdaList {
			a.state = viewLambdaList
			return a, nil
		}
		if a.prevState == viewLambdaDetail {
			a.state = viewLambdaDetail
			return a, nil
		}
		if a.prevState == viewCBBuildDetail {
			a.state = viewCBBuildDetail
			return a, nil
		}
		if a.prevState == viewLogStreams {
			a.state = viewLogStreams
			return a, nil
		}
		if a.prevState == viewLogGroups && a.logGroupsView.HasData() {
			a.state = viewLogGroups
			return a, nil
		}
		if a.selectedTask != nil {
			if a.prevState == viewStandaloneTasks {
				a.state = viewStandaloneTasks
			} else {
				a.state = viewTasks
			}
			return a, nil
		}
		a.state = viewServices
		return a, nil
	case viewStandaloneTasks:
		a.state = viewServices
		return a, nil
	case viewTaskDefDiff:
		a.state = viewServiceDetail
		return a, nil
	case viewMetrics:
		a.state = viewServices
		return a, nil
	case viewSSM:
		return a.showModePicker()
	case viewSecrets:
		return a.showModePicker()
	case viewSecretValue:
		a.state = viewSecrets
		return a, nil
	case viewS3Buckets:
		return a.showModePicker()
	case viewS3Objects:
		parent := a.s3ObjectsView.ParentPrefix()
		if parent != "" || a.s3ObjectsView.Prefix() != "" {
			return a.openS3Objects(a.s3ObjectsView.Bucket(), parent)
		}
		a.state = viewS3Buckets
		return a, nil
	case viewS3Detail:
		a.state = viewS3Objects
		return a, nil
	case viewLambdaList:
		return a.showModePicker()
	case viewLambdaDetail:
		a.state = viewLambdaList
		return a, nil
	case viewSQSQueues:
		return a.showModePicker()
	case viewSQSDetail:
		a.state = viewSQSQueues
		return a, nil
	case viewSQSMessages:
		a.state = viewSQSDetail
		return a, nil
	case viewSQSMessageDetail:
		a.state = viewSQSMessages
		return a, nil
	case viewDynamoTables:
		return a.showModePicker()
	case viewDynamoItems:
		a.state = viewDynamoTables
		return a, nil
	case viewDynamoItemDetail:
		a.state = viewDynamoItems
		return a, nil
	case viewEnvVars:
		if a.prevState == viewLambdaDetail {
			a.state = viewLambdaDetail
		} else {
			a.state = viewTaskDetail
		}
		return a, nil
	case viewLogGroups:
		return a.showModePicker()
	case viewLogStreams:
		if a.logGroupsView.HasData() {
			a.state = viewLogGroups
			return a, nil
		}
		return a.showModePicker()
	case viewLogSearch:
		if a.prevState == viewLogStreams {
			a.state = viewLogStreams
			return a, nil
		}
		if a.prevState == viewLogGroups && a.logGroupsView.HasData() {
			a.state = viewLogGroups
			return a, nil
		}
		if a.prevState == viewLambdaList {
			a.state = viewLambdaList
			return a, nil
		}
		if a.prevState == viewLambdaDetail {
			a.state = viewLambdaDetail
			return a, nil
		}
		return a.showModePicker()
	case viewAlarms:
		return a.showModePicker()
	case viewAlarmDetail:
		a.state = viewAlarms
		return a, nil
	case viewCBProjects:
		return a.showModePicker()
	case viewCBBuilds:
		a.state = viewCBProjects
		return a, nil
	case viewCBBuildDetail:
		a.state = viewCBBuilds
		return a, nil
	case viewTofuResources:
		return a.showModePicker()
	case viewTofuStateDetail:
		a.state = viewTofuResources
		return a, nil
	case viewTofuPlan:
		a.state = viewTofuResources
		return a, nil
	case viewTofuPlanDetail:
		a.state = viewTofuPlan
		return a, nil
	case viewECRRepos:
		return a.showModePicker()
	case viewECRImages:
		a.state = viewECRRepos
		return a, nil
	case viewECRFindings:
		a.state = viewECRImages
		return a, nil
	case viewR53Zones:
		return a.showModePicker()
	case viewR53Records:
		a.state = viewR53Zones
		return a, nil
	case viewR53RecordDetail:
		a.state = viewR53Records
		return a, nil
	case viewEC2Instances:
		return a.showModePicker()
	case viewEC2Detail:
		a.state = viewEC2Instances
		return a, nil
	case viewEC2Console:
		a.state = viewEC2Detail
		return a, nil
	}
	return a, nil
}
