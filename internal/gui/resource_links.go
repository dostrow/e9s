//go:build gui

package gui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

type workspaceResourceLink struct {
	label string
	ref   model.ResourceRef
}

type detailResourceTag struct {
	tag *gtk.TextTag
	ref model.ResourceRef
}

// resourceNavigationState retains the browser context that a ResourceRef alone
// cannot reconstruct after crossing module boundaries. EC2 loaders deliberately
// clear ECS selections, so the originating cluster/service/task and breadcrumb
// must travel with the history entry.
type resourceNavigationState struct {
	ref                 model.ResourceRef
	page                string
	detailContent       string
	breadcrumb          string
	filter              string
	cluster             string
	service             string
	task                string
	showingStoppedTasks bool
}

func (w *mainWindow) clearDetailResourceLinks() {
	if w.detailLinks == nil {
		return
	}
	for _, resourceTag := range w.detailResourceTags {
		w.detailBuffer.TagTable().Remove(resourceTag.tag)
	}
	w.detailResourceTags = nil
	w.detailLinks.SetPopover(nil)
	w.detailLinks.SetVisible(false)
}

func (w *mainWindow) setDetailResourceLinks(links []workspaceResourceLink) {
	w.clearDetailResourceLinks()
	links = uniqueWorkspaceResourceLinks(links)
	menu := gtk.NewBox(gtk.OrientationVertical, 2)
	menu.SetMarginTop(6)
	menu.SetMarginBottom(6)
	menu.SetMarginStart(6)
	menu.SetMarginEnd(6)
	for index, link := range links {
		link := link
		button := gtk.NewButtonWithLabel(link.label)
		button.AddCSSClass("flat")
		button.SetHAlign(gtk.AlignFill)
		button.SetTooltipText(fmt.Sprintf("Open %s %s", link.ref.Kind, link.ref.ID))
		button.ConnectClicked(func() { w.openResourceLink(link.ref) })
		menu.Append(button)
		w.applyInlineResourceLink(link, index)
	}
	popover := gtk.NewPopover()
	popover.SetChild(menu)
	w.detailLinks.SetPopover(popover)
	w.detailLinks.SetLabel(fmt.Sprintf("Linked resources (%d)", len(links)))
	w.detailLinks.SetVisible(len(links) > 0)
}

func (w *mainWindow) applyInlineResourceLink(link workspaceResourceLink, index int) {
	needle := link.ref.ID
	if needle == "" || !strings.Contains(w.detailText, needle) {
		needle = link.ref.Name
	}
	if needle == "" {
		return
	}
	tag := gtk.NewTextTag(fmt.Sprintf("resource-link-%d", index))
	tag.SetObjectProperty("underline", int(pango.UnderlineSingle))
	tag.SetObjectProperty("weight", int(pango.WeightSemibold))
	color := w.detailView.StyleContext().Color()
	if candidate, ok := w.detailView.StyleContext().LookupColor("link_color"); ok {
		color = candidate
	}
	tag.SetObjectProperty("foreground", color.String())
	w.detailBuffer.TagTable().Add(tag)
	w.detailResourceTags = append(w.detailResourceTags, detailResourceTag{tag: tag, ref: link.ref})
	searchFrom := 0
	for {
		position := strings.Index(w.detailText[searchFrom:], needle)
		if position < 0 {
			break
		}
		byteStart := searchFrom + position
		byteEnd := byteStart + len(needle)
		start := utf8.RuneCountInString(w.detailText[:byteStart])
		end := start + utf8.RuneCountInString(needle)
		w.detailBuffer.ApplyTag(tag, w.detailBuffer.IterAtOffset(start), w.detailBuffer.IterAtOffset(end))
		searchFrom = byteEnd
	}
}

func (w *mainWindow) renderServiceOverview(svc model.Service) {
	w.renderServiceText(formatServiceOverview(w.selectedCluster, svc), svc)
}

func (w *mainWindow) renderServiceTaskSummary(svc model.Service, prefix string) {
	text := w.serviceTaskSummary(svc)
	if prefix != "" {
		text = prefix + "\n\n" + text
	}
	w.renderServiceText(text, svc)
}

