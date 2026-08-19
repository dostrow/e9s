package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/views"
)

func (a App) openEC2LoadBalancers() (App, tea.Cmd) {
	a.state, a.mode, a.loading = viewEC2LoadBalancers, modeEC2, true
	a.resourceHistory = nil
	a.ec2ResourceListView = views.NewEC2ResourceList("EC2 Load Balancers", []string{"NAME", "TYPE", "STATE", "SCHEME", "VPC", "DNS NAME"}).SetSize(a.width-3, a.height-6)
	loadBalancing, ctx := a.loadBalancing, a.ctx
	return a, func() tea.Msg {
		loadBalancers, err := loadBalancing.List(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return ec2LoadBalancersLoadedMsg{loadBalancers}
	}
}

func (a App) loadEC2LoadBalancerDetail(arn string) (App, tea.Cmd) {
	a.state, a.mode, a.loading = viewEC2LoadBalancerDetail, modeEC2, true
	loadBalancing, ctx := a.loadBalancing, a.ctx
	return a, func() tea.Msg {
		loadBalancer, err := loadBalancing.Detail(ctx, arn)
		if err != nil {
			return errMsg{err}
		}
		return ec2LoadBalancerLoadedMsg{loadBalancer}
	}
}

func (a App) openEC2TargetGroups() (App, tea.Cmd) {
	a.state, a.mode, a.loading = viewEC2TargetGroups, modeEC2, true
	a.resourceHistory = nil
	a.ec2ResourceListView = views.NewEC2ResourceList("EC2 Target Groups", []string{"NAME", "PROTOCOL", "PORT", "TARGET TYPE", "VPC", "LOAD BALANCERS"}).SetSize(a.width-3, a.height-6)
	loadBalancing, ctx := a.loadBalancing, a.ctx
	return a, func() tea.Msg {
		targetGroups, err := loadBalancing.ListTargetGroups(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return ec2TargetGroupsLoadedMsg{targetGroups}
	}
}

func (a App) loadEC2TargetGroupDetail(arn string) (App, tea.Cmd) {
	a.state, a.mode, a.loading = viewEC2TargetGroupDetail, modeEC2, true
	loadBalancing, ctx := a.loadBalancing, a.ctx
	return a, func() tea.Msg {
		targetGroup, err := loadBalancing.TargetGroup(ctx, arn)
		if err != nil {
			return errMsg{err}
		}
		return ec2TargetGroupLoadedMsg{targetGroup}
	}
}

func (a App) refreshEC2LoadBalancers() tea.Cmd {
	loadBalancing, ctx := a.loadBalancing, a.ctx
	return func() tea.Msg {
		loadBalancers, err := loadBalancing.List(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return ec2LoadBalancersLoadedMsg{loadBalancers}
	}
}

func (a App) refreshEC2LoadBalancerDetail() tea.Cmd {
	if a.ec2LoadBalancerDetail == nil {
		return nil
	}
	arn := a.ec2LoadBalancerDetail.ARN
	loadBalancing, ctx := a.loadBalancing, a.ctx
	return func() tea.Msg {
		loadBalancer, err := loadBalancing.Detail(ctx, arn)
		if err != nil {
			return errMsg{err}
		}
		return ec2LoadBalancerLoadedMsg{loadBalancer}
	}
}

func (a App) refreshEC2TargetGroups() tea.Cmd {
	loadBalancing, ctx := a.loadBalancing, a.ctx
	return func() tea.Msg {
		targetGroups, err := loadBalancing.ListTargetGroups(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return ec2TargetGroupsLoadedMsg{targetGroups}
	}
}

func (a App) refreshEC2TargetGroupDetail() tea.Cmd {
	if a.ec2TargetGroupDetail == nil {
		return nil
	}
	arn := a.ec2TargetGroupDetail.ARN
	loadBalancing, ctx := a.loadBalancing, a.ctx
	return func() tea.Msg {
		targetGroup, err := loadBalancing.TargetGroup(ctx, arn)
		if err != nil {
			return errMsg{err}
		}
		return ec2TargetGroupLoadedMsg{targetGroup}
	}
}

func ec2LoadBalancerRows(loadBalancers []model.EC2LoadBalancer) []views.EC2ResourceRow {
	rows := make([]views.EC2ResourceRow, len(loadBalancers))
	for i, loadBalancer := range loadBalancers {
		search := strings.Join(append(append([]string{}, loadBalancer.SecurityGroups...), loadBalancer.ARN, loadBalancer.IPAddressType), " ")
		rows[i] = views.EC2ResourceRow{ID: loadBalancer.ARN, Search: search, Cells: []string{
			loadBalancer.Name, loadBalancer.Type, loadBalancer.State, loadBalancer.Scheme, loadBalancer.VpcID, loadBalancer.DNSName,
		}}
	}
	return rows
}

func formatTUIEC2LoadBalancer(loadBalancer model.EC2LoadBalancer) string {
	var out strings.Builder
	fmt.Fprintf(&out, "  Load Balancer: %s\n\n  ARN:          %s\n  Type:         %s\n  State:        %s\n  State reason: %s\n  Scheme:       %s\n  DNS name:     %s\n  Hosted zone:  %s\n  VPC:          %s\n  IP address:   %s\n\n  Availability Zones",
		loadBalancer.Name, loadBalancer.ARN, loadBalancer.Type, loadBalancer.State, loadBalancer.StateReason,
		loadBalancer.Scheme, loadBalancer.DNSName, loadBalancer.HostedZoneID, loadBalancer.VpcID, loadBalancer.IPAddressType)
	for _, zone := range loadBalancer.AvailabilityZones {
		fmt.Fprintf(&out, "\n\n  %-18s %-24s IPv4 %-16s IPv6 %s", zone.AZ, zone.SubnetID, zone.IPv4, zone.IPv6)
	}
	if len(loadBalancer.SecurityGroups) > 0 {
		out.WriteString("\n\n  Security Groups\n\n  " + strings.Join(loadBalancer.SecurityGroups, "\n  "))
	}
	out.WriteString("\n\n  Listeners")
	if len(loadBalancer.Listeners) == 0 {
		out.WriteString("\n\n  None")
	}
	for _, listener := range loadBalancer.Listeners {
		fmt.Fprintf(&out, "\n\n  %s:%d  %s", listener.Protocol, listener.Port, listener.SSLPolicy)
		for _, action := range listener.Actions {
			out.WriteString("\n    " + action)
		}
	}
	out.WriteString("\n\n  Target Groups")
	if len(loadBalancer.TargetGroups) == 0 {
		out.WriteString("\n\n  None")
	}
	for _, targetGroup := range loadBalancer.TargetGroups {
		fmt.Fprintf(&out, "\n\n  %s  %s:%d  target type: %s\n    health check: %s:%s%s",
			targetGroup.Name, targetGroup.Protocol, targetGroup.Port, targetGroup.TargetType,
			targetGroup.HealthCheckProtocol, targetGroup.HealthCheckPort, targetGroup.HealthCheckPath)
		for _, target := range targetGroup.Targets {
			fmt.Fprintf(&out, "\n    %-22s %-6d %-18s %s", target.ID, target.Port, target.AZ, target.State)
		}
	}
	out.WriteString("\n\n  [o] browse linked resources")
	return out.String()
}

func ec2TargetGroupRows(targetGroups []model.EC2TargetGroup) []views.EC2ResourceRow {
	rows := make([]views.EC2ResourceRow, len(targetGroups))
	for i, targetGroup := range targetGroups {
		loadBalancers := make([]string, 0, len(targetGroup.LoadBalancerARNs))
		for _, arn := range targetGroup.LoadBalancerARNs {
			loadBalancers = append(loadBalancers, tuiResourceNameFromARN(arn))
		}
		rows[i] = views.EC2ResourceRow{ID: targetGroup.ARN, Search: strings.Join(targetGroup.LoadBalancerARNs, " "), Cells: []string{
			targetGroup.Name, targetGroup.Protocol, fmt.Sprint(targetGroup.Port), targetGroup.TargetType, targetGroup.VpcID, strings.Join(loadBalancers, ", "),
		}}
	}
	return rows
}

func formatTUIEC2TargetGroup(targetGroup model.EC2TargetGroup) string {
	var out strings.Builder
	fmt.Fprintf(&out, "  Target Group: %s\n\n  ARN:              %s\n  Protocol:         %s\n  Protocol version: %s\n  Port:             %d\n  Target type:      %s\n  VPC:              %s\n  Health check:     %s:%s%s\n\n  Load Balancers",
		targetGroup.Name, targetGroup.ARN, targetGroup.Protocol, targetGroup.ProtocolVersion, targetGroup.Port,
		targetGroup.TargetType, targetGroup.VpcID, targetGroup.HealthCheckProtocol, targetGroup.HealthCheckPort, targetGroup.HealthCheckPath)
	if len(targetGroup.LoadBalancerARNs) == 0 {
		out.WriteString("\n\n  None")
	}
	for _, arn := range targetGroup.LoadBalancerARNs {
		fmt.Fprintf(&out, "\n\n  %s\n    %s", tuiResourceNameFromARN(arn), arn)
	}
	out.WriteString("\n\n  Registered Targets")
	if len(targetGroup.Targets) == 0 {
		out.WriteString("\n\n  None")
	}
	for _, target := range targetGroup.Targets {
		fmt.Fprintf(&out, "\n\n  %-22s %-6d %-18s %s", target.ID, target.Port, target.AZ, target.State)
		if target.Reason != "" {
			fmt.Fprintf(&out, " (%s)", target.Reason)
		}
		if target.Description != "" {
			out.WriteString("\n    " + target.Description)
		}
	}
	out.WriteString("\n\n  [o] browse linked resources")
	return out.String()
}

func tuiResourceNameFromARN(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2]
	}
	return arn
}
