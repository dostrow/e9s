//go:build gui

package gui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

func (w *mainWindow) openEC2VPCsModule() {
	if w.currentPage == pageEC2VPCs {
		w.resourceHistory = nil
		w.backButton.SetSensitive(false)
		return
	}
	w.resourceHistory = nil
	w.loadEC2VPCs("")
}

func (w *mainWindow) loadEC2VPCs(vpcID string) {
	w.resetWorkspaceForBrowserChange()
	w.allEC2VPCs, w.filteredEC2VPCs, w.selectedEC2VPC, w.ec2VPCDetail = nil, nil, "", nil
	w.ec2VPCTable.clear()
	w.currentPage = pageEC2VPCs
	w.setBreadcrumb("EC2 / VPCs")
	w.backButton.SetSensitive(len(w.resourceHistory) > 0)
	w.search.SetPlaceholderText("Filter VPCs…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageEC2VPCs)
	w.setDetail("Loading EC2 VPCs…", detailIntro)
	ctx, generation := w.startRequest("Loading EC2 VPCs…")
	go func() {
		vpcs, err := w.options.EC2Network.VPCs(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			w.allEC2VPCs = vpcs
			w.applyEC2VPCFilter()
			w.setDetail(fmt.Sprintf("Loaded %d VPCs. Select one to inspect its address ranges and settings.", len(vpcs)), detailIntro)
			if vpcID != "" {
				for index, vpc := range w.filteredEC2VPCs {
					if vpc.VpcID == vpcID {
						w.ec2VPCTable.selection.SetSelected(uint(index))
						return
					}
				}
				w.setStatus("EC2 VPC "+vpcID+" was not found", true)
			}
		})
	}()
}

func (w *mainWindow) applyEC2VPCFilter() {
	w.filteredEC2VPCs = service.FilterEC2VPCs(w.allEC2VPCs, w.search.Text())
	rows := make([]string, len(w.filteredEC2VPCs))
	for index, vpc := range w.filteredEC2VPCs {
		rows[index] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s", valueOrDash(vpc.Name), vpc.VpcID,
			valueOrDash(vpc.State), strings.Join(vpc.CIDRs, ", "), yesNo(vpc.IsDefault), valueOrDash(vpc.Tenancy))
	}
	w.ec2VPCTable.replace(rows)
}

func (w *mainWindow) selectEC2VPCRow() {
	if w.currentPage != pageEC2VPCs {
		return
	}
	position := w.ec2VPCTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredEC2VPCs) {
		w.selectedEC2VPC, w.ec2VPCDetail = "", nil
		w.setBreadcrumb("EC2 / VPCs")
		w.setDetail(fmt.Sprintf("Loaded %d VPCs.", len(w.allEC2VPCs)), detailIntro)
		return
	}
	vpc := w.filteredEC2VPCs[position]
	w.resetWorkspaceForBrowserChange()
	w.selectedEC2VPC = vpc.VpcID
	w.setBreadcrumb("EC2 / VPCs / " + firstValue(vpc.Name, vpc.VpcID))
	w.setDetail("Loading VPC details…", detailEC2VPC)
	ctx, generation := w.startRequest("Loading EC2 VPC " + vpc.VpcID + "…")
	go func() {
		detail, err := w.options.EC2Network.VPC(ctx, vpc.VpcID)
		w.finishRequest(ctx, generation, err, func() {
			w.ec2VPCDetail = detail
			w.renderEC2VPC(*detail)
		})
	}()
}

func (w *mainWindow) openEC2VPCAt(position uint) { w.ec2VPCTable.selection.SetSelected(position) }

func (w *mainWindow) renderEC2VPC(vpc model.EC2VPC) {
	var out strings.Builder
	fmt.Fprintf(&out, "VPC\n\nName          %s\nVPC ID        %s\nState         %s\nDefault       %s\nTenancy       %s\nOwner         %s\nDHCP options  %s\nIPv4 CIDRs    %s\nIPv6 CIDRs    %s",
		valueOrDash(vpc.Name), vpc.VpcID, valueOrDash(vpc.State), yesNo(vpc.IsDefault), valueOrDash(vpc.Tenancy),
		valueOrDash(vpc.OwnerID), valueOrDash(vpc.DHCPOptions), valueOrDash(strings.Join(vpc.CIDRs, ", ")),
		valueOrDash(strings.Join(vpc.IPv6CIDRs, ", ")))
	appendSortedTags(&out, vpc.Tags)
	w.setDetail(out.String(), detailEC2VPC)
	w.setDetailResourceLinks([]workspaceResourceLink{{
		label: "Browse subnets in this VPC",
		ref:   model.ResourceRef{Kind: "ec2-subnets", ID: vpc.VpcID, Name: vpc.Name},
	}})
}

func (w *mainWindow) refreshEC2VPCs(foreground bool) {
	selected := w.selectedEC2VPC
	ctx, generation := w.startRefreshRequest("Refreshing EC2 VPCs…", foreground)
	go func() {
		vpcs, err := w.options.EC2Network.VPCs(ctx, "")
		var detail *model.EC2VPC
		if err == nil && selected != "" {
			detail, err = w.options.EC2Network.VPC(ctx, selected)
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allEC2VPCs = vpcs
			w.applyEC2VPCFilter()
			if detail != nil {
				w.ec2VPCDetail = detail
				w.renderEC2VPC(*detail)
			}
		})
	}()
}

func (w *mainWindow) openEC2SubnetsModule() {
	if w.currentPage == pageEC2Subnets {
		w.resourceHistory = nil
		w.backButton.SetSensitive(false)
		return
	}
	w.resourceHistory = nil
	w.loadEC2Subnets("", "")
}

