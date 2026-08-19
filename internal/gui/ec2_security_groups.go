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

func (w *mainWindow) openEC2SecurityGroupsModule() {
	if w.currentPage == pageEC2SecurityGroups {
		w.resourceHistory = nil
		w.backButton.SetSensitive(false)
		return
	}
	if w.guardEditorNavigation(w.openEC2SecurityGroupsModule) {
		return
	}
	w.resourceHistory = nil
	w.loadEC2SecurityGroups("")
}

func (w *mainWindow) loadEC2SecurityGroups(groupID string) {
	w.resetWorkspaceForBrowserChange()
	w.clearEC2SecurityGroupBrowser()
	w.currentPage = pageEC2SecurityGroups
	w.updateActionSensitivity()
	w.setBreadcrumb("EC2 / Security Groups")
	w.backButton.SetSensitive(len(w.resourceHistory) > 0)
	w.search.SetPlaceholderText("Filter security groups…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageEC2SecurityGroups)
	w.setDetail("Loading EC2 security groups…", detailIntro)

	if w.options.EC2Network == nil {
		w.setDetail("EC2 networking is unavailable because no networking service was configured.", detailError)
		w.setStatus("EC2 networking service unavailable", true)
		return
	}
	ctx, generation := w.startRequest("Loading EC2 security groups…")
	go func() {
		groups, err := w.options.EC2Network.SecurityGroups(ctx, "", "")
		w.finishRequest(ctx, generation, err, func() {
			w.allEC2SecurityGroups = groups
			w.applyEC2SecurityGroupFilter()
			w.setDetail(ec2SecurityGroupListSummary(len(groups)), detailIntro)
			if groupID != "" {
				w.selectEC2SecurityGroupByID(groupID)
			}
		})
	}()
}

func (w *mainWindow) refreshEC2SecurityGroups(foreground bool) {
	if w.options.EC2Network == nil {
		return
	}
	selected := w.selectedEC2SecurityGroup
	ctx, generation := w.startRefreshRequest("Refreshing EC2 security groups…", foreground)
	go func() {
		groups, err := w.options.EC2Network.SecurityGroups(ctx, "", "")
		var detail *model.EC2SecurityGroup
		if err == nil && selected != "" {
			detail, err = w.options.EC2Network.SecurityGroup(ctx, selected)
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allEC2SecurityGroups = groups
			w.applyEC2SecurityGroupFilter()
			if selected == "" {
				w.setDetail(ec2SecurityGroupListSummary(len(groups)), detailIntro)
				return
			}
			if detail == nil {
				w.selectedEC2SecurityGroup = ""
				w.ec2SecurityGroupDetail = nil
				w.setBreadcrumb("EC2 / Security Groups")
				w.setDetail("The selected security group is no longer available.", detailIntro)
				return
			}
			w.ec2SecurityGroupDetail = detail
			w.renderEC2SecurityGroup(*detail)
		})
	}()
}

func (w *mainWindow) clearEC2SecurityGroupBrowser() {
	w.allEC2SecurityGroups = nil
	w.filteredEC2SecurityGroups = nil
	w.selectedEC2SecurityGroup = ""
	w.ec2SecurityGroupDetail = nil
	if w.ec2SecurityGroupTable != nil {
		w.ec2SecurityGroupTable.clear()
	}
}

func (w *mainWindow) applyEC2SecurityGroupFilter() {
	w.filteredEC2SecurityGroups = service.FilterEC2SecurityGroups(w.allEC2SecurityGroups, w.search.Text(), "")
	rows := make([]string, len(w.filteredEC2SecurityGroups))
	for index, group := range w.filteredEC2SecurityGroups {
		inbound, outbound := 0, 0
		for _, rule := range group.Rules {
			if rule.Direction == "inbound" {
				inbound++
			} else if rule.Direction == "outbound" {
				outbound++
			}
		}
		rows[index] = fmt.Sprintf("%s\t%s\t%s\t%d\t%d\t%s", valueOrDash(group.Name), group.GroupID,
			valueOrDash(group.VpcID), inbound, outbound, valueOrDash(group.Description))
	}
	w.ec2SecurityGroupTable.replace(rows)
}

func (w *mainWindow) selectEC2SecurityGroupRow() {
	if w.currentPage != pageEC2SecurityGroups {
		return
	}
	position := w.ec2SecurityGroupTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredEC2SecurityGroups) {
		w.selectedEC2SecurityGroup = ""
		w.ec2SecurityGroupDetail = nil
		w.setBreadcrumb("EC2 / Security Groups")
		w.setDetail(ec2SecurityGroupListSummary(len(w.allEC2SecurityGroups)), detailIntro)
		return
	}
	group := w.filteredEC2SecurityGroups[position]
	if w.selectedEC2SecurityGroup != group.GroupID {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedEC2SecurityGroup = group.GroupID
	w.ec2SecurityGroupDetail = nil
	w.setBreadcrumb("EC2 / Security Groups / " + ec2SecurityGroupDisplayName(group))
	w.setDetail("Loading security group details…\n\n"+formatEC2SecurityGroup(group), detailEC2SecurityGroup)
	w.loadEC2SecurityGroupDetail(group.GroupID)
}

