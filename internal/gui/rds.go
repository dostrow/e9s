//go:build gui

package gui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

func (w *mainWindow) openRDSModule() {
	if w.currentPage == pageRDSInstances {
		w.resourceHistory = nil
		w.backButton.SetSensitive(false)
		return
	}
	if w.guardEditorNavigation(w.openRDSModule) {
		return
	}
	w.resourceHistory = nil
	w.loadRDSInstances()
}

func (w *mainWindow) loadRDSInstances() {
	w.loadRDSInstancesAt("")
}

func (w *mainWindow) loadRDSInstancesAt(target string) {
	w.resetWorkspaceForBrowserChange()
	w.clearRDSBrowser()
	w.currentPage = pageRDSInstances
	w.setBreadcrumb("RDS / Instances")
	w.backButton.SetSensitive(len(w.resourceHistory) > 0)
	w.search.SetPlaceholderText("Filter database instances…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageRDSInstances)
	w.setDetail("Loading RDS instances…", detailIntro)
	w.updateActionSensitivity()
	if w.options.RDS == nil {
		w.setDetail("RDS is unavailable because no RDS service was configured.", detailError)
		w.setStatus("RDS service unavailable", true)
		return
	}
	ctx, generation := w.startRequest("Loading RDS instances…")
	go func() {
		instances, err := w.options.RDS.List(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			w.allRDSInstances = instances
			w.applyRDSFilter()
			w.setDetail(rdsListSummary(len(instances)), detailIntro)
			if target != "" {
				w.selectRDSInstanceByID(target)
			}
		})
	}()
}

func (w *mainWindow) selectRDSInstanceByID(identifier string) bool {
	for position, instance := range w.filteredRDSInstances {
		if instance.Identifier == identifier {
			w.rdsTable.selection.SetSelected(uint(position))
			return true
		}
	}
	w.setStatus("RDS instance "+identifier+" was not found", true)
	return false
}

func (w *mainWindow) refreshRDS(foreground bool) {
	if w.options.RDS == nil {
		return
	}
	selected := w.selectedRDSInstance
	ctx, generation := w.startRefreshRequest("Refreshing RDS instances…", foreground)
	go func() {
		instances, err := w.options.RDS.List(ctx, "")
		var detail *model.RDSInstanceDetail
		if err == nil && selected != "" {
			detail, err = w.options.RDS.Detail(ctx, selected)
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			w.allRDSInstances = instances
			w.applyRDSFilter()
			if selected == "" {
				w.setDetail(rdsListSummary(len(instances)), detailIntro)
				return
			}
			if detail == nil {
				w.selectedRDSInstance = ""
				w.rdsDetail = nil
				w.setBreadcrumb("RDS / Instances")
				w.setDetail("The selected RDS instance is no longer available.\n\n"+rdsListSummary(len(instances)), detailIntro)
				return
			}
			w.rdsDetail = detail
			w.renderRDSDetail(*detail)
		})
	}()
}

func (w *mainWindow) clearRDSBrowser() {
	w.allRDSInstances = nil
	w.filteredRDSInstances = nil
	w.selectedRDSInstance = ""
	w.rdsDetail = nil
	if w.rdsTable != nil {
		w.rdsTable.clear()
	}
}

func (w *mainWindow) applyRDSFilter() {
	w.filteredRDSInstances = service.FilterRDSInstances(w.allRDSInstances, w.search.Text())
	rows := make([]string, len(w.filteredRDSInstances))
	for i, instance := range w.filteredRDSInstances {
		engine := strings.TrimSpace(instance.Engine + " " + instance.Version)
		endpoint := instance.Endpoint
		if endpoint != "" && instance.Port > 0 {
			endpoint = fmt.Sprintf("%s:%d", endpoint, instance.Port)
		}
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s", instance.Identifier, engine,
			valueOrDash(instance.Class), valueOrDash(instance.Status), valueOrDash(instance.Role),
			valueOrDash(instance.AZ), valueOrDash(endpoint))
	}
	w.rdsTable.replace(rows)
}

func (w *mainWindow) selectRDSInstanceRow() {
	if w.currentPage != pageRDSInstances {
		return
	}
	position := w.rdsTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredRDSInstances) {
		if w.selectedRDSInstance != "" {
			w.resetWorkspaceForBrowserChange()
		}
		w.selectedRDSInstance = ""
		w.rdsDetail = nil
		w.setBreadcrumb("RDS / Instances")
		w.setDetail(rdsListSummary(len(w.allRDSInstances)), detailIntro)
		w.updateActionSensitivity()
		return
	}
	instance := w.filteredRDSInstances[position]
	if w.selectedRDSInstance != instance.Identifier {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedRDSInstance = instance.Identifier
	w.rdsDetail = nil
	w.setBreadcrumb("RDS / Instances / " + instance.Identifier)
	w.setDetail("Loading database instance details…\n\n"+formatRDSInstanceSummary(instance), detailRDS)
	w.updateActionSensitivity()
	w.loadRDSDetail(instance.Identifier)
}

