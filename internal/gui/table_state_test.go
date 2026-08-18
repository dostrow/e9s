package gui

import "testing"

func TestStringRowsEqual(t *testing.T) {
	rows := []string{"api\tACTIVE", "worker\tACTIVE"}
	if !stringRowsEqual(rows, append([]string(nil), rows...)) {
		t.Fatal("identical table rows should not trigger a model splice")
	}
	if stringRowsEqual(rows, []string{"api\tACTIVE", "worker\tDRAINING"}) {
		t.Fatal("changed table rows must trigger a model splice")
	}
}

func TestPreservedRowPositionUsesResourceIdentity(t *testing.T) {
	current := []string{"api\tACTIVE\t2", "worker\tACTIVE\t1"}
	next := []string{"worker\tACTIVE\t2", "api\tACTIVE\t2"}

	position, ok := preservedRowPosition(current, next, 1)
	if !ok || position != 0 {
		t.Fatalf("preservedRowPosition() = %d, %v; want 0, true", position, ok)
	}
	if _, ok := preservedRowPosition(current, []string{"api\tACTIVE\t2"}, 1); ok {
		t.Fatal("removed resource should not retain a selection")
	}
}
