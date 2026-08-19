package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

// RDSAPI is the low-level RDS and CloudWatch behavior used by both frontends.
type RDSAPI interface {
	ListRDSInstances(context.Context, string) ([]model.RDSInstance, error)
	DescribeRDSInstance(context.Context, string) (*model.RDSInstanceDetail, error)
	GetRDSMetrics(context.Context, string, time.Duration) (*model.MetricSnapshot, error)
}

type RDS struct{ api RDSAPI }

func NewRDS(api RDSAPI) *RDS { return &RDS{api: api} }

func (s *RDS) List(ctx context.Context, filter string) ([]model.RDSInstance, error) {
	instances, err := s.api.ListRDSInstances(ctx, strings.TrimSpace(filter))
	if err != nil {
		return nil, fmt.Errorf("list RDS instances: %w", err)
	}
	sort.SliceStable(instances, func(i, j int) bool {
		return strings.ToLower(instances[i].Identifier) < strings.ToLower(instances[j].Identifier)
	})
	return instances, nil
}

func FilterRDSInstances(instances []model.RDSInstance, filter string) []model.RDSInstance {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return append([]model.RDSInstance(nil), instances...)
	}
	result := make([]model.RDSInstance, 0, len(instances))
	for _, instance := range instances {
		values := []string{instance.Identifier, instance.Engine, instance.Version, instance.Class,
			instance.Status, instance.Role, instance.ClusterID, instance.AZ, instance.Endpoint}
		for _, value := range values {
			if strings.Contains(strings.ToLower(value), filter) {
				result = append(result, instance)
				break
			}
		}
	}
	return result
}

func (s *RDS) Detail(ctx context.Context, identifier string) (*model.RDSInstanceDetail, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, fmt.Errorf("read RDS instance: identifier is required")
	}
	detail, err := s.api.DescribeRDSInstance(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("read RDS instance %q: %w", identifier, err)
	}
	if detail == nil {
		return nil, fmt.Errorf("read RDS instance %q: instance was not found", identifier)
	}
	return detail, nil
}

func (s *RDS) Metrics(ctx context.Context, identifier string, window time.Duration) (*model.MetricSnapshot, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return nil, fmt.Errorf("read metrics for RDS instance: identifier is required")
	}
	if window <= 0 {
		return nil, fmt.Errorf("read metrics for RDS instance %q: time range must be positive", identifier)
	}
	metrics, err := s.api.GetRDSMetrics(ctx, identifier, window)
	if err != nil {
		return nil, fmt.Errorf("read metrics for RDS instance %q: %w", identifier, err)
	}
	return metrics, nil
}
