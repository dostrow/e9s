package aws

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	"github.com/dostrow/e9s/internal/model"
)

func (c *Client) ListElastiCacheReplicationGroups(ctx context.Context) ([]model.ElastiCacheResource, error) {
	if c.ElastiCache == nil {
		return nil, fmt.Errorf("ElastiCache client is unavailable")
	}
	clusters, err := c.listElastiCacheClusters(ctx)
	if err != nil {
		return nil, err
	}
	clusterByID := make(map[string]elasticachetypes.CacheCluster, len(clusters))
	for _, cluster := range clusters {
		clusterByID[awssdk.ToString(cluster.CacheClusterId)] = cluster
	}
	paginator := elasticache.NewDescribeReplicationGroupsPaginator(c.ElastiCache, &elasticache.DescribeReplicationGroupsInput{})
	var resources []model.ElastiCacheResource
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, group := range page.ReplicationGroups {
			resources = append(resources, elasticacheReplicationGroup(group, clusterByID))
		}
	}
	return resources, nil
}

func (c *Client) ListElastiCacheClusters(ctx context.Context) ([]model.ElastiCacheResource, error) {
	if c.ElastiCache == nil {
		return nil, fmt.Errorf("ElastiCache client is unavailable")
	}
	clusters, err := c.listElastiCacheClusters(ctx)
	if err != nil {
		return nil, err
	}
	resources := make([]model.ElastiCacheResource, 0, len(clusters))
	for _, cluster := range clusters {
		resources = append(resources, elasticacheCluster(cluster))
	}
	return resources, nil
}

func (c *Client) ListElastiCacheServerless(ctx context.Context) ([]model.ElastiCacheResource, error) {
	if c.ElastiCache == nil {
		return nil, fmt.Errorf("ElastiCache client is unavailable")
	}
	paginator := elasticache.NewDescribeServerlessCachesPaginator(c.ElastiCache, &elasticache.DescribeServerlessCachesInput{})
	var resources []model.ElastiCacheResource
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, cache := range page.ServerlessCaches {
			resources = append(resources, elasticacheServerless(cache))
		}
	}
	return resources, nil
}

func (c *Client) DescribeElastiCache(ctx context.Context, kind model.ElastiCacheKind, identifier string) (*model.ElastiCacheResource, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, fmt.Errorf("ElastiCache identifier is required")
	}
	if c.ElastiCache == nil {
		return nil, fmt.Errorf("ElastiCache client is unavailable")
	}
	var resource model.ElastiCacheResource
	switch kind {
	case model.ElastiCacheReplicationGroup:
		out, err := c.ElastiCache.DescribeReplicationGroups(ctx, &elasticache.DescribeReplicationGroupsInput{ReplicationGroupId: awssdk.String(identifier)})
		if err != nil {
			return nil, err
		}
		if len(out.ReplicationGroups) == 0 {
			return nil, fmt.Errorf("ElastiCache replication group %q was not found", identifier)
		}
		clusters, err := c.listElastiCacheClusters(ctx)
		if err != nil {
			return nil, err
		}
		clusterByID := make(map[string]elasticachetypes.CacheCluster, len(clusters))
		for _, cluster := range clusters {
			clusterByID[awssdk.ToString(cluster.CacheClusterId)] = cluster
		}
		resource = elasticacheReplicationGroup(out.ReplicationGroups[0], clusterByID)
	case model.ElastiCacheCluster:
		out, err := c.ElastiCache.DescribeCacheClusters(ctx, &elasticache.DescribeCacheClustersInput{
			CacheClusterId: awssdk.String(identifier), ShowCacheNodeInfo: awssdk.Bool(true),
		})
		if err != nil {
			return nil, err
		}
		if len(out.CacheClusters) == 0 {
			return nil, fmt.Errorf("ElastiCache cluster %q was not found", identifier)
		}
		resource = elasticacheCluster(out.CacheClusters[0])
	case model.ElastiCacheServerless:
		out, err := c.ElastiCache.DescribeServerlessCaches(ctx, &elasticache.DescribeServerlessCachesInput{ServerlessCacheName: awssdk.String(identifier)})
		if err != nil {
			return nil, err
		}
		if len(out.ServerlessCaches) == 0 {
			return nil, fmt.Errorf("ElastiCache serverless cache %q was not found", identifier)
		}
		resource = elasticacheServerless(out.ServerlessCaches[0])
	default:
		return nil, fmt.Errorf("unsupported ElastiCache resource kind %q", kind)
	}
	if resource.ARN != "" {
		tags, err := c.ElastiCache.ListTagsForResource(ctx, &elasticache.ListTagsForResourceInput{ResourceName: awssdk.String(resource.ARN)})
		if err == nil {
			resource.Tags = make(map[string]string, len(tags.TagList))
			for _, tag := range tags.TagList {
				resource.Tags[awssdk.ToString(tag.Key)] = awssdk.ToString(tag.Value)
			}
		}
	}
	return &resource, nil
}

