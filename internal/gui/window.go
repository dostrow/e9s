//go:build gui

package gui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
)

const (
	pageClusters        = "clusters"
	pageServices        = "services"
	pageTasks           = "tasks"
	pageStandaloneTasks = "standalone-tasks"
	pageTaskDefinitions = "task-definitions"

	detailIntro          = "intro"
	detailClusterSummary = "cluster-summary"
	detailService        = "service"
	detailTask           = "task"
	detailHelp           = "help"
	detailError          = "error"
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
	selectedTaskDefinition      *model.TaskDefSummary
	standaloneReturnPage        string
	standaloneReturnService     string
	clusterTable                *stringTable
	serviceTable                *stringTable
	taskTable                   *stringTable
	taskDefinitionTable         *stringTable
	resourceStack               *gtk.Stack
	search                      *gtk.SearchEntry
	backButton                  *gtk.Button
	headerBar                   *gtk.Box
	clustersNavButton           *gtk.ToggleButton
	taskDefinitionsNavButton    *gtk.ToggleButton
	logsButton                  *gtk.Button
	taskLogsButton              *gtk.Button
	standaloneButton            *gtk.Button
	runTaskButton               *gtk.Button
	metricsButton               *gtk.Button
	execButton                  *gtk.Button
	scaleButton                 *gtk.Button
	stopTaskButton              *gtk.Button
	deployButton                *gtk.Button
	breadcrumb                  *gtk.Label
	detailToolbar               *gtk.Box
	detailParentButton          *gtk.Button
	detailBuffer                *gtk.TextBuffer
	detailText                  string
	detailStack                 *gtk.Stack
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
	logStore                    *boundedLogs
	logIndentTags               map[int]*gtk.TextTag
	logSource                   model.LogSource
	logTitle                    string
	logLastTS                   int64
	logFollowing                bool
	showingLogs                 bool
	status                      *gtk.Label
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
	w.taskDefinitionTable = newStringTable([]columnSpec{
		{title: "FAMILY", field: 0, expand: true},
		{title: "REVISION", field: 1},
		{title: "TASK DEFINITION ARN", field: 2, expand: true},
	})
	w.clusterTable.view.ConnectActivate(w.openClusterAt)
	w.serviceTable.view.ConnectActivate(w.openServiceAt)
	w.taskTable.view.ConnectActivate(w.openTaskAt)
	w.taskDefinitionTable.view.ConnectActivate(w.openTaskDefinitionAt)

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

	go w.autoRefresh()
	return w
}

