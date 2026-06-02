package views

import (
	"strings"
	"testing"
	"time"

	"github.com/dostrow/e9s/internal/aws"
)

// --- rdsStatusStyle ---

func TestRDSStatusStyle_Available(t *testing.T) {
	s := rdsStatusStyle("available")
	rendered := s.Render("available")
	if rendered == "" {
		t.Error("expected non-empty render for available")
	}
}

func TestRDSStatusStyle_KnownStatuses(t *testing.T) {
	// Smoke test: none of these should panic
	for _, status := range []string{
		"available", "stopped", "starting", "stopping", "modifying",
		"upgrading", "rebooting", "backing-up", "maintenance",
		"failed", "incompatible-restore", "incompatible-network",
		"incompatible-parameters", "incompatible-option-group", "restore-error",
		"unknown-status-xyz",
	} {
		_ = rdsStatusStyle(status).Render(status)
	}
}

// --- rdsRoleStyle ---

func TestRDSRoleStyle_KnownRoles(t *testing.T) {
	for _, role := range []string{"writer", "reader", "primary", "replica", ""} {
		_ = rdsRoleStyle(role).Render(role)
	}
}

// --- fmtPct ---

func TestFmtPct_Green(t *testing.T) {
	got := fmtPct(50.0)
	if !strings.Contains(got, "50.0%") {
		t.Errorf("fmtPct(50.0) = %q, want to contain 50.0%%", got)
	}
}

func TestFmtPct_Yellow(t *testing.T) {
	got := fmtPct(75.0)
	if !strings.Contains(got, "75.0%") {
		t.Errorf("fmtPct(75.0) = %q, want to contain 75.0%%", got)
	}
}

func TestFmtPct_Red(t *testing.T) {
	got := fmtPct(95.0)
	if !strings.Contains(got, "95.0%") {
		t.Errorf("fmtPct(95.0) = %q, want to contain 95.0%%", got)
	}
}

func TestFmtPct_Boundary_70(t *testing.T) {
	// 70 is the yellow threshold
	got := fmtPct(70.0)
	if !strings.Contains(got, "70.0%") {
		t.Errorf("fmtPct(70.0) = %q", got)
	}
}

func TestFmtPct_Boundary_90(t *testing.T) {
	// 90 is the red threshold
	got := fmtPct(90.0)
	if !strings.Contains(got, "90.0%") {
		t.Errorf("fmtPct(90.0) = %q", got)
	}
}

// --- sortedKeys ---

func TestSortedKeys_Ordered(t *testing.T) {
	m := map[string]string{"zebra": "z", "apple": "a", "mango": "m"}
	got := sortedKeys(m)
	want := []string{"apple", "mango", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("sortedKeys len = %d, want %d", len(got), len(want))
	}
	for i, k := range got {
		if k != want[i] {
			t.Errorf("sortedKeys[%d] = %q, want %q", i, k, want[i])
		}
	}
}

func TestSortedKeys_Empty(t *testing.T) {
	got := sortedKeys(map[string]string{})
	if len(got) != 0 {
		t.Errorf("sortedKeys(empty) = %v, want []", got)
	}
}

func TestSortedKeys_Single(t *testing.T) {
	got := sortedKeys(map[string]string{"only": "v"})
	if len(got) != 1 || got[0] != "only" {
		t.Errorf("sortedKeys({only}) = %v", got)
	}
}

// --- RDSInstancesModel ---

func TestRDSInstancesModel_FilterByIdentifier(t *testing.T) {
	m := NewRDSInstances()
	m = m.SetInstances([]aws.RDSInstance{
		{Identifier: "prod-db-1", Engine: "postgres"},
		{Identifier: "staging-db-2", Engine: "mysql"},
		{Identifier: "prod-db-3", Engine: "postgres"},
	})
	m.filter = "prod"
	filtered := m.filteredInstances()
	if len(filtered) != 2 {
		t.Errorf("filter 'prod' returned %d instances, want 2", len(filtered))
	}
}