func (w *mainWindow) openRDSInstanceAt(position uint) {
	if int(position) >= len(w.filteredRDSInstances) {
		return
	}
	if w.rdsTable.selection.Selected() == position {
		w.loadRDSDetail(w.filteredRDSInstances[position].Identifier)
	} else {
		w.rdsTable.selection.SetSelected(position)
	}
}

func (w *mainWindow) loadRDSDetail(identifier string) {
	if identifier == "" || w.options.RDS == nil {
		return
	}
	ctx, generation := w.startRequest("Loading RDS instance " + identifier + "…")
	go func() {
		detail, err := w.options.RDS.Detail(ctx, identifier)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageRDSInstances || w.selectedRDSInstance != identifier {
				return
			}
			w.rdsDetail = detail
			w.renderRDSDetail(*detail)
			w.setStatus("Loaded RDS instance "+identifier, false)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) renderRDSDetail(detail model.RDSInstanceDetail) {
	w.setDetail(formatRDSDetail(detail), detailRDS)
	links := make([]workspaceResourceLink, 0, len(detail.SecurityGroups)+1)
	if detail.VPCID != "" {
		links = append(links, workspaceResourceLink{label: "VPC " + detail.VPCID, ref: model.ResourceRef{Kind: "ec2-vpc", ID: detail.VPCID}})
	}
	for _, value := range detail.SecurityGroups {
		groupID := strings.Fields(value)
		if len(groupID) > 0 {
			links = append(links, workspaceResourceLink{label: "Security group " + groupID[0], ref: model.ResourceRef{Kind: "ec2-security-group", ID: groupID[0]}})
		}
	}
	w.setDetailResourceLinks(links)
}

func rdsListSummary(count int) string {
	return fmt.Sprintf("RDS INSTANCES\n\n%d database instances loaded. Select one to inspect configuration and open its metrics dashboard.", count)
}

func formatRDSInstanceSummary(instance model.RDSInstance) string {
	return fmt.Sprintf("RDS INSTANCE\n\nIdentifier  %s\nEngine      %s %s\nClass       %s\nStatus      %s\nRole        %s\nAZ          %s",
		instance.Identifier, instance.Engine, instance.Version, valueOrDash(instance.Class),
		valueOrDash(instance.Status), valueOrDash(instance.Role), valueOrDash(instance.AZ))
}

func formatRDSDetail(detail model.RDSInstanceDetail) string {
	var b strings.Builder
	b.WriteString(formatRDSInstanceSummary(detail.RDSInstance))
	fmt.Fprintf(&b, "\nEndpoint    %s:%d\nMulti-AZ    %t\n", valueOrDash(detail.Endpoint), detail.Port, detail.MultiAZ)
	if detail.ClusterID != "" {
		fmt.Fprintf(&b, "Cluster     %s\n", detail.ClusterID)
	}
	b.WriteString("\nNETWORK\n\n")
	fmt.Fprintf(&b, "VPC              %s\nSubnet group     %s\n", valueOrDash(detail.VPCID), valueOrDash(detail.SubnetGroup))
	for _, group := range detail.SecurityGroups {
		fmt.Fprintf(&b, "Security group   %s\n", group)
	}
	b.WriteString("\nSTORAGE\n\n")
	fmt.Fprintf(&b, "Allocated        %d GiB\nType             %s\nEncrypted        %t\nDeletion protect %t\n",
		detail.StorageGB, valueOrDash(detail.StorageType), detail.Encrypted, detail.DeletionProtection)
	b.WriteString("\nBACKUP AND MAINTENANCE\n\n")
	fmt.Fprintf(&b, "Backup retention %d days\nBackup window    %s UTC\nMaintenance      %s UTC\n",
		detail.BackupRetentionDays, valueOrDash(detail.BackupWindow), valueOrDash(detail.MaintenanceWindow))
	if !detail.LatestRestorableTime.IsZero() {
		fmt.Fprintf(&b, "Latest restore   %s\n", formatTime(detail.LatestRestorableTime))
	}
	if len(detail.ParameterGroups) > 0 {
		fmt.Fprintf(&b, "Parameter groups %s\n", strings.Join(detail.ParameterGroups, ", "))
	}
	if len(detail.Tags) > 0 {
		b.WriteString("\nTAGS\n\n")
		keys := make([]string, 0, len(detail.Tags))
		for key := range detail.Tags {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "%s  %s\n", key, detail.Tags[key])
		}
	}
	return strings.TrimSpace(b.String())
}

func rdsAge(created time.Time) string {
	if created.IsZero() {
		return "—"
	}
	return formatTime(created)
}