func (w *mainWindow) renderServiceText(text string, svc model.Service) {
	refs := service.ECSServiceResourceRefs(svc)
	w.setDetail(text, detailService)
	links := make([]workspaceResourceLink, 0, len(refs))
	for _, ref := range refs {
		links = append(links, workspaceResourceLink{label: resourceRefLabel(ref), ref: ref})
	}
	w.setDetailResourceLinks(links)
}

func (w *mainWindow) installDetailLinkControllers() {
	click := gtk.NewGestureClick()
	click.SetButton(1)
	click.ConnectPressed(func(_ int, x, y float64) {
		if ref, ok := w.detailResourceAt(x, y); ok {
			w.openResourceLink(ref)
		}
	})
	w.detailView.AddController(click)
	motion := gtk.NewEventControllerMotion()
	motion.ConnectMotion(func(x, y float64) {
		if _, ok := w.detailResourceAt(x, y); ok {
			w.detailView.SetCursorFromName("pointer")
		} else {
			w.detailView.SetCursorFromName("default")
		}
	})
	motion.ConnectLeave(func() { w.detailView.SetCursorFromName("default") })
	w.detailView.AddController(motion)
}

func (w *mainWindow) detailResourceAt(x, y float64) (model.ResourceRef, bool) {
	bufferX, bufferY := w.detailView.WindowToBufferCoords(gtk.TextWindowWidget, int(x), int(y))
	iter, ok := w.detailView.IterAtLocation(bufferX, bufferY)
	if !ok {
		return model.ResourceRef{}, false
	}
	for _, resourceTag := range w.detailResourceTags {
		if iter.HasTag(resourceTag.tag) {
			return resourceTag.ref, true
		}
	}
	return model.ResourceRef{}, false
}

func (w *mainWindow) openResourceLink(ref model.ResourceRef) {
	if origin, ok := w.currentResourceNavigationState(); ok && (origin.ref.Kind != ref.Kind || origin.ref.ID != ref.ID) {
		w.resourceHistory = append(w.resourceHistory, origin)
	}
	w.navigateResourceRef(ref)
}

func (w *mainWindow) currentResourceNavigationState() (resourceNavigationState, bool) {
	ref, ok := w.currentResourceRef()
	if !ok {
		return resourceNavigationState{}, false
	}
	filter := ""
	if w.search != nil {
		filter = w.search.Text()
	}
	return resourceNavigationState{
		ref:                 ref,
		page:                w.currentPage,
		detailContent:       w.detailContent,
		breadcrumb:          w.breadcrumbText,
		filter:              filter,
		cluster:             w.selectedCluster,
		service:             w.selectedService,
		task:                w.selectedTask,
		showingStoppedTasks: w.showingStoppedTasks,
	}, true
}

func (w *mainWindow) navigateResourceRef(ref model.ResourceRef) {
	navigated := true
	switch ref.Kind {
	case "ec2-instance":
		w.loadEC2InstancesAt(ref.ID)
	case "ec2-security-group":
		w.loadEC2SecurityGroups(ref.ID)
	case "ec2-vpc":
		w.loadEC2VPCs(ref.ID)
	case "ec2-subnet":
		w.loadEC2Subnets(ref.ID, "")
	case "ec2-subnets":
		w.loadEC2Subnets("", ref.ID)
	case "ec2-volume":
		w.loadEC2Volumes(ref.ID)
	case "ec2-load-balancer":
		w.loadEC2LoadBalancers(ref.ID)
	case "ec2-target-group":
		w.loadEC2TargetGroups(ref.ID)
	case "ecs-task":
		w.restoreECSTask(ref.ID)
	case "rds-instance":
		w.loadRDSInstancesAt(ref.ID)
	case "rds-cluster":
		w.loadRDSClusters(ref.ID)
	default:
		navigated = false
		w.setStatus("Navigation is not implemented for "+ref.Kind, true)
	}
	if navigated {
		// Resource loaders set currentPage synchronously, even when the resource
		// itself is fetched asynchronously. updateActionSensitivity selects the
		// matching sub-item; revealing the parent makes that state visible.
		w.updateActionSensitivity()
		w.revealModuleForPage(w.currentPage)
	}
}

