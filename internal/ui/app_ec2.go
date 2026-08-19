package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
	"github.com/dostrow/e9s/internal/ui/views"
)

// --- EC2 ---

func (a App) openEC2Instances(filter string) (App, tea.Cmd) {
	a.resourceHistory = nil
	a.mode = modeEC2
	a.state = viewEC2Instances
	a.ec2InstancesView = views.NewEC2Instances()
	a.ec2InstancesView = a.ec2InstancesView.SetSize(a.width-3, a.height-6)
	a.loading = true
	ec2Service := a.ec2
	ctx := a.ctx
	return a, func() tea.Msg {
		instances, err := ec2Service.List(ctx, filter)
		if err != nil {
			return errMsg{err}
		}
		return ec2InstancesLoadedMsg{instances}
	}
}

func (a App) newTaskDetail(task *model.Task) views.TaskDetailModel {
	if task == nil {
		return views.NewTaskDetail(nil)
	}
	return views.NewTaskDetail(task).SetResourceRefs(service.ECSTaskResourceRefs(*task, a.selectedService))
}

func (a App) openEC2SecurityGroups() (App, tea.Cmd) {
	a.mode = modeEC2
	a.state = viewEC2SecurityGroups
	a.resourceHistory = nil
	a.ec2SecurityGroupsView = views.NewEC2SecurityGroups().SetSize(a.width-3, a.height-6)
	a.loading = true
	ec2Network := a.ec2Network
	ctx := a.ctx
	return a, func() tea.Msg {
		groups, err := ec2Network.SecurityGroups(ctx, "", "")
		if err != nil {
			return errMsg{err}
		}
		return ec2SecurityGroupsLoadedMsg{groups}
	}
}

func (a App) openEC2SecurityGroupDetail() (App, tea.Cmd) {
	group := a.ec2SecurityGroupsView.SelectedGroup()
	if group == nil {
		return a, nil
	}
	return a.navigateEC2Resource(model.ResourceRef{Kind: "ec2-security-group", ID: group.GroupID}, false)
}

func (a App) refreshEC2SecurityGroups() tea.Cmd {
	ec2Network := a.ec2Network
	ctx := a.ctx
	return func() tea.Msg {
		groups, err := ec2Network.SecurityGroups(ctx, "", "")
		if err != nil {
			return errMsg{err}
		}
		return ec2SecurityGroupsLoadedMsg{groups}
	}
}

func (a App) refreshEC2SecurityGroupDetail() tea.Cmd {
	group := a.ec2SecurityGroupDetailView.Group()
	if group == nil {
		return nil
	}
	ec2Network := a.ec2Network
	ctx := a.ctx
	groupID := group.GroupID
	return func() tea.Msg {
		detail, err := ec2Network.SecurityGroup(ctx, groupID)
		if err != nil {
			return errMsg{err}
		}
		return ec2SecurityGroupLoadedMsg{detail}
	}
}

func (a App) switchEC2Resource() (App, tea.Cmd) {
	switch a.state {
	case viewEC2Instances, viewEC2Detail, viewEC2Console:
		return a.openEC2LoadBalancers()
	case viewEC2LoadBalancers, viewEC2LoadBalancerDetail:
		return a.openEC2TargetGroups()
	case viewEC2TargetGroups, viewEC2TargetGroupDetail:
		return a.openEC2SecurityGroups()
	case viewEC2SecurityGroups, viewEC2SecurityGroupDetail:
		return a.openEC2VPCs()
	case viewEC2VPCs, viewEC2VPCDetail:
		return a.openEC2Subnets("")
	case viewEC2Subnets, viewEC2SubnetDetail:
		return a.openEC2Volumes()
	case viewEC2Volumes, viewEC2VolumeDetail:
		return a.openEC2Instances("")
	}
	return a.openEC2Instances("")
}

func (a App) openEC2ResourcePicker() (App, tea.Cmd) {
	links := a.currentEC2ResourceLinks()
	if len(links) == 0 {
		a.flashMessage = "No implemented linked resources are available"
		a.flashExpiry = time.Now().Add(3 * time.Second)
		return a, nil
	}
	items := make([]string, len(links))
	for index, link := range links {
		label := link.Name
		if label == "" {
			label = link.ID
		} else if link.ID != "" && link.ID != link.Name {
			label += " (" + link.ID + ")"
		}
		items[index] = strings.ReplaceAll(link.Kind, "-", " ") + ": " + label
	}
	a.resourceLinks = links
	a.picker = NewPicker(PickerResourceLink, "Open linked resource", items)
	return a, nil
}

