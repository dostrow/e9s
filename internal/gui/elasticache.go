//go:build gui

package gui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/service"
)

func (w *mainWindow) openElastiCacheModule(kind model.ElastiCacheKind) {
	if kind == "" {
		kind = model.ElastiCacheReplicationGroup
	}
	w.resetWorkspaceForBrowserChange()
	w.resourceHistory = nil
	w.clearElastiCacheBrowser()
	w.elastiCacheKind = kind
	w.currentPage = pageElastiCache
	w.setBreadcrumb("ElastiCache / " + elastiCacheKindTitle(kind))
	w.backButton.SetSensitive(false)
	w.search.SetText("")
	w.search.SetPlaceholderText("Filter " + strings.ToLower(elastiCacheKindTitle(kind)) + "…")
	w.resourceStack.SetVisibleChildName(pageElastiCache)
	w.setDetail("Loading ElastiCache "+strings.ToLower(elastiCacheKindTitle(kind))+"…", detailIntro)
	w.updateActionSensitivity()
	if w.options.ElastiCache == nil {
		w.setDetail("ElastiCache is unavailable because no service was configured.", detailError)
		w.setStatus("ElastiCache service unavailable", true)
		return
	}
	ctx, generation := w.startRequest("Loading ElastiCache " + strings.ToLower(elastiCacheKindTitle(kind)) + "…")
	go func() {
		resources, err := w.options.ElastiCache.List(ctx, kind, "")
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageElastiCache || w.elastiCacheKind != kind {
				return
			}
			w.allElastiCache = resources
			w.applyElastiCacheFilter()
			w.setDetail(elastiCacheListSummary(kind, len(resources)), detailIntro)
			w.setStatus(fmt.Sprintf("Loaded %d ElastiCache %s", len(resources), strings.ToLower(elastiCacheKindTitle(kind))), false)
		})
	}()
}

func (w *mainWindow) refreshElastiCache(foreground bool) {
	if w.options.ElastiCache == nil {
		return
	}
	kind, selected := w.elastiCacheKind, w.selectedElastiCache
	ctx, generation := w.startRefreshRequest("Refreshing ElastiCache…", foreground)
	go func() {
		resources, err := w.options.ElastiCache.List(ctx, kind, "")
		var detail *model.ElastiCacheResource
		if err == nil && selected != "" {
			detail, err = w.options.ElastiCache.Detail(ctx, kind, selected)
		}
		w.finishRefreshRequest(ctx, generation, err, foreground, func() {
			if w.currentPage != pageElastiCache || w.elastiCacheKind != kind {
				return
			}
			w.allElastiCache = resources
			w.applyElastiCacheFilter()
			if selected == "" {
				w.setDetail(elastiCacheListSummary(kind, len(resources)), detailIntro)
				return
			}
			if detail == nil {
				w.selectedElastiCache = ""
				w.elastiCacheDetail = nil
				w.setBreadcrumb("ElastiCache / " + elastiCacheKindTitle(kind))
				w.setDetail("The selected cache is no longer available.\n\n"+elastiCacheListSummary(kind, len(resources)), detailIntro)
				w.updateActionSensitivity()
				return
			}
			w.elastiCacheDetail = detail
			w.renderElastiCacheDetail(*detail)
		})
	}()
}

func (w *mainWindow) clearElastiCacheBrowser() {
	w.allElastiCache = nil
	w.filteredElastiCache = nil
	w.selectedElastiCache = ""
	w.elastiCacheDetail = nil
	if w.elastiCacheTable != nil {
		w.elastiCacheTable.clear()
	}
}