func (w *mainWindow) buildLayout() gtk.Widgetter {
	w.backButton = gtk.NewButtonWithLabel("Back")
	w.backButton.SetSensitive(false)
	w.backButton.ConnectClicked(w.navigateBrowserBack)

	title := gtk.NewLabel("e9s")
	title.AddCSSClass("app-title")
	w.breadcrumb = gtk.NewLabel("ECS / Clusters")
	w.breadcrumb.SetXAlign(0)
	w.breadcrumb.SetHExpand(true)
	w.breadcrumb.AddCSSClass("breadcrumb")

	refresh := gtk.NewButtonWithLabel("Refresh")
	refresh.ConnectClicked(w.refresh)
	w.logsButton = gtk.NewButtonWithLabel("Service logs")
	w.logsButton.SetSensitive(false)
	w.logsButton.ConnectClicked(w.openServiceLogs)
	w.taskLogsButton = gtk.NewButtonWithLabel("Task logs")
	w.taskLogsButton.SetSensitive(false)
	w.taskLogsButton.ConnectClicked(w.openTaskLogs)
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
	header.Append(w.scaleButton)
	header.Append(w.stopTaskButton)
	header.Append(w.deployButton)
	header.Append(refresh)

	sidebar := gtk.NewBox(gtk.OrientationVertical, 6)
	sidebar.AddCSSClass("mode-sidebar")
	sidebar.SetSizeRequest(150, -1)
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
	ecs := gtk.NewExpander("ECS")
	ecs.SetExpanded(true)
	ecs.SetChild(moduleItems)
	ecs.AddCSSClass("module-heading")
	comingSoon := gtk.NewLabel("More modules after PoC")
	comingSoon.SetXAlign(0)
	comingSoon.SetWrap(true)
	comingSoon.AddCSSClass("muted")
	sidebar.Append(modules)
	sidebar.Append(ecs)
	sidebar.Append(comingSoon)

	w.search = gtk.NewSearchEntry()
	w.search.SetPlaceholderText("Filter clusters…")
	w.search.ConnectSearchChanged(w.applyFilter)
	w.search.AddCSSClass("resource-search")

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
	taskDefinitionScroll := gtk.NewScrolledWindow()
	taskDefinitionScroll.SetVExpand(true)
	taskDefinitionScroll.SetHExpand(true)
	taskDefinitionScroll.SetChild(w.taskDefinitionTable.view)

	w.resourceStack = gtk.NewStack()
	w.resourceStack.SetVExpand(true)
	w.resourceStack.SetHExpand(true)
	w.resourceStack.AddNamed(clusterScroll, pageClusters)
	w.resourceStack.AddNamed(serviceScroll, pageServices)
	w.resourceStack.AddNamed(taskScroll, pageTasks)
	w.resourceStack.AddNamed(taskDefinitionScroll, pageTaskDefinitions)
	w.resourceStack.SetVisibleChildName(pageClusters)

	resourcePane := gtk.NewBox(gtk.OrientationVertical, 8)
	resourcePane.AddCSSClass("resource-pane")
	resourcePane.Append(w.search)
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

	contentSplit := gtk.NewPaned(gtk.OrientationHorizontal)
	contentSplit.SetStartChild(resourcePane)
	contentSplit.SetEndChild(w.detailStack)
	contentSplit.SetPosition(700)
	contentSplit.SetResizeStartChild(true)
	contentSplit.SetResizeEndChild(true)
	w.installPaneZoom(&resourcePane.Widget, &w.detailStack.Widget)

	mainSplit := gtk.NewPaned(gtk.OrientationHorizontal)
	mainSplit.SetStartChild(sidebar)
	mainSplit.SetEndChild(contentSplit)
	mainSplit.SetPosition(165)
	mainSplit.SetResizeStartChild(false)
	mainSplit.SetResizeEndChild(true)

	w.spinner = gtk.NewSpinner()
	w.status = gtk.NewLabel("Ready")
	w.status.SetXAlign(0)
	w.status.SetHExpand(true)
	identity := gtk.NewLabel(fmt.Sprintf("profile: %s   region: %s", valueOrDash(w.options.Profile), valueOrDash(w.options.Region)))
	identity.AddCSSClass("muted")
	footer := gtk.NewBox(gtk.OrientationHorizontal, 8)
	footer.AddCSSClass("status-bar")
	footer.Append(w.spinner)
	footer.Append(w.status)
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
	w.addAction(app, "search", []string{"slash"}, func() {
		if w.showingLogs {
			w.logSearch.GrabFocus()
		} else {
			w.search.GrabFocus()
		}
	})
	w.addAction(app, "back", []string{"Escape"}, w.goBack)
	w.addAction(app, "modes", []string{"<Control>p"}, func() {
		w.setStatus("ECS is the only module in this proof of concept", false)
	})
	w.addAction(app, "help", []string{"<Shift>slash"}, func() {
		if w.showingTerminal {
			w.setStatus("Disconnect ECS Exec before opening help", false)
			return
		}
		if w.showingEditor {
			w.setStatus("Close the task-definition editor before opening help", false)
			return
		}
		w.setDetail("KEYBOARD SHORTCUTS\n\nEnter          Open selected row or task\nEscape         Back / close auxiliary view\n/              Focus active filter\nCtrl++/-       Zoom active pane in/out\nCtrl+0         Reset active pane zoom\nCtrl+R         Refresh\nShift+S        Toggle standalone/service tasks\nCtrl+Enter     Run standalone task\nShift+T        Browse task definitions\nE              Task-definition environment\nD              Diff previous revision\nCtrl+E         Edit task-definition JSON\nCtrl+S         Register edited revision\nCtrl+Shift+E   ECS Exec in embedded terminal\nM              Service or selected-task metrics\nShift+L        Follow service logs\nCtrl+Shift+L   Follow selected task logs\nCtrl+Space     Pause/resume logs\nCtrl+Shift+C   Copy log buffer\nCtrl+L         Clear log buffer\nCtrl+Shift+S   Scale service\nCtrl+Shift+A   Toggle scale-in suspension\nCtrl+Shift+X   Stop selected task\nCtrl+Shift+R   Force deployment\nCtrl+P         Module switcher placeholder\n?              Show this help", detailHelp)
		w.detailStack.SetVisibleChildName("detail")
	})
	w.addAction(app, "logs", []string{"<Shift>l"}, w.openServiceLogs)
	w.addAction(app, "task-logs", []string{"<Control><Shift>l"}, w.openTaskLogs)
	w.addAction(app, "standalone-tasks", []string{"<Shift>s"}, w.toggleStandaloneTasks)
	w.addAction(app, "run-task", []string{"<Control>Return"}, w.promptRunTask)
	w.addAction(app, "metrics", []string{"m"}, w.openMetrics)
	w.addAction(app, "toggle-scale-in", []string{"<Control><Shift>a"}, w.confirmToggleScaleIn)
	w.addAction(app, "task-definitions", []string{"<Shift>t"}, w.openTaskDefinitions)
	w.addAction(app, "task-definition-env", []string{"e"}, w.openTaskDefinitionEnvironment)
	w.addAction(app, "task-definition-diff", []string{"d"}, w.openTaskDefinitionDiff)
	w.addAction(app, "task-definition-edit", []string{"<Control>e"}, w.openTaskDefinitionEditor)
	w.addAction(app, "task-definition-register", []string{"<Control>s"}, w.confirmRegisterTaskDefinition)
	w.addAction(app, "ecs-exec", []string{"<Control><Shift>e"}, w.openExec)
	w.addAction(app, "toggle-logs", []string{"<Control>space"}, w.toggleLogFollow)
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
	}
	return ctx, w.generation
}