func (c *Client) GetElastiCacheMetrics(ctx context.Context, resource model.ElastiCacheResource, window time.Duration) (*model.MetricSnapshot, error) {
	if strings.TrimSpace(resource.ID) == "" {
		return nil, fmt.Errorf("ElastiCache identifier is required")
	}
	if window <= 0 {
		window = 15 * time.Minute
	}
	end := time.Now()
	return c.GetMetricSeries(ctx, model.MetricRequest{
		StartTime: end.Add(-window), EndTime: end, MaxPoints: defaultMetricMaxPoints,
		Queries: elasticacheMetricQueries(resource),
	})
}

func (c *Client) listElastiCacheClusters(ctx context.Context) ([]elasticachetypes.CacheCluster, error) {
	paginator := elasticache.NewDescribeCacheClustersPaginator(c.ElastiCache, &elasticache.DescribeCacheClustersInput{ShowCacheNodeInfo: awssdk.Bool(true)})
	var clusters []elasticachetypes.CacheCluster
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		clusters = append(clusters, page.CacheClusters...)
	}
	return clusters, nil
}

func elasticacheReplicationGroup(group elasticachetypes.ReplicationGroup, clusters map[string]elasticachetypes.CacheCluster) model.ElastiCacheResource {
	resource := model.ElastiCacheResource{
		Kind: model.ElastiCacheReplicationGroup, ID: awssdk.ToString(group.ReplicationGroupId), ARN: awssdk.ToString(group.ARN),
		Description: awssdk.ToString(group.Description), Engine: awssdk.ToString(group.Engine), Status: awssdk.ToString(group.Status),
		NodeType: awssdk.ToString(group.CacheNodeType), ShardCount: len(group.NodeGroups), MemberClusterIDs: append([]string(nil), group.MemberClusters...),
		ClusterMode: string(group.ClusterMode), AutomaticFailover: string(group.AutomaticFailover), MultiAZ: string(group.MultiAZ),
		AtRestEncrypted: awssdk.ToBool(group.AtRestEncryptionEnabled), TransitEncrypted: awssdk.ToBool(group.TransitEncryptionEnabled),
		AuthTokenEnabled: awssdk.ToBool(group.AuthTokenEnabled), KMSKeyID: awssdk.ToString(group.KmsKeyId),
		UserGroupIDs: append([]string(nil), group.UserGroupIds...), SnapshotRetention: awssdk.ToInt32(group.SnapshotRetentionLimit),
		SnapshotWindow: awssdk.ToString(group.SnapshotWindow), Created: awssdk.ToTime(group.ReplicationGroupCreateTime),
		NetworkType: string(group.NetworkType), LogGroups: elasticacheLogGroups(group.LogDeliveryConfigurations),
	}
	if group.ConfigurationEndpoint != nil {
		resource.Endpoint, resource.Port = elasticacheEndpoint(group.ConfigurationEndpoint)
	}
	securityGroups, zones := map[string]struct{}{}, map[string]struct{}{}
	for _, shard := range group.NodeGroups {
		for _, member := range shard.NodeGroupMembers {
			endpoint, port := elasticacheEndpoint(member.ReadEndpoint)
			resource.Nodes = append(resource.Nodes, model.ElastiCacheNode{ClusterID: awssdk.ToString(member.CacheClusterId), NodeID: awssdk.ToString(member.CacheNodeId),
				ShardID: awssdk.ToString(shard.NodeGroupId), Role: awssdk.ToString(member.CurrentRole), Status: awssdk.ToString(shard.Status),
				Endpoint: endpoint, Port: port, AvailabilityZone: awssdk.ToString(member.PreferredAvailabilityZone)})
			zones[awssdk.ToString(member.PreferredAvailabilityZone)] = struct{}{}
		}
		if resource.Endpoint == "" {
			resource.Endpoint, resource.Port = elasticacheEndpoint(shard.PrimaryEndpoint)
		}
		if resource.ReaderEndpoint == "" {
			resource.ReaderEndpoint, _ = elasticacheEndpoint(shard.ReaderEndpoint)
		}
	}
	resource.NodeCount = len(resource.Nodes)
	for _, clusterID := range group.MemberClusters {
		cluster, ok := clusters[clusterID]
		if !ok {
			continue
		}
		if resource.Version == "" {
			resource.Version = awssdk.ToString(cluster.EngineVersion)
			resource.SubnetGroup = awssdk.ToString(cluster.CacheSubnetGroupName)
			resource.MaintenanceWindow = awssdk.ToString(cluster.PreferredMaintenanceWindow)
			if cluster.CacheParameterGroup != nil {
				resource.ParameterGroup = awssdk.ToString(cluster.CacheParameterGroup.CacheParameterGroupName)
			}
		}
		for _, group := range cluster.SecurityGroups {
			securityGroups[awssdk.ToString(group.SecurityGroupId)] = struct{}{}
		}
		resource.LogGroups = append(resource.LogGroups, elasticacheLogGroups(cluster.LogDeliveryConfigurations)...)
	}
	resource.SecurityGroupIDs = sortedNonEmptyKeys(securityGroups)
	resource.AvailabilityZones = sortedNonEmptyKeys(zones)
	resource.LogGroups = uniqueSortedStrings(resource.LogGroups)
	return resource
}

