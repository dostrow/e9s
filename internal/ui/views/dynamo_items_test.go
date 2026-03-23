package views

import (
	"testing"

	"github.com/dostrow/e9s/internal/aws"
)

func TestDiscoverColumns(t *testing.T) {
	items := []aws.DynamoItem{
		{"id": "1", "name": "Alice"},
		{"id": "2", "name": "Bob", "email": "bob@test.com"},
	}

	cols := discoverColumns(items)

	// Should have 3 columns: email, id, name (sorted)
	if len(cols) != 3 {
		t.Fatalf("columns count = %d, want 3", len(cols))
	}
	if cols[0] != "email" {
		t.Errorf("cols[0] = %q, want %q", cols[0], "email")
	}
	if cols[1] != "id" {
		t.Errorf("cols[1] = %q, want %q", cols[1], "id")
	}
	if cols[2] != "name" {
		t.Errorf("cols[2] = %q, want %q", cols[2], "name")
	}
}

func TestDiscoverColumns_Empty(t *testing.T) {
	cols := discoverColumns(nil)
	if len(cols) != 0 {
		t.Errorf("Expected no columns for nil items, got %d", len(cols))
	}
}

func TestFormatDynamoValue(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"nil", nil, ""},
		{"string", "hello", "hello"},
		{"multiline string", "line1\nline2\nline3", `line1\nline2\nline3`},
		{"integer float", float64(42), "42"},
		{"decimal float", float64(3.14), "3.14"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"map", map[string]interface{}{"a": "b"}, `{"a":"b"}`},
		{"slice", []interface{}{"a", "b"}, `["a","b"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDynamoValue(tt.input)
			if got != tt.want {
				t.Errorf("formatDynamoValue(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDynamoItemsModel_SetItems(t *testing.T) {
	m := NewDynamoItems("test-table")
	items := []aws.DynamoItem{
		{"id": "1"},
		{"id": "2"},
	}

	m = m.SetItems(items, true)

	if len(m.items) != 2 {
		t.Errorf("items count = %d, want 2", len(m.items))
	}
	if !m.HasMore() {
		t.Error("HasMore should be true")
	}
	if m.TableName() != "test-table" {
		t.Errorf("TableName = %q, want %q", m.TableName(), "test-table")
	}
}

func TestDynamoItemsModel_SelectedItem(t *testing.T) {
	m := NewDynamoItems("test-table")
	m = m.SetItems([]aws.DynamoItem{{"id": "1"}, {"id": "2"}}, false)

	item := m.SelectedItem()
	if item == nil {
		t.Fatal("SelectedItem should not be nil")
	}
	if (*item)["id"] != "1" {
		t.Errorf("SelectedItem id = %v, want %q", (*item)["id"], "1")
	}
}

func TestDynamoItemsModel_EmptySelection(t *testing.T) {
	m := NewDynamoItems("test-table")
	if m.SelectedItem() != nil {
		t.Error("SelectedItem should be nil for empty list")
	}
}
