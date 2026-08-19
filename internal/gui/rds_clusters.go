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

func (w *mainWindow) openRDSClustersModule() {
	if w.currentPage == pageRDSClusters {
		w.resourceHistory = nil
		w.backButton.SetSensitive(false)
		return
	}
	if w.guardEditorNavigation(w.openRDSClustersModule) {
		return
	}
	w.resourceHistory = nil
	w.loadRDSClusters("")
}

func (w *mainWindow) loadRDSClusters(target string) {
	w.resetWorkspaceForBrowserChange()
	w.clearRDSClusterBrowser()
	w.rdsClusterContext = ""
	w.currentPage = pageRDSClusters
	w.setBreadcrumb("RDS / Clusters")
	w.backButton.SetSensitive(len(w.resourceHistory) > 0)
	w.search.SetPlaceholderText("Filter database clusters…")
	w.search.SetText("")
	w.resourceStack.SetVisibleChildName(pageRDSClusters)
	w.setDetail("Loading RDS clusters…", detailIntro)
	w.updateActionSensitivity()
	if w.options.RDS == nil {
		w.setDetail("RDS is unavailable because no RDS service was configured.", detailError)
		w.setStatus("RDS service unavailable", true)
		return
	}
	ctx, generation := w.startRequest("Loading RDS clusters…")
	go func() {
		clusters, err := w.options.RDS.Clusters(ctx, "")
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageRDSClusters {
				return
			}
			w.allRDSClusters = clusters
			w.applyRDSClusterFilter()
			w.setDetail(rdsClusterListSummary(len(clusters)), detailIntro)
			if target != "" {
				w.selectRDSClusterByID(target)
			}
		})
	}()
}