func TestRDSInstancesModel_FilterByEngine(t *testing.T) {
	m := NewRDSInstances()
	m = m.SetInstances([]aws.RDSInstance{
		{Identifier: "db-1", Engine: "postgres"},
		{Identifier: "db-2", Engine: "mysql"},
	})
	m.filter = "mysql"
	filtered := m.filteredInstances()
	if len(filtered) != 1 || filtered[0].Identifier != "db-2" {
		t.Errorf("filter 'mysql' returned %v", filtered)
	}
}

func TestRDSInstancesModel_FilterByRole(t *testing.T) {
	m := NewRDSInstances()
	m = m.SetInstances([]aws.RDSInstance{
		{Identifier: "db-1", Role: "writer"},
		{Identifier: "db-2", Role: "reader"},
		{Identifier: "db-3", Role: "reader"},
	})
	m.filter = "reader"
	filtered := m.filteredInstances()
	if len(filtered) != 2 {
		t.Errorf("filter 'reader' returned %d instances, want 2", len(filtered))
	}
}

func TestRDSInstancesModel_FilterByCluster(t *testing.T) {
	m := NewRDSInstances()
	m = m.SetInstances([]aws.RDSInstance{
		{Identifier: "db-1", ClusterID: "aurora-cluster-a"},
		{Identifier: "db-2", ClusterID: "aurora-cluster-b"},
	})
	m.filter = "cluster-a"
	filtered := m.filteredInstances()
	if len(filtered) != 1 || filtered[0].Identifier != "db-1" {
		t.Errorf("filter 'cluster-a' = %v", filtered)
	}
}

func TestRDSInstancesModel_EmptyFilter(t *testing.T) {
	m := NewRDSInstances()
	instances := []aws.RDSInstance{
		{Identifier: "a"}, {Identifier: "b"}, {Identifier: "c"},
	}
	m = m.SetInstances(instances)
	filtered := m.filteredInstances()
	if len(filtered) != 3 {
		t.Errorf("empty filter returned %d, want 3", len(filtered))
	}
}

func TestRDSInstancesModel_CursorClampedOnLoad(t *testing.T) {
	m := NewRDSInstances()
	m.cursor = 99
	m = m.SetInstances([]aws.RDSInstance{{Identifier: "only"}})
	if m.cursor != 0 {
		t.Errorf("cursor after SetInstances = %d, want 0", m.cursor)
	}
}

func TestRDSInstancesModel_SelectedNilWhenEmpty(t *testing.T) {
	m := NewRDSInstances()
	m = m.SetInstances([]aws.RDSInstance{})
	if m.SelectedInstance() != nil {
		t.Error("SelectedInstance on empty list should be nil")
	}
}

func TestRDSInstancesModel_SelectedReturnsCorrect(t *testing.T) {
	m := NewRDSInstances()
	m = m.SetInstances([]aws.RDSInstance{
		{Identifier: "first"},
		{Identifier: "second"},
	})
	m.cursor = 1
	inst := m.SelectedInstance()
	if inst == nil || inst.Identifier != "second" {
		t.Errorf("SelectedInstance = %v, want second", inst)
	}
}

func TestRDSInstancesModel_ViewLoading(t *testing.T) {
	m := NewRDSInstances()
	m = m.SetSize(120, 40)
	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Errorf("view before load should show Loading, got: %q", view)
	}
}

func TestRDSInstancesModel_ViewShowsInstances(t *testing.T) {
	m := NewRDSInstances()
	m = m.SetSize(200, 40)
	m = m.SetInstances([]aws.RDSInstance{
		{
			Identifier: "prod-aurora-writer",
			Engine:     "aurora-postgresql",
			Version:    "15.4",
			Class:      "db.r6g.large",
			Status:     "available",
			Role:       "writer",
			AZ:         "us-east-1a",
			Endpoint:   "prod.cluster.us-east-1.rds.amazonaws.com",
			Port:       5432,
			Created:    time.Now().Add(-24 * time.Hour),
		},
	})
	view := m.View()
	if !strings.Contains(view, "prod-aurora-writer") {
		t.Errorf("view missing identifier, got: %q", view)
	}
	if !strings.Contains(view, "aurora-postgresql") {
		t.Errorf("view missing engine, got: %q", view)
	}
	if !strings.Contains(view, "writer") {
		t.Errorf("view missing role, got: %q", view)
	}
	if !strings.Contains(view, "us-east-1a") {
		t.Errorf("view missing AZ, got: %q", view)
	}
}

