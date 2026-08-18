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
	taskDefinitionReturnPage    string
	clusterTable                *stringTable
	serviceTable                *stringTable
	taskTable                   *stringTable
	taskDefinitionTable         *stringTable
	resourceStack               *gtk.Stack
	search                      *gtk.SearchEntry
	backButton                  *gtk.Button
	logsButton                  *gtk.Button
	taskLogsButton              *gtk.Button
	standaloneButton            *gtk.Button
	runTaskButton               *gtk.Button
	metricsButton               *gtk.Button
	taskDefinitionsButton       *gtk.Button
	scaleButton                 *gtk.Button
	stopTaskButton              *gtk.Button
	deployButton                *gtk.Button
	breadcrumb                  *gtk.Label
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
	metricsTimestamp            *gtk.Label
	metricsSnapshot             *model.ServiceMetrics
	metricsAlarms               []model.AlarmState
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
	w.installActions(app)

	go w.autoRefresh()
	return w
}

func (w *mainWindow) buildLayout() gtk.Widgetter {
	w.backButton = gtk.NewButtonWithLabel("Back")
	w.backButton.SetSensitive(false)
	w.backButton.ConnectClicked(w.goBack)

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
	w.standaloneButton.ConnectClicked(w.loadStandaloneTasks)
	w.runTaskButton = gtk.NewButtonWithLabel("Run task")
	w.runTaskButton.SetSensitive(false)
	w.runTaskButton.ConnectClicked(w.promptRunTask)
	w.metricsButton = gtk.NewButtonWithLabel("Metrics")
	w.metricsButton.SetSensitive(false)
	w.metricsButton.ConnectClicked(w.openServiceMetrics)
	w.taskDefinitionsButton = gtk.NewButtonWithLabel("Task defs")
	w.taskDefinitionsButton.ConnectClicked(w.openTaskDefinitions)
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
	header.AddCSSClass("toolbar")
	header.Append(w.backButton)
	header.Append(title)
	header.Append(w.breadcrumb)
	header.Append(w.standaloneButton)
	header.Append(w.runTaskButton)
	header.Append(w.metricsButton)
	header.Append(w.taskDefinitionsButton)
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
	ecs := gtk.NewLabel("ECS")
	ecs.SetXAlign(0)
	ecs.AddCSSClass("active-mode")
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

	w.detailStack = gtk.NewStack()
	w.detailStack.SetVExpand(true)
	w.detailStack.SetHExpand(true)
	w.detailStack.AddNamed(detailScroll, "detail")
	w.detailStack.AddNamed(w.buildLogPane(), "logs")
	w.detailStack.AddNamed(w.buildMetricsPane(), "metrics")
	w.detailStack.AddNamed(w.buildTaskDefinitionPane(), "task-definition")
	w.detailStack.AddNamed(w.buildTaskDefinitionEditor(), "editor")
	w.detailStack.SetVisibleChildName("detail")

	contentSplit := gtk.NewPaned(gtk.OrientationHorizontal)
	contentSplit.SetStartChild(resourcePane)
	contentSplit.SetEndChild(w.detailStack)
	contentSplit.SetPosition(700)
	contentSplit.SetResizeStartChild(true)
	contentSplit.SetResizeEndChild(true)

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
	return root
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
		w.setDetail("KEYBOARD SHORTCUTS\n\nEnter          Open selected row or task\nEscape         Back / close auxiliary view\n/              Focus active filter\nCtrl+R         Refresh\nShift+S        Browse standalone tasks\nCtrl+Enter     Run standalone task\nShift+T        Browse task definitions\nE              Task-definition environment\nD              Diff previous revision\nCtrl+E         Edit task-definition JSON\nCtrl+S         Register edited revision\nM              Service metrics and alarms\nShift+L        Follow service logs\nCtrl+Shift+L   Follow selected task logs\nCtrl+Space     Pause/resume logs\nCtrl+Shift+C   Copy log buffer\nCtrl+L         Clear log buffer\nCtrl+Shift+S   Scale service\nCtrl+Shift+A   Toggle scale-in suspension\nCtrl+Shift+X   Stop selected task\nCtrl+Shift+R   Force deployment\nCtrl+P         Module switcher placeholder\n?              Show this help", detailHelp)
		w.detailStack.SetVisibleChildName("detail")
	})
	w.addAction(app, "logs", []string{"<Shift>l"}, w.openServiceLogs)
	w.addAction(app, "task-logs", []string{"<Control><Shift>l"}, w.openTaskLogs)
	w.addAction(app, "standalone-tasks", []string{"<Shift>s"}, w.loadStandaloneTasks)
	w.addAction(app, "run-task", []string{"<Control>Return"}, w.promptRunTask)
	w.addAction(app, "metrics", []string{"m"}, w.openServiceMetrics)
	w.addAction(app, "toggle-scale-in", []string{"<Control><Shift>a"}, w.confirmToggleScaleIn)
	w.addAction(app, "task-definitions", []string{"<Shift>t"}, w.openTaskDefinitions)
	w.addAction(app, "task-definition-env", []string{"e"}, w.openTaskDefinitionEnvironment)
	w.addAction(app, "task-definition-diff", []string{"d"}, w.openTaskDefinitionDiff)
	w.addAction(app, "task-definition-edit", []string{"<Control>e"}, w.openTaskDefinitionEditor)
	w.addAction(app, "task-definition-register", []string{"<Control>s"}, w.confirmRegisterTaskDefinition)
	w.addAction(app, "toggle-logs", []string{"<Control>space"}, w.toggleLogFollow)
	w.addAction(app, "copy-logs", []string{"<Control><Shift>c"}, w.copyLogs)
	w.addAction(app, "clear-logs", []string{"<Control>l"}, w.clearLogs)
	w.addAction(app, "force-deploy", []string{"<Control><Shift>r"}, w.confirmForceDeployment)
	w.addAction(app, "scale-service", []string{"<Control><Shift>s"}, w.promptScaleService)
	w.addAction(app, "stop-task", []string{"<Control><Shift>x"}, w.confirmStopTask)
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
	if w.detailText == text && w.detailContent == content {
		return
	}
	w.detailBuffer.SetText(text)
	w.detailText = text
	w.detailContent = content
}

