//go:build gui

package gui

import (
	"fmt"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

func (w *mainWindow) openEC2LoadBalancersModule() {
	w.resourceHistory = nil
	w.loadEC2LoadBalancers("")
}

func (w *mainWindow) loadEC2LoadBalancers(arn string) {
	w.resetWorkspaceForBrowserChange()
	w.allEC2LoadBalancers = nil
	w.filteredEC2LoadBalancers = nil
	w.selectedEC2LoadBalancer = ""
	w.ec2LoadBalancerDetail = nil
	w.ec2LoadBalancerTable.clear()
	w.currentPage = pageEC2LoadBalancers
	w.setBreadcrumb("EC2 / Load Balancers")
	w.backButton.SetSensitive(len(w.resourceHistory) > 0)
	w.search.SetPlaceholderText("Filter load balancers…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageEC2LoadBalancers)
	w.setDetail("Loading application and network load balancers…", detailIntro)
	ctx, generation := w.startRequest("Loading load balancers…")
	go func() {
		loadBalancers, err := w.options.LoadBalancing.List(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			w.allEC2LoadBalancers = loadBalancers
			w.applyEC2LoadBalancerFilter()
			w.setDetail(fmt.Sprintf("Loaded %d application and network load balancers.", len(loadBalancers)), detailIntro)
			if arn != "" {
				for i, loadBalancer := range w.filteredEC2LoadBalancers {
					if loadBalancer.ARN == arn {
						w.ec2LoadBalancerTable.selection.SetSelected(uint(i))
						return
					}
				}
			}
		})
	}()
}

func (w *mainWindow) applyEC2LoadBalancerFilter() {
	w.filteredEC2LoadBalancers = service.FilterEC2LoadBalancers(w.allEC2LoadBalancers, w.search.Text())
	rows := make([]string, len(w.filteredEC2LoadBalancers))
	for i, loadBalancer := range w.filteredEC2LoadBalancers {
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s", loadBalancer.Name, loadBalancer.Type,
			loadBalancer.State, loadBalancer.Scheme, loadBalancer.VpcID, loadBalancer.DNSName)
	}
	w.ec2LoadBalancerTable.replace(rows)
}

func (w *mainWindow) selectEC2LoadBalancerRow() {
	if w.currentPage != pageEC2LoadBalancers {
		return
	}
	p := w.ec2LoadBalancerTable.selection.Selected()
	if p == gtk.InvalidListPosition || int(p) >= len(w.filteredEC2LoadBalancers) {
		return
	}
	loadBalancer := w.filteredEC2LoadBalancers[p]
	w.resetWorkspaceForBrowserChange()
	w.selectedEC2LoadBalancer = loadBalancer.ARN
	w.setBreadcrumb("EC2 / Load Balancers / " + loadBalancer.Name)
	w.setDetail("Loading load balancer details…", detailEC2LoadBalancer)
	ctx, generation := w.startRequest("Loading load balancer " + loadBalancer.Name + "…")
	go func() {
		detail, err := w.options.LoadBalancing.Detail(ctx, loadBalancer.ARN)
		w.finishRequest(ctx, generation, err, func() {
			w.ec2LoadBalancerDetail = detail
			w.renderEC2LoadBalancer(*detail)
		})
	}()
}

func (w *mainWindow) openEC2LoadBalancerAt(position uint) {
	w.ec2LoadBalancerTable.selection.SetSelected(position)
}