func (w *mainWindow) refreshRDSClusters(foreground bool) {
	if w.options.RDS == nil {
		return
	}
	selected := w.selectedRDSCluster
	ctx, generation := w.startRefreshRequest("Refreshing RDS clusters…", foreground)
	go func() {
		clusters, err := w.options.RDS.Clusters(ctx, "")
		var detail *model.RDSCluster
		if err == nil && selected != "" {
			detail, err = w.options.RDS.Cluster(ctx, selected)
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			if w.currentPage != pageRDSClusters {
				return
			}
			w.allRDSClusters = clusters
			w.applyRDSClusterFilter()
			if selected == "" || detail == nil {
				w.selectedRDSCluster = ""
				w.rdsClusterDetail = nil
				w.setBreadcrumb("RDS / Clusters")
				w.setDetail(rdsClusterListSummary(len(clusters)), detailIntro)
				w.updateActionSensitivity()
				return
			}
			w.rdsClusterDetail = detail
			w.renderRDSClusterDetail(*detail)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) clearRDSClusterBrowser() {
	w.allRDSClusters = nil
	w.filteredRDSClusters = nil
	w.selectedRDSCluster = ""
	w.rdsClusterDetail = nil
	if w.rdsClusterTable != nil {
		w.rdsClusterTable.clear()
	}
}

func (w *mainWindow) applyRDSClusterFilter() {
	w.filteredRDSClusters = service.FilterRDSClusters(w.allRDSClusters, w.search.Text())
	rows := make([]string, len(w.filteredRDSClusters))
	for i, cluster := range w.filteredRDSClusters {
		writer := "—"
		for _, member := range cluster.Members {
			if member.Writer {
				writer = member.Identifier
				break
			}
		}
		engine := strings.TrimSpace(cluster.Engine + " " + cluster.Version)
		rows[i] = fmt.Sprintf("%s\t%s\t%s\t%d\t%s\t%s", cluster.Identifier, engine,
			valueOrDash(cluster.Status), len(cluster.Members), writer, valueOrDash(cluster.Endpoint))
	}
	w.rdsClusterTable.replace(rows)
}

func (w *mainWindow) selectRDSClusterByID(identifier string) bool {
	for position, cluster := range w.filteredRDSClusters {
		if cluster.Identifier == identifier {
			w.rdsClusterTable.selection.SetSelected(uint(position))
			return true
		}
	}
	w.setStatus("RDS cluster "+identifier+" was not found", true)
	return false
}

func (w *mainWindow) selectRDSClusterRow() {
	if w.currentPage != pageRDSClusters {
		return
	}
	position := w.rdsClusterTable.selection.Selected()
	if position == gtk.InvalidListPosition || int(position) >= len(w.filteredRDSClusters) {
		w.selectedRDSCluster = ""
		w.rdsClusterDetail = nil
		w.setBreadcrumb("RDS / Clusters")
		w.setDetail(rdsClusterListSummary(len(w.allRDSClusters)), detailIntro)
		w.updateActionSensitivity()
		return
	}
	cluster := w.filteredRDSClusters[position]
	if w.selectedRDSCluster != cluster.Identifier {
		w.resetWorkspaceForBrowserChange()
	}
	w.selectedRDSCluster = cluster.Identifier
	w.rdsClusterDetail = nil
	w.setBreadcrumb("RDS / Clusters / " + cluster.Identifier)
	w.setDetail("Loading database cluster details…\n\n"+formatRDSClusterSummary(cluster), detailRDSCluster)
	w.updateActionSensitivity()
	w.loadRDSClusterDetail(cluster.Identifier)
}

func (w *mainWindow) openRDSClusterAt(position uint) {
	if int(position) >= len(w.filteredRDSClusters) {
		return
	}
	w.loadRDSClusterInstances(w.filteredRDSClusters[position].Identifier)
}

func (w *mainWindow) loadRDSClusterDetail(identifier string) {
	if identifier == "" || w.options.RDS == nil {
		return
	}
	ctx, generation := w.startRequest("Loading RDS cluster " + identifier + "…")
	go func() {
		detail, err := w.options.RDS.Cluster(ctx, identifier)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageRDSClusters || w.selectedRDSCluster != identifier {
				return
			}
			w.rdsClusterDetail = detail
			w.renderRDSClusterDetail(*detail)
			w.setStatus("Loaded RDS cluster "+identifier, false)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) renderRDSClusterDetail(cluster model.RDSCluster) {
	w.setDetail(formatRDSClusterDetail(cluster), detailRDSCluster)
	links := make([]workspaceResourceLink, 0, len(cluster.SecurityGroups)+len(cluster.Members))
	for _, member := range cluster.Members {
		links = append(links, workspaceResourceLink{label: "RDS instance " + member.Identifier,
			ref: model.ResourceRef{Kind: "rds-instance", ID: member.Identifier}})
	}
	for _, value := range cluster.SecurityGroups {
		fields := strings.Fields(value)
		if len(fields) > 0 {
			links = append(links, workspaceResourceLink{label: "Security group " + fields[0],
				ref: model.ResourceRef{Kind: "ec2-security-group", ID: fields[0]}})
		}
	}
	w.setDetailResourceLinks(links)
}

func rdsClusterListSummary(count int) string {
	return fmt.Sprintf("RDS CLUSTERS\n\n%d database clusters loaded. Select one for configuration; double-click to browse its member instances.", count)
}

func formatRDSClusterSummary(cluster model.RDSCluster) string {
	return fmt.Sprintf("RDS CLUSTER\n\nIdentifier  %s\nEngine      %s %s\nMode        %s\nStatus      %s\nMembers     %d",
		cluster.Identifier, cluster.Engine, cluster.Version, valueOrDash(cluster.EngineMode),
		valueOrDash(cluster.Status), len(cluster.Members))
}

func formatRDSClusterDetail(cluster model.RDSCluster) string {
	var b strings.Builder
	b.WriteString(formatRDSClusterSummary(cluster))
	fmt.Fprintf(&b, "\nDatabase    %s\nEndpoint    %s:%d\nReader      %s:%d\nMulti-AZ    %t\n",
		valueOrDash(cluster.DatabaseName), valueOrDash(cluster.Endpoint), cluster.Port,
		valueOrDash(cluster.ReaderEndpoint), cluster.Port, cluster.MultiAZ)
	b.WriteString("\nMEMBERS\n\n")
	for _, member := range cluster.Members {
		role := "reader"
		if member.Writer {
			role = "writer"
		}
		fmt.Fprintf(&b, "%s  %s  promotion tier %d\n", member.Identifier, role, member.PromotionTier)
	}
	b.WriteString("\nNETWORK AND SECURITY\n\n")
	fmt.Fprintf(&b, "Subnet group     %s\nAvailability AZs %s\n", valueOrDash(cluster.SubnetGroup), strings.Join(cluster.AvailabilityZones, ", "))
	for _, group := range cluster.SecurityGroups {
		fmt.Fprintf(&b, "Security group   %s\n", group)
	}
	b.WriteString("\nBACKUP AND MAINTENANCE\n\n")
	fmt.Fprintf(&b, "Backup retention %d days\nBackup window    %s UTC\nMaintenance      %s UTC\nEncrypted        %t\nDeletion protect %t\nParameter group  %s\n",
		cluster.BackupRetentionDays, valueOrDash(cluster.BackupWindow), valueOrDash(cluster.MaintenanceWindow),
		cluster.Encrypted, cluster.DeletionProtection, valueOrDash(cluster.ParameterGroup))
	if !cluster.LatestRestorableTime.IsZero() {
		fmt.Fprintf(&b, "Latest restore   %s\n", formatTime(cluster.LatestRestorableTime))
	}
	if len(cluster.Tags) > 0 {
		b.WriteString("\nTAGS\n\n")
		keys := make([]string, 0, len(cluster.Tags))
		for key := range cluster.Tags {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "%s  %s\n", key, cluster.Tags[key])
		}
	}
	return strings.TrimSpace(b.String())
}
