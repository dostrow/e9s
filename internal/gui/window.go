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

	pageClusters        = "clusters"
	pageServices        = "services"
	pageTasks           = "tasks"
	pageStandaloneTasks = "standalone-tasks"
	pageStoppedTasks    = "stopped-standalone-tasks"
	pageTaskDefinitions = "task-definitions"
	pageLogGroups       = "cloudwatch-log-groups"
	pageLogStreams      = "cloudwatch-log-streams"
	pageSavedLogSearch  = "cloudwatch-saved-search"
	pageAlarms          = "cloudwatch-alarms"

	detailIntro          = "intro"
	detailClusterSummary = "cluster-summary"
	detailService        = "service"
	detailTask           = "task"
	detailHelp           = "help"
	detailError          = "error"
	detailLogGroup       = "log-group"
	detailLogStream      = "log-stream"
	detailAlarm          = "alarm"
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
	resourceStack               *gtk.Stack
	search                      *gtk.SearchEntry
	backButton                  *gtk.Button
	headerBar                   *gtk.Box
	clustersNavButton           *gtk.ToggleButton
	taskDefinitionsNavButton    *gtk.ToggleButton
	logGroupsNavButton          *gtk.ToggleButton
	cloudWatchModuleItems       *gtk.Box
	moduleErrorGlyphs           map[string]*gtk.Image
	alarmNavButtons             map[string]*gtk.ToggleButton
	savedLogsLabel              *gtk.Label
	savedLogNavButtons          []*gtk.ToggleButton
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
	detailParentButton          *gtk.Button
	detailBuffer                *gtk.TextBuffer
	detailText                  string
	detailStack                 *gtk.Stack
	workspaceBusyBar            *gtk.Box
	workspaceBusySpinner        *gtk.Spinner
	workspaceBusyLabel          *gtk.Label
	workspaceBusy               bool
	metricsCPUAvg               *gtk.ProgressBar
	metricsCPUMax               *gtk.ProgressBar
	metricsMemAvg               *gtk.ProgressBar
	metricsMemMax               *gtk.ProgressBar
	metricsAlarmTable           *stringTable
	metricsScaleButton          *gtk.Button
	metricsScaleLabel           *gtk.Label
	metricsTitle                *gtk.Label
	metricsScope                *gtk.Label
	metricsNotice               *gtk.Label
	metricsTimestamp            *gtk.Label
	metricsAlarmSection         *gtk.Box
	metricsSnapshot             *model.ServiceMetrics
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
	showingEditor               bool
	editorDirty                 bool
	editorLoading               bool
	terminal                    *vteTerminal
	terminalTitle               *gtk.Label
	terminalTask                model.Task
	terminalContainer           string
	terminalCommand             string
	showingTerminal             bool
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