func (a App) currentEC2ResourceLinks() []model.ResourceRef {
	switch a.state {
	case viewTaskDetail:
		task := a.detailView.Task()
		if task == nil {
			return nil
		}
		return service.ECSTaskResourceRefs(*task, a.selectedService)
	case viewRDSDetail:
		detail := a.rdsDetailView.Detail()
		if detail == nil {
			return nil
		}
		refs := []model.ResourceRef{}
		if detail.VPCID != "" {
			refs = append(refs, model.ResourceRef{Kind: "ec2-vpc", ID: detail.VPCID})
		}
		for _, groupID := range detail.SecurityGroups {
			refs = append(refs, model.ResourceRef{Kind: "ec2-security-group", ID: groupID})
		}
		return service.UniqueResourceRefs(refs)
	case viewEC2Detail:
		detail := a.ec2DetailView.Detail()
		if detail == nil {
			return nil
		}
		links := make([]model.ResourceRef, 0, len(detail.SecurityGroups)+len(detail.Volumes)+2)
		if detail.VpcID != "" {
			links = append(links, model.ResourceRef{Kind: "ec2-vpc", ID: detail.VpcID})
		}
		if detail.SubnetID != "" {
			links = append(links, model.ResourceRef{Kind: "ec2-subnet", ID: detail.SubnetID})
		}
		for _, group := range detail.SecurityGroups {
			links = append(links, model.ResourceRef{Kind: "ec2-security-group", ID: group.ID, Name: group.Name})
		}
		for _, volume := range detail.Volumes {
			if volume.VolumeID != "" {
				links = append(links, model.ResourceRef{Kind: "ec2-volume", ID: volume.VolumeID, Name: volume.Name})
			}
		}
		return links
	case viewEC2SecurityGroupDetail:
		group := a.ec2SecurityGroupDetailView.Group()
		if group == nil {
			return nil
		}
		links := make([]model.ResourceRef, 0, len(group.Associations)+1)
		if group.VpcID != "" {
			links = append(links, model.ResourceRef{Kind: "ec2-vpc", ID: group.VpcID})
		}
		for _, association := range group.Associations {
			if association.Kind == "ec2-instance" {
				links = append(links, association)
			}
		}
		return links
	case viewEC2VPCDetail:
		if a.ec2VPCDetail != nil {
			return []model.ResourceRef{{Kind: "ec2-subnets", ID: a.ec2VPCDetail.VpcID, Name: "Subnets in " + a.ec2VPCDetail.VpcID}}
		}
	case viewEC2SubnetDetail:
		if a.ec2SubnetDetail != nil && a.ec2SubnetDetail.VpcID != "" {
			return []model.ResourceRef{{Kind: "ec2-vpc", ID: a.ec2SubnetDetail.VpcID}}
		}
	case viewEC2VolumeDetail:
		if a.ec2VolumeDetail != nil {
			links := make([]model.ResourceRef, 0, len(a.ec2VolumeDetail.Attachments))
			for _, attachment := range a.ec2VolumeDetail.Attachments {
				if attachment.InstanceID != "" {
					links = append(links, model.ResourceRef{Kind: "ec2-instance", ID: attachment.InstanceID})
				}
			}
			return links
		}
	case viewEC2LoadBalancerDetail:
		if a.ec2LoadBalancerDetail != nil {
			loadBalancer := a.ec2LoadBalancerDetail
			links := []model.ResourceRef{}
			if loadBalancer.VpcID != "" {
				links = append(links, model.ResourceRef{Kind: "ec2-vpc", ID: loadBalancer.VpcID})
			}
			for _, zone := range loadBalancer.AvailabilityZones {
				if zone.SubnetID != "" {
					links = append(links, model.ResourceRef{Kind: "ec2-subnet", ID: zone.SubnetID})
				}
			}
			for _, groupID := range loadBalancer.SecurityGroups {
				links = append(links, model.ResourceRef{Kind: "ec2-security-group", ID: groupID})
			}
			for _, targetGroup := range loadBalancer.TargetGroups {
				links = append(links, model.ResourceRef{Kind: "ec2-target-group", ID: targetGroup.ARN, Name: targetGroup.Name})
				for _, target := range targetGroup.Targets {
					if targetGroup.TargetType == "instance" && strings.HasPrefix(target.ID, "i-") {
						links = append(links, model.ResourceRef{Kind: "ec2-instance", ID: target.ID})
					}
				}
			}
			return uniqueResourceRefs(links)
		}
	case viewEC2TargetGroupDetail:
		if a.ec2TargetGroupDetail != nil {
			targetGroup := a.ec2TargetGroupDetail
			links := []model.ResourceRef{}
			if targetGroup.VpcID != "" {
				links = append(links, model.ResourceRef{Kind: "ec2-vpc", ID: targetGroup.VpcID})
			}
			for _, arn := range targetGroup.LoadBalancerARNs {
				links = append(links, model.ResourceRef{Kind: "ec2-load-balancer", ID: arn})
			}
			if targetGroup.TargetType == "instance" {
				for _, target := range targetGroup.Targets {
					if strings.HasPrefix(target.ID, "i-") {
						links = append(links, model.ResourceRef{Kind: "ec2-instance", ID: target.ID})
					}
				}
			}
			return uniqueResourceRefs(links)
		}
	}
	return nil
}

