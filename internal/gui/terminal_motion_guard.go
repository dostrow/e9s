//go:build gui

package gui

import "math"

// terminalMotionGuard filters motion events synthesized when terminal content
// changes underneath a stationary pointer. VTE may encode those events as SGR
// mouse input while a full-screen child is exiting, leaving the trailing bytes
// for the shell that resumes afterward.
//
// Do not use a timer here. GTK can deliver the synthetic motion after an
// arbitrarily delayed redraw. Instead, arm on a terminal key press and suppress
// unchanged, button-free positions until genuine pointer movement occurs.
type terminalMotionGuard struct {
	armed       bool
	hasPosition bool
	x           float64
	y           float64
}

func (guard *terminalMotionGuard) noteKeyPressed() {
	guard.armed = true
}

func (guard *terminalMotionGuard) filterMotion(x, y float64, positionOK, buttonsDown bool) bool {
	if !positionOK {
		if guard.armed && !buttonsDown {
			return true
		}
		if buttonsDown {
			guard.armed = false
		}
		return false
	}

	hadPosition := guard.hasPosition
	moved := hadPosition && (math.Abs(x-guard.x) >= 0.5 || math.Abs(y-guard.y) >= 0.5)
	guard.x = x
	guard.y = y
	guard.hasPosition = true

	if !guard.armed {
		return false
	}
	if buttonsDown || moved {
		guard.armed = false
		return false
	}

	// With no prior observation, conservatively consume the first position
	// after a key press. A real pointer move produces another changed position;
	// a GTK redraw continues to report this same stationary position.
	return true
}
