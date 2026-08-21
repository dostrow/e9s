package gui

import "strconv"

const (
	zoomMinimum = 70
	zoomMaximum = 200
	zoomStep    = 10
	zoomDefault = 100
)

func steppedZoom(current, direction int) int {
	if direction == 0 {
		return current
	}
	if direction > 0 {
		current += zoomStep
	} else {
		current -= zoomStep
	}
	if current < zoomMinimum {
		return zoomMinimum
	}
	if current > zoomMaximum {
		return zoomMaximum
	}
	return current
}

func zoomClass(level int) string {
	return "e9s-zoom-" + strconv.Itoa(level)
}