func (w *mainWindow) applyElastiCacheFilter() {
	w.filteredElastiCache = service.FilterElastiCache(w.allElastiCache, w.search.Text())
	rows := make([]string, len(w.filteredElastiCache))
	for index, resource := range w.filteredElastiCache {
		engine := strings.TrimSpace(resource.Engine + " " + resource.Version)
		resourceType := resource.NodeType
		if resource.Kind == model.ElastiCacheServerless {
			resourceType = "serverless"
		}
		rows[index] = fmt.Sprintf("%s\t%s\t%s\t%s\t%d\t%s", resource.ID, valueOrDash(engine),
			valueOrDash(resource.Status), valueOrDash(resourceType), resource.NodeCount, elastiCacheEndpointLabel(resource.Endpoint, resource.Port))
	}
	w.elastiCacheTable.replace(rows)
}

func (w *mainWindow) selectElastiCacheRow() {
	position := int(w.elastiCacheTable.selection.Selected())
	if position < 0 || position >= len(w.filteredElastiCache) {
		return
	}
	resource := w.filteredElastiCache[position]
	w.selectedElastiCache = resource.ID
	w.elastiCacheDetail = nil
	w.setBreadcrumb("ElastiCache / " + elastiCacheKindTitle(resource.Kind) + " / " + resource.ID)
	w.setDetail(formatElastiCacheSummary(resource)+"\n\nLoading complete configuration…", detailElastiCache)
	w.updateActionSensitivity()
	if w.options.ElastiCache == nil {
		return
	}
	kind, identifier := resource.Kind, resource.ID
	ctx, generation := w.startRequest("Loading ElastiCache " + identifier + "…")
	go func() {
		detail, err := w.options.ElastiCache.Detail(ctx, kind, identifier)
		w.finishRequest(ctx, generation, err, func() {
			if w.currentPage != pageElastiCache || w.elastiCacheKind != kind || w.selectedElastiCache != identifier {
				return
			}
			w.elastiCacheDetail = detail
			w.renderElastiCacheDetail(*detail)
			w.setStatus("Loaded ElastiCache "+identifier, false)
			w.updateActionSensitivity()
		})
	}()
}

func (w *mainWindow) openElastiCacheAt(position uint) {
	if int(position) >= len(w.filteredElastiCache) {
		return
	}
	w.elastiCacheTable.selection.SetSelected(position)
}

func (w *mainWindow) renderElastiCacheDetail(resource model.ElastiCacheResource) {
	w.setDetail(formatElastiCacheDetail(resource), detailElastiCache)
	links := make([]workspaceResourceLink, 0, len(resource.SecurityGroupIDs)+len(resource.SubnetIDs))
	for _, identifier := range resource.SecurityGroupIDs {
		links = append(links, workspaceResourceLink{label: "Security group " + identifier,
			ref: model.ResourceRef{Kind: "ec2-security-group", ID: identifier}})
	}
	for _, identifier := range resource.SubnetIDs {
		links = append(links, workspaceResourceLink{label: "Subnet " + identifier,
			ref: model.ResourceRef{Kind: "ec2-subnet", ID: identifier}})
	}
	w.setDetailResourceLinks(links)
}

func elastiCacheKindTitle(kind model.ElastiCacheKind) string {
	switch kind {
	case model.ElastiCacheCluster:
		return "Cache Clusters"
	case model.ElastiCacheServerless:
		return "Serverless Caches"
	default:
		return "Replication Groups"
	}
}

func elastiCacheListSummary(kind model.ElastiCacheKind, count int) string {
	return fmt.Sprintf("ELASTICACHE %s\n\n%d resources loaded. Select one to inspect its topology and configuration.",
		strings.ToUpper(elastiCacheKindTitle(kind)), count)
}

func formatElastiCacheSummary(resource model.ElastiCacheResource) string {
	return fmt.Sprintf("ELASTICACHE %s\n\nIdentifier  %s\nEngine      %s %s\nStatus      %s\nNodes       %d\nShards      %d\nEndpoint    %s",
		strings.ToUpper(strings.ReplaceAll(string(resource.Kind), "-", " ")), resource.ID, valueOrDash(resource.Engine),
		valueOrDash(resource.Version), valueOrDash(resource.Status), resource.NodeCount, resource.ShardCount,
		elastiCacheEndpointLabel(resource.Endpoint, resource.Port))
}