func uniqueResourceRefs(refs []model.ResourceRef) []model.ResourceRef {
	seen := make(map[string]bool, len(refs))
	unique := make([]model.ResourceRef, 0, len(refs))
	for _, ref := range refs {
		key := ref.Kind + "\x00" + ref.ID
		if ref.ID == "" || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, ref)
	}
	return unique
}

func (a App) currentEC2ResourceRef() (model.ResourceRef, bool) {
	switch a.state {
	case viewTaskDetail:
		if task := a.detailView.Task(); task != nil {
			return model.ResourceRef{Kind: "ecs-task", ID: task.TaskARN, Name: task.TaskID}, true
		}
	case viewRDSDetail:
		if detail := a.rdsDetailView.Detail(); detail != nil {
			return model.ResourceRef{Kind: "rds-instance", ID: detail.Identifier}, true
		}
	case viewEC2Detail:
		if detail := a.ec2DetailView.Detail(); detail != nil {
			return model.ResourceRef{Kind: "ec2-instance", ID: detail.InstanceID, Name: detail.Name}, true
		}
	case viewEC2SecurityGroupDetail:
		if group := a.ec2SecurityGroupDetailView.Group(); group != nil {
			return model.ResourceRef{Kind: "ec2-security-group", ID: group.GroupID, Name: group.Name}, true
		}
	case viewEC2VPCDetail:
		if a.ec2VPCDetail != nil {
			return model.ResourceRef{Kind: "ec2-vpc", ID: a.ec2VPCDetail.VpcID, Name: a.ec2VPCDetail.Name}, true
		}
	case viewEC2SubnetDetail:
		if a.ec2SubnetDetail != nil {
			return model.ResourceRef{Kind: "ec2-subnet", ID: a.ec2SubnetDetail.SubnetID, Name: a.ec2SubnetDetail.Name}, true
		}
	case viewEC2VolumeDetail:
		if a.ec2VolumeDetail != nil {
			return model.ResourceRef{Kind: "ec2-volume", ID: a.ec2VolumeDetail.VolumeID, Name: a.ec2VolumeDetail.Name}, true
		}
	case viewEC2LoadBalancerDetail:
		if a.ec2LoadBalancerDetail != nil {
			return model.ResourceRef{Kind: "ec2-load-balancer", ID: a.ec2LoadBalancerDetail.ARN, Name: a.ec2LoadBalancerDetail.Name}, true
		}
	case viewEC2TargetGroupDetail:
		if a.ec2TargetGroupDetail != nil {
			return model.ResourceRef{Kind: "ec2-target-group", ID: a.ec2TargetGroupDetail.ARN, Name: a.ec2TargetGroupDetail.Name}, true
		}
	}
	return model.ResourceRef{}, false
}

