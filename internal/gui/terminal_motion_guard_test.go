//go:build gui

package gui

import "testing"

func TestTerminalMotionGuardSuppressesDelayedStationaryMotion(t *testing.T) {
	var guard terminalMotionGuard
	if guard.filterMotion(116, 12, true, false) {
		t.Fatal("ordinary motion was suppressed before a key press")
	}

	guard.noteKeyPressed()
	if !guard.filterMotion(116, 12, true, false) {
		t.Fatal("stationary synthetic motion was not suppressed")
	}
	if !guard.filterMotion(116, 12, true, false) {
		t.Fatal("a later stationary synthetic motion was not suppressed")
	}
}

func TestTerminalMotionGuardAllowsRealPointerMovement(t *testing.T) {
	var guard terminalMotionGuard
	guard.filterMotion(116, 12, true, false)
	guard.noteKeyPressed()

	if guard.filterMotion(118, 12, true, false) {
		t.Fatal("real pointer movement was suppressed")
	}
	if guard.filterMotion(118, 12, true, false) {
		t.Fatal("motion remained suppressed after real pointer movement")
	}
}

func TestTerminalMotionGuardAllowsButtonDrag(t *testing.T) {
	var guard terminalMotionGuard
	guard.filterMotion(116, 12, true, false)
	guard.noteKeyPressed()

	if guard.filterMotion(116, 12, true, true) {
		t.Fatal("button drag was suppressed")
	}
	if guard.filterMotion(116, 12, true, false) {
		t.Fatal("motion remained suppressed after a button drag")
	}
}

func TestTerminalMotionGuardHandlesUnknownFirstPosition(t *testing.T) {
	var guard terminalMotionGuard
	guard.noteKeyPressed()

	if !guard.filterMotion(0, 0, false, false) {
		t.Fatal("positionless synthetic motion was not suppressed")
	}
	if !guard.filterMotion(116, 12, true, false) {
		t.Fatal("first observed stationary position was not suppressed")
	}
	if guard.filterMotion(118, 12, true, false) {
		t.Fatal("real pointer movement after the first observation was suppressed")
	}
}
