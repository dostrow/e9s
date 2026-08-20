package service

import (
	"context"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

type fakeAPIGatewayAPI struct {
	resources []model.APIGatewayAPI
	detail    *model.APIGatewayAPI
	snapshot  *model.MetricSnapshot
}

func (f fakeAPIGatewayAPI) ListAPIGateways(context.Context, model.APIGatewayKind) ([]model.APIGatewayAPI, error) {
	return f.resources, nil
}
func (f fakeAPIGatewayAPI) DescribeAPIGateway(context.Context, model.APIGatewayKind, string) (*model.APIGatewayAPI, error) {
	return f.detail, nil
}
func (f fakeAPIGatewayAPI) GetAPIGatewayMetrics(context.Context, model.APIGatewayAPI, time.Duration) (*model.MetricSnapshot, error) {
	return f.snapshot, nil
}

func TestAPIGatewayListFiltersAndSorts(t *testing.T) {
	svc := NewAPIGateway(fakeAPIGatewayAPI{resources: []model.APIGatewayAPI{
		{ID: "b", Name: "Zulu", Protocol: "HTTP"},
		{ID: "a", Name: "Alpha", Protocol: "REST"},
	}})
	resources, err := svc.List(context.Background(), model.APIGatewayREST, "rest")
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 1 || resources[0].Name != "Alpha" {
		t.Fatalf("unexpected resources: %#v", resources)
	}
}
