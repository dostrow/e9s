package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

type fakeElastiCacheAPI struct {
	resources []model.ElastiCacheResource
	kind      model.ElastiCacheKind
	err       error
}

func (f *fakeElastiCacheAPI) ListElastiCacheReplicationGroups(context.Context) ([]model.ElastiCacheResource, error) {
	f.kind = model.ElastiCacheReplicationGroup
	return f.resources, f.err
}
func (f *fakeElastiCacheAPI) ListElastiCacheClusters(context.Context) ([]model.ElastiCacheResource, error) {
	f.kind = model.ElastiCacheCluster
	return f.resources, f.err
}
func (f *fakeElastiCacheAPI) ListElastiCacheServerless(context.Context) ([]model.ElastiCacheResource, error) {
	f.kind = model.ElastiCacheServerless
	return f.resources, f.err
}
func (f *fakeElastiCacheAPI) DescribeElastiCache(context.Context, model.ElastiCacheKind, string) (*model.ElastiCacheResource, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &f.resources[0], nil
}
func (f *fakeElastiCacheAPI) GetElastiCacheMetrics(context.Context, model.ElastiCacheResource, time.Duration) (*model.MetricSnapshot, error) {
	return &model.MetricSnapshot{}, f.err
}

func TestElastiCacheListFiltersAndSorts(t *testing.T) {
	api := &fakeElastiCacheAPI{resources: []model.ElastiCacheResource{
		{ID: "zeta", Engine: "valkey"},
		{ID: "Alpha", Engine: "redis"},
		{ID: "middle", SecurityGroupIDs: []string{"sg-match"}},
	}}
	resources, err := NewElastiCache(api).List(context.Background(), model.ElastiCacheCluster, "")
	if err != nil {
		t.Fatal(err)
	}
	if api.kind != model.ElastiCacheCluster || len(resources) != 3 || resources[0].ID != "Alpha" || resources[1].ID != "middle" || resources[2].ID != "zeta" {
		t.Fatalf("resources = %+v, kind = %q", resources, api.kind)
	}
	resources, err = NewElastiCache(api).List(context.Background(), model.ElastiCacheCluster, "sg-match")
	if err != nil || len(resources) != 1 || resources[0].ID != "middle" {
		t.Fatalf("security-group filter = %+v, %v", resources, err)
	}
}

func TestElastiCacheListWrapsErrors(t *testing.T) {
	_, err := NewElastiCache(&fakeElastiCacheAPI{err: errors.New("denied")}).List(context.Background(), model.ElastiCacheServerless, "")
	if err == nil || err.Error() != "list ElastiCache serverless-cache resources: denied" {
		t.Fatalf("error = %v", err)
	}
}