func (a App) navigateEC2Resource(ref model.ResourceRef, pushOrigin bool) (App, tea.Cmd) {
	if pushOrigin {
		if origin, ok := a.currentEC2ResourceRef(); ok && (origin.Kind != ref.Kind || origin.ID != ref.ID) {
			a.resourceHistory = append(a.resourceHistory, origin)
		}
	}
	if ref.Kind != "ecs-task" && ref.Kind != "rds-instance" {
		a.mode = modeEC2
	}
	a.loading = true
	ctx := a.ctx
	switch ref.Kind {
	case "ec2-instance":
		a.state = viewEC2Detail
		ec2Service := a.ec2
		return a, func() tea.Msg {
			detail, err := ec2Service.Detail(ctx, ref.ID)
			if err != nil {
				return errMsg{err}
			}
			return ec2DetailLoadedMsg{detail}
		}
	case "ec2-security-group":
		a.state = viewEC2SecurityGroupDetail
		ec2Network := a.ec2Network
		return a, func() tea.Msg {
			group, err := ec2Network.SecurityGroup(ctx, ref.ID)
			if err != nil {
				return errMsg{err}
			}
			return ec2SecurityGroupLoadedMsg{group}
		}
	case "ec2-vpc":
		return a.loadEC2VPCDetail(ref.ID)
	case "ec2-subnet":
		return a.loadEC2SubnetDetail(ref.ID)
	case "ec2-subnets":
		originHistory := a.resourceHistory
		a, cmd := a.openEC2Subnets(ref.ID)
		a.resourceHistory = originHistory
		return a, cmd
	case "ec2-volume":
		return a.loadEC2VolumeDetail(ref.ID)
	case "ec2-load-balancer":
		return a.loadEC2LoadBalancerDetail(ref.ID)
	case "ec2-target-group":
		return a.loadEC2TargetGroupDetail(ref.ID)
	case "ecs-task":
		if a.selectedTask == nil || a.selectedTask.TaskARN != ref.ID {
			a.loading = false
			a.err = fmt.Errorf("ECS task %s is no longer available", ref.ID)
			return a, nil
		}
		a.mode = modeECS
		a.state = viewTaskDetail
		a.detailView = a.newTaskDetail(a.selectedTask).SetSize(a.width-3, a.height-6)
		a.loading = false
		return a, nil
	case "rds-instance":
		if detail := a.rdsDetailView.Detail(); detail == nil || detail.Identifier != ref.ID {
			a.loading = false
			a.err = fmt.Errorf("RDS instance %s is no longer available", ref.ID)
			return a, nil
		}
		a.mode = modeRDS
		a.state = viewRDSDetail
		a.loading = false
		return a, nil
	default:
		a.loading = false
		a.err = fmt.Errorf("navigation is not implemented for %s", ref.Kind)
		return a, nil
	}
}

func (a App) openEC2Detail() (App, tea.Cmd) {
	inst := a.ec2InstancesView.SelectedInstance()
	if inst == nil {
		return a, nil
	}
	a.state = viewEC2Detail
	a.loading = true
	ec2Service := a.ec2
	ctx := a.ctx
	instanceID := inst.InstanceID
	return a, func() tea.Msg {
		detail, err := ec2Service.Detail(ctx, instanceID)
		if err != nil {
			return errMsg{err}
		}
		return ec2DetailLoadedMsg{detail}
	}
}

func (a App) openEC2Console() (App, tea.Cmd) {
	instanceID := a.ec2DetailView.InstanceID()
	if instanceID == "" {
		return a, nil
	}
	a.state = viewEC2Console
	a.ec2ConsoleView = views.NewEC2Console(instanceID)
	a.ec2ConsoleView = a.ec2ConsoleView.SetSize(a.width-3, a.height-6)
	a.loading = true
	ec2Service := a.ec2
	ctx := a.ctx
	return a, func() tea.Msg {
		output, err := ec2Service.ConsoleOutput(ctx, instanceID)
		if err != nil {
			return errMsg{err}
		}
		return ec2ConsoleLoadedMsg{output}
	}
}

func (a App) startEC2SSMSession() (App, tea.Cmd) {
	instanceID := a.ec2DetailView.InstanceID()
	if instanceID == "" {
		return a, nil
	}
	state := a.ec2DetailView.InstanceState()
	if state != "running" {
		a.err = fmt.Errorf("instance must be running for SSM session (current state: %s)", state)
		return a, nil
	}
	ec2Service := a.ec2
	ctx := a.ctx
	return a, func() tea.Msg {
		launch, err := ec2Service.PrepareSession(ctx, instanceID, state)
		if err != nil {
			return errMsg{err}
		}
		return execSessionReadyMsg{pluginPath: launch.Executable, args: launch.Args}
	}
}