func elasticacheCluster(cluster elasticachetypes.CacheCluster) model.ElastiCacheResource {
	resource := model.ElastiCacheResource{
		Kind: model.ElastiCacheCluster, ID: awssdk.ToString(cluster.CacheClusterId), ARN: awssdk.ToString(cluster.ARN),
		Engine: awssdk.ToString(cluster.Engine), Version: awssdk.ToString(cluster.EngineVersion), Status: awssdk.ToString(cluster.CacheClusterStatus),
		NodeType: awssdk.ToString(cluster.CacheNodeType), NodeCount: int(awssdk.ToInt32(cluster.NumCacheNodes)),
		ReplicationGroupID: awssdk.ToString(cluster.ReplicationGroupId), SubnetGroup: awssdk.ToString(cluster.CacheSubnetGroupName),
		NetworkType: string(cluster.NetworkType), AtRestEncrypted: awssdk.ToBool(cluster.AtRestEncryptionEnabled),
		TransitEncrypted: awssdk.ToBool(cluster.TransitEncryptionEnabled), AuthTokenEnabled: awssdk.ToBool(cluster.AuthTokenEnabled),
		SnapshotRetention: awssdk.ToInt32(cluster.SnapshotRetentionLimit), SnapshotWindow: awssdk.ToString(cluster.SnapshotWindow),
		MaintenanceWindow: awssdk.ToString(cluster.PreferredMaintenanceWindow), Created: awssdk.ToTime(cluster.CacheClusterCreateTime),
		LogGroups: elasticacheLogGroups(cluster.LogDeliveryConfigurations),
	}
	if cluster.CacheParameterGroup != nil {
		resource.ParameterGroup = awssdk.ToString(cluster.CacheParameterGroup.CacheParameterGroupName)
	}
	resource.Endpoint, resource.Port = elasticacheEndpoint(cluster.ConfigurationEndpoint)
	for _, group := range cluster.SecurityGroups {
		if id := awssdk.ToString(group.SecurityGroupId); id != "" {
			resource.SecurityGroupIDs = append(resource.SecurityGroupIDs, id)
		}
	}
	for _, node := range cluster.CacheNodes {
		endpoint, port := elasticacheEndpoint(node.Endpoint)
		resource.Nodes = append(resource.Nodes, model.ElastiCacheNode{ClusterID: resource.ID, NodeID: awssdk.ToString(node.CacheNodeId),
			Status: awssdk.ToString(node.CacheNodeStatus), Endpoint: endpoint, Port: port,
			AvailabilityZone: awssdk.ToString(node.CustomerAvailabilityZone), Created: awssdk.ToTime(node.CacheNodeCreateTime)})
		if resource.Endpoint == "" {
			resource.Endpoint, resource.Port = endpoint, port
		}
		if zone := awssdk.ToString(node.CustomerAvailabilityZone); zone != "" {
			resource.AvailabilityZones = append(resource.AvailabilityZones, zone)
		}
	}
	resource.SecurityGroupIDs = uniqueSortedStrings(resource.SecurityGroupIDs)
	resource.AvailabilityZones = uniqueSortedStrings(resource.AvailabilityZones)
	return resource
}