func formatElastiCacheDetail(resource model.ElastiCacheResource) string {
	var b strings.Builder
	b.WriteString(formatElastiCacheSummary(resource))
	fmt.Fprintf(&b, "\nReader      %s\nNode type   %s\nNetwork     %s\n", valueOrDash(resource.ReaderEndpoint), valueOrDash(resource.NodeType), valueOrDash(resource.NetworkType))
	if resource.Description != "" {
		fmt.Fprintf(&b, "Description  %s\n", resource.Description)
	}
	if resource.ReplicationGroupID != "" {
		fmt.Fprintf(&b, "Replication %s\n", resource.ReplicationGroupID)
	}
	b.WriteString("\nTOPOLOGY AND AVAILABILITY\n\n")
	fmt.Fprintf(&b, "Cluster mode       %s\nAutomatic failover %s\nMulti-AZ            %s\nAvailability zones %s\n",
		valueOrDash(resource.ClusterMode), valueOrDash(resource.AutomaticFailover), valueOrDash(resource.MultiAZ),
		valueOrDash(strings.Join(resource.AvailabilityZones, ", ")))
	for _, node := range resource.Nodes {
		fmt.Fprintf(&b, "Node %-8s %-8s %-10s %s\n", valueOrDash(node.NodeID), valueOrDash(node.Role), valueOrDash(node.Status),
			elastiCacheEndpointLabel(node.Endpoint, node.Port))
	}
	b.WriteString("\nNETWORK AND SECURITY\n\n")
	fmt.Fprintf(&b, "Subnet group       %s\nSubnets            %s\nSecurity groups    %s\nEncryption at rest %t\nEncryption transit %t\nAuth token enabled %t\nKMS key            %s\nUser groups        %s\n",
		valueOrDash(resource.SubnetGroup), valueOrDash(strings.Join(resource.SubnetIDs, ", ")),
		valueOrDash(strings.Join(resource.SecurityGroupIDs, ", ")), resource.AtRestEncrypted, resource.TransitEncrypted,
		resource.AuthTokenEnabled, valueOrDash(resource.KMSKeyID), valueOrDash(strings.Join(resource.UserGroupIDs, ", ")))
	b.WriteString("\nCONFIGURATION\n\n")
	fmt.Fprintf(&b, "Parameter group    %s\nMaintenance window %s\nSnapshot retention %d days\nSnapshot window    %s\n",
		valueOrDash(resource.ParameterGroup), valueOrDash(resource.MaintenanceWindow), resource.SnapshotRetention, valueOrDash(resource.SnapshotWindow))
	if resource.DataStorageLimitGB > 0 || resource.ECPUPerSecondLimit > 0 {
		fmt.Fprintf(&b, "Storage limit      %.0f GB\nECPU/sec limit     %d\n", resource.DataStorageLimitGB, resource.ECPUPerSecondLimit)
	}
	if len(resource.LogGroups) > 0 {
		fmt.Fprintf(&b, "Log groups         %s\n", strings.Join(resource.LogGroups, ", "))
	}
	if !resource.Created.IsZero() {
		fmt.Fprintf(&b, "Created            %s\n", formatTime(resource.Created))
	}
	if len(resource.Tags) > 0 {
		b.WriteString("\nTAGS\n\n")
		keys := make([]string, 0, len(resource.Tags))
		for key := range resource.Tags {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, "%s  %s\n", key, resource.Tags[key])
		}
	}
	return strings.TrimSpace(b.String())
}

func elastiCacheEndpointLabel(endpoint string, port int32) string {
	if endpoint == "" {
		return "—"
	}
	if port <= 0 {
		return endpoint
	}
	return fmt.Sprintf("%s:%d", endpoint, port)
}