// --- RDSDetailModel ---

func TestRDSDetailModel_ViewNilDetail(t *testing.T) {
	m := NewRDSDetail(nil)
	m = m.SetSize(120, 40)
	view := m.View()
	if !strings.Contains(view, "Loading") {
		t.Errorf("nil detail should show Loading, got: %q", view)
	}
}

func TestRDSDetailModel_InstanceIDEmpty(t *testing.T) {
	m := NewRDSDetail(nil)
	if m.InstanceID() != "" {
		t.Error("InstanceID on nil detail should be empty")
	}
}

func TestRDSDetailModel_ViewShowsSections(t *testing.T) {
	detail := &aws.RDSInstanceDetail{
		RDSInstance: aws.RDSInstance{
			Identifier: "my-db",
			Engine:     "postgres",
			Version:    "15.4",
			Class:      "db.t3.medium",
			Status:     "available",
			Role:       "writer",
			ClusterID:  "my-cluster",
			AZ:         "us-east-1b",
			Endpoint:   "my-db.cluster.rds.amazonaws.com",
			Port:       5432,
			StorageGB:  100,
			StorageType: "gp3",
			Encrypted:  true,
			Created:    time.Now().Add(-7 * 24 * time.Hour),
		},
		SubnetGroup:         "my-subnet-group",
		VPCID:               "vpc-abc123",
		BackupRetentionDays: 7,
		BackupWindow:        "03:00-04:00",
		MaintenanceWindow:   "sun:05:00-sun:06:00",
		MetricsLoaded:       true,
		CPUPercent:          42.5,
		DBConnections:       18,
		FreeStorageGB:       78.3,
		ReadIOPS:            120.0,
		WriteIOPS:           45.0,
		ReadLatencyMs:       0.8,
		WriteLatencyMs:      1.2,
		Tags:                map[string]string{"env": "prod", "team": "backend"},
	}

	m := NewRDSDetail(detail)
	m = m.SetSize(160, 80)
	view := m.View()

	for _, want := range []string{
		"my-db", "postgres", "writer", "my-cluster",
		"us-east-1b", "vpc-abc123", "my-subnet-group",
		"100 GiB", "gp3",
		"42.5%",       // CPU
		"18",          // connections
		"prod", "backend", // tags
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q", want)
		}
	}
}

func TestRDSDetailModel_ViewReaderShowsPromotionTier(t *testing.T) {
	detail := &aws.RDSInstanceDetail{
		RDSInstance: aws.RDSInstance{
			Identifier: "reader-1",
			Role:       "reader",
			ClusterID:  "my-cluster",
			Status:     "available",
		},
		PromotionTier: 2,
		MetricsLoaded: false,
	}
	m := NewRDSDetail(detail)
	m = m.SetSize(160, 80)
	view := m.View()
	if !strings.Contains(view, "2") {
		t.Errorf("view should show promotion tier 2, got: %q", view)
	}
}

func TestRDSDetailModel_ViewNoDeletionProtection(t *testing.T) {
	detail := &aws.RDSInstanceDetail{
		RDSInstance: aws.RDSInstance{
			Identifier:         "db-nodp",
			Status:             "available",
			DeletionProtection: false,
		},
	}
	m := NewRDSDetail(detail)
	m = m.SetSize(160, 80)
	view := m.View()
	if !strings.Contains(view, "no") {
		t.Errorf("expected 'no' for deletion protection off, got: %q", view)
	}
}