func (w *mainWindow) currentResourceRef() (model.ResourceRef, bool) {
	switch w.currentPage {
	case pageServices:
		if w.detailContent == detailService && w.selectedService != "" {
			return model.ResourceRef{Kind: "ecs-service", ID: w.selectedService}, true
		}
	case pageTasks:
		if w.detailContent == detailTask && w.selectedTask != "" {
			return model.ResourceRef{Kind: "ecs-task", ID: w.selectedTask}, true
		}
		if w.detailContent == detailService && w.selectedService != "" {
			return model.ResourceRef{Kind: "ecs-service", ID: w.selectedService}, true
		}
	case pageStandaloneTasks, pageStoppedTasks:
		if w.detailContent == detailTask && w.selectedTask != "" {
			return model.ResourceRef{Kind: "ecs-task", ID: w.selectedTask}, true
		}
	case pageEC2Instances:
		if w.selectedEC2Instance != "" {
			return model.ResourceRef{Kind: "ec2-instance", ID: w.selectedEC2Instance}, true
		}
	case pageEC2SecurityGroups:
		if w.selectedEC2SecurityGroup != "" {
			return model.ResourceRef{Kind: "ec2-security-group", ID: w.selectedEC2SecurityGroup}, true
		}
	case pageEC2VPCs:
		if w.selectedEC2VPC != "" {
			return model.ResourceRef{Kind: "ec2-vpc", ID: w.selectedEC2VPC}, true
		}
	case pageEC2Subnets:
		if w.selectedEC2Subnet != "" {
			return model.ResourceRef{Kind: "ec2-subnet", ID: w.selectedEC2Subnet}, true
		}
	case pageEC2Volumes:
		if w.selectedEC2Volume != "" {
			return model.ResourceRef{Kind: "ec2-volume", ID: w.selectedEC2Volume}, true
		}
	case pageEC2LoadBalancers:
		if w.selectedEC2LoadBalancer != "" {
			return model.ResourceRef{Kind: "ec2-load-balancer", ID: w.selectedEC2LoadBalancer}, true
		}
	case pageEC2TargetGroups:
		if w.selectedEC2TargetGroup != "" {
			return model.ResourceRef{Kind: "ec2-target-group", ID: w.selectedEC2TargetGroup}, true
		}
	case pageRDSInstances:
		if w.selectedRDSInstance != "" {
			return model.ResourceRef{Kind: "rds-instance", ID: w.selectedRDSInstance}, true
		}
	case pageRDSClusters:
		if w.selectedRDSCluster != "" {
			return model.ResourceRef{Kind: "rds-cluster", ID: w.selectedRDSCluster}, true
		}
	}
	return model.ResourceRef{}, false
}

func (w *mainWindow) renderTaskDetail(task model.Task) {
	var parent *model.Service
	parentName := w.selectedService
	if strings.HasPrefix(task.Group, "service:") {
		parentName = strings.TrimPrefix(task.Group, "service:")
	}
	if serviceValue, found := findService(w.allServices, parentName); found {
		parent = &serviceValue
	}
	refs := service.ECSTaskResourceRefs(task, parent)
	text := formatTaskDetail(task)
	if parent != nil {
		var context strings.Builder
		fmt.Fprintf(&context, "\n\nSERVICE CONTEXT\n  Parent service         %s", parent.Name)
		if len(parent.TargetGroups) == 0 {
			context.WriteString("\n  Configured target groups  none returned by ECS")
		} else {
			for index, targetGroup := range parent.TargetGroups {
				label := ""
				if index == 0 {
					label = "Configured target groups"
				}
				fmt.Fprintf(&context, "\n  %-22s %s", label, targetGroup.ID)
			}
		}
		text += context.String()
	}
	if len(refs) > 0 {
		var related strings.Builder
		related.WriteString("\n\nRELATED RESOURCES")
		for _, ref := range refs {
			fmt.Fprintf(&related, "\n  %-22s %s", strings.ReplaceAll(ref.Kind, "-", " "), ref.ID)
		}
		text += related.String()
	}
	w.setDetail(text, detailTask)
	links := make([]workspaceResourceLink, 0, len(refs))
	for _, ref := range refs {
		links = append(links, workspaceResourceLink{label: resourceRefLabel(ref), ref: ref})
	}
	w.setDetailResourceLinks(links)
}

