package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

type APIGatewayAPI interface {
	ListAPIGateways(context.Context, model.APIGatewayKind) ([]model.APIGatewayAPI, error)
	DescribeAPIGateway(context.Context, model.APIGatewayKind, string) (*model.APIGatewayAPI, error)
	GetAPIGatewayMetrics(context.Context, model.APIGatewayAPI, time.Duration) (*model.MetricSnapshot, error)
}

type APIGateway struct{ api APIGatewayAPI }

func NewAPIGateway(api APIGatewayAPI) *APIGateway { return &APIGateway{api: api} }

func (s *APIGateway) List(ctx context.Context, kind model.APIGatewayKind, filter string) ([]model.APIGatewayAPI, error) {
	resources, err := s.api.ListAPIGateways(ctx, kind)
	if err != nil {
		return nil, fmt.Errorf("list API Gateway %s resources: %w", kind, err)
	}
	resources = FilterAPIGateways(resources, filter)
	sort.SliceStable(resources, func(i, j int) bool { return strings.ToLower(resources[i].Name) < strings.ToLower(resources[j].Name) })
	return resources, nil
}

func (s *APIGateway) Detail(ctx context.Context, kind model.APIGatewayKind, id string) (*model.APIGatewayAPI, error) {
	detail, err := s.api.DescribeAPIGateway(ctx, kind, strings.TrimSpace(id))
	if err != nil {
		return nil, fmt.Errorf("describe API Gateway %s %q: %w", kind, id, err)
	}
	return detail, nil
}

func (s *APIGateway) Metrics(ctx context.Context, resource model.APIGatewayAPI, window time.Duration) (*model.MetricSnapshot, error) {
	snapshot, err := s.api.GetAPIGatewayMetrics(ctx, resource, window)
	if err != nil {
		return nil, fmt.Errorf("load API Gateway metrics for %q: %w", resource.Name, err)
	}
	return snapshot, nil
}

func FilterAPIGateways(resources []model.APIGatewayAPI, filter string) []model.APIGatewayAPI {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return append([]model.APIGatewayAPI(nil), resources...)
	}
	result := make([]model.APIGatewayAPI, 0, len(resources))
	for _, resource := range resources {
		values := []string{resource.ID, resource.Name, resource.Description, resource.Protocol, resource.Endpoint, resource.Status, strings.Join(resource.EndpointTypes, " ")}
		if strings.Contains(strings.ToLower(strings.Join(values, "\x00")), filter) {
			result = append(result, resource)
		}
	}
	return result
}