func (w *mainWindow) renderEC2LoadBalancer(loadBalancer model.EC2LoadBalancer) {
	var out strings.Builder
	fmt.Fprintf(&out, "LOAD BALANCER\n\nName           %s\nARN            %s\nType           %s\nState          %s\nState reason   %s\nScheme         %s\nDNS name       %s\nHosted zone    %s\nVPC            %s\nIP address     %s\nCreated        %s",
		loadBalancer.Name, loadBalancer.ARN, loadBalancer.Type, loadBalancer.State,
		valueOrDash(loadBalancer.StateReason), loadBalancer.Scheme, loadBalancer.DNSName,
		loadBalancer.HostedZoneID, loadBalancer.VpcID, loadBalancer.IPAddressType, formatTime(loadBalancer.CreatedAt))
	links := []workspaceResourceLink{}
	if loadBalancer.VpcID != "" {
		links = append(links, workspaceResourceLink{label: "VPC: " + loadBalancer.VpcID, ref: model.ResourceRef{Kind: "ec2-vpc", ID: loadBalancer.VpcID}})
	}
	out.WriteString("\n\nAVAILABILITY ZONES\n")
	for _, zone := range loadBalancer.AvailabilityZones {
		fmt.Fprintf(&out, "\n  %-18s %-24s IPv4 %-16s IPv6 %s", zone.AZ, zone.SubnetID, valueOrDash(zone.IPv4), valueOrDash(zone.IPv6))
		if zone.SubnetID != "" {
			links = append(links, workspaceResourceLink{label: "Subnet: " + zone.SubnetID, ref: model.ResourceRef{Kind: "ec2-subnet", ID: zone.SubnetID}})
		}
	}
	if len(loadBalancer.SecurityGroups) > 0 {
		out.WriteString("\n\nSECURITY GROUPS\n")
		for _, groupID := range loadBalancer.SecurityGroups {
			out.WriteString("\n  " + groupID)
			links = append(links, workspaceResourceLink{label: "Security group: " + groupID, ref: model.ResourceRef{Kind: "ec2-security-group", ID: groupID}})
		}
	}
	out.WriteString("\n\nLISTENERS\n")
	if len(loadBalancer.Listeners) == 0 {
		out.WriteString("\n  None")
	}
	for _, listener := range loadBalancer.Listeners {
		fmt.Fprintf(&out, "\n  %s:%d", listener.Protocol, listener.Port)
		if listener.SSLPolicy != "" {
			out.WriteString("  " + listener.SSLPolicy)
		}
		for _, action := range listener.Actions {
			out.WriteString("\n    " + action)
		}
	}
	out.WriteString("\n\nTARGET GROUPS\n")
	if len(loadBalancer.TargetGroups) == 0 {
		out.WriteString("\n  None")
	}
	for _, targetGroup := range loadBalancer.TargetGroups {
		fmt.Fprintf(&out, "\n\n  %s  %s:%d  target type: %s", targetGroup.Name, targetGroup.Protocol, targetGroup.Port, targetGroup.TargetType)
		links = append(links, workspaceResourceLink{label: "Target group: " + targetGroup.Name, ref: model.ResourceRef{Kind: "ec2-target-group", ID: targetGroup.ARN, Name: targetGroup.Name}})
		fmt.Fprintf(&out, "\n    health check: %s:%s%s", targetGroup.HealthCheckProtocol, targetGroup.HealthCheckPort, targetGroup.HealthCheckPath)
		if targetGroup.VpcID != "" && targetGroup.VpcID != loadBalancer.VpcID {
			fmt.Fprintf(&out, "\n    VPC: %s", targetGroup.VpcID)
			links = append(links, workspaceResourceLink{label: "VPC: " + targetGroup.VpcID, ref: model.ResourceRef{Kind: "ec2-vpc", ID: targetGroup.VpcID}})
		}
		if len(targetGroup.Targets) == 0 {
			out.WriteString("\n    no registered targets")
		}
		for _, target := range targetGroup.Targets {
			fmt.Fprintf(&out, "\n    %-22s %-6d %-18s %s", target.ID, target.Port, target.AZ, target.State)
			if target.Reason != "" {
				fmt.Fprintf(&out, " (%s)", target.Reason)
			}
			if targetGroup.TargetType == "instance" && strings.HasPrefix(target.ID, "i-") {
				links = append(links, workspaceResourceLink{label: "Instance: " + target.ID, ref: model.ResourceRef{Kind: "ec2-instance", ID: target.ID}})
			}
		}
	}
	w.setDetail(out.String(), detailEC2LoadBalancer)
	w.setDetailResourceLinks(uniqueWorkspaceResourceLinks(links))
}