func (w *mainWindow) finishRequest(ctx context.Context, generation uint64, err error, apply func()) {
	w.finishRequestWithStatus(ctx, generation, err, "", apply)
}

func (w *mainWindow) finishRequestWithStatus(ctx context.Context, generation uint64, err error, success string, apply func()) {
	w.finishRequestResult(ctx, generation, err, success, true, apply)
}

func (w *mainWindow) finishRefreshRequest(ctx context.Context, generation uint64, err error, apply func()) {
	w.finishRequestResult(ctx, generation, err, "", false, apply)
}

func (w *mainWindow) finishRequestResult(ctx context.Context, generation uint64, err error, success string, showDetailError bool, apply func()) {
	glib.IdleAdd(func() {
		if ctx.Err() != nil || generation != w.generation {
			return
		}
		w.spinner.Stop()
		if err != nil {
			w.setStatus(err.Error(), true)
			if showDetailError {
				w.setDetail("ERROR\n\n"+err.Error(), detailError)
			}
			return
		}
		w.lastSuccessfulLoad = time.Now()
		apply()
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

func (w *mainWindow) setBreadcrumb(text string) {
	w.breadcrumb.SetLabel(text)
	w.breadcrumb.QueueResize()
	w.breadcrumb.QueueDraw()
	if w.headerBar != nil {
		w.headerBar.QueueDraw()
	}
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
		w.detailParentButton.SetLabel("Back to standalone summary")
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
	w.taskTable.selection.SetSelected(gtk.InvalidListPosition)
	w.updateActionSensitivity()
	if w.currentPage == pageStandaloneTasks {
		w.setBreadcrumb("ECS / " + w.selectedCluster + " / Standalone tasks")
		w.setDetail(standaloneTaskSummary(w.selectedCluster, w.allTasks), detailClusterSummary)
		return
	}
	service, _ := findService(w.allServices, w.selectedService)
	w.setBreadcrumb("ECS / " + w.selectedCluster + " / " + service.Name)
	w.setDetail(formatServiceDetail(w.selectedCluster, service, w.allTasks), detailService)
}

func (w *mainWindow) resetWorkspaceForBrowserChange() {
	if w.requestCancel != nil {
		w.requestCancel()
		w.requestCancel = nil
		w.generation++
	}
	w.spinner.Stop()
	if w.logCancel != nil {
		w.logCancel()
		w.logCancel = nil
		w.logGeneration++
	}
	w.logFollowing = false
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
	w.resetWorkspaceForBrowserChange()
	w.currentPage = pageTasks
	w.selectedService = service.Name
	w.selectedTask = ""
	w.updateActionSensitivity()
	w.setBreadcrumb("ECS / " + w.selectedCluster + " / " + service.Name)
	w.backButton.SetSensitive(true)
	w.search.SetPlaceholderText("Filter tasks…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageTasks)
	w.setDetail("Loading tasks and service details…", detailService)

	ctx, generation := w.startRequest("Loading " + service.Name + "…")
	cluster := w.selectedCluster
	go func() {
		tasks, err := w.options.ECS.ListTasks(ctx, cluster, service.Name)
		w.finishRequest(ctx, generation, err, func() {
			w.allTasks = tasks
			w.applyTaskFilter()
			w.setDetail(formatServiceDetail(cluster, service, tasks), detailService)
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
	}
	w.resetWorkspaceForBrowserChange()
	w.currentPage = pageStandaloneTasks
	w.selectedService = ""
	w.selectedTask = ""
	w.updateActionSensitivity()
	w.setBreadcrumb("ECS / " + w.selectedCluster + " / Standalone tasks")
	w.backButton.SetSensitive(true)
	w.search.SetPlaceholderText("Filter standalone tasks…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageTasks)
	w.setDetail("Loading standalone tasks…", detailClusterSummary)

	cluster := w.selectedCluster
	ctx, generation := w.startRequest("Loading standalone tasks in " + cluster + "…")
	go func() {
		tasks, err := w.options.ECS.ListStandaloneTasks(ctx, cluster)
		w.finishRequest(ctx, generation, err, func() {
			w.allTasks = tasks
			w.applyTaskFilter()
			w.setDetail(standaloneTaskSummary(cluster, tasks), detailClusterSummary)
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
			w.loadTasks(service)
			return
		}
	}
	w.loadServices(cluster)
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
		w.setBreadcrumb("ECS / " + w.selectedCluster + " / Standalone tasks / " + shortID(task.TaskID))
	} else {
		w.setBreadcrumb("ECS / " + w.selectedCluster + " / " + w.selectedService + " / " + shortID(task.TaskID))
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
	if w.currentPage == pageTasks || w.currentPage == pageStandaloneTasks {
		w.resetWorkspaceForBrowserChange()
		w.currentPage = pageServices
		w.selectedService = ""
		w.selectedTask = ""
		w.allTasks = nil
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
	cluster, taskARN := w.selectedCluster, w.selectedTask
	ctx, generation := w.startRefreshRequest("Refreshing standalone tasks in "+cluster+"…", foreground)
	go func() {
		tasks, err := w.options.ECS.ListStandaloneTasks(ctx, cluster)
		w.finishRefreshRequest(ctx, generation, err, func() {
			w.allTasks = tasks
			w.applyTaskFilter()
			if taskARN == "" {
				if w.detailContent == detailClusterSummary {
					w.setDetail(standaloneTaskSummary(cluster, tasks), detailClusterSummary)
				}
				return
			}
			task, found := findTask(tasks, taskARN)
			if !found {
				w.selectedTask = ""
				w.updateActionSensitivity()
				w.setBreadcrumb("ECS / " + cluster + " / Standalone tasks")
				if w.detailContent == detailTask {
					w.setDetail("The selected task is no longer available.\n\n"+standaloneTaskSummary(cluster, tasks), detailClusterSummary)
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
	cluster, serviceName, taskARN := w.selectedCluster, w.selectedService, w.selectedTask
	ctx, generation := w.startRefreshRequest("Refreshing tasks for "+serviceName+"…", foreground)
	go func() {
		services, err := w.options.ECS.ListServices(ctx, cluster)
		service, serviceFound := findService(services, serviceName)
		var tasks []model.Task
		if err == nil && serviceFound {
			tasks, err = w.options.ECS.ListTasks(ctx, cluster, serviceName)
		}

		w.finishRefreshRequest(ctx, generation, err, func() {
			w.allServices = services
			w.allTasks = tasks
			w.applyTaskFilter()
			if !serviceFound {
				w.navigateBrowserBack()
				w.setStatus("The selected service is no longer available", true)
				return
			}

			if taskARN == "" {
				if w.detailContent == detailService {
					w.setDetail(formatServiceDetail(cluster, service, tasks), detailService)
				}
				return
			}
			task, found := findTask(tasks, taskARN)
			if !found {
				w.selectedTask = ""
				w.updateActionSensitivity()
				w.setBreadcrumb("ECS / " + cluster + " / " + serviceName)
				if w.detailContent == detailTask {
					w.setDetail("The selected task is no longer available.\n\n"+formatServiceDetail(cluster, service, tasks), detailService)
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
		w.finishRefreshRequest(ctx, generation, err, func() {
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

		w.finishRefreshRequest(ctx, generation, err, func() {
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
		if w.ctx.Err() == nil {
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
	w.status.SetLabel(message)
	w.status.RemoveCSSClass("error")
	if isError {
		w.status.AddCSSClass("error")
	}
}

func (w *mainWindow) updateActionSensitivity() {
	serviceSelected := w.currentPage == pageTasks && w.selectedCluster != "" && w.selectedService != ""
	standalonePage := w.currentPage == pageStandaloneTasks && w.selectedCluster != ""
	taskSelected := (serviceSelected || standalonePage) && w.selectedTask != ""
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
		w.clustersNavButton.SetActive(w.currentPage != pageTaskDefinitions)
		w.taskDefinitionsNavButton.SetActive(w.currentPage == pageTaskDefinitions)
	}
	w.runTaskButton.SetVisible(standalonePage)
	w.runTaskButton.SetSensitive(standalonePage)
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
	w.execButton.SetVisible(taskSelected)
	w.execButton.SetSensitive(execEnabled)
	w.logsButton.SetVisible(serviceSelected)
	w.logsButton.SetSensitive(serviceSelected && w.options.Logs != nil)
	w.taskLogsButton.SetVisible(taskSelected)
	w.taskLogsButton.SetSensitive(taskSelected && w.options.Logs != nil)
	w.scaleButton.SetVisible(serviceSelected)
	w.scaleButton.SetSensitive(serviceSelected)
	w.stopTaskButton.SetVisible(taskSelected)
	w.stopTaskButton.SetSensitive(taskSelected)
	w.deployButton.SetVisible(serviceSelected)
	w.deployButton.SetSensitive(serviceSelected)
}
