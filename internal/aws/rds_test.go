package aws

import (
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

func TestRDSClusterFromSDKPreservesMembersAndConfiguration(t *testing.T) {
	created := time.Now().Add(-time.Hour)
	cluster := rdsClusterFromSDK(rdstypes.DBCluster{
		DBClusterIdentifier: awssdk.String("cluster-1"), Engine: awssdk.String("aurora-postgresql"),
		EngineVersion: awssdk.String("16.3"), Status: awssdk.String("available"), Endpoint: awssdk.String("writer.local"),
		ReaderEndpoint: awssdk.String("reader.local"), Port: awssdk.Int32(5432), ClusterCreateTime: &created,
		DBClusterMembers:  []rdstypes.DBClusterMember{{DBInstanceIdentifier: awssdk.String("db-1"), IsClusterWriter: awssdk.Bool(true)}},
		VpcSecurityGroups: []rdstypes.VpcSecurityGroupMembership{{VpcSecurityGroupId: awssdk.String("sg-1"), Status: awssdk.String("active")}},
	})
	if cluster.Identifier != "cluster-1" || cluster.Endpoint != "writer.local" || len(cluster.Members) != 1 || !cluster.Members[0].Writer {
		t.Fatalf("rdsClusterFromSDK() = %#v", cluster)
	}
	if len(cluster.SecurityGroups) != 1 || cluster.SecurityGroups[0] != "sg-1 (active)" {
		t.Fatalf("security groups = %#v", cluster.SecurityGroups)
	}
}

func TestRDSRole_AuroraWriter(t *testing.T) {
	writerMap := map[string]bool{"db-1": true, "db-2": false}
	got := rdsRole("db-1", "my-cluster", "", false, writerMap)
	if got != "writer" {
		t.Errorf("rdsRole = %q, want %q", got, "writer")
	}
}

func TestRDSMetricQueriesIncludeAggregateDBLoad(t *testing.T) {
	queries := rdsMetricQueries("db-1")
	wanted := map[string]string{
		"db_load": "DBLoad", "db_load_cpu": "DBLoadCPU",
		"db_load_non_cpu": "DBLoadNonCPU", "db_load_per_vcpu": "DBLoadRelativeToNumVCPUs",
	}
	for _, query := range queries {
		if metric, ok := wanted[query.ID]; ok {
			if query.MetricName != metric || len(query.Dimensions) != 1 || query.Dimensions[0].Value != "db-1" {
				t.Errorf("query %q = %#v", query.ID, query)
			}
			delete(wanted, query.ID)
		}
	}
	if len(wanted) != 0 {
		t.Fatalf("missing DB Load queries: %#v", wanted)
	}
}

func TestRDSRole_AuroraReader(t *testing.T) {
	writerMap := map[string]bool{"db-1": true, "db-2": false}
	got := rdsRole("db-2", "my-cluster", "", false, writerMap)
	if got != "reader" {
		t.Errorf("rdsRole = %q, want %q", got, "reader")
	}
}

func TestRDSRole_AuroraInstanceNotInMap(t *testing.T) {
	// Instance is in a cluster but wasn't in the cluster member list yet
	got := rdsRole("db-new", "my-cluster", "", false, map[string]bool{})
	if got != "" {
		t.Errorf("rdsRole = %q, want empty string", got)
	}
}

func TestRDSRole_AuroraNilWriterMap(t *testing.T) {
	// nil writerMap (used in DescribeRDSInstance when cluster lookup succeeds but instance not found)
	got := rdsRole("db-1", "my-cluster", "", false, nil)
	if got != "" {
		t.Errorf("rdsRole = %q, want empty string", got)
	}
}

func TestRDSRole_ReadReplica(t *testing.T) {
	got := rdsRole("db-replica", "", "db-primary", false, nil)
	if got != "replica" {
		t.Errorf("rdsRole = %q, want %q", got, "replica")
	}
}

func TestRDSRole_PrimaryWithReplicas(t *testing.T) {
	got := rdsRole("db-primary", "", "", true, nil)
	if got != "primary" {
		t.Errorf("rdsRole = %q, want %q", got, "primary")
	}
}

func TestRDSRole_Standalone(t *testing.T) {
	got := rdsRole("db-standalone", "", "", false, nil)
	if got != "" {
		t.Errorf("rdsRole = %q, want empty string for standalone", got)
	}
}

func TestRDSRole_ClusterTakesPrecedenceOverReplica(t *testing.T) {
	// If clusterID is set it wins regardless of readReplicaSource
	writerMap := map[string]bool{"db-1": false}
	got := rdsRole("db-1", "cluster-a", "some-source", false, writerMap)
	if got != "reader" {
		t.Errorf("rdsRole = %q, want %q (cluster takes precedence)", got, "reader")
	}
}