func resourceRefLabel(ref model.ResourceRef) string {
	kind := strings.ReplaceAll(ref.Kind, "-", " ")
	name := ref.Name
	if name == "" {
		name = ref.ID
	} else if name != ref.ID {
		name += " (" + ref.ID + ")"
	}
	return kind + ": " + name
}

func (w *mainWindow) restoreECSTask(taskARN string) {
	task, found := findTask(w.allTasks, taskARN)
	if !found {
		w.setStatus("The ECS task is no longer available in the current browser buffer", true)
		return
	}
	if w.selectedService != "" {
		w.currentPage = pageTasks
		w.resourceStack.SetVisibleChildName(pageTasks)
		w.setBreadcrumb(w.serviceTaskBreadcrumb() + " / " + shortID(task.TaskID))
		w.search.SetPlaceholderText("Filter tasks…")
	} else if w.showingStoppedTasks {
		w.currentPage = pageStoppedTasks
		w.resourceStack.SetVisibleChildName(pageStoppedTasks)
		w.setBreadcrumb(w.standaloneTaskBreadcrumb() + " / " + shortID(task.TaskID))
		w.search.SetPlaceholderText("Filter stopped tasks…")
	} else {
		w.currentPage = pageStandaloneTasks
		w.resourceStack.SetVisibleChildName(pageStandaloneTasks)
		w.setBreadcrumb(w.standaloneTaskBreadcrumb() + " / " + shortID(task.TaskID))
		w.search.SetPlaceholderText("Filter tasks…")
	}
	w.selectedTask = taskARN
	w.renderTaskDetail(task)
	w.backButton.SetSensitive(true)
	w.updateActionSensitivity()
}

func (w *mainWindow) navigateResourceHistoryBack() bool {
	if len(w.resourceHistory) == 0 {
		return false
	}
	last := len(w.resourceHistory) - 1
	state := w.resourceHistory[last]
	w.resourceHistory = w.resourceHistory[:last]
	if isECSPage(state.page) {
		w.restoreECSResourceNavigationState(state)
	} else {
		w.navigateResourceRef(state.ref)
	}
	return true
}

func (w *mainWindow) restoreECSResourceNavigationState(state resourceNavigationState) {
	w.resetWorkspaceForBrowserChange()
	w.currentPage = state.page
	w.selectedCluster = state.cluster
	w.selectedService = state.service
	w.selectedTask = state.task
	w.showingStoppedTasks = state.showingStoppedTasks
	w.search.SetText(state.filter)
	w.backButton.SetSensitive(true)

	switch state.page {
	case pageServices:
		w.search.SetPlaceholderText("Filter services…")
		w.resourceStack.SetVisibleChildName(pageServices)
		w.applyServiceFilter()
		if svc, found := findService(w.allServices, state.service); found {
			w.renderServiceOverview(svc)
			for index, candidate := range w.filteredServices {
				if candidate.Name == state.service {
					w.serviceTable.selection.SetSelected(uint(index))
					break
				}
			}
		} else {
			w.selectedService = ""
			w.setDetail(clusterSummary(state.cluster, len(w.allServices)), detailClusterSummary)
		}
	case pageTasks:
		w.restoreServiceTaskBrowser(state)
	case pageStandaloneTasks, pageStoppedTasks:
		w.restoreStandaloneTaskBrowser(state)
	default:
		// ECS resource links currently originate only from service and task
		// details. Fall back to the normal ECS landing view if that expands.
		w.loadClusters()
		return
	}

	if state.breadcrumb != "" {
		w.setBreadcrumb(state.breadcrumb)
	}
	w.updateActionSensitivity()
	w.revealModuleForPage(w.currentPage)
	w.setStatus("Ready", false)
}

