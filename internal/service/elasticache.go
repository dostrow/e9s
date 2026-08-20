package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

type ElastiCacheAPI interface {
	ListElastiCacheReplicationGroups(context.Context) ([]model.ElastiCacheResource, error)
	ListElastiCacheClusters(context.Context) ([]model.ElastiCacheResource, error)
	ListElastiCacheServerless(context.Context) ([]model.ElastiCacheResource, error)
	DescribeElastiCache(context.Context, model.ElastiCacheKind, string) (*model.ElastiCacheResource, error)
	GetElastiCacheMetrics(context.Context, model.ElastiCacheResource, time.Duration) (*model.MetricSnapshot, error)
}

type ElastiCache struct{ api ElastiCacheAPI }

func NewElastiCache(api ElastiCacheAPI) *ElastiCache { return &ElastiCache{api: api} }

func (s *ElastiCache) List(ctx context.Context, kind model.ElastiCacheKind, filter string) ([]model.ElastiCacheResource, error) {
	var (
		resources []model.ElastiCacheResource
		err       error
	)
	switch kind {
	case model.ElastiCacheReplicationGroup:
		resources, err = s.api.ListElastiCacheReplicationGroups(ctx)
	case model.ElastiCacheCluster:
		resources, err = s.api.ListElastiCacheClusters(ctx)
	case model.ElastiCacheServerless:
		resources, err = s.api.ListElastiCacheServerless(ctx)
	default:
		return nil, fmt.Errorf("unsupported ElastiCache resource kind %q", kind)
	}
	if err != nil {
		return nil, fmt.Errorf("list ElastiCache %s resources: %w", kind, err)
	}
	resources = FilterElastiCache(resources, filter)
	sort.SliceStable(resources, func(i, j int) bool {
		return strings.ToLower(resources[i].ID) < strings.ToLower(resources[j].ID)
	})
	return resources, nil
}

func (s *ElastiCache) Detail(ctx context.Context, kind model.ElastiCacheKind, identifier string) (*model.ElastiCacheResource, error) {
	resource, err := s.api.DescribeElastiCache(ctx, kind, strings.TrimSpace(identifier))
	if err != nil {
		return nil, fmt.Errorf("describe ElastiCache %s %q: %w", kind, identifier, err)
	}
	return resource, nil
}

func (s *ElastiCache) Metrics(ctx context.Context, resource model.ElastiCacheResource, window time.Duration) (*model.MetricSnapshot, error) {
	metrics, err := s.api.GetElastiCacheMetrics(ctx, resource, window)
	if err != nil {
		return nil, fmt.Errorf("load ElastiCache metrics for %q: %w", resource.ID, err)
	}
	return metrics, nil
}

func FilterElastiCache(resources []model.ElastiCacheResource, filter string) []model.ElastiCacheResource {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return append([]model.ElastiCacheResource(nil), resources...)
	}
	result := make([]model.ElastiCacheResource, 0, len(resources))
	for _, resource := range resources {
		values := []string{resource.ID, resource.ARN, resource.Engine, resource.Version, resource.Status,
			resource.NodeType, resource.Endpoint, resource.ReaderEndpoint, resource.ReplicationGroupID,
			resource.SubnetGroup, strings.Join(resource.SecurityGroupIDs, " "), strings.Join(resource.SubnetIDs, " ")}
		if strings.Contains(strings.ToLower(strings.Join(values, "\x00")), filter) {
			result = append(result, resource)
		}
	}
	return result
}
