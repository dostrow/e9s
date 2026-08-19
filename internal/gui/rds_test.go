//go:build gui

package gui

import (
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestFormatRDSDetailIncludesLinkedResourceIdentifiers(t *testing.T) {
	detail := model.RDSInstanceDetail{
		RDSInstance:    model.RDSInstance{Identifier: "db-1", Engine: "postgres", Version: "16.3", Endpoint: "db.local", Port: 5432, StorageType: "gp3"},
		VPCID:          "vpc-1",
		SubnetGroup:    "private-db",
		SecurityGroups: []string{"sg-1 (active)"},
	}
	text := formatRDSDetail(detail)
	for _, want := range []string{"db-1", "postgres 16.3", "vpc-1", "sg-1", "db.local:5432"} {
		if !strings.Contains(text, want) {
			t.Errorf("formatRDSDetail() did not contain %q:\n%s", want, text)
		}
	}
}

func TestFormatRDSClusterDetailIncludesMembersAndEndpoints(t *testing.T) {
	cluster := model.RDSCluster{
		Identifier: "cluster-1", Engine: "aurora-postgresql", Version: "16.3", Status: "available",
		Endpoint: "writer.local", ReaderEndpoint: "reader.local", Port: 5432,
		Members:        []model.RDSClusterMember{{Identifier: "db-writer", Writer: true}, {Identifier: "db-reader", PromotionTier: 2}},
		SecurityGroups: []string{"sg-1 (active)"},
	}
	text := formatRDSClusterDetail(cluster)
	for _, want := range []string{"cluster-1", "writer.local:5432", "reader.local:5432", "db-writer", "writer", "db-reader", "sg-1"} {
		if !strings.Contains(text, want) {
			t.Errorf("formatRDSClusterDetail() did not contain %q:\n%s", want, text)
		}
	}
}

func TestMetricSpecHasData(t *testing.T) {
	snapshot := &model.MetricSnapshot{Series: []model.MetricSeries{
		{ID: "cpu"},
		{ID: "connections", Points: []model.MetricPoint{{Value: 2}}},
	}}
	if metricSpecHasData(snapshot, []string{"cpu"}) {
		t.Fatal("empty CPU series reported data")
	}
	if !metricSpecHasData(snapshot, []string{"connections"}) {
		t.Fatal("connections series did not report data")
	}
}