func elasticacheServerless(cache elasticachetypes.ServerlessCache) model.ElastiCacheResource {
	resource := model.ElastiCacheResource{
		Kind: model.ElastiCacheServerless, ID: awssdk.ToString(cache.ServerlessCacheName), ARN: awssdk.ToString(cache.ARN),
		Description: awssdk.ToString(cache.Description), Engine: awssdk.ToString(cache.Engine), Version: awssdk.ToString(cache.FullEngineVersion),
		Status: awssdk.ToString(cache.Status), NetworkType: string(cache.NetworkType), AtRestEncrypted: true, TransitEncrypted: true,
		KMSKeyID: awssdk.ToString(cache.KmsKeyId), SubnetIDs: append([]string(nil), cache.SubnetIds...),
		SecurityGroupIDs: append([]string(nil), cache.SecurityGroupIds...), SnapshotRetention: awssdk.ToInt32(cache.SnapshotRetentionLimit),
		SnapshotWindow: awssdk.ToString(cache.DailySnapshotTime), Created: awssdk.ToTime(cache.CreateTime),
	}
	resource.Endpoint, resource.Port = elasticacheEndpoint(cache.Endpoint)
	resource.ReaderEndpoint, _ = elasticacheEndpoint(cache.ReaderEndpoint)
	if id := awssdk.ToString(cache.UserGroupId); id != "" {
		resource.UserGroupIDs = []string{id}
	}
	if cache.CacheUsageLimits != nil {
		if cache.CacheUsageLimits.DataStorage != nil {
			resource.DataStorageLimitGB = float64(awssdk.ToInt32(cache.CacheUsageLimits.DataStorage.Maximum))
		}
		if cache.CacheUsageLimits.ECPUPerSecond != nil {
			resource.ECPUPerSecondLimit = int64(awssdk.ToInt32(cache.CacheUsageLimits.ECPUPerSecond.Maximum))
		}
	}
	return resource
}

func elasticacheEndpoint(endpoint *elasticachetypes.Endpoint) (string, int32) {
	if endpoint == nil {
		return "", 0
	}
	return awssdk.ToString(endpoint.Address), awssdk.ToInt32(endpoint.Port)
}

func elasticacheLogGroups(configurations []elasticachetypes.LogDeliveryConfiguration) []string {
	var groups []string
	for _, configuration := range configurations {
		if configuration.DestinationDetails == nil || configuration.DestinationDetails.CloudWatchLogsDetails == nil {
			continue
		}
		if group := awssdk.ToString(configuration.DestinationDetails.CloudWatchLogsDetails.LogGroup); group != "" {
			groups = append(groups, group)
		}
	}
	return uniqueSortedStrings(groups)
}

func elasticacheMetricQueries(resource model.ElastiCacheResource) []model.MetricQuery {
	dimensionName := "CacheClusterId"
	if resource.Kind == model.ElastiCacheReplicationGroup {
		dimensionName = "ReplicationGroupId"
	} else if resource.Kind == model.ElastiCacheServerless {
		dimensionName = "CacheName"
	}
	dimensions := []model.MetricDimension{{Name: dimensionName, Value: resource.ID}}
	metric := func(id, label, name, unit string, scale float64) model.MetricQuery {
		return model.MetricQuery{ID: id, Label: label, Namespace: "AWS/ElastiCache", MetricName: name,
			Dimensions: dimensions, Statistic: "Average", Unit: unit, Scale: scale}
	}
	if resource.Kind == model.ElastiCacheServerless {
		return []model.MetricQuery{
			metric("ecpu", "ECPU utilization", "ECPUUtilization", "%", 1),
			metric("storage", "Bytes used", "BytesUsedForCache", "bytes", 1),
			metric("connections", "Connections", "CurrConnections", "count", 1),
			metric("hits", "Hits", "CacheHits", "count", 1), metric("misses", "Misses", "CacheMisses", "count", 1),
			metric("network_in", "Received", "NetworkBytesIn", "bytes/s", 1), metric("network_out", "Transmitted", "NetworkBytesOut", "bytes/s", 1),
		}
	}
	return []model.MetricQuery{
		metric("cpu", "CPU", "CPUUtilization", "%", 1), metric("engine_cpu", "Engine CPU", "EngineCPUUtilization", "%", 1),
		metric("memory", "Free memory", "FreeableMemory", "bytes", 1), metric("storage", "Bytes used", "BytesUsedForCache", "bytes", 1),
		metric("connections", "Connections", "CurrConnections", "count", 1), metric("evictions", "Evictions", "Evictions", "count", 1),
		metric("hits", "Hits", "CacheHits", "count", 1), metric("misses", "Misses", "CacheMisses", "count", 1),
		metric("network_in", "Received", "NetworkBytesIn", "bytes/s", 1), metric("network_out", "Transmitted", "NetworkBytesOut", "bytes/s", 1),
		metric("replication_lag", "Replica lag", "ReplicationLag", "seconds", 1),
	}
}

func sortedNonEmptyKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result
}

func uniqueSortedStrings(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = struct{}{}
		}
	}
	return sortedNonEmptyKeys(set)
}
