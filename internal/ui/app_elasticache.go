package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dostrow/e9s/internal/model"
	"github.com/dostrow/e9s/internal/ui/views"
)

func (a App) openElastiCache(kind model.ElastiCacheKind) (App, tea.Cmd) {
	if kind == "" {
		kind = model.ElastiCacheReplicationGroup
	}
	a.mode = modeElastiCache
	a.state = viewElastiCache
	a.elastiCacheKind = kind
	a.selectedElastiCache = nil
	a.elastiCacheView = views.NewEC2ResourceList(elastiCacheTUITitle(kind), []string{"IDENTIFIER", "ENGINE", "STATUS", "TYPE", "NODES", "ENDPOINT"}).SetSize(a.width-3, a.height-6)
	a.loading = true
	return a, a.loadElastiCache(kind)
}

func (a App) loadElastiCache(kind model.ElastiCacheKind) tea.Cmd {
	cacheService, ctx := a.elastiCache, a.ctx
	return func() tea.Msg {
		resources, err := cacheService.List(ctx, kind, "")
		if err != nil {
			return errMsg{err}
		}
		return elastiCacheLoadedMsg{kind: kind, resources: resources}
	}
}

func (a App) openElastiCacheDetail() (App, tea.Cmd) {
	id := a.elastiCacheView.SelectedID()
	if id == "" {
		return a, nil
	}
	kind, cacheService, ctx := a.elastiCacheKind, a.elastiCache, a.ctx
	a.loading = true
	return a, func() tea.Msg {
		resource, err := cacheService.Detail(ctx, kind, id)
		if err != nil {
			return errMsg{err}
		}
		metrics, err := cacheService.Metrics(ctx, *resource, 15*time.Minute)
		if err != nil {
			return errMsg{err}
		}
		return elastiCacheDetailLoadedMsg{resource: resource, metrics: metrics}
	}
}

func elastiCacheTUITitle(kind model.ElastiCacheKind) string {
	switch kind {
	case model.ElastiCacheCluster:
		return "ElastiCache Cache Clusters"
	case model.ElastiCacheServerless:
		return "ElastiCache Serverless Caches"
	default:
		return "ElastiCache Replication Groups"
	}
}

func elastiCacheTUIRows(resources []model.ElastiCacheResource) []views.EC2ResourceRow {
	rows := make([]views.EC2ResourceRow, 0, len(resources))
	for _, resource := range resources {
		resourceType := resource.NodeType
		if resource.Kind == model.ElastiCacheServerless {
			resourceType = "serverless"
		}
		endpoint := resource.Endpoint
		if resource.Port > 0 && endpoint != "" {
			endpoint = fmt.Sprintf("%s:%d", endpoint, resource.Port)
		}
		rows = append(rows, views.EC2ResourceRow{ID: resource.ID, Search: resource.ARN + " " + strings.Join(resource.SecurityGroupIDs, " "), Cells: []string{
			resource.ID, strings.TrimSpace(resource.Engine + " " + resource.Version), resource.Status, resourceType,
			fmt.Sprintf("%d", resource.NodeCount), endpoint,
		}})
	}
	return rows
}

func formatTUIElastiCache(resource model.ElastiCacheResource, snapshot *model.MetricSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ELASTICACHE %s\n\nIdentifier          %s\nARN                 %s\nEngine              %s %s\nStatus              %s\nNode type           %s\nNodes / shards      %d / %d\nEndpoint            %s\nReader endpoint     %s\n",
		strings.ToUpper(strings.ReplaceAll(string(resource.Kind), "-", " ")), resource.ID, resource.ARN, resource.Engine,
		resource.Version, resource.Status, resource.NodeType, resource.NodeCount, resource.ShardCount,
		resource.Endpoint, resource.ReaderEndpoint)
	fmt.Fprintf(&b, "\nNETWORK AND SECURITY\n\nSubnet group        %s\nSubnets             %s\nSecurity groups     %s\nNetwork type        %s\nEncryption at rest  %t\nEncryption transit  %t\nAuth token enabled  %t\n",
		resource.SubnetGroup, strings.Join(resource.SubnetIDs, ", "), strings.Join(resource.SecurityGroupIDs, ", "),
		resource.NetworkType, resource.AtRestEncrypted, resource.TransitEncrypted, resource.AuthTokenEnabled)
	fmt.Fprintf(&b, "\nAVAILABILITY AND CONFIGURATION\n\nCluster mode        %s\nAutomatic failover  %s\nMulti-AZ             %s\nAvailability zones  %s\nParameter group     %s\nMaintenance window  %s\nSnapshot retention  %d days\n",
		resource.ClusterMode, resource.AutomaticFailover, resource.MultiAZ, strings.Join(resource.AvailabilityZones, ", "),
		resource.ParameterGroup, resource.MaintenanceWindow, resource.SnapshotRetention)
	if snapshot != nil && len(snapshot.Series) > 0 {
		b.WriteString("\nLATEST CLOUDWATCH METRICS (15 MINUTES)\n\n")
		for _, series := range snapshot.Series {
			if len(series.Points) == 0 {
				continue
			}
			point := series.Points[len(series.Points)-1]
			fmt.Fprintf(&b, "%-24s %.2f %s\n", series.Label, point.Value, series.Unit)
		}
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
