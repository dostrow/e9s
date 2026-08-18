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
	pageClusters = "clusters"
	pageServices = "services"
)

type mainWindow struct {
	ctx     context.Context
	options Options
	window  *gtk.ApplicationWindow

	requestCancel context.CancelFunc
	generation    uint64
	logCancel     context.CancelFunc
	logGeneration uint64

	currentPage        string
	selectedCluster    string
	selectedService    string
	pendingService     string
	allClusters        []model.Cluster
	filteredClusters   []model.Cluster
	allServices        []model.Service
	filteredServices   []model.Service
	clusterTable       *stringTable
	serviceTable       *stringTable
	resourceStack      *gtk.Stack
	search             *gtk.SearchEntry
	backButton         *gtk.Button
	logsButton         *gtk.Button
	deployButton       *gtk.Button
	breadcrumb         *gtk.Label
	detailBuffer       *gtk.TextBuffer
	detailStack        *gtk.Stack
	logView            *gtk.TextView
	logTextBuffer      *gtk.TextBuffer
	logSearch          *gtk.SearchEntry
	logPauseButton     *gtk.Button
	logStore           *boundedLogs
	logSource          model.LogSource
	logLastTS          int64
	logFollowing       bool
	showingLogs        bool
	status             *gtk.Label
	spinner            *gtk.Spinner
	lastSuccessfulLoad time.Time
}

func newMainWindow(ctx context.Context, app *gtk.Application, options Options) *mainWindow {
	w := &mainWindow{
		ctx:         ctx,
		options:     options,
		currentPage: pageClusters,
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
	w.clusterTable.view.ConnectActivate(w.openClusterAt)
	w.serviceTable.view.ConnectActivate(w.openServiceAt)

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
	w.logsButton = gtk.NewButtonWithLabel("Logs")
	w.logsButton.SetSensitive(false)
	w.logsButton.ConnectClicked(w.openServiceLogs)
	w.deployButton = gtk.NewButtonWithLabel("Force deploy")
	w.deployButton.SetSensitive(false)
	w.deployButton.AddCSSClass("destructive-action")
	w.deployButton.ConnectClicked(w.confirmForceDeployment)

	header := gtk.NewBox(gtk.OrientationHorizontal, 10)
	header.AddCSSClass("toolbar")
	header.Append(w.backButton)
	header.Append(title)
	header.Append(w.breadcrumb)
	header.Append(w.logsButton)
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

	w.resourceStack = gtk.NewStack()
	w.resourceStack.SetVExpand(true)
	w.resourceStack.SetHExpand(true)
	w.resourceStack.AddNamed(clusterScroll, pageClusters)
	w.resourceStack.AddNamed(serviceScroll, pageServices)
	w.resourceStack.SetVisibleChildName(pageClusters)

	resourcePane := gtk.NewBox(gtk.OrientationVertical, 8)
	resourcePane.AddCSSClass("resource-pane")
	resourcePane.Append(w.search)
	resourcePane.Append(w.resourceStack)

	w.detailBuffer = gtk.NewTextBuffer(nil)
	w.detailBuffer.SetText("Select a cluster and press Enter to browse its services.")
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
		w.detailBuffer.SetText("KEYBOARD SHORTCUTS\n\nEnter          Open selected row\nEscape         Back / close logs\n/              Focus active filter\nCtrl+R         Refresh\nShift+L        Follow service logs\nCtrl+Space     Pause/resume logs\nCtrl+Shift+C   Copy log buffer\nCtrl+L         Clear log buffer\nCtrl+Shift+R   Force deployment\nCtrl+P         Module switcher placeholder\n?              Show this help")
		w.detailStack.SetVisibleChildName("detail")
	})
	w.addAction(app, "logs", []string{"<Shift>l"}, w.openServiceLogs)
	w.addAction(app, "toggle-logs", []string{"<Control>space"}, w.toggleLogFollow)
	w.addAction(app, "copy-logs", []string{"<Control><Shift>c"}, w.copyLogs)
	w.addAction(app, "clear-logs", []string{"<Control>l"}, w.clearLogs)
	w.addAction(app, "force-deploy", []string{"<Control><Shift>r"}, w.confirmForceDeployment)
}

func (w *mainWindow) addAction(app *gtk.Application, name string, accels []string, run func()) {
	action := gio.NewSimpleAction(name, nil)
	action.ConnectActivate(func(_ *glib.Variant) { run() })
	app.AddAction(action)
	app.SetAccelsForAction("app."+name, accels)
}

func (w *mainWindow) startRequest(label string) (context.Context, uint64) {
	if w.requestCancel != nil {
		w.requestCancel()
	}
	ctx, cancel := context.WithCancel(w.ctx)
	w.requestCancel = cancel
	w.generation++
	w.spinner.Start()
	w.setStatus(label, false)
	return ctx, w.generation
}

