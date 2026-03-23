// Package ui implements the bubbletea TUI application, views, and modal dialogs.
package ui

import (
	"fmt"
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
	regionPicker      views.RegionPickerModel

	// Navigation context
	selectedCluster    *model.Cluster
	selectedService    *model.Service
	selectedTask       *model.Task
	execContainerName  string
	logSearchGroup     string
	logSearchGroups    []string // multi-group search
	logSearchStream    string
	logSearchStartMs   int64
	logSearchEndMs     int64
	logSaveGroup       string
	logSaveStream      string
	ssmEditName        string
	ssmEditValue       string
	smEditName         string
	smEditValue        string
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
	refreshSec   int
	loading      bool
	err          error
	flashMessage   string
	flashExpiry    time.Time
	configModTime  time.Time // last known config file mod time
	width          int
	height         int
}

func NewApp(client *e9saws.Client, cfg *config.Config, defaultCluster string, refreshSec int) App {
	app := App{
		client:      client,
		cfg:         cfg,
		state:       viewClusters,
		clusterView: views.NewClusterList(),
		refreshSec:  refreshSec,
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
		lookback := 5 * time.Minute
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
						a.logSearchView.Pattern(), a.logSearchStream,
						a.logSearchStartMs, a.logSearchEndMs)
					return a, tea.Batch(viewCmd, nextCmd)
				}
			}
			// For single-group paginated search, chain next page if not done
			if !msg.Done && len(a.logSearchGroups) <= 1 && msg.NextToken != nil {
				nextCmd := searchGroupPaginated(a.client, msg.Source, a.logSearchStream,
					a.logSearchView.Pattern(), a.logSearchStartMs, a.logSearchEndMs,
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
		}
		return a, nil

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
		case InputS3Search:
			return a.openS3Buckets(msg.Value)
		case InputS3SaveName:
			return a.doSaveS3Search(msg.Value)
		case InputS3Download:
			return a, a.doS3Download(msg.Value)
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
			if lp.Stream != "" {
				return a, a.startLogTail(lp.LogGroup, []string{lp.Stream},
					fmt.Sprintf("%s / %s", lp.LogGroup, lp.Stream))
			}
			return a.openLogStreams(lp.LogGroup)
		case PickerLogSearchTimeRange:
			return a.handleTimeRangePick(msg.Value)
		case PickerCWAlarmState:
			return a.handleCWAlarmStatePick(msg.Value)
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
		return a, tea.Batch(a.refreshCurrentView(), a.tick())

	// --- Key input ---
	case tea.KeyMsg:
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
		case msg.String() == "ctrl+r":
			a.regionPicker = views.NewRegionPicker(a.client.Region())
			return a, nil
		case msg.String() == "`":
			a.modeSwitcher = NewModeSwitcher(a.modeTabs, a.mode)
			return a, nil
		case msg.String() == "ctrl+e":
			return a.openConfigEditor()
		case msg.String() == "ctrl+p":
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

		// Context-specific keys
		switch a.state {
		case viewServices:
			switch msg.String() {
			case "r":
				return a.promptForceDeploy()
			case "s":
				return a.promptScale()
			case "d":
				return a.showServiceDetail()
			case "L":
				return a.openServiceLogs()
			case "S":
				return a.showStandaloneTasks()
			case "m":
				return a.showMetrics()
			}
		case viewTasks:
			switch msg.String() {
			case "x":
				return a.promptStopTask()
			case "l":
				return a.openTaskLogs()
			case "e":
				return a.execIntoTask()
			}
		case viewTaskDetail:
			switch msg.String() {
			case "E":
				return a.showEnvVars()
			}
		case viewLogs:
			switch msg.String() {
			case "w":
				return a.promptSaveLogBuffer()
			case "y":
				return a.copyLogBufferToClipboard()
			case "o":
				return a.openLogBufferInEditor()
			}
		case viewStandaloneTasks:
			switch msg.String() {
			case "l":
				return a.openStandaloneTaskLogs()
			case "x":
				return a.promptStopStandaloneTask()
			}
		case viewServiceDetail:
			switch msg.String() {
			case "D":
				return a.showTaskDefDiff()
			}
		case viewSSM:
			switch msg.String() {
			case "W":
				return a.saveSSMPrefix()
			case "e":
				return a.editSSMParam()
			}
		case viewSecrets:
			switch msg.String() {
			case "W":
				return a.saveSMFilter()
			case "e":
				return a.editSecret()
			}
		case viewS3Buckets:
			switch msg.String() {
			case "W":
				return a.saveS3Search()
			}
		case viewS3Objects:
			switch msg.String() {
			case "i":
				return a.showS3ObjectDetail()
			case "D":
				return a.promptS3Download()
			}
		case viewS3Detail:
			switch msg.String() {
			case "D":
				return a.promptS3DownloadFromDetail()
			}
		case viewDynamoTables:
			switch msg.String() {
			case "W":
				return a.saveDynamoTable()
			}
		case viewDynamoItems:
			switch msg.String() {
			case "f":
				return a.promptDynamoFilter()
			case "p":
				return a.promptDynamoPartiQL()
			case "W":
				return a.saveDynamoQuery()
			case "]":
				return a.loadDynamoNextPage()
			}
		case viewDynamoItemDetail:
			switch msg.String() {
			case "e":
				return a.editDynamoField()
			case "c":
				return a.cloneDynamoItem()
			}
		case viewSQSQueues:
			switch msg.String() {
			case "W":
				return a.saveSQSQueue()
			}
		case viewSQSDetail:
			switch msg.String() {
			case "m":
				return a.openSQSMessages()
			case "s":
				return a.sendSQSMessage()
			case "d":
				return a.openSQSDLQ()
			}
		case viewSQSMessages:
			switch msg.String() {
			case "p":
				return a.pollSQSMessages()
			case "x":
				return a.deleteSQSMessage()
			case "s":
				return a.sendSQSMessage()
			case "X":
				a.sqsMessagesView = a.sqsMessagesView.ClearMessages()
				return a, nil
			}
		case viewSQSMessageDetail:
			switch msg.String() {
			case "x":
				return a.deleteSQSMessage()
			case "c":
				return a.cloneSQSMessage()
			}
		case viewLambdaList:
			switch msg.String() {
			case "W":
				return a.saveLambdaSearch()
			case "l":
				return a.tailLambdaLogs()
			case "s":
				return a.searchLambdaLogs()
			}
		case viewLambdaDetail:
			switch msg.String() {
			case "l":
				return a.tailLambdaDetailLogs()
			case "s":
				return a.searchLambdaDetailLogs()
			case "E":
				return a.showLambdaEnvVars()
			}
		case viewLogGroups:
			switch msg.String() {
			case "l":
				return a.tailLogGroup()
			case "s":
				return a.promptLogSearchFromGroups()
			case "W":
				return a.saveLogGroupPath()
			}
		case viewLogStreams:
			switch msg.String() {
			case "l":
				return a.tailLogStream()
			case "L":
				return a.tailEntireLogGroup()
			case "s":
				return a.promptLogSearchFromStreams()
			case "W":
				return a.saveLogStreamPath()
			}
		case viewAlarmDetail:
			switch msg.String() {
			case "a":
				return a.toggleAlarmActions()
			case "S":
				return a.promptSetAlarmState()
			}
		case viewCBProjects:
			switch msg.String() {
			case "b":
				return a.triggerCBBuild()
			}
		case viewCBBuilds:
			switch msg.String() {
			case "b":
				return a.triggerCBBuild()
			}
		case viewCBBuildDetail:
			switch msg.String() {
			case "l":
				return a.viewCBBuildLogs()
			case "s":
				return a.searchCBBuildLogs()
			case "b":
				return a.triggerCBBuild()
			case "x":
				return a.stopCBBuild()
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
		a.flashMessage, a.flashExpiry, a.err)

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
		primary = "[E] env vars"
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
		{"`", "Switch mode"},
		{"ctrl+p", "Reopen mode picker"},
		{"ctrl+e", "Edit config"},
		{"ctrl+r", "Switch AWS region"},
		{"R", "Refresh data"},
		{"/", "Filter/search"},
		{"esc", "Go back"},
		{"q", "Quit"},
		{"?", "Toggle this help"},
	}

	var context []kv
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
			{"d", "Service detail (deployments + events)"},
			{"L", "Tail logs (all tasks)"},
			{"r", "Force new deployment"},
			{"s", "Scale service"},
			{"m", "CPU/memory metrics + alarms"},
			{"S", "Standalone tasks (non-service)"},
		}
	case viewTasks:
		context = []kv{
			{"enter", "Task detail"},
			{"l", "Tail logs"},
			{"x", "Stop task"},
			{"e", "ECS Exec (shell into container)"},
		}
	case viewTaskDetail:
		context = []kv{
			{"E", "View environment variables"},
		}
	case viewServiceDetail:
		context = []kv{
			{"tab", "Switch between Deployments and Events"},
			{"D", "Task definition diff"},
			{"j/k", "Scroll"},
		}
	case viewLogs:
		context = []kv{
			{"f", "Toggle follow mode"},
			{"t", "Cycle timestamps (relative/local/UTC)"},
			{"/", "Search — jump with n/N"},
			{"n/N", "Next/previous match"},
			{"[/]", "Load older/newer logs"},
			{"w", "Save buffer to file"},
			{"y", "Copy buffer to clipboard"},
			{"o", "Open buffer in $EDITOR"},
			{"g/G", "Jump to top/bottom"},
			{"PgUp/PgDn", "Scroll by page"},
		}
	case viewStandaloneTasks:
		context = []kv{
			{"enter", "Task detail"},
			{"l", "Tail logs"},
			{"x", "Stop task"},
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
			{"e", "Edit parameter value"},
			{"W", "Save prefix for quick access"},
		}
	case viewSecrets:
		context = []kv{
			{"enter", "View secret value"},
			{"e", "Edit secret value"},
			{"W", "Save filter for quick access"},
		}
	case viewSecretValue:
		context = []kv{
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewS3Buckets:
		context = []kv{
			{"enter", "Browse bucket"},
			{"W", "Save search"},
		}
	case viewS3Objects:
		context = []kv{
			{"enter", "Open folder / view detail"},
			{"i", "View detail + tags"},
			{"D", "Download object or folder"},
		}
	case viewS3Detail:
		context = []kv{
			{"D", "Download"},
		}
	case viewLambdaList:
		context = []kv{
			{"enter", "Function detail"},
			{"l", "Tail function logs"},
			{"s", "Search function logs"},
			{"W", "Save search"},
		}
	case viewLambdaDetail:
		context = []kv{
			{"E", "View environment variables"},
			{"l", "Tail logs"},
			{"s", "Search logs"},
		}
	case viewDynamoTables:
		context = []kv{
			{"enter", "Browse table"},
			{"W", "Save table"},
		}
	case viewDynamoItems:
		context = []kv{
			{"enter", "View item detail"},
			{"f", "Filter scan (attribute + operator + value)"},
			{"p", "PartiQL query"},
			{"]", "Load next page"},
			{"W", "Save PartiQL query"},
		}
	case viewDynamoItemDetail:
		context = []kv{
			{"e", "Edit field value"},
			{"c", "Clone item"},
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewSQSQueues:
		context = []kv{
			{"enter", "Queue detail"},
			{"W", "Save queue"},
		}
	case viewSQSDetail:
		context = []kv{
			{"m", "Browse messages"},
			{"s", "Send new message"},
			{"d", "Navigate to dead letter queue"},
			{"R", "Refresh stats"},
		}
	case viewSQSMessages:
		context = []kv{
			{"enter", "Message detail"},
			{"p", "Poll for messages"},
			{"s", "Send new message"},
			{"x", "Delete message"},
			{"X", "Clear message list"},
		}
	case viewSQSMessageDetail:
		context = []kv{
			{"c", "Clone & send"},
			{"x", "Delete message"},
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
			{"l", "Tail entire log group"},
			{"s", "Search selected groups"},
			{"W", "Save log path"},
		}
	case viewLogStreams:
		context = []kv{
			{"enter", "Peek (last 1 min, paused)"},
			{"l", "Tail stream"},
			{"L", "Tail entire log group"},
			{"s", "Search stream"},
			{"W", "Save log path"},
		}
	case viewLogSearch:
		context = []kv{
			{"enter", "Jump to log at timestamp"},
			{"t", "Toggle timestamps"},
			{"g/G", "Top/bottom"},
		}
	case viewAlarms:
		context = []kv{
			{"enter", "View alarm detail"},
			{"t", "Toggle local/UTC timestamps"},
			{"/", "Filter alarms"},
		}
	case viewAlarmDetail:
		context = []kv{
			{"a", "Enable/disable alarm actions"},
			{"S", "Set alarm state (testing)"},
			{"j/k", "Scroll"},
			{"g/G", "Top/bottom"},
		}
	case viewCBProjects:
		context = []kv{
			{"enter", "View builds"},
			{"b", "Start new build"},
			{"/", "Filter projects"},
		}
	case viewCBBuilds:
		context = []kv{
			{"enter", "View build detail"},
			{"b", "Start new build"},
			{"t", "Toggle local/UTC timestamps"},
		}
	case viewCBBuildDetail:
		context = []kv{
			{"l", "View build logs (buffer)"},
			{"s", "Search build logs (full)"},
			{"b", "Start new build"},
			{"x", "Stop build (if in progress)"},
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
		if s := a.logStreamsView.SelectedStream(); s != nil {
			return a.peekLogStream(s.Name)
		}
	case viewAlarms:
		return a.openAlarmDetail()
	case viewCBProjects:
		if p := a.cbProjectsView.SelectedProject(); p != nil {
			return a.openCBBuilds(p.Name)
		}
	case viewCBBuilds:
		return a.openCBBuildDetail()
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
	}
	return a, nil
}
