package ui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/views"
)

func (a App) openEC2VPCs() (App, tea.Cmd) {
	a.state, a.mode, a.loading = viewEC2VPCs, modeEC2, true
	a.resourceHistory = nil
	a.ec2ResourceListView = views.NewEC2ResourceList("EC2 VPCs", []string{"NAME", "VPC ID", "STATE", "CIDRS", "DEFAULT", "TENANCY"}).SetSize(a.width-3, a.height-6)
	service, ctx := a.ec2Network, a.ctx
	return a, func() tea.Msg {
		vpcs, err := service.VPCs(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return ec2VPCsLoadedMsg{vpcs}
	}
}

func (a App) openEC2Subnets(vpcID string) (App, tea.Cmd) {
	a.state, a.mode, a.loading = viewEC2Subnets, modeEC2, true
	a.resourceHistory = nil
	a.ec2SubnetVPCFilter = vpcID
	a.ec2ResourceListView = views.NewEC2ResourceList("EC2 Subnets", []string{"NAME", "SUBNET ID", "VPC", "AZ", "CIDR", "AVAILABLE", "PUBLIC IP"}).SetSize(a.width-3, a.height-6)
	service, ctx := a.ec2Network, a.ctx
	return a, func() tea.Msg {
		subnets, err := service.Subnets(ctx, "", vpcID)
		if err != nil {
			return errMsg{err}
		}
		return ec2SubnetsLoadedMsg{subnets}
	}
}

func (a App) openEC2Volumes() (App, tea.Cmd) {
	a.state, a.mode, a.loading = viewEC2Volumes, modeEC2, true
	a.resourceHistory = nil
	a.ec2ResourceListView = views.NewEC2ResourceList("EBS Volumes", []string{"NAME", "VOLUME ID", "STATE", "TYPE", "SIZE", "AZ", "ATTACHED TO"}).SetSize(a.width-3, a.height-6)
	service, ctx := a.ebs, a.ctx
	return a, func() tea.Msg {
		volumes, err := service.List(ctx, "")
		if err != nil {
			return errMsg{err}
		}
		return ec2VolumesLoadedMsg{volumes}
	}
}

func (a App) loadEC2VolumeDetail(volumeID string) (App, tea.Cmd) {
	a.state, a.loading = viewEC2VolumeDetail, true
	service, ctx := a.ebs, a.ctx
	return a, func() tea.Msg {
		volume, err := service.Detail(ctx, volumeID)
		if err != nil {
			return errMsg{err}
		}
		return ec2VolumeLoadedMsg{volume}
	}
}

func ec2VolumeRows(volumes []model.EC2Volume) []views.EC2ResourceRow {
	rows := make([]views.EC2ResourceRow, len(volumes))
	for i, v := range volumes {
		attached := []string{}
		for _, a := range v.Attachments {
			attached = append(attached, a.InstanceID+" "+a.DeviceName)
		}
		rows[i] = views.EC2ResourceRow{ID: v.VolumeID, Search: tagSearch(v.Tags), Cells: []string{v.Name, v.VolumeID, v.State, v.VolumeType, fmt.Sprintf("%d GiB", v.Size), v.AZ, strings.Join(attached, ",")}}
	}
	return rows
}

func formatTUIEC2Volume(v model.EC2Volume) string {
	var out strings.Builder
	fmt.Fprintf(&out, "  EBS Volume: %s\n\n  Volume ID:    %s\n  State:        %s\n  Type:         %s\n  Size:         %d GiB\n  AZ:           %s\n  IOPS:         %d\n  Throughput:   %d MiB/s\n  Encrypted:    %t\n  KMS key:      %s\n  Snapshot:     %s\n  Multi-attach: %t\n\n  Attachments", firstNonBlank(v.Name, v.VolumeID), v.VolumeID, v.State, v.VolumeType, v.Size, v.AZ, v.IOPS, v.Throughput, v.Encrypted, v.KMSKeyID, v.SnapshotID, v.MultiAttach)
	if len(v.Attachments) == 0 {
		out.WriteString("\n\n  None (unattached)")
	} else {
		for _, a := range v.Attachments {
			fmt.Fprintf(&out, "\n\n  %-20s %-14s %s", a.InstanceID, a.DeviceName, a.State)
		}
		out.WriteString("\n\n  [o] open attached instance")
	}
	appendTUITags(&out, v.Tags)
	return out.String()
}

func (a App) loadEC2VPCDetail(vpcID string) (App, tea.Cmd) {
	a.state, a.loading = viewEC2VPCDetail, true
	service, ctx := a.ec2Network, a.ctx
	return a, func() tea.Msg {
		vpc, err := service.VPC(ctx, vpcID)
		if err != nil {
			return errMsg{err}
		}
		return ec2VPCLoadedMsg{vpc}
	}
}

func (a App) loadEC2SubnetDetail(subnetID string) (App, tea.Cmd) {
	a.state, a.loading = viewEC2SubnetDetail, true
	service, ctx := a.ec2Network, a.ctx
	return a, func() tea.Msg {
		subnet, err := service.Subnet(ctx, subnetID)
		if err != nil {
			return errMsg{err}
		}
		return ec2SubnetLoadedMsg{subnet}
	}
}

func ec2VPCRows(vpcs []model.EC2VPC) []views.EC2ResourceRow {
	rows := make([]views.EC2ResourceRow, len(vpcs))
	for i, vpc := range vpcs {
		rows[i] = views.EC2ResourceRow{ID: vpc.VpcID, Search: tagSearch(vpc.Tags), Cells: []string{
			vpc.Name, vpc.VpcID, vpc.State, strings.Join(vpc.CIDRs, ","), fmt.Sprint(vpc.IsDefault), vpc.Tenancy,
		}}
	}
	return rows
}

func ec2SubnetRows(subnets []model.EC2Subnet) []views.EC2ResourceRow {
	rows := make([]views.EC2ResourceRow, len(subnets))
	for i, subnet := range subnets {
		rows[i] = views.EC2ResourceRow{ID: subnet.SubnetID, Search: tagSearch(subnet.Tags), Cells: []string{
			subnet.Name, subnet.SubnetID, subnet.VpcID, subnet.AZ, subnet.CIDR, fmt.Sprint(subnet.AvailableIPs), fmt.Sprint(subnet.MapPublicIP),
		}}
	}
	return rows
}

func formatTUIEC2VPC(vpc model.EC2VPC) string {
	var out strings.Builder
	fmt.Fprintf(&out, "  VPC: %s\n\n  VPC ID:       %s\n  State:        %s\n  Default:      %t\n  Tenancy:      %s\n  Owner:        %s\n  DHCP options: %s\n  IPv4 CIDRs:   %s\n  IPv6 CIDRs:   %s",
		firstNonBlank(vpc.Name, vpc.VpcID), vpc.VpcID, vpc.State, vpc.IsDefault, vpc.Tenancy, vpc.OwnerID,
		vpc.DHCPOptions, strings.Join(vpc.CIDRs, ", "), strings.Join(vpc.IPv6CIDRs, ", "))
	appendTUITags(&out, vpc.Tags)
	out.WriteString("\n\n  [o] browse linked resources")
	return out.String()
}

func formatTUIEC2Subnet(subnet model.EC2Subnet) string {
	var out strings.Builder
	fmt.Fprintf(&out, "  Subnet: %s\n\n  Subnet ID:       %s\n  VPC:             %s\n  State:           %s\n  AZ:              %s (%s)\n  IPv4 CIDR:       %s\n  IPv6 CIDRs:      %s\n  Available IPs:   %d\n  Map public IP:   %t\n  Default for AZ:  %t\n  Assign IPv6:     %t\n  Owner:           %s",
		firstNonBlank(subnet.Name, subnet.SubnetID), subnet.SubnetID, subnet.VpcID, subnet.State,
		subnet.AZ, subnet.AZID, subnet.CIDR, strings.Join(subnet.IPv6CIDRs, ", "), subnet.AvailableIPs,
		subnet.MapPublicIP, subnet.DefaultForAZ, subnet.AssignIPv6OnCreate, subnet.OwnerID)
	appendTUITags(&out, subnet.Tags)
	out.WriteString("\n\n  [o] open VPC")
	return out.String()
}

func appendTUITags(out *strings.Builder, tags map[string]string) {
	if len(tags) == 0 {
		return
	}
	out.WriteString("\n\n  Tags\n")
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(out, "\n  %-28s %s", key, tags[key])
	}
}

func tagSearch(tags map[string]string) string {
	var values []string
	for key, value := range tags {
		values = append(values, key, value)
	}
	return strings.Join(values, " ")
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "(unnamed)"
}
