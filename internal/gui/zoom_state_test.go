package gui

import "testing"

func TestSteppedZoomClampsToSupportedRange(t *testing.T) {
	if got := steppedZoom(zoomDefault, 1); got != 110 {
		t.Fatalf("zoom in from default = %d, want 110", got)
	}
	if got := steppedZoom(zoomDefault, -1); got != 90 {
		t.Fatalf("zoom out from default = %d, want 90", got)
	}
	if got := steppedZoom(zoomMaximum, 1); got != zoomMaximum {
		t.Fatalf("zoom above maximum = %d, want %d", got, zoomMaximum)
	}
	if got := steppedZoom(zoomMinimum, -1); got != zoomMinimum {
		t.Fatalf("zoom below minimum = %d, want %d", got, zoomMinimum)
	}
}

func TestZoomClass(t *testing.T) {
	if got := zoomClass(130); got != "e9s-zoom-130" {
		t.Fatalf("zoomClass(130) = %q", got)
	}
}