func (w *mainWindow) loadEC2Subnets(subnetID, vpcID string) {
	w.resetWorkspaceForBrowserChange()
	w.allEC2Subnets, w.filteredEC2Subnets, w.selectedEC2Subnet, w.ec2SubnetDetail = nil, nil, "", nil
	w.ec2SubnetVPCFilter = vpcID
	w.ec2SubnetTable.clear()
	w.currentPage = pageEC2Subnets
	breadcrumb := "EC2 / Subnets"
	if vpcID != "" {
		breadcrumb += " / " + vpcID
	}
	w.setBreadcrumb(breadcrumb)
	w.backButton.SetSensitive(len(w.resourceHistory) > 0)
	w.search.SetPlaceholderText("Filter subnets…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageEC2Subnets)
	w.setDetail("Loading EC2 subnets…", detailIntro)
	ctx, generation := w.startRequest("Loading EC2 subnets…")
	go func() {
		subnets, err := w.options.EC2Network.Subnets(ctx, "", vpcID)
		w.finishRequest(ctx, generation, err, func() {
			w.allEC2Subnets = subnets
			w.applyEC2SubnetFilter()
			w.setDetail(fmt.Sprintf("Loaded %d subnets. Select one to inspect its network settings.", len(subnets)), detailIntro)
			if subnetID != "" {
				for index, subnet := range w.filteredEC2Subnets {
					if subnet.SubnetID == subnetID {
						w.ec2SubnetTable.selection.SetSelected(uint(index))
						return
					}
				}
			}
		})
	}()
}

func (w *mainWindow) applyEC2SubnetFilter() {
	w.filteredEC2Subnets = service.FilterEC2Subnets(w.allEC2Subnets, w.search.Text(), w.ec2SubnetVPCFilter)
	rows := make([]string, len(w.filteredEC2Subnets))
	for index, subnet := range w.filteredEC2Subnets {
		rows[index] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%d\t%s", valueOrDash(subnet.Name), subnet.SubnetID,
			subnet.VpcID, subnet.AZ, subnet.CIDR, subnet.AvailableIPs, yesNo(subnet.MapPublicIP))
	}
	w.ec2SubnetTable.replace(rows)
}

func (w *mainWindow) selectEC2SubnetRow() {
	if w.currentPage != pageEC2Subnets {
		return
	}
	position := w.ec2SubnetTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredEC2Subnets) {
		return
	}
	subnet := w.filteredEC2Subnets[position]
	w.resetWorkspaceForBrowserChange()
	w.selectedEC2Subnet = subnet.SubnetID
	w.setBreadcrumb("EC2 / Subnets / " + firstValue(subnet.Name, subnet.SubnetID))
	w.setDetail("Loading subnet details…", detailEC2Subnet)
	ctx, generation := w.startRequest("Loading EC2 subnet " + subnet.SubnetID + "…")
	go func() {
		detail, err := w.options.EC2Network.Subnet(ctx, subnet.SubnetID)
		w.finishRequest(ctx, generation, err, func() {
			w.ec2SubnetDetail = detail
			w.renderEC2Subnet(*detail)
		})
	}()
}

func (w *mainWindow) openEC2SubnetAt(position uint) { w.ec2SubnetTable.selection.SetSelected(position) }

func (w *mainWindow) renderEC2Subnet(subnet model.EC2Subnet) {
	var out strings.Builder
	fmt.Fprintf(&out, "SUBNET\n\nName                   %s\nSubnet ID              %s\nVPC                     %s\nState                   %s\nAvailability zone       %s (%s)\nIPv4 CIDR               %s\nIPv6 CIDRs              %s\nAvailable IPv4 addresses %d\nMap public IP           %s\nDefault for AZ          %s\nAssign IPv6 on create   %s\nOwner                   %s",
		valueOrDash(subnet.Name), subnet.SubnetID, subnet.VpcID, subnet.State, subnet.AZ, subnet.AZID,
		subnet.CIDR, valueOrDash(strings.Join(subnet.IPv6CIDRs, ", ")), subnet.AvailableIPs,
		yesNo(subnet.MapPublicIP), yesNo(subnet.DefaultForAZ), yesNo(subnet.AssignIPv6OnCreate), valueOrDash(subnet.OwnerID))
	appendSortedTags(&out, subnet.Tags)
	w.setDetail(out.String(), detailEC2Subnet)
	w.setDetailResourceLinks([]workspaceResourceLink{{label: "VPC: " + subnet.VpcID,
		ref: model.ResourceRef{Kind: "ec2-vpc", ID: subnet.VpcID}}})
}

func (w *mainWindow) refreshEC2Subnets(foreground bool) {
	selected, vpcID := w.selectedEC2Subnet, w.ec2SubnetVPCFilter
	ctx, generation := w.startRefreshRequest("Refreshing EC2 subnets…", foreground)
	go func() {
		subnets, err := w.options.EC2Network.Subnets(ctx, "", vpcID)
		var detail *model.EC2Subnet
		if err == nil && selected != "" {
			detail, err = w.options.EC2Network.Subnet(ctx, selected)
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allEC2Subnets = subnets
			w.applyEC2SubnetFilter()
			if detail != nil {
				w.ec2SubnetDetail = detail
				w.renderEC2Subnet(*detail)
			}
		})
	}()
}

func appendSortedTags(out *strings.Builder, tags map[string]string) {
	out.WriteString("\n\nTAGS\n")
	if len(tags) == 0 {
		out.WriteString("\n  None")
		return
	}
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(out, "\n%-30s %s", key, tags[key])
	}
}

func firstValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return "—"
}
