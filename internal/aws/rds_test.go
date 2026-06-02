package aws

import "testing"

func TestRDSRole_AuroraWriter(t *testing.T) {
	writerMap := map[string]bool{"db-1": true, "db-2": false}
	got := rdsRole("db-1", "my-cluster", "", false, writerMap)
	if got != "writer" {
		t.Errorf("rdsRole = %q, want %q", got, "writer")
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
