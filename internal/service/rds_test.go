package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/model"
)

type fakeRDSAPI struct {
	clusters  []model.RDSCluster
	cluster   *model.RDSCluster
	instances []model.RDSInstance
	detail    *model.RDSInstanceDetail
	metrics   *model.MetricSnapshot
	err       error
	id        string
}

func (f *fakeRDSAPI) ListRDSClusters(context.Context, string) ([]model.RDSCluster, error) {
	return append([]model.RDSCluster(nil), f.clusters...), f.err
}
func (f *fakeRDSAPI) DescribeRDSCluster(_ context.Context, id string) (*model.RDSCluster, error) {
	f.id = id
	return f.cluster, f.err
}

func (f *fakeRDSAPI) ListRDSInstances(context.Context, string) ([]model.RDSInstance, error) {
	return append([]model.RDSInstance(nil), f.instances...), f.err
}
func (f *fakeRDSAPI) DescribeRDSInstance(_ context.Context, id string) (*model.RDSInstanceDetail, error) {
	f.id = id
	return f.detail, f.err
}
func (f *fakeRDSAPI) GetRDSMetrics(_ context.Context, id string, _ time.Duration) (*model.MetricSnapshot, error) {
	f.id = id
	return f.metrics, f.err
}

func TestRDSListSortsAndWrapsErrors(t *testing.T) {
	api := &fakeRDSAPI{instances: []model.RDSInstance{{Identifier: "z"}, {Identifier: "A"}}}
	instances, err := NewRDS(api).List(context.Background(), "")
	if err != nil || len(instances) != 2 || instances[0].Identifier != "A" {
		t.Fatalf("List() = %#v, %v", instances, err)
	}
	api.err = errors.New("denied")
	if _, err := NewRDS(api).List(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "list RDS instances") {
		t.Fatalf("List() error = %v", err)
	}
}

func TestRDSDetailAndMetricsValidateIdentifiers(t *testing.T) {
	api := &fakeRDSAPI{
		detail:  &model.RDSInstanceDetail{RDSInstance: model.RDSInstance{Identifier: "db-1"}},
		metrics: &model.MetricSnapshot{Period: time.Minute},
	}
	if _, err := NewRDS(api).Detail(context.Background(), " db-1 "); err != nil || api.id != "db-1" {
		t.Fatalf("Detail() error = %v, id = %q", err, api.id)
	}
	if _, err := NewRDS(api).Metrics(context.Background(), "db-1", time.Hour); err != nil {
		t.Fatalf("Metrics() error = %v", err)
	}
	if _, err := NewRDS(api).Metrics(context.Background(), "", time.Hour); err == nil {
		t.Fatal("Metrics() accepted an empty identifier")
	}
}

func TestRDSClustersSortFilterAndDrillIntoInstances(t *testing.T) {
	api := &fakeRDSAPI{
		clusters: []model.RDSCluster{{Identifier: "z", Engine: "aurora-postgresql"}, {Identifier: "A", Status: "available"}},
		cluster:  &model.RDSCluster{Identifier: "A"},
		instances: []model.RDSInstance{
			{Identifier: "db-2", ClusterID: "other"}, {Identifier: "db-1", ClusterID: "A"},
		},
	}
	svc := NewRDS(api)
	clusters, err := svc.Clusters(context.Background(), "")
	if err != nil || len(clusters) != 2 || clusters[0].Identifier != "A" {
		t.Fatalf("Clusters() = %#v, %v", clusters, err)
	}
	if got := FilterRDSClusters(clusters, "postgres"); len(got) != 1 || got[0].Identifier != "z" {
		t.Fatalf("FilterRDSClusters() = %#v", got)
	}
	if _, err := svc.Cluster(context.Background(), " A "); err != nil || api.id != "A" {
		t.Fatalf("Cluster() error = %v, id = %q", err, api.id)
	}
	instances, err := svc.ClusterInstances(context.Background(), "A")
	if err != nil || len(instances) != 1 || instances[0].Identifier != "db-1" {
		t.Fatalf("ClusterInstances() = %#v, %v", instances, err)
	}
}