func (w *mainWindow) loadClusters() {
	ctx, generation := w.startRequest("Loading ECS clusters…")
	go func() {
		clusters, err := w.options.ECS.ListClusters(ctx)
		w.finishRequest(ctx, generation, err, func() {
			w.allClusters = clusters
			w.currentPage = pageClusters
			w.selectedCluster = ""
			w.selectedService = ""
			w.selectedTask = ""
			w.updateActionSensitivity()
			w.breadcrumb.SetLabel("ECS / Clusters")
			w.backButton.SetSensitive(false)
			w.search.SetPlaceholderText("Filter clusters…")
			w.resourceStack.SetVisibleChildName(pageClusters)
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
	w.showingMetrics = false
	w.currentPage = pageServices
	w.selectedCluster = cluster
	w.selectedService = ""
	w.selectedTask = ""
	w.updateActionSensitivity()
	w.breadcrumb.SetLabel("ECS / " + cluster)
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
	w.showingMetrics = false
	w.currentPage = pageTasks
	w.selectedService = service.Name
	w.selectedTask = ""
	w.updateActionSensitivity()
	if !w.showingLogs {
		w.detailStack.SetVisibleChildName("detail")
	}
	w.breadcrumb.SetLabel("ECS / " + w.selectedCluster + " / " + service.Name)
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
	w.showingMetrics = false
	w.currentPage = pageStandaloneTasks
	w.selectedService = ""
	w.selectedTask = ""
	w.updateActionSensitivity()
	if !w.showingLogs {
		w.detailStack.SetVisibleChildName("detail")
	}
	w.breadcrumb.SetLabel("ECS / " + w.selectedCluster + " / Standalone tasks")
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
	w.selectedTask = task.TaskARN
	w.updateActionSensitivity()
	if w.currentPage == pageStandaloneTasks {
		w.breadcrumb.SetLabel("ECS / " + w.selectedCluster + " / Standalone tasks / " + shortID(task.TaskID))
	} else {
		w.breadcrumb.SetLabel("ECS / " + w.selectedCluster + " / " + w.selectedService + " / " + shortID(task.TaskID))
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
	if w.currentPage == pageTasks || w.currentPage == pageStandaloneTasks {
		if w.requestCancel != nil {
			w.requestCancel()
			w.generation++
		}
		w.spinner.Stop()
		w.currentPage = pageServices
		w.selectedService = ""
		w.selectedTask = ""
		w.allTasks = nil
		w.updateActionSensitivity()
		w.search.SetText("")
		w.search.SetPlaceholderText("Filter services…")
		w.resourceStack.SetVisibleChildName(pageServices)
		w.breadcrumb.SetLabel("ECS / " + w.selectedCluster)
		w.setDetail(clusterSummary(w.selectedCluster, len(w.allServices)), detailClusterSummary)
		w.applyServiceFilter()
		w.setStatus("Ready", false)
		return
	}
	if w.currentPage == pageTaskDefinitions {
		w.restoreTaskDefinitionReturnPage()
		return
	}
	if w.currentPage != pageServices {
		return
	}
	if w.requestCancel != nil {
		w.requestCancel()
		w.generation++
	}
	w.spinner.Stop()
	w.currentPage = pageClusters
	w.selectedCluster = ""
	w.selectedService = ""
	w.selectedTask = ""
	w.updateActionSensitivity()
	w.allServices = nil
	w.search.SetText("")
	w.search.SetPlaceholderText("Filter clusters…")
	w.resourceStack.SetVisibleChildName(pageClusters)
	w.breadcrumb.SetLabel("ECS / Clusters")
	w.backButton.SetSensitive(false)
	w.setDetail("Select a cluster and press Enter to browse its services.", detailIntro)
	w.applyClusterFilter()
	w.setStatus("Ready", false)
}

func (w *mainWindow) refresh() {
	w.refreshCurrent(true)
}

func (w *mainWindow) refreshCurrent(foreground bool) {
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
		w.loadServiceMetrics(foreground)
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
				w.breadcrumb.SetLabel("ECS / " + cluster + " / Standalone tasks")
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
				w.goBack()
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
				w.breadcrumb.SetLabel("ECS / " + cluster + " / " + serviceName)
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
				w.breadcrumb.SetLabel("ECS / " + cluster)
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
	clusterSelected := w.currentPage != pageClusters && w.selectedCluster != ""
	w.standaloneButton.SetSensitive(clusterSelected && !standalonePage)
	w.runTaskButton.SetSensitive(standalonePage)
	w.metricsButton.SetSensitive(serviceSelected)
	w.logsButton.SetSensitive(serviceSelected && w.options.Logs != nil)
	w.taskLogsButton.SetSensitive(taskSelected && w.options.Logs != nil)
	w.scaleButton.SetSensitive(serviceSelected)
	w.stopTaskButton.SetSensitive(taskSelected)
	w.deployButton.SetSensitive(serviceSelected)
}
