package views

import (
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestClusterListModel_SelectIndex(t *testing.T) {
	m := NewClusterList()
	m = m.SetClusters([]model.Cluster{
		{Name: "cluster-a"},
		{Name: "cluster-b"},
		{Name: "cluster-c"},
	})

	c := m.SelectIndex(1)
	if c == nil {
		t.Fatal("SelectIndex(1) should not be nil")
	}
	if c.Name != "cluster-b" {
		t.Errorf("SelectIndex(1) = %q, want %q", c.Name, "cluster-b")
	}

	if m.SelectIndex(-1) != nil {
		t.Error("SelectIndex(-1) should be nil")
	}
	if m.SelectIndex(10) != nil {
		t.Error("SelectIndex(10) should be nil for 3 items")
	}
}

func TestClusterListModel_WithCursor(t *testing.T) {
	m := NewClusterList()
	m = m.SetClusters([]model.Cluster{
		{Name: "a"},
		{Name: "b"},
		{Name: "c"},
	})

	m = m.WithCursor(2)
	c := m.SelectedCluster()
	if c == nil || c.Name != "c" {
		t.Errorf("WithCursor(2) selected = %v, want cluster-c", c)
	}
}

func TestClusterListModel_Filter(t *testing.T) {
	m := NewClusterList()
	m = m.SetClusters([]model.Cluster{
		{Name: "prod-cluster"},
		{Name: "dev-cluster"},
		{Name: "staging"},
	})

	// Simulate filter
	m.filter = "prod"
	filtered := m.filteredClusters()
	if len(filtered) != 1 {
		t.Errorf("Filter 'prod' should match 1, got %d", len(filtered))
	}
	if filtered[0].Name != "prod-cluster" {
		t.Errorf("Filtered result = %q, want %q", filtered[0].Name, "prod-cluster")
	}
}