func (a App) startEC2Instance() (App, tea.Cmd) {
	instanceID := a.ec2DetailView.InstanceID()
	a.confirm = NewConfirm(ConfirmEC2Start, fmt.Sprintf("Start instance %s?", instanceID))
	return a, nil
}

func (a App) stopEC2Instance() (App, tea.Cmd) {
	instanceID := a.ec2DetailView.InstanceID()
	a.confirm = NewConfirm(ConfirmEC2Stop, fmt.Sprintf("Stop instance %s?", instanceID))
	return a, nil
}

func (a App) rebootEC2Instance() (App, tea.Cmd) {
	instanceID := a.ec2DetailView.InstanceID()
	a.confirm = NewConfirm(ConfirmEC2Reboot, fmt.Sprintf("Reboot instance %s?", instanceID))
	return a, nil
}

func (a App) terminateEC2Instance() (App, tea.Cmd) {
	instanceID := a.ec2DetailView.InstanceID()
	a.confirm = NewConfirm(ConfirmEC2Terminate,
		fmt.Sprintf("TERMINATE instance %s? This cannot be undone!", instanceID))
	return a, nil
}

func (a App) doStartEC2() tea.Cmd {
	ec2Service := a.ec2
	ctx := a.ctx
	instanceID := a.ec2DetailView.InstanceID()
	state := a.ec2DetailView.InstanceState()
	return func() tea.Msg {
		err := ec2Service.Start(ctx, instanceID, state)
		if err != nil {
			return errMsg{err}
		}
		return ec2ActionDoneMsg{fmt.Sprintf("Starting instance %s", instanceID)}
	}
}

func (a App) doStopEC2() tea.Cmd {
	ec2Service := a.ec2
	ctx := a.ctx
	instanceID := a.ec2DetailView.InstanceID()
	state := a.ec2DetailView.InstanceState()
	return func() tea.Msg {
		err := ec2Service.Stop(ctx, instanceID, state)
		if err != nil {
			return errMsg{err}
		}
		return ec2ActionDoneMsg{fmt.Sprintf("Stopping instance %s", instanceID)}
	}
}

func (a App) doRebootEC2() tea.Cmd {
	ec2Service := a.ec2
	ctx := a.ctx
	instanceID := a.ec2DetailView.InstanceID()
	state := a.ec2DetailView.InstanceState()
	return func() tea.Msg {
		err := ec2Service.Reboot(ctx, instanceID, state)
		if err != nil {
			return errMsg{err}
		}
		return ec2ActionDoneMsg{fmt.Sprintf("Rebooting instance %s", instanceID)}
	}
}

func (a App) doTerminateEC2() tea.Cmd {
	ec2Service := a.ec2
	ctx := a.ctx
	instanceID := a.ec2DetailView.InstanceID()
	state := a.ec2DetailView.InstanceState()
	return func() tea.Msg {
		err := ec2Service.Terminate(ctx, instanceID, state)
		if err != nil {
			return errMsg{err}
		}
		return ec2ActionDoneMsg{fmt.Sprintf("Terminating instance %s", instanceID)}
	}
}

func (a App) refreshEC2Detail() tea.Cmd {
	instanceID := a.ec2DetailView.InstanceID()
	if instanceID == "" {
		return nil
	}
	ec2Service := a.ec2
	ctx := a.ctx
	return func() tea.Msg {
		detail, err := ec2Service.Detail(ctx, instanceID)
		if err != nil {
			return errMsg{err}
		}
		return ec2DetailLoadedMsg{detail}
	}
}

func (a App) refreshEC2Instances() tea.Cmd {
	ec2Service := a.ec2
	ctx := a.ctx
	return func() tea.Msg {
		instances, err := ec2Service.List(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return ec2InstancesLoadedMsg{instances}
	}
}

func (a App) handleEC2Action(msg ec2ActionDoneMsg) (App, tea.Cmd) {
	a.flashMessage = msg.message
	a.flashExpiry = time.Now().Add(5 * time.Second)
	a.loading = false
	if a.state == viewEC2Detail {
		return a, a.refreshEC2Detail()
	}
	return a, nil
}