func (w *mainWindow) openEC2TargetGroupsModule() {
	w.resourceHistory = nil
	w.loadEC2TargetGroups("")
}

func (w *mainWindow) loadEC2TargetGroups(arn string) {
	w.resetWorkspaceForBrowserChange()
	w.allEC2TargetGroups = nil
	w.filteredEC2TargetGroups = nil
	w.selectedEC2TargetGroup = ""
	w.ec2TargetGroupDetail = nil
	w.ec2TargetGroupTable.clear()
	w.currentPage = pageEC2TargetGroups
	w.setBreadcrumb("EC2 / Target Groups")
	w.backButton.SetSensitive(len(w.resourceHistory) > 0)
	w.search.SetPlaceholderText("Filter target groups…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageEC2TargetGroups)
	w.setDetail("Loading target groups…", detailIntro)
	ctx, generation := w.startRequest("Loading target groups…")
	go func() {
		targetGroups, err := w.options.LoadBalancing.ListTargetGroups(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			w.allEC2TargetGroups = targetGroups
			w.applyEC2TargetGroupFilter()
			w.setDetail(fmt.Sprintf("Loaded %d target groups.", len(targetGroups)), detailIntro)
			if arn != "" {
				for i, targetGroup := range w.filteredEC2TargetGroups {
					if targetGroup.ARN == arn {
						w.ec2TargetGroupTable.selection.SetSelected(uint(i))
						return
					}
				}
			}
		})
	}()
}

func (w *mainWindow) applyEC2TargetGroupFilter() {
	w.filteredEC2TargetGroups = service.FilterEC2TargetGroups(w.allEC2TargetGroups, w.search.Text())
	rows := make([]string, len(w.filteredEC2TargetGroups))
	for i, targetGroup := range w.filteredEC2TargetGroups {
		loadBalancers := make([]string, 0, len(targetGroup.LoadBalancerARNs))
		for _, arn := range targetGroup.LoadBalancerARNs {
			loadBalancers = append(loadBalancers, resourceNameFromARN(arn))
		}
		rows[i] = fmt.Sprintf("%s\t%s\t%d\t%s\t%s\t%s", targetGroup.Name, targetGroup.Protocol,
			targetGroup.Port, targetGroup.TargetType, targetGroup.VpcID, strings.Join(loadBalancers, ", "))
	}
	w.ec2TargetGroupTable.replace(rows)
}

func (w *mainWindow) selectEC2TargetGroupRow() {
	if w.currentPage != pageEC2TargetGroups {
		return
	}
	p := w.ec2TargetGroupTable.selection.Selected()
	if p == gtk.InvalidListPosition || int(p) >= len(w.filteredEC2TargetGroups) {
		return
	}
	targetGroup := w.filteredEC2TargetGroups[p]
	w.resetWorkspaceForBrowserChange()
	w.selectedEC2TargetGroup = targetGroup.ARN
	w.setBreadcrumb("EC2 / Target Groups / " + targetGroup.Name)
	w.setDetail("Loading target group details…", detailEC2TargetGroup)
	ctx, generation := w.startRequest("Loading target group " + targetGroup.Name + "…")
	go func() {
		detail, err := w.options.LoadBalancing.TargetGroup(ctx, targetGroup.ARN)
		w.finishRequest(ctx, generation, err, func() {
			w.ec2TargetGroupDetail = detail
			w.renderEC2TargetGroup(*detail)
		})
	}()
}

func (w *mainWindow) openEC2TargetGroupAt(position uint) {
	w.ec2TargetGroupTable.selection.SetSelected(position)
}