func (w *mainWindow) openEC2SecurityGroupAt(position uint) {
	if int(position) >= len(w.filteredEC2SecurityGroups) {
		return
	}
	if w.ec2SecurityGroupTable.selection.Selected() == position {
		w.loadEC2SecurityGroupDetail(w.filteredEC2SecurityGroups[position].GroupID)
	} else {
		w.ec2SecurityGroupTable.selection.SetSelected(position)
	}
}

func (w *mainWindow) loadEC2SecurityGroupDetail(groupID string) {
	if groupID == "" || w.options.EC2Network == nil {
		return
	}
	ctx, generation := w.startRequest("Loading EC2 security group " + groupID + "…")
	go func() {
		group, err := w.options.EC2Network.SecurityGroup(ctx, groupID)
		w.finishRequestWithStatus(ctx, generation, err, "Loaded EC2 security group "+groupID, func() {
			if w.currentPage != pageEC2SecurityGroups || w.selectedEC2SecurityGroup != groupID {
				return
			}
			w.ec2SecurityGroupDetail = group
			w.renderEC2SecurityGroup(*group)
		})
	}()
}

func (w *mainWindow) selectEC2SecurityGroupByID(groupID string) bool {
	for position, group := range w.filteredEC2SecurityGroups {
		if group.GroupID == groupID {
			w.ec2SecurityGroupTable.selection.SetSelected(uint(position))
			return true
		}
	}
	w.setStatus("EC2 security group "+groupID+" was not found", true)
	return false
}

func (w *mainWindow) renderEC2SecurityGroup(group model.EC2SecurityGroup) {
	w.setDetail(formatEC2SecurityGroup(group), detailEC2SecurityGroup)
	links := make([]workspaceResourceLink, 0, len(group.Associations))
	for _, association := range group.Associations {
		if association.Kind != "ec2-instance" {
			continue
		}
		links = append(links, workspaceResourceLink{
			label: "Instance: " + association.ID,
			ref:   association,
		})
	}
	w.setDetailResourceLinks(links)
}

func formatEC2SecurityGroup(group model.EC2SecurityGroup) string {
	var out strings.Builder
	fmt.Fprintf(&out, "SECURITY GROUP\n\nName         %s\nGroup ID     %s\nVPC          %s\nOwner        %s\nDescription  %s",
		valueOrDash(group.Name), valueOrDash(group.GroupID), valueOrDash(group.VpcID),
		valueOrDash(group.OwnerID), valueOrDash(group.Description))
	out.WriteString("\n\nRULES\n")
	if len(group.Rules) == 0 {
		out.WriteString("\n  None\n")
	} else {
		fmt.Fprintf(&out, "\n%-10s %-10s %-12s %-24s %-20s %s\n", "DIRECTION", "PROTOCOL", "PORTS", "SOURCE / DESTINATION", "RULE ID", "DESCRIPTION")
		for _, rule := range group.Rules {
			fmt.Fprintf(&out, "%-10s %-10s %-12s %-24s %-20s %s\n", valueOrDash(rule.Direction),
				valueOrDash(rule.Protocol), valueOrDash(rule.PortRange), valueOrDash(rule.Source),
				valueOrDash(rule.RuleID), valueOrDash(rule.Description))
		}
	}
	out.WriteString("\nASSOCIATED RESOURCES\n")
	if len(group.Associations) == 0 {
		out.WriteString("\n  None\n")
	} else {
		for _, association := range group.Associations {
			fmt.Fprintf(&out, "\n  %-20s %-24s %s", association.Kind, association.ID, valueOrDash(association.Name))
		}
		out.WriteByte('\n')
	}
	out.WriteString("\nTAGS\n")
	if len(group.Tags) == 0 {
		out.WriteString("\n  None")
	} else {
		keys := make([]string, 0, len(group.Tags))
		for key := range group.Tags {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&out, "\n%-30s %s", key, group.Tags[key])
		}
	}
	return strings.TrimRight(out.String(), "\n")
}

func ec2SecurityGroupDisplayName(group model.EC2SecurityGroup) string {
	if group.Name != "" {
		return group.Name
	}
	return group.GroupID
}

func ec2SecurityGroupListSummary(count int) string {
	return fmt.Sprintf("Loaded %d EC2 security groups. Select one to inspect rules and associated resources.", count)
}