func newMainWindow(ctx context.Context, app *gtk.Application, options Options) *mainWindow {
	w := &mainWindow{
		ctx:           ctx,
		options:       options,
		currentPage:   pageClusters,
		detailContent: detailIntro,
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
	w.clusterTable.view.ConnectActivate(w.openClusterAt)
	w.serviceTable.view.ConnectActivate(w.openServiceAt)
	w.taskTable.view.ConnectActivate(w.openTaskAt)
	w.stoppedTaskTable.view.ConnectActivate(w.openStoppedTaskAt)
	w.taskDefinitionTable.view.ConnectActivate(w.openTaskDefinitionAt)
	w.logGroupTable.view.ConnectActivate(w.openLogGroupAt)
	w.logStreamTable.view.ConnectActivate(w.peekLogStreamAt)
	w.alarmTable.view.ConnectActivate(w.openAlarmAt)
	w.logGroupTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectLogGroupRow() })
	w.logStreamTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectLogStreamRow() })
	w.alarmTable.selection.ConnectSelectionChanged(func(_, _ uint) { w.selectAlarmRow() })

	w.window = gtk.NewApplicationWindow(app)
	w.window.SetTitle("e9s")
	w.window.SetDefaultSize(1380, 820)
	w.window.SetChild(w.buildLayout())
	w.window.ConnectDestroy(func() {
		if w.terminal != nil {
			w.terminal.Stop()
		}
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
	w.breadcrumbText = "ECS / Clusters"
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
	w.moduleErrorGlyphs = make(map[string]*gtk.Image, 3)
	ecs := w.newModuleExpander("ECS", moduleECS, moduleItems)
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
	comingSoon := gtk.NewLabel("More modules planned")
	comingSoon.SetXAlign(0)
	comingSoon.SetWrap(true)
	comingSoon.AddCSSClass("muted")
	sidebar.Append(modules)
	sidebar.Append(ecs)
	sidebar.Append(cloudWatch)
	sidebar.Append(cloudWatchAlarms)
	sidebar.Append(comingSoon)

	w.search = gtk.NewSearchEntry()
	w.search.SetPlaceholderText("Filter clusters…")
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

	w.resourceStack = gtk.NewStack()
	w.resourceStack.SetVExpand(true)
	w.resourceStack.SetHExpand(true)
	w.resourceStack.AddNamed(clusterScroll, pageClusters)
	w.resourceStack.AddNamed(serviceScroll, pageServices)
	w.resourceStack.AddNamed(taskScroll, pageTasks)
	w.resourceStack.AddNamed(stoppedTaskScroll, pageStoppedTasks)
	w.resourceStack.AddNamed(taskDefinitionScroll, pageTaskDefinitions)
	w.resourceStack.AddNamed(logGroupScroll, pageLogGroups)
	w.resourceStack.AddNamed(logStreamScroll, pageLogStreams)
	w.resourceStack.AddNamed(alarmScroll, pageAlarms)
	savedLogScope := gtk.NewLabel("Saved CloudWatch Logs destination")
	savedLogScope.SetXAlign(0)
	savedLogScope.SetYAlign(0)
	savedLogScope.SetMarginTop(12)
	savedLogScope.SetMarginStart(12)
	w.resourceStack.AddNamed(savedLogScope, pageSavedLogSearch)
	w.resourceStack.SetVisibleChildName(pageClusters)

	resourcePane := gtk.NewBox(gtk.OrientationVertical, 8)
	resourcePane.AddCSSClass("resource-pane")
	resourcePane.Append(w.search)
	resourcePane.Append(w.taskScopeBar)
	resourcePane.Append(w.resourceStack)

	w.detailBuffer = gtk.NewTextBuffer(nil)
	w.detailText = "Select a cluster and press Enter to browse its services."
	w.detailBuffer.SetText(w.detailText)
	w.detailParentButton = gtk.NewButtonWithLabel("Back to service details")
	w.detailParentButton.ConnectClicked(w.showParentDetails)
	w.detailToolbar = gtk.NewBox(gtk.OrientationHorizontal, 8)
	w.detailToolbar.AddCSSClass("log-toolbar")
	w.detailToolbar.Append(w.detailParentButton)
	w.detailToolbar.SetVisible(false)
	detail := gtk.NewTextViewWithBuffer(w.detailBuffer)
	detail.SetEditable(false)
	detail.SetCursorVisible(false)
	detail.SetMonospace(true)
	detail.SetWrapMode(gtk.WrapWordChar)
	detail.AddCSSClass("inspector")
	detailScroll := gtk.NewScrolledWindow()
	detailScroll.SetVExpand(true)
	detailScroll.SetHExpand(true)
	detailScroll.SetChild(detail)
	detailPane := gtk.NewBox(gtk.OrientationVertical, 0)
	detailPane.Append(w.detailToolbar)
	detailPane.Append(detailScroll)

	w.detailStack = gtk.NewStack()
	w.detailStack.SetVExpand(true)
	w.detailStack.SetHExpand(true)
	w.detailStack.AddNamed(detailPane, "detail")
	w.detailStack.AddNamed(w.buildLogPane(), "logs")
	w.detailStack.AddNamed(w.buildMetricsPane(), "metrics")
	w.detailStack.AddNamed(w.buildTaskDefinitionPane(), "task-definition")
	w.detailStack.AddNamed(w.buildTaskDefinitionEditor(), "editor")
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
	w.addAction(app, "modes", []string{"<Control>p"}, func() {
		w.setStatus("Choose ECS, CloudWatch Logs, or CloudWatch Alarms from the Module Rail", false)
	})
	w.addAction(app, "help", nil, func() {
		if w.showingTerminal {
			w.setStatus("Disconnect ECS Exec before opening help", false)
			return
		}
		if w.showingEditor {
			w.setStatus("Close the task-definition editor before opening help", false)
			return
		}
		w.setDetail("KEYBOARD SHORTCUTS\n\nEnter          Open selected row or task\nEscape         Back / close auxiliary view\n/              Focus active filter\nCtrl++/-       Zoom active pane in/out\nCtrl+0         Reset active pane zoom\nCtrl+R         Refresh\nShift+S        Toggle standalone/service tasks\nCtrl+Enter     Run standalone task\nShift+T        Browse task definitions\nE              Task-definition environment\nD              Diff previous revision\nCtrl+E         Edit task-definition JSON\nCtrl+S         Register edited revision\nCtrl+Shift+E   ECS Exec in embedded terminal\nM              Service or selected-task metrics\nShift+L        Follow service logs\nCtrl+Shift+L   Follow selected task logs\nCtrl+Space     Pause/resume logs\nT              Cycle log timestamps\nCtrl+Shift+C   Copy log buffer\nCtrl+L         Clear log buffer\nCtrl+Shift+S   Scale service\nCtrl+Shift+A   Toggle scale-in suspension\nCtrl+Shift+X   Stop selected task\nCtrl+Shift+R   Force deployment\nCtrl+P         Module switcher placeholder\n?              Show this help", detailHelp)
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
	w.detailBuffer.SetText(text)
	w.detailText = text
	w.detailContent = content
}

func (w *mainWindow) setWorkspaceBusy(label string, busy bool) {
	if w.workspaceBusyBar == nil {
		return
	}
	if !busy {
		w.workspaceBusy = false
		w.workspaceBusySpinner.Stop()
		w.workspaceBusyBar.SetVisible(false)
		return
	}
	w.workspaceBusy = true
	w.workspaceBusyLabel.SetLabel(label)
	w.workspaceBusySpinner.Start()
	w.workspaceBusyBar.SetVisible(true)
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
		background, ok := style.LookupColor("theme_bg_color")
		if !ok {
			background, ok = style.LookupColor("window_bg_color")
		}
		if ok {
			cr.SetSourceRGBA(
				float64(background.Red()),
				float64(background.Green()),
				float64(background.Blue()),
				float64(background.Alpha()),
			)
			cr.Rectangle(0, 0, float64(width), float64(height))
			cr.Fill()
		}

		foreground := style.Color()
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
		w.detailParentButton.SetLabel("Back to service details")
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
	w.setDetail(w.serviceTaskSummary(service), detailService)
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
	w.showingLogs = false
	w.showingMetrics = false
	if w.showingTerminal {
		w.closeTerminalNow(false)
	}
	w.showingEditor = false
	w.editorDirty = false
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
	if w.showingEditor {
		w.closeTaskDefinitionEditorThen(w.loadClusters)
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
		w.setDetail("Loading recently stopped tasks for "+service.Name+"…", detailService)
	} else {
		w.resourceStack.SetVisibleChildName(pageTasks)
		w.applyTaskFilter()
		w.setDetail("Loading active tasks and service details…", detailService)
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
			w.setDetail(w.serviceTaskSummary(service), detailService)
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
							w.setDetail(w.serviceTaskSummary(selectedService), detailService)
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
	if w.showingStoppedTasks {
		return formatServiceStoppedTasksDetail(w.selectedCluster, service, w.allTasks, w.taskNextToken != "")
	}
	return formatServiceDetail(w.selectedCluster, service, w.allTasks)
}

func (w *mainWindow) openClusterAt(position uint) {
	if int(position) >= len(w.filteredClusters) {
		return
	}
	w.loadServices(w.filteredClusters[position].Name)
}

func (w *mainWindow) openServiceAt(position uint) {
	if int(position) >= len(w.filteredServices) {
		return
	}
	w.loadTasks(w.filteredServices[position])
}

func (w *mainWindow) openTaskAt(position uint) {
	w.openTaskFromFiltered(position)
}

func (w *mainWindow) openStoppedTaskAt(position uint) {
	w.openTaskFromFiltered(position)
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
	w.setDetail(formatTaskDetail(task), detailTask)
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
		w.closeTaskDefinitionEditor()
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
		w.closeTaskDefinitionEditorThen(w.navigateBrowserBack)
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
		w.resetWorkspaceForBrowserChange()
		w.clearTaskBrowser()
		w.currentPage = pageServices
		w.selectedService = ""
		w.updateActionSensitivity()
		w.search.SetText("")
		w.search.SetPlaceholderText("Filter services…")
		w.resourceStack.SetVisibleChildName(pageServices)
		w.setBreadcrumb("ECS / " + w.selectedCluster)
		w.setDetail(clusterSummary(w.selectedCluster, len(w.allServices)), detailClusterSummary)
		w.applyServiceFilter()
		w.setStatus("Ready", false)
		return
	}
	if w.currentPage != pageServices {
		return
	}
	w.resetWorkspaceForBrowserChange()
	w.currentPage = pageClusters
	w.selectedCluster = ""
	w.selectedService = ""
	w.selectedTask = ""
	w.updateActionSensitivity()
	w.allServices = nil
	w.search.SetText("")
	w.search.SetPlaceholderText("Filter clusters…")
	w.resourceStack.SetVisibleChildName(pageClusters)
	w.setBreadcrumb("ECS / Clusters")
	w.backButton.SetSensitive(false)
	w.setDetail("Select a cluster and press Enter to browse its services.", detailIntro)
	w.applyClusterFilter()
	w.setStatus("Ready", false)
}

func (w *mainWindow) refresh() {
	w.refreshCurrent(true)
}

func (w *mainWindow) refreshCurrent(foreground bool) {
	if w.showingTerminal {
		if foreground {
			w.setStatus("Disconnect ECS Exec before refreshing", false)
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
	if w.showingMetrics {
		w.loadMetrics(foreground)
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
				w.setDetail(formatTaskDetail(task), detailTask)
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
					w.setDetail(w.serviceTaskSummary(service), detailService)
				}
				return
			}
			task, found := findTask(tasks, taskARN)
			if !found {
				w.selectedTask = ""
				w.updateActionSensitivity()
				w.setBreadcrumb(w.serviceTaskBreadcrumb())
				if w.detailContent == detailTask {
					w.setDetail("The selected task is no longer available.\n\n"+w.serviceTaskSummary(service), detailService)
				}
				return
			}
			if w.detailContent == detailTask {
				w.setDetail(formatTaskDetail(task), detailTask)
			}
		})
	}()
}

func (w *mainWindow) refreshClusters(foreground bool) {
	ctx, generation := w.startRefreshRequest("Refreshing ECS clusters…", foreground)
	go func() {
		clusters, err := w.options.ECS.ListClusters(ctx)
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allClusters = clusters
			w.applyClusterFilter()
			if w.detailContent == detailIntro && len(clusters) == 0 {
				w.setDetail("No ECS clusters found in this region.", detailIntro)
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
			tasks    []model.Task
			found    bool
		)
		if err == nil && selectedName != "" {
			selected, found = findService(services, selectedName)
			if found {
				tasks, err = w.options.ECS.ListTasks(ctx, cluster, selectedName)
			}
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
				w.setDetail(formatServiceDetail(cluster, selected, tasks), detailService)
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
	}
	w.runTaskButton.SetVisible(standalonePage)
	w.runTaskButton.SetSensitive(standalonePage)
	if w.taskScopeBar != nil {
		taskBrowser := serviceSelected || standalonePage
		w.taskScopeBar.SetVisible(taskBrowser)
		w.loadMoreTasksButton.SetVisible(taskBrowser && w.showingStoppedTasks && w.taskNextToken != "")
		w.loadMoreTasksButton.SetSensitive(w.taskNextToken != "")
	}
	w.metricsButton.SetVisible(serviceSelected || taskSelected)
	w.metricsButton.SetSensitive(serviceSelected || taskSelected)
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
	w.scaleButton.SetVisible(serviceSelected)
	w.scaleButton.SetSensitive(serviceSelected)
	w.stopTaskButton.SetVisible(taskSelected && !taskStopped)
	w.stopTaskButton.SetSensitive(taskSelected && !taskStopped)
	w.deployButton.SetVisible(serviceSelected)
	w.deployButton.SetSensitive(serviceSelected)
}