func (w *mainWindow) finishRequest(ctx context.Context, generation uint64, err error, apply func()) {
	w.finishRequestWithStatus(ctx, generation, err, "", apply)
}

func (w *mainWindow) finishRequestWithStatus(ctx context.Context, generation uint64, err error, success string, apply func()) {
	glib.IdleAdd(func() {
		if ctx.Err() != nil || generation != w.generation {
			return
		}
		w.spinner.Stop()
		if err != nil {
			w.setStatus(err.Error(), true)
			w.detailBuffer.SetText("ERROR\n\n" + err.Error())
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

func (w *mainWindow) loadClusters() {
	ctx, generation := w.startRequest("Loading ECS clusters…")
	go func() {
		clusters, err := w.options.ECS.ListClusters(ctx)
		w.finishRequest(ctx, generation, err, func() {
			w.allClusters = clusters
			w.currentPage = pageClusters
			w.selectedCluster = ""
			w.selectedService = ""
			w.logsButton.SetSensitive(false)
			w.deployButton.SetSensitive(false)
			w.breadcrumb.SetLabel("ECS / Clusters")
			w.backButton.SetSensitive(false)
			w.search.SetPlaceholderText("Filter clusters…")
			w.resourceStack.SetVisibleChildName(pageClusters)
			w.applyClusterFilter()
			if len(clusters) == 0 {
				w.detailBuffer.SetText("No ECS clusters found in this region.")
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
	w.currentPage = pageServices
	w.selectedCluster = cluster
	w.selectedService = ""
	w.logsButton.SetSensitive(false)
	w.deployButton.SetSensitive(false)
	w.breadcrumb.SetLabel("ECS / " + cluster)
	w.backButton.SetSensitive(true)
	w.search.SetPlaceholderText("Filter services…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageServices)
	w.detailBuffer.SetText("Loading services for " + cluster + "…")

	ctx, generation := w.startRequest("Loading services in " + cluster + "…")
	go func() {
		services, err := w.options.ECS.ListServices(ctx, cluster)
		w.finishRequest(ctx, generation, err, func() {
			w.allServices = services
			w.applyServiceFilter()
			if len(services) == 0 {
				w.detailBuffer.SetText("No ECS services found in " + cluster + ".")
			} else {
				w.detailBuffer.SetText(fmt.Sprintf("%s\n\n%d services\n\nSelect a service and press Enter for deployments, tasks, and recent events.", cluster, len(services)))
			}
			if w.pendingService != "" {
				name := w.pendingService
				w.pendingService = ""
				w.openServiceByName(name)
			}
		})
	}()
}

func (w *mainWindow) loadServiceDetail(service model.Service) {
	w.selectedService = service.Name
	w.logsButton.SetSensitive(true)
	w.deployButton.SetSensitive(true)
	if !w.showingLogs {
		w.detailStack.SetVisibleChildName("detail")
	}
	w.breadcrumb.SetLabel("ECS / " + w.selectedCluster + " / " + service.Name)
	w.detailBuffer.SetText("Loading tasks and service details…")

	ctx, generation := w.startRequest("Loading " + service.Name + "…")
	cluster := w.selectedCluster
	go func() {
		tasks, err := w.options.ECS.ListTasks(ctx, cluster, service.Name)
		w.finishRequest(ctx, generation, err, func() {
			w.detailBuffer.SetText(formatServiceDetail(cluster, service, tasks))
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
	w.loadServiceDetail(w.filteredServices[position])
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

func (w *mainWindow) openServiceByName(name string) {
	for _, service := range w.allServices {
		if service.Name == name {
			w.loadServiceDetail(service)
			return
		}
	}
}

func (w *mainWindow) applyFilter() {
	if w.currentPage == pageServices {
		w.applyServiceFilter()
		return
	}
	w.applyClusterFilter()
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
	if w.showingLogs {
		w.closeLogs()
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
	w.logsButton.SetSensitive(false)
	w.deployButton.SetSensitive(false)
	w.allServices = nil
	w.search.SetText("")
	w.search.SetPlaceholderText("Filter clusters…")
	w.resourceStack.SetVisibleChildName(pageClusters)
	w.breadcrumb.SetLabel("ECS / Clusters")
	w.backButton.SetSensitive(false)
	w.detailBuffer.SetText("Select a cluster and press Enter to browse its services.")
	w.applyClusterFilter()
	w.setStatus("Ready", false)
}

func (w *mainWindow) refresh() {
	if w.currentPage == pageServices && w.selectedCluster != "" {
		w.pendingService = w.selectedService
		w.loadServices(w.selectedCluster)
		return
	}
	w.loadClusters()
}

func (w *mainWindow) scheduleRefresh() {
	glib.IdleAdd(func() {
		if w.ctx.Err() == nil {
			w.refresh()
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