func (w *mainWindow) renderEC2TargetGroup(targetGroup model.EC2TargetGroup) {
	var out strings.Builder
	fmt.Fprintf(&out, "TARGET GROUP\n\nName             %s\nARN              %s\nProtocol         %s\nProtocol version %s\nPort             %d\nTarget type      %s\nVPC              %s\nHealth check     %s:%s%s",
		targetGroup.Name, targetGroup.ARN, targetGroup.Protocol, valueOrDash(targetGroup.ProtocolVersion),
		targetGroup.Port, targetGroup.TargetType, targetGroup.VpcID, targetGroup.HealthCheckProtocol,
		targetGroup.HealthCheckPort, targetGroup.HealthCheckPath)
	links := []workspaceResourceLink{}
	if targetGroup.VpcID != "" {
		links = append(links, workspaceResourceLink{label: "VPC: " + targetGroup.VpcID, ref: model.ResourceRef{Kind: "ec2-vpc", ID: targetGroup.VpcID}})
	}
	out.WriteString("\n\nLOAD BALANCERS\n")
	if len(targetGroup.LoadBalancerARNs) == 0 {
		out.WriteString("\n  None")
	}
	for _, arn := range targetGroup.LoadBalancerARNs {
		name := resourceNameFromARN(arn)
		out.WriteString("\n  " + name + "\n    " + arn)
		links = append(links, workspaceResourceLink{label: "Load balancer: " + name, ref: model.ResourceRef{Kind: "ec2-load-balancer", ID: arn, Name: name}})
	}
	out.WriteString("\n\nREGISTERED TARGETS\n")
	if len(targetGroup.Targets) == 0 {
		out.WriteString("\n  None")
	}
	for _, target := range targetGroup.Targets {
		fmt.Fprintf(&out, "\n  %-22s %-6d %-18s %s", target.ID, target.Port, target.AZ, target.State)
		if target.Reason != "" {
			fmt.Fprintf(&out, " (%s)", target.Reason)
		}
		if target.Description != "" {
			out.WriteString("\n    " + target.Description)
		}
		if targetGroup.TargetType == "instance" && strings.HasPrefix(target.ID, "i-") {
			links = append(links, workspaceResourceLink{label: "Instance: " + target.ID, ref: model.ResourceRef{Kind: "ec2-instance", ID: target.ID}})
		}
	}
	w.setDetail(out.String(), detailEC2TargetGroup)
	w.setDetailResourceLinks(uniqueWorkspaceResourceLinks(links))
}

func (w *mainWindow) refreshEC2TargetGroups(foreground bool) {
	selected := w.selectedEC2TargetGroup
	ctx, generation := w.startRefreshRequest("Refreshing target groups…", foreground)
	go func() {
		targetGroups, err := w.options.LoadBalancing.ListTargetGroups(ctx, "")
		var detail *model.EC2TargetGroup
		if err == nil && selected != "" {
			detail, err = w.options.LoadBalancing.TargetGroup(ctx, selected)
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allEC2TargetGroups = targetGroups
			w.applyEC2TargetGroupFilter()
			if detail != nil {
				w.ec2TargetGroupDetail = detail
				w.renderEC2TargetGroup(*detail)
			}
		})
	}()
}

func resourceNameFromARN(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return arn
}

func (w *mainWindow) refreshEC2LoadBalancers(foreground bool) {
	selected := w.selectedEC2LoadBalancer
	ctx, generation := w.startRefreshRequest("Refreshing load balancers…", foreground)
	go func() {
		loadBalancers, err := w.options.LoadBalancing.List(ctx, "")
		var detail *model.EC2LoadBalancer
		if err == nil && selected != "" {
			detail, err = w.options.LoadBalancing.Detail(ctx, selected)
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allEC2LoadBalancers = loadBalancers
			w.applyEC2LoadBalancerFilter()
			if detail != nil {
				w.ec2LoadBalancerDetail = detail
				w.renderEC2LoadBalancer(*detail)
			}
		})
	}()
}

func uniqueWorkspaceResourceLinks(links []workspaceResourceLink) []workspaceResourceLink {
	seen := make(map[string]bool, len(links))
	unique := make([]workspaceResourceLink, 0, len(links))
	for _, link := range links {
		key := link.ref.Kind + "\x00" + link.ref.ID
		if link.ref.ID == "" || seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, link)
	}
	return unique
}