func (w *mainWindow) restoreServiceTaskBrowser(state resourceNavigationState) {
	w.activeTasksButton.SetActive(!state.showingStoppedTasks)
	w.stoppedTasksButton.SetActive(state.showingStoppedTasks)
	if state.showingStoppedTasks {
		w.search.SetPlaceholderText("Filter recently stopped service tasks…")
		w.resourceStack.SetVisibleChildName(pageStoppedTasks)
	} else {
		w.search.SetPlaceholderText("Filter active service tasks…")
		w.resourceStack.SetVisibleChildName(pageTasks)
	}
	w.applyTaskFilter()
	if state.task != "" {
		if task, found := findTask(w.allTasks, state.task); found {
			w.renderTaskDetail(task)
			w.selectRestoredTaskRow(state.task)
			return
		}
	}
	w.selectedTask = ""
	if svc, found := findService(w.allServices, state.service); found {
		w.renderServiceTaskSummary(svc, "")
	} else {
		w.setDetail("The selected service is no longer available.", detailError)
	}
}

func (w *mainWindow) restoreStandaloneTaskBrowser(state resourceNavigationState) {
	w.activeTasksButton.SetActive(!state.showingStoppedTasks)
	w.stoppedTasksButton.SetActive(state.showingStoppedTasks)
	if state.showingStoppedTasks {
		w.search.SetPlaceholderText("Filter recently stopped tasks…")
		w.resourceStack.SetVisibleChildName(pageStoppedTasks)
	} else {
		w.search.SetPlaceholderText("Filter active standalone tasks…")
		w.resourceStack.SetVisibleChildName(pageTasks)
	}
	w.applyTaskFilter()
	if state.task != "" {
		if task, found := findTask(w.allTasks, state.task); found {
			w.renderTaskDetail(task)
			w.selectRestoredTaskRow(state.task)
			return
		}
	}
	w.selectedTask = ""
	w.setDetail(w.standaloneTaskSummary(), detailClusterSummary)
}

func (w *mainWindow) selectRestoredTaskRow(taskARN string) {
	for index, task := range w.filteredTasks {
		if task.TaskARN != taskARN {
			continue
		}
		if w.showingStoppedTasks {
			w.stoppedTaskTable.selection.SetSelected(uint(index))
		} else {
			w.taskTable.selection.SetSelected(uint(index))
		}
		return
	}
}

func (w *mainWindow) setEC2InstanceResourceLinks(detail model.EC2InstanceDetail) {
	links := make([]workspaceResourceLink, 0, len(detail.SecurityGroups)+2)
	if detail.VpcID != "" {
		links = append(links, workspaceResourceLink{label: "VPC: " + detail.VpcID,
			ref: model.ResourceRef{Kind: "ec2-vpc", ID: detail.VpcID}})
	}
	if detail.SubnetID != "" {
		links = append(links, workspaceResourceLink{label: "Subnet: " + detail.SubnetID,
			ref: model.ResourceRef{Kind: "ec2-subnet", ID: detail.SubnetID}})
	}
	for _, group := range detail.SecurityGroups {
		label := group.Name
		if label == "" {
			label = group.ID
		} else {
			label += " (" + group.ID + ")"
		}
		links = append(links, workspaceResourceLink{
			label: "Security group: " + label,
			ref:   model.ResourceRef{Kind: "ec2-security-group", ID: group.ID, Name: group.Name},
		})
	}
	for _, volume := range detail.Volumes {
		if volume.VolumeID != "" {
			links = append(links, workspaceResourceLink{label: "Volume: " + volume.VolumeID, ref: model.ResourceRef{Kind: "ec2-volume", ID: volume.VolumeID}})
		}
	}
	w.setDetailResourceLinks(links)
}
